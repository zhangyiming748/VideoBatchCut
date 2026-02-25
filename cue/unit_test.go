package cue

import (
	"VideoBatchCut/util"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func init() {
	util.SetLog("cue.log")
}

// go test -v -timeout 10h -run TestFindAll
func TestFindAll(t *testing.T) {
	files := FindCue("/Users/zen/Downloads/古典百分百专辑")
	for _, file := range files {
		fmt.Printf("media = %s\ncue = %s\n", file.media, file.cue)
		media := file.media
		cue := file.cue
		// shnsplit -f your_cue_file.cue -t "%n - %t" your_flac_file.flac
		// shnsplit -f your_cue_file.cue -t "%n - %t" your_flac_file.flac -o "wav lame -b 320 %f -"
		cmd := exec.Command("shnsplit", "-f", cue, "-t", "%n - %t", media)
		util.Exec(cmd)
	}

	// 添加匿名函数实现
	func() {
		// 获取当前目录下的所有文件
		files, err := filepath.Glob("*.wav")
		if err != nil {
			t.Errorf("读取WAV文件失败: %v", err)
			return
		}

		// 遍历处理每个WAV文件
		for _, wavFile := range files {
			// 构建输出文件名（保持相同文件名，仅改扩展名）
			mp3File := strings.TrimSuffix(wavFile, ".wav") + ".mp3"

			// 构建ffmpeg命令
			cmd := exec.Command("ffmpeg",
				"-i", wavFile, // 输入文件
				"-c:a", "libmp3lame", // 使用AAC编码器
				"-b:a", "320k", // 设置比特率
				"-ar", "44100", // 设置采样率
				"-ac", "2", // 设置声道数
				mp3File, // 输出文件
			)

			// 执行命令
			if err := util.Exec(cmd); err != nil {
				t.Errorf("转换文件 %s 失败: %v", wavFile, err)
			}
		}
	}()
}

// go test -v -timeout 10h -run TestConvertWAV
func TestConvertWAV(t *testing.T) {
	mp3Files, err := Wav2Mp3("/Users/zen/Downloads", false)
	if err != nil {
		log.Fatal(err)
	}
	for _, mp3File := range mp3Files {
		log.Println(mp3File)
	}
}
