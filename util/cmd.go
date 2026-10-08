// 提供命令执行相关的工具函数。
package util

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
)

// ExitError 表示外部命令以非零状态退出（或运行失败），并携带其 stderr 输出。
//
// 存在意义：exec.Cmd.Run/Wait 默认返回的 *exec.ExitError 只含 "exit status 1"，
// 而 ffmpeg 真正的失败原因（编码器打开失败、设备不可用、参数不支持等）全部打印在
// stderr 上。若直接把 exit error 往上抛，上层只能看到“硬件编码器失败”却不知道为什么。
// 故用本类型把 stderr 固化进 error，使其沿调用链返回时始终带着详细原因，
// 调用方也可用 errors.As 取出原始 stderr 做进一步判断。
type ExitError struct {
	Name   string // 命令名（如 "ffmpeg"）
	Err    error  // 底层错误（通常是 *exec.ExitError）
	Output string // 捕获到的 stderr 全文
}

func (e *ExitError) Error() string {
	output := strings.TrimSpace(e.Output)
	if output == "" {
		return fmt.Sprintf("%s: %v", e.Name, e.Err)
	}
	return fmt.Sprintf("%s: %v, stderr:\n%s", e.Name, e.Err, output)
}

// Unwrap 暴露底层错误，支持 errors.Is / errors.As（如取回 *exec.ExitError 的退出码）。
func (e *ExitError) Unwrap() error { return e.Err }

// newExitError 把命令失败与捕获到的 stderr 包装成 *ExitError。
func newExitError(cmd *exec.Cmd, err error, stderr string) *ExitError {
	name := "command"
	if len(cmd.Args) > 0 {
		name = cmd.Args[0]
	}
	return &ExitError{Name: name, Err: err, Output: stderr}
}

// Exec 执行外部命令，流式处理输出。
//
// 日志策略：
//   - 成功：只打印精简结果（"命令完成"），不刷屏；
//   - 失败：打印完整 stderr 输出，便于排查 ffmpeg 报错原因。
//
// 进度支持：若命令参数含 "-progress" "pipe:1"，则从 stdout 解析 ffmpeg 的进度行
// （out_time=...），周期性打印已编码时长，让长视频处理时用户能看到进度而非以为卡死。
func Exec(cmd *exec.Cmd) error {
	log.Printf("[exec] 运行命令: %s", cmd.String())

	// ffmpeg 的正常日志与错误都输出到 stderr，统一捕获到缓冲区。
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	hasProgress := hasPipeProgress(cmd.Args)

	if hasProgress {
		// 启用了 -progress pipe:1：stdout 是机器可读的进度流，解析它。
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}
		if err := cmd.Start(); err != nil {
			return err
		}
		parseProgress(stdoutPipe)
		if err := cmd.Wait(); err != nil {
			exitErr := newExitError(cmd, err, stderrBuf.String())
			log.Printf("[exec] 命令失败: %v", exitErr)
			return exitErr
		}
	} else {
		// 无进度输出：直接运行，stderr 已被缓冲区捕获。
		if err := cmd.Run(); err != nil {
			exitErr := newExitError(cmd, err, stderrBuf.String())
			log.Printf("[exec] 命令失败: %v", exitErr)
			return exitErr
		}
	}

	log.Printf("[exec] 命令完成")
	return nil
}

// hasPipeProgress 判断命令参数是否包含 "-progress" "pipe:1"。
func hasPipeProgress(args []string) bool {
	for i, a := range args {
		if a == "-progress" && i+1 < len(args) && args[i+1] == "pipe:1" {
			return true
		}
	}
	return false
}

// parseProgress 从 ffmpeg -progress pipe:1 的 stdout 流中解析进度。
// ffmpeg 每编码一批帧输出一组 key=value 行，以 progress=continue / progress=end 标记批次结束。
// 这里只取 out_time=HH:MM:SS.xxx 打印已编码时长，让用户感知处理进度。
func parseProgress(r io.Reader) {
	scanner := bufio.NewScanner(r)
	// 限制单行长度，防止异常输出撑爆内存
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "out_time=") {
			t := strings.TrimPrefix(line, "out_time=")
			// 去掉微秒部分的精度，只保留到毫秒，日志更干净
			if dot := strings.Index(t, "."); dot > 0 && len(t) > dot+4 {
				t = t[:dot+4]
			}
			log.Printf("[exec] 编码进度: %s", t)
		} else if line == "progress=end" {
			break
		}
	}
}
