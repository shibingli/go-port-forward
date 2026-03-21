// Package ioutil 提供高性能的IO操作工具 | Provides high-performance IO utilities
//
// 本包扩展了标准库的io功能，提供：| This package extends standard io with:
// - 高性能复制：使用对象池减少内存分配 | High-performance copy: uses object pool to reduce allocations
// - 超时控制：支持上下文取消和超时 | Timeout control: supports context cancellation and timeout
// - 进度监控：提供复制进度回调 | Progress monitoring: provides copy progress callback
// - 内存优化：智能缓冲区管理 | Memory optimization: smart buffer management
//
// 设计原则 | Design principles:
// - 零拷贝：尽可能减少内存拷贝 | Zero-copy: minimize memory copies
// - 对象池：复用缓冲区减少GC压力 | Object pool: reuse buffers to reduce GC pressure
// - 上下文控制：支持取消和超时 | Context control: supports cancellation and timeout
// - 进度可见：提供操作进度反馈 | Progress visibility: provides operation progress feedback
package ioutil

import (
	"context"
	"errors"
	"go-port-forward/pkg/pool"
	"io"
	"time"
)

// CopyWithBuffer 使用对象池中的缓冲区进行高效复制 | Copy efficiently using buffer from object pool
//
// 相比标准库的io.Copy，此函数使用预分配的缓冲区池，
// 避免每次复制都分配新的缓冲区，显著减少GC压力
// Compared to io.Copy, this function uses pre-allocated buffer pool,
// avoiding new buffer allocation per copy, significantly reducing GC pressure
//
// 参数:
//   - dst: 目标写入器
//   - src: 源读取器
//
// 返回:
//   - written: 复制的字节数
//   - err: 复制过程中的错误
func CopyWithBuffer(dst io.Writer, src io.Reader) (written int64, err error) {
	// 从对象池获取缓冲区
	buffer := pool.BufferPool.Get().([]byte)
	defer pool.BufferPool.Put(buffer) // 使用完毕后归还到池中

	return io.CopyBuffer(dst, src, buffer)
}

// CopyWithTimeout 带超时控制的复制操作 | Copy with timeout control
//
// 在CopyWithBuffer的基础上增加超时控制，支持：| Adds timeout control on top of CopyWithBuffer:
// 1. 上下文取消：可以通过context取消操作 | Context cancellation
// 2. 超时控制：设置最大执行时间 | Timeout control
// 3. 并发安全：使用goroutine异步执行，不阻塞调用者 | Concurrency safe
//
// 参数:
//   - ctx: 上下文，用于取消控制
//   - dst: 目标写入器
//   - src: 源读取器
//   - timeout: 超时时间，0表示不设置超时
//
// 返回:
//   - written: 复制的字节数
//   - err: 复制过程中的错误或超时错误
func CopyWithTimeout(ctx context.Context, dst io.Writer, src io.Reader, timeout time.Duration) (written int64, err error) {
	// 如果设置了超时时间，创建带超时的上下文
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// 使用channel进行goroutine间通信
	done := make(chan struct{})
	var copyErr error
	var copyWritten int64

	// 在协程池中执行复制操作
	if err := pool.Submit(func() {
		defer close(done)
		copyWritten, copyErr = CopyWithBuffer(dst, src)
	}); err != nil {
		return 0, err
	}

	// 等待复制完成或上下文取消
	select {
	case <-done:
		return copyWritten, copyErr
	case <-ctx.Done():
		return copyWritten, ctx.Err()
	}
}

