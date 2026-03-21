// Package cbor 提供CBOR序列化功能
// Package cbor provides CBOR serialization functionality
//
// 本包提供了CBOR序列化功能，使用github.com/fxamacker/cbor/v2实现
// This package provides CBOR serialization functionality using github.com/fxamacker/cbor/v2
//
// 使用示例 | Usage Example:
//
//	import "go-port-forward/pkg/serializer/cbor"
//
//	// 基本使用 | Basic usage
//	data, err := cbor.Marshal(obj)
//	err = cbor.Unmarshal(data, &obj)
//
//	// 使用对象池优化 | Use object pool optimization
//	buf, err := cbor.MarshalToBuffer(obj)
//	if err == nil {
//		defer pool.PutByteBuffer(buf)
//		// 使用buf.Bytes() | Use buf.Bytes()
//	}
package cbor

import (
	"github.com/fxamacker/cbor/v2"
	"github.com/valyala/bytebufferpool"

	"go-port-forward/pkg/pool"
)

var (
	// encMode CBOR编码模式 | CBOR encoding mode
	encMode cbor.EncMode
	// decMode CBOR解码模式 | CBOR decoding mode
	decMode cbor.DecMode
)

func init() {
	// 配置CBOR编码选项 | Configure CBOR encoding options
	encOpts := cbor.EncOptions{
		Sort:          cbor.SortCanonical,
		Time:          cbor.TimeRFC3339,
		TimeTag:       cbor.EncTagRequired,
		IndefLength:   cbor.IndefLengthForbidden,
		NilContainers: cbor.NilContainerAsNull,
	}

	var err error
	encMode, err = encOpts.EncMode()
	if err != nil {
		panic(err)
	}

	// 配置CBOR解码选项 | Configure CBOR decoding options
	decOpts := cbor.DecOptions{
		TimeTag:             cbor.DecTagRequired,
		DupMapKey:           cbor.DupMapKeyEnforcedAPF,
		IndefLength:         cbor.IndefLengthForbidden,
		IntDec:              cbor.IntDecConvertNone,
		MaxArrayElements:    131072,
		MaxMapPairs:         131072,
		MaxNestedLevels:     32,
		UTF8:                cbor.UTF8RejectInvalid,
		FieldNameByteString: cbor.FieldNameByteStringAllowed,
	}

	decMode, err = decOpts.DecMode()
	if err != nil {
		panic(err)
	}
}

// Marshal CBOR序列化 | CBOR marshal
func Marshal(v any) ([]byte, error) {
	return encMode.Marshal(v)
}

// Unmarshal CBOR反序列化 | CBOR unmarshal
func Unmarshal(data []byte, v any) error {
	return decMode.Unmarshal(data, v)
}

// MarshalToBuffer 使用字节池优化的CBOR序列化 | CBOR marshal with byte pool optimization
// 返回的ByteBuffer使用完后必须调用pool.PutByteBuffer归还 | Must call pool.PutByteBuffer after use
func MarshalToBuffer(v any) (*bytebufferpool.ByteBuffer, error) {
	buf := pool.GetByteBuffer()

	encoder := encMode.NewEncoder(buf)
	if err := encoder.Encode(v); err != nil {
		pool.PutByteBuffer(buf)
		return nil, err
	}

	return buf, nil
}

// Name 返回当前使用的CBOR序列化器名称 | Return current CBOR serializer name
func Name() string {
	return "github.com/fxamacker/cbor/v2"
}
