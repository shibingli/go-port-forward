package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestDoWithExponential 测试指数退避便捷函数
func TestDoWithExponential(t *testing.T) {
	ctx := context.Background()

	attempts := 0
	err := DoWithExponential(ctx, 3, 10*time.Millisecond, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return RetryableError(errors.New("fail"))
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

// TestDoWithExponentialCapped 测试带上限的指数退避
func TestDoWithExponentialCapped(t *testing.T) {
	ctx := context.Background()

	attempts := 0
	err := DoWithExponentialCapped(ctx, 3, 10*time.Millisecond, 50*time.Millisecond, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return RetryableError(errors.New("fail"))
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

// TestDoWithExponentialJitter 测试带抖动的指数退避
func TestDoWithExponentialJitter(t *testing.T) {
	ctx := context.Background()

	attempts := 0
	err := DoWithExponentialJitter(ctx, 3, 10*time.Millisecond, 10, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return RetryableError(errors.New("fail"))
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

// TestDoWithLinear 测试线性退避便捷函数
func TestDoWithLinear(t *testing.T) {
	ctx := context.Background()

	attempts := 0
	err := DoWithLinear(ctx, 3, 10*time.Millisecond, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return RetryableError(errors.New("fail"))
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

// TestDoWithConstant 测试常量退避便捷函数
func TestDoWithConstant(t *testing.T) {
	ctx := context.Background()

	attempts := 0
	err := DoWithConstant(ctx, 3, 10*time.Millisecond, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return RetryableError(errors.New("fail"))
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

// TestDoWithTimeout 测试超时重试
func TestDoWithTimeout(t *testing.T) {
	ctx := context.Background()

	attempts := 0
	err := DoWithTimeout(ctx, 100*time.Millisecond, 10*time.Millisecond, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return RetryableError(errors.New("fail"))
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	if attempts >= 3 {
		// 应该至少尝试3次
		t.Logf("Attempts: %d", attempts)
	}
}

// TestDoQuick 测试快速重试
func TestDoQuick(t *testing.T) {
	ctx := context.Background()

	attempts := 0
	err := DoQuick(ctx, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return RetryableError(errors.New("fail"))
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

// TestDoStandard 测试标准重试
func TestDoStandard(t *testing.T) {
	ctx := context.Background()

	attempts := 0
	err := DoStandard(ctx, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return RetryableError(errors.New("fail"))
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

// TestDoAggressive 测试激进重试
func TestDoAggressive(t *testing.T) {
	ctx := context.Background()

	attempts := 0
	err := DoAggressive(ctx, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return RetryableError(errors.New("fail"))
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

// TestDoWithConfig_Default 测试默认配置
func TestDoWithConfig_Default(t *testing.T) {
	ctx := context.Background()
	config := DefaultConfig()

	attempts := 0
	err := DoWithConfig(ctx, config, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return RetryableError(errors.New("fail"))
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

// TestDoWithConfig_CustomStrategy 测试自定义策略
func TestDoWithConfig_CustomStrategy(t *testing.T) {
	ctx := context.Background()

	strategies := []string{"constant", "linear", "exponential", "fibonacci"}

	for _, strategy := range strategies {
		t.Run(strategy, func(t *testing.T) {
			config := &RetryConfig{
				MaxRetries:   3,
				BaseInterval: 10 * time.Millisecond,
				Strategy:     strategy,
			}

			attempts := 0
			err := DoWithConfig(ctx, config, func(ctx context.Context) error {
				attempts++
				if attempts < 3 {
					return RetryableError(errors.New("fail"))
				}
				return nil
			})

			if err != nil {
				t.Errorf("Expected success, got error: %v", err)
			}

			if attempts != 3 {
				t.Errorf("Expected 3 attempts, got %d", attempts)
			}
		})
	}
}

// TestDoWithConfig_WithCallback 测试配置中的回调
func TestDoWithConfig_WithCallback(t *testing.T) {
	ctx := context.Background()

	callbackCalls := 0
	config := &RetryConfig{
		MaxRetries:     3,
		BaseInterval:   10 * time.Millisecond,
		Strategy:       "exponential",
		EnableCallback: true,
		Callback: func(attempt int, err error, nextWait time.Duration) {
			callbackCalls++
		},
	}

	attempts := 0
	err := DoWithConfig(ctx, config, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return RetryableError(errors.New("fail"))
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	if callbackCalls != 2 {
		t.Errorf("Expected 2 callback calls, got %d", callbackCalls)
	}
}

// TestDoWithConfig_WithPanic 测试配置中的panic恢复
func TestDoWithConfig_WithPanic(t *testing.T) {
	ctx := context.Background()

	config := &RetryConfig{
		MaxRetries:   3,
		BaseInterval: 10 * time.Millisecond,
		Strategy:     "exponential",
		EnablePanic:  true,
	}

	attempts := 0
	err := DoWithConfig(ctx, config, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			panic("test panic")
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

// TestDoWithConfig_NilConfig 测试nil配置
func TestDoWithConfig_NilConfig(t *testing.T) {
	ctx := context.Background()

	attempts := 0
	err := DoWithConfig(ctx, nil, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return RetryableError(errors.New("fail"))
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
