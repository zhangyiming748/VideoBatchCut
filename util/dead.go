// 本文件集中收纳当前“无正式调用方”的历史/预留函数（疑似死代码）。
//
// 收纳原则：这些函数不被 core / ffmpeg / cue / main 的任何正式路径调用（部分仅被
// 同包开发测试引用）。为避免它们散落在各文件干扰阅读，统一搬到本文件归档；
// 一旦确认不再需要可整体删除，若将来重新启用再搬回对应文件。
// 注意：它们仍属于 util 包，签名与行为均未改动，同包代码/测试照常可调用。
package util

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/h2non/filetype"
)

// ---------------------------------------------------------------------------
// 定时等待类（原 timer.go，整文件归档）
// ---------------------------------------------------------------------------

var loc *time.Location

func init() {
	loc, _ = time.LoadLocation("Asia/Shanghai")
}

// CheckHour 检查当前时间是否到达指定小时
// hour: 指定的小时数（24小时制）
// 当到达指定小时时返回 true
func CheckHour(hour string) {
	for {
		currentHour := time.Now().In(loc).Format("15") // 移到循环内部，每次都获取最新时间
		if currentHour == hour {
			return
		}
		log.Printf("当前时间为 %s,未达到 %s 点,等待30分钟后再次检查...\n", currentHour, hour)
		time.Sleep(30 * time.Minute) // 缩短检查间隔为1分钟
	}
}

// CheckExactTime 检查是否到达指定的具体时间点
// timeStr: 指定的时间字符串，格式为 "HH:MM:SS"，例如 "08:30:00"
func CheckExactTime(timeStr string) {
	for {
		// 使用上海时区获取当前时间
		currentTime := time.Now().In(loc).Format("15:04:05")
		if currentTime == timeStr {
			return
		}
		log.Printf("当前时间为 %s,未达到 %s 点,等待30分钟后再次检查...\n", currentTime, timeStr)
		time.Sleep(1 * time.Second)
	}
}

// ---------------------------------------------------------------------------
// 控制台退出监听类（原 gracefullyExit.go，整文件归档）
// ---------------------------------------------------------------------------

// SafeExitWithAtomicity 持续监听用户输入，支持随时优雅退出
// 当用户在控制台输入'q'时，会向exitSignal通道发送true信号
func SafeExitWithAtomicity(exitSignal chan bool) {
	go func() {
		fmt.Println("程序正在运行，输入 'q' 并按回车键可以优雅退出...")
		fmt.Println("注意：强制终止可能导致数据丢失！")

		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Print("请输入命令 (q=退出): ")
			input, err := reader.ReadString('\n')
			if err != nil {
				fmt.Printf("读取输入时发生错误: %v\n", err)
				continue
			}

			input = strings.TrimSpace(input)
			if strings.ToLower(input) == "q" {
				fmt.Println("收到退出信号，正在优雅关闭...")
				exitSignal <- true
				return
			} else if input != "" {
				fmt.Printf("未知命令: %s，请输入 'q' 退出\n", input)
			}
		}
	}()
}

// OneTimeExit 一次性监听用户输入，只等待一次输入就结束
// 适用于只需要确认一次是否退出的场景
func OneTimeExit(exitSignal chan bool) {
	go func() {
		fmt.Println("程序正在运行，输入 'q' 并按回车键可以退出...")

		reader := bufio.NewReader(os.Stdin)
		fmt.Print("请输入命令 (q=退出): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("读取输入时发生错误: %v\n", err)
			return
		}

		input = strings.TrimSpace(input)
		if strings.ToLower(input) == "q" {
			fmt.Println("收到退出信号")
			exitSignal <- true
		}
	}()
}

// ---------------------------------------------------------------------------
// 文件/视频枚举与按行读写类（原 io.go，整文件归档）
// ---------------------------------------------------------------------------

// ReadByLine 按行读取文件内容
// fp: 文件路径
// 返回: 字符串切片，每个元素为文件的一行
func ReadByLine(fp string) []string {
	lines := []string{}
	fi, err := os.Open(fp)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		log.Println("按行读文件出错")
		return []string{}
	}
	defer fi.Close()

	br := bufio.NewReader(fi)
	for {
		a, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		lines = append(lines, string(a))
	}
	return lines
}

// 按行写文件
func WriteByLine(fp string, s []string) {
	file, err := os.OpenFile(fp, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		return
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	for _, v := range s {
		writer.WriteString(v)
		writer.WriteString("\n")
	}
	writer.Flush()
}

/*
获取当前文件夹下视频文件
*/
func GetFiles(root string) (files []string) {
	files = append(files, getFilesByHead(root)...)
	return files
}

/*
获取当前文件夹和全部子文件夹下指定扩展名的全部文件
*/
func getFilesByHead(root string) []string {
	var files []string
	defer func() {
		if err := recover(); err != nil {
			log.Println("获取文件出错")
			os.Exit(-1)
		}
	}()
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			// Open a file descriptor
			file, _ := os.Open(path)
			// We only have to pass the file header = first 261 bytes
			head := make([]byte, 261)
			file.Read(head)
			if filetype.IsVideo(head) {
				fmt.Printf("File: %v is a video\n", path)
				files = append(files, path)
			}

		}
		return nil
	})
	return files
}

// GetAllFilesInFolder 获取指定文件夹及其所有子文件夹中的文件的绝对路径
func GetAllVideoButMP4FilesInRootFolder(root string) ([]string, error) {
	var files []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 如果是文件而不是目录，则添加到结果列表中
		if !info.IsDir() {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}

			f, err := os.Open(absPath)
			if err != nil {
				return err
			}
			defer f.Close() // 确保文件被关闭

			head := make([]byte, 261)
			_, err = f.Read(head)
			if err != nil {
				return err
			}

			if filetype.IsVideo(head) {
				fmt.Printf("%s is a Video\n", absPath)
				if filepath.Ext(absPath) != ".mp4" { // 修正条件判断语法
					fmt.Printf("%s is not a mp4 Video\n", absPath)
					files = append(files, absPath)
				}
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// ---------------------------------------------------------------------------
// proj.llc 旧工作流类（原 segment.go 中的归档部分）
// ---------------------------------------------------------------------------

func UseProjLLCFile(llcFile string) []string {
	seconds, _ := extractStartsFromTextFile(llcFile)
	timestamps := SecondToHMS(seconds)
	return timestamps
}

// 提取start后边的秒数
func extractStartsFromTextFile(filePath string) ([]float64, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	var startValues []float64
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "start:") {
			line = strings.Replace(line, ",", "", 1)
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				valueStr := strings.TrimSpace(parts[1])
				value, err := strconv.ParseFloat(valueStr, 64)
				if err != nil {
					return nil, err
				}
				startValues = append(startValues, value)
			}
		}
	}
	return startValues, nil
}

func SecondToHMS(currentTime []float64) []string {
	var timestamps []string
	for _, second := range currentTime {
		timestamps = append(timestamps, FormatSecondToHMS(second))
	}
	return timestamps
}
