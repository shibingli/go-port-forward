// Package msgpack 提供MsgPack序列化功能
// Package msgpack provides MsgPack serialization functionality
//
// 本包提供了MsgPack序列化功能，支持通过build tags切换不同的实现：
// This package provides MsgPack serialization functionality, and supports switching implementations via build tags:
//
// - 默认（无tags）: github.com/vmihailenco/msgpack/v5
// - Default (no tags): github.com/vmihailenco/msgpack/v5
//
// - -tags shamaton: github.com/shamaton/msgpack/v3
// - -tags shamaton: github.com/shamaton/msgpack/v3
//
// 使用示例 | Usage Example:
//
//	import "go-port-forward/pkg/serializer/msgpack"
//
//	// 基本使用 | Basic usage
//	data, err := msgpack.Marshal(obj)
//	err = msgpack.Unmarshal(data, &obj)
//
//	// 使用对象池优化 | Use object pool optimization
//	buf, err := msgpack.MarshalToBuffer(obj)
//	if err == nil {
//		defer pool.PutByteBuffer(buf)
//		// 使用buf.Bytes() | Use buf.Bytes()
//	}
package msgpack
