// 提供无损切割相关的工具函数
package util

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func UseProjLLCFile(llcFile string) []string {
	seconds, _ := extractStartsFromTextFile(llcFile)
	timestamps := SecondToHMS(seconds)
	return timestamps
}

// 搜索目标文件夹是否包含后缀为proj.llc的文件
func FindProjLLCFile(folderPath string) (string, bool) {
	var projLLCFile string
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, "-proj.llc") {
			projLLCFile = path
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		fmt.Printf("遍历文件夹时出错: %v\n", err)
		return "", false
	}
	if projLLCFile != "" {
		return projLLCFile, true
	}
	return "", false
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

// Segment 定义视频片段的结构
type Segment struct {
	Start float64 // 开始时间（秒）
	End   float64 // 结束时间（秒）
	Name  string  // 片段名称
}

// FormatSecondToHMS 将秒数转换为时分秒格式
// 输入: 秒数（float64）
// 输出: "HH:MM:SS.mmm" 格式的时间字符串
// 毫秒用截断而非四舍五入：旧实现 math.Round 在秒的小数部分 ≥0.9995 时会进位到 1000，
// 格式化出 13 位再被 [:12] 硬截断，切割点会整整提前 0.9 秒。
func FormatSecondToHMS(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	// 先整体截断到毫秒，再拆分时分秒，从根上消灭“毫秒进位到 1000”的可能
	totalMs := int64(seconds * 1000)
	hours := totalMs / 3600000
	minutes := totalMs % 3600000 / 60000
	secs := totalMs % 60000 / 1000
	ms := totalMs % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, secs, ms)
}

// ParseSegments 解析proj.llc文件中的片段信息
// filename: proj.llc文件路径
// 返回: 片段列表和可能的错误
func ParseSegments(filename string) ([]Segment, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var segments []Segment
	var currentSegment *Segment

	// 用于提取数值的正则表达式
	numRegex := regexp.MustCompile(`[-]?\d+\.?\d*`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 开始新的段落
		if strings.Contains(line, "{") {
			currentSegment = &Segment{}
			continue
		}

		// 结束当前段落
		if strings.Contains(line, "},") {
			if currentSegment != nil {
				segments = append(segments, *currentSegment)
				currentSegment = nil
			}
			continue
		}

		// 解析字段
		if currentSegment != nil {
			if strings.Contains(line, "start:") {
				if num := numRegex.FindString(line); num != "" {
					currentSegment.Start, _ = strconv.ParseFloat(num, 64)
				}
			} else if strings.Contains(line, "end:") {
				if num := numRegex.FindString(line); num != "" {
					currentSegment.End, _ = strconv.ParseFloat(num, 64)
				}
			} else if strings.Contains(line, "name:") {
				parts := strings.Split(line, ":")
				if len(parts) > 1 {
					currentSegment.Name = strings.Trim(parts[1], " ',")
				}
			}
		}
	}

	// 处理最后一个段落（如果有的话）
	if currentSegment != nil {
		segments = append(segments, *currentSegment)
	}

	return segments, scanner.Err()
}
