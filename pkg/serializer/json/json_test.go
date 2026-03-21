package json

import (
	"bytes"
	"reflect"
	"testing"

	"go-port-forward/pkg/pool"
)

// TestData 测试数据结构 | Test data structure
type TestData struct {
	Metadata map[string]string `json:"metadata"`
	Name     string            `json:"name"`
	Tags     []string          `json:"tags"`
	ID       int               `json:"id"`
	Active   bool              `json:"active"`
}

// TestMarshalUnmarshal 测试基本序列化和反序列化 | Test basic marshal and unmarshal
func TestMarshalUnmarshal(t *testing.T) {
	original := TestData{
		ID:     1,
		Name:   "测试",
		Active: true,
		Tags:   []string{"tag1", "tag2"},
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
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
	if decoded.Active != original.Active {
		t.Errorf("Active mismatch: got %v, want %v", decoded.Active, original.Active)
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
	t.Logf("JSON serializer: %s", name)
}

// TestPreload 测试预热功能 | Test preload functionality
func TestPreload(t *testing.T) {
	// 预热不应该panic | Preload should not panic
	Preload(&TestData{})
	t.Log("Preload completed successfully")
}

// TestValid 测试JSON格式验证 | Test JSON format validation
func TestValid(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  bool
	}{
		{
			name:  "valid object | 有效的对象",
			input: []byte(`{"name":"test","id":1}`),
			want:  true,
		},
		{
			name:  "valid array | 有效的数组",
			input: []byte(`[1,2,3]`),
			want:  true,
		},
		{
			name:  "valid string | 有效的字符串",
			input: []byte(`"hello"`),
			want:  true,
		},
		{
			name:  "valid number | 有效的数字",
			input: []byte(`42`),
			want:  true,
		},
		{
			name:  "valid null | 有效的null",
			input: []byte(`null`),
			want:  true,
		},
		{
			name:  "valid boolean | 有效的布尔值",
			input: []byte(`true`),
			want:  true,
		},
		{
			name:  "invalid JSON | 无效的JSON",
			input: []byte(`{invalid}`),
			want:  false,
		},
		{
			name:  "empty input | 空输入",
			input: []byte(``),
			want:  false,
		},
		{
			name:  "incomplete object | 不完整的对象",
			input: []byte(`{"name":`),
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Valid(tt.input)
			if got != tt.want {
				t.Errorf("Valid(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestRawMessage 测试RawMessage类型 | Test RawMessage type
func TestRawMessage(t *testing.T) {
	// 测试RawMessage序列化 | Test RawMessage marshal
	type Wrapper struct {
		Data RawMessage `json:"data"`
		Name string     `json:"name"`
	}

	raw := RawMessage(`{"nested":"value","count":42}`)
	wrapper := Wrapper{
		Name: "test",
		Data: raw,
	}

	data, err := Marshal(wrapper)
	if err != nil {
		t.Fatalf("Marshal with RawMessage failed: %v", err)
	}

	// 验证RawMessage内容被原样保留 | Verify RawMessage content is preserved as-is
	var decoded Wrapper
	if err = Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal with RawMessage failed: %v", err)
	}

	if decoded.Name != "test" {
		t.Errorf("Name mismatch: got %s, want test", decoded.Name)
	}

	// 验证RawMessage可以进一步解码 | Verify RawMessage can be further decoded
	var nested map[string]any
	if err = Unmarshal(decoded.Data, &nested); err != nil {
		t.Fatalf("Unmarshal nested RawMessage failed: %v", err)
	}

	if nested["nested"] != "value" {
		t.Errorf("Nested value mismatch: got %v, want value", nested["nested"])
	}
}

// TestCompact 测试JSON压缩 | Test JSON compact
func TestCompact(t *testing.T) {
	input := []byte(`{
		"name": "test",
		"id": 1,
		"tags": ["a", "b"]
	}`)

	var buf bytes.Buffer
	if err := Compact(&buf, input); err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	// 验证Compact结果是有效的紧凑JSON且语义等价（不依赖键序，兼容sonic排序键行为）
	// Verify Compact result is valid compact JSON and semantically equivalent (key-order independent, compatible with sonic sorted keys)
	result := buf.Bytes()
	var gotMap, wantMap map[string]any
	if err := Unmarshal(result, &gotMap); err != nil {
		t.Fatalf("Compact result is not valid JSON: %v", err)
	}
	if err := Unmarshal(input, &wantMap); err != nil {
		t.Fatalf("Failed to unmarshal input: %v", err)
	}
	// 验证无多余空白（紧凑格式）| Verify no extra whitespace (compact format)
	var rebuf bytes.Buffer
	if err := Compact(&rebuf, result); err != nil {
		t.Fatalf("Re-compact failed: %v", err)
	}
	if rebuf.String() != string(result) {
		t.Errorf("Compact result contains extra whitespace: got %s", result)
	}
	// 语义等价比较（使用DeepEqual避免map序列化顺序不确定性）
	// Semantic equivalence comparison (using DeepEqual to avoid map serialization order non-determinism)
	if !reflect.DeepEqual(gotMap, wantMap) {
		t.Errorf("Compact result semantically different: got %s, want equivalent of %s", result, input)
	}

	// 测试无效JSON | Test invalid JSON
	buf.Reset()
	if err := Compact(&buf, []byte(`{invalid}`)); err == nil {
		t.Error("Compact should fail for invalid JSON")
	}
}

// TestHTMLEscape 测试HTML转义 | Test HTML escape
func TestHTMLEscape(t *testing.T) {
	input := []byte(`{"html":"<script>alert('xss')</script>"}`)

	var buf bytes.Buffer
	HTMLEscape(&buf, input)

	result := buf.String()
	// 验证HTML特殊字符被转义 | Verify HTML special characters are escaped
	if bytes.Contains(buf.Bytes(), []byte("<script>")) {
		t.Error("HTMLEscape should escape <script> tags")
	}
	if len(result) == 0 {
		t.Error("HTMLEscape returned empty result")
	}
}

// TestIndent 测试JSON格式化 | Test JSON indent
func TestIndent(t *testing.T) {
	input := []byte(`{"name":"test","id":1,"tags":["a","b"]}`)

	var buf bytes.Buffer
	if err := Indent(&buf, input, "", "  "); err != nil {
		t.Fatalf("Indent failed: %v", err)
	}

	result := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("\n")) {
		t.Error("Indent result should contain newlines")
	}
	if len(result) <= len(string(input)) {
		t.Error("Indent result should be longer than compact input")
	}

	// 验证格式化后仍然是有效JSON | Verify indented result is still valid JSON
	if !Valid(buf.Bytes()) {
		t.Error("Indent result should be valid JSON")
	}

	// 测试无效JSON | Test invalid JSON
	buf.Reset()
	if err := Indent(&buf, []byte(`{invalid}`), "", "  "); err == nil {
		t.Error("Indent should fail for invalid JSON")
	}
}

// TestNumber 测试Number类型 | Test Number type
func TestNumber(t *testing.T) {
	// 测试Number可以作为JSON值使用 | Test Number can be used as JSON value
	type NumberWrapper struct {
		Value Number `json:"value"`
	}

	// 序列化 | Marshal
	wrapper := NumberWrapper{Value: Number("123.456")}
	data, err := Marshal(wrapper)
	if err != nil {
		t.Fatalf("Marshal Number failed: %v", err)
	}

	// 反序列化 | Unmarshal
	var decoded NumberWrapper
	if err = Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal Number failed: %v", err)
	}

	if decoded.Value.String() != "123.456" {
		t.Errorf("Number value mismatch: got %s, want 123.456", decoded.Value.String())
	}

	// 测试Number转换 | Test Number conversion
	f, err := decoded.Value.Float64()
	if err != nil {
		t.Fatalf("Number.Float64 failed: %v", err)
	}
	if f != 123.456 {
		t.Errorf("Number.Float64 mismatch: got %f, want 123.456", f)
	}
}

// TestMarshalerInterface 测试Marshaler接口类型 | Test Marshaler interface type
func TestMarshalerInterface(t *testing.T) {
	// 验证Marshaler接口可以被使用 | Verify Marshaler interface can be used
	var _ Marshaler = &customMarshaler{}

	cm := &customMarshaler{value: "custom"}
	data, err := Marshal(cm)
	if err != nil {
		t.Fatalf("Marshal custom Marshaler failed: %v", err)
	}

	expected := `"custom_value"`
	if string(data) != expected {
		t.Errorf("Custom Marshaler result mismatch: got %s, want %s", string(data), expected)
	}
}

// TestUnmarshalerInterface 测试Unmarshaler接口类型 | Test Unmarshaler interface type
func TestUnmarshalerInterface(t *testing.T) {
	// 验证Unmarshaler接口可以被使用 | Verify Unmarshaler interface can be used
	var _ Unmarshaler = &customUnmarshaler{}

	cu := &customUnmarshaler{}
	if err := Unmarshal([]byte(`"test_input"`), cu); err != nil {
		t.Fatalf("Unmarshal custom Unmarshaler failed: %v", err)
	}

	if cu.value != "test_input" {
		t.Errorf("Custom Unmarshaler value mismatch: got %s, want test_input", cu.value)
	}
}

// customMarshaler 自定义Marshaler实现 | Custom Marshaler implementation
type customMarshaler struct {
	value string
}

// MarshalJSON 实现Marshaler接口 | Implement Marshaler interface
func (c *customMarshaler) MarshalJSON() ([]byte, error) {
	return []byte(`"` + c.value + `_value"`), nil
}

// customUnmarshaler 自定义Unmarshaler实现 | Custom Unmarshaler implementation
type customUnmarshaler struct {
	value string
}

// UnmarshalJSON 实现Unmarshaler接口 | Implement Unmarshaler interface
func (c *customUnmarshaler) UnmarshalJSON(data []byte) error {
	// 去掉引号 | Remove quotes
	if len(data) >= 2 && data[0] == '"' && data[len(data)-1] == '"' {
		c.value = string(data[1 : len(data)-1])
	}
	return nil
}

// TestNewDecoder 测试创建JSON解码器 | Test create JSON decoder
func TestNewDecoder(t *testing.T) {
	input := `{"name":"decoder_test","id":99}`
	reader := bytes.NewReader([]byte(input))

	dec := NewDecoder(reader)
	if dec == nil {
		t.Fatal("NewDecoder returned nil")
	}

	var decoded TestData
	if err := dec.Decode(&decoded); err != nil {
		t.Fatalf("Decoder.Decode failed: %v", err)
	}

	if decoded.Name != "decoder_test" {
		t.Errorf("Name mismatch: got %s, want decoder_test", decoded.Name)
	}
	if decoded.ID != 99 {
		t.Errorf("ID mismatch: got %d, want 99", decoded.ID)
	}
}

// TestNewEncoder 测试创建JSON编码器 | Test create JSON encoder
func TestNewEncoder(t *testing.T) {
	var buf bytes.Buffer

	enc := NewEncoder(&buf)
	if enc == nil {
		t.Fatal("NewEncoder returned nil")
	}

	original := TestData{
		ID:   88,
		Name: "encoder_test",
	}

	if err := enc.Encode(original); err != nil {
		t.Fatalf("Encoder.Encode failed: %v", err)
	}

	// 验证编码结果可以反序列化 | Verify encoded result can be unmarshaled
	var decoded TestData
	if err := Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal encoded data failed: %v", err)
	}

	if decoded.Name != "encoder_test" {
		t.Errorf("Name mismatch: got %s, want encoder_test", decoded.Name)
	}
	if decoded.ID != 88 {
		t.Errorf("ID mismatch: got %d, want 88", decoded.ID)
	}
}
