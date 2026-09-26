package ffmpeg

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"VideoBatchCut/util"
)

var (
	OperatingSystem string
	Architecture    string
)

func init() {
	OperatingSystem = runtime.GOOS
	Architecture = runtime.GOARCH
}

/*
输入文件名和时间点切片
*/
func CutOne(fp string, timestamps []string) (err error) {
	defer func() {
		log.Println("运行完成")
	}()
	if timestamps[0] != "000000000" {
		newElement := "000000000"
		newSlice := append([]string{newElement}, timestamps...)
		timestamps = newSlice
	}
	if !IsValidate(timestamps) {
		return fmt.Errorf("给定的时间戳文件:%v格式非法", timestamps)
		//error strings should not end with punctuation or newlines (ST1005)
	}
	timestamps = formatTimestamps(timestamps)
	fname := fp
	//folder := strings.Split(fname, ".")[0]
	folder := strings.TrimSuffix(fname, filepath.Ext(fname))
	folder = strings.ToUpper(folder)
	_ = os.Mkdir(folder, 0777)
	length := len(timestamps)
	log.Printf("时间戳%v\n", timestamps)
	for i := 0; i < length-1; i++ {
		// 简化索引格式化逻辑
		index := fmt.Sprintf("%02d", i+1)
		mp4 := strings.Join([]string{index, "mp4"}, ".")
		mp4 = strings.Join([]string{folder, mp4}, string(os.PathSeparator))
		cmd := exec.Command("ffmpeg")
		//cmd.Args = append(cmd.Args, "-loglevel", "error")
		if util.HasNvidia() {
			cmd.Args = append(cmd.Args, "-hwaccel", "cuda")
			cmd.Args = append(cmd.Args, "-i", fname)
			cmd.Args = append(cmd.Args, "-ss", timestamps[i])
			cmd.Args = append(cmd.Args, "-to", timestamps[i+1])
			cmd.Args = append(cmd.Args, "-c:v", "h264_nvenc")
			cmd.Args = append(cmd.Args, "-c:a", "aac")
			cmd.Args = append(cmd.Args, "-preset", "slow")
			cmd.Args = append(cmd.Args, "-cq", "18")
			cmd.Args = append(cmd.Args, "-map_metadata", "-1")
			// -fps_mode passthrough 等价于旧的 -vsync 0；ffmpeg 8+ 已移除 -vsync（本机 9.0.1）
			cmd.Args = append(cmd.Args, "-fps_mode", "passthrough")
			cmd.Args = append(cmd.Args, "-copyts") // 添加这行
			cmd.Args = append(cmd.Args, mp4)
		} else if util.HasAppleSilicon() {
			// Apple Silicon：用 VideoToolbox 硬件 H.264 编码，画质对标高质量 libx265。
			// 关键点：
			//   -spatial_aq 1：开启空间自适应量化，把码率向大面积纯色/平坦区倾斜，专治“纯色块出方块”；
			//   -q:v 95：VideoToolbox 恒定质量档（1-100）。H.264 压缩效率低于 H.265，故取更高档；
			//            实测同一片源 90→637KB、95→1.4MB、100→3.1MB，95 足以对标高质量 libx265 又不至于像 100 那样翻倍；
			//   -profile:v high + -coder cabac：High Profile + CABAC 熵编码，压缩效率优于默认 CAVLC，同等画质更省码率、平坦区更干净；
			//   -allow_sw 1：硬件编码器被占用时回退软件 VideoToolbox，避免整批失败。
			cmd.Args = append(cmd.Args, "-i", fname)
			cmd.Args = append(cmd.Args, "-ss", timestamps[i])
			cmd.Args = append(cmd.Args, "-to", timestamps[i+1])
			cmd.Args = append(cmd.Args, "-c:v", "h264_videotoolbox")
			cmd.Args = append(cmd.Args, "-profile:v", "high")
			cmd.Args = append(cmd.Args, "-coder", "cabac")
			cmd.Args = append(cmd.Args, "-q:v", "95")
			cmd.Args = append(cmd.Args, "-spatial_aq", "1")
			cmd.Args = append(cmd.Args, "-allow_sw", "1")
			cmd.Args = append(cmd.Args, "-c:a", "aac")
			cmd.Args = append(cmd.Args, "-map_metadata", "-1")
			// -fps_mode passthrough 等价于旧的 -vsync 0（保持原始帧时戳）；
			// ffmpeg 8+ 已移除 -vsync（本机为 9.0.1），用新名以免“Unrecognized option 'vsync'”。
			cmd.Args = append(cmd.Args, "-fps_mode", "passthrough")
			cmd.Args = append(cmd.Args, "-copyts") // 添加这行
			cmd.Args = append(cmd.Args, mp4)
		} else {
			cmd.Args = append(cmd.Args, "-i", fname)
			cmd.Args = append(cmd.Args, "-ss", timestamps[i])
			cmd.Args = append(cmd.Args, "-to", timestamps[i+1])
			cmd.Args = append(cmd.Args, "-c:v", "libx265")
			cmd.Args = append(cmd.Args, "-tag:v", "hvc1")
			cmd.Args = append(cmd.Args, "-c:a", "aac")
			cmd.Args = append(cmd.Args, "-map_metadata", "-1")
			// -fps_mode passthrough 等价于旧的 -vsync 0；ffmpeg 8+ 已移除 -vsync（本机 9.0.1）
			cmd.Args = append(cmd.Args, "-fps_mode", "passthrough")
			cmd.Args = append(cmd.Args, "-copyts") // 添加这行
			cmd.Args = append(cmd.Args, mp4)
		}
		err = util.Exec(cmd)
		if err != nil {
			return err
		}
	}
	var last string
	if length < 10 {
		last = fmt.Sprintf("%02d", length)
	} else {
		last = fmt.Sprintf("%02d", length)
	}
	mp4 := strings.Join([]string{last, "mp4"}, ".")
	mp4 = strings.Join([]string{folder, mp4}, string(os.PathSeparator))
	var cmd *exec.Cmd
	// 复用 util.HasNvidia()：它用真实编码探测判断 h264_nvenc 是否可用，且结果带缓存，
	// 与前面循环分支保持一致，避免再靠硬编码主机名（DESKTOP-VGFTVD8）判断本机。
	if util.HasNvidia() {
		cmd = exec.Command("ffmpeg", "-hwaccel", "cuda", "-i", fname, "-ss", timestamps[length-1], "-c:v", "h264_nvenc", "-c:a", "aac", "-ac", "1", "-preset", "medium", "-cq", "20", "-progress", "pipe:1", mp4)
	} else if util.HasAppleSilicon() {
		// Apple Silicon：VideoToolbox 硬件 H.264，参数与循环分支保持一致（高画质档 + CABAC + 空间自适应量化，避免纯色块）
		cmd = exec.Command("ffmpeg", "-i", fname, "-ss", timestamps[length-1], "-c:v", "h264_videotoolbox", "-profile:v", "high", "-coder", "cabac", "-q:v", "95", "-spatial_aq", "1", "-allow_sw", "1", "-c:a", "aac", "-ac", "1", "-progress", "pipe:1", mp4)
	} else {
		cmd = exec.Command("ffmpeg", "-i", fname, "-ss", timestamps[length-1], "-c:v", "libx265", "-c:a", "aac", "-tag:v", "hvc1", "-ac", "1", "-progress", "pipe:1", mp4)
	}
	err = util.Exec(cmd)
	if err != nil {
		return err
	} else {
		log.Println("运行完成")
		if err := os.Remove(fname); err != nil {
			log.Printf("删除原文件失败:%v\n", err)
		}
	}
	return nil
}

