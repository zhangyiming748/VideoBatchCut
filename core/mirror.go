package core

import (
	"VideoBatchCut/util"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/zhangyiming748/FastMediaInfo"
	"github.com/zhangyiming748/finder"
)

// Mirror 遍历根目录下的所有视频，使用硬件加速在原文件相同位置生成一份 MP4 副本，
// 文件名在扩展名之前追加 "_mirror" 后缀，如 video.mkv -> video_mirror.mp4、
// video.mp4 -> video_mirror.mp4。
// 与其它子命令不同：镜像视频只供其他软件“打点”调试用，因此开启 Job 的极速档
// （EnableFastest）——只追求转换速度，不保证画质，也不保留音频（-an）、不处理字幕，
// 只映射视频流。全程不修改、不删除、不重命名原文件。编码器仍由 util.SelectEncoder
// 自动选择硬件加速（NVIDIA / Apple / Intel / AMD / Qualcomm），失败时极速回退 CPU。
func Mirror(root string) {
	videos := finder.FindAllVideos(root)
	for _, video := range videos {
		// 跳过镜像文件自身（严格以 _mirror.mp4 结尾，大小写不敏感），
		// 避免重复执行时把镜像当作原文件再套娃一层。
		if strings.HasSuffix(strings.ToLower(filepath.Base(video)), "_mirror.mp4") {
			continue
		}
		outName := strings.TrimSuffix(video, filepath.Ext(video)) + "_mirror.mp4"
		// 同级目录已存在对应的镜像文件，说明该原文件已处理完成，原文件与镜像一并跳过，
		// 不再重新转换（也避免对已处理文件做无谓的 MediaInfo 探测）。
		if _, err := os.Stat(outName); err == nil {
			log.Printf("镜像文件%s已存在，跳过原文件%s\n", outName, video)
			continue
		}
		// 再用 MediaInfo 判断视频编码：已是 AVC 或 HEVC 的直接跳过，
		// 不重新创建一个编码相同的镜像文件
		mi := FastMediaInfo.GetStandMediaInfo(video)
		if mi.Video.Format == "AVC" || mi.Video.Format == "HEVC" {
			log.Printf("文件%s已是%s编码，跳过镜像创建\n", video, mi.Video.Format)
			continue
		}
		// 极速档：速度优先、丢弃音频(-an)、只映射视频流，不做字幕预检/处理。
		job := util.NewJob(video, outName).EnableFastest()
		// 时间轴保真：镜像用于在其它软件中打点，而实际切割作用在原文件上。
		// 必须让镜像每一帧的 PTS 与原文件解码时间轴严格一一对应：
		//   - PassthroughFps(-fps_mode passthrough)：不丢帧、不补帧，
		//     默认 auto 模式会丢弃时间戳缺失/重复帧并可能把 VFR 源压成 CFR，
		//     实测会造成片头数帧丢失、start_time 偏移约 100ms；
		//   - CopyTimestamps(-copyts)：直接沿用原文件时间戳，不重新生成。
		// 否则镜像上标记的时间点与切割命令按原文件 PTS 定位的画面会发生偏移。
		job.PassthroughFps = true
		job.CopyTimestamps = true
		log.Printf("执行命令:%v\n", job.String())
		if err := job.Run(); err != nil {
			// 失败也继续处理下一个，原文件保持不动
			log.Printf("mirror转换文件%s失败:%v\n", video, err)
			continue
		}
		log.Printf("文件%s已镜像为%s\n", video, outName)
	}
}
