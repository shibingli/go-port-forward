//go:build sonic
// +build sonic

// Package json 提供JSON序列化功能（sonic实现）
// Package json provides JSON serialization functionality (sonic implementation)
// 类型定义使用encoding/json别名确保100%兼容性，函数实现使用sonic确保最大性能
// Type definitions use encoding/json aliases for 100% compatibility, function implementations use sonic for maximum performance
package json

import (
	"bytes"
	stdjson "encoding/json"
	"io"
	"reflect"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/decoder"
	"github.com/bytedance/sonic/encoder"
	"github.com/bytedance/sonic/option"
	"github.com/valyala/bytebufferpool"

	"go-port-forward/pkg/pool"
)

// sonicSortedAPI sonic排序键的API实例，用于Compact/Indent保证键序一致性
// sonic API instance with sorted keys, used for Compact/Indent to ensure consistent key order
var sonicSortedAPI = sonic.Config{
	SortMapKeys: true,
}.Froze()

// Marshal JSON序列化 | JSON marshal
// 使用sonic高性能实现 | Using sonic high-performance implementation
func Marshal(v any) ([]byte, error) {
	return sonic.Marshal(v)
}

// MarshalIndent JSON格式化序列化 | JSON marshal with indentation
// 使用sonic高性能实现 | Using sonic high-performance implementation
func MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	return sonic.MarshalIndent(v, prefix, indent)
}

// Unmarshal JSON反序列化 | JSON unmarshal
// 使用sonic高性能实现 | Using sonic high-performance implementation
func Unmarshal(data []byte, v any) error {
	return sonic.Unmarshal(data, v)
}

// MarshalToBuffer 使用字节池优化的JSON序列化 | JSON marshal with byte pool optimization
// 使用sonic流式编码器直接写入buffer，避免中间分配 | Using sonic stream encoder to write directly to buffer, avoiding intermediate allocation
// 返回的ByteBuffer使用完后必须调用pool.PutByteBuffer归还 | Must call pool.PutByteBuffer after use
func MarshalToBuffer(v any) (*bytebufferpool.ByteBuffer, error) {
	buf := pool.GetByteBuffer()

	// 使用sonic流式编码器直接写入buffer | Use sonic stream encoder to write directly to buffer
	enc := encoder.NewStreamEncoder(buf)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(v); err != nil {
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
	data, err := sonic.MarshalIndent(v, prefix, indent)
	if err != nil {
		return nil, err
	}

	buf := pool.GetByteBuffer()
	_, _ = buf.Write(data)

	return buf, nil
}

// Name 返回当前使用的JSON序列化器名称 | Return current JSON serializer name
func Name() string {
	return "github.com/bytedance/sonic"
}

// Preload 预热JSON序列化器 | Preload JSON serializer
// sonic支持预热以提升性能 | sonic supports preloading for better performance
func Preload(types ...any) {
	for _, t := range types {
		// sonic.Pretouch需要reflect.Type，而不是any
		// sonic.Pretouch requires reflect.Type, not any
		var rt reflect.Type
		if typ, ok := t.(reflect.Type); ok {
			rt = typ
		} else {
			rt = reflect.TypeOf(t)
		}

		if rt != nil {
			_ = sonic.Pretouch(rt, option.WithCompileRecursiveDepth(10))
		}
	}
}

// Decoder JSON解码器（sonic流式解码器）| JSON decoder (sonic stream decoder)
type Decoder = decoder.StreamDecoder

// Encoder JSON编码器（sonic流式编码器）| JSON encoder (sonic stream encoder)
type Encoder = encoder.StreamEncoder

// NewDecoder 创建JSON解码器 | Create JSON decoder
// 使用sonic流式解码器 | Using sonic stream decoder
func NewDecoder(r io.Reader) *Decoder {
	return decoder.NewStreamDecoder(r)
}

// NewEncoder 创建JSON编码器 | Create JSON encoder
// 默认启用HTML转义，与标准库行为一致 | Default HTML escaping enabled, consistent with stdlib behavior
func NewEncoder(w io.Writer) *Encoder {
	enc := encoder.NewStreamEncoder(w)
	// sonic默认不转义HTML，这里设置为true以与标准库行为一致
	// sonic defaults to NOT escaping HTML, set to true for stdlib compatibility
	enc.SetEscapeHTML(true)
	return enc
}

// RawMessage 原始JSON消息类型，用于延迟JSON解码或预计算JSON编码
// Raw JSON message type for delayed JSON decoding or precomputed JSON encoding
// 使用encoding/json类型别名确保100%类型兼容性 | Using encoding/json type alias for 100% type compatibility
type RawMessage = stdjson.RawMessage

// Number JSON数字字面量类型 | JSON number literal type
// 使用encoding/json类型别名确保100%类型兼容性 | Using encoding/json type alias for 100% type compatibility
type Number = stdjson.Number

// Token JSON令牌类型，用于流式解码 | JSON token type for streaming decoding
// 使用encoding/json类型别名确保100%类型兼容性 | Using encoding/json type alias for 100% type compatibility
type Token = stdjson.Token

// Delim JSON数组或对象分隔符 | JSON array or object delimiter
// 使用encoding/json类型别名确保100%类型兼容性 | Using encoding/json type alias for 100% type compatibility
type Delim = stdjson.Delim

// Marshaler JSON序列化接口 | JSON marshaler interface
// 使用encoding/json接口别名确保100%类型兼容性 | Using encoding/json interface alias for 100% type compatibility
type Marshaler = stdjson.Marshaler

// Unmarshaler JSON反序列化接口 | JSON unmarshaler interface
// 使用encoding/json接口别名确保100%类型兼容性 | Using encoding/json interface alias for 100% type compatibility
type Unmarshaler = stdjson.Unmarshaler

// InvalidUnmarshalError 无效的Unmarshal目标错误 | Invalid Unmarshal target error
// 使用encoding/json类型别名确保100%兼容性 | Using encoding/json type alias for 100% compatibility
type InvalidUnmarshalError = stdjson.InvalidUnmarshalError

// UnmarshalTypeError 类型不匹配错误 | Type mismatch error
// 使用encoding/json类型别名确保100%兼容性 | Using encoding/json type alias for 100% compatibility
type UnmarshalTypeError = stdjson.UnmarshalTypeError

// SyntaxError JSON语法错误 | JSON syntax error
// 使用encoding/json类型别名确保100%兼容性 | Using encoding/json type alias for 100% compatibility
type SyntaxError = stdjson.SyntaxError

// MarshalerError Marshaler接口返回的错误 | Error returned by Marshaler interface
// 使用encoding/json类型别名确保100%兼容性 | Using encoding/json type alias for 100% compatibility
type MarshalerError = stdjson.MarshalerError

// UnsupportedTypeError 不支持的类型错误 | Unsupported type error
// 使用encoding/json类型别名确保100%兼容性 | Using encoding/json type alias for 100% compatibility
type UnsupportedTypeError = stdjson.UnsupportedTypeError

// UnsupportedValueError 不支持的值错误 | Unsupported value error
// 使用encoding/json类型别名确保100%兼容性 | Using encoding/json type alias for 100% compatibility
type UnsupportedValueError = stdjson.UnsupportedValueError

// Valid 检查数据是否为有效的JSON编码 | Check if data is valid JSON encoding
// 使用sonic原生高性能实现 | Using sonic native high-performance implementation
func Valid(data []byte) bool {
	return sonic.Valid(data)
}

// Compact 将JSON编码的src追加到dst，去除无意义的空白字符
// Append JSON-encoded src to dst with insignificant space characters elided
// 使用sonic Decoder(UseNumber) + Marshal实现，避免大整数精度丢失
// Using sonic Decoder(UseNumber) + Marshal to avoid large integer precision loss
func Compact(dst *bytes.Buffer, src []byte) error {
	// 使用sonic StreamDecoder + UseNumber保留数字精度
	// Use sonic StreamDecoder + UseNumber to preserve number precision
	dec := decoder.NewStreamDecoder(bytes.NewReader(src))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return err
	}
	// 使用sonic排序键API重新序列化（默认紧凑格式）
	// Re-marshal using sonic sorted keys API (default compact format)
	compacted, err := sonicSortedAPI.Marshal(v)
	if err != nil {
		return err
	}
	_, err = dst.Write(compacted)
	return err
}

