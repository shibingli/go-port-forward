// Package file 提供文件验证功能测试
// Package file provides file validation functionality tests
package file

import (
	"bytes"
	"strings"
	"testing"
)

// TestNewValidator 测试创建验证器 | Test creating validator
func TestNewValidator(t *testing.T) {
	tests := []struct {
		config *ValidatorConfig
		name   string
	}{
		{
			name: "with mime types and extensions",
			config: &ValidatorConfig{
				AllowedMimeTypes:  []string{"application/pdf", "image/png"},
				AllowedExtensions: []string{".pdf", ".png"},
				MaxFileSize:       1024 * 1024,
			},
		},
		{
			name: "with only mime types",
			config: &ValidatorConfig{
				AllowedMimeTypes: []string{"application/pdf"},
				MaxFileSize:      1024 * 1024,
			},
		},
		{
			name: "with only extensions",
			config: &ValidatorConfig{
				AllowedExtensions: []string{".pdf"},
				MaxFileSize:       1024 * 1024,
			},
		},
		{
			name: "empty config",
			config: &ValidatorConfig{
				MaxFileSize: 1024 * 1024,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(tt.config)
			if v == nil {
				t.Error("NewValidator() returned nil")
			}
			if v.maxFileSize != tt.config.MaxFileSize {
				t.Errorf("maxFileSize = %d, want %d", v.maxFileSize, tt.config.MaxFileSize)
			}
		})
	}
}

