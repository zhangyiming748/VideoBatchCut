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
			dir, _ := cmd.Flags().GetString("dir")
			if dir == "" {
				fmt.Println("错误：必须指定 --dir 参数")
				return
			}
			fmt.Printf("开始执行视频切割任务...\n根目录：%s\n", dir)
			core.Cut(dir)
		},
	}
	// 为 cut 命令添加标志
	cutCmd.Flags().StringP("dir", "d", "./", "根目录路径 (必需)")

	// 添加fastmp4命令
	var fastmp4Cmd = &cobra.Command{
		Use:   "fastmp4",
		Short: "快速转换MP4",
		Long:  "根据指定的根目录快速转换MP4",
		Run: func(cmd *cobra.Command, args []string) {
			dir, _ := cmd.Flags().GetString("dir")
			if dir == "" {
				fmt.Println("错误：必须指定 --dir 参数")
				return
			}
			fmt.Printf("开始执行快速转换 mp4 任务...\n根目录：%s\n", dir)
			core.FastMP4(dir)
		},
	}
	// 为 fast 命令添加标志
	fastmp4Cmd.Flags().StringP("dir", "d", "./", "根目录路径 (必需)")

	// 添加 archive 命令
	var archiveCmd = &cobra.Command{
		Use:   "archive",
		Short: "转换符合特征的视频文件",
		Long:  "转换一个文件夹中已经经过分割的视频文件",
		Run: func(cmd *cobra.Command, args []string) {
			dir, _ := cmd.Flags().GetString("dir")
			if dir == "" {
				fmt.Println("错误：必须指定 --dir 参数")
				return
			}
			fmt.Printf("开始执行视频归档转换任务...\n根目录：%s\n", dir)
			core.Archive(dir)
		},
	}
	// 为 archive 命令添加标志
	archiveCmd.Flags().StringP("dir", "d", "./", "根目录路径 (必需)")

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
	rootCmd.AddCommand(archiveCmd)
	rootCmd.AddCommand(versionCmd)

	// 执行命令
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
