package cue

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// 获取指定目录下的所有wav文件
func findWavFiles(root string) ([]string, error) {
	log.Printf("开始在目录 %s 中查找WAV文件", root)
	var wavFiles []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("遍历目录时出错: %v", err)
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".wav" {
			log.Printf("找到WAV文件: %s", path)
			wavFiles = append(wavFiles, path)
		}
		return nil
	})
	log.Printf("WAV文件查找完成，共找到 %d 个文件", len(wavFiles))
	return wavFiles, err
}

// 将单个wav文件转换为mp3
func convertToMp3(wavFile string) error {
	mp3File := strings.TrimSuffix(wavFile, ".wav") + ".mp3"
	log.Printf("开始转换文件: %s -> %s", wavFile, mp3File)

	cmd := exec.Command("ffmpeg",
		"-i", wavFile,
		"-c:a", "libmp3lame",
		"-b:a", "320k",
		"-ar", "44100",
		"-ac", "2",
		mp3File,
	)

	log.Printf("执行命令: %s", cmd.String())
	if err := cmd.Run(); err != nil {
		log.Printf("转换失败: %v", err)
		return err
	}
	log.Printf("转换成功: %s", mp3File)
	return nil
}

// 主函数：处理wav到mp3的转换
func Wav2Mp3(inputDir string, deleteOriginal bool) ([]string, error) {
	log.Printf("开始处理目录: %s, 是否删除原文件: %v", inputDir, deleteOriginal)

	wavFiles, err := findWavFiles(inputDir)
	if err != nil {
		log.Printf("查找WAV文件失败: %v", err)
		return nil, err
	}
	log.Printf("找到 %d 个WAV文件待处理", len(wavFiles))

	var mp3Files []string
	var successCount, failCount int

	for i, wavFile := range wavFiles {
		log.Printf("处理第 %d/%d 个文件: %s", i+1, len(wavFiles), wavFile)

		if err := convertToMp3(wavFile); err != nil {
			log.Printf("转换失败 %s: %v", wavFile, err)
			failCount++
			continue
		}

		mp3File := strings.TrimSuffix(wavFile, ".wav") + ".mp3"
		mp3Files = append(mp3Files, mp3File)
		successCount++

		if deleteOriginal {
			log.Printf("准备删除原始文件: %s", wavFile)
			if err := deleteWavFile(wavFile); err != nil {
				log.Printf("删除原始文件失败 %s: %v", wavFile, err)
			} else {
				log.Printf("成功删除原始文件: %s", wavFile)
			}
		}
	}

	log.Printf("转换完成。成功: %d, 失败: %d, 总计: %d",
		successCount, failCount, len(wavFiles))
	return mp3Files, nil
}

// 删除wav文件
func deleteWavFile(wavFile string) error {
	log.Printf("删除文件: %s", wavFile)
	if err := os.Remove(wavFile); err != nil {
		log.Printf("删除失败: %v", err)
		return err
	}
	log.Printf("删除成功: %s", wavFile)
	return nil
}
