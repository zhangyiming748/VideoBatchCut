package ffmpeg

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func AnyVideoToMP4(fp string) error {
	if filepath.Ext(fp) == ".mp4" {
		tempName := strings.Replace(fp, ".mp4", "_tmp.mp4", 1)
		var cmd *exec.Cmd
		var args []string
		args = append(args, "-i", fp)
		if runtime.GOOS == "darwin" {
			args = append(args, "-c:v", "h264_videotoolbox")
			args = append(args, "-q:v", "50")
		} else if HasH264NVENC() {
			args = append(args, "-c:v", "h264_nvenc")
		} else {
			args = append(args, "-c:v", "libx265")
		}
		args = append(args, "-c:a", "aac")
		args = append(args, tempName)
		cmd = exec.Command("ffmpeg", args...)
		log.Printf("执行命令:%v\n", cmd.String())
		_, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatalf("ffmpeg快速处理文件%s失败:%v\n", fp, err)
		} else {
			err = os.Remove(fp)
			if err != nil {
				log.Fatalf("删除文件%s失败:%v\n", fp, err)
			}
			err = os.Rename(tempName, fp)
			if err != nil {
				log.Fatalf("重命名文件%s失败:%v\n", fp, err)
			}
		}
	} else {
		out := strings.Replace(fp, filepath.Ext(fp), ".mp4", 1)
		var cmd *exec.Cmd
		var args []string
		args = append(args, "-i", fp)
		if runtime.GOOS == "darwin" {
			args = append(args, "-c:v", "h264_videotoolbox")
			args = append(args, "-q:v", "50")
		} else if HasH264NVENC() {
			args = append(args, "-c:v", "h264_nvenc")
		} else {
			args = append(args, "-c:v", "libx265")
		}
		args = append(args, "-c:a", "aac")
		args = append(args, out)
		cmd = exec.Command("ffmpeg", args...)
		log.Printf("执行命令:%v\n", cmd.String())
		_, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatalf("ffmpeg快速处理文件%s失败:%v\n", fp, err)
		} else {
			err = os.Remove(fp)
			if err != nil {
				log.Fatalf("删除文件%s失败:%v\n", fp, err)
			}
		}
	}
	return nil
}
