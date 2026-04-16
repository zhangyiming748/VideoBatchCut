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
		cmd      *exec.Cmd
		args     []string
		tempName string
	)
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
	if filepath.Ext(fp) == ".mp4" {
		tempName = strings.Replace(fp, ".mp4", "_tmp.mp4", 1)
		args = append(args, tempName)
	} else {
		tempName = strings.Replace(fp, filepath.Ext(fp), ".mp4", 1)
		args = append(args, tempName)
	}
	cmd = exec.Command("ffmpeg", args...)
	log.Printf("执行命令:%v\n", cmd.String())
	_, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("ffmpeg快速处理文件%s失败:%v\n", fp, err)
	}

	// 命令执行成功后处理文件
	if filepath.Ext(fp) == ".mp4" {
		// 对于mp4文件，将临时文件重命名为原文件名
		err = os.Rename(tempName, fp)
		if err != nil {
			log.Fatalf("重命名文件%s失败:%v\n", tempName, err)
		}
	} else {
		// 对于其他格式文件，删除原文件
		err = os.Remove(fp)
		if err != nil {
			log.Fatalf("删除文件%s失败:%v\n", fp, err)
		}
	}
	return nil
}
