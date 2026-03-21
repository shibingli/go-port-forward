package retry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ErrorCollector 错误收集器，用于收集重试过程中的所有错误 | Error collector for collecting all errors during retry
// 并发安全：所有方法都使用读写锁保护 | Concurrency-safe: all methods are protected by read-write locks
// 性能优化：使用 sync.RWMutex 提升并发读性能
type ErrorCollector struct {
	errors []error
	mu     sync.RWMutex
}

// NewErrorCollector 创建一个新的错误收集器 | Create a new error collector
func NewErrorCollector() *ErrorCollector {
	return &ErrorCollector{
		errors: make([]error, 0),
	}
}

// Add 添加一个错误到收集器 | Add an error to the collector
// 并发安全：可以从多个goroutine同时调用 | Concurrency-safe: can be called from multiple goroutines
func (ec *ErrorCollector) Add(err error) {
	if err != nil {
		ec.mu.Lock() // 写锁
		defer ec.mu.Unlock()
		ec.errors = append(ec.errors, err)
	}
}

// Errors 返回所有收集的错误的副本 | Return a copy of all collected errors
// 并发安全：返回的是副本，外部修改不会影响内部状态 | Concurrency-safe: returns a copy, external modifications won't affect internal state
// 性能优化：使用读锁，允许多个goroutine并发读取
func (ec *ErrorCollector) Errors() []error {
	ec.mu.RLock() // 读锁
	defer ec.mu.RUnlock()

	// 返回副本，防止外部修改
	result := make([]error, len(ec.errors))
	copy(result, ec.errors)
	return result
}

// Count 返回收集的错误数量 | Return the number of collected errors
// 并发安全：使用读锁保护 | Concurrency-safe: protected by read lock
// 性能优化：使用读锁，允许多个goroutine并发读取
func (ec *ErrorCollector) Count() int {
	ec.mu.RLock() // 读锁
	defer ec.mu.RUnlock()
	return len(ec.errors)
}

// Error 实现error接口，返回所有错误的组合信息 | Implement error interface, return combined info of all errors
// 并发安全：使用读锁保护 | Concurrency-safe: protected by read lock
// 性能优化：使用读锁，允许多个goroutine并发读取
func (ec *ErrorCollector) Error() string {
	ec.mu.RLock() // 读锁
	defer ec.mu.RUnlock()

	if len(ec.errors) == 0 {
		return "no errors"
	}

	if len(ec.errors) == 1 {
		return ec.errors[0].Error()
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("multiple errors (%d):\n", len(ec.errors)))
	for i, err := range ec.errors {
		sb.WriteString(fmt.Sprintf("  [%d] %v\n", i+1, err))
	}
	return sb.String()
}

// DoWithErrorCollection 使用退避算法包装函数进行重试，并收集所有错误 | Retry with backoff and collect all errors
// 与Do()不同，这个函数会收集重试过程中的所有错误 | Unlike Do(), this function collects all errors during retry
// 参数:
//   - ctx: 上下文，用于取消和超时控制
//   - b: 退避算法
//   - f: 要重试的函数
//
// 返回:
//   - *ErrorCollector: 错误收集器，包含所有重试过程中的错误
func DoWithErrorCollection(ctx context.Context, b Backoff, f RetryFunc) *ErrorCollector {
	collector := NewErrorCollector()

	for {
		// Return immediately if ctx is canceled
		select {
		case <-ctx.Done():
			collector.Add(ctx.Err())
			return collector
		default:
		}

		err := f(ctx)
		if err == nil {
			return collector
		}

		// 收集错误
		collector.Add(err)

		// Not retryable - Go 1.26+ errors.AsType | Go 1.26+ errors.AsType
		_, ok := errors.AsType[*retryableError](err)
		if !ok {
			return collector
		}

		next, stop := b.Next()
		if stop {
			return collector
		}

		// ctx.Done() has priority, so we test it alone first
		select {
		case <-ctx.Done():
			collector.Add(ctx.Err())
			return collector
		default:
		}

		t := time.NewTimer(next)
		select {
		case <-ctx.Done():
			t.Stop()
			collector.Add(ctx.Err())
			return collector
		case <-t.C:
			t.Stop()
			continue
		}
	}
}

// RetryCallback 重试回调函数类型 | Retry callback function type
type RetryCallback func(attempt int, err error, nextWait time.Duration)

// PanicError 表示RetryFunc发生了panic | Represents a panic in RetryFunc
type PanicError struct {
	Value any    // panic的值
	Stack string // 堆栈信息（如果可用）
}

// Error 实现error接口 | Implement error interface
func (e *PanicError) Error() string {
	return fmt.Sprintf("panic in retry function: %v", e.Value)
}

// DoWithPanicRecovery 使用退避算法包装函数进行重试，并捕获panic | Retry with backoff and recover from panics
// 如果RetryFunc发生panic，会将其转换为PanicError并作为可重试错误处理 | Panics are converted to PanicError and treated as retryable
// 参数:
//   - ctx: 上下文，用于取消和超时控制
//   - b: 退避算法
//   - f: 要重试的函数
//
// 返回:
//   - error: 最终的错误，如果成功则返回nil
func DoWithPanicRecovery(ctx context.Context, b Backoff, f RetryFunc) error {
	wrappedFunc := func(ctx context.Context) (err error) {
		defer func() {
			if r := recover(); r != nil {
				// 将panic转换为错误
				panicErr := &PanicError{Value: r}
				err = RetryableError(panicErr)
			}
		}()
		return f(ctx)
	}

	return Do(ctx, b, wrappedFunc)
}

// DoWithCallback 使用退避算法包装函数进行重试，并在每次重试前调用回调函数 | Retry with backoff and call callback before each retry
// 回调函数可用于日志记录、指标收集等 | Callback can be used for logging, metrics collection, etc.
// 参数:
//   - ctx: 上下文，用于取消和超时控制
//   - b: 退避算法
//   - f: 要重试的函数
//   - callback: 每次重试前调用的回调函数（可以为nil）
//
// 返回:
//   - error: 最终的错误，如果成功则返回nil
func DoWithCallback(ctx context.Context, b Backoff, f RetryFunc, callback RetryCallback) error {
	attempt := 0

	for {
		// Return immediately if ctx is canceled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		attempt++
		err := f(ctx)
		if err == nil {
			return nil
		}

		// Not retryable - Go 1.26+ errors.AsType | Go 1.26+ errors.AsType
		rerr, ok := errors.AsType[*retryableError](err)
		if !ok {
			return err
		}

		next, stop := b.Next()
		if stop {
			return rerr.Unwrap()
		}

		// 调用回调函数
		if callback != nil {
			callback(attempt, rerr.Unwrap(), next)
		}

		// ctx.Done() has priority, so we test it alone first
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		t := time.NewTimer(next)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
			t.Stop()
			continue
		}
	}
}
