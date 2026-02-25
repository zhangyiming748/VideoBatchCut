package util

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestReadLLC(t *testing.T) {
	llcFile, has := FindProjLLCFile("D:\\迅雷下载")
	if !has {
		t.Log("未找到文件")
		return
	}
	seconds, err := extractStartsFromTextFile(llcFile)
	if err != nil {
		t.Log(err)
	}
	for _, v := range seconds {
		t.Log(v)
	}
	timestamps := SecondToHMS(seconds)
	for _, v := range timestamps {
		t.Log(v)
	}
}

func TestWindowsName(t *testing.T) {
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(hostname)
}

// TestSafeExitWithAtomicity 测试持续监听的优雅退出功能
func TestSafeExitWithAtomicity(t *testing.T) {
	exitSignal := make(chan bool, 1)
	SafeExitWithAtomicity(exitSignal)

	time.Sleep(100 * time.Millisecond)
	t.Log("持续监听模式已启动")

	// 测试超时
	timeout := time.After(1 * time.Second)
	select {
	case <-exitSignal:
		t.Log("收到了退出信号")
	case <-timeout:
		t.Log("测试超时，正常行为 - 没有输入'q'")
	}
}

// TestOneTimeExit 测试一次性监听的退出功能
func TestOneTimeExit(t *testing.T) {
	exitSignal := make(chan bool, 1)
	OneTimeExit(exitSignal)

	time.Sleep(100 * time.Millisecond)
	t.Log("一次性监听模式已启动")

	// 测试超时
	timeout := time.After(1 * time.Second)
	select {
	case <-exitSignal:
		t.Log("收到了退出信号")
	case <-timeout:
		t.Log("测试超时，正常行为 - 没有输入'q'")
	}
}
