package core

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"VideoBatchCut/ffmpeg"

	"github.com/zhangyiming748/GracefullyExit"
	"github.com/zhangyiming748/finder"
)

/*
Merge 遍历根目录下的每一个文件夹，把同一文件夹内同名的 MP4 和 SRT 配对，
在视频流、音频流直接复制的前提下，将 SRT 内嵌进 MP4。

配对严格限定在“同一个文件夹”内：A/01.mp4 只会与 A/01.srt 配对，
不会与 B/01.srt 混淆。每个文件夹独立处理，互不影响。

内嵌成功后删除外部 SRT；内嵌失败时保留 SRT 以便排查或重试。
*/
func Merge(root string) {
	folders := finder.FindAllFolders(root)
	for _, folder := range folders {
		mp4s, srts := collectPairs(folder)
		for name, mp4Path := range mp4s {
			srtPath, ok := srts[name]
			if !ok {
				log.Printf("文件夹%s中的%s没有同名 srt，跳过\n", folder, name)
				continue
			}
			if err := ffmpeg.MergeSubtitle(mp4Path, srtPath); err != nil {
				// 内嵌失败：保留外部 SRT，方便排查或重试
				log.Printf("文件夹%s中%s合并字幕失败，跳过并保留srt: %v\n", folder, name, err)
				continue
			}
			// 内嵌成功：删除外部 SRT；删除失败仅记录日志，不影响已完成的内嵌结果
			if err := os.Remove(srtPath); err != nil {
				log.Printf("删除字幕文件%s失败: %v\n", srtPath, err)
			}
			if GracefullyExit.ShouldExit() {
				log.Println("收到退出信号，结束任务")
				os.Exit(0)
			}
		}
	}
	log.Println("字幕内嵌任务全部完成")
}

// collectPairs 收集指定文件夹“直接包含”（不递归子文件夹）的 mp4 和 srt 文件，
// 以不含扩展名的文件名为键。扩展名大小写不敏感。
// 同一文件夹内出现多个同名同类文件时打印警告，以先出现的为准。
func collectPairs(folder string) (mp4s map[string]string, srts map[string]string) {
	mp4s = make(map[string]string)
	srts = make(map[string]string)
	entries, err := os.ReadDir(folder)
	if err != nil {
		log.Printf("读取文件夹%s失败: %v\n", folder, err)
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(folder, entry.Name())
		ext := filepath.Ext(entry.Name())
		name := strings.TrimSuffix(entry.Name(), ext)
		switch {
		case strings.EqualFold(ext, ".mp4"):
			if exist, dup := mp4s[name]; dup {
				log.Printf("警告 文件夹%s中存在多个同名mp4: %s 和 %s\n", folder, exist, fullPath)
				continue
			}
			mp4s[name] = fullPath
		case strings.EqualFold(ext, ".srt"):
			if exist, dup := srts[name]; dup {
				log.Printf("警告 文件夹%s中存在多个同名srt: %s 和 %s\n", folder, exist, fullPath)
				continue
			}
			srts[name] = fullPath
		}
	}
	return
}
