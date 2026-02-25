package cue

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func init() {
	commands := []string{"shnsplit", "lame", "ffmpeg"}
	for _, cmd := range commands {
		if _, err := exec.LookPath(cmd); err != nil {
			switch runtime.GOOS {
			case "darwin":
				log.Fatalln("brew install shntool lame ffmpeg")
			case "linux":
				log.Fatalln("sudo apt install shntool lame", cmd, err)
			}
		}
	}
}

type disc struct {
	media string
	cue   string
}

type track struct {
	TITLE     string `json:"title"`
	PERFORMER string `json:"performer"`
	INDEX0    string `json:"index 00"`
	INDEX1    string `json:"index 01"`
}

/*
查找root目录和全部子目录下的所有cue和同名的音频文件(只有扩展名不同) 返回音频文件和cue文件的绝对路径(track结构体)切片
*/
func FindCue(root string) []disc {
	var tracks []disc
	// 支持的音频文件扩展名
	audioExts := []string{".flac", ".wav", ".ape", ".m4a", ".mp3"}
	// 遍历目录
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// 只处理 .cue 文件
		if !info.IsDir() && strings.ToLower(filepath.Ext(path)) == ".cue" {
			baseName := strings.TrimSuffix(path, ".cue")
			// 查找对应的音频文件
			for _, ext := range audioExts {
				mediaPath := baseName + ext
				if _, err := os.Stat(mediaPath); err == nil {
					// 找到匹配的音频文件，添加到结果中
					tracks = append(tracks, disc{
						media: mediaPath,
						cue:   path,
					})
					break
				}
			}
		}
		return nil
	})
	return tracks
}
