//go:build !base64x
// +build !base64x

// Package base64 标准库实现（默认） | standard library implementation (default)
package base64

import (
	"encoding/base64"
	"errors"
	"io"
)

// stdEncoding 包装encoding/base64.Encoding以实现Encoding接口
// stdEncoding wraps encoding/base64.Encoding to implement Encoding interface
type stdEncoding struct {
	enc *base64.Encoding
}

// Encode 编码 | Encode
func (e *stdEncoding) Encode(dst, src []byte) {
	e.enc.Encode(dst, src)
}

// EncodeToString 编码为字符串 | Encode to string
func (e *stdEncoding) EncodeToString(src []byte) string {
	return e.enc.EncodeToString(src)
}

// Decode 解码 | Decode
func (e *stdEncoding) Decode(dst, src []byte) (n int, err error) {
	return e.enc.Decode(dst, src)
}

// DecodeString 解码字符串 | Decode string
func (e *stdEncoding) DecodeString(s string) ([]byte, error) {
	return e.enc.DecodeString(s)
}

// EncodedLen 返回编码后的长度 | Return encoded length
func (e *stdEncoding) EncodedLen(n int) int {
	return e.enc.EncodedLen(n)
}

// DecodedLen 返回解码后的长度 | Return decoded length
func (e *stdEncoding) DecodedLen(n int) int {
	return e.enc.DecodedLen(n)
}

func init() {
	// 初始化预定义的编码方案 | Initialize predefined encoding schemes
	StdEncoding = &stdEncoding{enc: base64.StdEncoding}
	URLEncoding = &stdEncoding{enc: base64.URLEncoding}
	RawStdEncoding = &stdEncoding{enc: base64.RawStdEncoding}
	RawURLEncoding = &stdEncoding{enc: base64.RawURLEncoding}
}

// stdWriter 包装base64编码器以实现Writer接口
// stdWriter wraps base64 encoder to implement Writer interface
type stdWriter struct {
	encoder io.WriteCloser
}

// Write 写入数据 | Write data
func (w *stdWriter) Write(p []byte) (n int, err error) {
	return w.encoder.Write(p)
}

// Close 关闭编码器 | Close encoder
func (w *stdWriter) Close() error {
	return w.encoder.Close()
}

// newEncoder 创建标准库的编码写入器 | Create standard library encoding writer
func newEncoder(enc Encoding, w io.Writer) (Writer, error) {
	stdEnc, ok := enc.(*stdEncoding)
	if !ok {
		return nil, errors.New("encoding must be created by this package")
	}
	return &stdWriter{
		encoder: base64.NewEncoder(stdEnc.enc, w),
	}, nil
}

// newDecoder 创建标准库的解码读取器 | Create standard library decoding reader
func newDecoder(enc Encoding, r io.Reader) (io.Reader, error) {
	stdEnc, ok := enc.(*stdEncoding)
	if !ok {
		return nil, errors.New("encoding must be created by this package")
	}
	return base64.NewDecoder(stdEnc.enc, r), nil
}
