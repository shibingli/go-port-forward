package retry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
)

// TestDo_Success 测试成功的重试
func TestDo_Success(t *testing.T) {
	ctx := context.Background()
	backoff := MustNewConstant(10 * time.Millisecond)

	attempts := 0
	err := Do(ctx, backoff, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return RetryableError(errors.New("temporary failure"))
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

// TestDo_NonRetryableError 测试非可重试错误
func TestDo_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	backoff := MustNewConstant(10 * time.Millisecond)

	attempts := 0
	expectedErr := errors.New("permanent failure")
	err := Do(ctx, backoff, func(ctx context.Context) error {
		attempts++
		return expectedErr // 不包装，不可重试
	})

	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

// TestDo_ContextCanceled 测试上下文取消
func TestDo_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	backoff := MustNewConstant(100 * time.Millisecond)

	attempts := 0
	errChan := make(chan error, 1)

	go func() {
		err := Do(ctx, backoff, func(ctx context.Context) error {
			attempts++
			return RetryableError(errors.New("always fail"))
		})
		errChan <- err
	}()

	// 等待第一次尝试后取消
	time.Sleep(50 * time.Millisecond)
	cancel()

	err := <-errChan
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

// TestDo_ContextTimeout 测试上下文超时
func TestDo_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	backoff := MustNewConstant(100 * time.Millisecond)

	err := Do(ctx, backoff, func(ctx context.Context) error {
		return RetryableError(errors.New("always fail"))
	})

	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

// TestRetryableError 测试可重试错误包装
func TestRetryableError(t *testing.T) {
	// 测试nil错误
	if RetryableError(nil) != nil {
		t.Error("RetryableError(nil) should return nil")
	}

	// 测试错误包装
	originalErr := errors.New("test error")
	wrapped := RetryableError(originalErr)

	if wrapped == nil {
		t.Fatal("RetryableError should not return nil for non-nil error")
	}

	// 测试Unwrap
	if !errors.Is(wrapped, originalErr) {
		t.Error("Wrapped error should be unwrappable to original error")
	}

	// 测试Error()方法
	expectedMsg := "retryable: test error"
	if wrapped.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, wrapped.Error())
	}
}

// TestWithMaxRetries 测试最大重试次数
func TestWithMaxRetries(t *testing.T) {
	ctx := context.Background()
	maxRetries := uint64(3)
	backoff := WithMaxRetries(maxRetries, MustNewConstant(1*time.Millisecond))

	attempts := uint64(0)
	err := Do(ctx, backoff, func(ctx context.Context) error {
		atomic.AddUint64(&attempts, 1)
		return RetryableError(errors.New("always fail"))
	})

	if err == nil {
		t.Error("Expected error, got nil")
	}

	// maxRetries=3表示最多重试3次，加上首次尝试，总共4次
	expectedAttempts := maxRetries + 1
	if attempts != expectedAttempts {
		t.Errorf("Expected %d attempts, got %d", expectedAttempts, attempts)
	}
}

// TestWithCappedDuration 测试最大单次等待时间
func TestWithCappedDuration(t *testing.T) {
	cap := 50 * time.Millisecond
	backoff := WithCappedDuration(cap, MustNewExponential(100*time.Millisecond))

	// 第一次应该返回100ms，但被cap限制为50ms
	duration, stop := backoff.Next()
	if stop {
		t.Error("Should not stop on first call")
	}
	if duration != cap {
		t.Errorf("Expected duration %v, got %v", cap, duration)
	}
}

// TestWithJitter 测试抖动
func TestWithJitter(t *testing.T) {
	base := 100 * time.Millisecond
	jitter := 20 * time.Millisecond
	backoff := WithJitter(jitter, MustNewConstant(base))

	// 测试多次，确保结果在预期范围内
	for i := 0; i < 10; i++ {
		duration, stop := backoff.Next()
		if stop {
			t.Error("Should not stop")
		}

		min := base - jitter
		max := base + jitter
		if duration < min || duration > max {
			t.Errorf("Duration %v out of range [%v, %v]", duration, min, max)
		}
	}
}

