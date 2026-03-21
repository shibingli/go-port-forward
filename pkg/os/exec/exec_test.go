package exec

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestRunCmd(t *testing.T) {
	// 这是一个特定环境的测试，检查可执行文件是否存在
	exePath := "D:\\AppData\\Local\\Temp\\qdi\\bin\\qdi_ai_server_cuda.exe"
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		t.Skipf("Skipping test: executable not found at %s", exePath)
		return
	}

	cmd := ExecCommand(exePath, "--host", "127.0.0.1", "--port", "57597", "--threads", "8", "--batch-size", "512", "--parallel", "1", "-m", "D:\\developer\\gpt\\llm_models\\BAAI\\bge-reranker-v2-m3-gguf\\bge-reranker-v2-m3-f16.gguf", "--alias", "bge-reranker-v2-m3", "--rerank", "--pooling", "rank", "--ctx-size", "0", "--predict", "-1", "--n-gpu-layers", "25", "--tensor-split", "25")

	envs := make([]string, 0)

	if cmd.Env != nil && len(cmd.Env) > 0 {
		envs = append(envs, cmd.Env...)
	}

	if cmd.Environ() != nil && len(cmd.Environ()) > 0 {
		envs = append(envs, cmd.Environ()...)
	}

	envs = append(envs, fmt.Sprintf("CUDA_VISIBLE_DEVICES=%s", "0"))

	cmd.Env = envs

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	//cmd.Stdout = nil
	//cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
		return
	}

	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
		return
	}
}

// TestCommandContext 测试带上下文的命令执行
func TestCommandContext(t *testing.T) {
	// 根据操作系统选择合适的测试命令
	var cmdName string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmdName = "cmd"
		args = []string{"/c", "echo", "Hello, World!"}
	default:
		cmdName = "echo"
		args = []string{"Hello, World!"}
	}

	t.Run("NormalExecution", func(t *testing.T) {
		ctx := context.Background()
		cmd := CommandContext(ctx, cmdName, args...)

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Command execution failed: %v", err)
		}

		if len(output) == 0 {
			t.Error("Expected output, got empty")
		}
	})

	t.Run("ContextTimeout", func(t *testing.T) {
		// 创建一个会超时的上下文
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		// 等待一下确保上下文超时
		time.Sleep(10 * time.Millisecond)

		var sleepCmd string
		var sleepArgs []string

		switch runtime.GOOS {
		case "windows":
			sleepCmd = "cmd"
			sleepArgs = []string{"/c", "timeout", "/t", "5", "/nobreak"}
		default:
			sleepCmd = "sleep"
			sleepArgs = []string{"5"}
		}

		cmd := CommandContext(ctx, sleepCmd, sleepArgs...)
		err := cmd.Run()

		// 应该返回上下文超时错误
		if err == nil {
			t.Error("Expected context timeout error, got nil")
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		var sleepCmd string
		var sleepArgs []string

		switch runtime.GOOS {
		case "windows":
			sleepCmd = "cmd"
			sleepArgs = []string{"/c", "timeout", "/t", "10", "/nobreak"}
		default:
			sleepCmd = "sleep"
			sleepArgs = []string{"10"}
		}

		cmd := CommandContext(ctx, sleepCmd, sleepArgs...)

		// 在另一个goroutine中启动命令
		errChan := make(chan error, 1)
		go func() {
			errChan <- cmd.Run()
		}()

		// 等待一小段时间后取消上下文
		time.Sleep(100 * time.Millisecond)
		cancel()

		// 等待命令完成
		err := <-errChan

		// 应该返回上下文取消错误
		if err == nil {
			t.Error("Expected context cancellation error, got nil")
		}
	})
}

// TestExecCommand 测试基本命令执行
func TestExecCommand(t *testing.T) {
	var cmdName string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmdName = "cmd"
		args = []string{"/c", "echo", "Test"}
	default:
		cmdName = "echo"
		args = []string{"Test"}
	}

	cmd := ExecCommand(cmdName, args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	if len(output) == 0 {
		t.Error("Expected output, got empty")
	}
}
