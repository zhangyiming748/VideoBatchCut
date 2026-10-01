// 提供“硬件编码器是否真的可用”的探测工具函数，供全项目公用。
//
// 设计原则：不看“机器上有没有这块硬件”，而是让 ffmpeg 真跑一次最小编码，
// 退出码为 0 才认为该硬件编码器可用。原因：
//   - nvidia-smi / wmic / lspci 只能说明“有卡”，不代表 ffmpeg 真能调用：
//     老卡驱动 EOL、ffmpeg 未编译对应编码器、或编码参数不被支持，都会在运行期报错，
//     仅凭“设备存在”放行会导致整批任务跑挂；
//   - 真实编码探测能干净地挡掉这些情况，不可用时自动回退到软件编码分支。
//
// 这些函数会被逐文件调用，而 GPU 能力在一次运行中不会变，故用 probeCache
// 保证每种硬件（以及 ffmpeg 是否存在）整批只探测一次，避免重复拖慢。
package util

import (
	"context"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

// probeTimeout 单次硬件编码探测的最长等待时间。
// 正常探测（256x256 1 秒合成源）通常 <1 秒，给 15 秒余量覆盖机器卡顿场景；
// 超时直接视为该硬件不可用，避免驱动异常时永久卡死整批任务。
const probeTimeout = 15 * time.Second

// probeCache 缓存一次“真实编码探测”的结果，保证整批处理中每种探测只真跑一次。
type probeCache struct {
	once sync.Once
	ok   bool
}

func (c *probeCache) get(probe func() bool) bool {
	c.once.Do(func() { c.ok = probe() })
	return c.ok
}

var (
	ffmpegProbe          probeCache
	nvencProbe           probeCache
	qsvProbe             probeCache
	amfProbe             probeCache
	videotoolboxProbe    probeCache
	mediaFoundationProbe probeCache
)

// hasFFmpeg 探测 ffmpeg 二进制是否存在。缺失时直接判定所有硬件编码器不可用，
// 省去无谓地拉起一个注定失败的 ffmpeg 探测进程。
func hasFFmpeg() bool {
	return ffmpegProbe.get(func() bool {
		_, err := exec.LookPath("ffmpeg")
		return err == nil
	})
}

// HasNvidia 判断 ffmpeg 是否真的能用 h264_nvenc 编码（而非仅检测有无 N 卡）。
func HasNvidia() bool {
	// 平台门槛：NVENC 仅在 amd64 的 Windows/Linux 上可用；
	// macOS 统一走 VideoToolbox，ARM64（含 Windows ARM64）无 NVENC，直接短路省去无谓探测。
	if runtime.GOOS == "darwin" || runtime.GOARCH != "amd64" {
		return false
	}
	if !hasFFmpeg() {
		return false
	}
	return nvencProbe.get(probeNvenc)
}

// HasIntel 判断 ffmpeg 是否真的能用 h264_qsv 编码。
func HasIntel() bool {
	// 平台门槛：QSV 仅 Windows amd64。
	// 本分支参数按 Windows 设计（自动选默认适配器，不带 -init_hw_device）；
	// Linux QSV 需显式 -init_hw_device 指向 render 节点，与本分支参数不匹配；
	// ARM64（含 Windows ARM64）无 Intel iGPU，直接短路。
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return false
	}
	if !hasFFmpeg() {
		return false
	}
	return qsvProbe.get(probeQsv)
}

// HasAMD 判断 ffmpeg 是否真的能用 h264_amf 编码。
func HasAMD() bool {
	// 平台门槛：AMF 仅 Windows amd64 提供；
	// macOS 统一走 VideoToolbox，ARM64（含 Windows ARM64）无 AMF，直接短路省去无谓探测。
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return false
	}
	if !hasFFmpeg() {
		return false
	}
	return amfProbe.get(probeAmf)
}

