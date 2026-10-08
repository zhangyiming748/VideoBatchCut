// ffmpeg 命令的统一封装。
//
// 设计目标：把散落在各处的 “ffmpeg 输入文件 + 一堆参数 + 输出文件” 收敛到一个地方。
// 顶层只需给出【输入文件名】和【输出文件名】，视频编码参数由本文件结合 hwaccel.go 的
// 硬件探测结果自动决定，调用方不必再在每一处手写编码器参数；改参数时也只改这一个文件。
//
// 典型用法（后续迁移到各调用点时）：
//
//	util.Convert("in.mp4", "out.mp4")                                  // 全自动：硬件编码 + 高质量 Opus 音频 + 覆盖
//	util.Cut("in.mp4", "out.mp4", "00:00:01.000", "00:00:05.000")      // 精确切割 + 时间戳修复
//
// 需要更细粒度控制时，直接构造/修改 Job 这个 model（也供其它函数复用）：
//
//	job := util.NewJob("in.mp4", "out.mp4")
//	job.Audio = util.AudioCopy()          // 特例：不重编码直接复制音频流（默认已是高质量 Opus）
//	job.Encoder = util.EncoderX265        // 强制某种编码器（跳过自动选择）
//	job.ExtraArgs = []string{"-map_chapters", "-1"}
//	if err := job.Run(); err != nil { ... }
//
// 只想看命令、先不执行：job.Args() 返回参数切片，job.String() 返回完整命令字符串。
//
// ⚠️ 单一事实来源：各硬件分支的视频编码参数（videoEncoderArgs）必须与 hwaccel.go 里
// 对应的 probe*（probeNvenc / probeQsv / probeAmf / probeVideoToolbox / probeMediaFoundation）保持一致——
// 探测用的就是这套参数，只有两者一致，“探测通过” 才等价于 “真实编码能跑通”。
// 改动其中一处，另一处必须同步。
//
// 流保留保证：默认（MapAllStreams）把输入的所有视频/音频/字幕/数据流全部映射到输出，
// 并按输出容器自动指定字幕编码（MP4→mov_text、MKV→srt）。含 PGS / DVD 等图形字幕的
// 文件无法封装进 MP4，转换前必须先经 IncompatibleSubtitles 预检并跳过该文件
// （只返回错误、不中断批处理），避免转换中途失败或悄悄丢字幕。
package util

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Encoder 表示一次任务最终选用的视频编码器。
type Encoder int

const (
	EncoderAuto     Encoder = iota // 交给 SelectEncoder 按硬件自动选择（Job.Encoder 的默认值）
	EncoderNvidia                  // h264_nvenc（NVIDIA NVENC）
	EncoderApple                   // h264_videotoolbox（Apple Silicon VideoToolbox）
	EncoderIntel                   // h264_qsv（Intel Quick Sync，仅 Windows）
	EncoderAMD                     // h264_amf（AMD，仅 Windows）
	EncoderQualcomm                // h264_mf（高通 Media Foundation，仅 Windows ARM64）
	EncoderX264                    // libx264（CPU，FASTCUT 快速档）
	EncoderX265                    // libx265（CPU，高质量档）
	EncoderCopy                    // -c:v copy，直接复制视频流不重编码
	EncoderNone                    // 不加任何视频编码参数（音频-only 任务，如 wav→mp3）
)

// 音频滤镜常量，供 Job.AudioFilter 组合使用。
const (
	// AudioFilterSync 强制音视频重同步，消除开头多余音频或结尾缺失音频。
	AudioFilterSync = "adelay=0|0, aresample=async=1"
	// AudioFilterStereoDownmix 把任意声道数下混为立体声。
	// 必须用 aformat 做“真下混”：实测 pan=stereo|c0=FL|c1=FR 只是按名挑通道，
	// 对单声道（FC）和 5.1（对白在中置 FC）会输出完全静音。libopus 立体声编码需要它。
	AudioFilterStereoDownmix = "aformat=channel_layouts=stereo"
)

// String 返回编码器的可读名（即 ffmpeg 的编码器名），便于日志。
func (e Encoder) String() string {
	switch e {
	case EncoderNvidia:
		return "h264_nvenc"
	case EncoderApple:
		return "h264_videotoolbox"
	case EncoderIntel:
		return "h264_qsv"
	case EncoderAMD:
		return "h264_amf"
	case EncoderQualcomm:
		return "h264_mf"
	case EncoderX264:
		return "libx264"
	case EncoderX265:
		return "libx265"
	case EncoderCopy:
		return "copy"
	case EncoderNone:
		return "none"
	default:
		return "auto"
	}
}