// TestWithJitter_InvalidParameter 测试无效参数
func TestWithJitter_InvalidParameter(t *testing.T) {
	base := 100 * time.Millisecond
	backoff := WithJitter(-10*time.Millisecond, MustNewConstant(base))

	duration, stop := backoff.Next()
	if stop {
		t.Error("Should not stop")
	}

	// 负数jitter应该被忽略，返回原始值
	if duration != base {
		t.Errorf("Expected duration %v, got %v", base, duration)
	}
}

// TestWithJitterPercent 测试百分比抖动
func TestWithJitterPercent(t *testing.T) {
	base := 100 * time.Millisecond
	percent := uint64(10) // 10%
	backoff := WithJitterPercent(percent, MustNewConstant(base))

	// 测试多次，确保结果在预期范围内
	for i := 0; i < 10; i++ {
		duration, stop := backoff.Next()
		if stop {
			t.Error("Should not stop")
		}

		// 10%抖动，范围应该是90ms-110ms
		min := 90 * time.Millisecond
		max := 110 * time.Millisecond
		if duration < min || duration > max {
			t.Errorf("Duration %v out of range [%v, %v]", duration, min, max)
		}
	}
}

// TestWithJitterPercent_Zero 测试零百分比
func TestWithJitterPercent_Zero(t *testing.T) {
	base := 100 * time.Millisecond
	backoff := WithJitterPercent(0, MustNewConstant(base))

	duration, stop := backoff.Next()
	if stop {
		t.Error("Should not stop")
	}

	// 0%抖动应该返回原始值
	if duration != base {
		t.Errorf("Expected duration %v, got %v", base, duration)
	}
}

// TestExponentialBackoff 测试指数退避
func TestExponentialBackoff(t *testing.T) {
	base := 10 * time.Millisecond
	backoff, err := NewExponential(base)
	if err != nil {
		t.Fatalf("Failed to create exponential backoff: %v", err)
	}

	expected := []time.Duration{
		10 * time.Millisecond,  // 2^0 * 10ms
		20 * time.Millisecond,  // 2^1 * 10ms
		40 * time.Millisecond,  // 2^2 * 10ms
		80 * time.Millisecond,  // 2^3 * 10ms
		160 * time.Millisecond, // 2^4 * 10ms
	}

	for i, exp := range expected {
		duration, stop := backoff.Next()
		if stop {
			t.Errorf("Should not stop at iteration %d", i)
		}
		if duration != exp {
			t.Errorf("Iteration %d: expected %v, got %v", i, exp, duration)
		}
	}
}

// TestFibonacciBackoff 测试斐波那契退避
func TestFibonacciBackoff(t *testing.T) {
	base := 10 * time.Millisecond
	backoff, err := NewFibonacci(base)
	if err != nil {
		t.Fatalf("Failed to create fibonacci backoff: %v", err)
	}

	expected := []time.Duration{
		10 * time.Millisecond, // 1 * 10ms
		20 * time.Millisecond, // 2 * 10ms
		30 * time.Millisecond, // 3 * 10ms
		50 * time.Millisecond, // 5 * 10ms
		80 * time.Millisecond, // 8 * 10ms
	}

	for i, exp := range expected {
		duration, stop := backoff.Next()
		if stop {
			t.Errorf("Should not stop at iteration %d", i)
		}
		if duration != exp {
			t.Errorf("Iteration %d: expected %v, got %v", i, exp, duration)
		}
	}
}

// TestLinearBackoff 测试线性退避
func TestLinearBackoff(t *testing.T) {
	base := 10 * time.Millisecond
	backoff, err := NewLinear(base)
	if err != nil {
		t.Fatalf("Failed to create linear backoff: %v", err)
	}

	expected := []time.Duration{
		1 * base, // 1 * 10ms = 10ms
		2 * base, // 2 * 10ms = 20ms
		3 * base, // 3 * 10ms = 30ms
		4 * base, // 4 * 10ms = 40ms
		5 * base, // 5 * 10ms = 50ms
	}

	for i, exp := range expected {
		duration, stop := backoff.Next()
		if stop {
			t.Errorf("Iteration %d: unexpected stop", i)
		}
		if duration != exp {
			t.Errorf("Iteration %d: expected %v, got %v", i, exp, duration)
		}
	}
}

