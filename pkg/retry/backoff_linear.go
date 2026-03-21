package retry

import (
	"context"
	"errors"
	"math"
	"sync/atomic"
	"time"
)

// Linear 使用线性退避策略的重试包装器 | Retry wrapper with linear backoff strategy
// 等待时间按线性增长：base, 2*base, 3*base, 4*base, ... | Wait time grows linearly: base, 2*base, 3*base, 4*base, ...
// 参数:
//   - ctx: 上下文
//   - base: 基础等待时间
//   - f: 重试函数
//
// 返回:
//   - error: 重试过程中的错误
func Linear(ctx context.Context, base time.Duration, f RetryFunc) error {
	backoff, err := NewLinear(base)
	if err != nil {
		return err
	}
	return Do(ctx, backoff, f)
}

// linearBackoff 实现线性退避策略
// 等待时间按线性增长：base, 2*base, 3*base, 4*base, ...
// 使用原子操作确保并发安全
type linearBackoff struct {
	base    time.Duration
	attempt uint64
}

// NewLinear 创建一个新的线性退避策略 | Create a new linear backoff strategy
//
// 线性退避策略的等待时间按线性增长：
//   - 第1次重试：base
//   - 第2次重试：2 * base
//   - 第3次重试：3 * base
//   - 第n次重试：n * base
//
// 参数:
//   - base: 基础等待时间，必须 > 0
//
// 返回:
//   - Backoff: 线性退避策略实例
//   - error: 如果参数无效则返回错误
//
// 注意:
//   - 线性退避增长速度介于常量退避和指数退避之间
//   - 适合需要逐步增加等待时间但不希望增长过快的场景
//   - 建议配合 WithCappedDuration 使用以限制最大等待时间
//   - 建议配合 WithMaxRetries 或 WithMaxDuration 使用以限制总重试次数/时间
//   - 每个重试操作应创建独立的实例，不要在多个goroutine间共享
//
// 溢出处理:
//   - 当 attempt * base 溢出时，返回 math.MaxInt64
//   - 建议使用 WithCappedDuration 防止溢出
//
// 示例:
//
//	// 创建线性退避：1s, 2s, 3s, 4s, ...
//	backoff, err := retry.NewLinear(time.Second)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// 配合其他策略使用
//	backoff := retry.WithMaxRetries(10,
//	    retry.WithCappedDuration(30*time.Second,
//	        retry.NewLinear(time.Second)))
func NewLinear(base time.Duration) (Backoff, error) {
	if base <= 0 {
		return nil, errors.New("invalid base duration")
	}

	return &linearBackoff{
		base:    base,
		attempt: 0,
	}, nil
}

// Next 实现 Backoff 接口
// 返回下一次重试的等待时间
func (b *linearBackoff) Next() (time.Duration, bool) {
	// 原子递增尝试次数
	attempt := atomic.AddUint64(&b.attempt, 1)

	// 检查溢出：attempt * base
	// 如果 attempt > math.MaxInt64 / base，则会溢出
	if attempt > uint64(math.MaxInt64)/uint64(b.base) {
		return time.Duration(math.MaxInt64), false
	}

	// 计算等待时间：attempt * base
	wait := time.Duration(attempt) * b.base

	return wait, false
}
