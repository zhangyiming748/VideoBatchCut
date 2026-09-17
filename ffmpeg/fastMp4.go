package ffmpeg

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func AnyVideoToMP4(fp string) error {
	var (
		cmd       *exec.Cmd
		args      []string
		tempName  string
		finalName string
	)

	// 生成输出文件名（确保小写扩展名）
	ext := strings.ToLower(filepath.Ext(fp))
	switch ext {
	case ".mp4":
		tempName = strings.Replace(fp, filepath.Ext(fp), "_tmp.mp4", 1)
		finalName = strings.Replace(fp, filepath.Ext(fp), ".mp4", 1)
	case ".mkv":
		return forMkv(fp)
	default:
		tempName = strings.Replace(fp, filepath.Ext(fp), ".mp4", 1)
		finalName = tempName
	}
	// 这里首先判断一下 是否已经存在最终会输出的文件 如果存在 先给一个警告然后直接返回
	// if isExist(finalName) {
	// 	log.Printf("文件%s已存在，请勿重复处理\n", finalName)
	// 	return nil
	// }
	if hasNvidia() {
		log.Println("[分支] AnyVideoToMP4 使用 NVIDIA NVENC 硬件编码")
		time.Sleep(3 * time.Second)
		// 构建ffmpeg命令参数
		args = append(args, "-i", fp)
		args = append(args, "-c:v", "h264_nvenc")
		args = append(args, "-preset", "p7")
		args = append(args, "-tune", "hq")
		args = append(args, "-rc", "vbr")
		args = append(args, "-b:v", "0")
		args = append(args, "-cq:v", "19") // 恒定质量 (0-51)；片源多为二次压缩视频，19 已足够透明，再低只是为源中已有的 artifact 白付码率
		args = append(args, "-rc-lookahead", "32")
		args = append(args, "-spatial-aq", "1")   // 空间自适应量化：官方说明会把额外比特分配给平坦区域
		args = append(args, "-temporal-aq", "1")  // 时间自适应量化：改善静态高细节区域（大面积亮/暗部）
		args = append(args, "-aq-strength", "11") // AQ 强度 1-15；不拉满，避免从复杂纹理区抽走过量码率（驱动默认 8）
		args = append(args, "-profile:v", "high")
		args = append(args, "-c:a", "aac")
		args = append(args, tempName)
	} else if hasIntel() {
		log.Println("[分支] AnyVideoToMP4 使用 Intel VAAPI 硬件编码")
		time.Sleep(3 * time.Second)
		// 使用 Intel 核显的 H.264 硬件加速编码 (VAAPI)
		// 容器化部署经实测：QSV 在容器内不易穿透，改用 VAAPI + render node 稳定可用
		// 需将 /dev/dri/renderD128 透传进容器（--device /dev/dri/renderD128），并把用户加入 video/render 组
		// 以下三项均为输入选项，必须排在 -i 之前
		args = append(args, "-hwaccel", "vaapi")
		args = append(args, "-hwaccel_device", "/dev/dri/renderD128")
		args = append(args, "-hwaccel_output_format", "vaapi")
		args = append(args, "-i", fp)
		args = append(args, "-c:v", "h264_vaapi")
		// 画质：沿用代码原有的恒定质量档 global_quality 18
		// 关键（据 ffmpeg vaapi_encode.c 源码）：只给 -global_quality 而不加 -rc_mode/-q:v 时，
		// ffmpeg 自动优先选 ICQ（内容自适应，Intel iHD 的 H264 支持 VA_RC_ICQ），驱动不支持时再优雅回退 CQP；
		// 不要显式写 -rc_mode ICQ——那样驱动若不支持会直接 EINVAL 报错、整批任务失败；
		// 也不要用 -q:v——它会置位 QSCALE flag 强制落入 CQP（与 QSV 的老坑同构）
		// 18 经 ArchWiki 实测为 h264_vaapi 视觉无损档（20 起才有极轻微损失）
		// 注意：QSV 专属选项（look_ahead/look_ahead_depth/extbrc/mbbrc/rdo/adaptive_i/adaptive_b）
		// 在 h264_vaapi 下不存在，会被判为未识别选项导致整条命令失败，故移除
		args = append(args, "-global_quality", "18") // 质量档 (1-51，越小越好)，与代码原值一致
		args = append(args, "-bf", "4")              // B 帧数量，60fps 下提升压缩效率
		args = append(args, "-profile:v", "high")    // H.264 High Profile
		args = append(args, "-c:a", "aac")           // AAC音频编码
		args = append(args, "-b:a", "192k")          // 音频比特率
		args = append(args, tempName)
	} else if hasAMD() {
		log.Println("[分支] AnyVideoToMP4 使用 AMD AMF 硬件编码")
		time.Sleep(3 * time.Second)
		// 使用AMD显卡的H.264硬件加速编码 (AMF/VCE)
		args = append(args, "-i", fp)
		args = append(args, "-c:v", "h264_amf")
		args = append(args, "-usage", "high_quality") // 高质量转码预设；原 transcoding 是低码率网络传输场景
		args = append(args, "-quality", "quality")    // 质量优先模式
		args = append(args, "-qp_i", "18")            // I帧量化参数（越小质量越高）
		args = append(args, "-qp_p", "20")            // P帧量化参数
		args = append(args, "-qp_b", "22")            // B帧量化参数
		args = append(args, "-vbaq", "true")          // 方差自适应量化，把码率优先分给平坦区域
		args = append(args, "-preanalysis", "true")   // 预分析，改善码率分配（AMD 官方推荐设置）
		args = append(args, "-profile", "high")       // H.264 High Profile
		args = append(args, "-c:a", "aac")            // AAC音频编码
		args = append(args, "-b:a", "192k")           // 音频比特率
		args = append(args, tempName)
	} else if hasAppleSilicon() {
		log.Println("[分支] AnyVideoToMP4 使用 Apple VideoToolbox 硬件编码")
		time.Sleep(3 * time.Second)
		// Apple Silicon（M1/M2/M3/M4 系列）自带媒体引擎，使用 VideoToolbox 硬件 H.264 编码
		args = append(args, "-i", fp)
		args = append(args, "-c:v", "h264_videotoolbox")
		// -q:v 是 VideoToolbox 的恒定质量档，范围 1-100 且“越大画质越高”
		//（已上机验证：同源 q30≈388KB、q90≈3.86MB）；与 libx264 crf19 标定后取 70：
		// VideoToolbox 是比 x264 slow 更简单的编码器，同等体积画质略逊，稍抬高质量档以保证对二手压缩源透明
		args = append(args, "-q:v", "70")
		args = append(args, "-profile:v", "high") // H.264 High Profile
		args = append(args, "-coder", "cabac")    // CABAC 熵编码，压缩效率优于默认 CAVLC
		args = append(args, "-spatial_aq", "1")   // 空间自适应量化（macOS 支持），把额外比特分配给平坦区域，压制块效应
		args = append(args, "-allow_sw", "1")     // 硬件编码器不可用时回退到 VideoToolbox 软件编码，避免整批任务失败
		args = append(args, "-c:a", "aac")        // AAC音频编码
		args = append(args, "-b:a", "192k")       // 音频比特率
		args = append(args, tempName)
	} else {
		log.Println("[分支] AnyVideoToMP4 使用 CPU libx264 软件编码")
		time.Sleep(3 * time.Second)
		// 使用CPU软件编码 libx264（平衡质量和文件大小）
		args = append(args, "-i", fp)
		args = append(args, "-c:v", "libx264")
		args = append(args, "-preset", "slow")    // 慢速预设，压缩效率更高
		args = append(args, "-crf", "19")         // 恒定速率因子；二手压缩源用 19 足够透明，再低只是为已有 artifact 白付码率
		args = append(args, "-profile:v", "high") // H.264 High Profile
		// 不再硬写 -level：1080p60 需要 MaxMBPS 522240（Level 4.2+），而 4.1 只有 245760，会被限流降质
		args = append(args, "-pix_fmt", "yuv420p")             // 广泛兼容的像素格式
		args = append(args, "-x264-params", "aq-strength=1.2") // 加强平坦区域自适应量化，压制块效应（默认1.0）
		args = append(args, "-psy-rd", "0.6:0.0")              // 压低心理视觉优化（preset slow 默认1.0:0.0），不去增强源中本不存在的纹理
		args = append(args, "-c:a", "aac")                     // AAC音频编码
		args = append(args, "-b:a", "192k")                    // 音频比特率
		args = append(args, tempName)
	}
	cmd = exec.Command("ffmpeg", args...)
	log.Printf("执行命令:%v\n", cmd.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("ffmpeg快速处理文件%s失败:%v\nFFmpeg输出:\n%s\n", fp, err, string(output))
		return err
	} else {
		log.Printf("文件%s处理成功\n", fp)
	}

	// 验证输出文件是否有效
	if fileInfo, err := os.Stat(tempName); err != nil {
		log.Printf("输出文件%s不存在:%v\n", tempName, err)
		return err
	} else if fileInfo.Size() == 0 {
		log.Printf("输出文件%s大小为0字节，转换失败，保留原文件\n", tempName)
		// 删除失败的输出文件
		os.Remove(tempName)
		return fmt.Errorf("输出文件大小为0")
	}

	// 如果原文件是MP4，需要删除原文件并重命名
	if ext == ".mp4" {
		err = os.Rename(tempName, finalName)
		if err != nil {
			log.Printf("重命名文件%s失败:%v\n", finalName, err)
			return err
		}
		err = os.Remove(fp)
		if err != nil {
			log.Printf("删除文件%s失败:%v\n", fp, err)
			return err
		}
	} else {
		// 其他格式：tempName和finalName相同，只需删除原文件
		err = os.Remove(fp)
		if err != nil {
			log.Printf("删除文件%s失败:%v\n", fp, err)
			return err
		}
	}

	return nil
}

