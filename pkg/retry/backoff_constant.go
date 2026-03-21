package retry

import (
	"context"
	"errors"
	"time"
)

// Constant 使用固定时间间隔的重试包装器 | Retry wrapper with constant time interval
// 如果给定的时间间隔小于等于零，会返回错误 | Returns error if interval is less than or equal to zero
// 参数:
//   - ctx: 上下文
//   - t: 固定的重试间隔时间
//   - f: 重试函数
//
// 返回:
//   - error: 重试过程中的错误
func Constant(ctx context.Context, t time.Duration, f RetryFunc) error {
	backoff, err := NewConstant(t)
	if err != nil {
		return err
	}
	return Do(ctx, backoff, f)
}

// NewConstant 创建一个使用固定时间间隔的退避算法 | Create a constant backoff algorithm
// 等待时间是提供的固定值。如果给定的时间间隔小于等于零，会返回错误 | Wait time is the provided fixed value. Returns error if interval <= 0
// 参数:
//   - t: 固定的时间间隔
//
// 返回:
//   - Backoff: 固定间隔退避算法
//   - error: 参数验证错误
func NewConstant(t time.Duration) (Backoff, error) {
	if t <= 0 {
		return nil, errors.New("invalid time duration")
	}

	return BackoffFunc(func() (time.Duration, bool) {
		return t, false
	}), nil
}
