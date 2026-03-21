//go:build darwin

package exec

import (
	"syscall"
)

// SysProcAttr macOS 进程属性配置 | macOS process attribute configuration
// 与 Linux 相同，设置进程组ID | Same as Linux, set process group ID
var SysProcAttr = &syscall.SysProcAttr{
	Setpgid: true,
}
