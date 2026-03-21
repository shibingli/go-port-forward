//go:build windows

package process

import (
	"errors"
	"fmt"
	executil "go-port-forward/pkg/os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess      = kernel32.NewProc("OpenProcess")
	procTerminateProcess = kernel32.NewProc("TerminateProcess")
	procCloseHandle      = kernel32.NewProc("CloseHandle")
)

const (
	PROCESS_TERMINATE         = 0x0001
	PROCESS_QUERY_INFORMATION = 0x0400
)

// getProcessByPort 通过端口获取占用该端口的进程信息（Windows实现）
func getProcessByPort(port int) (*ProcessInfo, error) {
	// 使用netstat命令查找占用端口的进程
	cmd := executil.ExecCommand("netstat", "-ano")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute netstat: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	targetPort := fmt.Sprintf(":%d", port)

	for _, line := range lines {
		if strings.Contains(line, targetPort) && strings.Contains(line, "LISTENING") {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				pidStr := fields[len(fields)-1]
				pid, err := strconv.Atoi(pidStr)
				if err != nil {
					continue
				}

				// 获取进程详细信息
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

// getProcessesByName 通过进程名称获取进程列表（Windows实现）
func getProcessesByName(name string) ([]*ProcessInfo, error) {
	// 使用tasklist命令查找进程
	cmd := executil.ExecCommand("tasklist", "/fo", "csv", "/nh")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute tasklist: %w", err)
	}

	var processes []*ProcessInfo
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 解析CSV格式的输出
		fields := parseCSVLine(line)
		if len(fields) >= 2 {
			processName := strings.Trim(fields[0], "\"")
			pidStr := strings.Trim(fields[1], "\"")

			// 检查进程名称是否匹配（支持部分匹配）
			if strings.Contains(strings.ToLower(processName), strings.ToLower(name)) {
				pid, err := strconv.Atoi(pidStr)
				if err != nil {
					continue
				}

				processInfo, err := getProcessInfo(pid)
				if err != nil {
					continue
				}
				processes = append(processes, processInfo)
			}
		}
	}

	return processes, nil
}

// getProcessesByPath 通过可执行文件路径获取进程列表（Windows实现）
func getProcessesByPath(execPath string) ([]*ProcessInfo, error) {
	// 首先尝试使用PowerShell（更可靠）
	if processes, err := getProcessesByPathUsingPowerShell(execPath); err == nil {
		return processes, nil
	}

	// 备用方法：使用wmic
	if processes, err := getProcessesByPathUsingWMIC(execPath); err == nil {
		return processes, nil
	}

	// 最后备用方法：通过进程名称匹配
	processName := filepath.Base(execPath)
	return getProcessesByName(processName)
}

// getProcessesByPathUsingPowerShell 使用PowerShell获取进程列表
func getProcessesByPathUsingPowerShell(execPath string) ([]*ProcessInfo, error) {
	// 使用PowerShell命令获取进程信息
	psCmd := fmt.Sprintf(`Get-Process | Where-Object {$_.Path -eq '%s'} | Select-Object Id,ProcessName,Path | ConvertTo-Csv -NoTypeInformation`,
		strings.ReplaceAll(execPath, "'", "''"))

	cmd := executil.ExecCommand("powershell", "-Command", psCmd)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute powershell: %w", err)
	}

	var processes []*ProcessInfo
	lines := strings.Split(string(output), "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if i == 0 || line == "" { // 跳过标题行和空行
			continue
		}

		// 解析CSV格式的输出
		fields := parseCSVLine(line)
		if len(fields) >= 3 {
			pidStr := strings.Trim(fields[0], "\"")
			processName := strings.Trim(fields[1], "\"")
			processPath := strings.Trim(fields[2], "\"")

			pid, err := strconv.Atoi(pidStr)
			if err != nil {
				continue
			}

			processes = append(processes, &ProcessInfo{
				PID:         pid,
				Name:        processName,
				ExecutePath: processPath,
			})
		}
	}

	return processes, nil
}

