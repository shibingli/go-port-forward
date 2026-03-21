package exec

import (
	"context"
	"os/exec"
)

// ExecCommand 创建一个命令，使用平台特定的进程属性 | Create a command with platform-specific process attributes
func ExecCommand(binPath string, args ...string) (cmd *exec.Cmd) {
	cmd = exec.Command(binPath, args...)
	cmd.SysProcAttr = SysProcAttr
	return
}

// CommandContext 创建一个带上下文的命令，使用平台特定的进程属性 | Create a command with context and platform-specific process attributes
func CommandContext(ctx context.Context, binPath string, args ...string) (cmd *exec.Cmd) {
	cmd = exec.CommandContext(ctx, binPath, args...)
	cmd.SysProcAttr = SysProcAttr
	return
}
