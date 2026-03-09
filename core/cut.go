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

func init() {
	// 初始化日志文件和配置
	util.SetLog("BitchCut.log")
	// 设置日志标志：包含文件名和行号
	log.SetFlags(2 | 16)
}

func Cut(root string) {
	ge := GracefullyExit.New()
	defer ge.Stop() // 程序结束时清理
	sqlite.SetSqlite()
	// 获取包含LLC文件的所有文件夹
	folders, _ := util.GetFoldersWithLLCFiles(root)
	if len(folders) == 0 {
		log.Fatalln("没有找到任何符合条件的文件")
	}
	// 遍历每个文件夹进行处理
	for _, folder := range folders {
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
			if ge.ShouldExit("q") {
				log.Println("Exit signal received. Quitting after current operation.")
				break
			} else {
				continue
			}
		} else {
			if err := os.RemoveAll(mp4); err != nil {
				log.Printf("删除%v失败\t%v\n", mp4, err)
			}
			if err := os.RemoveAll(llcFile); err != nil {
				log.Printf("删除%v失败\t%v\n", llcFile, err)
			}
			if ge.ShouldExit("q") {
				log.Println("Exit signal received. Quitting after current operation.")
				break
			} else {
				log.Printf("分割文件结束,删除%v成功\n", mp4)
			}
		}
	}
}
