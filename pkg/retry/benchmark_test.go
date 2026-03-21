package retry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// BenchmarkDo_Success 基准测试：成功的重试
func BenchmarkDo_Success(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff := MustNewConstant(1 * time.Nanosecond)
		attempts := 0
		_ = Do(ctx, backoff, func(ctx context.Context) error {
			attempts++
			if attempts < 3 {
				return RetryableError(errors.New("fail"))
			}
			return nil
		})
	}
}

// BenchmarkDo_ImmediateSuccess 基准测试：立即成功
func BenchmarkDo_ImmediateSuccess(b *testing.B) {
	ctx := context.Background()
	backoff := MustNewConstant(1 * time.Nanosecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Do(ctx, backoff, func(ctx context.Context) error {
			return nil
		})
	}
}

// BenchmarkConstantBackoff 基准测试：常量退避
func BenchmarkConstantBackoff(b *testing.B) {
	backoff := MustNewConstant(1 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = backoff.Next()
	}
}

// BenchmarkExponentialBackoff 基准测试：指数退避
func BenchmarkExponentialBackoff(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff := MustNewExponential(1 * time.Millisecond)
		for j := 0; j < 10; j++ {
			_, _ = backoff.Next()
		}
	}
}

// BenchmarkFibonacciBackoff 基准测试：斐波那契退避
func BenchmarkFibonacciBackoff(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff := MustNewFibonacci(1 * time.Millisecond)
		for j := 0; j < 10; j++ {
			_, _ = backoff.Next()
		}
	}
}

// BenchmarkLinearBackoff 基准测试：线性退避
func BenchmarkLinearBackoff(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff := MustNewLinear(1 * time.Millisecond)
		for j := 0; j < 10; j++ {
			_, _ = backoff.Next()
		}
	}
}

// BenchmarkWithJitter 基准测试：带抖动的退避
func BenchmarkWithJitter(b *testing.B) {
	base := MustNewConstant(100 * time.Millisecond)
	backoff := WithJitter(20*time.Millisecond, base)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = backoff.Next()
	}
}

// BenchmarkWithJitterPercent 基准测试：带百分比抖动的退避
func BenchmarkWithJitterPercent(b *testing.B) {
	base := MustNewConstant(100 * time.Millisecond)
	backoff := WithJitterPercent(10, base)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = backoff.Next()
	}
}

// BenchmarkWithJitter_HighFrequency 基准测试：高频调用WithJitter
func BenchmarkWithJitter_HighFrequency(b *testing.B) {
	base := MustNewConstant(100 * time.Millisecond)
	backoff := WithJitter(20*time.Millisecond, base)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = backoff.Next()
		}
	})
}

// BenchmarkWithJitterPercent_HighFrequency 基准测试：高频调用WithJitterPercent
func BenchmarkWithJitterPercent_HighFrequency(b *testing.B) {
	base := MustNewConstant(100 * time.Millisecond)
	backoff := WithJitterPercent(10, base)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = backoff.Next()
		}
	})
}

// BenchmarkErrorCollector_ConcurrentRead 基准测试：ErrorCollector并发读
func BenchmarkErrorCollector_ConcurrentRead(b *testing.B) {
	collector := NewErrorCollector()
	// 预先添加一些错误
	for i := 0; i < 100; i++ {
		collector.Add(fmt.Errorf("error %d", i))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = collector.Count()
			_ = collector.Errors()
			_ = collector.Error()
		}
	})
}

// BenchmarkErrorCollector_MixedReadWrite 基准测试：ErrorCollector混合读写
func BenchmarkErrorCollector_MixedReadWrite(b *testing.B) {
	collector := NewErrorCollector()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%10 == 0 {
				// 10% 写操作
				collector.Add(fmt.Errorf("error %d", i))
			} else {
				// 90% 读操作
				_ = collector.Count()
			}
			i++
		}
	})
}

// BenchmarkHighConcurrency 基准测试：高并发重试场景
func BenchmarkHighConcurrency(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			backoff := WithMaxRetries(3,
				WithJitterPercent(10,
					MustNewExponential(1*time.Nanosecond)))

			attempts := 0
			_ = Do(ctx, backoff, func(ctx context.Context) error {
				attempts++
				if attempts < 3 {
					return RetryableError(errors.New("fail"))
				}
				return nil
			})
		}
	})
}

