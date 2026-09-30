package ffmpeg

import (
	"VideoBatchCut/util"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func AnyVideoToMP4(fp string) error {
	var (
		cmd       *exec.Cmd
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
	// 字幕预检：含 MP4 装不下的字幕（如 MKV 的 PGS）时直接跳过，
	// 不启动 ffmpeg、不动原文件，只返回错误由批量调用方打印后继续下一个。
	if util.HasIncompatibleSubtitles(fp, tempName) {
		return fmt.Errorf("跳过文件%s：含 MP4 容器无法封装的字幕流（如 PGS），未做任何转换", fp)
	}
	// 编码器及视频参数由 util/ffmpeg.go 结合 hwaccel.go 的硬件探测自动决定（NVIDIA / Intel / AMD / Apple / CPU），
	// 音频统一为高质量 Opus，无需再在此处逐硬件手写参数。
	job := util.NewJob(fp, tempName)
	cmd = job.Command()
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
