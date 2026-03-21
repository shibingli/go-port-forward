// Package process 提供跨平台的进程管理工具函数 | Package process provides cross-platform process management utilities
package process

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"go-port-forward/pkg/logger"
)

// ProcessInfo 进程信息结构 | Process information structure
type ProcessInfo struct {
	Name        string `json:"name" msgpack:"name"`
	ExecutePath string `json:"exe_path" msgpack:"exe_path"`
	PID         int    `json:"pid" msgpack:"pid"`
	Port        int    `json:"port" msgpack:"port"`
}

// ForceStartOptions 强制启动选项 | Force start options
type ForceStartOptions struct {
	ProcessName string `json:"process_name" msgpack:"process_name"`
	ExecutePath string `json:"exe_path" msgpack:"exe_path"`
	Port        int    `json:"port" msgpack:"port"`
	Timeout     int    `json:"timeout" msgpack:"timeout"`
}

// IsPortInUse 检查端口是否被占用 | Check if port is in use
func IsPortInUse(port int) bool {
	conn, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return true // 端口被占用
	}
	defer conn.Close()
	return false // 端口可用
}

// GetProcessByPort 通过端口获取占用该端口的进程信息 | Get process information by port
func GetProcessByPort(port int) (*ProcessInfo, error) {
	return getProcessByPort(port)
}

// GetProcessesByName 通过进程名称获取进程列表 | Get process list by process name
func GetProcessesByName(name string) ([]*ProcessInfo, error) {
	return getProcessesByName(name)
}

// GetProcessesByPath 通过可执行文件路径获取进程列表 | Get process list by executable path
func GetProcessesByPath(execPath string) ([]*ProcessInfo, error) {
	return getProcessesByPath(execPath)
}

// KillProcess 终止进程 | Kill process
func KillProcess(pid int) error {
	return killProcess(pid)
}

// KillProcessGracefully 优雅地终止进程（先尝试SIGTERM，再SIGKILL）| Gracefully kill process (try SIGTERM first, then SIGKILL)
func KillProcessGracefully(pid int, timeout time.Duration) error {
	return killProcessGracefully(pid, timeout)
}

// ForceStart 强制启动程序 | Force start program
// 检查端口占用和同名进程，如果存在则终止后返回 | Check port usage and same-name processes, terminate if exists then return
func ForceStart(options ForceStartOptions) error {
	return ForceStartWithLogger(options, logger.Get())
}

// ForceStartWithLogger 强制启动程序（使用日志器输出）| Force start program with logger output
// 检查端口占用和同名进程，如果存在则终止后返回 | Check port usage and same-name processes, terminate if exists then return
func ForceStartWithLogger(options ForceStartOptions, logger *zap.Logger) error {
	if options.Timeout == 0 {
		options.Timeout = 30 // 默认30秒超时
	}

	var processesToKill []*ProcessInfo

	// 1. 检查端口占用
	if options.Port > 0 && IsPortInUse(options.Port) {
		process, err := GetProcessByPort(options.Port)
		if err != nil {
			logger.Error("Failed to get process by port",
				zap.Int("port", options.Port),
				zap.Error(err))
			return fmt.Errorf("failed to get process by port %d: %w", options.Port, err)
		}
		if process != nil {
			processesToKill = append(processesToKill, process)
		}
	}

	// 2. 检查同名进程
	if options.ProcessName != "" {
		processes, err := GetProcessesByName(options.ProcessName)
		if err != nil {
			logger.Error("Failed to get processes by name",
				zap.String("process_name", options.ProcessName),
				zap.Error(err))
			return fmt.Errorf("failed to get processes by name %s: %w", options.ProcessName, err)
		}
		processesToKill = append(processesToKill, processes...)
	}

	// 3. 检查同路径进程
	if options.ExecutePath != "" {
		// 获取绝对路径
		absPath, err := filepath.Abs(options.ExecutePath)
		if err != nil {
			logger.Error("Failed to get absolute path",
				zap.String("execute_path", options.ExecutePath),
				zap.Error(err))
			return fmt.Errorf("failed to get absolute path for %s: %w", options.ExecutePath, err)
		}

		processes, err := GetProcessesByPath(absPath)
		if err != nil {
			logger.Error("Failed to get processes by path",
				zap.String("absolute_path", absPath),
				zap.Error(err))
			return fmt.Errorf("failed to get processes by path %s: %w", absPath, err)
		}
		processesToKill = append(processesToKill, processes...)
	}

	// 4. 去重进程列表
	uniqueProcesses := make(map[int]*ProcessInfo)
	for _, process := range processesToKill {
		// 排除当前进程
		if process.PID != os.Getpid() {
			uniqueProcesses[process.PID] = process
		}
	}

	// 5. 终止进程
	if len(uniqueProcesses) > 0 {
		logger.Info("Found conflicting processes, terminating",
			zap.Int("process_count", len(uniqueProcesses)))

		for _, process := range uniqueProcesses {
			logger.Info("Terminating process",
				zap.Int("pid", process.PID),
				zap.String("name", process.Name),
				zap.String("path", process.ExecutePath))

			// 优雅终止，超时后强制终止
			timeout := time.Duration(options.Timeout) * time.Second
			if err := KillProcessGracefully(process.PID, timeout); err != nil {
				logger.Warn("Failed to terminate process",
					zap.Int("pid", process.PID),
					zap.Error(err))
				// 继续处理其他进程，不返回错误
			} else {
				logger.Info("Successfully terminated process",
					zap.Int("pid", process.PID))
			}
		}

		// 等待一小段时间确保进程完全终止
		time.Sleep(1 * time.Second)

		// 再次检查端口是否释放
		if options.Port > 0 {
			for i := 0; i < 10; i++ { // 最多等待10秒
				if !IsPortInUse(options.Port) {
					break
				}
				time.Sleep(1 * time.Second)
			}

			if IsPortInUse(options.Port) {
				logger.Error("Port is still in use after terminating processes",
					zap.Int("port", options.Port))
				return errors.New("port " + strconv.Itoa(options.Port) + " is still in use after terminating processes")
			}
		}
	} else {
		logger.Info("No conflicting processes found")
	}

	return nil
}

// GetCurrentExecutablePath 获取当前可执行文件的路径 | Get current executable file path
func GetCurrentExecutablePath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	// 解析符号链接
	realPath, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		return execPath, nil // 如果解析失败，返回原路径
	}

	return realPath, nil
}

// ParsePortFromAddress 从地址字符串中解析端口号 | Parse port number from address string
func ParsePortFromAddress(address string) (int, error) {
	// 处理各种地址格式
	// :8080 -> 8080
	// 0.0.0.0:8080 -> 8080
	// localhost:8080 -> 8080
	// [::]:8080 -> 8080

	if strings.HasPrefix(address, ":") {
		// :8080 格式
		portStr := address[1:]
		return strconv.Atoi(portStr)
	}

	// 使用net包解析
	_, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return 0, fmt.Errorf("failed to parse address %s: %w", address, err)
	}

	return strconv.Atoi(portStr)
}