// SelectEncoder 结合 hwaccel.go 的探测结果，选出当前机器可用的最优视频编码器。
// 优先级：NVIDIA > Apple > Intel > AMD > Qualcomm > CPU（FASTCUT=yes 用 libx264，否则 libx265）。
// 这些探测结果都带缓存（见 hwaccel.go 的 probeCache），重复调用不会反复拉起 ffmpeg 探测进程。
func SelectEncoder() Encoder {
	switch {
	case HasNvidia():
		return EncoderNvidia
	case HasAppleSilicon():
		return EncoderApple
	case HasIntel():
		return EncoderIntel
	case HasAMD():
		return EncoderAMD
	case HasQualcomm():
		return EncoderQualcomm
	case os.Getenv("FASTCUT") == "yes":
		return EncoderX264
	default:
		return EncoderX265
	}
}

// resolveEncoder 把 EncoderAuto 解析成具体编码器，其它原样返回。
func resolveEncoder(e Encoder) Encoder {
	if e == EncoderAuto {
		return SelectEncoder()
	}
	return e
}

// videoEncoderArgs 返回某个编码器对应的视频参数（本项目的单一事实来源）。
// ⚠️ 硬件分支必须与 hwaccel.go 的 probe* 保持一致，详见文件头注释。
func videoEncoderArgs(e Encoder) []string {
	switch e {
	case EncoderNvidia:
		// 与 hwaccel.go probeNvenc 一致
		return []string{
			"-c:v", "h264_nvenc",
			"-preset", "p7",
			"-tune", "hq",
			"-rc", "vbr",
			"-b:v", "0",
			"-cq:v", "19",
			"-rc-lookahead", "32",
			"-spatial-aq", "1",
			"-temporal-aq", "1",
			"-aq-strength", "11",
			"-profile:v", "high",
		}
	case EncoderApple:
		// 能力参数（profile/coder/spatial_aq/allow_sw）与 hwaccel.go probeVideoToolbox 一致；
		// -q:v 只是质量档（非能力开关），此处按本项目需求取 95（探测用 70，不影响探测有效性）。
		return []string{
			"-c:v", "h264_videotoolbox",
			"-profile:v", "high",
			"-coder", "cabac",
			"-q:v", "95",
			"-spatial_aq", "1",
			"-allow_sw", "1",
		}
	case EncoderIntel:
		// 与 hwaccel.go probeQsv 一致：刻意只用高兼容的最小参数。
		// 实测 look_ahead / extbrc / mbbrc / rdo / bf / preset=veryslow 一起下发时，
		// 会在部分 Intel 核显驱动上让编码器初始化失败（即便 QSV 本身可用），
		// 导致探测误判并回退 CPU，故移除这些激进调优选项。
		// ⚠️ -global_quality 必须带 :v 后缀：它是 AVCodecContext 通用选项，不加后缀会同时
		// 套到音频流上，使 libopus 进入 quality-based 模式并与 -b:a 冲突，直接报
		// “Quality-based encoding not supported” 打开编码器失败。
		return []string{
			"-c:v", "h264_qsv",
			"-global_quality:v", "18",
			"-profile:v", "high",
		}
	case EncoderAMD:
		// 与 hwaccel.go probeAmf 一致
		return []string{
			"-c:v", "h264_amf",
			"-usage", "high_quality",
			"-quality", "quality",
			"-qp_i", "18",
			"-qp_p", "20",
			"-qp_b", "22",
			"-vbaq", "true",
			"-preanalysis", "true",
			"-profile", "high",
		}
	case EncoderQualcomm:
		// 与 hwaccel.go probeMediaFoundation 一致。
		// Windows 平台（含 ARM64 高通 Surface）通过 Media Foundation 走 h264_mf 硬件编码。
		// 参数取向 1080p60 中上等画质，并针对纯色区域 banding/暗影做了优化：
		//   - hw_encoding 1 强制枚举硬件 MFT：不加时 h264_mf 可能静默落到微软 CPU 软编码器，
		//     表现为 CPU 70%+ / GPU 1%（编码其实在 CPU 跑）；
		//   - level 4.2 支撑 1080p@60；
		//   - VBR 目标 10M / 峰值 14M（相比初版提高，缓解纯色区域量化色带）；
		//   - CABAC 熵编码 + 4 参考帧，同等码率下质量更优、纯色伪影更少；
		//   - g=120 约 2 秒一个关键帧，兼顾随机访问与压缩率。
		// 必须配合输入端 -hwaccel d3d11va -hwaccel_output_format d3d11 提供显存帧，
		// 且此处不能写 -pix_fmt yuv420p（强制下载回 CPU 会导致硬编码器打开失败）。
		// 注意：h264_mf 无 x264 的 aq-mode/psy-rd 等高级调优，纯色 banding 主要靠码率兜底。
		// ⚠️ -profile:v 必须传数字 profile_idc（100=High），传 "high" 字符串 h264_mf 无法解析。
		return []string{
			"-c:v", "h264_mf",
			"-hw_encoding", "1",
			"-profile:v", "100",
			"-level", "4.2",
			"-b:v", "10M",
			"-maxrate", "14M",
			"-bufsize", "20M",
			"-g", "120",
			"-bf", "3",
			"-refs", "4",
			"-coder", "1",
		}
	case EncoderX264:
		// CPU 快速档（FASTCUT）：平衡速度与质量
		return []string{
			"-c:v", "libx264",
			"-preset", "fast",
			"-crf", "20",
			"-profile:v", "high",
			"-pix_fmt", "yuv420p",
		}
	case EncoderX265:
		// CPU 高质量档：与 segment.go 的 libx265 分支一致
		return []string{
			"-c:v", "libx265",
			"-tag:v", "hvc1",
			"-preset", "slow",
			"-crf", "24",
			"-pix_fmt", "yuv420p",
			"-x265-params", "aq-mode=3:aq-strength=1.2:psy-rd=2.0:psy-rdoq=2.0:rdoq-level=1",
		}
	case EncoderCopy:
		return []string{"-c:v", "copy"}
	case EncoderNone:
		// 音频-only：不产生任何视频参数（如需从含视频的源抽取音频，可自行在 ExtraArgs 加 "-vn"）
		return nil
	default:
		// EncoderAuto 不应走到这里（调用前已由 resolveEncoder 解析）
		return nil
	}
}

