package core

import (
	"VideoBatchCut/ffmpeg"
	"VideoBatchCut/util"
	"log"
	"os"
	"path/filepath"
	"strings"

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

// Mirror 遍历根目录下的所有视频，使用硬件加速在原文件相同位置生成一份 MP4 副本，
// 文件名在扩展名之前追加 "_mirror" 后缀，如 video.mkv -> video_mirror.mp4、
// video.mp4 -> video_mirror.mp4。
// 与 FastMP4 不同：全程不修改、不删除、不重命名原文件，仅用于生成供其他软件
// 打断点调试用的镜像文件。编码器由 util.SelectEncoder 自动选择硬件加速
// （NVIDIA / Apple / Intel / AMD / Qualcomm），不可用时回退 CPU。
func Mirror(root string) {
	videos := finder.FindAllVideos(root)
	for _, video := range videos {
		// 跳过已带 _mirror 后缀的文件，避免重复执行时套娃
		if strings.Contains(filepath.Base(video), "_mirror") {
			continue
		}
		outName := strings.TrimSuffix(video, filepath.Ext(video)) + "_mirror.mp4"
		// 已存在镜像文件时跳过，避免重复转换
		if _, err := os.Stat(outName); err == nil {
			log.Printf("镜像文件%s已存在，跳过\n", outName)
			continue
		}
		// 字幕预检：含 MP4 无法封装的图形字幕（PGS/VobSub）则跳过该文件
		if bad, err := util.IncompatibleSubtitles(video, outName); err != nil {
			log.Printf("mirror字幕预检失败，跳过%s：%v\n", video, err)
			continue
		} else if len(bad) > 0 {
			log.Printf("mirror跳过文件%s：含 MP4 无法封装的字幕流 %v\n", video, bad)
			continue
		}
		// NewJob 默认：自动硬件编码 + 高质量 Opus 音频 + 覆盖输出
		job := util.NewJob(video, outName)
		log.Printf("执行命令:%v\n", job.String())
		if err := job.Run(); err != nil {
			// 失败也继续处理下一个，原文件保持不动
			log.Printf("mirror转换文件%s失败:%v\n", video, err)
			continue
		}
		log.Printf("文件%s已镜像为%s\n", video, outName)
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
