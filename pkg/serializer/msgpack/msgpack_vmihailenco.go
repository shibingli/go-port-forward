//go:build !shamaton
// +build !shamaton

// Package msgpack 提供MsgPack序列化功能 | Package msgpack provides MsgPack serialization functionality
package msgpack

import (
	"github.com/valyala/bytebufferpool"
	"github.com/vmihailenco/msgpack/v5"

	"go-port-forward/pkg/pool"
)

// Marshal MsgPack序列化 | MsgPack marshal
func Marshal(v any) ([]byte, error) {
	return msgpack.Marshal(v)
}

// Unmarshal MsgPack反序列化 | MsgPack unmarshal
func Unmarshal(data []byte, v any) error {
	return msgpack.Unmarshal(data, v)
}

// MarshalToBuffer 使用字节池优化的MsgPack序列化 | MsgPack marshal with byte pool optimization
// 返回的ByteBuffer使用完后必须调用pool.PutByteBuffer归还 | Must call pool.PutByteBuffer after use
func MarshalToBuffer(v any) (*bytebufferpool.ByteBuffer, error) {
	buf := pool.GetByteBuffer()

	encoder := msgpack.NewEncoder(buf)
	if err := encoder.Encode(v); err != nil {
		pool.PutByteBuffer(buf)
		return nil, err
	}

	return buf, nil
}

// Name 返回当前使用的MsgPack序列化器名称 | Return current MsgPack serializer name
func Name() string {
	return "github.com/vmihailenco/msgpack/v5"
}
