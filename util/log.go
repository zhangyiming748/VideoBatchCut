package util

import (
	"github.com/zhangyiming748/lumberjack"
	"io"
	"log"
	"os"
)

func SetLog(l string) {
	// 创建一个用于写入文件的Logger实例

	fileLogger := &lumberjack.Logger{
		Filename:   l,
		MaxSize:    1, // MB
		MaxBackups: 1,
		MaxAge:     28, // days
	}
	//Rotate causes Logger to close the existing log file and immediately create a new one. This is a helper function for applications that want to initiate rotations outside of the normal rotation rules, such as in response to SIGHUP. After rotating, this initiates a cleanup of old log files according to the normal rules.
	err := fileLogger.Rotate()
	if err != nil {
		log.Printf("Rotate error: %v", err)
	}
	consoleLogger := log.New(os.Stdout, "CONSOLE: ", log.LstdFlags)
	log.SetOutput(io.MultiWriter(fileLogger, consoleLogger.Writer()))
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
