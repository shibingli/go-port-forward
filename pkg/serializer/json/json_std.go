//go:build !sonic && !go_json && !jsoniter
// +build !sonic,!go_json,!jsoniter

// Package json 提供JSON序列化功能 | Package json provides JSON serialization functionality
package json

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/valyala/bytebufferpool"

	"go-port-forward/pkg/pool"
)

// Marshal JSON序列化 | JSON marshal
// 兼容标准库encoding/json.Marshal | Compatible with encoding/json.Marshal
func Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// MarshalIndent JSON格式化序列化 | JSON marshal with indentation
// 兼容标准库encoding/json.MarshalIndent | Compatible with encoding/json.MarshalIndent
func MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	return json.MarshalIndent(v, prefix, indent)
}

// Unmarshal JSON反序列化 | JSON unmarshal
// 兼容标准库encoding/json.Unmarshal | Compatible with encoding/json.Unmarshal
func Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// MarshalToBuffer 使用字节池优化的JSON序列化 | JSON marshal with byte pool optimization
// 返回的ByteBuffer使用完后必须调用pool.PutByteBuffer归还 | Must call pool.PutByteBuffer after use
func MarshalToBuffer(v any) (*bytebufferpool.ByteBuffer, error) {
	buf := pool.GetByteBuffer()

	encoder := json.NewEncoder(buf)
	if err := encoder.Encode(v); err != nil {
		pool.PutByteBuffer(buf)
		return nil, err
	}

	// 移除encoder.Encode添加的换行符 | Remove newline added by encoder.Encode
	if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] == '\n' {
		buf.B = buf.B[:buf.Len()-1]
	}

	return buf, nil
}

// MarshalIndentToBuffer 使用字节池优化的JSON格式化序列化 | JSON marshal indent with byte pool optimization
// 返回的ByteBuffer使用完后必须调用pool.PutByteBuffer归还 | Must call pool.PutByteBuffer after use
func MarshalIndentToBuffer(v any, prefix, indent string) (*bytebufferpool.ByteBuffer, error) {
	data, err := json.MarshalIndent(v, prefix, indent)
	if err != nil {
		return nil, err
	}

	buf := pool.GetByteBuffer()
	_, _ = buf.Write(data)

	return buf, nil
}

// Name 返回当前使用的JSON序列化器名称 | Return current JSON serializer name
func Name() string {
	return "encoding/json"
}

// Preload 预热JSON序列化器（标准库不需要预热）| Preload JSON serializer (not needed for stdlib)
func Preload(types ...any) {
	// 标准库不需要预热 | Standard library doesn't need preloading
}

// Decoder JSON解码器 | JSON decoder
type Decoder = json.Decoder

// Encoder JSON编码器 | JSON encoder
type Encoder = json.Encoder

// NewDecoder 创建JSON解码器 | Create JSON decoder
// 兼容标准库encoding/json.NewDecoder | Compatible with encoding/json.NewDecoder
func NewDecoder(r io.Reader) *Decoder {
	return json.NewDecoder(r)
}

// NewEncoder 创建JSON编码器 | Create JSON encoder
// 兼容标准库encoding/json.NewEncoder | Compatible with encoding/json.NewEncoder
func NewEncoder(w io.Writer) *Encoder {
	return json.NewEncoder(w)
}

// RawMessage 原始JSON消息类型，用于延迟JSON解码或预计算JSON编码
// Raw JSON message type for delayed JSON decoding or precomputed JSON encoding
type RawMessage = json.RawMessage

// Number JSON数字字面量类型 | JSON number literal type
type Number = json.Number

// Token JSON令牌类型，用于流式解码 | JSON token type for streaming decoding
type Token = json.Token

// Delim JSON数组或对象分隔符 | JSON array or object delimiter
type Delim = json.Delim

// Marshaler JSON序列化接口 | JSON marshaler interface
type Marshaler = json.Marshaler

// Unmarshaler JSON反序列化接口 | JSON unmarshaler interface
type Unmarshaler = json.Unmarshaler

// InvalidUnmarshalError 无效的Unmarshal目标错误 | Invalid Unmarshal target error
type InvalidUnmarshalError = json.InvalidUnmarshalError

// UnmarshalTypeError 类型不匹配错误 | Type mismatch error
type UnmarshalTypeError = json.UnmarshalTypeError

// SyntaxError JSON语法错误 | JSON syntax error
type SyntaxError = json.SyntaxError

// MarshalerError Marshaler接口返回的错误 | Error returned by Marshaler interface
type MarshalerError = json.MarshalerError

// UnsupportedTypeError 不支持的类型错误 | Unsupported type error
type UnsupportedTypeError = json.UnsupportedTypeError

// UnsupportedValueError 不支持的值错误 | Unsupported value error
type UnsupportedValueError = json.UnsupportedValueError

// Valid 检查数据是否为有效的JSON编码 | Check if data is valid JSON encoding
// 兼容标准库encoding/json.Valid | Compatible with encoding/json.Valid
func Valid(data []byte) bool {
	return json.Valid(data)
}

// Compact 将JSON编码的src追加到dst，去除无意义的空白字符
// Append JSON-encoded src to dst with insignificant space characters elided
// 兼容标准库encoding/json.Compact | Compatible with encoding/json.Compact
func Compact(dst *bytes.Buffer, src []byte) error {
	return json.Compact(dst, src)
}

// HTMLEscape 将JSON编码的src追加到dst，对HTML特殊字符进行转义
// Append JSON-encoded src to dst with HTML special characters escaped
// 兼容标准库encoding/json.HTMLEscape | Compatible with encoding/json.HTMLEscape
func HTMLEscape(dst *bytes.Buffer, src []byte) {
	json.HTMLEscape(dst, src)
}

// Indent 将JSON编码的src格式化后追加到dst
// Append indented form of JSON-encoded src to dst
// 兼容标准库encoding/json.Indent | Compatible with encoding/json.Indent
func Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	return json.Indent(dst, src, prefix, indent)
}
