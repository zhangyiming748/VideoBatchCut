package util

import (
	"fmt"
	"github.com/h2non/filetype"
	"os"
	"path/filepath"
)

func GetFoldersWithLLCFiles(dir string) ([]string, error) {
	var folders []string
	// Walk the directory tree
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Check if the current path is a directory
		if info.IsDir() {
			// Check if there is a file with.llc extension in the current directory
			var fileFound bool
			err := filepath.Walk(path, func(subPath string, subInfo os.FileInfo, subErr error) error {
				if subErr != nil {
					return subErr
				}
				if !subInfo.IsDir() && filepath.Ext(subPath) == ".llc" {
					fileFound = true
					return filepath.SkipDir
				}
				return nil
			})
			if err != nil {
				return err
			}
			if fileFound {
				// If the file exists, add the directory to the slice
				folders = append(folders, path)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return folders, nil
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