// fastVideoEncoderArgs 返回某个编码器“速度优先”的视频参数，仅在 Job.Fastest=true 时使用。
// 与 videoEncoderArgs 的区别：取向是“越快越好”，统一选最快 preset、去掉一切高级调优/分析
// 选项（look_ahead / preanalysis / AQ / 多 B 帧等），画质与体积不做要求。
// 只对 mirror 这类“打点用镜像”生效，不影响追求画质的其它子命令。
func fastVideoEncoderArgs(e Encoder) []string {
	switch e {
	case EncoderNvidia:
		// NVENC：p1=最快（旧 ffmpeg 写作 fastest，新版为 p1），硬件编码本身已很快。
		return []string{
			"-c:v", "h264_nvenc",
			"-preset", "p1",
			"-profile:v", "baseline",
		}
	case EncoderIntel:
		// QSV：veryfast 档，仅保留最基础选项，避免高级参数拖慢或初始化失败。
		return []string{
			"-c:v", "h264_qsv",
			"-preset", "veryfast",
			"-profile:v", "baseline",
		}
	case EncoderAMD:
		// AMF：低延迟优先、关闭 preanalysis/vbaq 等耗时分析。
		return []string{
			"-c:v", "h264_amf",
			"-usage", "lowlatency",
			"-profile", "baseline",
		}
	case EncoderApple:
		// VideoToolbox：q:v 数值越低越快（画质越低），取较低档；allow_sw 兜底。
		return []string{
			"-c:v", "h264_videotoolbox",
			"-profile:v", "baseline",
			"-q:v", "30",
			"-allow_sw", "1",
		}
	case EncoderQualcomm:
		// h264_mf：硬编码本身够快，去掉多参考帧/B帧等额外开销。
		return []string{
			"-c:v", "h264_mf",
			"-hw_encoding", "1",
			"-profile:v", "66",
			"-bf", "0",
		}
	case EncoderX264:
		// CPU 快速档：ultrafast 是 libx264 最快 preset。
		return []string{
			"-c:v", "libx264",
			"-preset", "ultrafast",
			"-profile:v", "baseline",
			"-pix_fmt", "yuv420p",
		}
	case EncoderX265:
		// 极速档下 CPU 回退也走 x264（比 x265 快），由 Run 保证；此处兜底给最快 x265。
		return []string{
			"-c:v", "libx265",
			"-preset", "ultrafast",
			"-pix_fmt", "yuv420p",
		}
	default:
		// EncoderCopy / EncoderNone / 未解析：交由 ffmpeg 默认。
		return videoEncoderArgs(e)
	}
}

