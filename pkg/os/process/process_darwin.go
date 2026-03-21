//go:build darwin

package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	executil "go-port-forward/pkg/os/exec"
	"go-port-forward/pkg/pool"
)

// getProcessByPort 通过端口获取占用该端口的进程信息（macOS实现）| Get process by port (macOS implementation)
func getProcessByPort(port int) (*ProcessInfo, error) {
	// 使用 lsof 命令查找占用端口的进程 | Use lsof command to find process using port
	cmd := executil.ExecCommand("lsof", "-i", fmt.Sprintf(":%d", port), "-t")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to find process on port %d: %w", port, err)
	}

	pidStr := strings.TrimSpace(string(output))
	if pidStr == "" {
		return nil, errors.New("no process found on port " + strconv.Itoa(port))
	}

	// lsof -t 可能返回多个PID，取第一个 | lsof -t may return multiple PIDs, take the first one
	lines := strings.Split(pidStr, "\n")
	pidStr = strings.TrimSpace(lines[0])

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return nil, errors.New("invalid PID: " + pidStr)
	}

	processInfo, err := getProcessInfo(pid)
	if err != nil {
		return nil, err
	}
	processInfo.Port = port
	return processInfo, nil
}

// getProcessesByName 通过进程名称获取进程列表（macOS实现）| Get processes by name (macOS implementation)
func getProcessesByName(name string) ([]*ProcessInfo, error) {
	// 使用 pgrep 命令查找进程 | Use pgrep command to find processes
	cmd := executil.ExecCommand("pgrep", "-f", name)
	output, err := cmd.Output()
	if err != nil {
		// pgrep没找到进程时会返回错误，这是正常的 | pgrep returns error when no process found, this is normal
		return []*ProcessInfo{}, nil
	}

	var processes []*ProcessInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		pid, err := strconv.Atoi(line)
		if err != nil {
			continue
		}

		processInfo, err := getProcessInfo(pid)
		if err != nil {
			continue
		}

		// 验证进程名称是否真的匹配 | Verify process name actually matches
		if strings.Contains(strings.ToLower(processInfo.Name), strings.ToLower(name)) ||
			strings.Contains(strings.ToLower(processInfo.ExecutePath), strings.ToLower(name)) {
			processes = append(processes, processInfo)
		}
	}

	return processes, nil
}

// getProcessesByPath 通过可执行文件路径获取进程列表（macOS实现）| Get processes by executable path (macOS implementation)
func getProcessesByPath(execPath string) ([]*ProcessInfo, error) {
	var processes []*ProcessInfo

	// 标准化目标路径 | Normalize target path
	normalizedTargetPath := filepath.Clean(execPath)

	// 使用 ps 命令获取所有进程 | Use ps command to get all processes
	cmd := executil.ExecCommand("ps", "-eo", "pid,comm")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute ps command: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue // 跳过标题行和空行 | Skip header and empty lines
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}

		// 获取进程的可执行文件路径 | Get process executable path
		processInfo, err := getProcessInfo(pid)
		if err != nil {
			continue
		}

		// 标准化进程路径 | Normalize process path
		normalizedProcessPath := filepath.Clean(processInfo.ExecutePath)

		if normalizedProcessPath == normalizedTargetPath {
			processes = append(processes, processInfo)
		}
	}

	return processes, nil
}

// getProcessInfo 获取进程详细信息（macOS实现）| Get process details (macOS implementation)
func getProcessInfo(pid int) (*ProcessInfo, error) {
	// 使用 ps 命令获取进程名称 | Use ps command to get process name
	cmd := executil.ExecCommand("ps", "-p", strconv.Itoa(pid), "-o", "comm=")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get process name for PID %d: %w", pid, err)
	}
	processName := strings.TrimSpace(string(output))

	// 获取可执行文件路径 | Get executable path
	// 方法1: 尝试使用 lsof 获取 txt 类型文件 | Method 1: Try using lsof to get txt type file
	executablePath := ""
	lsofCmd := executil.ExecCommand("lsof", "-p", strconv.Itoa(pid), "-Fn")
	lsofOutput, err := lsofCmd.Output()
	if err == nil {
		lines := strings.Split(string(lsofOutput), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "n") && strings.Contains(line, processName) {
				executablePath = strings.TrimPrefix(line, "n")
				break
			}
		}
	}

	// 方法2: 如果 lsof 失败，尝试从 ps 获取命令行 | Method 2: If lsof fails, try getting command line from ps
	if executablePath == "" {
		psCmd := executil.ExecCommand("ps", "-p", strconv.Itoa(pid), "-o", "command=")
		psOutput, err := psCmd.Output()
		if err == nil {
			cmdline := strings.TrimSpace(string(psOutput))
			if cmdline != "" {
				// 命令行的第一个参数通常是可执行文件路径 | First argument is usually the executable path
				parts := strings.Fields(cmdline)
				if len(parts) > 0 {
					executablePath = parts[0]
				}
			}
		}
	}

	return &ProcessInfo{
		PID:         pid,
		Name:        processName,
		ExecutePath: executablePath,
	}, nil
}

// killProcess 终止进程（macOS实现）| Kill process (macOS implementation)
func killProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	err = process.Kill()
	if err != nil {
		return fmt.Errorf("failed to kill process %d: %w", pid, err)
	}

	return nil
}

// killProcessGracefully 优雅地终止进程（macOS实现）| Kill process gracefully (macOS implementation)
func killProcessGracefully(pid int, timeout time.Duration) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	// 首先发送SIGTERM信号 | First send SIGTERM signal
	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		// 如果SIGTERM失败，直接使用SIGKILL | If SIGTERM fails, use SIGKILL directly
		return process.Kill()
	}

	// 等待进程终止 | Wait for process to terminate
	done := make(chan bool, 1)
	if err := pool.Submit(func() {
		for {
			if !isProcessRunning(pid) {
				done <- true
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}); err != nil {
		return err
	}

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		// 超时后发送SIGKILL | Send SIGKILL after timeout
		return process.Kill()
	}
}

// isProcessRunning 检查进程是否正在运行（macOS实现）| Check if process is running (macOS implementation)
func isProcessRunning(pid int) bool {
	// 使用 kill -0 检查进程是否存在 | Use kill -0 to check if process exists
	cmd := exec.Command("kill", "-0", strconv.Itoa(pid))
	err := cmd.Run()
	return err == nil
}
