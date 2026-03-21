// Package base64 提供统一的Base64编解码接口 | Package base64 provides unified Base64 encoding/decoding interface
//
// 本包通过build tags支持两种实现：
// This package supports two implementations via build tags:
//
//  1. 默认实现（标准库）：使用encoding/base64，兼容性好，支持流式编解码
//     Default implementation (standard library): Uses encoding/base64, good compatibility, supports streaming
//     编译命令 | Build command: go build
//
//  2. 高性能实现（base64x）：使用github.com/cloudwego/base64x，性能更高，但不支持流式编解码
//     High performance implementation (base64x): Uses github.com/cloudwego/base64x, higher performance, no streaming support
//     编译命令 | Build command: go build -tags base64x
//
// 使用示例 | Usage example:
//
//	import "go-port-forward/pkg/serializer/base64"
//
//	// 编码 | Encode
//	encoded := base64.StdEncoding.EncodeToString([]byte("hello"))
//
//	// 解码 | Decode
//	decoded, err := base64.StdEncoding.DecodeString(encoded)
package base64

import (
	"errors"
	"io"
)

// Encoding 表示一个Base64编码方案 | Encoding represents a Base64 encoding scheme
type Encoding interface {
	// Encode 将src编码到dst | Encode encodes src to dst
	// dst的长度必须至少为EncodedLen(len(src)) | dst must have length at least EncodedLen(len(src))
	Encode(dst, src []byte)

	// EncodeToString 将src编码为字符串 | EncodeToString encodes src to string
	EncodeToString(src []byte) string

	// Decode 将src解码到dst | Decode decodes src to dst
	// 返回写入dst的字节数和遇到的错误 | Returns number of bytes written to dst and any error
	Decode(dst, src []byte) (n int, err error)

	// DecodeString 将字符串解码为字节切片 | DecodeString decodes string to byte slice
	DecodeString(s string) ([]byte, error)

	// EncodedLen 返回编码后的长度 | EncodedLen returns the encoded length
	EncodedLen(n int) int

	// DecodedLen 返回解码后的长度 | DecodedLen returns the decoded length
	DecodedLen(n int) int
}

// Writer 是一个Base64编码写入器接口 | Writer is a Base64 encoding writer interface
type Writer interface {
	io.WriteCloser
}

// 预定义的编码方案 | Predefined encoding schemes
var (
	// StdEncoding 标准Base64编码（RFC 4648） | Standard Base64 encoding (RFC 4648)
	StdEncoding Encoding

	// URLEncoding URL安全的Base64编码 | URL-safe Base64 encoding
	URLEncoding Encoding

	// RawStdEncoding 无填充的标准Base64编码 | Standard Base64 encoding without padding
	RawStdEncoding Encoding

	// RawURLEncoding 无填充的URL安全Base64编码 | URL-safe Base64 encoding without padding
	RawURLEncoding Encoding
)

// 错误定义 | Error definitions
var (
	// ErrStreamingNotSupported 流式编解码不支持错误 | Streaming encoding/decoding not supported error
	ErrStreamingNotSupported = errors.New("streaming encoding/decoding is not supported by this implementation, use EncodeToString/DecodeString instead")
)

// NewEncoder 创建一个新的Base64编码写入器 | NewEncoder creates a new Base64 encoding writer
// 注意：仅标准库实现支持此方法，base64x实现会返回ErrStreamingNotSupported错误
// Note: Only standard library implementation supports this method, base64x will return ErrStreamingNotSupported
func NewEncoder(enc Encoding, w io.Writer) (Writer, error) {
	return newEncoder(enc, w)
}

// NewDecoder 创建一个新的Base64解码读取器 | NewDecoder creates a new Base64 decoding reader
// 注意：仅标准库实现支持此方法，base64x实现会返回ErrStreamingNotSupported错误
// Note: Only standard library implementation supports this method, base64x will return ErrStreamingNotSupported
func NewDecoder(enc Encoding, r io.Reader) (io.Reader, error) {
	return newDecoder(enc, r)
}