// inputHwaccelArgs 返回输入端（-i 之前）的硬件解码参数；仅对支持且开启 HwDecode 的编码器生效。
func inputHwaccelArgs(e Encoder) []string {
	switch e {
	case EncoderNvidia:
		return []string{"-hwaccel", "cuda"}
	case EncoderIntel:
		// 默认走软解、不追加 QSV 硬解参数。原因：-hwaccel qsv -hwaccel_output_format qsv
		// 这条全硬链路在精确切割 / 多音轨 / filter_complex 等场景兼容性差，实测会在
		// “QSV 编码本身可用”的机器上导致整条命令失败。软解 + h264_qsv 硬编已验证稳定，
		// 也与 probeQsv 探测的配置保持一致。如需硬解可在 Job.PreInputArgs 手动追加。
		return nil
	case EncoderQualcomm:
		// Windows ARM64 高通平台：Media Foundation 硬件编码器必须吃 d3d11 显存帧。
		// -hwaccel_output_format d3d11 让解码帧留在 GPU（NV12），编码端零拷贝；
		// 遇到 d3d11va 解不了的编码，ffmpeg 会自动回退软解并插入 hwupload，链路不中断。
		// 注意：此分支绝不能再指定 -pix_fmt yuv420p，否则强制把帧下载回 CPU，
		// 硬编码器拒收并报 “Error reinitializing filters”。
		return []string{"-hwaccel", "d3d11va", "-hwaccel_output_format", "d3d11"}
	case EncoderApple:
		// VideoToolbox 编码器本身不通过 -hwaccel 暴露硬解接口，ffmpeg 会在需要时自动使用 VT 解码，
		// 故此处不追加 -hwaccel 参数。
		return nil
	case EncoderAMD:
		// AMF 编码器无配套的 -hwaccel 解码路径，硬解需走 D3D11VA/VAAPI 但跨平台兼容性差，
		// 故默认走软解以保证稳定性；如需硬解可在 Job.PreInputArgs 手动追加。
		return nil
	default:
		return nil
	}
}

// Audio 描述音频流的编码方式。零值（Codec 为空）表示不指定，交给 ffmpeg 按容器默认处理。
type Audio struct {
	Codec       string   // "libopus"（唯一编码）/ "copy"（不重编码）/ "none"（丢弃音频）
	Bitrate     string   // -b:a，如 "192k"；空则不加
	Channels    int      // -ac；0 则不加
	SampleRate  int      // -ar；0 则不加
	Application string   // libopus 专用 -application（audio / voip）
	Extra       []string // 追加的其它音频参数
}

// 音频预设：本项目只使用高质量 Opus 一种编码，AAC / MP3 等其它编码一律不考虑。
// AudioOpus 是唯一的音频编码预设；AudioCopy / AudioNone 并非编码，而是流级别操作
// （直接复制音频流 / 丢弃音频），仅在确有需要的特例下使用。
func AudioOpus() Audio {
	// 高质量 Opus：192k VBR + audio 应用模式 + 最高压缩档，取向高保真而非编码速度。
	// ⚠️ -compression_level 必须带 :a 后缀：它也是通用选项，不加后缀会漏到视频编码器上
	// （如 h264_qsv 的合法范围是 0-7，会告警 “Invalid compression level”）。
	return Audio{
		Codec:       "libopus",
		Bitrate:     "192k",
		Application: "audio",
		Extra:       []string{"-vbr", "on", "-compression_level:a", "10"},
	}
}
func AudioCopy() Audio { return Audio{Codec: "copy"} }
func AudioNone() Audio { return Audio{Codec: "none"} }

// mp4SubtitleCodecs 是 MP4/M4V 容器可封装的字幕编码白名单。
// PGS（hdmv_pgs_subtitle）、DVD（dvd_subtitle）等图形字幕无法封装进 MP4；
// VobSub 实为 dvd_subtitle、蓝光文本字幕实为 hdmv_text_subtitle，均已覆盖。
var mp4SubtitleCodecs = map[string]bool{
	"mov_text": true,
	"subrip":   true,
	"srt":      true,
	"ass":      true,
	"ssa":      true,
	"webvtt":   true,
	"text":     true,
	"ttml":     true,
}

