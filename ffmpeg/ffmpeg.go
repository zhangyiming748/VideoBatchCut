package ffmpeg

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"VideoBatchCut/util"
)

// 注释掉 OperatingSystem / Architecture 及 init() 的原因：
// 这两个全局变量在全项目中没有任何引用，硬件平台判断已收敛到 util/hwaccel.go 的
// HasNvidia/HasIntel/HasAMD/HasAppleSilicon/HasQualcomm 函数中，直接使用 runtime.GOOS/GOARCH。
// 保留这两个变量只会造成信息冗余和维护负担，故注释掉（不直接删除是为了留档，
// 万一后续有外部代码依赖可快速恢复）。
// var (
// 	OperatingSystem string
// 	Architecture    string
// )
//
// func init() {
// 	OperatingSystem = runtime.GOOS
// 	Architecture = runtime.GOARCH
// }

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
		// 精确切割 [timestamps[i], timestamps[i+1]]：编码器/音频/时间戳修复均由 util/ffmpeg.go 统一决定。
		if err := util.Cut(fname, mp4, timestamps[i], timestamps[i+1]); err != nil {
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
	// 最后一段：从 timestamps[length-1] 切到文件结尾（end 留空表示不限制），并输出机器可读进度。
	// 编码器自动选择（NVIDIA / Apple / Intel / AMD / CPU），与前面循环分支共用同一套参数。
	job := util.NewCutJob(fname, mp4, timestamps[length-1], "")
	job.Progress = true
	if err := job.Run(); err != nil {
		return err
	}
	log.Println("运行完成")
	if err := os.Remove(fname); err != nil {
		log.Printf("删除原文件失败:%v\n", err)
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
