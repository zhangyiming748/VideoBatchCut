// 程序入口点，用于批量处理视频切割任务
package core

import (
	"fmt"
	"log"
	"os"

	"github.com/zhangyiming748/GracefullyExit"
	"github.com/zhangyiming748/finder"

	"VideoBatchCut/ffmpeg"
	"VideoBatchCut/sqlite"
	"VideoBatchCut/util"
)

func Cut(root string) {
	sqlite.SetSqlite()
	// 获取包含LLC文件的所有文件夹
	folders, _ := util.GetFoldersWithLLCFiles(root)
	if len(folders) == 0 {
		log.Fatalln("没有找到任何符合条件的文件")
	}
	// 遍历每个文件夹进行处理
	for _, folder := range folders {
		if GracefullyExit.ShouldExit() {
			log.Println("Exit signal received. Quitting after current operation.")
			os.Exit(0)
		}
		fmt.Printf("for遍历到的文件夹:%v\n", folder)
		llcFile, has := util.FindProjLLCFile(folder)
		if !has {
			log.Println("未找到文件")
			continue
		}
		log.Printf("找到的工程文件:%v\n", llcFile)
		videos := finder.FindAllVideosInRoot(folder)
		if len(videos) > 1 {
			log.Printf("跳过包含多个视频,可能是分割后的文件夹%v\n", folder)
			continue
		}
		if len(videos) == 0 {
			log.Printf("跳过没有视频的文件夹%v\n", folder)
			continue
		}
		mp4 := videos[0]
		log.Printf("找到的视频文件:%v\n", mp4)
		segments, err := util.ParseSegments(llcFile)
		if err != nil {
			log.Printf("解析%v失败:%v\n", llcFile, err)
			continue
		}
		log.Printf("目录%v\t文件%v共有%d章节\n", folder, mp4, len(segments))
		if err = ffmpeg.CutBySegments(mp4, segments); err != nil {
			log.Printf("%v\n", err)
		} else {
			// 切割前文件夹里只有 1 个视频，切割成功后必然多出多个片段文件。
			// 通过切割后视频数量是否严格增加来判断是否真的切出了片段，
			// 避免 ffmpeg 静默失败（退出码 0 但未生成产物）时误删原文件。
			// 注：finder.FindAllVideosInRoot 通过文件头识别视频，空文件/损坏文件不会被计入，
			// 因此能同时过滤"片段没生成"和"片段是空文件"两种失败情况。
			afterVideos := finder.FindAllVideosInRoot(folder)
			if len(afterVideos) <= len(videos) {
				log.Printf("警告: 切割 %s 后文件夹内视频数量未增加（切割前:%d 切割后:%d），片段可能未生成，保留原文件\n", mp4, len(videos), len(afterVideos))
				continue
			}
			if err := os.RemoveAll(mp4); err != nil {
				log.Printf("删除%v失败\t%v\n", mp4, err)
			}
			if err := os.RemoveAll(llcFile); err != nil {
				log.Printf("删除%v失败\t%v\n", llcFile, err)
			}
		}
	}
}
