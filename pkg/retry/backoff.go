// Package retry 提供重试和退避算法的实现
package retry

import (
	"sync/atomic"
	"time"
)

// Backoff 退避算法接口 | Backoff algorithm interface
type Backoff interface {
	// Next 返回下次等待的时间间隔和是否停止重试
	// 返回:
	//   - next: 下次等待的时间间隔
	//   - stop: 是否停止重试
	Next() (next time.Duration, stop bool)
}

var _ Backoff = (BackoffFunc)(nil)

// BackoffFunc 以函数形式表达的退避算法 | Backoff algorithm expressed as a function
type BackoffFunc func() (time.Duration, bool)

// Next 实现Backoff接口 | Implement Backoff interface
// 返回 Returns:
//   - time.Duration: 等待时间间隔
//   - bool: 是否停止重试
func (b BackoffFunc) Next() (time.Duration, bool) {
	return b()
}

// WithJitter 为退避算法添加指定的抖动时间 | Add specified jitter to backoff algorithm
// 抖动值j可以理解为"+/- j"。例如，如果j为5秒，退避算法返回20秒，
// 那么实际值可能在15到25秒之间。返回值永远不会小于0。
// 性能优化：在创建时初始化随机数生成器，避免每次调用时的开销
// 参数:
//   - j: 抖动时间间隔，必须大于0
//   - next: 下一个退避算法
//
// 返回:
//   - Backoff: 包装后的退避算法
//
// 注意：如果j为负数或0，将不添加抖动，直接返回原始值
func WithJitter(j time.Duration, next Backoff) Backoff {
	// 参数验证：如果j无效，直接返回原始退避算法
	if j <= 0 {
		return next
	}

	// 在创建时初始化随机数生成器，避免每次调用时的开销
	r := newLockedRandom(time.Now().UnixNano())
	jInt := int64(j)
	jInt2 := jInt * 2

	return BackoffFunc(func() (time.Duration, bool) {
		val, stop := next.Next()
		if stop {
			return 0, true
		}

		// 直接使用已初始化的随机数生成器
		randomValue, err := r.Int63n(jInt2)
		if err != nil {
			// 如果随机数生成失败，返回原始值（无抖动）
			return val, false
		}

		diff := time.Duration(randomValue - jInt)
		val = val + diff
		if val < 0 {
			val = 0
		}
		return val, false
	})
}

// WithJitterPercent 为退避算法添加指定百分比的抖动 | Add specified percentage jitter to backoff algorithm
// 抖动值j可以理解为"+/- j%"。例如，如果j为5，退避算法返回20秒，
// 那么实际值可能在19到21秒之间。返回值永远不会小于0。
// 性能优化：在创建时初始化随机数生成器，避免每次调用时的开销
// 参数:
//   - j: 抖动百分比，建议范围0-100，超过100也可以但会导致更大的抖动
//   - next: 下一个退避算法
//
// 返回:
//   - Backoff: 包装后的退避算法
//
// 注意：如果j为0，将不添加抖动，直接返回原始值
func WithJitterPercent(j uint64, next Backoff) Backoff {
	// 参数验证：如果j为0，直接返回原始退避算法
	if j == 0 {
		return next
	}

	// 在创建时初始化随机数生成器，避免每次调用时的开销
	r := newLockedRandom(time.Now().UnixNano())
	jInt := int64(j)
	jInt2 := jInt * 2

	return BackoffFunc(func() (time.Duration, bool) {
		val, stop := next.Next()
		if stop {
			return 0, true
		}

		// 获取[-j, j]范围内的随机值，然后转换为百分比
		// 例如：j=10时，randomValue范围[0,20)，offset范围[-10,10)
		// pct范围[1-10/100, 1+10/100] = [0.9, 1.1]
		randomValue, err := r.Int63n(jInt2)
		if err != nil {
			// 如果随机数生成失败，返回原始值（无抖动）
			return val, false
		}

		// 计算抖动百分比：1 +/- (j/100)
		// 步骤1：将randomValue从[0, 2j)映射到[-j, j)
		offset := randomValue - jInt
		// 步骤2：转换为百分比偏移：offset/100
		percentOffset := float64(offset) / 100.0
		// 步骤3：计算最终百分比：1 - percentOffset（注意这里是减法，因为我们要的是1 +/- j%）
		// 当offset=-j时，pct=1-(-j/100)=1+j/100
		// 当offset=j时，pct=1-(j/100)=1-j/100
		pct := 1.0 - percentOffset

		val = time.Duration(float64(val) * pct)
		if val < 0 {
			val = 0
		}
		return val, false
	})
}

