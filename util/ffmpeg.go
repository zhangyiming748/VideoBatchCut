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
// 对应的 probe*（probeNvenc / probeQsv / probeAmf / probeVideoToolbox）保持一致——
// 探测用的就是这套参数，只有两者一致，“探测通过” 才等价于 “真实编码能跑通”。
// 改动其中一处，另一处必须同步。
//
// 流保留保证：默认（MapAllStreams）把输入的所有视频/音频/字幕/数据流全部映射到输出，
// 并按输出容器自动指定字幕编码（MP4→mov_text、MKV→srt）。含 PGS / DVD 等图形字幕的
// 文件无法封装进 MP4，转换前必须先经 HasIncompatibleSubtitles 预检并跳过该文件
// （只打印错误、不中断批处理），避免转换中途失败或悄悄丢字幕。
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
	EncoderAuto   Encoder = iota // 交给 SelectEncoder 按硬件自动选择（Job.Encoder 的默认值）
	EncoderNvidia                // h264_nvenc（NVIDIA NVENC）
	EncoderApple                 // h264_videotoolbox（Apple Silicon VideoToolbox）
	EncoderIntel                 // h264_qsv（Intel Quick Sync，仅 Windows）
	EncoderAMD                   // h264_amf（AMD，仅 Windows）
	EncoderX264                  // libx264（CPU，FASTCUT 快速档）
	EncoderX265                  // libx265（CPU，高质量档）
	EncoderCopy                  // -c:v copy，直接复制视频流不重编码
	EncoderNone                  // 不加任何视频编码参数（音频-only 任务，如 wav→mp3）
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
// 优先级：NVIDIA > Apple > Intel > AMD > CPU（FASTCUT=yes 用 libx264，否则 libx265）。
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
		// 与 hwaccel.go probeQsv 一致。
		// ⚠️ -global_quality 必须带 :v 后缀：它是 AVCodecContext 通用选项，不加后缀会同时
		// 套到音频流上，使 libopus 进入 quality-based 模式并与 -b:a 冲突，直接报
		// “Quality-based encoding not supported” 打开编码器失败。
		return []string{
			"-c:v", "h264_qsv",
			"-preset", "veryslow",
			"-global_quality:v", "18",
			"-look_ahead", "1",
			"-look_ahead_depth", "40",
			"-extbrc", "1",
			"-mbbrc", "1",
			"-rdo", "1",
			"-adaptive_i", "1",
			"-adaptive_b", "1",
			"-bf", "4",
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

// inputHwaccelArgs 返回输入端（-i 之前）的硬件解码参数；仅对支持且开启 HwDecode 的编码器生效。
func inputHwaccelArgs(e Encoder) []string {
	switch e {
	case EncoderNvidia:
		return []string{"-hwaccel", "cuda"}
	case EncoderIntel:
		// QSV 硬解需同时把输出指定为 qsv surface，供 h264_qsv 编码零拷贝消费
		return []string{"-hwaccel", "qsv", "-hwaccel_output_format", "qsv"}
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

// HasIncompatibleSubtitles 供批量转换入口做前置预检：文件含目标容器装不下的字幕（如 PGS→MP4）时返回 true。
// 预检自身失败（如 ffprobe 缺失）按保守策略同样返回 true 并打印错误，宁可跳过也不误转。
func HasIncompatibleSubtitles(input, output string) bool {
	bad, err := IncompatibleSubtitles(input, output)
	if err != nil {
		log.Printf("[ffmpeg] 字幕预检失败，跳过 %s: %v", input, err)
		return true
	}
	if len(bad) > 0 {
		log.Printf("[ffmpeg] %s 含 %s 容器无法封装的字幕流 %v，跳过该文件（保留原文件）",
			input, filepath.Ext(output), bad)
		return true
	}
	return false
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

	// 视频编码参数（按硬件自动决定的单一事实来源）
	args = append(args, videoEncoderArgs(enc)...)

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
func (j *Job) Run() error {
	log.Printf("[ffmpeg] 选用视频编码器: %s", j.ResolvedEncoder())
	return Exec(j.Command())
}

// Convert 是最顶层的便捷函数：只给输入和输出文件名，视频编码参数由 hwaccel.go 自动判定，
// 音频默认高质量 Opus，全流保留，覆盖输出。
// 转换前预检字幕兼容性：含目标容器装不下的字幕（如 MKV 的 PGS→MP4）时直接跳过——
// 只返回错误、不动输入文件，批量调用方打印错误后继续下一个即可。
func Convert(input, output string) error {
	if HasIncompatibleSubtitles(input, output) {
		return fmt.Errorf("跳过 %s：含 %s 容器无法封装的字幕流", input, filepath.Ext(output))
	}
	return NewJob(input, output).Run()
}

// Cut 顶层便捷函数：按 [start, end] 精确切割并启用时间戳修复。
// start/end 为 ffmpeg 时间格式（如 "00:00:01.000"），传空字符串表示该端不限制。
// 等价于 NewCutJob(input, output, start, end).Run()。
func Cut(input, output, start, end string) error {
	return NewCutJob(input, output, start, end).Run()
}
