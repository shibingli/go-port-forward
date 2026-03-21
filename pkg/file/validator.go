// Package file 提供文件验证功能
// Package file provides file validation functionality
package file

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gabriel-vasile/mimetype"
)

// Validator 文件验证器 | File validator
type Validator struct {
	// allowedMimeTypes 允许的MIME类型集合 | Allowed MIME types set
	allowedMimeTypes map[string]bool
	// allowedExtensions 允许的扩展名集合 | Allowed extensions set
	allowedExtensions map[string]bool
	// maxFileSize 最大文件大小(字节) | Max file size (bytes)
	maxFileSize int64
}

// ValidatorConfig 验证器配置 | Validator configuration
type ValidatorConfig struct {
	// AllowedMimeTypes 允许的MIME类型列表 | Allowed MIME types list
	AllowedMimeTypes []string
	// AllowedExtensions 允许的扩展名列表 | Allowed extensions list
	AllowedExtensions []string
	// MaxFileSize 最大文件大小(字节) | Max file size (bytes)
	MaxFileSize int64
}

// NewValidator 创建文件验证器 | Create file validator
func NewValidator(config *ValidatorConfig) *Validator {
	v := &Validator{
		allowedMimeTypes:  make(map[string]bool),
		allowedExtensions: make(map[string]bool),
		maxFileSize:       config.MaxFileSize,
	}

	// 初始化MIME类型集合 | Initialize MIME types set
	for _, mimeType := range config.AllowedMimeTypes {
		v.allowedMimeTypes[strings.ToLower(strings.TrimSpace(mimeType))] = true
	}

	// 初始化扩展名集合 | Initialize extensions set
	for _, ext := range config.AllowedExtensions {
		ext = strings.ToLower(strings.TrimSpace(ext))
		// 确保扩展名以点开头 | Ensure extension starts with dot
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		v.allowedExtensions[ext] = true
	}

	return v
}

// ValidateFile 验证文件 | Validate file
// 同时检查MIME类型和扩展名 | Check both MIME type and extension
func (v *Validator) ValidateFile(reader io.Reader, filename string) error {
	// 读取文件头用于MIME类型检测 | Read file header for MIME type detection
	// 最多读取512字节用于检测 | Read up to 512 bytes for detection
	buf := make([]byte, 512)
	n, err := reader.Read(buf)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read file: %w", err)
	}
	buf = buf[:n]

	// 检测MIME类型 | Detect MIME type
	mtype := mimetype.Detect(buf)
	detectedMime := strings.ToLower(mtype.String())

	// 获取文件扩展名 | Get file extension
	ext := strings.ToLower(filepath.Ext(filename))

	// 验证MIME类型(如果配置了) | Validate MIME type (if configured)
	mimeValid := len(v.allowedMimeTypes) == 0 || v.allowedMimeTypes[detectedMime]

	// 验证扩展名(如果配置了) | Validate extension (if configured)
	extValid := len(v.allowedExtensions) == 0 || v.allowedExtensions[ext]

	// 两者都配置时,需要同时满足 | When both configured, both must be satisfied
	if len(v.allowedMimeTypes) > 0 && len(v.allowedExtensions) > 0 {
		if !mimeValid || !extValid {
			return errors.New("file type not allowed: mime=" + detectedMime + ", ext=" + ext)
		}
	} else if len(v.allowedMimeTypes) > 0 {
		// 只配置了MIME类型 | Only MIME type configured
		if !mimeValid {
			return errors.New("MIME type not allowed: " + detectedMime)
		}
	} else if len(v.allowedExtensions) > 0 {
		// 只配置了扩展名 | Only extension configured
		if !extValid {
			return errors.New("file extension not allowed: " + ext)
		}
	}

	return nil
}

// ValidateFileSize 验证文件大小 | Validate file size
func (v *Validator) ValidateFileSize(size int64) error {
	if v.maxFileSize > 0 && size > v.maxFileSize {
		return errors.New("file too large: " + strconv.FormatInt(size, 10) + " bytes (max: " + strconv.FormatInt(v.maxFileSize, 10) + " bytes)")
	}
	return nil
}

// IsAllowedMimeType 检查MIME类型是否允许 | Check if MIME type is allowed
func (v *Validator) IsAllowedMimeType(mimeType string) bool {
	if len(v.allowedMimeTypes) == 0 {
		return true
	}
	return v.allowedMimeTypes[strings.ToLower(strings.TrimSpace(mimeType))]
}

// IsAllowedExtension 检查扩展名是否允许 | Check if extension is allowed
func (v *Validator) IsAllowedExtension(ext string) bool {
	if len(v.allowedExtensions) == 0 {
		return true
	}
	ext = strings.ToLower(strings.TrimSpace(ext))
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return v.allowedExtensions[ext]
}

// GetAllowedMimeTypes 获取允许的MIME类型列表 | Get allowed MIME types list
func (v *Validator) GetAllowedMimeTypes() []string {
	types := make([]string, 0, len(v.allowedMimeTypes))
	for mimeType := range v.allowedMimeTypes {
		types = append(types, mimeType)
	}
	return types
}

// GetAllowedExtensions 获取允许的扩展名列表 | Get allowed extensions list
func (v *Validator) GetAllowedExtensions() []string {
	exts := make([]string, 0, len(v.allowedExtensions))
	for ext := range v.allowedExtensions {
		exts = append(exts, ext)
	}
	return exts
}
