// Package base64 流式编解码测试 | Streaming encoding/decoding tests
package base64

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// TestStreamingEncoder 测试流式编码器 | Test streaming encoder
func TestStreamingEncoder(t *testing.T) {
	tests := []struct {
		encoding Encoding
		name     string
		input    string
	}{
		{
			name:     "标准编码 | Standard encoding",
			input:    "Hello, World!",
			encoding: StdEncoding,
		},
		{
			name:     "URL编码 | URL encoding",
			input:    "Hello, World!",
			encoding: URLEncoding,
		},
		{
			name:     "长文本 | Long text",
			input:    strings.Repeat("The quick brown fox jumps over the lazy dog. ", 100),
			encoding: StdEncoding,
		},
		{
			name:     "空字符串 | Empty string",
			input:    "",
			encoding: StdEncoding,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			// 创建编码器 | Create encoder
			encoder, err := NewEncoder(tt.encoding, &buf)
			if err != nil {
				t.Fatalf("NewEncoder() error = %v", err)
			}

			// 写入数据 | Write data
			n, err := encoder.Write([]byte(tt.input))
			if err != nil {
				t.Fatalf("Write() error = %v", err)
			}
			if n != len(tt.input) {
				t.Errorf("Write() wrote %d bytes, want %d", n, len(tt.input))
			}

			// 关闭编码器 | Close encoder
			if err := encoder.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}

			// 验证结果 | Verify result
			encoded := buf.String()
			expected := tt.encoding.EncodeToString([]byte(tt.input))
			if encoded != expected {
				t.Errorf("Encoded = %q, want %q", encoded, expected)
			}
		})
	}
}

// TestStreamingDecoder 测试流式解码器 | Test streaming decoder
func TestStreamingDecoder(t *testing.T) {
	tests := []struct {
		encoding Encoding
		name     string
		input    string
	}{
		{
			name:     "标准编码 | Standard encoding",
			input:    "Hello, World!",
			encoding: StdEncoding,
		},
		{
			name:     "URL编码 | URL encoding",
			input:    "Hello, World!",
			encoding: URLEncoding,
		},
		{
			name:     "长文本 | Long text",
			input:    strings.Repeat("The quick brown fox jumps over the lazy dog. ", 100),
			encoding: StdEncoding,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 先编码 | Encode first
			encoded := tt.encoding.EncodeToString([]byte(tt.input))

			// 创建解码器 | Create decoder
			decoder, err := NewDecoder(tt.encoding, strings.NewReader(encoded))
			if err != nil {
				t.Fatalf("NewDecoder() error = %v", err)
			}

			// 读取解码数据 | Read decoded data
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, decoder); err != nil {
				t.Fatalf("io.Copy() error = %v", err)
			}

			// 验证结果 | Verify result
			decoded := buf.String()
			if decoded != tt.input {
				t.Errorf("Decoded = %q, want %q", decoded, tt.input)
			}
		})
	}
}

// TestStreamingEncoderMultipleWrites 测试多次写入 | Test multiple writes
func TestStreamingEncoderMultipleWrites(t *testing.T) {
	var buf bytes.Buffer

	encoder, err := NewEncoder(StdEncoding, &buf)
	if err != nil {
		t.Fatalf("NewEncoder() error = %v", err)
	}

	// 多次写入 | Multiple writes
	parts := []string{"Hello", ", ", "World", "!"}
	for _, part := range parts {
		if _, err := encoder.Write([]byte(part)); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}

	if err := encoder.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// 验证结果 | Verify result
	expected := StdEncoding.EncodeToString([]byte("Hello, World!"))
	if buf.String() != expected {
		t.Errorf("Encoded = %q, want %q", buf.String(), expected)
	}
}
