package serializer

import (
	"testing"

	"go-port-forward/pkg/serializer/cbor"
	"go-port-forward/pkg/serializer/json"
	"go-port-forward/pkg/serializer/msgpack"
	"go-port-forward/pkg/serializer/xml"
)

// TestData 测试数据结构 | Test data structure
type TestData struct {
	Metadata map[string]string `json:"metadata,omitempty" msgpack:"metadata,omitempty" cbor:"metadata,omitempty" xml:"-"`
	Name     string            `json:"name" msgpack:"name" cbor:"name" xml:"name"`
	Tags     []string          `json:"tags" msgpack:"tags" cbor:"tags" xml:"tags>tag"`
	ID       int               `json:"id" msgpack:"id" cbor:"id" xml:"id"`
	Active   bool              `json:"active" msgpack:"active" cbor:"active" xml:"active"`
}

// TestJSONMarshalUnmarshal 测试JSON序列化和反序列化 | Test JSON marshal and unmarshal
func TestJSONMarshalUnmarshal(t *testing.T) {
	original := TestData{
		ID:     1,
		Name:   "测试 Test",
		Active: true,
		Tags:   []string{"tag1", "tag2"},
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	// 序列化 | Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("json.Marshal returned empty data")
	}

	// 反序列化 | Unmarshal
	var decoded TestData
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// 验证 | Verify
	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: got %d, want %d", decoded.ID, original.ID)
	}
	if decoded.Name != original.Name {
		t.Errorf("Name mismatch: got %s, want %s", decoded.Name, original.Name)
	}
	if decoded.Active != original.Active {
		t.Errorf("Active mismatch: got %v, want %v", decoded.Active, original.Active)
	}
}

// TestMsgPackMarshalUnmarshal 测试MsgPack序列化和反序列化 | Test MsgPack marshal and unmarshal
func TestMsgPackMarshalUnmarshal(t *testing.T) {
	original := TestData{
		ID:     2,
		Name:   "MsgPack测试",
		Active: false,
		Tags:   []string{"msgpack", "test"},
		Metadata: map[string]string{
			"format": "msgpack",
		},
	}

	// 序列化 | Marshal
	data, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("msgpack.Marshal failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("msgpack.Marshal returned empty data")
	}

	// 反序列化 | Unmarshal
	var decoded TestData
	err = msgpack.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("MsgPackUnmarshal failed: %v", err)
	}

	// 验证 | Verify
	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: got %d, want %d", decoded.ID, original.ID)
	}
	if decoded.Name != original.Name {
		t.Errorf("Name mismatch: got %s, want %s", decoded.Name, original.Name)
	}
}

// TestCBORMarshalUnmarshal 测试CBOR序列化和反序列化 | Test CBOR marshal and unmarshal
func TestCBORMarshalUnmarshal(t *testing.T) {
	original := TestData{
		ID:     3,
		Name:   "CBOR测试",
		Active: true,
		Tags:   []string{"cbor", "binary"},
		Metadata: map[string]string{
			"encoding": "cbor",
		},
	}

	// 序列化 | Marshal
	data, err := cbor.Marshal(original)
	if err != nil {
		t.Fatalf("cbor.Marshal failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("cbor.Marshal returned empty data")
	}

	// 反序列化 | Unmarshal
	var decoded TestData
	err = cbor.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("cbor.Unmarshal failed: %v", err)
	}

	// 验证 | Verify
	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: got %d, want %d", decoded.ID, original.ID)
	}
	if decoded.Name != original.Name {
		t.Errorf("Name mismatch: got %s, want %s", decoded.Name, original.Name)
	}
}

// TestGetSerializerInfo 测试获取序列化器信息 | Test get serializer info
func TestGetSerializerInfo(t *testing.T) {
	info := GetSerializerInfo()

	if info == nil {
		t.Fatal("GetSerializerInfo returned nil")
	}

	if info["json"] == "" {
		t.Error("JSON serializer name is empty")
	}
	if info["msgpack"] == "" {
		t.Error("MsgPack serializer name is empty")
	}
	if info["cbor"] == "" {
		t.Error("CBOR serializer name is empty")
	}
	if info["xml"] == "" {
		t.Error("XML serializer name is empty")
	}

	t.Logf("JSON: %s", info["json"])
	t.Logf("MsgPack: %s", info["msgpack"])
	t.Logf("CBOR: %s", info["cbor"])
	t.Logf("XML: %s", info["xml"])
}

// TestJSONSerializerName 测试JSON序列化器名称 | Test JSON serializer name
func TestJSONSerializerName(t *testing.T) {
	name := json.Name()
	if name == "" {
		t.Error("json.Name returned empty string")
	}
	t.Logf("JSON Serializer: %s", name)
}

