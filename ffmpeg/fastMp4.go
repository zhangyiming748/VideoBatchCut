package ffmpeg

import (
	"VideoBatchCut/util"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func AnyVideoToMP4(fp string) error {
	var (
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
	// 字幕预检：含 MP4 装不下的字幕（如 MKV 的 PGS）时直接跳过，
	// 不启动 ffmpeg、不动原文件，只返回错误由批量调用方打印后继续下一个。
	if bad, err := util.IncompatibleSubtitles(fp, tempName); err != nil {
		return fmt.Errorf("字幕预检失败 %s: %w", fp, err)
	} else if len(bad) > 0 {
		return fmt.Errorf("跳过文件%s：含 MP4 容器无法封装的字幕流 %v（如 PGS），未做任何转换", fp, bad)
	}
	// 编码器及视频参数由 util/ffmpeg.go 结合 hwaccel.go 的硬件探测自动决定
	// （NVIDIA / Intel / AMD / Apple / Qualcomm / CPU），音频统一为高质量 Opus。
	job := util.NewJob(fp, tempName)
	if err := job.Run(); err != nil {
		return err
	}
	log.Printf("文件%s处理成功\n", fp)

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
		err := os.Rename(tempName, finalName)
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
		err := os.Remove(fp)
		if err != nil {
			log.Printf("删除文件%s失败:%v\n", fp, err)
			return err
		}
	}

	return nil
}

// ForDji 为 DJI 录制视频替换音轨：将原视频音轨替换为指定音频文件循环播放，直到视频结束。
// 编码器/音频/流映射统一走 util.Job 模型，与项目其它路径保持一致，
// 不再硬编码 NVENC 参数（在非 N 卡机器上也能自动回退到可用硬件或 CPU）。
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

	job := util.NewJob(videoPath, tempName)
	// 第二路输入：循环播放的音频，直到视频结束
	// -map 放在 PostInputArgs 会被 hasMapFlag 检测到，从而跳过自动全流映射，
	// 只保留我们指定的视频（输入0）和音频（输入1）。
	job.PostInputArgs = []string{
		"-stream_loop", "-1", "-i", audioPath,
		"-map", "0:v:0",
		"-map", "1:a:0",
	}
	// 以最短流（视频）为准结束；循环音频可能导致 muxing 队列积压，加大缓冲区。
	job.ExtraArgs = []string{"-shortest", "-max_muxing_queue_size", "9999"}

	if err := job.Run(); err != nil {
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

	if err := os.Remove(videoPath); err != nil {
		log.Printf("删除文件%s失败:%v\n", videoPath, err)
		return err
	}
	if err := os.Rename(tempName, finalName); err != nil {
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
