package util

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

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
