package ffmpeg

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func AnyVideoToMP4(fp string) error {
	if filepath.Ext(fp) == ".mp4" {
		tempName := strings.Replace(fp, ".mp4", "_tmp.mp4", 1)
		var cmd *exec.Cmd
		if HasH264NVENC() {
			cmd = exec.Command("ffmpeg", "-i", fp, "-c:v", "h264_nvenc", "-c:a", "aac", tempName)

		} else {
			cmd = exec.Command("ffmpeg", "-i", fp, "-c:v", "libx264", "-c:a", "aac", tempName)

		}
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
		cmd := exec.Command("ffmpeg", "-i", fp, "-c:v", "h264_nvenc", "-c:a", "aac", out)
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
