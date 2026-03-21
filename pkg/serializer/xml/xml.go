// Package xml 提供XML序列化功能
// Package xml provides XML serialization functionality
//
// 本包提供了与标准库encoding/xml完全兼容的API
// This package provides APIs fully compatible with encoding/xml
//
// 使用示例 | Usage Example:
//
//	import "go-port-forward/pkg/serializer/xml"
//
//	// 基本使用 | Basic usage
//	data, err := xml.Marshal(obj)
//	err = xml.Unmarshal(data, &obj)
//
//	// 格式化输出 | Formatted output
//	data, err := xml.MarshalIndent(obj, "", "  ")
//
//	// 使用对象池优化 | Use object pool optimization
//	buf, err := xml.MarshalToBuffer(obj)
//	if err == nil {
//		defer pool.PutByteBuffer(buf)
//		// 使用buf.Bytes() | Use buf.Bytes()
//	}
package xml

import (
	"encoding/xml"

	"github.com/valyala/bytebufferpool"

	"go-port-forward/pkg/pool"
)

// Marshal XML序列化 | XML marshal
// 兼容标准库encoding/xml.Marshal | Compatible with encoding/xml.Marshal
func Marshal(v any) ([]byte, error) {
	return xml.Marshal(v)
}

// MarshalIndent XML格式化序列化 | XML marshal with indentation
// 兼容标准库encoding/xml.MarshalIndent | Compatible with encoding/xml.MarshalIndent
func MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	return xml.MarshalIndent(v, prefix, indent)
}

// Unmarshal XML反序列化 | XML unmarshal
// 兼容标准库encoding/xml.Unmarshal | Compatible with encoding/xml.Unmarshal
func Unmarshal(data []byte, v any) error {
	return xml.Unmarshal(data, v)
}

// MarshalToBuffer 使用字节池优化的XML序列化 | XML marshal with byte pool optimization
// 返回的ByteBuffer使用完后必须调用pool.PutByteBuffer归还 | Must call pool.PutByteBuffer after use
func MarshalToBuffer(v any) (*bytebufferpool.ByteBuffer, error) {
	buf := pool.GetByteBuffer()

	encoder := xml.NewEncoder(buf)
	if err := encoder.Encode(v); err != nil {
		pool.PutByteBuffer(buf)
		return nil, err
	}

	return buf, nil
}

// MarshalIndentToBuffer 使用字节池优化的XML格式化序列化 | XML marshal indent with byte pool optimization
// 返回的ByteBuffer使用完后必须调用pool.PutByteBuffer归还 | Must call pool.PutByteBuffer after use
func MarshalIndentToBuffer(v any, prefix, indent string) (*bytebufferpool.ByteBuffer, error) {
	data, err := xml.MarshalIndent(v, prefix, indent)
	if err != nil {
		return nil, err
	}

	buf := pool.GetByteBuffer()
	_, _ = buf.Write(data)

	return buf, nil
}

// Name 返回当前使用的XML序列化器名称 | Return current XML serializer name
func Name() string {
	return "encoding/xml"
}