// getProcessesByPathUsingWMIC 使用wmic获取进程列表（备用方法）
func getProcessesByPathUsingWMIC(execPath string) ([]*ProcessInfo, error) {
	// 使用wmic命令查找进程
	cmd := executil.ExecCommand("wmic", "process", "get", "ProcessId,ExecutablePath", "/format:csv")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute wmic: %w", err)
	}

	var processes []*ProcessInfo
	lines := strings.Split(string(output), "\n")

	// 标准化路径用于比较
	normalizedTargetPath := strings.ToLower(filepath.Clean(execPath))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Node,") {
			continue
		}

		fields := strings.Split(line, ",")
		if len(fields) >= 3 {
			processPath := strings.TrimSpace(fields[1])
			pidStr := strings.TrimSpace(fields[2])

			if processPath != "" && pidStr != "" {
				// 标准化进程路径用于比较
				normalizedProcessPath := strings.ToLower(filepath.Clean(processPath))

				if normalizedProcessPath == normalizedTargetPath {
					pid, err := strconv.Atoi(pidStr)
					if err != nil {
						continue
					}

					processInfo, err := getProcessInfo(pid)
					if err != nil {
						continue
					}
					processes = append(processes, processInfo)
				}
			}
		}
	}

	return processes, nil
}

// getProcessInfo 获取进程详细信息
func getProcessInfo(pid int) (*ProcessInfo, error) {
	// 使用tasklist获取进程名称
	cmd := executil.ExecCommand("tasklist", "/fi", fmt.Sprintf("PID eq %d", pid), "/fo", "csv", "/nh")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get process info for PID %d: %w", pid, err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := parseCSVLine(line)
		if len(fields) >= 2 {
			processName := strings.Trim(fields[0], "\"")

			// 获取可执行文件路径
			execPath := getProcessExecutablePath(pid)

			return &ProcessInfo{
				PID:         pid,
				Name:        processName,
				ExecutePath: execPath,
			}, nil
		}
	}

	return nil, errors.New("process with PID " + strconv.Itoa(pid) + " not found")
}

// getProcessExecutablePath 获取进程的可执行文件路径
func getProcessExecutablePath(pid int) string {
	// 首先尝试使用PowerShell
	if path := getProcessExecutablePathUsingPowerShell(pid); path != "" {
		return path
	}

	// 备用方法：使用wmic
	return getProcessExecutablePathUsingWMIC(pid)
}

// getProcessExecutablePathUsingPowerShell 使用PowerShell获取进程可执行文件路径
func getProcessExecutablePathUsingPowerShell(pid int) string {
	psCmd := fmt.Sprintf(`Get-Process -Id %d | Select-Object -ExpandProperty Path`, pid)
	cmd := executil.ExecCommand("powershell", "-Command", psCmd)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

// getProcessExecutablePathUsingWMIC 使用wmic获取进程可执行文件路径（备用方法）
func getProcessExecutablePathUsingWMIC(pid int) string {
	cmd := executil.ExecCommand("wmic", "process", "where", fmt.Sprintf("ProcessId=%d", pid), "get", "ExecutablePath", "/format:list")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ExecutablePath=") {
			return strings.TrimPrefix(line, "ExecutablePath=")
		}
	}

	return ""
}

// killProcess 终止进程（Windows实现）
func killProcess(pid int) error {
	// 使用taskkill命令终止进程
	cmd := executil.ExecCommand("taskkill", "/F", "/PID", strconv.Itoa(pid))
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to kill process %d: %w", pid, err)
	}
	return nil
}

// killProcessGracefully 优雅地终止进程（Windows实现）
func killProcessGracefully(pid int, timeout time.Duration) error {
	// 首先尝试优雅终止
	cmd := executil.ExecCommand("taskkill", "/PID", strconv.Itoa(pid))
	err := cmd.Run()
	if err == nil {
		// 等待进程终止
		for i := 0; i < int(timeout.Seconds()); i++ {
			if !isProcessRunning(pid) {
				return nil
			}
			time.Sleep(1 * time.Second)
		}
	}

	// 如果优雅终止失败或超时，强制终止
	return killProcess(pid)
}

// isProcessRunning 检查进程是否正在运行
func isProcessRunning(pid int) bool {
	cmd := executil.ExecCommand("tasklist", "/fi", fmt.Sprintf("PID eq %d", pid), "/fo", "csv", "/nh")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(output)) != ""
}

// parseCSVLine 解析CSV行（简单实现）
func parseCSVLine(line string) []string {
	var fields []string
	var current strings.Builder
	inQuotes := false

	for _, char := range line {
		switch char {
		case '"':
			inQuotes = !inQuotes
			current.WriteRune(char)
		case ',':
			if inQuotes {
				current.WriteRune(char)
			} else {
				fields = append(fields, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(char)
		}
	}

	// 添加最后一个字段
	if current.Len() > 0 {
		fields = append(fields, current.String())
	}

	return fields
}
