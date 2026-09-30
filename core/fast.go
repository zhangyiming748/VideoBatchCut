package core

import (
	"VideoBatchCut/ffmpeg"
	"log"
	"path/filepath"

	"github.com/zhangyiming748/FastMediaInfo"
	"github.com/zhangyiming748/finder"
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
				// 失败（含字幕不兼容被跳过）只打印错误，不退出，继续处理下一个
				log.Printf("转换文件%s失败，跳过:%v\n", video, err)
				continue
			}
		}

	}
}

/*
DJI录制的视频专用
找到mp4视频
使用ffmpeg替换音轨为指定的mp3文件循环播放直到视频结束
*/
func DJI(root, audioPath string) {
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
			if err := ffmpeg.ForDji(video, audioPath); err != nil {
				continue
			}
		}

	}
}
