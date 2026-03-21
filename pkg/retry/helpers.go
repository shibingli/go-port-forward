package retry

import (
	"context"
	"time"
)

// DoWithExponential 使用指数退避策略进行重试的便捷函数 | Convenience function for retry with exponential backoff
// 这是最常用的重试模式，提供合理的默认配置 | Most common retry pattern with reasonable defaults
//
// 参数:
//   - ctx: 上下文
//   - maxRetries: 最大重试次数
//   - base: 基础等待时间
//   - f: 要重试的函数
//
// 返回:
//   - error: 最终的错误，如果成功则返回nil
//
// 示例:
//
//	err := retry.DoWithExponential(ctx, 3, time.Second, func(ctx context.Context) error {
//	    return doSomething()
//	})
func DoWithExponential(ctx context.Context, maxRetries uint64, base time.Duration, f RetryFunc) error {
	backoff := WithMaxRetries(maxRetries, MustNewExponential(base))
	return Do(ctx, backoff, f)
}

// DoWithExponentialCapped 使用带上限的指数退避策略进行重试 | Retry with capped exponential backoff
// 这是推荐的生产环境配置，防止等待时间过长 | Recommended for production, prevents excessive wait times
//
// 参数:
//   - ctx: 上下文
//   - maxRetries: 最大重试次数
//   - base: 基础等待时间
//   - maxWait: 单次等待的最大时间
//   - f: 要重试的函数
//
// 返回:
//   - error: 最终的错误，如果成功则返回nil
//
// 示例:
//
//	err := retry.DoWithExponentialCapped(ctx, 5, time.Second, 30*time.Second, func(ctx context.Context) error {
//	    return doSomething()
//	})
func DoWithExponentialCapped(ctx context.Context, maxRetries uint64, base, maxWait time.Duration, f RetryFunc) error {
	backoff := WithMaxRetries(maxRetries,
		WithCappedDuration(maxWait,
			MustNewExponential(base)))
	return Do(ctx, backoff, f)
}

// DoWithExponentialJitter 使用带抖动的指数退避策略进行重试 | Retry with exponential backoff and jitter
// 适合高并发场景，避免惊群效应 | Suitable for high-concurrency scenarios, avoids thundering herd
//
// 参数:
//   - ctx: 上下文
//   - maxRetries: 最大重试次数
//   - base: 基础等待时间
//   - jitterPercent: 抖动百分比（例如：10表示±10%）
//   - f: 要重试的函数
//
// 返回:
//   - error: 最终的错误，如果成功则返回nil
//
// 示例:
//
//	err := retry.DoWithExponentialJitter(ctx, 3, time.Second, 10, func(ctx context.Context) error {
//	    return doSomething()
//	})
func DoWithExponentialJitter(ctx context.Context, maxRetries uint64, base time.Duration, jitterPercent uint64, f RetryFunc) error {
	backoff := WithMaxRetries(maxRetries,
		WithJitterPercent(jitterPercent,
			MustNewExponential(base)))
	return Do(ctx, backoff, f)
}

// DoWithLinear 使用线性退避策略进行重试的便捷函数 | Convenience function for retry with linear backoff
//
// 参数:
//   - ctx: 上下文
//   - maxRetries: 最大重试次数
//   - base: 基础等待时间
//   - f: 要重试的函数
//
// 返回:
//   - error: 最终的错误，如果成功则返回nil
//
// 示例:
//
//	err := retry.DoWithLinear(ctx, 3, time.Second, func(ctx context.Context) error {
//	    return doSomething()
//	})
func DoWithLinear(ctx context.Context, maxRetries uint64, base time.Duration, f RetryFunc) error {
	backoff := WithMaxRetries(maxRetries, MustNewLinear(base))
	return Do(ctx, backoff, f)
}

// DoWithConstant 使用常量退避策略进行重试的便捷函数 | Convenience function for retry with constant backoff
//
// 参数:
//   - ctx: 上下文
//   - maxRetries: 最大重试次数
//   - interval: 固定的等待时间
//   - f: 要重试的函数
//
// 返回:
//   - error: 最终的错误，如果成功则返回nil
//
// 示例:
//
//	err := retry.DoWithConstant(ctx, 3, time.Second, func(ctx context.Context) error {
//	    return doSomething()
//	})
func DoWithConstant(ctx context.Context, maxRetries uint64, interval time.Duration, f RetryFunc) error {
	backoff := WithMaxRetries(maxRetries, MustNewConstant(interval))
	return Do(ctx, backoff, f)
}

// DoWithTimeout 使用超时和指数退避策略进行重试 | Retry with timeout and exponential backoff
// 这是另一种常用模式，限制总重试时间而不是重试次数 | Another common pattern, limits total retry time instead of retry count
//
// 参数:
//   - ctx: 上下文
//   - timeout: 总超时时间
//   - base: 基础等待时间
//   - f: 要重试的函数
//
// 返回:
//   - error: 最终的错误，如果成功则返回nil
//
// 示例:
//
//	err := retry.DoWithTimeout(ctx, 30*time.Second, time.Second, func(ctx context.Context) error {
//	    return doSomething()
//	})
func DoWithTimeout(ctx context.Context, timeout, base time.Duration, f RetryFunc) error {
	backoff := WithMaxDuration(timeout, MustNewExponential(base))
	return Do(ctx, backoff, f)
}