/*
	ffmpeg -i video.mp4 -stream_loop -1 -i audio.mp3 \
	  -c:v h264_nvenc -preset p7 -tune hq -rc vbr_hq -cq 18 -b:v 0 \
	  -spatial-aq 1 -temporal-aq 1 -aq-strength 15 \
	  -profile:v high -level 5.1 \
	  -c:a aac -b:a 320k -ar 48000 \
	  -map 0:v:0 -map 1:a:0 -shortest \
	  -max_muxing_queue_size 9999 output.mp4
*/
func ForDji(videoPath, audioPath string) error {
	// 检查是否为mp4文件(大小写不敏感)
	ext := strings.ToLower(filepath.Ext(videoPath))
	if ext != ".mp4" {
		return nil
	}
	// 生成临时文件名
	tempName := strings.Replace(videoPath, filepath.Ext(videoPath), "_tmp.mp4", 1)
	// 最终文件名（确保小写扩展名）
	finalName := strings.Replace(videoPath, filepath.Ext(videoPath), ".mp4", 1)
	var cmd *exec.Cmd
	var args []string
	args = append(args, "-i", videoPath)
	args = append(args, "-stream_loop", "-1")
	args = append(args, "-i", audioPath)
	// 使用NVENC高质量编码，几乎无损
	args = append(args, "-c:v", "h264_nvenc")
	args = append(args, "-preset", "p7")      // 最高质量预设（最慢但质量最好）
	args = append(args, "-tune", "hq")        // 高质量调优
	args = append(args, "-rc", "vbr_hq")      // 高质量可变比特率
	args = append(args, "-cq", "18")          // 恒定质量模式，18接近无损（范围0-51，越小质量越高）
	args = append(args, "-b:v", "0")          // 不限制比特率
	args = append(args, "-maxrate", "0")      // 不限制最大比特率
	args = append(args, "-bufsize", "0")      // 不限制缓冲区大小
	args = append(args, "-spatial-aq", "1")   // 空间自适应量化，提升质量
	args = append(args, "-temporal-aq", "1")  // 时间自适应量化，提升质量
	args = append(args, "-aq-strength", "15") // AQ强度（1-15，15最强）
	args = append(args, "-profile:v", "high") // H.264 High Profile
	args = append(args, "-level", "5.1")      // 支持1080p60
	// 音频编码
	args = append(args, "-c:a", "aac")
	args = append(args, "-b:a", "320k") // 高质量音频
	args = append(args, "-ar", "48000") // 采样率48kHz
	args = append(args, "-map", "0:v:0")
	args = append(args, "-map", "1:a:0")
	args = append(args, "-shortest")
	// 增加缓冲区大小以避免无限循环音频导致的缓冲区溢出
	args = append(args, "-max_muxing_queue_size", "9999")
	args = append(args, tempName)
	cmd = exec.Command("ffmpeg", args...)
	log.Printf("执行命令:%v\n", cmd.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("ffmpeg快速处理文件%s失败:%v\n输出:%s\n", videoPath, err, string(output))
		return err
	}

	// 验证输出文件是否有效
	if fileInfo, err := os.Stat(tempName); err != nil {
		log.Printf("输出文件%s不存在:%v\n", tempName, err)
		return err
	} else if fileInfo.Size() == 0 {
		log.Printf("输出文件%s大小为0字节，转换失败，保留原文件\n", tempName)
		// 删除失败的输出文件
		os.Remove(tempName)
		return fmt.Errorf("输出文件大小为0")
	}

	err = os.Remove(videoPath)
	if err != nil {
		log.Printf("删除文件%s失败:%v\n", videoPath, err)
		return err
	}
	err = os.Rename(tempName, finalName)
	if err != nil {
		log.Printf("重命名文件%s失败:%v\n", finalName, err)
		return err
	}
	return nil
}

