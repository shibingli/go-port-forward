//go:build jsoniter
// +build jsoniter

// Package json 提供JSON序列化功能（jsoniter实现）
// Package json provides JSON serialization functionality (jsoniter implementation)
// 类型定义使用encoding/json别名确保100%兼容性，函数实现使用jsoniter确保最大性能
// Type definitions use encoding/json aliases for 100% compatibility, function implementations use jsoniter for maximum performance
package json

import (
	"bytes"
	stdjson "encoding/json"
	"io"

	jsoniter "github.com/json-iterator/go"
	"github.com/valyala/bytebufferpool"

	"go-port-forward/pkg/pool"
)

var (
	// jsonAPI jsoniter实例（兼容标准库配置）| jsoniter instance (stdlib compatible config)
	jsonAPI = jsoniter.ConfigCompatibleWithStandardLibrary
)

// Marshal JSON序列化 | JSON marshal
// 兼容标准库encoding/json.Marshal | Compatible with encoding/json.Marshal
func Marshal(v any) ([]byte, error) {
	return jsonAPI.Marshal(v)
}

// MarshalIndent JSON格式化序列化 | JSON marshal with indentation
// 兼容标准库encoding/json.MarshalIndent | Compatible with encoding/json.MarshalIndent
func MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	return jsonAPI.MarshalIndent(v, prefix, indent)
}

// Unmarshal JSON反序列化 | JSON unmarshal
// 兼容标准库encoding/json.Unmarshal | Compatible with encoding/json.Unmarshal
func Unmarshal(data []byte, v any) error {
	return jsonAPI.Unmarshal(data, v)
}

// MarshalToBuffer 使用字节池优化的JSON序列化 | JSON marshal with byte pool optimization
// 返回的ByteBuffer使用完后必须调用pool.PutByteBuffer归还 | Must call pool.PutByteBuffer after use
func MarshalToBuffer(v any) (*bytebufferpool.ByteBuffer, error) {
	buf := pool.GetByteBuffer()

	stream := jsonAPI.BorrowStream(buf)
	defer jsonAPI.ReturnStream(stream)

	stream.WriteVal(v)
	if stream.Error != nil {
		pool.PutByteBuffer(buf)
		return nil, stream.Error
	}

	return buf, nil
}

// MarshalIndentToBuffer 使用字节池优化的JSON格式化序列化 | JSON marshal indent with byte pool optimization
// 返回的ByteBuffer使用完后必须调用pool.PutByteBuffer归还 | Must call pool.PutByteBuffer after use
func MarshalIndentToBuffer(v any, prefix, indent string) (*bytebufferpool.ByteBuffer, error) {
	data, err := jsonAPI.MarshalIndent(v, prefix, indent)
	if err != nil {
		return nil, err
	}

	buf := pool.GetByteBuffer()
	_, _ = buf.Write(data)

	return buf, nil
}

// Name 返回当前使用的JSON序列化器名称 | Return current JSON serializer name
func Name() string {
	return "github.com/json-iterator/go"
}

// Preload 预热JSON序列化器（jsoniter不需要预热）| Preload JSON serializer (not needed for jsoniter)
func Preload(types ...any) {
	// jsoniter不需要预热 | jsoniter doesn't need preloading
}

// Decoder JSON解码器 | JSON decoder
type Decoder = jsoniter.Decoder

// Encoder JSON编码器 | JSON encoder
type Encoder = jsoniter.Encoder

// NewDecoder 创建JSON解码器 | Create JSON decoder
// 兼容标准库encoding/json.NewDecoder | Compatible with encoding/json.NewDecoder
func NewDecoder(r io.Reader) *Decoder {
	return jsonAPI.NewDecoder(r)
}

// NewEncoder 创建JSON编码器 | Create JSON encoder
// 兼容标准库encoding/json.NewEncoder | Compatible with encoding/json.NewEncoder
func NewEncoder(w io.Writer) *Encoder {
	return jsonAPI.NewEncoder(w)
}