// WithMaxRetries 限制退避算法的最大重试次数 | Limit maximum retries for backoff algorithm
// 优化：使用原子操作替代互斥锁，提高并发性能 | Optimization: use atomic operations instead of mutex for better concurrency
//
// 重要说明：
//   - max 表示最大重试次数（不包括首次尝试）
//   - 实际总尝试次数 = max + 1
//   - 例如：max=3 表示首次尝试失败后最多再重试3次，总共4次尝试
//   - 每个独立的重试操作必须创建自己的Backoff实例，不要共享！
//
// 参数:
//   - max: 最大重试次数（不包括首次尝试）
//   - next: 下一个退避算法
//
// 返回:
//   - Backoff: 包装后的退避算法
func WithMaxRetries(max uint64, next Backoff) Backoff {
	var attempt uint64

	return BackoffFunc(func() (time.Duration, bool) {
		// 使用原子操作检查和增加计数器
		currentAttempt := atomic.AddUint64(&attempt, 1)
		if currentAttempt > max {
			return 0, true
		}

		val, stop := next.Next()
		if stop {
			return 0, true
		}

		return val, false
	})
}

// WithCappedDuration 设置退避算法返回时间间隔的最大值 | Set maximum duration for backoff interval
// 这不是总的退避时间，而是单次退避时间的上限。
// 如果没有其他中间件，退避将无限继续。
//
// 重要说明：
//   - 这个函数限制的是单次等待时间，不是总重试时间
//   - 建议与指数或斐波那契退避配合使用，防止等待时间过长
//   - 例如：WithCappedDuration(time.Minute, NewExponential(time.Second))
//     可以防止指数退避溢出后等待292年
//
// 参数:
//   - cap: 单次退避时间的最大值
//   - next: 下一个退避算法
//
// 返回:
//   - Backoff: 包装后的退避算法
func WithCappedDuration(cap time.Duration, next Backoff) Backoff {
	return BackoffFunc(func() (time.Duration, bool) {
		val, stop := next.Next()
		if stop {
			return 0, true
		}

		if val <= 0 || val > cap {
			val = cap
		}
		return val, false
	})
}

// WithMaxDuration 设置退避算法执行的最大总时间 | Set maximum total duration for backoff execution
// 这是尽力而为的实现，不应用于保证精确的时间控制。
// 注意：时间从第一次调用Next()开始计时，而不是从创建Backoff时开始。
// 参数:
//   - timeout: 最大总执行时间
//   - next: 下一个退避算法
//
// 返回:
//   - Backoff: 包装后的退避算法
func WithMaxDuration(timeout time.Duration, next Backoff) Backoff {
	var start time.Time
	var started uint32 // 使用原子操作标记是否已开始

	return BackoffFunc(func() (time.Duration, bool) {
		// 使用sync/atomic确保只在第一次调用时设置start时间
		if atomic.CompareAndSwapUint32(&started, 0, 1) {
			start = time.Now()
		}

		diff := timeout - time.Since(start)
		if diff <= 0 {
			return 0, true
		}

		val, stop := next.Next()
		if stop {
			return 0, true
		}

		if val <= 0 || val > diff {
			val = diff
		}
		return val, false
	})
}