// HasAppleSilicon 判断是否为 Apple Silicon 且 ffmpeg 真的能用 h264_videotoolbox 编码。
func HasAppleSilicon() bool {
	// 平台门槛：仅 Apple Silicon（macOS + arm64）走 VideoToolbox 分支
	// 注意：若用户误在 Apple Silicon 上运行 amd64 二进制（Rosetta 转译），GOARCH 会是 amd64 而落入 CPU 分支；
	// release 已提供 darwin/arm64 构建，正常下载 arm64 版本即可命中此分支
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		return false
	}
	if !hasFFmpeg() {
		return false
	}
	return videotoolboxProbe.get(probeVideoToolbox)
}

// HasQualcomm 判断是否为 Windows ARM64 高通平台（Surface 10/11 等）且 ffmpeg 真的能用 h264_mf 编码。
// 高通 Adreno GPU 在 Windows ARM64 上通过 Media Foundation 暴露硬件编码器（h264_mf），
// 这也是该平台 ffmpeg 可用的硬件编码路径（无 NVENC/QSV/AMF）。
func HasQualcomm() bool {
	// 平台门槛：所有 Windows 都允许进入真实探测（不再强制 GOARCH==arm64）。
	// 原因：Windows on ARM（Surface 10/11 等高通机型）上用户安装的 ffmpeg 常为 x64 构建、
	// 靠 x64 模拟运行，此时 Go 二进制的 GOARCH 是 amd64，但 h264_mf 经 Media Foundation 的 COM
	// 接口仍能调到高通硬件编码器；若用 GOARCH 把 amd64 一刀切掉，模拟运行场景会被误杀。
	// 安全性由后面的“真实编码探测”兜底——探测通过才认为可用，无需担心误判。
	// 优先级低于 NVIDIA/Intel/AMD：普通 x64 PC 在前面三个分支就会命中，走不到这里。
	if runtime.GOOS != "windows" {
		return false
	}
	if !hasFFmpeg() {
		return false
	}
	return mediaFoundationProbe.get(probeMediaFoundation)
}

// runProbe 带超时地执行一次 ffmpeg 探测命令，退出码为 0 才返回 true。
// 超时（驱动异常导致 ffmpeg 卡死）同样返回 false，避免拖垮整批任务。
func runProbe(cmd *exec.Cmd) bool {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	// Stdout/Stderr 均为 nil 时 Go 会把 ffmpeg 输出接到 /dev/null，探测过程不污染批处理日志
	if err := cmd.Start(); err != nil {
		return false
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-ctx.Done():
		// 超时：杀掉进程并返回 false
		_ = cmd.Process.Kill()
		return false
	case err := <-done:
		return err == nil
	}
}

// probeNvenc 用一段合成源跑一次真实的 h264_nvenc 最小编码，退出码为 0 才认为 NVENC 可用。
//
// 为什么不能只看 nvidia-smi 是否存在 + ffmpeg 是否编译了 h264_nvenc：
// 这两个条件在老卡（如 Kepler GT 710）上同样成立，但真正编码时会因驱动已 EOL、
// 或不支持本分支用到的 Pascal+ 参数（spatial-aq / temporal-aq / rc-lookahead）而失败，导致整批任务跑挂。
// 改用真实探测后，GT 710 / GT 1030 / MX 这类“有 N 卡但无可用 NVENC”的机器会干净地回退到 Intel/AMD/CPU 分支。
//
// 注意：探测的视频参数需与 ffmpeg 包 AnyVideoToMP4 的 NVENC 分支保持一致，
// 这样“探测通过”才等价于“真实编码能跑通”；那边若调整参数，这里也要同步。
func probeNvenc() bool {
	probe := exec.Command("ffmpeg",
		"-hide_banner",
		"-f", "lavfi", "-i", "nullsrc=s=256x256:d=1",
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
		"-f", "null", "-",
	)
	// Stdout/Stderr 均为 nil 时 Go 会把 ffmpeg 输出接到 /dev/null，探测过程不会污染批处理日志
	return runProbe(probe)
}

// probeQsv 用合成源跑一次真实的 h264_qsv 最小编码，退出码为 0 才认为 QSV 可用。
// 能挡掉驱动缺失、或 iGPU 不支持本分支较新参数（mbbrc / rdo / look_ahead）而运行期报错的情况。
//
// 探测刻意不带 -hwaccel qsv / -hwaccel_output_format qsv：那是“硬件解码”输入选项，nullsrc 合成源无需解码；
// 这里只验证“编码”能力，也正是编码参数不兼容会失败的地方。参数需与 ffmpeg 包 QSV 分支保持一致。
func probeQsv() bool {
	probe := exec.Command("ffmpeg",
		"-hide_banner",
		"-f", "lavfi", "-i", "nullsrc=s=256x256:d=1",
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
		"-f", "null", "-",
	)
	return runProbe(probe)
}