// subtitleCodecForOutput 按输出容器返回应指定的字幕编码（-c:s）；
// 其它容器返回 nil，交给 ffmpeg 按容器默认选择。
func subtitleCodecForOutput(output string) []string {
	switch strings.ToLower(filepath.Ext(output)) {
	case ".mp4", ".m4v":
		return []string{"-c:s", "mov_text"}
	case ".mkv":
		return []string{"-c:s", "srt"}
	default:
		return nil
	}
}

// probeStreams 用 ffprobe 列出输入文件的所有流，每行返回 "类型,编码名"（如 "subtitle,hdmv_pgs_subtitle"）。
func probeStreams(input string) ([][]string, error) {
	cmd := exec.Command("ffprobe", "-v", "error",
		"-show_entries", "stream=codec_type,codec_name",
		"-of", "csv=p=0", input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe 探测 %s 失败: %w (%s)", input, err, strings.TrimSpace(stderr.String()))
	}
	return csv.NewReader(bytes.NewReader(bytes.TrimSpace(stdout.Bytes()))).ReadAll()
}

// IncompatibleSubtitles 返回输入文件中无法封装进目标容器（按输出扩展名判定）的字幕编码列表。
// ffprobe 探测失败时返回错误，由调用方决定跳过还是放行，避免在信息不全的情况下误转。
func IncompatibleSubtitles(input, output string) ([]string, error) {
	if !strings.EqualFold(filepath.Ext(output), ".mp4") && !strings.EqualFold(filepath.Ext(output), ".m4v") {
		return nil, nil // 目前只有 MP4 系容器存在装不进的字幕，其它容器一律放行
	}
	streams, err := probeStreams(input)
	if err != nil {
		return nil, err
	}
	var bad []string
	for _, s := range streams {
		if len(s) >= 2 && s[0] == "subtitle" && !mp4SubtitleCodecs[s[1]] {
			bad = append(bad, s[1])
		}
	}
	return bad, nil
}

// audioStreamCount 返回输入文件的音频流数量；探测失败返回 0（按单音频流保守处理）。
func audioStreamCount(input string) int {
	streams, err := probeStreams(input)
	if err != nil {
		log.Printf("[ffmpeg] 探测音频流数量失败（%v），按单音频流处理", err)
		return 0
	}
	n := 0
	for _, s := range streams {
		if len(s) >= 1 && s[0] == "audio" {
			n++
		}
	}
	return n
}

// allStreamMaps 返回“保留全部流”的映射参数：视频/音频/字幕/数据流逐类映射，
// "?" 保证输入没有该类流时不报错；附件流（字体等）仅 MKV 目标容器支持。
func allStreamMaps(output string) []string {
	maps := []string{"-map", "0:v?", "-map", "0:a?", "-map", "0:s?", "-map", "0:d?"}
	if strings.EqualFold(filepath.Ext(output), ".mkv") {
		maps = append(maps, "-map", "0:t?")
	}
	return maps
}

// audioFilterComplexArgs 把同一个音频滤镜应用到所有音频流，并返回 filter_complex 及对应的映射参数。
// 背景：-af 只能作用于单条输出音频流，一旦输入有多条音轨且全部映射，ffmpeg 会直接报错
// “Filtergraph ... was specified through the -vf/-af/-df option for output stream 0:a:N”，
// 因此多音轨场景必须改用 filter_complex 给每条音轨分别挂滤镜。
func audioFilterComplexArgs(audioFilter string, n int) []string {
	var parts []string
	args := []string{"-filter_complex"}
	var graph strings.Builder
	for i := 0; i < n; i++ {
		graph.WriteString(fmt.Sprintf("[0:a:%d]%s[a%d];", i, audioFilter, i))
		parts = append(parts, "-map", fmt.Sprintf("[a%d]", i))
	}
	args = append(args, strings.TrimSuffix(graph.String(), ";"))
	return append(args, parts...)
}

// args 生成音频相关参数。
func (a Audio) args() []string {
	switch a.Codec {
	case "":
		return nil
	case "none":
		return []string{"-an"}
	case "copy":
		return []string{"-c:a", "copy"}
	}
	args := []string{"-c:a", a.Codec}
	if a.Bitrate != "" {
		args = append(args, "-b:a", a.Bitrate)
	}
	if a.Channels > 0 {
		args = append(args, "-ac", strconv.Itoa(a.Channels))
	}
	if a.SampleRate > 0 {
		args = append(args, "-ar", strconv.Itoa(a.SampleRate))
	}
	if a.Application != "" {
		args = append(args, "-application", a.Application)
	}
	return append(args, a.Extra...)
}

