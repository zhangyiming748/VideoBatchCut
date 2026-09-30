package core

import (
	"VideoBatchCut/util"
	"log"
	"path/filepath"
	"strings"

	"github.com/zhangyiming748/finder"
)

// Fix 遍历根目录下的所有视频，无论扩展名与编码格式，一律强制重编码为 MP4 容器的 H264。
// 编码器由 util.SelectEncoder 结合 hwaccel.go 的真实编码探测自动选择
// （NVIDIA / Apple / Intel / AMD，均不可用时回退 CPU 软编）。
// 与 AnyVideoToMP4 不同：无论转换成功还是失败，原文件一律保留、不删除不重命名；
// 新文件在扩展名之前追加 "duplicate" 后缀，如 video.mkv -> video.duplicate.mp4、
// video.mp4 -> video.duplicate.mp4。
// 转换前先做字幕预检（util.HasIncompatibleSubtitles）：含 MP4 无法封装的图形字幕
// （PGS/VobSub）时打印提示并跳过该文件，而不是让 ffmpeg 转换失败。
func Fix(root string) {
	videos := finder.FindAllVideos(root)
	for _, video := range videos {
		// 跳过已带 duplicate 后缀的文件，避免重复执行 fix 时产生 .duplicate.duplicate.mp4
		if strings.Contains(filepath.Base(video), ".duplicate") {
			continue
		}
		outName := strings.TrimSuffix(video, filepath.Ext(video)) + ".duplicate.mp4"
		// 字幕预检：含 MP4 无法封装的图形字幕（PGS/VobSub）则跳过该文件，
		// 避免 ffmpeg 转换中途失败。与 util.Convert 的预检逻辑一致。
		if util.HasIncompatibleSubtitles(video, outName) {
			log.Printf("fix跳过文件%s：含 MP4 无法封装的字幕流\n", video)
			continue
		}
		// NewJob 默认：自动硬件编码 + 高质量 Opus 音频 + 覆盖输出
		job := util.NewJob(video, outName)
		log.Printf("执行命令:%v\n", job.String())
		if err := job.Run(); err != nil {
			// 失败也继续处理下一个，原文件保持不动
			log.Printf("fix转换文件%s失败:%v\n", video, err)
			continue
		}
		log.Printf("文件%s已转换为%s\n", video, outName)
	}
}
