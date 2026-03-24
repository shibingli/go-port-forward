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
	"sync/atomic"
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
	// 使用统一的 GetBuffer/PutBuffer API，确保归还时重置 slice 长度
	// Use unified GetBuffer/PutBuffer API to ensure slice length is reset on return
	buffer := pool.GetBuffer(pool.DefaultBufferSize)
	defer pool.PutBuffer(buffer)

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

	// 使用 channel 传递结果，避免共享变量的 data race。
	// Use a channel to pass results, avoiding data race on shared variables.
	type result struct {
		written int64
		err     error
	}
	done := make(chan result, 1)

	// 在协程池中执行复制操作
	if err := pool.Submit(func() {
		w, e := CopyWithBuffer(dst, src)
		done <- result{w, e}
	}); err != nil {
		return 0, err
	}

	// 等待复制完成或上下文取消
	select {
	case r := <-done:
		return r.written, r.err
	case <-ctx.Done():
		// 超时/取消时，拷贝协程可能仍在运行，不读取共享变量以避免 data race。
		// On timeout/cancel, copy goroutine may still be running; avoid reading shared vars.
		return 0, ctx.Err()
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
	// 使用统一的 GetBuffer/PutBuffer API | Use unified GetBuffer/PutBuffer API
	buffer := pool.GetBuffer(pool.DefaultBufferSize)
	defer pool.PutBuffer(buffer)

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
//
// 注意：progressCallback 在拷贝协程中调用，超时后协程不会立即终止，
// 回调中应使用 atomic 或其他并发安全方式读取进度值。
// Note: progressCallback is invoked in the copy goroutine; the goroutine does not
// stop immediately on timeout. Use atomic or other concurrency-safe reads in the callback.
func CopyWithProgressAndTimeout(ctx context.Context, dst io.Writer, src io.Reader,
	progressCallback func(written int64), timeout time.Duration) (written int64, err error) {

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// 使用 atomic 避免 data race：超时时拷贝协程可能仍在更新 written。
	// Use atomic to avoid data race: copy goroutine may still update written after timeout.
	var atomicWritten atomic.Int64

	type result struct {
		written int64
		err     error
	}
	done := make(chan result, 1)

	// 包装 progressCallback，使其同时更新 atomic 计数器
	wrappedCallback := func(w int64) {
		atomicWritten.Store(w)
		if progressCallback != nil {
			progressCallback(w)
		}
	}

	if err := pool.Submit(func() {
		w, e := CopyWithProgress(dst, src, wrappedCallback)
		done <- result{w, e}
	}); err != nil {
		return 0, err
	}

	select {
	case r := <-done:
		return r.written, r.err
	case <-ctx.Done():
		// 返回最后已知的安全进度值 | Return last known safe progress value
		return atomicWritten.Load(), ctx.Err()
	}
}

// CopyN 复制指定字节数 | Copy specified number of bytes
//
// 使用池化缓冲区 + io.LimitReader 替代 io.CopyN，避免标准库内部重复分配缓冲区。
// Uses pooled buffer + io.LimitReader instead of io.CopyN to avoid redundant
// buffer allocation inside the standard library.
//
// 返回值语义与 io.CopyN 完全一致：
// Return value semantics are identical to io.CopyN:
//   - written == n: err = nil
//   - written < n && 底层无错误: err = io.EOF
//   - 底层有错误: 返回底层错误
func CopyN(dst io.Writer, src io.Reader, n int64) (written int64, err error) {
	buffer := pool.GetBuffer(int(min(n, int64(pool.MediumBufferSize))))
	defer pool.PutBuffer(buffer)

	written, err = io.CopyBuffer(dst, io.LimitReader(src, n), buffer)
	if written == n {
		return n, nil
	}
	if written < n && err == nil {
		// src 提前结束，一定是遇到了 EOF | src stopped early; must have been EOF.
		err = io.EOF
	}
	return
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
