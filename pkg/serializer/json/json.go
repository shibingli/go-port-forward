// Package json 提供JSON序列化功能
// Package json provides JSON serialization functionality
//
// 本包提供了与标准库encoding/json完全兼容的API，同时支持通过build tags切换不同的实现：
// This package provides APIs fully compatible with encoding/json, and supports switching implementations via build tags:
//
// - 默认（无tags）: encoding/json (标准库)
// - Default (no tags): encoding/json (standard library)
//
// - -tags sonic: github.com/bytedance/sonic (推荐，性能最佳)
// - -tags sonic: github.com/bytedance/sonic (recommended, best performance)
//
// - -tags jsoniter: github.com/json-iterator/go
// - -tags jsoniter: github.com/json-iterator/go
//
// - -tags go_json: github.com/goccy/go-json
// - -tags go_json: github.com/goccy/go-json
//
// 使用示例 | Usage Example:
//
//	import "go-port-forward/pkg/serializer/json"
//
//	// 基本使用 | Basic usage
//	data, err := json.Marshal(obj)
//	err = json.Unmarshal(data, &obj)
//
//	// 格式化输出 | Formatted output
//	data, err := json.MarshalIndent(obj, "", "  ")
//
//	// 使用对象池优化 | Use object pool optimization
//	buf, err := json.MarshalToBuffer(obj)
//	if err == nil {
//		defer pool.PutByteBuffer(buf)
//		// 使用buf.Bytes() | Use buf.Bytes()
//	}
//
//	// 流式编码/解码 | Stream encoding/decoding
//	encoder := json.NewEncoder(writer)
//	err = encoder.Encode(obj)
//
//	decoder := json.NewDecoder(reader)
//	err = decoder.Decode(&obj)
//
//	// 预热（仅sonic支持）| Preload (sonic only)
//	json.Preload(&MyStruct{})
//
//	// JSON验证 | JSON validation
//	if json.Valid(data) {
//		// data是有效的JSON | data is valid JSON
//	}
//
//	// 延迟解码 | Delayed decoding
//	var raw json.RawMessage
//	err = json.Unmarshal(data, &raw)
//
//	// JSON格式化工具 | JSON formatting utilities
//	var buf bytes.Buffer
//	json.Compact(&buf, data)
//	json.Indent(&buf, data, "", "  ")
//	json.HTMLEscape(&buf, data)
//
// 导出的类型 | Exported Types:
//
//   - RawMessage: 原始JSON消息，用于延迟解码 | Raw JSON message for delayed decoding
//   - Number: JSON数字字面量 | JSON number literal
//   - Token: JSON令牌，用于流式解码 | JSON token for streaming decoding
//   - Delim: JSON分隔符 | JSON delimiter
//   - Marshaler: JSON序列化接口 | JSON marshaler interface
//   - Unmarshaler: JSON反序列化接口 | JSON unmarshaler interface
//   - Decoder: JSON解码器 | JSON decoder
//   - Encoder: JSON编码器 | JSON encoder
//
// 导出的函数 | Exported Functions:
//
//   - Marshal/MarshalIndent: JSON序列化 | JSON serialization
//   - Unmarshal: JSON反序列化 | JSON deserialization
//   - MarshalToBuffer/MarshalIndentToBuffer: 使用对象池的序列化 | Serialization with object pool
//   - Valid: JSON格式验证 | JSON format validation
//   - Compact: 压缩JSON | Compact JSON
//   - HTMLEscape: HTML转义JSON | HTML-escape JSON
//   - Indent: 格式化JSON | Format JSON
//   - NewDecoder/NewEncoder: 创建流式编解码器 | Create streaming codec
//   - Name: 获取当前实现名称 | Get current implementation name
//   - Preload: 预热序列化器 | Preload serializer
//
// 性能优化 | Performance Optimization:
//
// 1. 使用sonic实现可获得2-3倍性能提升
// 1. Using sonic implementation can get 2-3x performance improvement
//
// 2. 使用MarshalToBuffer可减少内存分配
// 2. Using MarshalToBuffer can reduce memory allocation
//
// 3. 对于sonic，使用Preload预热可进一步提升性能
// 3. For sonic, using Preload can further improve performance
package json
