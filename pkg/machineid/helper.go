package machineid

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// cmdTimeout 外部命令的最大执行时间，防止 ioreg/kenv 等命令挂起时阻塞进程
// Maximum execution time for external commands to prevent process blocking
// when ioreg/kenv or similar commands hang.
const cmdTimeout = 5 * time.Second

// run 使用默认超时执行外部命令，将 stdout/stderr 写入指定 Writer。
// 不继承父进程 stdin，避免在守护进程场景下引发问题。
//
// Execute external command with default timeout, writing stdout/stderr to given writers.
// Does not inherit parent process stdin to avoid issues in daemon scenarios.
func run(stdout, stderr io.Writer, cmd string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	c := exec.CommandContext(ctx, cmd, args...)
	c.Stdout = stdout
	c.Stderr = stderr
	return c.Run()
}

// protect 使用 HMAC-SHA256 对 appID 进行哈希，以 machine ID 为密钥，返回十六进制编码字符串。
// Calculate HMAC-SHA256 of the application ID, keyed by the machine ID, returns hex-encoded string.
func protect(appID, id string) string {
	mac := hmac.New(sha256.New, []byte(id))
	mac.Write([]byte(appID))
	return hex.EncodeToString(mac.Sum(nil))
}

// readFile 读取指定文件的全部内容 | Read entire contents of the specified file.
func readFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

// writeFile 以 0600 权限写入文件（machine-id 属于敏感信息，仅属主可读写）。
// Write file with 0600 permission (machine-id is sensitive, owner-only read/write).
func writeFile(filename string, data []byte) error {
	return os.WriteFile(filename, data, 0600)
}

// readFirstFile 按顺序尝试读取路径列表中的文件，返回第一个可读文件的内容。
// 空路径会被跳过；若所有路径均不可读或列表为空，返回错误。
//
// Try reading files in order from pathnames, return contents of the first readable file.
// Empty pathnames are skipped; returns error if all paths fail or list is empty.
func readFirstFile(pathnames []string) ([]byte, error) {
	var lastErr error
	for _, pathname := range pathnames {
		if pathname == "" {
			continue
		}
		data, err := readFile(pathname)
		if err == nil {
			return data, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("no valid machine-id file paths available")
	}
	return nil, lastErr
}

// writeFirstFile 按顺序尝试写入路径列表中的第一个可写文件。
// 空路径会被跳过。
//
// Try writing to the first writable file among listed pathnames.
// Empty pathnames are skipped.
func writeFirstFile(pathnames []string, data []byte) error {
	var lastErr error
	for _, pathname := range pathnames {
		if pathname == "" {
			continue
		}
		if err := writeFile(pathname, data); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

// trim 移除字符串首尾的所有空白字符（包括换行符）。
// Remove all leading and trailing whitespace (including newlines) from s.
func trim(s string) string {
	return strings.TrimSpace(s)
}
