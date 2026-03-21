// Package file 提供文件验证功能示例
// Package file provides file validation functionality examples
package file_test

import (
	"bytes"
	"fmt"
	"log"

	"go-port-forward/pkg/file"
)

// ExampleValidator_ValidateFile 演示如何验证文件 | Example of validating a file
func ExampleValidator_ValidateFile() {
	// 创建验证器配置 | Create validator configuration
	config := &file.ValidatorConfig{
		AllowedMimeTypes:  []string{"application/pdf", "image/png"},
		AllowedExtensions: []string{".pdf", ".png"},
		MaxFileSize:       10 * 1024 * 1024, // 10MB
	}

	// 创建验证器 | Create validator
	validator := file.NewValidator(config)

	// 模拟PDF文件内容 | Simulate PDF file content
	pdfContent := []byte("%PDF-1.4")
	reader := bytes.NewReader(pdfContent)

	// 验证文件 | Validate file
	err := validator.ValidateFile(reader, "document.pdf")
	if err != nil {
		log.Printf("File validation failed: %v", err)
		return
	}

	fmt.Println("File validation passed")
	// Output: File validation passed
}

// ExampleValidator_ValidateFileSize 演示如何验证文件大小 | Example of validating file size
func ExampleValidator_ValidateFileSize() {
	// 创建验证器 | Create validator
	validator := file.NewValidator(&file.ValidatorConfig{
		MaxFileSize: 5 * 1024 * 1024, // 5MB
	})

	// 验证文件大小 | Validate file size
	fileSize := int64(3 * 1024 * 1024) // 3MB
	err := validator.ValidateFileSize(fileSize)
	if err != nil {
		log.Printf("File size validation failed: %v", err)
		return
	}

	fmt.Println("File size validation passed")
	// Output: File size validation passed
}

// ExampleValidator_IsAllowedMimeType 演示如何检查MIME类型 | Example of checking MIME type
func ExampleValidator_IsAllowedMimeType() {
	// 创建验证器 | Create validator
	validator := file.NewValidator(&file.ValidatorConfig{
		AllowedMimeTypes: []string{"application/pdf", "image/png", "image/jpeg"},
	})

	// 检查MIME类型 | Check MIME type
	if validator.IsAllowedMimeType("application/pdf") {
		fmt.Println("PDF is allowed")
	}

	if !validator.IsAllowedMimeType("text/plain") {
		fmt.Println("Plain text is not allowed")
	}

	// Output:
	// PDF is allowed
	// Plain text is not allowed
}

// ExampleValidator_IsAllowedExtension 演示如何检查文件扩展名 | Example of checking file extension
func ExampleValidator_IsAllowedExtension() {
	// 创建验证器 | Create validator
	validator := file.NewValidator(&file.ValidatorConfig{
		AllowedExtensions: []string{".pdf", ".png", ".jpg", ".jpeg"},
	})

	// 检查扩展名 | Check extension
	if validator.IsAllowedExtension(".pdf") {
		fmt.Println("PDF extension is allowed")
	}

	if validator.IsAllowedExtension("png") { // 不带点号也可以 | Works without dot
		fmt.Println("PNG extension is allowed")
	}

	if !validator.IsAllowedExtension(".txt") {
		fmt.Println("TXT extension is not allowed")
	}

	// Output:
	// PDF extension is allowed
	// PNG extension is allowed
	// TXT extension is not allowed
}

// ExampleNewValidator 演示如何创建验证器 | Example of creating a validator
func ExampleNewValidator() {
	// 只验证MIME类型 | Validate only MIME type
	mimeValidator := file.NewValidator(&file.ValidatorConfig{
		AllowedMimeTypes: []string{"application/pdf"},
		MaxFileSize:      10 * 1024 * 1024,
	})
	fmt.Printf("MIME validator created with %d allowed types\n", len(mimeValidator.GetAllowedMimeTypes()))

	// 只验证扩展名 | Validate only extension
	extValidator := file.NewValidator(&file.ValidatorConfig{
		AllowedExtensions: []string{".pdf", ".doc", ".docx"},
		MaxFileSize:       10 * 1024 * 1024,
	})
	fmt.Printf("Extension validator created with %d allowed extensions\n", len(extValidator.GetAllowedExtensions()))

	// 同时验证MIME类型和扩展名 | Validate both MIME type and extension
	bothValidator := file.NewValidator(&file.ValidatorConfig{
		AllowedMimeTypes:  []string{"application/pdf"},
		AllowedExtensions: []string{".pdf"},
		MaxFileSize:       10 * 1024 * 1024,
	})
	fmt.Printf("Combined validator created with %d MIME types and %d extensions\n",
		len(bothValidator.GetAllowedMimeTypes()),
		len(bothValidator.GetAllowedExtensions()))

	// Output:
	// MIME validator created with 1 allowed types
	// Extension validator created with 3 allowed extensions
	// Combined validator created with 1 MIME types and 1 extensions
}
