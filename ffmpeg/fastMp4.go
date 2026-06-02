package ffmpeg

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	if ext == ".mp4" {
		tempName = strings.Replace(fp, filepath.Ext(fp), "_tmp.mp4", 1)
		finalName = strings.Replace(fp, filepath.Ext(fp), ".mp4", 1)
	} else {
		tempName = strings.Replace(fp, filepath.Ext(fp), ".mp4", 1)
		finalName = tempName
	}
	// 这里首先判断一下 是否已经存在最终会输出的文件 如果存在 先给一个警告然后直接返回
	if isExist(finalName) {
		log.Printf("文件%s已存在，请勿重复处理\n", finalName)
		return nil
	}
	if hasNvidia() {
		// 构建ffmpeg命令参数
		args = append(args, "-i", fp)
		args = append(args, "-c:v", "h264_nvenc")
		args = append(args, "-preset", "p7")
		args = append(args, "-tune", "hq")
		args = append(args, "-rc", "vbr")
		args = append(args, "-b:v", "0")
		args = append(args, "-cq:v", "23")
		args = append(args, "-rc-lookahead", "32")
		args = append(args, "-spatial-aq", "1")
		args = append(args, "-profile:v", "high")
		args = append(args, "-c:a", "aac")
		args = append(args, tempName)
	} else if hasIntel() {
		// 使用Intel核显的H.264硬件加速编码 (QSV)
		args = append(args, "-i", fp)
		args = append(args, "-c:v", "h264_qsv")
		args = append(args, "-preset", "slow")         // 慢速预设，质量更好
		args = append(args, "-q", "18")                // 恒定质量模式，18接近无损（范围1-51，越小质量越高）
		args = append(args, "-look_ahead", "1")        // 启用 lookahead，提升质量
		args = append(args, "-look_ahead_depth", "40") // lookahead 深度
		args = append(args, "-profile:v", "high")      // H.264 High Profile
		args = append(args, "-c:a", "aac")             // AAC音频编码
		args = append(args, "-b:a", "192k")            // 音频比特率
		args = append(args, tempName)
	} else if hasAMD() {
		// 使用AMD显卡的H.264硬件加速编码 (AMF/VCE)
		args = append(args, "-i", fp)
		args = append(args, "-c:v", "h264_amf")
		args = append(args, "-quality", "quality")   // 质量优先模式
		args = append(args, "-qp_i", "18")           // I帧量化参数（越小质量越高）
		args = append(args, "-qp_p", "20")           // P帧量化参数
		args = append(args, "-qp_b", "22")           // B帧量化参数
		args = append(args, "-profile", "high")      // H.264 High Profile
		args = append(args, "-usage", "transcoding") // 转码优化模式
		args = append(args, "-c:a", "aac")           // AAC音频编码
		args = append(args, "-b:a", "192k")          // 音频比特率
		args = append(args, tempName)
	} else {
		// 使用CPU软件编码 libx264（平衡质量和文件大小）
		args = append(args, "-i", fp)
		args = append(args, "-c:v", "libx264")
		args = append(args, "-preset", "slow")     // 慢速预设，压缩效率更高
		args = append(args, "-crf", "23")          // 恒定速率因子，23是默认值，平衡质量和大小
		args = append(args, "-profile:v", "high")  // H.264 High Profile
		args = append(args, "-level", "4.1")       // 兼容性更好的级别
		args = append(args, "-pix_fmt", "yuv420p") // 广泛兼容的像素格式
		args = append(args, "-c:a", "aac")         // AAC音频编码
		args = append(args, "-b:a", "192k")        // 音频比特率
		args = append(args, tempName)
	}
	cmd = exec.Command("ffmpeg", args...)
	log.Printf("执行命令:%v\n", cmd.String())
	_, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("ffmpeg快速处理文件%s失败:%v\n", fp, err)
		return err
	}

	// 如果原文件是MP4，需要删除原文件并重命名
	if ext == ".mp4" {
		err = os.Remove(fp)
		if err != nil {
			log.Printf("删除文件%s失败:%v\n", fp, err)
			return err
		}
		err = os.Rename(tempName, finalName)
		if err != nil {
			log.Printf("重命名文件%s失败:%v\n", finalName, err)
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
	// 检查FFmpeg是否支持NVIDIA NVENC H.264编码器
	cmd := exec.Command("ffmpeg", "-encoders")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	// 查找h264_nvenc编码器
	return strings.Contains(string(output), "h264_nvenc")
}

func hasIntel() bool {
	// 检查FFmpeg是否支持Intel QSV H.264编码器
	cmd := exec.Command("ffmpeg", "-encoders")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	// 查找h264_qsv编码器
	return strings.Contains(string(output), "h264_qsv")
}

func hasAMD() bool {
	// 检查FFmpeg是否支持AMD VCE H.264编码器
	cmd := exec.Command("ffmpeg", "-encoders")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	// 查找h264_amf编码器
	return strings.Contains(string(output), "h264_amf")
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