func hasNvidia() bool {
	// 检查系统中是否存在NVIDIA GPU
	// 跨平台检测：尝试执行nvidia-smi（Linux/Windows）或检查system_profiler（macOS）
	var cmd *exec.Cmd

	// 先尝试nvidia-smi（Linux和Windows通用）
	cmd = exec.Command("nvidia-smi")
	if err := cmd.Run(); err == nil {
		// nvidia-smi存在且执行成功，再检查FFmpeg是否支持nvenc
		ffmpegCmd := exec.Command("ffmpeg", "-encoders")
		output, err := ffmpegCmd.CombinedOutput()
		if err != nil {
			return false
		}
		return strings.Contains(string(output), "h264_nvenc")
	}

	return false
}

func hasIntel() bool {
	// 检查系统中是否存在Intel GPU并支持QSV
	// 跨平台检测策略：
	// 1. Linux: 检查/dev/dri设备
	// 2. macOS: 检查system_profiler输出
	// 3. Windows: 通过wmic或powershell检测

	hasIntelGPU := false

	// 尝试Linux方式：检查/dev/dri
	if _, err := os.Stat("/dev/dri"); err == nil {
		hasIntelGPU = true
	}

	// 如果Linux方式失败，尝试macOS方式
	if !hasIntelGPU {
		cmd := exec.Command("system_profiler", "SPDisplaysDataType")
		output, err := cmd.CombinedOutput()
		if err == nil && strings.Contains(string(output), "Intel") {
			hasIntelGPU = true
		}
	}

	// 如果前两种方式都失败，尝试Windows方式
	if !hasIntelGPU {
		cmd := exec.Command("wmic", "path", "win32_VideoController", "get", "name")
		output, err := cmd.CombinedOutput()
		if err == nil && strings.Contains(string(output), "Intel") {
			hasIntelGPU = true
		}
	}

	// 检测到Intel GPU后，再检查FFmpeg是否支持vaapi（本分支已改用 VAAPI 编码）
	if hasIntelGPU {
		ffmpegCmd := exec.Command("ffmpeg", "-encoders")
		output, err := ffmpegCmd.CombinedOutput()
		if err != nil {
			return false
		}
		return strings.Contains(string(output), "h264_vaapi")
	}

	return false
}

