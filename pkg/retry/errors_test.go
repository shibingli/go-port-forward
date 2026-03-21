package retry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
)

// TestErrorCollector 测试错误收集器
func TestErrorCollector(t *testing.T) {
	collector := NewErrorCollector()

	if collector.Count() != 0 {
		t.Errorf("Expected 0 errors, got %d", collector.Count())
	}

	// 添加错误
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	collector.Add(err1)
	collector.Add(err2)

	if collector.Count() != 2 {
		t.Errorf("Expected 2 errors, got %d", collector.Count())
	}

	// 测试Error()方法
	errMsg := collector.Error()
	if !strings.Contains(errMsg, "error 1") || !strings.Contains(errMsg, "error 2") {
		t.Errorf("Error message should contain both errors: %s", errMsg)
	}

	// 测试Errors()方法
	errs := collector.Errors()
	if len(errs) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(errs))
	}
}

// TestDoWithErrorCollection 测试错误收集功能
func TestDoWithErrorCollection(t *testing.T) {
	ctx := context.Background()
	backoff := WithMaxRetries(3, MustNewConstant(1*time.Millisecond))

	attempts := 0
	collector := DoWithErrorCollection(ctx, backoff, func(ctx context.Context) error {
		attempts++
		return RetryableError(fmt.Errorf("attempt %d failed", attempts))
	})

	// 应该有4个错误（首次尝试 + 3次重试）
	if collector.Count() != 4 {
		t.Errorf("Expected 4 errors, got %d", collector.Count())
	}

	// 验证错误内容
	errs := collector.Errors()
	for i, err := range errs {
		expected := fmt.Sprintf("attempt %d failed", i+1)
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("Error %d should contain %q, got %q", i, expected, err.Error())
		}
	}
}

// TestDoWithErrorCollection_Success 测试成功时的错误收集
func TestDoWithErrorCollection_Success(t *testing.T) {
	ctx := context.Background()
	backoff := MustNewConstant(1 * time.Millisecond)

	attempts := 0
	collector := DoWithErrorCollection(ctx, backoff, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return RetryableError(fmt.Errorf("attempt %d failed", attempts))
		}
		return nil
	})

	// 应该有2个错误（前两次失败）
	if collector.Count() != 2 {
		t.Errorf("Expected 2 errors, got %d", collector.Count())
	}
}

// TestDoWithCallback 测试回调功能
func TestDoWithCallback(t *testing.T) {
	ctx := context.Background()
	backoff := WithMaxRetries(3, MustNewConstant(10*time.Millisecond))

	callbackCalls := 0
	var callbackAttempts []int
	var callbackErrors []error
	var callbackWaits []time.Duration

	callback := func(attempt int, err error, nextWait time.Duration) {
		callbackCalls++
		callbackAttempts = append(callbackAttempts, attempt)
		callbackErrors = append(callbackErrors, err)
		callbackWaits = append(callbackWaits, nextWait)
	}

	attempts := 0
	err := DoWithCallback(ctx, backoff, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return RetryableError(fmt.Errorf("attempt %d failed", attempts))
		}
		return nil
	}, callback)

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	// 应该调用2次回调（前两次失败后）
	if callbackCalls != 2 {
		t.Errorf("Expected 2 callback calls, got %d", callbackCalls)
	}

	// 验证回调参数
	for i := 0; i < callbackCalls; i++ {
		if callbackAttempts[i] != i+1 {
			t.Errorf("Callback %d: expected attempt %d, got %d", i, i+1, callbackAttempts[i])
		}
		if callbackWaits[i] != 10*time.Millisecond {
			t.Errorf("Callback %d: expected wait %v, got %v", i, 10*time.Millisecond, callbackWaits[i])
		}
	}
}