// TestValidator_ValidateFile 测试文件验证 | Test file validation
func TestValidator_ValidateFile(t *testing.T) {
	tests := []struct {
		config    *ValidatorConfig
		name      string
		filename  string
		content   []byte
		wantError bool
	}{
		{
			name: "valid PDF file",
			config: &ValidatorConfig{
				AllowedMimeTypes:  []string{"application/pdf"},
				AllowedExtensions: []string{".pdf"},
			},
			content:   []byte("%PDF-1.4"),
			filename:  "test.pdf",
			wantError: false,
		},
		{
			name: "invalid mime type",
			config: &ValidatorConfig{
				AllowedMimeTypes: []string{"application/pdf"},
			},
			content:   []byte("plain text"),
			filename:  "test.txt",
			wantError: true,
		},
		{
			name: "invalid extension",
			config: &ValidatorConfig{
				AllowedExtensions: []string{".pdf"},
			},
			content:   []byte("plain text"),
			filename:  "test.txt",
			wantError: true,
		},
		{
			name: "no restrictions",
			config: &ValidatorConfig{
				MaxFileSize: 1024 * 1024,
			},
			content:   []byte("any content"),
			filename:  "test.any",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(tt.config)
			reader := bytes.NewReader(tt.content)
			err := v.ValidateFile(reader, tt.filename)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateFile() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestValidator_ValidateFileSize 测试文件大小验证 | Test file size validation
func TestValidator_ValidateFileSize(t *testing.T) {
	tests := []struct {
		name      string
		maxSize   int64
		fileSize  int64
		wantError bool
	}{
		{
			name:      "size within limit",
			maxSize:   1024,
			fileSize:  512,
			wantError: false,
		},
		{
			name:      "size exceeds limit",
			maxSize:   1024,
			fileSize:  2048,
			wantError: true,
		},
		{
			name:      "no size limit",
			maxSize:   0,
			fileSize:  999999,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(&ValidatorConfig{MaxFileSize: tt.maxSize})
			err := v.ValidateFileSize(tt.fileSize)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateFileSize() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestValidator_IsAllowedMimeType 测试MIME类型检查 | Test MIME type check
func TestValidator_IsAllowedMimeType(t *testing.T) {
	v := NewValidator(&ValidatorConfig{
		AllowedMimeTypes: []string{"application/pdf", "image/png"},
	})

	tests := []struct {
		name     string
		mimeType string
		want     bool
	}{
		{
			name:     "allowed pdf",
			mimeType: "application/pdf",
			want:     true,
		},
		{
			name:     "allowed png",
			mimeType: "image/png",
			want:     true,
		},
		{
			name:     "not allowed",
			mimeType: "text/plain",
			want:     false,
		},
		{
			name:     "case insensitive",
			mimeType: "APPLICATION/PDF",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := v.IsAllowedMimeType(tt.mimeType); got != tt.want {
				t.Errorf("IsAllowedMimeType() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestValidator_IsAllowedExtension 测试扩展名检查 | Test extension check
func TestValidator_IsAllowedExtension(t *testing.T) {
	v := NewValidator(&ValidatorConfig{
		AllowedExtensions: []string{".pdf", ".png", ".jpg"},
	})

	tests := []struct {
		name string
		ext  string
		want bool
	}{
		{
			name: "allowed pdf",
			ext:  ".pdf",
			want: true,
		},
		{
			name: "allowed without dot",
			ext:  "png",
			want: true,
		},
		{
			name: "not allowed",
			ext:  ".txt",
			want: false,
		},
		{
			name: "case insensitive",
			ext:  ".PDF",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := v.IsAllowedExtension(tt.ext); got != tt.want {
				t.Errorf("IsAllowedExtension() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestValidator_GetAllowedMimeTypes 测试获取允许的MIME类型 | Test get allowed MIME types
func TestValidator_GetAllowedMimeTypes(t *testing.T) {
	v := NewValidator(&ValidatorConfig{
		AllowedMimeTypes: []string{"application/pdf", "image/png"},
	})

	types := v.GetAllowedMimeTypes()
	if len(types) != 2 {
		t.Errorf("GetAllowedMimeTypes() returned %d types, want 2", len(types))
	}

	// 检查是否包含预期的类型 | Check if contains expected types
	typeMap := make(map[string]bool)
	for _, mimeType := range types {
		typeMap[mimeType] = true
	}

	if !typeMap["application/pdf"] || !typeMap["image/png"] {
		t.Error("GetAllowedMimeTypes() missing expected types")
	}
}

// TestValidator_GetAllowedExtensions 测试获取允许的扩展名 | Test get allowed extensions
func TestValidator_GetAllowedExtensions(t *testing.T) {
	v := NewValidator(&ValidatorConfig{
		AllowedExtensions: []string{".pdf", ".png"},
	})

	exts := v.GetAllowedExtensions()
	if len(exts) != 2 {
		t.Errorf("GetAllowedExtensions() returned %d extensions, want 2", len(exts))
	}

	// 检查是否包含预期的扩展名 | Check if contains expected extensions
	extMap := make(map[string]bool)
	for _, ext := range exts {
		extMap[ext] = true
	}

	if !extMap[".pdf"] || !extMap[".png"] {
		t.Error("GetAllowedExtensions() missing expected extensions")
	}
}

// TestValidator_ValidateFile_EdgeCases 测试边界情况 | Test edge cases
func TestValidator_ValidateFile_EdgeCases(t *testing.T) {
	tests := []struct {
		config    *ValidatorConfig
		name      string
		filename  string
		content   []byte
		wantError bool
	}{
		{
			name: "empty file",
			config: &ValidatorConfig{
				AllowedMimeTypes: []string{"application/pdf"},
			},
			content:   []byte{},
			filename:  "test.pdf",
			wantError: true,
		},
		{
			name: "extension with spaces",
			config: &ValidatorConfig{
				AllowedExtensions: []string{" .pdf "},
			},
			content:   []byte("%PDF-1.4"),
			filename:  "test.pdf",
			wantError: false,
		},
		{
			name: "mime type with spaces",
			config: &ValidatorConfig{
				AllowedMimeTypes: []string{" application/pdf "},
			},
			content:   []byte("%PDF-1.4"),
			filename:  "test.pdf",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(tt.config)
			reader := strings.NewReader(string(tt.content))
			err := v.ValidateFile(reader, tt.filename)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateFile() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
