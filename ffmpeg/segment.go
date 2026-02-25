// Package ffmpeg 视频切割相关功能的实现
package ffmpeg

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"VideoBatchCut/sqlite"
	"VideoBatchCut/util"
)

// HasH264NVENC 检查ffmpeg是否支持h264_nvenc编码器
func HasH264NVENC() bool {
	// 使用更准确的方法检测 CUDA 硬件编码器支持
	cmd := exec.Command("nvidia-smi")
	_, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return true
}

// CutBySegments 根据给定的片段列表切割视频文件
// mp4: 输入视频文件路径
// segments: 切割片段列表
func CutBySegments(mp4 string, segments []util.Segment) error {
	for i, segment := range segments {
		// 将 i+1 转换为两位数的字符串，不足两位前面补0
		index := fmt.Sprintf("%02d", i+1)
		total := fmt.Sprintf("%02d", len(segments))
		// 构造输出文件名，格式为 "01.mp4"
		start := util.FormatSecondToHMS(segment.Start)
		end := util.FormatSecondToHMS(segment.End)
		// 调用 CutBySegment 函数进行切割
		if err := CutBySegment(index, total, mp4, start, end); err != nil {
			return fmt.Errorf("cut File: %s By Segment error: %v", mp4, err)
		}
	}
	return nil
}

// CutBySegment 执行单个视频片段的切割
// index: 输出文件的序号（两位数字）
// mp4: 输入视频文件路径
// start: 开始时间点
// end: 结束时间点
func CutBySegment(index, total, mp4, start, end string) error {
	out := filepath.Join(filepath.Dir(mp4), index+".mp4")
	cmd := exec.Command("ffmpeg")
	if HasH264NVENC() {
		cmd.Args = append(cmd.Args, "-hwaccel", "cuda")
	} else {
		//
	}
	cmd.Args = append(cmd.Args, "-i", mp4)
	// cmd.Args = append(cmd.Args, "-threads", "1")
	if start != "00:00:00.000" {
		cmd.Args = append(cmd.Args, "-ss", start)
	}
	if end != "00:00:00.000" {
		cmd.Args = append(cmd.Args, "-to", end)
	}
	if HasH264NVENC() {
		cmd.Args = append(cmd.Args, "-c:v", "h264_nvenc")
		cmd.Args = append(cmd.Args, "-preset", "slow")
		cmd.Args = append(cmd.Args, "-cq", "18")
	} else if fast := os.Getenv("FASTCUT"); fast == "yes" {
		cmd.Args = append(cmd.Args, "-c:v", "libx264")
	} else {
		cmd.Args = append(cmd.Args, "-c:v", "libx265")
		cmd.Args = append(cmd.Args, "-tag:v", "hvc1")
	}

	cmd.Args = append(cmd.Args, "-c:a", "aac")
	cmd.Args = append(cmd.Args, "-map_metadata", "-1")
	// vsync 0: 禁用视频同步，保持原始帧时戳
	cmd.Args = append(cmd.Args, "-vsync", "0")
	// 强制把负时间戳校正为 0，消除开头黑帧/不同步
	cmd.Args = append(cmd.Args, "-avoid_negative_ts", "make_zero")
	// 重新生成 PTS（presentation timestamp），忽略乱序 DTS，解决时间戳问题
	cmd.Args = append(cmd.Args, "-fflags", "+genpts+igndts")
	// 强制音视频同步，消除开头多余音频或结尾缺失音频
	cmd.Args = append(cmd.Args, "-af", "adelay=0|0, aresample=async=1")
	// copyts: 复制输入的时间戳到输出，保持时间戳连续性
	cmd.Args = append(cmd.Args, "-copyts")
	cmd.Args = append(cmd.Args, out)
	err := util.Exec(cmd)
	if err != nil {
		return err
	}

	bc := new(sqlite.BatchCut)
	bc.Sync()
	bc.Index = index
	bc.Total = total
	bc.FileName = mp4
	bc.Start = start
	bc.End = end
	err = bc.Insert()
	if err != nil {
		log.Fatalf("此次文件:%s分割成功但写入sqlite数据库失败:%v\n", mp4, err)
	}

	log.Printf("此次文件:%s分割成功并写入sqlite数据库成功\n", mp4)

	return nil
}
