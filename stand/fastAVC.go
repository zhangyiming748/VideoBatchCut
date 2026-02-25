package stand

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/zhangyiming748/FastMediaInfo"
	"github.com/zhangyiming748/finder"
)

/*
使用cuda加速把
1. 非mp4视频
2. mp4格式但不是h264编码的视频
3. mp4格式的但不是h265编码视频
使用h264_nvenc编码器快速转换为h264编码
*/
func FastAVC(root string) {
	folders := finder.FindAllFolders(root)
	for i, folder := range folders {
		files := finder.FindAllVideosInRoot(folder)
		for j, file := range files {
			log.Printf("处理第%d/%d个文件夹%s中的第%d/%d个文件%s", i+1, len(folder), folder, j+1, len(files), file)
			if filepath.Ext(file) == ".mp4" {
				// 这里只有是h264和不是两种可能
				mi := FastMediaInfo.GetStandMediaInfo(file)
				if mi.Video.Format == "AVC" || mi.Video.CodecID == "avc1" {
					log.Printf("h264编码的mp4文件:%s\n", file)
				} else if mi.Video.Format == "HEVC" || (mi.Video.CodecID == "hev1" || mi.Video.CodecID == "hvc1") {
					log.Printf("h265编码的mp4文件:%s\n", file)
				} else {
					log.Printf("其他编码的mp4文件:%s\n", file)
					tmp_name := strings.Replace(file, filepath.Ext(file), "_tmp", 1)
					tmp_name = strings.Join([]string{tmp_name, "mp4"}, ".")
					cmd := exec.Command("ffmpeg", "-i", file, "-c:v", "h264_nvenc", "-c:a", "aac", "-map_chapters", "-1", tmp_name)
					log.Printf("执行命令:%s\n", cmd.String())
					if output, err := cmd.CombinedOutput(); err != nil {
						log.Printf("执行命令%v失败:%v\n%v", cmd.String(), err, string(output))
						// 失败的话删除临时文件
						os.Remove(tmp_name)
					} else {
						log.Printf("转换%s成功:%s\n", file, string(output))
						//准备删除源文件并把临时文件重命名为源文件
						if err := os.Remove(file); err != nil {
							log.Printf("删除源文件%s失败:%v", file, err)
						} else {
							if err := os.Rename(tmp_name, file); err != nil {
								log.Printf("重命名临时文件%s失败:%v", tmp_name, err)
							}
						}
					}
				}
			} else {
				//无论如何都要处理为h264编码的mp4文件
				log.Printf("其他编码的非mp4文件:%s\n", file)
				tmp_name := strings.Replace(file, filepath.Ext(file), "_tmp", 1)
				tmp_name = strings.Join([]string{tmp_name, "mp4"}, ".")
				cmd := exec.Command("ffmpeg", "-i", file, "-c:v", "h264_nvenc", "-c:a", "aac", "-map_chapters", "-1", tmp_name)
				log.Printf("执行命令:%s\n", cmd.String())
				if output, err := cmd.CombinedOutput(); err != nil {
					log.Printf("执行命令%v失败:%v\n%v", cmd.String(), err, string(output))
					// 失败的话删除临时文件
					os.Remove(tmp_name)
				} else {
					log.Printf("转换%s成功:%s\n", file, string(output))
					//准备删除源文件并把临时文件重命名为源文件
					if err := os.Remove(file); err != nil {
						log.Printf("删除源文件%s失败:%v", file, err)
					} else {
						file_name := strings.Replace(file, filepath.Ext(file), ".mp4", 1)
						if err := os.Rename(tmp_name, file_name); err != nil {
							log.Printf("重命名临时文件%s失败:%v", tmp_name, err)
						}
					}
				}
			}
		}
	}
}