// TestDoWithCallback_NilCallback 测试nil回调
func TestDoWithCallback_NilCallback(t *testing.T) {
	ctx := context.Background()
	backoff := MustNewConstant(1 * time.Millisecond)

	attempts := 0
	err := DoWithCallback(ctx, backoff, func(ctx context.Context) error {
		attempts++
		if attempts < 2 {
			return RetryableError(errors.New("fail"))
		}
		return nil
	}, nil) // nil回调

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

// TestDoWithPanicRecovery 测试panic恢复
func TestDoWithPanicRecovery(t *testing.T) {
	ctx := context.Background()
	backoff := WithMaxRetries(3, MustNewConstant(1*time.Millisecond))

	attempts := 0
	err := DoWithPanicRecovery(ctx, backoff, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			panic(fmt.Sprintf("panic at attempt %d", attempts))
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected success after panic recovery, got error: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

// TestDoWithPanicRecovery_PanicError 测试panic错误类型
func TestDoWithPanicRecovery_PanicError(t *testing.T) {
	ctx := context.Background()
	backoff := WithMaxRetries(2, MustNewConstant(1*time.Millisecond))

	err := DoWithPanicRecovery(ctx, backoff, func(ctx context.Context) error {
		panic("always panic")
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// 检查是否是PanicError - Go 1.26+ errors.AsType | Go 1.26+ errors.AsType
	panicErr, ok := errors.AsType[*PanicError](err)
	if !ok {
		t.Errorf("Expected PanicError, got %T", err)
	}

	if panicErr.Value != "always panic" {
		t.Errorf("Expected panic value 'always panic', got %v", panicErr.Value)
	}
}

// TestDoWithPanicRecovery_NormalError 测试正常错误不受影响
func TestDoWithPanicRecovery_NormalError(t *testing.T) {
	ctx := context.Background()
	backoff := MustNewConstant(1 * time.Millisecond)

	expectedErr := errors.New("normal error")
	err := DoWithPanicRecovery(ctx, backoff, func(ctx context.Context) error {
		return expectedErr
	})

	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

// TestCombinedFeatures 测试组合使用多个功能
func TestCombinedFeatures(t *testing.T) {
	ctx := context.Background()
	backoff := WithMaxRetries(5, WithCappedDuration(50*time.Millisecond, MustNewExponential(10*time.Millisecond)))

	callbackCalls := 0
	callback := func(attempt int, err error, nextWait time.Duration) {
		callbackCalls++
		t.Logf("Attempt %d failed: %v, next wait: %v", attempt, err, nextWait)
	}

	attempts := 0
	err := DoWithCallback(ctx, backoff, func(ctx context.Context) error {
		attempts++
		if attempts < 4 {
			return RetryableError(fmt.Errorf("attempt %d failed", attempts))
		}
		return nil
	}, callback)

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	if attempts != 4 {
		t.Errorf("Expected 4 attempts, got %d", attempts)
	}

	if callbackCalls != 3 {
		t.Errorf("Expected 3 callback calls, got %d", callbackCalls)
	}
}

// BenchmarkDoWithErrorCollection 性能测试：错误收集
func BenchmarkDoWithErrorCollection(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff := WithMaxRetries(10, MustNewConstant(1*time.Nanosecond))
		attempts := 0
		_ = DoWithErrorCollection(ctx, backoff, func(ctx context.Context) error {
			attempts++
			return RetryableError(errors.New("fail"))
		})
	}
}

// BenchmarkDoWithCallback 性能测试：回调
func BenchmarkDoWithCallback(b *testing.B) {
	ctx := context.Background()
	callback := func(attempt int, err error, nextWait time.Duration) {}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff := WithMaxRetries(10, MustNewConstant(1*time.Nanosecond))
		attempts := 0
		_ = DoWithCallback(ctx, backoff, func(ctx context.Context) error {
			attempts++
			return RetryableError(errors.New("fail"))
		}, callback)
	}
}

// TestErrorCollector_Concurrent 测试ErrorCollector的并发安全性
func TestErrorCollector_Concurrent(t *testing.T) {
	collector := NewErrorCollector()

	const numGoroutines = 100

	// 使用 errgroup 并发添加错误，SetLimit 控制最大并发数
	// Use errgroup to add errors concurrently, SetLimit controls max concurrency
	g := new(errgroup.Group)
	g.SetLimit(20) // 限制最大 20 个 goroutine 并发 | Limit max 20 concurrent goroutines
	for i := 0; i < numGoroutines; i++ {
		id := i
		g.Go(func() error {
			collector.Add(fmt.Errorf("error %d", id))
			return nil
		})
	}
	_ = g.Wait()

	// 验证错误数量
	if collector.Count() != numGoroutines {
		t.Errorf("Expected %d errors, got %d", numGoroutines, collector.Count())
	}

	// 验证可以安全获取错误列表
	errors := collector.Errors()
	if len(errors) != numGoroutines {
		t.Errorf("Expected %d errors in list, got %d", numGoroutines, len(errors))
	}

	// 验证Error()方法
	errStr := collector.Error()
	if !strings.Contains(errStr, fmt.Sprintf("multiple errors (%d)", numGoroutines)) {
		t.Errorf("Error string doesn't contain expected count: %s", errStr)
	}
}

// TestErrorCollector_ConcurrentReadWrite 测试并发读写
func TestErrorCollector_ConcurrentReadWrite(t *testing.T) {
	collector := NewErrorCollector()

	const numWriters = 50
	const numReaders = 50

	// 使用 errgroup 统一管理读写 goroutine，无需手动 Add/Done
	// Use errgroup to manage both read and write goroutines, no manual Add/Done
	g := new(errgroup.Group)
	g.SetLimit(30) // 限制最大并发数防止资源过载 | Limit max concurrency to prevent resource exhaustion

	// 启动写入goroutine
	for i := 0; i < numWriters; i++ {
		id := i
		g.Go(func() error {
			for j := 0; j < 10; j++ {
				collector.Add(fmt.Errorf("error %d-%d", id, j))
			}
			return nil
		})
	}

	// 启动读取goroutine
	for i := 0; i < numReaders; i++ {
		g.Go(func() error {
			for j := 0; j < 10; j++ {
				_ = collector.Count()
				_ = collector.Errors()
				_ = collector.Error()
			}
			return nil
		})
	}

	_ = g.Wait()

	// 验证最终错误数量
	expectedCount := numWriters * 10
	if collector.Count() != expectedCount {
		t.Errorf("Expected %d errors, got %d", expectedCount, collector.Count())
	}
}
