//go:build base64x && amd64
// +build base64x,amd64

// Package base64 base64x实现（高性能） | base64x implementation (high performance)
package base64

import (
	"go-port-forward/pkg/pool"
	"io"
	"sync"

	"github.com/cloudwego/base64x"
	"github.com/valyala/bytebufferpool"
)

// base64xEncoding 包装base64x.Encoding以实现Encoding接口
// base64xEncoding wraps base64x.Encoding to implement Encoding interface
type base64xEncoding struct {
	enc base64x.Encoding
}

// Encode 编码 | Encode
func (e *base64xEncoding) Encode(dst, src []byte) {
	e.enc.Encode(dst, src)
}

// EncodeToString 编码为字符串 | Encode to string
func (e *base64xEncoding) EncodeToString(src []byte) string {
	return e.enc.EncodeToString(src)
}

// Decode 解码 | Decode
func (e *base64xEncoding) Decode(dst, src []byte) (n int, err error) {
	return e.enc.Decode(dst, src)
}

// DecodeString 解码字符串 | Decode string
func (e *base64xEncoding) DecodeString(s string) ([]byte, error) {
	return e.enc.DecodeString(s)
}

// EncodedLen 返回编码后的长度 | Return encoded length
func (e *base64xEncoding) EncodedLen(n int) int {
	return e.enc.EncodedLen(n)
}

// DecodedLen 返回解码后的长度 | Return decoded length
func (e *base64xEncoding) DecodedLen(n int) int {
	return e.enc.DecodedLen(n)
}

var (
	// writerPool Writer对象池 | Writer object pool
	writerPool = sync.Pool{
		New: func() any {
			return new(base64xWriter)
		},
	}

	// readerPool Reader对象池 | Reader object pool
	readerPool = sync.Pool{
		New: func() any {
			return new(base64xReader)
		},
	}
)

func init() {
	// 初始化预定义的编码方案 | Initialize predefined encoding schemes
	StdEncoding = &base64xEncoding{enc: base64x.StdEncoding}
	URLEncoding = &base64xEncoding{enc: base64x.URLEncoding}
	RawStdEncoding = &base64xEncoding{enc: base64x.RawStdEncoding}
	RawURLEncoding = &base64xEncoding{enc: base64x.RawURLEncoding}
}

// base64xWriter 包装base64x编码器以实现流式编码 | base64xWriter wraps base64x encoder for streaming encoding
type base64xWriter struct {
	enc    *base64xEncoding           // 编码器 | encoder
	w      io.Writer                  // 底层写入器 | underlying writer
	buf    *bytebufferpool.ByteBuffer // 缓冲区（使用字节池）| buffer (using byte pool)
	closed bool                       // 是否已关闭 | whether closed
}

// Write 写入数据到缓冲区 | Write data to buffer
func (w *base64xWriter) Write(p []byte) (n int, err error) {
	if w.closed {
		return 0, io.ErrClosedPipe
	}
	return w.buf.Write(p)
}

// Close 编码缓冲区数据并写入底层写入器 | Close encodes buffered data and writes to underlying writer
func (w *base64xWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	var err error

	// 编码缓冲区中的所有数据 | Encode all buffered data
	data := w.buf.Bytes()
	if len(data) > 0 {
		encoded := w.enc.EncodeToString(data)
		_, err = w.w.Write([]byte(encoded))
	}

	// 归还缓冲区到池 | Return buffer to pool
	pool.PutByteBuffer(w.buf)

	// 重置Writer并归还到对象池 | Reset Writer and return to pool
	w.enc = nil
	w.w = nil
	w.buf = nil
	writerPool.Put(w)

	return err
}