// Job 是一次 ffmpeg 任务的完整描述，也是供其它函数复用的 “model”。
// 建议用 NewJob / NewCutJob 构造以拿到默认值，再按需覆盖字段。
type Job struct {
	Input  string
	Output string

	// Encoder 为 EncoderAuto（默认）时由 SelectEncoder 按硬件自动决定；也可强制指定。
	Encoder Encoder

	// 切割范围（输出端精确切割，等价旧代码把 -ss/-to 放在 -i 之后）。留空表示该端不限制。
	Start string // -ss
	End   string // -to

	// 音频编码规格，NewJob 默认高质量 Opus（见 AudioOpus）。
	Audio Audio

	// 时间戳/同步相关开关（精确切割时建议全开，见 NewCutJob / EnableTimestampFixes）。
	StripMetadata   bool   // -map_metadata -1
	PassthroughFps  bool   // -fps_mode passthrough（等价旧 -vsync 0；ffmpeg 8+ 已移除 -vsync）
	CopyTimestamps  bool   // -copyts
	AvoidNegativeTs bool   // -avoid_negative_ts make_zero
	GenPts          bool   // -fflags +genpts+igndts
	AudioFilter     string // -af，如 AudioFilterSync

	// HwDecode 为 true 时，对支持的编码器（NVIDIA / Intel）追加输入端硬件解码参数。
	// 精确切割 + QSV 硬解可能有兼容性问题，需要时可置 false 走软解。
	HwDecode bool

	Progress  bool // -progress pipe:1，把机器可读进度输出到 stdout
	Overwrite bool // -y，覆盖已存在的输出文件

	// Fastest 为“极速档”：只追求转换速度，不保证画质、也不需要音频，
	// 专供 mirror 这类“打点用镜像”场景。开启后：
	//   - 视频改用 fastVideoEncoderArgs 的速度优先参数（最快 preset）；
	//   - 音频丢弃（-an，见 EnableFastest 里设置的 AudioNone）；
	//   - 只映射视频流（-map 0:v:0），字幕/数据流/音频全部不处理，进一步提速；
	//   - 硬件失败回退 CPU 时也用最快的 libx264 ultrafast，而非高质量 libx265。
	// 默认 false，其它子命令不受任何影响。
	Fastest bool

	// MapAllStreams 为 true（NewJob 默认）时自动添加流映射，把输入的所有视频/音频/字幕/数据流
	// 全部保留到输出（不加时退回 ffmpeg 默认的“每类挑一条”行为，字幕会被整体丢弃），
	// 并按输出容器自动指定字幕编码（MP4→mov_text、MKV→srt）。
	// 若 PostInputArgs 里已手写 -map，则不再自动添加映射。
	// EncoderNone（音频-only 任务，如 wav→mp3）时本开关自动失效，避免把视频流带进纯音频输出。
	MapAllStreams bool

	// 通用扩展点，覆盖 model 未直接建模的特殊参数：
	PreInputArgs  []string // 放在 -i 之前（输入选项，如额外的 -f / -re）
	PostInputArgs []string // 放在 -i 之后（如第二路输入 -stream_loop -1 -i x、-map 等）
	ExtraArgs     []string // 放在输出文件之前的其它输出选项（如 -shortest / -max_muxing_queue_size）

	HideBanner bool // -hide_banner，NewJob 默认 true
}

// NewJob 返回默认的全量重编码任务：自动硬件编码 + 高质量 Opus 音频 + 覆盖输出。
func NewJob(input, output string) *Job {
	return &Job{
		Input:         input,
		Output:        output,
		Encoder:       EncoderAuto,
		Audio:         AudioOpus(),
		HwDecode:      true,
		Overwrite:     true,
		HideBanner:    true,
		MapAllStreams: true,
	}
}

// NewCutJob 返回精确切割任务：在 NewJob 基础上设置切割区间并开启时间戳/同步修复，
// 与既有 CutBySegment 的行为对齐。
func NewCutJob(input, output, start, end string) *Job {
	j := NewJob(input, output)
	j.Start = start
	j.End = end
	j.EnableTimestampFixes()
	return j
}