// RawMessage 原始JSON消息类型，用于延迟JSON解码或预计算JSON编码
// Raw JSON message type for delayed JSON decoding or precomputed JSON encoding
// jsoniter.RawMessage内部已是encoding/json.RawMessage的别名 | jsoniter.RawMessage is internally an alias to encoding/json.RawMessage
type RawMessage = jsoniter.RawMessage

// Number JSON数字字面量类型 | JSON number literal type
// jsoniter.Number内部已是encoding/json.Number的别名 | jsoniter.Number is internally an alias to encoding/json.Number
type Number = jsoniter.Number

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

// jsonHTMLEscapeAPI 带HTML转义的jsoniter实例，用于HTMLEscape函数
// jsoniter instance with HTML escaping enabled, used for HTMLEscape function
var jsonHTMLEscapeAPI = jsoniter.Config{
	EscapeHTML:                    true,
	SortMapKeys:                   true,
	ValidateJsonRawMessage:        true,
	ObjectFieldMustBeSimpleString: true,
}.Froze()

// Valid 检查数据是否为有效的JSON编码 | Check if data is valid JSON encoding
// 使用jsoniter原生实现 | Using jsoniter native implementation
func Valid(data []byte) bool {
	return jsoniter.Valid(data)
}

// Compact 将JSON编码的src追加到dst，去除无意义的空白字符
// Append JSON-encoded src to dst with insignificant space characters elided
// 使用jsoniter Decoder(UseNumber) + Marshal实现，避免大整数精度丢失
// Using jsoniter Decoder(UseNumber) + Marshal to avoid large integer precision loss
func Compact(dst *bytes.Buffer, src []byte) error {
	// 使用jsoniter Decoder + UseNumber保留数字精度
	// Use jsoniter Decoder + UseNumber to preserve number precision
	dec := jsonAPI.NewDecoder(bytes.NewReader(src))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return err
	}
	// 使用jsoniter重新序列化（默认紧凑格式）
	// Re-marshal using jsoniter (default compact format)
	compacted, err := jsonAPI.Marshal(v)
	if err != nil {
		return err
	}
	_, err = dst.Write(compacted)
	return err
}

// HTMLEscape 将JSON编码的src追加到dst，对HTML特殊字符进行转义
// Append JSON-encoded src to dst with HTML special characters escaped
// 使用jsoniter Decoder(UseNumber) + EscapeHTML配置实现，避免精度丢失
// Using jsoniter Decoder(UseNumber) + EscapeHTML config to avoid precision loss
func HTMLEscape(dst *bytes.Buffer, src []byte) {
	// 使用jsoniter Decoder + UseNumber保留数字精度
	// Use jsoniter Decoder + UseNumber to preserve number precision
	dec := jsonAPI.NewDecoder(bytes.NewReader(src))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		// 如果解析失败，直接写入原始数据 | If parsing fails, write raw data
		_, _ = dst.Write(src)
		return
	}
	// 使用带EscapeHTML的jsoniter实例重新序列化
	// Re-marshal using jsoniter instance with EscapeHTML
	escaped, err := jsonHTMLEscapeAPI.Marshal(v)
	if err != nil {
		_, _ = dst.Write(src)
		return
	}
	_, _ = dst.Write(escaped)
}

// Indent 将JSON编码的src格式化后追加到dst
// Append indented form of JSON-encoded src to dst
// 使用jsoniter Decoder(UseNumber) + MarshalIndent实现，避免大整数精度丢失
// Using jsoniter Decoder(UseNumber) + MarshalIndent to avoid large integer precision loss
func Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	// 使用jsoniter Decoder + UseNumber保留数字精度
	// Use jsoniter Decoder + UseNumber to preserve number precision
	dec := jsonAPI.NewDecoder(bytes.NewReader(src))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return err
	}
	// 使用jsoniter重新格式化序列化
	// Re-marshal with indentation using jsoniter
	indented, err := jsonAPI.MarshalIndent(v, prefix, indent)
	if err != nil {
		return err
	}
	_, err = dst.Write(indented)
	return err
}
