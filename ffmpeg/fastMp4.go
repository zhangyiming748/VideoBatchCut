package ffmpeg

import (
	"VideoBatchCut/util"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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
	default:
		// 其余所有格式（含 MKV）一视同仁转成 MP4
		tempName = strings.Replace(fp, filepath.Ext(fp), ".mp4", 1)
		finalName = tempName
	}
	// 这里首先判断一下 是否已经存在最终会输出的文件 如果存在 先给一个警告然后直接返回
	// if isExist(finalName) {
	// 	log.Printf("文件%s已存在，请勿重复处理\n", finalName)
	// 	return nil
	// }
	if util.HasNvidia() {
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
	} else if util.HasIntel() {
		log.Println("[分支] AnyVideoToMP4 使用 Intel QSV 硬件编码")
		time.Sleep(3 * time.Second)
		// 使用 Intel 核显的 H.264 硬件加速编码 (QSV，Quick Sync Video)
		// 本分支运行于 Windows，QSV 会自动选用默认显卡适配器，无需指定 device 路径
		// 以下两项均为输入选项，必须排在 -i 之前
		args = append(args, "-hwaccel", "qsv")
		args = append(args, "-hwaccel_output_format", "qsv")
		args = append(args, "-i", fp)
		args = append(args, "-c:v", "h264_qsv")
		// 画质：沿用代码原有的恒定质量档 global_quality 18（视觉无损档，与其它分支口径一致）
		// 只给 -global_quality 时 QSV 默认走 ICQ（内容自适应恒定质量）；再开启 look_ahead 会升级为 LA_ICQ，画质更好
		args = append(args, "-preset", "veryslow")     // QSV 质量优先档，压缩效率最高
		args = append(args, "-global_quality", "18")   // 质量档 (1-51，越小越好)，与代码原值一致
		args = append(args, "-look_ahead", "1")        // 前瞻码率控制（LA_ICQ），改善复杂场景的比特分配
		args = append(args, "-look_ahead_depth", "40") // 前瞻帧数
		args = append(args, "-extbrc", "1")            // 扩展码率控制，进一步平滑质量
		args = append(args, "-mbbrc", "1")             // 宏块级码率控制，把码率重分配到平坦区（官方称可改善主观质量）
		args = append(args, "-rdo", "1")               // 率失真优化，同等画质下压缩更高效
		args = append(args, "-adaptive_i", "1")        // 自适应插入 I 帧
		args = append(args, "-adaptive_b", "1")        // 自适应插入 B 帧
		args = append(args, "-bf", "4")                // B 帧数量，60fps 下提升压缩效率
		args = append(args, "-profile:v", "high")      // H.264 High Profile
		args = append(args, "-c:a", "aac")             // AAC音频编码
		args = append(args, "-b:a", "192k")            // 音频比特率
		args = append(args, tempName)
	} else if util.HasAMD() {
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
	} else if util.HasAppleSilicon() {
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
