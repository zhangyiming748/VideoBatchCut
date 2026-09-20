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
	"os/exec"
	"runtime"
	"sync"
)

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
	ffmpegProbe       probeCache
	nvencProbe        probeCache
	qsvProbe          probeCache
	amfProbe          probeCache
	videotoolboxProbe probeCache
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
	// macOS 上不走 NVENC（苹果平台统一用 VideoToolbox），直接短路，省去无谓的编码探测
	if runtime.GOOS == "darwin" {
		return false
	}
	if !hasFFmpeg() {
		return false
	}
	return nvencProbe.get(probeNvenc)
}

// HasIntel 判断 ffmpeg 是否真的能用 h264_qsv 编码。
func HasIntel() bool {
	// 平台门槛：本分支的 QSV 参数按 Windows 设计（自动选默认适配器，不带 -init_hw_device）。
	// Linux 上的 QSV 需要显式 -init_hw_device 指向 render 节点（见 README），与本分支参数不匹配；
	// 若仅凭编码探测放行，可能在 Linux 上误入本分支、导致 -hwaccel qsv 解码失败。故沿用 Windows-only 约束
	//（该约束已隐含排除 macOS：macOS 统一走 VideoToolbox，不会进入本 Intel 分支）。
	if runtime.GOOS != "windows" {
		return false
	}
	if !hasFFmpeg() {
		return false
	}
	return qsvProbe.get(probeQsv)
}

// HasAMD 判断 ffmpeg 是否真的能用 h264_amf 编码。
func HasAMD() bool {
	// macOS 上不走 AMF（AMF 仅 Windows 提供），直接短路，省去无谓的编码探测
	if runtime.GOOS == "darwin" {
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
	return probe.Run() == nil
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
		"-global_quality", "18",
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
	return probe.Run() == nil
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
	return probe.Run() == nil
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
	return probe.Run() == nil
}
