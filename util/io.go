// 提供文件和目录操作的工具函数
package util

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/h2non/filetype"
)

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