// TestMsgPackSerializerName 测试MsgPack序列化器名称 | Test MsgPack serializer name
func TestMsgPackSerializerName(t *testing.T) {
	name := msgpack.Name()
	if name == "" {
		t.Error("msgpack.Name returned empty string")
	}
	t.Logf("MsgPack Serializer: %s", name)
}

// TestCBORSerializerName 测试CBOR序列化器名称 | Test CBOR serializer name
func TestCBORSerializerName(t *testing.T) {
	name := cbor.Name()
	if name == "" {
		t.Error("cbor.Name returned empty string")
	}
	t.Logf("CBOR Serializer: %s", name)
}

// TestXMLSerializerName 测试XML序列化器名称 | Test XML serializer name
func TestXMLSerializerName(t *testing.T) {
	name := xml.Name()
	if name == "" {
		t.Error("xml.Name returned empty string")
	}
	t.Logf("XML Serializer: %s", name)
}

// TestXMLMarshalUnmarshal 测试XML序列化和反序列化 | Test XML marshal and unmarshal
func TestXMLMarshalUnmarshal(t *testing.T) {
	// XML不支持map，所以使用简化的数据 | XML doesn't support map, use simplified data
	original := TestData{
		ID:     4,
		Name:   "XML测试",
		Active: true,
		Tags:   []string{"xml", "test"},
	}

	// 序列化 | Marshal
	data, err := xml.Marshal(original)
	if err != nil {
		t.Fatalf("xml.Marshal failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("xml.Marshal returned empty data")
	}

	// 反序列化 | Unmarshal
	var decoded TestData
	err = xml.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("xml.Unmarshal failed: %v", err)
	}

	// 验证 | Verify
	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: got %d, want %d", decoded.ID, original.ID)
	}
	if decoded.Name != original.Name {
		t.Errorf("Name mismatch: got %s, want %s", decoded.Name, original.Name)
	}
}

// TestJSONMarshalError 测试JSON序列化错误 | Test JSON marshal error
func TestJSONMarshalError(t *testing.T) {
	// 创建一个无法序列化的类型 | Create an unserializable type
	invalidData := make(chan int)

	_, err := json.Marshal(invalidData)
	if err == nil {
		t.Error("Expected error when marshaling channel, got nil")
	}
}

// TestJSONUnmarshalError 测试JSON反序列化错误 | Test JSON unmarshal error
func TestJSONUnmarshalError(t *testing.T) {
	invalidJSON := []byte(`{"invalid json`)

	var result TestData
	err := json.Unmarshal(invalidJSON, &result)
	if err == nil {
		t.Error("Expected error when unmarshaling invalid JSON, got nil")
	}
}

// TestMsgPackMarshalError 测试MsgPack序列化错误 | Test MsgPack marshal error
func TestMsgPackMarshalError(t *testing.T) {
	// 创建一个无法序列化的类型 | Create an unserializable type
	invalidData := make(chan int)

	_, err := msgpack.Marshal(invalidData)
	if err == nil {
		t.Error("Expected error when marshaling channel, got nil")
	}
}

// TestMsgPackUnmarshalError 测试MsgPack反序列化错误 | Test MsgPack unmarshal error
func TestMsgPackUnmarshalError(t *testing.T) {
	invalidData := []byte{0xFF, 0xFF, 0xFF}

	var result TestData
	err := msgpack.Unmarshal(invalidData, &result)
	if err == nil {
		t.Error("Expected error when unmarshaling invalid MsgPack, got nil")
	}
}

// TestCBORMarshalError 测试CBOR序列化错误 | Test CBOR marshal error
func TestCBORMarshalError(t *testing.T) {
	// 创建一个无法序列化的类型 | Create an unserializable type
	invalidData := make(chan int)

	_, err := cbor.Marshal(invalidData)
	if err == nil {
		t.Error("Expected error when marshaling channel, got nil")
	}
}

// TestCBORUnmarshalError 测试CBOR反序列化错误 | Test CBOR unmarshal error
func TestCBORUnmarshalError(t *testing.T) {
	invalidData := []byte{0xFF, 0xFF, 0xFF}

	var result TestData
	err := cbor.Unmarshal(invalidData, &result)
	if err == nil {
		t.Error("Expected error when unmarshaling invalid CBOR, got nil")
	}
}

// TestXMLMarshalError 测试XML序列化错误 | Test XML marshal error
func TestXMLMarshalError(t *testing.T) {
	// 创建一个无法序列化的类型 | Create an unserializable type
	invalidData := make(chan int)

	_, err := xml.Marshal(invalidData)
	if err == nil {
		t.Error("Expected error when marshaling channel, got nil")
	}
}

// TestXMLUnmarshalError 测试XML反序列化错误 | Test XML unmarshal error
func TestXMLUnmarshalError(t *testing.T) {
	invalidXML := []byte(`<invalid xml`)

	var result TestData
	err := xml.Unmarshal(invalidXML, &result)
	if err == nil {
		t.Error("Expected error when unmarshaling invalid XML, got nil")
	}
}
