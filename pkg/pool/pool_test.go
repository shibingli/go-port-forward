// Package pool 提供对象池和字节池管理功能测试
// Package pool provides object pool and byte pool management tests
package pool

import (
	"sync"
	"testing"
	"time"
)

// TestInitGoroutinePool 测试协程池初始化 | Test goroutine pool initialization
func TestInitGoroutinePool(t *testing.T) {
	tests := []struct {
		name     string
		size     int
		preAlloc bool
		wantErr  bool
	}{
		{
			name:     "有效的协程池配置 | Valid pool config",
			size:     100,
			preAlloc: true,
			wantErr:  false,
		},
		{
			name:     "无预分配的协程池 | Pool without pre-allocation",
			size:     50,
			preAlloc: false,
			wantErr:  false,
		},
		{
			name:     "大容量协程池 | Large capacity pool",
			size:     10000,
			preAlloc: true,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 清理之前的池 | Clean up previous pool
			Release()
			goroutinePool = nil

			err := InitGoroutinePool(tt.size, tt.preAlloc)
			if (err != nil) != tt.wantErr {
				t.Errorf("InitGoroutinePool() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil {
				if Cap() != tt.size {
					t.Errorf("Cap() = %v, want %v", Cap(), tt.size)
				}
			}

			// 清理 | Cleanup
			Release()
		})
	}
}

// TestSubmit 测试任务提交 | Test task submission
func TestSubmit(t *testing.T) {
	// 初始化协程池 | Initialize goroutine pool
	Release()
	goroutinePool = nil
	err := InitGoroutinePool(10, true)
	if err != nil {
		t.Fatalf("Failed to initialize goroutine pool: %v", err)
	}
	defer Release()

	// 测试任务提交 | Test task submission
	//
	// 【规范例外说明 Standards Exception Note】
	// 此处保留 sync.WaitGroup 是合理的例外场景：
	// pool.Submit 接受 func()（非 func() error），goroutine 生命周期由协程池管理。
	// WaitGroup 作为 pool-task 完成追踪器，与直接 go func() 的并发模式不同，
	// 无法直接替换为 errgroup（errgroup.Go 只接受 func() error，且自行管理 goroutine）。
	//
	// This use of sync.WaitGroup is a justified exception:
	// pool.Submit takes func() (not func() error); goroutine lifecycle is managed by the pool.
	// WaitGroup serves as a completion tracker for pool-managed tasks, which differs from
	// direct goroutine launches. Cannot replace with errgroup (errgroup.Go takes func() error
	// and manages its own goroutines).
	var wg sync.WaitGroup
	counter := 0
	mu := sync.Mutex{}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		err := Submit(func() {
			defer wg.Done()
			mu.Lock()
			counter++
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
		})
		if err != nil {
			t.Errorf("Submit() error = %v", err)
			wg.Done()
		}
	}

	wg.Wait()

	if counter != 100 {
		t.Errorf("Expected counter = 100, got %d", counter)
	}
}

// TestSubmitWithoutInit 测试未初始化时提交任务 | Test submit without initialization
func TestSubmitWithoutInit(t *testing.T) {
	// 清理池 | Clean up pool
	Release()
	goroutinePool = nil

	// 【规范例外说明 Standards Exception Note】
	// pool.Submit 接受 func()，goroutine 由协程池管理，此处 WaitGroup 是合理的任务完成追踪器。
	// pool.Submit takes func(); goroutine is managed by the pool. WaitGroup is justified here
	// as a task completion tracker for pool-managed goroutines.
	var wg sync.WaitGroup
	wg.Add(1)

	// 应该自动初始化 | Should auto-initialize
	err := Submit(func() {
		defer wg.Done()
	})

	if err != nil {
		t.Errorf("Submit() without init error = %v", err)
	}

	wg.Wait()

	// 清理 | Cleanup
	Release()
}

// TestPoolStats 测试池统计信息 | Test pool statistics
func TestPoolStats(t *testing.T) {
	// 初始化协程池 | Initialize goroutine pool
	Release()
	goroutinePool = nil
	err := InitGoroutinePool(10, true)
	if err != nil {
		t.Fatalf("Failed to initialize goroutine pool: %v", err)
	}
	defer Release()

	// 测试统计信息 | Test statistics
	if Cap() != 10 {
		t.Errorf("Cap() = %v, want 10", Cap())
	}

	if Running() < 0 {
		t.Errorf("Running() should be >= 0, got %d", Running())
	}

	if Free() < 0 {
		t.Errorf("Free() should be >= 0, got %d", Free())
	}
}

// TestByteBuffer 测试字节缓冲池 | Test byte buffer pool
func TestByteBuffer(t *testing.T) {
	// 获取缓冲区 | Get buffer
	buf := GetByteBuffer()
	if buf == nil {
		t.Fatal("GetByteBuffer() returned nil")
	}

	// 写入数据 | Write data
	testData := []byte("测试数据 | Test data")
	_, err := buf.Write(testData)
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}

	// 验证数据 | Verify data
	if buf.Len() != len(testData) {
		t.Errorf("Buffer length = %d, want %d", buf.Len(), len(testData))
	}

	// 归还缓冲区 | Return buffer
	PutByteBuffer(buf)

	// 再次获取应该得到清空的缓冲区 | Get again should return clean buffer
	buf2 := GetByteBuffer()
	if buf2 == nil {
		t.Fatal("GetByteBuffer() returned nil")
	}
	if buf2.Len() != 0 {
		t.Errorf("Reused buffer should be empty, got length %d", buf2.Len())
	}

	PutByteBuffer(buf2)
}

// TestPoolStatsWithoutInit 测试未初始化时的池统计 | Test pool stats without initialization
func TestPoolStatsWithoutInit(t *testing.T) {
	// 清理池 | Clean up pool
	Release()
	goroutinePool = nil

	// 测试统计信息 | Test statistics
	if Running() != 0 {
		t.Errorf("Running() should be 0 when pool is nil, got %d", Running())
	}

	if Free() != 0 {
		t.Errorf("Free() should be 0 when pool is nil, got %d", Free())
	}

	if Cap() != 0 {
		t.Errorf("Cap() should be 0 when pool is nil, got %d", Cap())
	}
}

// TestReleaseWithoutInit 测试未初始化时的释放 | Test release without initialization
func TestReleaseWithoutInit(t *testing.T) {
	// 清理池 | Clean up pool
	goroutinePool = nil

	// 应该不会panic | Should not panic
	Release()
}
