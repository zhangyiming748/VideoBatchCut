// 提供命令执行相关的工具函数
package util

import (
	"fmt"
	"log"
	"os/exec"
)

// Exec 执行外部命令并处理输出
// cmd: 要执行的命令
// 返回: 可能的错误
func Exec(cmd *exec.Cmd) error {
	log.Printf("当前运行的命令是:%s\n", cmd.String())
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("命令执行失败:%s\n", err.Error())
		return err
	} else {
		fmt.Printf("命令执行成功:%v\n", string(output))
	}
	return nil
}
