// Package serializer 提供序列化功能
// Package serializer provides serialization functionality
package serializer

import (
	"testing"
)

// TestType 测试类型 | Test type
type TestType struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
}

// TestRegisterPreloadType 测试注册预热类型 | Test register preload type
func TestRegisterPreloadType(t *testing.T) {
	// 清空已注册的类型 | Clear registered types
	ClearPreloadTypes()

	// 注册类型 | Register types
	RegisterPreloadType(TestType{})

	// 获取已注册的类型 | Get registered types
	types := GetPreloadTypes()

	if len(types) != 1 {
		t.Errorf("Expected 1 type, got %d", len(types))
	}

	// 清理 | Cleanup
	ClearPreloadTypes()
}

// TestRegisterMultiplePreloadTypes 测试注册多个预热类型 | Test register multiple preload types
func TestRegisterMultiplePreloadTypes(t *testing.T) {
	// 清空已注册的类型 | Clear registered types
	ClearPreloadTypes()

	// 注册多个类型 | Register multiple types
	RegisterPreloadType(TestType{}, TestType{}, TestType{})

	// 获取已注册的类型 | Get registered types
	types := GetPreloadTypes()

	if len(types) != 3 {
		t.Errorf("Expected 3 types, got %d", len(types))
	}

	// 清理 | Cleanup
	ClearPreloadTypes()
}

// TestPreloadNoTypes 测试无类型时的预热 | Test preload with no types
func TestPreloadNoTypes(t *testing.T) {
	// 清空已注册的类型 | Clear registered types
	ClearPreloadTypes()

	// 执行预热 | Execute preload
	Preload()

	// 不应该panic | Should not panic

	// 清理 | Cleanup
	ClearPreloadTypes()
}

// TestPreloadWithTypes 测试有类型时的预热 | Test preload with types
func TestPreloadWithTypes(t *testing.T) {
	// 清空已注册的类型 | Clear registered types
	ClearPreloadTypes()

	// 注册类型 | Register types
	RegisterPreloadType(TestType{})

	// 执行预热 | Execute preload
	Preload()

	// 不应该panic | Should not panic

	// 清理 | Cleanup
	ClearPreloadTypes()
}

// TestPreloadOnce 测试预热只执行一次 | Test preload runs only once
func TestPreloadOnce(t *testing.T) {
	// 清空已注册的类型 | Clear registered types
	ClearPreloadTypes()

	// 注册类型 | Register types
	RegisterPreloadType(TestType{})

	// 多次执行预热 | Execute preload multiple times
	Preload()
	Preload()
	Preload()

	// 不应该panic | Should not panic

	// 清理 | Cleanup
	ClearPreloadTypes()
}

// TestClearPreloadTypes 测试清空已注册的预热类型 | Test clear registered preload types
func TestClearPreloadTypes(t *testing.T) {
	// 注册类型 | Register types
	RegisterPreloadType(TestType{})

	// 获取已注册的类型 | Get registered types
	types := GetPreloadTypes()
	if len(types) != 1 {
		t.Errorf("Expected 1 type before clear, got %d", len(types))
	}

	// 清空 | Clear
	ClearPreloadTypes()

	// 再次获取 | Get again
	types = GetPreloadTypes()
	if len(types) != 0 {
		t.Errorf("Expected 0 types after clear, got %d", len(types))
	}
}

// TestRegisterPreloadTypeConcurrency 测试并发注册预热类型 | Test concurrent preload type registration
func TestRegisterPreloadTypeConcurrency(t *testing.T) {
	// 清空已注册的类型 | Clear registered types
	ClearPreloadTypes()

	// 并发注册类型 | Concurrent registration
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			RegisterPreloadType(TestType{})
			done <- true
		}()
	}

	// 等待所有goroutine完成 | Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// 获取已注册的类型 | Get registered types
	types := GetPreloadTypes()
	if len(types) != 10 {
		t.Errorf("Expected 10 types, got %d", len(types))
	}

	// 清理 | Cleanup
	ClearPreloadTypes()
}
