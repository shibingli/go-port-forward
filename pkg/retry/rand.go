package retry

import (
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// lockedSource 线程安全的随机数源
// 使用互斥锁保护对rand.Rand的并发访问
type lockedSource struct {
	src *rand.Rand // 随机数生成器
	mu  sync.Mutex // 互斥锁
}

var _ rand.Source64 = (*lockedSource)(nil)

// 全局随机数生成器池，减少内存分配
var (
	globalRandPool = sync.Pool{
		New: func() any {
			// 使用更好的初始种子：结合时间和计数器
			seed := time.Now().UnixNano() + int64(atomic.AddUint64(&seedCounter, 1))
			return &lockedSource{
				src: rand.New(rand.NewSource(seed)),
			}
		},
	}
	// 种子计数器，确保每个随机数生成器有不同的种子
	// 使用原子操作确保线程安全
	seedCounter uint64
)

// newLockedRandom 创建一个新的线程安全随机数源
// 优化：使用对象池减少内存分配，使用原子计数器确保种子唯一性
// 参数:
//   - seed: 随机数种子
//
// 返回:
//   - *lockedSource: 线程安全的随机数源
func newLockedRandom(seed int64) *lockedSource {
	r := globalRandPool.Get().(*lockedSource)
	// 结合传入的种子和原子计数器，确保种子的唯一性
	uniqueSeed := seed + int64(atomic.AddUint64(&seedCounter, 1))
	r.Seed(uniqueSeed)
	return r
}

// returnToPool 将随机数生成器返回到池中
func (r *lockedSource) returnToPool() {
	globalRandPool.Put(r)
}

// Int63 模拟math/rand.(*Rand).Int63，使用互斥锁保护
// 返回:
//   - int64: 63位随机整数
func (r *lockedSource) Int63() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.src.Int63()
}

// Seed 模拟math/rand.(*Rand).Seed，使用互斥锁保护
// 参数:
//   - seed: 随机数种子
func (r *lockedSource) Seed(seed int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.src.Seed(seed)
}

// Uint64 模拟math/rand.(*Rand).Uint64，使用互斥锁保护
// 返回:
//   - uint64: 64位无符号随机整数
func (r *lockedSource) Uint64() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.src.Uint64()
}

// Int63n 模拟math/rand.(*Rand).Int63n，使用互斥锁保护
// 优化：统一锁管理，避免重复加锁，使用错误处理替代panic
// 返回[0,n)范围内的随机整数
// 参数:
//   - n: 上界（不包含）
//
// 返回:
//   - int64: [0,n)范围内的随机整数
//   - error: 参数验证错误
func (r *lockedSource) Int63n(n int64) (int64, error) {
	if n <= 0 {
		return 0, errors.New("invalid argument")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 快速路径：n是2的幂
	if n&(n-1) == 0 {
		return r.src.Int63() & (n - 1), nil
	}

	// 使用拒绝采样避免模运算的偏差
	// 添加最大尝试次数保护，防止理论上的无限循环
	maxInt := int64((1 << 63) - 1 - (1<<63)%uint64(n))
	const maxAttempts = 1000 // 理论上几乎不可能达到这个次数
	for attempt := 0; attempt < maxAttempts; attempt++ {
		v := r.src.Int63()
		if v <= maxInt {
			return v % n, nil
		}
	}

	// 如果达到最大尝试次数（理论上不应该发生），
	// 返回一个有轻微偏差但仍然有效的随机数
	return r.src.Int63() % n, nil
}