// BenchmarkWithMaxRetries 基准测试：带最大重试次数的退避
func BenchmarkWithMaxRetries(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff := WithMaxRetries(10, MustNewConstant(1*time.Nanosecond))
		for j := 0; j < 10; j++ {
			_, stop := backoff.Next()
			if stop {
				break
			}
		}
	}
}

// BenchmarkWithCappedDuration 基准测试：带上限的退避
func BenchmarkWithCappedDuration(b *testing.B) {
	backoff := WithCappedDuration(50*time.Millisecond, MustNewExponential(10*time.Millisecond))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = backoff.Next()
	}
}

// BenchmarkWithMaxDuration 基准测试：带最大总时间的退避
func BenchmarkWithMaxDuration(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff := WithMaxDuration(100*time.Millisecond, MustNewConstant(10*time.Millisecond))
		for j := 0; j < 5; j++ {
			_, stop := backoff.Next()
			if stop {
				break
			}
		}
	}
}

// BenchmarkRetryableError 基准测试：错误包装
func BenchmarkRetryableError(b *testing.B) {
	err := errors.New("test error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RetryableError(err)
	}
}

// BenchmarkRandomNumberGeneration 基准测试：随机数生成
func BenchmarkRandomNumberGeneration(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := newLockedRandom(time.Now().UnixNano())
		_, _ = r.Int63n(100)
		r.returnToPool()
	}
}

// BenchmarkComplexBackoff 基准测试：复杂的退避组合
func BenchmarkComplexBackoff(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff := WithMaxRetries(10,
			WithCappedDuration(time.Minute,
				WithJitterPercent(10,
					MustNewExponential(time.Second))))

		for j := 0; j < 10; j++ {
			_, stop := backoff.Next()
			if stop {
				break
			}
		}
	}
}

// BenchmarkConcurrentBackoff 基准测试：并发使用退避
func BenchmarkConcurrentBackoff(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			backoff := MustNewExponential(1 * time.Millisecond)
			for j := 0; j < 5; j++ {
				_, _ = backoff.Next()
			}
		}
	})
}

// BenchmarkDoWithCallback_NoCallback 基准测试：无回调的重试
func BenchmarkDoWithCallback_NoCallback(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff := WithMaxRetries(3, MustNewConstant(1*time.Nanosecond))
		attempts := 0
		_ = DoWithCallback(ctx, backoff, func(ctx context.Context) error {
			attempts++
			if attempts < 3 {
				return RetryableError(errors.New("fail"))
			}
			return nil
		}, nil)
	}
}

// BenchmarkDoWithCallback_WithCallback 基准测试：带回调的重试
func BenchmarkDoWithCallback_WithCallback(b *testing.B) {
	ctx := context.Background()
	callback := func(attempt int, err error, nextWait time.Duration) {
		// 空回调，只测试调用开销
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff := WithMaxRetries(3, MustNewConstant(1*time.Nanosecond))
		attempts := 0
		_ = DoWithCallback(ctx, backoff, func(ctx context.Context) error {
			attempts++
			if attempts < 3 {
				return RetryableError(errors.New("fail"))
			}
			return nil
		}, callback)
	}
}

// BenchmarkDoWithPanicRecovery 基准测试：带panic恢复的重试
func BenchmarkDoWithPanicRecovery(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff := WithMaxRetries(3, MustNewConstant(1*time.Nanosecond))
		attempts := 0
		_ = DoWithPanicRecovery(ctx, backoff, func(ctx context.Context) error {
			attempts++
			if attempts < 3 {
				return RetryableError(errors.New("fail"))
			}
			return nil
		})
	}
}

// BenchmarkMemoryAllocation 基准测试：内存分配
func BenchmarkMemoryAllocation(b *testing.B) {
	b.ReportAllocs()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff := WithMaxRetries(5,
			WithJitter(10*time.Millisecond,
				MustNewExponential(time.Millisecond)))

		attempts := 0
		_ = Do(ctx, backoff, func(ctx context.Context) error {
			attempts++
			if attempts < 3 {
				return RetryableError(errors.New("fail"))
			}
			return nil
		})
	}
}