// EnableTimestampFixes 打开精确切割常用的一组时间戳/同步开关，并设置 A/V 重同步滤镜。
// 返回自身以支持链式调用。
func (j *Job) EnableTimestampFixes() *Job {
	j.StripMetadata = true
	j.PassthroughFps = true
	j.CopyTimestamps = true
	j.AvoidNegativeTs = true
	j.GenPts = true
	if j.AudioFilter == "" {
		j.AudioFilter = AudioFilterSync
	}
	return j
}

// EnableFastest 打开“极速档”，专供 mirror 等只看速度、不需要画质和音频的场景。
// 它只改本 Job，不触碰任何默认参数函数，因此不影响其它子命令。具体动作：
//   - Fastest=true：视频走 fastVideoEncoderArgs；
//   - Audio=AudioNone()：产出 -an，丢弃音频；
//   - 手写 PostInputArgs 的 -map 0:v:0：只保留视频流（Args 检测到 -map 会自动让位，
//     不再做全流映射 / 字幕编码 / 多音轨滤镜等额外工作）；
//   - 关闭硬解：极速档软解 + 硬编最稳，避免硬解链路拖慢或失败。
//
// 返回自身以支持链式调用。
func (j *Job) EnableFastest() *Job {
	j.Fastest = true
	j.Audio = AudioNone()
	// 仅映射第一条视频流；hasMapFlag 识别后会跳过自动全流映射与字幕处理。
	j.PostInputArgs = append(j.PostInputArgs, "-map", "0:v:0")
	j.HwDecode = false
	return j
}

// ResolvedEncoder 返回本任务实际会使用的编码器（把 Auto 解析掉），便于日志/判断。
func (j *Job) ResolvedEncoder() Encoder { return resolveEncoder(j.Encoder) }

// Args 生成完整的 ffmpeg 参数列表（不含程序名 "ffmpeg"）。
// 单独暴露以便在不执行的情况下检查/验证将要运行的命令。
func (j *Job) Args() []string {
	enc := resolveEncoder(j.Encoder)

	var args []string
	if j.HideBanner {
		args = append(args, "-hide_banner")
	}
	if j.Overwrite {
		args = append(args, "-y")
	}

	// 输入端选项
	args = append(args, j.PreInputArgs...)
	if j.HwDecode {
		args = append(args, inputHwaccelArgs(enc)...)
	}
	args = append(args, "-i", j.Input)
	args = append(args, j.PostInputArgs...)

	// 切割区间（输出端精确切割）
	if j.Start != "" {
		args = append(args, "-ss", j.Start)
	}
	if j.End != "" {
		args = append(args, "-to", j.End)
	}

	// 流映射：保留输入的所有视频/音频/字幕/数据流（PostInputArgs 已手写 -map 时让位）。
	// 多音轨 + 音频滤镜不能共存于 -af（见 audioFilterComplexArgs），改走 filter_complex 逐轨挂滤镜。
	mapAll := j.MapAllStreams && enc != EncoderNone && !hasMapFlag(j.PostInputArgs)
	useFilterComplex := false
	nAudio := 0
	if mapAll && j.AudioFilter != "" {
		nAudio = audioStreamCount(j.Input)
		useFilterComplex = nAudio > 1
	}
	if mapAll && !useFilterComplex {
		args = append(args, allStreamMaps(j.Output)...)
	}

	// 视频编码参数：极速档走速度优先参数，否则走默认的画质参数。
	if j.Fastest {
		args = append(args, fastVideoEncoderArgs(enc)...)
	} else {
		args = append(args, videoEncoderArgs(enc)...)
	}

	// 音频编码参数
	args = append(args, j.Audio.args()...)

	// 字幕编码：按输出容器指定，保证映射过来的字幕流能真正封装进去
	if mapAll {
		args = append(args, subtitleCodecForOutput(j.Output)...)
	}

	// 时间戳/同步相关（顺序与既有 CutBySegment 保持一致）
	if j.StripMetadata {
		args = append(args, "-map_metadata", "-1")
	}
	if j.PassthroughFps {
		args = append(args, "-fps_mode", "passthrough")
	}
	if j.AvoidNegativeTs {
		args = append(args, "-avoid_negative_ts", "make_zero")
	}
	if j.GenPts {
		args = append(args, "-fflags", "+genpts+igndts")
	}
	if useFilterComplex {
		// 多音轨：每条音轨分别挂滤镜，音频映射由 filter_complex 分支给出
		args = append(args, audioFilterComplexArgs(j.AudioFilter, nAudio)...)
		args = append(args, "-map", "0:v?", "-map", "0:s?", "-map", "0:d?")
		if strings.EqualFold(filepath.Ext(j.Output), ".mkv") {
			args = append(args, "-map", "0:t?")
		}
	} else if j.AudioFilter != "" {
		args = append(args, "-af", j.AudioFilter)
	}
	if j.CopyTimestamps {
		args = append(args, "-copyts")
	}

	// 其它输出选项
	args = append(args, j.ExtraArgs...)

	// 进度输出
	if j.Progress {
		args = append(args, "-progress", "pipe:1")
	}

	// 输出文件放最后
	return append(args, j.Output)
}

