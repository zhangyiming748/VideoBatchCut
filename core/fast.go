package core

import (
	"VideoBatchCut/ffmpeg"
	"github.com/zhangyiming748/FastMediaInfo"
	"github.com/zhangyiming748/GracefullyExit"
	"github.com/zhangyiming748/finder"
	"log"
	"os"
	"path/filepath"
)

func FastMP4(root string) {
	folders := finder.FindAllFolders(root)
	for _, folder := range folders {
		videos := finder.FindAllVideosInRoot(folder)
		if len(videos) > 1 {
			log.Printf("警告 根文件夹:%s下包含多个视频\n", folder)
		}
		for _, video := range videos {
			mi := FastMediaInfo.GetStandMediaInfo(video)
			if filepath.Ext(video) == ".mp4" {
				if mi.Video.Format == "AVC" || mi.Video.Format == "HEVC" {
					continue
				}
			}
			if err := ffmpeg.AnyVideoToMP4(video); err != nil {
				continue
			}
		}
		if GracefullyExit.ShouldExit() {
			log.Println("Exit signal received. Quitting after current operation.")
			os.Exit(0)
		}
	}
}
