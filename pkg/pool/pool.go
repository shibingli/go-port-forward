// Package pool 提供对象池和字节池管理功能
// Package pool provides object pool and byte pool management
package pool

import (
	"errors"
	"sync"

	"github.com/panjf2000/ants/v2"
	"github.com/valyala/bytebufferpool"
)

const (
	// SmallBufferSize 小缓冲区大小 | Small buffer size
	SmallBufferSize = 4 * 1024 // 4KB
	// MediumBufferSize 中等缓冲区大小 | Medium buffer size
	MediumBufferSize = 32 * 1024 // 32KB
	// LargeBufferSize 大缓冲区大小 | Large buffer size
	LargeBufferSize = 128 * 1024 // 128KB
	// DefaultBufferSize 默认缓冲区大小 | Default buffer size
	DefaultBufferSize = MediumBufferSize
)

var (
	// goroutinePool 协程池实例（阻塞模式，池满时等待）| Goroutine pool instance (blocking mode, waits when full)
	goroutinePool *ants.Pool
	goroutineMu   sync.RWMutex

	// nonBlockingPool 非阻塞协程池实例（池满时立即返回 ErrPoolOverload）
	// Non-blocking goroutine pool instance (returns ErrPoolOverload immediately when full)
	nonBlockingPool *ants.Pool
	nonBlockingMu   sync.RWMutex

	// BufferPool 字节切片对象池 | Byte slice object pool
	BufferPool = sync.Pool{
		New: func() any {
			buf := make([]byte, DefaultBufferSize)
			return buf
		},
	}

	// SmallBufferPool 小缓冲区对象池 | Small buffer object pool
	SmallBufferPool = sync.Pool{
		New: func() any {
			buf := make([]byte, SmallBufferSize)
			return buf
		},
	}

	// LargeBufferPool 大缓冲区对象池 | Large buffer object pool
	LargeBufferPool = sync.Pool{
		New: func() any {
			buf := make([]byte, LargeBufferSize)
			return buf
		},
	}
)

// InitGoroutinePool 初始化协程池（阻塞模式）| Initialize goroutine pool (blocking mode)
// 池满时 Submit 会阻塞等待，适合通用异步任务。
// Submit blocks when pool is full; suitable for general async tasks.
func InitGoroutinePool(size int, preAlloc bool) error {
	if size <= 0 {
		return errors.New("goroutine pool size must be positive")
	}
	if p := getGoroutinePool(); p != nil {
		return nil
	}
	options := ants.Options{
		PreAlloc: preAlloc,
	}
	p, err := ants.NewPool(size, ants.WithOptions(options))
	if err != nil {
		return err
	}
	goroutineMu.Lock()
	if goroutinePool != nil {
		goroutineMu.Unlock()
		p.Release()
		return nil
	}
	goroutinePool = p
	goroutineMu.Unlock()
	return nil
}

// InitNonBlockingPool 初始化非阻塞协程池 | Initialize non-blocking goroutine pool
// 池满时 SubmitNonBlocking 立即返回 ants.ErrPoolOverload，调用方自行决定如何处理（拒绝/降级）。
// When full, SubmitNonBlocking returns ants.ErrPoolOverload immediately;
// caller decides how to handle (reject/degrade).
//
// size: 池容量 | pool capacity
// maxBlocking: 允许排队等待的最大任务数（0 表示不排队，满即拒绝）
//
//	max tasks allowed to queue (0 = no queue, reject immediately when full)
func InitNonBlockingPool(size int, maxBlocking int) error {
	if size <= 0 {
		return errors.New("non-blocking goroutine pool size must be positive")
	}
	if p := getNonBlockingPool(); p != nil {
		return nil
	}
	options := ants.Options{
		Nonblocking:      true,
		MaxBlockingTasks: maxBlocking,
		PreAlloc:         true,
	}
	p, err := ants.NewPool(size, ants.WithOptions(options))
	if err != nil {
		return err
	}
	nonBlockingMu.Lock()
	if nonBlockingPool != nil {
		nonBlockingMu.Unlock()
		p.Release()
		return nil
	}
	nonBlockingPool = p
	nonBlockingMu.Unlock()
	return nil
}

// SubmitNonBlocking 向非阻塞协程池提交任务 | Submit task to non-blocking goroutine pool
// 池满时立即返回 ants.ErrPoolOverload，不阻塞调用方。
// Returns ants.ErrPoolOverload immediately when pool is full; never blocks the caller.
func SubmitNonBlocking(task func()) error {
	if getNonBlockingPool() == nil {
		// 懒初始化：容量 5000，不排队（满即拒绝）
		// Lazy init: capacity 5000, no queue (reject when full)
		if err := InitNonBlockingPool(5000, 0); err != nil {
			return err
		}
	}
	p := getNonBlockingPool()
	if p == nil {
		return errors.New("non-blocking goroutine pool is unavailable")
	}
	return p.Submit(task)
}

