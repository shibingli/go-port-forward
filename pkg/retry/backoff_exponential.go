package retry

import (
	"context"
	"errors"
	"math"
	"sync/atomic"
	"time"
)

// exponentialBackoff 指数退避算法实现
type exponentialBackoff struct {
	base    time.Duration // 基础时间间隔
	attempt uint64        // 尝试次数（原子操作）
}

// Exponential 使用指数退避算法的重试包装器 | Retry wrapper with exponential backoff algorithm
// 参数 Parameters:
//   - ctx: 上下文
//   - base: 基础时间间隔
//   - f: 重试函数
//
// 返回:
//   - error: 重试过程中的错误
func Exponential(ctx context.Context, base time.Duration, f RetryFunc) error {
	backoff, err := NewExponential(base)
	if err != nil {
		return err
	}
	return Do(ctx, backoff, f)
}

// NewExponential 创建一个指数退避算法 | Create an exponential backoff algorithm
// 使用基础值base，每次失败时翻倍（1, 2, 4, 8, 16, 32, 64...）| Uses base value, doubles on each failure (1, 2, 4, 8, 16, 32, 64...)
// 当发生溢出时，函数将持续返回64位整数的最大time.Duration值（约292年）
//
// 重要说明：
//   - 建议配合 WithCappedDuration 使用以限制单次等待时间
//   - 例如：WithCappedDuration(time.Minute, NewExponential(time.Second))
//   - 第63次重试后会溢出，返回math.MaxInt64
//
// 如果给定的base小于等于零，会返回错误
// 参数:
//   - base: 基础时间间隔
//
// 返回:
//   - Backoff: 指数退避算法
//   - error: 参数验证错误
func NewExponential(base time.Duration) (Backoff, error) {
	if base <= 0 {
		return nil, errors.New("invalid base duration")
	}

	return &exponentialBackoff{
		base: base,
	}, nil
}

// Next 实现Backoff接口，线程安全
// 优化：修复溢出处理的竞态条件，使用更安全的溢出检测
// 返回:
//   - time.Duration: 下次等待的时间间隔
//   - bool: 是否停止重试（总是返回false）
func (b *exponentialBackoff) Next() (time.Duration, bool) {
	attempt := atomic.AddUint64(&b.attempt, 1)

	// 防止位移操作溢出，最大位移量为63
	if attempt > 63 {
		return math.MaxInt64, false
	}

	// 检查是否会发生溢出
	shift := attempt - 1
	if shift >= 63 || b.base > math.MaxInt64>>shift {
		return math.MaxInt64, false
	}

	next := b.base << shift
	return next, false
}
