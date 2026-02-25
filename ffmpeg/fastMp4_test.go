package ffmpeg

import (
	"path/filepath"
	"testing"

	"github.com/zhangyiming748/FastMediaInfo"
	"github.com/zhangyiming748/finder"
)

// go test -v -timeout 0 -run TestAnyVideoToMP4
func TestAnyVideoToMP4(t *testing.T) {
	// 测试用例：将指定文件夹中的所有视频文件转换为 MP4 格式
	// 输入：指定文件夹路径
	// 输出：转换后的 MP4 文件路径
	// 预期结果：所有视频文件都被成功转换为 MP4 格式
	root := "F:\\月刊隆行通信"
	folders := finder.FindAllFolders(root)
	for _, folder := range folders {
		videos := finder.FindAllVideosInRoot(folder)
		for _, video := range videos {
			mi := FastMediaInfo.GetStandMediaInfo(video)
			if filepath.Ext(video) == ".mp4" {
				if mi.Video.Format == "AVC" || mi.Video.Format == "HEVC" {
					continue
				}
			}
			if err := AnyVideoToMP4(video); err != nil {
				t.Fatal(err)
			}
		}
	}
}
