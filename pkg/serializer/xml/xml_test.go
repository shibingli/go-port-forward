package xml

import (
	"testing"

	"go-port-forward/pkg/pool"
)

// TestData 测试数据结构 | Test data structure
type TestData struct {
	Name   string   `xml:"name"`
	Tags   []string `xml:"tags>tag"`
	ID     int      `xml:"id"`
	Active bool     `xml:"active"`
}

// TestMarshalUnmarshal 测试基本序列化和反序列化 | Test basic marshal and unmarshal
func TestMarshalUnmarshal(t *testing.T) {
	original := TestData{
		ID:     1,
		Name:   "测试",
		Active: true,
		Tags:   []string{"tag1", "tag2"},
	}

	// 序列化 | Marshal
	data, err := Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Marshal returned empty data")
	}

	// 反序列化 | Unmarshal
	var decoded TestData
	err = Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// 验证 | Verify
	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: got %d, want %d", decoded.ID, original.ID)
	}
	if decoded.Name != original.Name {
		t.Errorf("Name mismatch: got %s, want %s", decoded.Name, original.Name)
	}
}

// TestMarshalIndent 测试格式化序列化 | Test marshal with indentation
func TestMarshalIndent(t *testing.T) {
	original := TestData{
		ID:     2,
		Name:   "格式化测试",
		Active: false,
	}

	data, err := MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("MarshalIndent returned empty data")
	}

	// 验证可以反序列化 | Verify can unmarshal
	var decoded TestData
	err = Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: got %d, want %d", decoded.ID, original.ID)
	}
}

// TestMarshalToBuffer 测试使用对象池的序列化 | Test marshal with object pool
func TestMarshalToBuffer(t *testing.T) {
	original := TestData{
		ID:     3,
		Name:   "对象池测试",
		Active: true,
	}

	buf, err := MarshalToBuffer(original)
	if err != nil {
		t.Fatalf("MarshalToBuffer failed: %v", err)
	}
	defer pool.PutByteBuffer(buf)

	if buf.Len() == 0 {
		t.Fatal("MarshalToBuffer returned empty buffer")
	}

	// 验证可以反序列化 | Verify can unmarshal
	var decoded TestData
	err = Unmarshal(buf.Bytes(), &decoded)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: got %d, want %d", decoded.ID, original.ID)
	}
}

// TestMarshalIndentToBuffer 测试使用对象池的格式化序列化 | Test marshal indent with object pool
func TestMarshalIndentToBuffer(t *testing.T) {
	original := TestData{
		ID:     4,
		Name:   "对象池格式化测试",
		Active: false,
	}

	buf, err := MarshalIndentToBuffer(original, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndentToBuffer failed: %v", err)
	}
	defer pool.PutByteBuffer(buf)

	if buf.Len() == 0 {
		t.Fatal("MarshalIndentToBuffer returned empty buffer")
	}

	// 验证可以反序列化 | Verify can unmarshal
	var decoded TestData
	err = Unmarshal(buf.Bytes(), &decoded)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: got %d, want %d", decoded.ID, original.ID)
	}
}

// TestName 测试获取序列化器名称 | Test get serializer name
func TestName(t *testing.T) {
	name := Name()
	if name == "" {
		t.Error("Name returned empty string")
	}
	t.Logf("XML serializer: %s", name)
}