// base64xReader 包装base64x解码器以实现流式解码 | base64xReader wraps base64x decoder for streaming decoding
type base64xReader struct {
	enc        *base64xEncoding           // 解码器 | decoder
	r          io.Reader                  // 底层读取器 | underlying reader
	buf        *bytebufferpool.ByteBuffer // 缓冲区（使用字节池）| buffer (using byte pool)
	readBuf    []byte                     // 读取缓冲区（使用字节池）| read buffer (using byte pool)
	eof        bool                       // 是否到达EOF | whether reached EOF
	bufferUsed bool                       // 缓冲区是否已使用 | whether buffer is used
}

// Read 从底层读取器读取并解码数据 | Read and decode data from underlying reader
func (r *base64xReader) Read(p []byte) (n int, err error) {
	// 如果缓冲区有数据，先返回缓冲区数据 | If buffer has data, return buffered data first
	if r.buf.Len() > 0 {
		n = copy(p, r.buf.Bytes())
		r.buf.B = r.buf.B[n:]
		return n, nil
	}

	// 如果已经到达EOF，返回EOF | If already reached EOF, return EOF
	if r.eof {
		// 归还缓冲区到池 | Return buffers to pool
		r.release()
		return 0, io.EOF
	}

	// 读取底层数据 | Read underlying data
	// 读取足够的数据以确保是4的倍数（base64编码单元）| Read enough data to ensure multiple of 4 (base64 encoding unit)
	n, err = r.r.Read(r.readBuf)
	if err != nil {
		if err == io.EOF {
			r.eof = true
			if n == 0 {
				// 归还缓冲区到池 | Return buffers to pool
				r.release()
				return 0, io.EOF
			}
		} else {
			// 归还缓冲区到池 | Return buffers to pool
			r.release()
			return 0, err
		}
	}

	// 解码数据 | Decode data
	decoded, decErr := r.enc.DecodeString(string(r.readBuf[:n]))
	if decErr != nil {
		// 归还缓冲区到池 | Return buffers to pool
		r.release()
		return 0, decErr
	}

	// 将解码后的数据写入缓冲区 | Write decoded data to buffer
	_, _ = r.buf.Write(decoded)

	// 从缓冲区读取数据返回 | Read data from buffer and return
	n = copy(p, r.buf.Bytes())
	r.buf.B = r.buf.B[n:]
	return n, nil
}

// release 释放Reader资源并归还到对象池 | Release Reader resources and return to pool
func (r *base64xReader) release() {
	if r.bufferUsed {
		pool.PutByteBuffer(r.buf)
		pool.PutBuffer(r.readBuf)
		r.bufferUsed = false
	}

	// 重置Reader并归还到对象池 | Reset Reader and return to pool
	r.enc = nil
	r.r = nil
	r.buf = nil
	r.readBuf = nil
	readerPool.Put(r)
}

// newEncoder 创建base64x流式编码器 | Create base64x streaming encoder
func newEncoder(enc Encoding, w io.Writer) (Writer, error) {
	b64xEnc, ok := enc.(*base64xEncoding)
	if !ok {
		return nil, ErrStreamingNotSupported
	}

	// 从对象池获取Writer | Get Writer from pool
	writer := writerPool.Get().(*base64xWriter)

	// 从字节池获取缓冲区 | Get buffer from byte pool
	writer.enc = b64xEnc
	writer.w = w
	writer.buf = pool.GetByteBuffer()
	writer.closed = false

	return writer, nil
}

// newDecoder 创建base64x流式解码器 | Create base64x streaming decoder
func newDecoder(enc Encoding, r io.Reader) (io.Reader, error) {
	b64xEnc, ok := enc.(*base64xEncoding)
	if !ok {
		return nil, ErrStreamingNotSupported
	}

	// 从对象池获取Reader | Get Reader from pool
	reader := readerPool.Get().(*base64xReader)

	// 从字节池获取缓冲区 | Get buffers from byte pool
	reader.enc = b64xEnc
	reader.r = r
	reader.buf = pool.GetByteBuffer()
	reader.readBuf = pool.GetBuffer(pool.SmallBufferSize)
	reader.eof = false
	reader.bufferUsed = true

	return reader, nil
}
