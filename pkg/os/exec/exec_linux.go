//go:build linux

package exec

import (
	"syscall"
)

var SysProcAttr = &syscall.SysProcAttr{
	Setpgid: true,
}
