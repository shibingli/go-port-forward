// Package retry 提供重试机制的辅助工具
//
// 本包为可能不稳定或最终一致的Go函数定义了灵活的重试接口。
// 它抽象了"退避"（重试间隔等待时间）和"重试"（重新执行函数）机制，
// 以获得最大的灵活性。此外，所有组件都是接口，因此您可以定义自己的实现。
//
// 本包模仿Go内置的HTTP包设计，使您可以轻松地用自定义逻辑定制内置退避算法。
// 此外，调用者通过包装错误来指定哪些错误是可重试的。这对于复杂操作中
// 只有某些结果应该重试的情况很有帮助。
//
// # 重要使用说明
//
//  1. 并发使用：Backoff实例维护内部状态（如重试次数、斐波那契序列等），
//     因此每个独立的重试操作必须创建自己的Backoff实例。
//     不要在多个goroutine或多个重试操作之间共享同一个Backoff实例！
//
//     正确用法：
//     for i := 0; i < 10; i++ {
//     go func() {
//     backoff := WithMaxRetries(3, NewExponential(time.Second))
//     Do(ctx, backoff, retryFunc)
//     }()
//     }
//
//     错误用法：
//     backoff := WithMaxRetries(3, NewExponential(time.Second))
//     for i := 0; i < 10; i++ {
//     go func() {
//     Do(ctx, backoff, retryFunc) // 错误！共享了backoff实例
//     }()
//     }
//
//  2. 可重试错误：只有使用RetryableError()包装的错误才会触发重试。
//     普通错误会立即返回，不会重试。
//
//  3. 上下文取消：通过context可以随时取消重试操作。
//     建议总是设置超时：ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//
// 4. 参数限制：
//
//   - WithMaxRetries: max表示最大重试次数（不包括首次尝试），实际总尝试次数为max+1
//
//   - WithJitterPercent: j应该在0-100范围内，表示抖动百分比
//
//   - WithMaxDuration: timeout从创建Backoff时开始计时，不是从首次调用Next()开始
//
//   - NewConstant/NewExponential/NewFibonacci: 参数必须大于0
//
//     5. 溢出处理：指数和斐波那契退避在溢出时会返回math.MaxInt64（约292年）。
//     建议配合WithCappedDuration使用以限制单次等待时间：
//     backoff := WithCappedDuration(time.Minute, WithMaxRetries(10, NewExponential(time.Second)))
//
//     6. Panic处理：如果RetryFunc发生panic，会导致整个程序崩溃。
//     请在RetryFunc内部处理panic，或使用defer recover()。
package retry

import (
	"context"
	"errors"
	"time"
)

// 预定义的错误已移至 corpus_genie/errors 包中统一管理
// 请使用 errors.docsErrors.ErrInvalidDuration, errors.docsErrors.ErrInvalidArgument

// RetryFunc 传递给重试机制的函数类型 | Function type passed to retry mechanism
type RetryFunc func(ctx context.Context) error

// retryableError 可重试错误的内部实现
type retryableError struct {
	err error // 包装的原始错误
}

// RetryableError 将错误标记为可重试 | Mark error as retryable
// 参数 Parameters:
//   - err: 要标记为可重试的错误
//
// 返回:
//   - error: 包装后的可重试错误，如果输入为nil则返回nil
func RetryableError(err error) error {
	if err == nil {
		return nil
	}
	return &retryableError{err}
}

// Unwrap 实现错误包装接口
// 返回:
//   - error: 被包装的原始错误
func (e *retryableError) Unwrap() error {
	return e.err
}

// Error 返回错误字符串
// 返回:
//   - string: 格式化的错误信息
func (e *retryableError) Error() string {
	if e.err == nil {
		return "retryable: <nil>"
	}
	return "retryable: " + e.err.Error()
}

// Do 使用退避算法包装函数进行重试 | Retry function with backoff algorithm
// 提供的上下文与传递给RetryFunc的上下文相同 | The provided context is the same as the one passed to RetryFunc
// 参数:
//   - ctx: 上下文，用于取消和超时控制
//   - b: 退避算法
//   - f: 要重试的函数
//
// 返回:
//   - error: 最终的错误，如果成功则返回nil
func Do(ctx context.Context, b Backoff, f RetryFunc) error {
	for {
		// Return immediately if ctx is canceled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

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
			// 显式停止Timer，虽然已触发但这是最佳实践
			t.Stop()
			continue
		}
	}
}