func formatTimestamps(timestamps []string) []string {
	var formatted []string
	for _, ts := range timestamps {
		// 将字符串分割为小时、分钟、秒和毫秒
		hours := ts[0:2]
		minutes := ts[2:4]
		seconds := ts[4:6]
		milliseconds := ts[6:9]

		// 格式化为所需的格式
		formattedTimestamp := fmt.Sprintf("%s:%s:%s.%s", hours, minutes, seconds, milliseconds)
		formatted = append(formatted, formattedTimestamp)
	}
	return formatted
}

func IsValidate(timestamps []string) bool {
	var s []int
	for index, v := range timestamps {
		i, err := strconv.Atoi(v)
		if err != nil {
			log.Printf("可能包含非数字字符:%v\n", v)
			return false
		}
		if isNineDigitNumber(v) {
			//fmt.Printf("%s 是九位纯数字\n", v)
		} else {
			log.Printf("第%d行的数字有问题:%s不是九位纯数字\n", index+2, v)
			return false
		}
		s = append(s, i)
	}
	for i := 0; i < len(s)-1; i++ {
		if s[i] > s[i+1] {
			log.Printf("第%v行的数字有问题:%s\n", i+2, timestamps[i+1])
			return false
		}
	}
	return true
}

func isNineDigitNumber(str string) bool {
	match, _ := regexp.MatchString("^[0-9]{9}$", str)
	return match
}
