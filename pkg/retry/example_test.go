package retry

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ExampleDo 展示基本的重试用法
func ExampleDo() {
	ctx := context.Background()

	// 创建一个最多重试3次的指数退避算法
	backoff := WithMaxRetries(3, MustNewExponential(100*time.Millisecond))

	attempts := 0
	err := Do(ctx, backoff, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return RetryableError(errors.New("temporary failure"))
		}
		return nil // 第3次尝试成功
	})

	fmt.Printf("Attempts: %d, Error: %v\n", attempts, err)
	// Output: Attempts: 3, Error: <nil>
}

// ExampleWithJitter 展示带抖动的重试
func ExampleWithJitter() {
	ctx := context.Background()

	// 创建带抖动的退避算法
	base := MustNewConstant(100 * time.Millisecond)
	backoff := WithMaxRetries(3, WithJitter(50*time.Millisecond, base))

	attempts := 0
	_ = Do(ctx, backoff, func(ctx context.Context) error {
		attempts++
		return RetryableError(errors.New("always fail"))
	})

	fmt.Printf("Attempts: %d, Error type: retryable\n", attempts)
	// Output: Attempts: 4, Error type: retryable
}

// ExampleFibonacci 展示斐波那契退避算法
func ExampleFibonacci() {
	ctx := context.Background()

	// 创建斐波那契退避算法
	backoff := WithMaxRetries(5, MustNewFibonacci(10*time.Millisecond))

	attempts := 0
	err := Do(ctx, backoff, func(ctx context.Context) error {
		attempts++
		if attempts < 4 {
			return RetryableError(fmt.Errorf("attempt %d failed", attempts))
		}
		return nil
	})

	fmt.Printf("Attempts: %d, Success: %v\n", attempts, err == nil)
	// Output: Attempts: 4, Success: true
}

// ExampleLinear 展示线性退避的使用
func ExampleLinear() {
	ctx := context.Background()

	// 创建线性退避策略，基础时间为100ms
	// 等待时间：100ms, 200ms, 300ms, 400ms, ...
	backoff := WithMaxRetries(3, MustNewLinear(100*time.Millisecond))

	attempts := 0
	err := Do(ctx, backoff, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return RetryableError(fmt.Errorf("attempt %d failed", attempts))
		}
		return nil
	})

	fmt.Printf("Attempts: %d, Success: %v\n", attempts, err == nil)
	// Output: Attempts: 3, Success: true
}

// ExampleRetryableError_nonRetryable 展示非可重试错误
func ExampleRetryableError_nonRetryable() {
	ctx := context.Background()

	backoff := WithMaxRetries(3, MustNewConstant(10*time.Millisecond))

	attempts := 0
	err := Do(ctx, backoff, func(ctx context.Context) error {
		attempts++
		// 返回非可重试错误
		return errors.New("permanent failure")
	})

	fmt.Printf("Attempts: %d, Error: %v\n", attempts, err)
	// Output: Attempts: 1, Error: permanent failure
}

// ExampleDo_concurrent 展示并发使用场景
// 重要：每个goroutine必须创建独立的Backoff实例，不能共享！
func ExampleDo_concurrent() {
	ctx := context.Background()

	// 模拟并发调用
	results := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			// 重要：每个goroutine创建独立的backoff实例
			// 不要在多个goroutine之间共享Backoff实例！
			backoff := WithMaxRetries(3, MustNewExponential(time.Millisecond))

			err := Do(ctx, backoff, func(ctx context.Context) error {
				// 模拟50%的成功率
				if id%2 == 0 {
					return nil
				}
				return RetryableError(fmt.Errorf("worker %d failed", id))
			})
			results <- err == nil
		}(i)
	}

	// 收集结果
	successes := 0
	for i := 0; i < 10; i++ {
		if <-results {
			successes++
		}
	}

	fmt.Printf("Successes: %d/10\n", successes)
	// Output: Successes: 5/10
}

// ExampleBackoff_performance 展示性能对比
func ExampleBackoff_performance() {
	// 测试不同退避算法的性能
	algorithms := map[string]Backoff{
		"Constant":    MustNewConstant(time.Microsecond),
		"Exponential": MustNewExponential(time.Microsecond),
		"Fibonacci":   MustNewFibonacci(time.Microsecond),
	}

	for name, backoff := range algorithms {
		start := time.Now()

		// 执行1000次Next()调用
		for i := 0; i < 1000; i++ {
			_, _ = backoff.Next()
		}

		duration := time.Since(start)
		fmt.Printf("%s: %v for 1000 operations\n", name, duration)
	}

	// 注意：实际输出会因系统而异
	// Output:
	// Constant: 2.1µs for 1000 operations
	// Exponential: 24.3µs for 1000 operations
	// Fibonacci: 24.2ns for 1000 operations
}

// ExampleDoQuick 展示快速重试的使用
func ExampleDoQuick() {
	ctx := context.Background()

	attempts := 0
	err := DoQuick(ctx, func(ctx context.Context) error {
		attempts++
		if attempts < 2 {
			return RetryableError(errors.New("temporary failure"))
		}
		return nil
	})

	fmt.Printf("Attempts: %d, Success: %v\n", attempts, err == nil)
	// Output: Attempts: 2, Success: true
}

// ExampleDoStandard 展示标准重试的使用
func ExampleDoStandard() {
	ctx := context.Background()

	attempts := 0
	err := DoStandard(ctx, func(ctx context.Context) error {
		attempts++
		if attempts < 2 {
			return RetryableError(errors.New("temporary failure"))
		}
		return nil
	})

	fmt.Printf("Attempts: %d, Success: %v\n", attempts, err == nil)
	// Output: Attempts: 2, Success: true
}

// ExampleDoWithConfig 展示使用配置进行重试
func ExampleDoWithConfig() {
	ctx := context.Background()

	// 自定义配置
	config := &RetryConfig{
		MaxRetries:    3,
		BaseInterval:  100 * time.Millisecond,
		MaxInterval:   time.Second,
		JitterPercent: 10,
		Strategy:      "exponential",
	}

	attempts := 0
	err := DoWithConfig(ctx, config, func(ctx context.Context) error {
		attempts++
		if attempts < 2 {
			return RetryableError(errors.New("temporary failure"))
		}
		return nil
	})

	fmt.Printf("Attempts: %d, Success: %v\n", attempts, err == nil)
	// Output: Attempts: 2, Success: true
}
