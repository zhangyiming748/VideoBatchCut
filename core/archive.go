package core

import (
	"github.com/zhangyiming748/archive"
	"github.com/zhangyiming748/finder"
)

/*
仅处理分割后的视频文件
分割后的文件夹特征:
1. 是最后一级目录
2. 文件夹下有两个或以上mp4文件
*/

func Archive(root string) {
	// 获取所有子目录
	folders:=finder.FindAllFolders(root)
	for _, folder := range folders {
		// 获取该目录下的所有mp4文件
		mp4Files := finder.FindAllVideosInRoot(folder)
		if len(mp4Files) >= 2 {
			// 文件夹符合特征
			for _, mp4File := range mp4Files {
				archive.Convert2H265(mp4File)
			}
		}
	}
}