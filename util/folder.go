package util

import (
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
