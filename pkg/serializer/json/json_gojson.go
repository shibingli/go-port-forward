//go:build go_json
// +build go_json

// Package json 提供JSON序列化功能 | Package json provides JSON serialization functionality
package json

import (
	"bytes"
	"io"

	json "github.com/goccy/go-json"
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
	return "github.com/goccy/go-json"
}

// Preload 预热JSON序列化器（go-json不需要预热）| Preload JSON serializer (not needed for go-json)
func Preload(types ...any) {
	// go-json不需要预热 | go-json doesn't need preloading
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
// 使用go-json原生类型（别名到encoding/json）| Using go-json native type (alias to encoding/json)
type RawMessage = json.RawMessage

// Number JSON数字字面量类型 | JSON number literal type
// 使用go-json原生类型（别名到encoding/json）| Using go-json native type (alias to encoding/json)
type Number = json.Number

// Token JSON令牌类型，用于流式解码 | JSON token type for streaming decoding
// 使用go-json原生类型（别名到encoding/json）| Using go-json native type (alias to encoding/json)
type Token = json.Token

// Delim JSON数组或对象分隔符 | JSON array or object delimiter
// 使用go-json原生类型（别名到encoding/json）| Using go-json native type (alias to encoding/json)
type Delim = json.Delim

// Marshaler JSON序列化接口 | JSON marshaler interface
// 使用go-json原生接口 | Using go-json native interface
type Marshaler = json.Marshaler

// Unmarshaler JSON反序列化接口 | JSON unmarshaler interface
// 使用go-json原生接口 | Using go-json native interface
type Unmarshaler = json.Unmarshaler

// InvalidUnmarshalError 无效的Unmarshal目标错误 | Invalid Unmarshal target error
// 使用go-json原生类型（别名到encoding/json）| Using go-json native type (alias to encoding/json)
type InvalidUnmarshalError = json.InvalidUnmarshalError

// UnmarshalTypeError 类型不匹配错误 | Type mismatch error
// 使用go-json原生类型（别名到encoding/json）| Using go-json native type (alias to encoding/json)
type UnmarshalTypeError = json.UnmarshalTypeError

// SyntaxError JSON语法错误 | JSON syntax error
// 使用go-json原生类型（别名到encoding/json）| Using go-json native type (alias to encoding/json)
type SyntaxError = json.SyntaxError

// MarshalerError Marshaler接口返回的错误 | Error returned by Marshaler interface
// 使用go-json原生类型（别名到encoding/json）| Using go-json native type (alias to encoding/json)
type MarshalerError = json.MarshalerError

// UnsupportedTypeError 不支持的类型错误 | Unsupported type error
// 使用go-json原生类型（别名到encoding/json）| Using go-json native type (alias to encoding/json)
type UnsupportedTypeError = json.UnsupportedTypeError

// UnsupportedValueError 不支持的值错误 | Unsupported value error
// 使用go-json原生类型（别名到encoding/json）| Using go-json native type (alias to encoding/json)
type UnsupportedValueError = json.UnsupportedValueError

// Valid 检查数据是否为有效的JSON编码 | Check if data is valid JSON encoding
// 使用go-json原生实现 | Using go-json native implementation
func Valid(data []byte) bool {
	return json.Valid(data)
}

// Compact 将JSON编码的src追加到dst，去除无意义的空白字符
// Append JSON-encoded src to dst with insignificant space characters elided
// 使用go-json原生实现 | Using go-json native implementation
func Compact(dst *bytes.Buffer, src []byte) error {
	return json.Compact(dst, src)
}

// HTMLEscape 将JSON编码的src追加到dst，对HTML特殊字符进行转义
// Append JSON-encoded src to dst with HTML special characters escaped
// 使用go-json原生实现 | Using go-json native implementation
func HTMLEscape(dst *bytes.Buffer, src []byte) {
	json.HTMLEscape(dst, src)
}

// Indent 将JSON编码的src格式化后追加到dst
// Append indented form of JSON-encoded src to dst
// 使用go-json原生实现 | Using go-json native implementation
func Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	return json.Indent(dst, src, prefix, indent)
}
