//go:build linux

package process

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	executil "go-port-forward/pkg/os/exec"
	"go-port-forward/pkg/pool"
)

// getProcessByPort 通过端口获取占用该端口的进程信息（Linux实现）
func getProcessByPort(port int) (*ProcessInfo, error) {
	// 方法1: 使用ss命令（推荐）
	if processInfo, err := getProcessByPortUsingSS(port); err == nil && processInfo != nil {
		return processInfo, nil
	}

	// 方法2: 使用netstat命令（备用）
	if processInfo, err := getProcessByPortUsingNetstat(port); err == nil && processInfo != nil {
		return processInfo, nil
	}

	// 方法3: 直接读取/proc/net/tcp（最后备用）
	return getProcessByPortUsingProc(port)
}

// getProcessByPortUsingSS 使用ss命令获取进程信息
func getProcessByPortUsingSS(port int) (*ProcessInfo, error) {
	cmd := executil.ExecCommand("ss", "-tlnp", fmt.Sprintf("sport = :%d", port))
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, fmt.Sprintf(":%d", port)) {
			// 解析ss输出格式
			fields := strings.Fields(line)
			for _, field := range fields {
				if strings.Contains(field, "pid=") {
					// 提取PID
					parts := strings.Split(field, ",")
					for _, part := range parts {
						if strings.HasPrefix(part, "pid=") {
							pidStr := strings.TrimPrefix(part, "pid=")
							pid, err := strconv.Atoi(pidStr)
							if err != nil {
								continue
							}

							processInfo, err := getProcessInfo(pid)
							if err != nil {
								continue
							}
							processInfo.Port = port
							return processInfo, nil
						}
					}
				}
			}
		}
	}

	return nil, nil
}

// getProcessByPortUsingNetstat 使用netstat命令获取进程信息
func getProcessByPortUsingNetstat(port int) (*ProcessInfo, error) {
	cmd := executil.ExecCommand("netstat", "-tlnp")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	targetPort := fmt.Sprintf(":%d", port)

	for _, line := range lines {
		if strings.Contains(line, targetPort) && strings.Contains(line, "LISTEN") {
			fields := strings.Fields(line)
			if len(fields) >= 7 {
				pidProgram := fields[6]
				if pidProgram != "-" {
					parts := strings.Split(pidProgram, "/")
					if len(parts) >= 1 {
						pidStr := parts[0]
						pid, err := strconv.Atoi(pidStr)
						if err != nil {
							continue
						}

						processInfo, err := getProcessInfo(pid)
						if err != nil {
							continue
						}
						processInfo.Port = port
						return processInfo, nil
					}
				}
			}
		}
	}

	return nil, nil
}

// getProcessByPortUsingProc 直接读取/proc/net/tcp获取进程信息
func getProcessByPortUsingProc(port int) (*ProcessInfo, error) {
	// 读取/proc/net/tcp
	data, err := os.ReadFile("/proc/net/tcp")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	hexPort := fmt.Sprintf("%04X", port)

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 10 {
			localAddr := fields[1]
			if strings.HasSuffix(localAddr, ":"+hexPort) {
				inode := fields[9]

				// 通过inode查找进程
				pid, err := findProcessByInode(inode)
				if err != nil {
					continue
				}

				processInfo, err := getProcessInfo(pid)
				if err != nil {
					continue
				}
				processInfo.Port = port
				return processInfo, nil
			}
		}
	}

	return nil, nil
}

// findProcessByInode 通过inode查找进程PID
func findProcessByInode(inode string) (int, error) {
	procDir := "/proc"
	entries, err := os.ReadDir(procDir)
	if err != nil {
		return 0, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pidStr := entry.Name()
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		fdDir := filepath.Join(procDir, pidStr, "fd")
		fdEntries, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}

		for _, fdEntry := range fdEntries {
			fdPath := filepath.Join(fdDir, fdEntry.Name())
			link, err := os.Readlink(fdPath)
			if err != nil {
				continue
			}

			if strings.Contains(link, "socket:["+inode+"]") {
				return pid, nil
			}
		}
	}

	return 0, errors.New("process not found for inode " + inode)
}

// getProcessesByName 通过进程名称获取进程列表（Linux实现）
func getProcessesByName(name string) ([]*ProcessInfo, error) {
	cmd := executil.ExecCommand("pgrep", "-f", name)
	output, err := cmd.Output()
	if err != nil {
		// pgrep没找到进程时会返回错误，这是正常的
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

		// 验证进程名称是否真的匹配
		if strings.Contains(strings.ToLower(processInfo.Name), strings.ToLower(name)) ||
			strings.Contains(strings.ToLower(processInfo.ExecutePath), strings.ToLower(name)) {
			processes = append(processes, processInfo)
		}
	}

	return processes, nil
}

// getProcessesByPath 通过可执行文件路径获取进程列表（Linux实现）
func getProcessesByPath(execPath string) ([]*ProcessInfo, error) {
	var processes []*ProcessInfo

	// 遍历/proc目录
	procDir := "/proc"
	entries, err := os.ReadDir(procDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc directory: %w", err)
	}

	// 标准化目标路径
	normalizedTargetPath := filepath.Clean(execPath)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pidStr := entry.Name()
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		// 读取/proc/PID/exe链接
		exePath := filepath.Join(procDir, pidStr, "exe")
		realPath, err := os.Readlink(exePath)
		if err != nil {
			continue
		}

		// 标准化进程路径
		normalizedProcessPath := filepath.Clean(realPath)

		if normalizedProcessPath == normalizedTargetPath {
			processInfo, err := getProcessInfo(pid)
			if err != nil {
				continue
			}
			processes = append(processes, processInfo)
		}
	}

	return processes, nil
}

// getProcessInfo 获取进程详细信息（Linux实现）
func getProcessInfo(pid int) (*ProcessInfo, error) {
	// 读取/proc/PID/comm获取进程名称
	commPath := fmt.Sprintf("/proc/%d/comm", pid)
	commData, err := os.ReadFile(commPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read process name for PID %d: %w", pid, err)
	}
	processName := strings.TrimSpace(string(commData))

	// 读取/proc/PID/exe获取可执行文件路径
	exePath := fmt.Sprintf("/proc/%d/exe", pid)
	executablePath, err := os.Readlink(exePath)
	if err != nil {
		// 如果无法读取exe链接，尝试从cmdline获取
		cmdlinePath := fmt.Sprintf("/proc/%d/cmdline", pid)
		cmdlineData, err := os.ReadFile(cmdlinePath)
		if err != nil {
			executablePath = ""
		} else {
			// cmdline以null字符分隔，第一个通常是可执行文件路径
			cmdline := string(cmdlineData)
			if len(cmdline) > 0 {
				parts := strings.Split(cmdline, "\x00")
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

// killProcess 终止进程（Linux实现）
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

// killProcessGracefully 优雅地终止进程（Linux实现）
func killProcessGracefully(pid int, timeout time.Duration) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	// 首先发送SIGTERM信号
	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		// 如果SIGTERM失败，直接使用SIGKILL
		return process.Kill()
	}

	// 等待进程终止
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
		// 超时后发送SIGKILL
		return process.Kill()
	}
}

// isProcessRunning 检查进程是否正在运行（Linux实现）
func isProcessRunning(pid int) bool {
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	_, err := os.Stat(statPath)
	return err == nil
}