// DoQuick 快速重试，适合快速失败的场景 | Quick retry, suitable for fast-fail scenarios
// 使用常量退避，最多重试3次，每次等待100ms | Constant backoff, max 3 retries, 100ms each
//
// 参数:
//   - ctx: 上下文
//   - f: 要重试的函数
//
// 返回:
//   - error: 最终的错误，如果成功则返回nil
//
// 示例:
//
//	err := retry.DoQuick(ctx, func(ctx context.Context) error {
//	    return doSomething()
//	})
func DoQuick(ctx context.Context, f RetryFunc) error {
	return DoWithConstant(ctx, 3, 100*time.Millisecond, f)
}

// DoStandard 标准重试，适合大多数场景 | Standard retry, suitable for most scenarios
// 使用指数退避，最多重试5次，基础等待时间1秒，最大等待30秒 | Exponential backoff, max 5 retries, 1s base, 30s cap
//
// 参数:
//   - ctx: 上下文
//   - f: 要重试的函数
//
// 返回:
//   - error: 最终的错误，如果成功则返回nil
//
// 示例:
//
//	err := retry.DoStandard(ctx, func(ctx context.Context) error {
//	    return doSomething()
//	})
func DoStandard(ctx context.Context, f RetryFunc) error {
	return DoWithExponentialCapped(ctx, 5, time.Second, 30*time.Second, f)
}

// DoAggressive 激进重试，适合需要高可用性的场景 | Aggressive retry, suitable for high-availability scenarios
// 使用指数退避+抖动，最多重试10次，基础等待时间500ms，最大等待1分钟，10%抖动 | Exponential backoff+jitter, max 10 retries, 500ms base, 1min cap, 10% jitter
//
// 参数:
//   - ctx: 上下文
//   - f: 要重试的函数
//
// 返回:
//   - error: 最终的错误，如果成功则返回nil
//
// 示例:
//
//	err := retry.DoAggressive(ctx, func(ctx context.Context) error {
//	    return doSomething()
//	})
func DoAggressive(ctx context.Context, f RetryFunc) error {
	backoff := WithMaxRetries(10,
		WithCappedDuration(time.Minute,
			WithJitterPercent(10,
				MustNewExponential(500*time.Millisecond))))
	return Do(ctx, backoff, f)
}

// RetryConfig 重试配置 | Retry configuration
type RetryConfig struct {
	Callback       RetryCallback
	Strategy       string
	MaxRetries     uint64
	MaxDuration    time.Duration
	BaseInterval   time.Duration
	MaxInterval    time.Duration
	JitterPercent  uint64
	EnableCallback bool
	EnablePanic    bool
}

// DefaultConfig 返回默认配置 | Return default configuration
func DefaultConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:    5,
		BaseInterval:  time.Second,
		MaxInterval:   30 * time.Second,
		JitterPercent: 0,
		Strategy:      "exponential",
	}
}

// DoWithConfig 使用配置进行重试 | Retry with configuration
//
// 参数:
//   - ctx: 上下文
//   - config: 重试配置
//   - f: 要重试的函数
//
// 返回:
//   - error: 最终的错误，如果成功则返回nil
//
// 示例:
//
//	config := retry.DefaultConfig()
//	config.MaxRetries = 10
//	config.JitterPercent = 10
//	err := retry.DoWithConfig(ctx, config, func(ctx context.Context) error {
//	    return doSomething()
//	})
func DoWithConfig(ctx context.Context, config *RetryConfig, f RetryFunc) error {
	if config == nil {
		config = DefaultConfig()
	}

	// 创建基础退避策略
	var backoff Backoff
	switch config.Strategy {
	case "constant":
		backoff = MustNewConstant(config.BaseInterval)
	case "linear":
		backoff = MustNewLinear(config.BaseInterval)
	case "fibonacci":
		backoff = MustNewFibonacci(config.BaseInterval)
	default: // "exponential"
		backoff = MustNewExponential(config.BaseInterval)
	}

	// 应用抖动
	if config.JitterPercent > 0 {
		backoff = WithJitterPercent(config.JitterPercent, backoff)
	}

	// 应用最大等待时间
	if config.MaxInterval > 0 {
		backoff = WithCappedDuration(config.MaxInterval, backoff)
	}

	// 应用最大重试次数
	if config.MaxRetries > 0 {
		backoff = WithMaxRetries(config.MaxRetries, backoff)
	}

	// 应用最大总时间
	if config.MaxDuration > 0 {
		backoff = WithMaxDuration(config.MaxDuration, backoff)
	}

	// 选择执行方式
	if config.EnablePanic {
		return DoWithPanicRecovery(ctx, backoff, f)
	} else if config.EnableCallback && config.Callback != nil {
		return DoWithCallback(ctx, backoff, f, config.Callback)
	}

	return Do(ctx, backoff, f)
}