func hasAMD() bool {
	// 检查系统中是否存在AMD GPU
	// 跨平台检测策略：
	// 1. Linux: 检查lspci输出
	// 2. macOS: 检查system_profiler输出
	// 3. Windows: 通过wmic检测

	hasAMDGPU := false

	// 尝试Linux方式：检查lspci
	cmd := exec.Command("lspci")
	output, err := cmd.CombinedOutput()
	if err == nil && (strings.Contains(string(output), "AMD") || strings.Contains(string(output), "ATI")) {
		hasAMDGPU = true
	}

	// 如果Linux方式失败，尝试macOS方式
	if !hasAMDGPU {
		cmd := exec.Command("system_profiler", "SPDisplaysDataType")
		output, err := cmd.CombinedOutput()
		if err == nil && (strings.Contains(string(output), "AMD") || strings.Contains(string(output), "Radeon")) {
			hasAMDGPU = true
		}
	}

	// 如果前两种方式都失败，尝试Windows方式
	if !hasAMDGPU {
		cmd := exec.Command("wmic", "path", "win32_VideoController", "get", "name")
		output, err := cmd.CombinedOutput()
		if err == nil && (strings.Contains(string(output), "AMD") || strings.Contains(string(output), "Radeon")) {
			hasAMDGPU = true
		}
	}

	// 检测到AMD GPU后，再检查FFmpeg是否支持amf
	if hasAMDGPU {
		ffmpegCmd := exec.Command("ffmpeg", "-encoders")
		ffmpegOutput, err := ffmpegCmd.CombinedOutput()
		if err != nil {
			return false
		}
		return strings.Contains(string(ffmpegOutput), "h264_amf")
	}

	return false
}

