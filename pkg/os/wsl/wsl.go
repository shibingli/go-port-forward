// Package wsl 提供 WSL2 检测和集成工具
// Package wsl provides WSL2 detection and integration utilities
package wsl

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"unicode/utf16"

	pkgexec "go-port-forward/pkg/os/exec"
)

// Distro represents a WSL2 distribution.
type Distro struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Version string `json:"version"`
	Default bool   `json:"default"`
}

// Port represents a listening port inside a WSL2 distribution.
type Port struct {
	Protocol  string `json:"protocol"`
	Process   string `json:"process"`
	LocalAddr string `json:"local_addr"`
	Port      int    `json:"port"`
}

// decodeUTF16LE 将 UTF-16LE 编码的字节解码为 UTF-8 字符串
// Decode UTF-16LE encoded bytes to UTF-8 string
// Windows 的 wsl.exe 命令输出使用 UTF-16LE 编码，Go 的 exec.Command 读取原始字节，
// 需要手动解码才能正确进行字符串匹配
// Windows wsl.exe commands output UTF-16LE encoded text, Go's exec.Command reads raw bytes,
// manual decoding is required for correct string matching
func decodeUTF16LE(b []byte) string {
	// 移除 BOM（如果存在）| Remove BOM if present
	if len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE {
		b = b[2:]
	}

	// 确保字节长度为偶数 | Ensure byte length is even
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}

	// 将字节对转换为 uint16 切片 | Convert byte pairs to uint16 slice
	u16s := make([]uint16, len(b)/2)
	for i := range u16s {
		u16s[i] = binary.LittleEndian.Uint16(b[i*2:])
	}

	// 解码 UTF-16 为 rune 切片 | Decode UTF-16 to rune slice
	runes := utf16.Decode(u16s)
	return string(runes)
}

// isUTF16LE 检测字节是否为 UTF-16LE 编码 | Detect if bytes are UTF-16LE encoded
// 通过检查是否存在交替的 null 字节来判断 | Detects by checking for alternating null bytes
func isUTF16LE(b []byte) bool {
	if len(b) < 4 {
		return false
	}
	// 检查 BOM | Check BOM
	if b[0] == 0xFF && b[1] == 0xFE {
		return true
	}
	// 检查交替的 null 字节模式（ASCII 字符在 UTF-16LE 中为 XX 00）
	// Check alternating null byte pattern (ASCII chars in UTF-16LE are XX 00)
	nullCount := 0
	for i := 1; i < len(b) && i < 20; i += 2 {
		if b[i] == 0x00 {
			nullCount++
		}
	}
	return nullCount >= 3
}

// decodeOutput 自动检测并解码命令输出 | Auto-detect and decode command output
// 如果输出是 UTF-16LE 编码则解码，否则直接转换为字符串
// Decodes if output is UTF-16LE encoded, otherwise converts directly to string
func decodeOutput(b []byte) string {
	if isUTF16LE(b) {
		return decodeUTF16LE(b)
	}
	return string(b)
}

// IsWSL2Available 检查 WSL2 是否可用 | Check if WSL2 is available
// 仅在 Windows 平台上有效 | Only valid on Windows platform
func IsWSL2Available() bool {
	if runtime.GOOS != "windows" {
		return false
	}

	// 检查 wsl.exe 是否存在 | Check if wsl.exe exists
	_, err := exec.LookPath("wsl.exe")
	if err != nil {
		return false
	}

	// 策略1: 使用 wsl.exe --version（仅 WSL2 支持此命令）
	// Strategy 1: Use wsl.exe --version (only supported by WSL2)
	cmd := pkgexec.ExecCommand("wsl.exe", "--version")
	output, err := cmd.CombinedOutput()
	if err == nil {
		decoded := decodeOutput(output)
		// 输出包含 "WSL" 即表示 WSL2 可用（此命令仅在 WSL2 中存在）
		// Output containing "WSL" means WSL2 is available (this command only exists in WSL2)
		if strings.Contains(decoded, "WSL") {
			return true
		}
	}

	// 策略2: 使用 wsl.exe --status 作为后备
	// Strategy 2: Use wsl.exe --status as fallback
	cmd = pkgexec.ExecCommand("wsl.exe", "--status")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return false
	}

	decoded := decodeOutput(output)
	// 检查多种语言的版本标识 | Check version identifiers in multiple languages
	// 英文: "Default Version: 2" | 中文: "默认版本: 2"
	return strings.Contains(decoded, "WSL 2") ||
		strings.Contains(decoded, "WSL2") ||
		strings.Contains(decoded, ": 2")
}

// GetDefaultDistribution 获取默认的 WSL2 发行版 | Get default WSL2 distribution
func GetDefaultDistribution() (string, error) {
	if runtime.GOOS != "windows" {
		return "", errors.New("WSL2 is only available on Windows")
	}

	cmd := pkgexec.ExecCommand("wsl.exe", "--list", "--quiet")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list WSL distributions: %w", err)
	}

	// 解码 UTF-16LE 输出 | Decode UTF-16LE output
	decoded := decodeOutput(output)
	lines := strings.Split(decoded, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 移除 BOM 和 null 字符 | Remove BOM and null characters
		line = strings.Trim(line, "\x00\xef\xbb\xbf\uFEFF")
		if line != "" {
			return line, nil
		}
	}

	return "", errors.New("no WSL distribution found")
}

