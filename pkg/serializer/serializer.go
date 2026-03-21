// Package serializer 提供统一的序列化管理功能
// Package serializer provides unified serialization management
package serializer

import (
	"sync"

	"go-port-forward/pkg/serializer/cbor"
	"go-port-forward/pkg/serializer/json"
	"go-port-forward/pkg/serializer/msgpack"
	"go-port-forward/pkg/serializer/xml"
)

var (
	// preloadTypes 需要预热的类型列表 | Types to preload
	preloadTypes []any
	// preloadMu 预热类型列表的互斥锁 | Mutex for preload types
	preloadMu sync.Mutex
	// preloadOnce 确保预热只执行一次 | Ensure preload runs only once
	preloadOnce sync.Once
)

// GetSerializerInfo 获取所有序列化器信息 | Get all serializer information
func GetSerializerInfo() map[string]string {
	return map[string]string{
		"json":    json.Name(),
		"msgpack": msgpack.Name(),
		"cbor":    cbor.Name(),
		"xml":     xml.Name(),
	}
}

// RegisterPreloadType 注册需要预热的类型 | Register type for preloading
// 各个模块应该在init函数中调用此方法注册需要预热的类型
// Each module should call this method in init function to register types for preloading
func RegisterPreloadType(types ...any) {
	preloadMu.Lock()
	defer preloadMu.Unlock()

	preloadTypes = append(preloadTypes, types...)
}

// Preload 执行JSON预热 | Execute JSON preloading
// 此方法会根据编译标签选择合适的预热实现
// This method will choose appropriate preload implementation based on build tags
func Preload() {
	preloadOnce.Do(func() {
		preloadMu.Lock()
		types := make([]any, len(preloadTypes))
		copy(types, preloadTypes)
		preloadMu.Unlock()

		if len(types) == 0 {
			return
		}
		// 执行实际的预热操作 | Execute actual preload operation
		json.Preload(types...)
	})
}

// PreloadJSON 预热JSON序列化器 | Preload JSON serializer
// 仅对sonic有效，其他实现会忽略 | Only effective for sonic, other implementations will ignore
func PreloadJSON(types ...any) {
	json.Preload(types...)
}

// GetPreloadTypes 获取已注册的预热类型列表（用于测试）| Get registered preload types (for testing)
func GetPreloadTypes() []any {
	preloadMu.Lock()
	defer preloadMu.Unlock()

	types := make([]any, len(preloadTypes))
	copy(types, preloadTypes)
	return types
}

// ClearPreloadTypes 清空已注册的预热类型列表（用于测试）| Clear registered preload types (for testing)
func ClearPreloadTypes() {
	preloadMu.Lock()
	defer preloadMu.Unlock()

	preloadTypes = nil
	preloadOnce = sync.Once{}
}
