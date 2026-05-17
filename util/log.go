package util

import (
	"io"
	"log"
	"os"

	"github.com/zhangyiming748/lumberjack"
)

func SetLog(l string) {
	// 创建一个用于写入文件的Logger实例
	fileLogger := &lumberjack.Logger{
		Filename:   l,
		MaxSize:    1, // MB
		MaxBackups: 1,
		MaxAge:     28, // days
	}

	// 设置日志输出：同时写入文件和控制台
	log.SetOutput(io.MultiWriter(fileLogger, os.Stdout))
	// 设置日志标志：包含日期、时间、文件名和行号
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