func hasAppleSilicon() bool {
	// 检测是否为 Apple Silicon（M1/M2/M3/M4 系列）芯片：macOS + arm64 架构
	// 注意：若用户误在 Apple Silicon 上运行 amd64 二进制（Rosetta 转译），GOARCH 会是 amd64 而落入 CPU 分支；
	// release 已提供 darwin/arm64 构建，正常下载 arm64 版本即可命中此分支
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		return false
	}
	// 确认 FFmpeg 编译进了 VideoToolbox 编码器
	ffmpegCmd := exec.Command("ffmpeg", "-encoders")
	output, err := ffmpegCmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "h264_videotoolbox")
}

func isExist(path string) bool {
	// 检查文件或目录是否存在
	_, err := os.Stat(path)
	if err == nil {
		return true // 文件存在
	}
	if os.IsNotExist(err) {
		return false // 文件不存在
	}
	// 其他错误（如权限问题），也认为不存在
	return false
}
func forMkv(fp string) error {
	var (
		cmd      *exec.Cmd
		args     []string
		tempName string
	)
	tempName = strings.Replace(fp, filepath.Ext(fp), "_tmp.mkv", 1)
	if hasNvidia() {
		log.Println("[分支] forMkv 使用 NVIDIA NVENC 硬件编码")
		time.Sleep(3 * time.Second)
		// NVIDIA GPU 硬件加速编码 - MKV 格式
		args = append(args, "-i", fp)
		// 视频流：H.264 NVENC 编码
		args = append(args, "-c:v", "h264_nvenc")
		args = append(args, "-preset", "p7")
		args = append(args, "-tune", "hq")
		args = append(args, "-rc", "vbr")
		args = append(args, "-b:v", "0")
		args = append(args, "-cq:v", "19") // 恒定质量 (0-51)；片源多为二次压缩视频，19 已足够透明，再低只是为源中已有的 artifact 白付码率
		args = append(args, "-rc-lookahead", "32")
		args = append(args, "-spatial-aq", "1")   // 空间自适应量化：官方说明会把额外比特分配给平坦区域
		args = append(args, "-temporal-aq", "1")  // 时间自适应量化：改善静态高细节区域（大面积亮/暗部）
		args = append(args, "-aq-strength", "11") // AQ 强度 1-15；不拉满，避免从复杂纹理区抽走过量码率（驱动默认 8）
		args = append(args, "-profile:v", "high")
		// 音频流：转码为 FLAC（无损）
		args = append(args, "-c:a", "flac")
		// 字幕流：完全复制
		args = append(args, "-c:s", "copy")
		args = append(args, tempName)
	} else if hasIntel() {
		log.Println("[分支] forMkv 使用 Intel VAAPI 硬件编码")
		time.Sleep(3 * time.Second)
		// Intel VAAPI 硬件加速编码 - MKV 格式
		// 容器化部署经实测：QSV 在容器内不易穿透，改用 VAAPI + render node 稳定可用
		// 需将 /dev/dri/renderD128 透传进容器，并把用户加入 video/render 组
		// 以下三项均为输入选项，必须排在 -i 之前
		args = append(args, "-hwaccel", "vaapi")
		args = append(args, "-hwaccel_device", "/dev/dri/renderD128")
		args = append(args, "-hwaccel_output_format", "vaapi")
		args = append(args, "-i", fp)
		// 视频流：H.264 VAAPI 编码
		args = append(args, "-c:v", "h264_vaapi")
		// 画质：沿用代码原有的恒定质量档 global_quality 18
		// 只给 -global_quality（不加 -rc_mode/-q:v）时 ffmpeg 自动优先 ICQ、不支持则优雅回退 CQP；
		// 显式 -rc_mode ICQ 会在驱动不支持时直接报错，-q:v 会强制 CQP，两者都不要用
		// QSV 专属选项（look_ahead/look_ahead_depth/extbrc/mbbrc/rdo/adaptive_i/adaptive_b）在 h264_vaapi 下不存在，移除以免命令失败
		args = append(args, "-global_quality", "18") // 质量档 (1-51，越小越好)，与代码原值一致
		args = append(args, "-bf", "4")              // B 帧数量，60fps 下提升压缩效率
		args = append(args, "-profile:v", "high")    // H.264 High Profile
		// 音频流：转码为 FLAC（无损）
		args = append(args, "-c:a", "flac")
		// 字幕流：完全复制
		args = append(args, "-c:s", "copy")
		args = append(args, tempName)
	} else if hasAMD() {
		log.Println("[分支] forMkv 使用 AMD AMF 硬件编码")
		time.Sleep(3 * time.Second)
		// AMD AMF 硬件加速编码 - MKV 格式
		args = append(args, "-i", fp)
		// 视频流：H.264 AMF 编码
		args = append(args, "-c:v", "h264_amf")
		args = append(args, "-usage", "high_quality") // 高质量转码预设；原 transcoding 是低码率网络传输场景
		args = append(args, "-quality", "quality")    // 质量优先模式
		args = append(args, "-qp_i", "18")            // I帧量化参数（越小质量越高）
		args = append(args, "-qp_p", "20")            // P帧量化参数
		args = append(args, "-qp_b", "22")            // B帧量化参数
		args = append(args, "-vbaq", "true")          // 方差自适应量化，把码率优先分给平坦区域
		args = append(args, "-preanalysis", "true")   // 预分析，改善码率分配（AMD 官方推荐设置）
		args = append(args, "-profile", "high")       // H.264 High Profile
		// 音频流：转码为 FLAC（无损）
		args = append(args, "-c:a", "flac")
		// 字幕流：完全复制
		args = append(args, "-c:s", "copy")
		args = append(args, tempName)
	} else if hasAppleSilicon() {
		log.Println("[分支] forMkv 使用 Apple VideoToolbox 硬件编码")
		time.Sleep(3 * time.Second)
		// Apple Silicon（M1/M2/M3/M4 系列）自带媒体引擎，使用 VideoToolbox 硬件 H.264 编码 - MKV 格式
		args = append(args, "-i", fp)
		// 视频流：H.264 VideoToolbox 编码
		args = append(args, "-c:v", "h264_videotoolbox")
		// -q:v 是 VideoToolbox 的恒定质量档，范围 1-100 且“越大画质越高”，取 70 与 libx264 crf19 对标
		args = append(args, "-q:v", "70")
		args = append(args, "-profile:v", "high") // H.264 High Profile
		args = append(args, "-coder", "cabac")    // CABAC 熵编码，压缩效率优于默认 CAVLC
		args = append(args, "-spatial_aq", "1")   // 空间自适应量化（macOS 支持），把额外比特分配给平坦区域，压制块效应
		args = append(args, "-allow_sw", "1")     // 硬件编码器不可用时回退到 VideoToolbox 软件编码，避免整批任务失败
		// 音频流：转码为 FLAC（无损）
		args = append(args, "-c:a", "flac")
		// 字幕流：完全复制
		args = append(args, "-c:s", "copy")
		args = append(args, tempName)
	} else {
		log.Println("[分支] forMkv 使用 CPU libx264 软件编码")
		time.Sleep(3 * time.Second)
		// CPU 软件编码 libx264 - MKV 格式
		args = append(args, "-i", fp)
		// 视频流：H.264 软件编码
		args = append(args, "-c:v", "libx264")
		args = append(args, "-preset", "slow")    // 慢速预设，压缩效率更高
		args = append(args, "-crf", "19")         // 恒定速率因子；二手压缩源用 19 足够透明，再低只是为已有 artifact 白付码率
		args = append(args, "-profile:v", "high") // H.264 High Profile
		// 不再硬写 -level：1080p60 需要 MaxMBPS 522240（Level 4.2+），而 4.1 只有 245760，会被限流降质
		args = append(args, "-pix_fmt", "yuv420p")             // 广泛兼容的像素格式
		args = append(args, "-x264-params", "aq-strength=1.2") // 加强平坦区域自适应量化，压制块效应（默认1.0）
		args = append(args, "-psy-rd", "0.6:0.0")              // 压低心理视觉优化（preset slow 默认1.0:0.0），不去增强源中本不存在的纹理
		// 音频流：转码为 FLAC（无损）
		args = append(args, "-c:a", "flac")
		// 字幕流：完全复制
		args = append(args, "-c:s", "copy")
		args = append(args, tempName)
	}
	cmd = exec.Command("ffmpeg", args...)
	log.Printf("执行命令:%v\n", cmd.String())
	_, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("ffmpeg快速处理文件%s失败:%v\n", fp, err)
		return err
	}

	// 验证输出文件是否有效
	if fileInfo, err := os.Stat(tempName); err != nil {
		log.Printf("输出文件%s不存在:%v\n", tempName, err)
		return err
	} else if fileInfo.Size() == 0 {
		log.Printf("输出文件%s大小为0字节，转换失败，保留原文件\n", tempName)
		// 删除失败的输出文件
		os.Remove(tempName)
		return fmt.Errorf("输出文件大小为0")
	}

	err = os.Remove(fp)
	if err != nil {
		log.Printf("删除文件%s失败:%v\n", fp, err)
		return err
	}
	err = os.Rename(tempName, fp)
	if err != nil {
		log.Printf("重命名文件%s失败:%v\n", fp, err)
		return err
	}

	return nil
}
