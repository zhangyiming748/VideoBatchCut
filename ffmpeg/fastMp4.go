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

	cmd = exec.Command("ffmpeg", args...)
	log.Printf("执行命令:%v\n", cmd.String())
	_, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("ffmpeg快速处理文件%s失败:%v\n", fp, err)
	}

	// 如果原文件是MP4，需要删除原文件并重命名
	if ext == ".mp4" {
		err = os.Remove(fp)
		if err != nil {
			log.Fatalf("删除文件%s失败:%v\n", fp, err)
		}
		err = os.Rename(tempName, finalName)
		if err != nil {
			log.Fatalf("重命名文件%s失败:%v\n", finalName, err)
		}
	}

	return nil
}

/*
ffmpeg -i video.mp4 -stream_loop -1 -i audio.mp3 -c:v h264_nvenc -c:a aac -b:a 192k -map 0:v:0 -map 1:a:0 -shortest output.mp4
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
	args = append(args, "-c:v", "h264_nvenc")
	args = append(args, "-c:a", "aac")
	args = append(args, "-b:a", "192k")
	args = append(args, "-map", "0:v:0")
	args = append(args, "-map", "1:a:0")
	args = append(args, "-shortest")
	args = append(args, tempName)
	cmd = exec.Command("ffmpeg", args...)
	log.Printf("执行命令:%v\n", cmd.String())
	_, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("ffmpeg快速处理文件%s失败:%v\n", videoPath, err)
	} else {
		err = os.Remove(videoPath)
		if err != nil {
			log.Fatalf("删除文件%s失败:%v\n", videoPath, err)
		}
		err = os.Rename(tempName, finalName)
		if err != nil {
			log.Fatalf("重命名文件%s失败:%v\n", finalName, err)
		}
	}
	return nil
}
