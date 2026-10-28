package core

import (
	"VideoBatchCut/util"
	"fmt"
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
// force 为 true 时（-f/--force），跳过 AVC/HEVC 编码检测，无论原视频是什么编码
// 都重新生成镜像；默认 false，保持跳过 AVC/HEVC 的行为。force 同时会忽略已存在的
// 配对镜像文件并重新生成（ffmpeg -y 覆盖）。
// 目录级过滤：存在 -proj.llc 的目录、以及无子目录且包含多个视频的目录（视为已完成
// 分割）整体跳过；单个视频只有在同目录存在同名 _mirror.mp4 配对时才跳过。
func Mirror(root string, force bool) {
	videos := finder.FindAllVideos(root)
	// videosByDir 统计每个目录下的视频数量（排除镜像文件自身），
	// 用于判断叶子目录是否已经分割完成。
	videosByDir := make(map[string]int)
	for _, video := range videos {
		if strings.HasSuffix(strings.ToLower(filepath.Base(video)), "_mirror.mp4") {
			continue
		}
		videosByDir[filepath.Dir(video)]++
	}
	// dirChecked 缓存每个目录的整体跳过原因，同一目录下可能有多个视频，只 ReadDir 一次。
	dirChecked := make(map[string]string)
	// dirLogged 记录已经打印过跳过日志的目录，避免目录下每个视频都重复打印一遍。
	dirLogged := make(map[string]bool)
	skipReason := func(dir string) string {
		if reason, ok := dirChecked[dir]; ok {
			return reason
		}
		reason := ""
		entries, err := os.ReadDir(dir)
		if err == nil {
			hasSubDir := false
			hasProjLLC := false
			for _, e := range entries {
				if e.IsDir() {
					hasSubDir = true
					continue
				}
				if strings.HasSuffix(strings.ToLower(e.Name()), "-proj.llc") {
					hasProjLLC = true
				}
			}
			switch {
			case hasProjLLC:
				// 同目录存在 -proj.llc 打点工程文件，说明该视频已生成过镜像并完成打点，
				// 用户随后删掉了镜像只保留原视频，整个文件夹都不再处理。
				reason = "存在打点工程文件，视为已处理"
			case !hasSubDir && videosByDir[dir] >= 2:
				// 最终文件夹（无更深一层子文件夹）已经包含多个视频，说明该目录的视频已完成分割。
				// 注意：如果分割只产出一段，目录里只剩一个视频，无法靠数量识别；
				// 这种情况依赖同目录保留的 -proj.llc 兜底。
				reason = fmt.Sprintf("无子目录且包含%d个视频，视为已分割完成", videosByDir[dir])
			}
		} else {
			log.Printf("读取目录%s失败:%v\n", dir, err)
		}
		dirChecked[dir] = reason
		return reason
	}
	for _, video := range videos {
		// 跳过镜像文件自身（严格以 _mirror.mp4 结尾，大小写不敏感），
		// 避免重复执行时把镜像当作原文件再套娃一层。
		if strings.HasSuffix(strings.ToLower(filepath.Base(video)), "_mirror.mp4") {
			continue
		}
		dir := filepath.Dir(video)
		if reason := skipReason(dir); reason != "" {
			// 目录级跳过只打印一次，避免一个已分割目录下的每个视频都刷一条日志。
			if !dirLogged[dir] {
				log.Printf("目录%s%s，跳过该目录下的视频\n", dir, reason)
				dirLogged[dir] = true
			}
			continue
		}
		// 镜像配对判断：只有“同一个文件名 + _mirror.mp4”同时存在，才算该视频已生成镜像。
		// 不能因为目录里存在任意 _mirror.mp4 就跳过整个目录，否则同目录下尚未镜像的
		// 其它视频（例如 AB 两个片段）会被漏掉。
		outName := strings.TrimSuffix(video, filepath.Ext(video)) + "_mirror.mp4"
		// force 模式视为强制重建，即使配对镜像已存在也重新生成（ffmpeg -y 覆盖）。
		if !force {
			if _, err := os.Stat(outName); err == nil {
				log.Printf("镜像文件%s已存在，跳过原文件%s\n", outName, video)
				continue
			}
		}
		// 再用 MediaInfo 判断视频编码：已是 AVC 或 HEVC 的直接跳过，
		// 不重新创建一个编码相同的镜像文件；force 模式下跳过该检测，
		// 无论什么编码都重新生成镜像。
		if !force {
			mi := FastMediaInfo.GetStandMediaInfo(video)
			if mi.Video.Format == "AVC" || mi.Video.Format == "HEVC" {
				log.Printf("文件%s已是%s编码，跳过镜像创建\n", video, mi.Video.Format)
				continue
			}
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