// probeAmf 用合成源跑一次真实的 h264_amf 最小编码，退出码为 0 才认为 AMF 可用。
// 取代原来 lspci/system_profiler/wmic 查厂商名 + ffmpeg 编译了 h264_amf 的存在性检查——
// 那些只说明“有 AMD 卡/有编码器”，不代表这台机器真能用 AMF 编码（老卡、驱动缺失都会运行期失败）。
// AMF 实际仅 Windows 构建提供，非 Windows 上 h264_amf 不存在、探测自然失败，无需额外平台门槛。
// 探测参数需与 ffmpeg 包 AnyVideoToMP4 的 AMF 分支保持一致。
func probeAmf() bool {
	probe := exec.Command("ffmpeg",
		"-hide_banner",
		"-f", "lavfi", "-i", "nullsrc=s=256x256:d=1",
		"-c:v", "h264_amf",
		"-usage", "high_quality",
		"-quality", "quality",
		"-qp_i", "18",
		"-qp_p", "20",
		"-qp_b", "22",
		"-vbaq", "true",
		"-preanalysis", "true",
		"-profile", "high",
		"-f", "null", "-",
	)
	return runProbe(probe)
}

// probeVideoToolbox 用合成源跑一次真实的 h264_videotoolbox 最小编码，退出码为 0 才认为可用。
// 探测参数需与 ffmpeg 包 AnyVideoToMP4 的 VideoToolbox 分支保持一致。
func probeVideoToolbox() bool {
	probe := exec.Command("ffmpeg",
		"-hide_banner",
		"-f", "lavfi", "-i", "nullsrc=s=256x256:d=1",
		"-c:v", "h264_videotoolbox",
		"-q:v", "70",
		"-profile:v", "high",
		"-coder", "cabac",
		"-spatial_aq", "1",
		"-allow_sw", "1",
		"-f", "null", "-",
	)
	return runProbe(probe)
}

// probeMediaFoundation 探测是否存在真正可用的 Media Foundation 硬件 H.264 编码器，
// 退出码为 0 才认为可用。适用于 Windows ARM64 高通平台（Surface 10/11 等）以及任何
// 通过 Media Foundation 暴露硬编码器的 Windows 机器。
//
// 为什么探测要刻意走「d3d11 设备 → hwupload 上传显存帧 → -hw_encoding 1」这条全硬链路，
// 而不是像其它 probe* 那样直接拿 nullsrc 软帧编码：
// h264_mf 不加 -hw_encoding 时会优先选用微软的软件 MFT（CPU 编码），那种“探测通过”
// 只代表机器上装了 Media Foundation，不代表有可用硬件编码器，真实任务就会出现
// “日志显示 h264_mf、但 CPU 70%+ / GPU 1%”的假象。-hw_encoding 1 又只接受 d3d11
// 显存帧，所以探测必须先 -init_hw_device d3d11va 建设备、format=nv12,hwupload 上传。
//
// ⚠️ -profile:v 必须传数字 profile_idc（66=Baseline / 77=Main / 100=High），
// 传 "high" 字符串会在参数解析阶段直接失败（与硬件无关，任何机器必现）。
// 编码质量参数需与 ffmpeg 包 EncoderQualcomm 分支保持一致；该分支真实任务由
// -hwaccel d3d11va -hwaccel_output_format d3d11 提供显存帧，探测则用 hwupload 等价模拟。
func probeMediaFoundation() bool {
	probe := exec.Command("ffmpeg",
		"-hide_banner",
		"-init_hw_device", "d3d11va=hw",
		"-filter_hw_device", "hw",
		"-f", "lavfi", "-i", "nullsrc=s=256x256:d=1",
		"-vf", "format=nv12,hwupload",
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
		"-f", "null", "-",
	)
	return runProbe(probe)
}
