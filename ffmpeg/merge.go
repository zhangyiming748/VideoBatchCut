package ffmpeg

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"VideoBatchCut/util"
)

// MergeSubtitle 将 srtPath 的字幕内嵌进 mp4Path。
// 视频流、音频流直接复制（-c:v copy -c:a copy），不做任何重编码；
// SRT 是文本字幕，MP4 容器要求 mov_text，故字幕转为 mov_text（仅文本格式转换）。
// MP4 中原有的字幕流保留，新字幕作为追加的字幕流。
// 处理时先输出到临时文件，校验成功后再替换原 MP4；外部 SRT 文件不动。
func MergeSubtitle(mp4Path, srtPath string) error {
	if !strings.EqualFold(filepath.Ext(mp4Path), ".mp4") {
		return fmt.Errorf("%s 不是 mp4 文件", mp4Path)
	}
	tempName := strings.TrimSuffix(mp4Path, filepath.Ext(mp4Path)) + "_mergetmp.mp4"

	job := util.NewJob(mp4Path, tempName)
	// 视频/音频流直接复制
	job.Encoder = util.EncoderCopy
	job.Audio = util.AudioCopy()
	// 第二路输入为 SRT。PostInputArgs 中手写 -map 后 MapAllStreams 自动让位：
	// 保留 MP4 的视频、音频、原有字幕，再追加 SRT 的字幕。
	job.PostInputArgs = []string{
		"-i", srtPath,
		"-map", "0:v?",
		"-map", "0:a?",
		"-map", "0:s?",
		"-map", "1:s:0",
	}
	// 所有字幕（原有字幕 + 新 SRT）统一指定 mov_text 以封装进 MP4
	job.ExtraArgs = []string{"-c:s", "mov_text"}

	if err := job.Run(); err != nil {
		return err
	}

	// 校验输出文件是否有效
	fi, err := os.Stat(tempName)
	if err != nil {
		return fmt.Errorf("输出文件%s不存在: %w", tempName, err)
	}
	if fi.Size() == 0 {
		os.Remove(tempName)
		return fmt.Errorf("输出文件%s大小为0字节，合并失败，保留原文件", tempName)
	}

	// 替换原 MP4
	if err := os.Remove(mp4Path); err != nil {
		return fmt.Errorf("删除原文件%s失败: %w", mp4Path, err)
	}
	if err := os.Rename(tempName, mp4Path); err != nil {
		return fmt.Errorf("重命名%s为%s失败: %w", tempName, mp4Path, err)
	}
	log.Printf("文件%s内嵌字幕成功\n", mp4Path)
	return nil
}
