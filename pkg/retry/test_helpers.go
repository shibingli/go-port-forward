package retry

import "time"

// 辅助函数，用于安全地创建退避算法
// 这些函数会处理错误，如果参数无效会使用默认值

// MustNewConstant 创建常量退避策略，如果参数无效则使用默认值（1秒）
// 此函数不返回错误，适合在已知参数有效的场景使用
// 如果需要错误处理，请使用 NewConstant
func MustNewConstant(t time.Duration) Backoff {
	if t <= 0 {
		t = time.Second // 使用默认值
	}
	backoff, _ := NewConstant(t)
	return backoff
}

// MustNewExponential 创建指数退避策略，如果参数无效则使用默认值（1秒）
// 此函数不返回错误，适合在已知参数有效的场景使用
// 如果需要错误处理，请使用 NewExponential
func MustNewExponential(base time.Duration) Backoff {
	if base <= 0 {
		base = time.Second // 使用默认值
	}
	backoff, _ := NewExponential(base)
	return backoff
}

// MustNewFibonacci 创建斐波那契退避策略，如果参数无效则使用默认值（1秒）
// 此函数不返回错误，适合在已知参数有效的场景使用
// 如果需要错误处理，请使用 NewFibonacci
func MustNewFibonacci(base time.Duration) Backoff {
	if base <= 0 {
		base = time.Second // 使用默认值
	}
	backoff, _ := NewFibonacci(base)
	return backoff
}

// MustNewLinear 创建线性退避策略，如果参数无效则使用默认值（1秒）
// 此函数不返回错误，适合在已知参数有效的场景使用
// 如果需要错误处理，请使用 NewLinear
func MustNewLinear(base time.Duration) Backoff {
	if base <= 0 {
		base = time.Second // 使用默认值
	}
	backoff, _ := NewLinear(base)
	return backoff
}