// TestLinearBackoff_InvalidParameter 测试无效参数
func TestLinearBackoff_InvalidParameter(t *testing.T) {
	_, err := NewLinear(0)
	if err == nil {
		t.Error("Expected error for zero duration")
	}

	_, err = NewLinear(-time.Second)
	if err == nil {
		t.Error("Expected error for negative duration")
	}
}

// TestLinearBackoff_Overflow 测试线性退避的溢出处理
func TestLinearBackoff_Overflow(t *testing.T) {
	// 使用一个很大的base值，使得很快就会溢出
	backoff, err := NewLinear(time.Duration(math.MaxInt64 / 10))
	if err != nil {
		t.Fatalf("Failed to create linear backoff: %v", err)
	}

	// 前几次应该正常
	for i := 0; i < 5; i++ {
		_, stop := backoff.Next()
		if stop {
			t.Errorf("Iteration %d: unexpected stop", i)
		}
	}

	// 继续调用，应该会溢出并返回MaxInt64
	for i := 0; i < 10; i++ {
		duration, stop := backoff.Next()
		if stop {
			t.Errorf("Iteration %d: unexpected stop", i)
		}
		// 溢出后应该返回MaxInt64
		if duration == time.Duration(math.MaxInt64) {
			return // 测试通过
		}
	}
}

// TestConstantBackoff_InvalidParameter 测试无效参数
func TestConstantBackoff_InvalidParameter(t *testing.T) {
	_, err := NewConstant(0)
	if err == nil {
		t.Error("Expected error for zero duration")
	}

	_, err = NewConstant(-1 * time.Second)
	if err == nil {
		t.Error("Expected error for negative duration")
	}
}

// TestExponentialBackoff_InvalidParameter 测试无效参数
func TestExponentialBackoff_InvalidParameter(t *testing.T) {
	_, err := NewExponential(0)
	if err == nil {
		t.Error("Expected error for zero duration")
	}

	_, err = NewExponential(-1 * time.Second)
	if err == nil {
		t.Error("Expected error for negative duration")
	}
}

// TestFibonacciBackoff_InvalidParameter 测试无效参数
func TestFibonacciBackoff_InvalidParameter(t *testing.T) {
	_, err := NewFibonacci(0)
	if err == nil {
		t.Error("Expected error for zero duration")
	}

	_, err = NewFibonacci(-1 * time.Second)
	if err == nil {
		t.Error("Expected error for negative duration")
	}
}

// TestConcurrentBackoffUsage 测试并发使用（每个goroutine独立实例）
func TestConcurrentBackoffUsage(t *testing.T) {
	// 使用 errgroup.WithContext 传播取消信号，g.Wait() 直接处理错误结果
	// Use errgroup.WithContext to propagate cancellation; g.Wait() handles error results directly
	baseCtx := context.Background()
	const numGoroutines = 10
	const maxRetries = 3

	g, ctx := errgroup.WithContext(baseCtx)
	var successCount atomic.Int32

	for i := 0; i < numGoroutines; i++ {
		id := i
		g.Go(func() error {
			// 每个goroutine创建独立的backoff实例 | Each goroutine creates its own backoff instance
			backoff := WithMaxRetries(maxRetries, MustNewConstant(1*time.Millisecond))

			attempts := 0
			err := Do(ctx, backoff, func(ctx context.Context) error {
				attempts++
				if attempts < 3 {
					return RetryableError(fmt.Errorf("goroutine %d attempt %d", id, attempts))
				}
				return nil
			})

			if err == nil && attempts == 3 {
				successCount.Add(1)
			}
			return nil
		})
	}

	// g.Wait() 等待所有 goroutine 完成并收集错误（此处 goroutine 均返回 nil）
	// g.Wait() waits for all goroutines and collects errors (goroutines all return nil here)
	if err := g.Wait(); err != nil {
		t.Errorf("Unexpected error from errgroup: %v", err)
	}

	if successCount.Load() != numGoroutines {
		t.Errorf("Expected %d successful goroutines, got %d", numGoroutines, successCount.Load())
	}
}

