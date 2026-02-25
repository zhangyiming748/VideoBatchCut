package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"VideoBatchCut/core"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "batch-cut",
		Short: "VideoBatchCut的命令行版",
		Long:  `一个用golang写的视频批量切割工具`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("欢迎使用VideoBatchCut")
		},
	}
	var cutCmd = &cobra.Command{
		Use:   "cut",
		Short: "切割文件",
		Long:  "根据指定的根目录切割文件",
		Run: func(cmd *cobra.Command, args []string) {
			root, _ := cmd.Flags().GetString("root")
			if root == "" {
				fmt.Println("错误: 必须指定 --root 参数")
				return
			}
			fmt.Printf("开始执行视频切割任务...\n根目录: %s\n", root)
			core.Cut(root)
		},
	}

	// 为cut命令添加标志
	cutCmd.Flags().String("root", "./", "根目录路径 (必需)")

	// 添加fastmp4命令
	var fastmp4Cmd = &cobra.Command{
		Use:   "fastmp4",
		Short: "快速转换MP4",
		Long:  "根据指定的根目录快速转换MP4",
		Run: func(cmd *cobra.Command, args []string) {
			root, _ := cmd.Flags().GetString("root")
			if root == "" {
				fmt.Println("错误: 必须指定 --root 参数")
				return
			}
			fmt.Printf("开始执行快速转换mp4任务...\n根目录: %s\n", root)
			core.FastMP4(root)
		},
	}
	// 为fast命令添加标志
	fastmp4Cmd.Flags().String("root", "./", "根目录路径 (必需)")

	// 添加version命令
	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "显示版本信息",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("VideoBatchCut 版本: 1.0.0\n")
			fmt.Printf("Go版本: %s\n", runtime.Version())
			fmt.Printf("操作系统: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
	}

	rootCmd.AddCommand(cutCmd)
	rootCmd.AddCommand(fastmp4Cmd)
	rootCmd.AddCommand(versionCmd)

	// 执行命令
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