// hasMapFlag 判断扩展参数里是否已手写 -map（含 -map:v 等带后缀形式），有则不再自动加流映射。
func hasMapFlag(args []string) bool {
	for _, a := range args {
		if a == "-map" || strings.HasPrefix(a, "-map:") {
			return true
		}
	}
	return false
}

// Command 构建 *exec.Cmd（尚未执行）。
func (j *Job) Command() *exec.Cmd { return exec.Command("ffmpeg", j.Args()...) }

// String 返回将要执行的完整命令字符串，便于日志/调试。
func (j *Job) String() string { return j.Command().String() }

// Run 构建并执行命令，复用 util.Exec 的日志与错误处理。
// 若当前编码器是硬件编码器（NVIDIA/Apple/Intel/AMD/Qualcomm）且执行失败，
// 自动回退到 CPU 编码（EncoderX265）重试一次，避免整批任务因个别硬件兼容性问题中断。
// 强制指定的软件编码器（X264/X265/Copy/None）不触发回退。
func (j *Job) Run() error {
	enc := j.ResolvedEncoder()
	log.Printf("[ffmpeg] 选用视频编码器: %s", enc)
	if err := Exec(j.Command()); err != nil {
		if isHardwareEncoder(enc) {
			backup := *j
			// 极速档回退到最快的 libx264（Fastest 仍为 true，会走 ultrafast）；
			// 正常画质档回退到高质量 libx265。
			if j.Fastest {
				log.Printf("[ffmpeg] 硬件编码器 %s 失败，极速档回退到 CPU (libx264 ultrafast) 重试；失败原因: %v", enc, err)
				backup.Encoder = EncoderX264
			} else {
				log.Printf("[ffmpeg] 硬件编码器 %s 失败，回退到 CPU (libx265) 重试；失败原因: %v", enc, err)
				backup.Encoder = EncoderX265
			}
			if err2 := Exec(backup.Command()); err2 != nil {
				return fmt.Errorf("硬件编码 %s 失败(%v)，CPU 回退也失败: %w", enc, err, err2)
			}
			log.Printf("[ffmpeg] CPU 回退编码成功")
			return nil
		}
		return err
	}
	return nil
}

// isHardwareEncoder 判断编码器是否为硬件编码器（用于失败时决定是否回退到 CPU）。
func isHardwareEncoder(e Encoder) bool {
	switch e {
	case EncoderNvidia, EncoderApple, EncoderIntel, EncoderAMD, EncoderQualcomm:
		return true
	default:
		return false
	}
}

// Convert 是最顶层的便捷函数：只给输入和输出文件名，视频编码参数由 hwaccel.go 自动判定，
// 音频默认高质量 Opus，全流保留，覆盖输出。
// 转换前预检字幕兼容性：含目标容器装不下的字幕（如 MKV 的 PGS→MP4）时直接跳过——
// 只返回错误、不动输入文件，批量调用方打印错误后继续下一个即可。
func Convert(input, output string) error {
	if bad, err := IncompatibleSubtitles(input, output); err != nil {
		return fmt.Errorf("字幕预检失败 %s: %w", input, err)
	} else if len(bad) > 0 {
		return fmt.Errorf("跳过 %s：含 %s 容器无法封装的字幕流 %v", input, filepath.Ext(output), bad)
	}
	return NewJob(input, output).Run()
}

// Cut 顶层便捷函数：按 [start, end] 精确切割并启用时间戳修复。
// start/end 为 ffmpeg 时间格式（如 "00:00:01.000"），传空字符串表示该端不限制。
// 等价于 NewCutJob(input, output, start, end).Run()。
func Cut(input, output, start, end string) error {
	return NewCutJob(input, output, start, end).Run()
}