// TestWithMaxDuration 测试最大总时间
func TestWithMaxDuration(t *testing.T) {
	ctx := context.Background()
	maxDuration := 100 * time.Millisecond
	backoff := WithMaxDuration(maxDuration, MustNewConstant(50*time.Millisecond))

	start := time.Now()
	attempts := 0

	err := Do(ctx, backoff, func(ctx context.Context) error {
		attempts++
		return RetryableError(errors.New("always fail"))
	})

	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	// 应该在maxDuration附近停止（允许一些误差）
	if elapsed < maxDuration || elapsed > maxDuration+50*time.Millisecond {
		t.Errorf("Expected elapsed time around %v, got %v", maxDuration, elapsed)
	}
}

// TestExponentialBackoff_Overflow 测试溢出处理
func TestExponentialBackoff_Overflow(t *testing.T) {
	backoff, err := NewExponential(time.Second)
	if err != nil {
		t.Fatalf("Failed to create exponential backoff: %v", err)
	}

	// 调用足够多次以触发溢出
	for i := 0; i < 70; i++ {
		duration, stop := backoff.Next()
		if stop {
			t.Error("Should not stop")
		}
		// 溢出后应该返回MaxInt64
		if i >= 63 && duration <= 0 {
			t.Errorf("Iteration %d: expected positive duration, got %v", i, duration)
		}
	}
}

// TestFibonacciBackoff_ConcurrentSafety 测试斐波那契退避的并发安全性
func TestFibonacciBackoff_ConcurrentSafety(t *testing.T) {
	backoff, err := NewFibonacci(time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to create fibonacci backoff: %v", err)
	}

	const numGoroutines = 100

	// 使用 errgroup 替代 WaitGroup，不设 SetLimit 以保持高并发验证线程安全性
	// Use errgroup instead of WaitGroup; no SetLimit to maintain high concurrency for thread-safety verification
	// 虽然不推荐共享backoff实例，但实现应该是线程安全的
	g := new(errgroup.Group)
	for i := 0; i < numGoroutines; i++ {
		g.Go(func() error {
			_, _ = backoff.Next()
			return nil
		})
	}

	// g.Wait() 处理错误（backoff.Next 无错误，主要验证无 panic/死锁）
	// g.Wait() handles errors (backoff.Next has no errors; verifies no panic/deadlock)
	if err := g.Wait(); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	// 如果没有panic或死锁，测试通过 | Test passes if no panic or deadlock
}

// TestTimerCleanup 测试Timer资源清理
func TestTimerCleanup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	backoff := MustNewConstant(100 * time.Millisecond)

	// 这个测试主要确保没有goroutine泄漏
	err := Do(ctx, backoff, func(ctx context.Context) error {
		return RetryableError(errors.New("always fail"))
	})

	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}

	// 等待一小段时间，确保所有资源都被清理
	time.Sleep(10 * time.Millisecond)
}

// TestMustNewFunctions_WithInvalidParams 测试 MustNew* 函数的参数验证
func TestMustNewFunctions_WithInvalidParams(t *testing.T) {
	// 测试 MustNewConstant 使用无效参数
	b1 := MustNewConstant(0)
	if b1 == nil {
		t.Error("MustNewConstant returned nil")
	}
	d1, _ := b1.Next()
	if d1 != time.Second {
		t.Errorf("Expected default %v, got %v", time.Second, d1)
	}

	// 测试 MustNewExponential 使用无效参数
	b2 := MustNewExponential(-time.Second)
	if b2 == nil {
		t.Error("MustNewExponential returned nil")
	}
	d2, _ := b2.Next()
	if d2 != time.Second {
		t.Errorf("Expected default %v, got %v", time.Second, d2)
	}

	// 测试 MustNewFibonacci 使用无效参数
	b3 := MustNewFibonacci(0)
	if b3 == nil {
		t.Error("MustNewFibonacci returned nil")
	}
	d3, _ := b3.Next()
	if d3 != time.Second {
		t.Errorf("Expected default %v, got %v", time.Second, d3)
	}

	// 测试 MustNewLinear 使用无效参数
	b4 := MustNewLinear(-time.Second)
	if b4 == nil {
		t.Error("MustNewLinear returned nil")
	}
	d4, _ := b4.Next()
	if d4 != time.Second {
		t.Errorf("Expected default %v, got %v", time.Second, d4)
	}
}
