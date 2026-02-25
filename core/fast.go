package core

import (
	"VideoBatchCut/ffmpeg"
	"github.com/zhangyiming748/FastMediaInfo"
	"github.com/zhangyiming748/finder"
	"path/filepath"
)

func FastMP4(root string) {
	folders := finder.FindAllFolders(root)
	for _, folder := range folders {
		videos := finder.FindAllVideosInRoot(folder)
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
	}
}