// ConvertWindowsPathToWSL 将 Windows 路径转换为 WSL 路径 | Convert Windows path to WSL path
// 例如: C:\Users\test -> /mnt/c/Users/test | Example: C:\Users\test -> /mnt/c/Users/test
func ConvertWindowsPathToWSL(windowsPath string) string {
	if runtime.GOOS != "windows" {
		return windowsPath
	}

	// 替换反斜杠为正斜杠 | Replace backslashes with forward slashes
	path := strings.ReplaceAll(windowsPath, "\\", "/")

	// 处理驱动器号 | Handle drive letter
	// C: -> /mnt/c
	if len(path) >= 2 && path[1] == ':' {
		drive := strings.ToLower(string(path[0]))
		path = "/mnt/" + drive + path[2:]
	}

	return path
}

// ConvertWSLPathToWindows 将 WSL 路径转换为 Windows 路径 | Convert WSL path to Windows path
// 例如: /mnt/c/Users/test -> C:\Users\test | Example: /mnt/c/Users/test -> C:\Users\test
func ConvertWSLPathToWindows(wslPath string) string {
	if runtime.GOOS != "windows" {
		return wslPath
	}

	// 检查是否为 /mnt/ 路径 | Check if it's a /mnt/ path
	if !strings.HasPrefix(wslPath, "/mnt/") {
		return wslPath
	}

	// 提取驱动器号 | Extract drive letter
	parts := strings.Split(wslPath, "/")
	if len(parts) < 3 {
		return wslPath
	}

	drive := strings.ToUpper(parts[2])
	remainingPath := strings.Join(parts[3:], "\\")

	return drive + ":\\" + remainingPath
}

// ExecuteInWSL 在 WSL2 中执行命令 | Execute command in WSL2
//
// 直接将命令和参数传递给 wsl.exe，不使用 bash -c 包装。
// 这样可以确保 WSL 默认 shell 的 PATH 被正确加载（包含 /usr/sbin 等路径），
// 避免非交互 bash -c 导致 PATH 不完整的问题。
//
// Passes command and args directly to wsl.exe without bash -c wrapper.
// This ensures the WSL default shell's PATH is properly loaded (including /usr/sbin etc.),
// avoiding the incomplete PATH issue with non-interactive bash -c.
func ExecuteInWSL(distro string, command string, args ...string) (string, error) {
	if runtime.GOOS != "windows" {
		return "", errors.New("WSL2 is only available on Windows")
	}

	// 构建 wsl.exe 参数列表 | Build wsl.exe argument list
	// wsl.exe [-d distro] -- command arg1 arg2 ...
	var wslArgs []string
	if distro != "" {
		wslArgs = append(wslArgs, "-d", distro)
	}
	wslArgs = append(wslArgs, "--", command)
	wslArgs = append(wslArgs, args...)

	cmd := pkgexec.ExecCommand("wsl.exe", wslArgs...)

	// 使用 Output() 而非 CombinedOutput()，避免 stderr 混入 stdout 导致解析失败
	// Use Output() instead of CombinedOutput() to prevent stderr from mixing into stdout
	output, err := cmd.Output()
	if err != nil {
		// 如果 Output() 失败，尝试获取 stderr 信息用于错误报告
		// If Output() fails, try to get stderr info for error reporting
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("failed to execute command in WSL: %w, stderr: %s", err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to execute command in WSL: %w", err)
	}

	// WSL2 内部命令输出通常是 UTF-8，但仍然尝试自动检测
	// WSL2 internal command output is typically UTF-8, but still try auto-detection
	return strings.TrimSpace(decodeOutput(output)), nil
}

// ExecuteInWSLShell 在 WSL2 中通过 login shell 执行命令 | Execute command in WSL2 via login shell
//
// 适用于需要 shell 特性（管道、重定向、变量展开等）的命令。
// 使用 bash -lc 确保加载完整的 PATH 和环境变量。
//
// For commands that need shell features (pipes, redirects, variable expansion, etc.).
// Uses bash -lc to ensure complete PATH and environment variables are loaded.
func ExecuteInWSLShell(distro string, shellCommand string) (string, error) {
	if runtime.GOOS != "windows" {
		return "", errors.New("WSL2 is only available on Windows")
	}

	var wslArgs []string
	if distro != "" {
		wslArgs = append(wslArgs, "-d", distro)
	}
	// 使用 bash -lc (login shell) 确保 /etc/profile 和 ~/.profile 被加载
	// Use bash -lc (login shell) to ensure /etc/profile and ~/.profile are sourced
	wslArgs = append(wslArgs, "--", "bash", "-lc", shellCommand)

	cmd := pkgexec.ExecCommand("wsl.exe", wslArgs...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("failed to execute shell command in WSL: %w, stderr: %s", err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to execute shell command in WSL: %w", err)
	}

	return strings.TrimSpace(decodeOutput(output)), nil
}

// GetWSLVersion 获取 WSL 版本信息 | Get WSL version information
func GetWSLVersion() (string, error) {
	if runtime.GOOS != "windows" {
		return "", errors.New("WSL2 is only available on Windows")
	}

	cmd := pkgexec.ExecCommand("wsl.exe", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get WSL version: %w", err)
	}

	// wsl.exe --version 输出为 UTF-16LE 编码 | wsl.exe --version output is UTF-16LE encoded
	return strings.TrimSpace(decodeOutput(output)), nil
}
