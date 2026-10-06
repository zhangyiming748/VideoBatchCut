// Package ffmpeg 视频切割相关功能的实现
package ffmpeg

import (
	"VideoBatchCut/util"
	"fmt"
	"log"
	"path/filepath"
)

// CutBySegments 根据给定的片段列表切割视频文件
// mp4: 输入视频文件路径
// segments: 切割片段列表
func CutBySegments(mp4 string, segments []util.Segment) error {
	// 补零宽度由分段总数决定：不超过99补一位（两位宽度），超过99但不超过999补两位（三位宽度）
	width := 2
	if len(segments) > 99 {
		width = 3
	}
	for i, segment := range segments {
		// 按统一宽度补零，例如总数99时为 01、02，总数超过99时为 001、002
		index := fmt.Sprintf("%0*d", width, i+1)
		total := fmt.Sprintf("%0*d", width, len(segments))
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
	// "00:00:00.000" 表示该端不限制；Job 只按是否为空决定加不加 -ss/-to，故转成空串。
	if start == "00:00:00.000" {
		start = ""
	}
	if end == "00:00:00.000" {
		end = ""
	}
	// 精确切割：自动硬件编码 + 高质量 Opus 音频 + 时间戳/同步修复，参数由 util/ffmpeg.go 统一决定。
	job := util.NewCutJob(mp4, out, start, end)
	// 在 A/V 重同步基础上追加“真下混为立体声”，供 libopus 立体声编码（勿用 pan，见 util.AudioFilterStereoDownmix 注释）。
	job.AudioFilter = util.AudioFilterSync + ", " + util.AudioFilterStereoDownmix
	if err := job.Run(); err != nil {
		return err
	}
	log.Printf("此次文件:%s分割成功并写入sqlite数据库成功\n", mp4)
	return nil
}