// CopyWithProgress 带进度回调的复制操作 | Copy with progress callback
//
// 在复制过程中提供实时进度反馈，适用于：| Provides real-time progress feedback during copy:
// 1. 大文件传输：显示传输进度 | Large file transfer: show progress
// 2. 用户界面：更新进度条 | UI: update progress bar
// 3. 监控系统：记录传输状态 | Monitoring: record transfer status
//
// 实现细节：
// - 使用对象池优化内存分配
// - 每次写入后调用进度回调
// - 处理短写入和错误情况
//
// 参数:
//   - dst: 目标写入器
//   - src: 源读取器
//   - progressCallback: 进度回调函数，参数为已写入的字节数
//
// 返回:
//   - written: 复制的字节数
//   - err: 复制过程中的错误
func CopyWithProgress(dst io.Writer, src io.Reader, progressCallback func(written int64)) (written int64, err error) {
	// 从对象池获取缓冲区
	buffer := pool.BufferPool.Get().([]byte)
	defer pool.BufferPool.Put(buffer)

	var totalWritten int64
	for {
		// 从源读取数据
		nr, er := src.Read(buffer)
		if nr > 0 {
			// 写入目标
			nw, ew := dst.Write(buffer[0:nr])

			// 验证写入结果
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errors.New("invalid write result")
				}
			}

			// 更新总写入量
			totalWritten += int64(nw)

			// 调用进度回调
			if progressCallback != nil {
				progressCallback(totalWritten)
			}

			// 检查写入错误
			if ew != nil {
				err = ew
				break
			}

			// 检查短写入
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}

		// 检查读取错误
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return totalWritten, err
}

// CopyWithProgressAndTimeout 带进度回调和超时的复制操作 | Copy with progress callback and timeout
func CopyWithProgressAndTimeout(ctx context.Context, dst io.Writer, src io.Reader,
	progressCallback func(written int64), timeout time.Duration) (written int64, err error) {

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	done := make(chan struct{})
	var copyErr error
	var copyWritten int64

	if err := pool.Submit(func() {
		defer close(done)
		copyWritten, copyErr = CopyWithProgress(dst, src, progressCallback)
	}); err != nil {
		return 0, err
	}

	select {
	case <-done:
		return copyWritten, copyErr
	case <-ctx.Done():
		return copyWritten, ctx.Err()
	}
}

// CopyN 复制指定字节数 | Copy specified number of bytes
func CopyN(dst io.Writer, src io.Reader, n int64) (written int64, err error) {
	buffer := pool.GetBuffer(int(min(n, int64(pool.MediumBufferSize))))
	defer pool.PutBuffer(buffer)

	return io.CopyN(dst, src, n)
}

// CopyNWithProgress 复制指定字节数并提供进度回调 | Copy specified bytes with progress callback
func CopyNWithProgress(dst io.Writer, src io.Reader, n int64, progressCallback func(written int64)) (written int64, err error) {
	buffer := pool.GetBuffer(int(min(n, int64(pool.MediumBufferSize))))
	defer pool.PutBuffer(buffer)

	var totalWritten int64
	remaining := n

	for remaining > 0 {
		toRead := int64(len(buffer))
		if remaining < toRead {
			toRead = remaining
		}

		nr, er := src.Read(buffer[:toRead])
		if nr > 0 {
			nw, ew := dst.Write(buffer[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errors.New("invalid write result")
				}
			}
			totalWritten += int64(nw)
			remaining -= int64(nw)

			if progressCallback != nil {
				progressCallback(totalWritten)
			}

			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return totalWritten, err
}

// ReadAll 读取所有数据，使用对象池优化 | Read all data with object pool optimization
func ReadAll(r io.Reader) ([]byte, error) {
	buf := pool.GetBytesBuffer()
	defer pool.PutBytesBuffer(buf)

	_, err := buf.ReadFrom(r)
	if err != nil {
		return nil, err
	}

	// 复制数据，因为 buffer 会被放回池中
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result, nil
}

// WriteString 写入字符串，使用对象池优化 | Write string with object pool optimization
func WriteString(w io.Writer, s string) (n int, err error) {
	if sw, ok := w.(io.StringWriter); ok {
		return sw.WriteString(s)
	}
	return w.Write([]byte(s))
}