// RunningNonBlocking 获取非阻塞池正在运行的协程数 | Get running goroutine count in non-blocking pool
func RunningNonBlocking() int {
	p := getNonBlockingPool()
	if p == nil {
		return 0
	}
	return p.Running()
}

// FreeNonBlocking 获取非阻塞池空闲协程数 | Get free goroutine count in non-blocking pool
func FreeNonBlocking() int {
	p := getNonBlockingPool()
	if p == nil {
		return 0
	}
	return p.Free()
}

// ReleaseNonBlocking 释放非阻塞协程池 | Release non-blocking goroutine pool
func ReleaseNonBlocking() {
	nonBlockingMu.Lock()
	p := nonBlockingPool
	nonBlockingPool = nil
	nonBlockingMu.Unlock()
	if p != nil {
		p.Release()
	}
}

// Submit 提交任务到协程池 | Submit task to goroutine pool
func Submit(task func()) error {
	if getGoroutinePool() == nil {
		// 如果未初始化，使用默认配置 | If not initialized, use default config
		if err := InitGoroutinePool(10000, true); err != nil {
			return err
		}
	}
	p := getGoroutinePool()
	if p == nil {
		return errors.New("goroutine pool is unavailable")
	}
	return p.Submit(task)
}

// Running 获取正在运行的协程数 | Get running goroutine count
func Running() int {
	p := getGoroutinePool()
	if p == nil {
		return 0
	}
	return p.Running()
}

// Free 获取空闲协程数 | Get free goroutine count
func Free() int {
	p := getGoroutinePool()
	if p == nil {
		return 0
	}
	return p.Free()
}

// Cap 获取协程池容量 | Get goroutine pool capacity
func Cap() int {
	p := getGoroutinePool()
	if p == nil {
		return 0
	}
	return p.Cap()
}

// Release 释放协程池 | Release goroutine pool
func Release() {
	goroutineMu.Lock()
	p := goroutinePool
	goroutinePool = nil
	goroutineMu.Unlock()
	if p != nil {
		p.Release()
	}
}

func getGoroutinePool() *ants.Pool {
	goroutineMu.RLock()
	defer goroutineMu.RUnlock()
	return goroutinePool
}

func getNonBlockingPool() *ants.Pool {
	nonBlockingMu.RLock()
	defer nonBlockingMu.RUnlock()
	return nonBlockingPool
}

// GetByteBuffer 从字节池获取缓冲区 | Get byte buffer from pool
func GetByteBuffer() *bytebufferpool.ByteBuffer {
	return bytebufferpool.Get()
}

// PutByteBuffer 将缓冲区放回字节池 | Put byte buffer back to pool
func PutByteBuffer(buf *bytebufferpool.ByteBuffer) {
	bytebufferpool.Put(buf)
}

// GetBytesBuffer 获取ByteBuffer（别名方法，为了兼容性）| Get ByteBuffer (alias method for compatibility)
func GetBytesBuffer() *bytebufferpool.ByteBuffer {
	return GetByteBuffer()
}

// PutBytesBuffer 放回ByteBuffer（别名方法，为了兼容性）| Put ByteBuffer back (alias method for compatibility)
func PutBytesBuffer(buf *bytebufferpool.ByteBuffer) {
	PutByteBuffer(buf)
}

// GetBuffer 从对象池获取指定大小的缓冲区 | Get buffer of specified size from pool
func GetBuffer(size int) []byte {
	if size <= SmallBufferSize {
		return SmallBufferPool.Get().([]byte)
	} else if size <= MediumBufferSize {
		return BufferPool.Get().([]byte)
	}
	return LargeBufferPool.Get().([]byte)
}

// PutBuffer 将缓冲区放回对象池 | Put buffer back to pool
func PutBuffer(buf []byte) {
	if cap(buf) <= SmallBufferSize {
		SmallBufferPool.Put(buf[:SmallBufferSize])
	} else if cap(buf) <= MediumBufferSize {
		BufferPool.Put(buf[:MediumBufferSize])
	} else if cap(buf) <= LargeBufferSize {
		LargeBufferPool.Put(buf[:LargeBufferSize])
	}
	// 超大缓冲区不放回池中，让GC回收 | Don't put oversized buffers back, let GC collect them
}
