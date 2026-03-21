package retry

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"
)

// fibonacciBackoff 斐波那契退避算法实现
// 优化：使用互斥锁确保状态一致性
type fibonacciBackoff struct {
	base time.Duration // 基础时间间隔
	prev uint64        // 前一个值
	curr uint64        // 当前值
	mu   sync.Mutex    // 保护状态的互斥锁
}

// Fibonacci 使用斐波那契退避算法的重试包装器 | Retry wrapper with Fibonacci backoff algorithm
// 参数 Parameters:
//   - ctx: 上下文
//   - base: 基础时间间隔
//   - f: 重试函数
//
// 返回:
//   - error: 重试过程中的错误
func Fibonacci(ctx context.Context, base time.Duration, f RetryFunc) error {
	backoff, err := NewFibonacci(base)
	if err != nil {
		return err
	}
	return Do(ctx, backoff, f)
}

// NewFibonacci 创建一个斐波那契退避算法 | Create a Fibonacci backoff algorithm
// 使用基础值base，等待时间是前两次等待时间的和（1, 1, 2, 3, 5, 8, 13...）| Uses base value, wait time is sum of previous two (1, 1, 2, 3, 5, 8, 13...)
// 当发生溢出时，函数将持续返回64位整数的最大time.Duration值（约292年）
//
// 重要说明：
//   - 建议配合 WithCappedDuration 使用以限制单次等待时间
//   - 例如：WithCappedDuration(time.Minute, NewFibonacci(time.Second))
//   - 斐波那契数列增长较慢，但最终也会溢出
//
// 优化：使用互斥锁确保状态一致性
// 如果给定的base小于等于零，会返回错误
// 参数:
//   - base: 基础时间间隔
//
// 返回:
//   - Backoff: 斐波那契退避算法
//   - error: 参数验证错误
func NewFibonacci(base time.Duration) (Backoff, error) {
	if base <= 0 {
		return nil, errors.New("invalid base duration")
	}

	return &fibonacciBackoff{
		base: base,
		prev: 0,
		curr: uint64(base),
	}, nil
}

// Next 实现Backoff接口，线程安全
// 优化：使用互斥锁确保状态一致性，避免竞态条件
// 返回:
//   - time.Duration: 下次等待的时间间隔
//   - bool: 是否停止重试（总是返回false）
func (b *fibonacciBackoff) Next() (time.Duration, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// 读取当前值
	prev := b.prev
	curr := b.curr

	// 计算下一个斐波那契数，检查溢出
	// 使用更安全的溢出检测
	if prev > math.MaxUint64-curr {
		// 溢出，返回最大值
		return math.MaxInt64, false
	}

	next := prev + curr

	// 额外的溢出检查（防止转换为 time.Duration 时溢出）
	if next > uint64(math.MaxInt64) {
		return math.MaxInt64, false
	}

	// 更新状态
	b.prev = curr
	b.curr = next

	return time.Duration(next), false
}
