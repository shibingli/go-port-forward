//go:build darwin

package machineid

import (
	"errors"
	"io"
	"strings"

	"go-port-forward/pkg/pool"
)

// machineID 通过 `ioreg -rd1 -c IOPlatformExpertDevice` 读取 IOPlatformUUID。
// machineID returns the IOPlatformUUID from `ioreg -rd1 -c IOPlatformExpertDevice`.
func machineID() (string, error) {
	output, err := runIoreg(false)
	if err != nil {
		// cron 等最小化环境下 PATH 可能不包含 /usr/sbin，尝试使用绝对路径
		// ioreg is in /usr/sbin which may not be in PATH in minimal environments (e.g., cron)
		output, err = runIoreg(true)
		if err != nil {
			return "", err
		}
	}

	id, err := extractID(output)
	if err != nil {
		return "", err
	}

	id = trim(id)
	if id == "" {
		return "", errors.New("empty IOPlatformUUID")
	}
	return id, nil
}

// extractID 从 ioreg 输出中解析 IOPlatformUUID 值。
// extractID parses the IOPlatformUUID value from ioreg output.
// 期望格式 | Expected format: `  "IOPlatformUUID" = "XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX"`
func extractID(output string) (string, error) {
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "IOPlatformUUID") {
			continue
		}
		if _, after, found := strings.Cut(line, `" = "`); found {
			return strings.Trim(after, `"`), nil
		}
	}
	return "", errors.New(`failed to find "IOPlatformUUID" in ioreg output`)
}

// runIoreg 执行 ioreg 命令获取 IOPlatformExpertDevice 信息。
// 当 tryAbsolutePath 为 true 时使用 /usr/sbin/ioreg 绝对路径。
// stderr 写入 io.Discard：library 不应直接向进程 stderr 输出。
//
// runIoreg runs `ioreg -rd1 -c IOPlatformExpertDevice`.
// When tryAbsolutePath is true, uses /usr/sbin/ioreg instead of relying on PATH.
// stderr goes to io.Discard: libraries should not write directly to process stderr.
func runIoreg(tryAbsolutePath bool) (string, error) {
	cmd := "ioreg"
	if tryAbsolutePath {
		cmd = "/usr/sbin/ioreg"
	}
	buf := pool.GetByteBuffer()
	defer pool.PutByteBuffer(buf)
	if err := run(buf, io.Discard, cmd, "-rd1", "-c", "IOPlatformExpertDevice"); err != nil {
		return "", err
	}
	return buf.String(), nil
}