// HTMLEscape 将JSON编码的src追加到dst，对HTML特殊字符进行转义
// Append JSON-encoded src to dst with HTML special characters escaped
// 使用sonic的encoder.HTMLEscape字节流操作，高性能且精确
// Using sonic encoder.HTMLEscape byte-stream operation, high-performance and accurate
func HTMLEscape(dst *bytes.Buffer, src []byte) {
	// sonic的encoder.HTMLEscape签名: func(dst []byte, src []byte) []byte
	// 适配为标准库签名: func(dst *bytes.Buffer, src []byte)
	// sonic encoder.HTMLEscape signature: func(dst []byte, src []byte) []byte
	// Adapt to stdlib signature: func(dst *bytes.Buffer, src []byte)
	result := encoder.HTMLEscape(nil, src)
	_, _ = dst.Write(result)
}

// Indent 将JSON编码的src格式化后追加到dst
// Append indented form of JSON-encoded src to dst
// 使用sonic Decoder(UseNumber) + MarshalIndent实现，避免大整数精度丢失
// Using sonic Decoder(UseNumber) + MarshalIndent to avoid large integer precision loss
func Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	// 使用sonic StreamDecoder + UseNumber保留数字精度
	// Use sonic StreamDecoder + UseNumber to preserve number precision
	dec := decoder.NewStreamDecoder(bytes.NewReader(src))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return err
	}
	// 使用sonic排序键API重新格式化序列化
	// Re-marshal with indentation using sonic sorted keys API
	indented, err := sonicSortedAPI.MarshalIndent(v, prefix, indent)
	if err != nil {
		return err
	}
	_, err = dst.Write(indented)
	return err
}
