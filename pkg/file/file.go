// Package file 提供文件和目录操作相关的工具函数 | Provides file and directory operation utilities
package file

import (
	"bufio"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go-port-forward/pkg/pool"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Info 扩展的文件信息结构 | Extended file information struct
type Info struct {
	// ModTime 修改时间 | Modification time
	ModTime time.Time
	// CreationTime 创建时间 | Creation time
	CreationTime time.Time

	// Path 文件路径 | File path
	Path string
	// Name 文件名 | File name
	Name string
	// Ext 文件扩展名 | File extension
	Ext string
	// MimeType MIME类型 | MIME type
	MimeType string

	// Size 文件大小（字节）| File size (bytes)
	Size int64

	// IsDir 是否为目录 | Whether it is a directory
	IsDir bool

	// Mode 文件权限模式 | File permission mode
	Mode os.FileMode
}

// GetFileInfo 获取扩展的文件信息 | Get extended file information
// 参数 Parameters:
//   - path: 文件或目录路径 | File or directory path
//
// 返回 Returns:
//   - *Info: 扩展的文件信息 | Extended file information
//   - error: 获取信息过程中的错误 | Error during information retrieval
func GetFileInfo(path string) (*Info, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	creationTime, _ := CreationTime(path)
	ext := strings.ToLower(filepath.Ext(path))

	return &Info{
		Path:         path,
		Name:         info.Name(),
		Size:         info.Size(),
		Mode:         info.Mode(),
		ModTime:      info.ModTime(),
		CreationTime: creationTime,
		IsDir:        info.IsDir(),
		Ext:          ext,
		MimeType:     GetMimeType(ext),
	}, nil
}

// Exists 检查文件或目录是否存在 | Check if file or directory exists
// 参数 Parameters:
//   - path: 文件或目录路径 | File or directory path
//
// 返回 Returns:
//   - bool: 文件或目录是否存在 | Whether file or directory exists
func Exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// IsFile 检查路径是否为文件 | Check if path is a file
// 参数 Parameters:
//   - path: 文件路径 | File path
//
// 返回 Returns:
//   - bool: 是否为文件（非目录）| Whether it is a file (not directory)
func IsFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// IsDir 检查路径是否为目录 | Check if path is a directory
// 参数 Parameters:
//   - path: 目录路径 | Directory path
//
// 返回 Returns:
//   - bool: 是否为目录 | Whether it is a directory
func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// IsEmpty 检查文件是否为空 | Check if file is empty
// 参数 Parameters:
//   - path: 文件路径 | File path
//
// 返回 Returns:
//   - bool: 文件是否为空（大小为0）| Whether file is empty (size is 0)
//   - error: 检查过程中的错误 | Error during check
func IsEmpty(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return info.Size() == 0, nil
}

// Size 获取文件大小 | Get file size
// 参数 Parameters:
//   - path: 文件路径 | File path
//
// 返回 Returns:
//   - int64: 文件大小（字节）| File size (bytes)
//   - error: 获取大小过程中的错误 | Error during size retrieval
func Size(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// EnsureDir 确保目录存在，如果不存在则创建 | Ensure directory exists, create if not
// 参数 Parameters:
//   - dir: 目录路径 | Directory path
//
// 返回 Returns:
//   - error: 创建目录过程中的错误 | Error during directory creation
func EnsureDir(dir string) error {
	if !Exists(dir) {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}

// EnsureFile 确保文件存在，如果不存在则创建空文件 | Ensure file exists, create empty file if not
// 参数 Parameters:
//   - path: 文件路径 | File path
//
// 返回 Returns:
//   - error: 创建文件过程中的错误 | Error during file creation
func EnsureFile(path string) error {
	if Exists(path) {
		return nil
	}

	// 确保父目录存在
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	return file.Close()
}

// Copy 复制文件 | Copy file
// 参数 Parameters:
//   - src: 源文件路径 | Source file path
//   - dst: 目标文件路径 | Destination file path
//
// 返回 Returns:
//   - error: 复制过程中的错误 | Error during copy
func Copy(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	// 确保目标目录存在
	if err = EnsureDir(filepath.Dir(dst)); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	// 使用对象池的缓冲区进行高效复制 | Use buffer from pool for efficient copy
	buffer := pool.BufferPool.Get().([]byte)
	defer pool.BufferPool.Put(buffer)

	_, err = io.CopyBuffer(dstFile, srcFile, buffer)
	if err != nil {
		return err
	}

	// 复制文件权限
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	return os.Chmod(dst, srcInfo.Mode())
}

// Move 移动文件 | Move file
// 参数 Parameters:
//   - src: 源文件路径 | Source file path
//   - dst: 目标文件路径 | Destination file path
//
// 返回 Returns:
//   - error: 移动过程中的错误 | Error during move
func Move(src, dst string) error {
	// 确保目标目录存在
	if err := EnsureDir(filepath.Dir(dst)); err != nil {
		return err
	}

	return os.Rename(src, dst)
}

// Remove 删除文件或目录 | Remove file or directory
// 参数 Parameters:
//   - path: 文件或目录路径 | File or directory path
//
// 返回 Returns:
//   - error: 删除过程中的错误 | Error during removal
func Remove(path string) error {
	return os.RemoveAll(path)
}

// ReadLines 按行读取文件 | Read file line by line
// 参数 Parameters:
//   - path: 文件路径 | File path
//
// 返回 Returns:
//   - []string: 文件中的所有行 | All lines in the file
//   - error: 读取过程中的错误 | Error during reading
func ReadLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines, scanner.Err()
}

// WriteLines 按行写入文件 | Write lines to file
// 参数 Parameters:
//   - path: 文件路径 | File path
//   - lines: 要写入的行数组 | Lines to write
//
// 返回 Returns:
//   - error: 写入过程中的错误 | Error during writing
func WriteLines(path string, lines []string) error {
	// 确保目录存在
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	writer := bufio.NewWriter(file)
	defer func() { _ = writer.Flush() }()

	for _, line := range lines {
		if _, err = writer.WriteString(line + "\n"); err != nil {
			return err
		}
	}

	return nil
}

// AppendLine 向文件追加一行 | Append a line to file
// 参数 Parameters:
//   - path: 文件路径 | File path
//   - line: 要追加的行内容 | Line content to append
//
// 返回 Returns:
//   - error: 追加过程中的错误 | Error during appending
func AppendLine(path, line string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	_, err = file.WriteString(line + "\n")
	return err
}

// HashFile 计算文件哈希值 | Calculate file hash
// 参数 Parameters:
//   - path: 文件路径 | File path
//   - hashType: 哈希类型（"md5", "sha1", "sha256"）| Hash type ("md5", "sha1", "sha256")
//
// 返回 Returns:
//   - string: 文件的哈希值（十六进制字符串）| File hash (hex string)
//   - error: 计算过程中的错误 | Error during calculation
func HashFile(path string, hashType string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	var hasher hash.Hash
	switch strings.ToLower(hashType) {
	case "md5":
		hasher = md5.New()
	case "sha1":
		hasher = sha1.New()
	case "sha256":
		hasher = sha256.New()
	default:
		return "", errors.New("unsupported hash type: " + hashType)
	}

	// 使用对象池的缓冲区进行高效复制 | Use buffer from pool for efficient copy
	buffer := pool.BufferPool.Get().([]byte)
	defer pool.BufferPool.Put(buffer)

	if _, err = io.CopyBuffer(hasher, file, buffer); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// MD5File 计算文件 MD5 值 | Calculate file MD5 hash
// 参数 Parameters:
//   - path: 文件路径 | File path
//
// 返回 Returns:
//   - string: 文件的 MD5 哈希值 | File MD5 hash
//   - error: 计算过程中的错误 | Error during calculation
func MD5File(path string) (string, error) {
	return HashFile(path, "md5")
}

// SHA1File 计算文件 SHA1 值 | Calculate file SHA1 hash
// 参数 Parameters:
//   - path: 文件路径 | File path
//
// 返回 Returns:
//   - string: 文件的 SHA1 哈希值 | File SHA1 hash
//   - error: 计算过程中的错误 | Error during calculation
func SHA1File(path string) (string, error) {
	return HashFile(path, "sha1")
}

// SHA256File 计算文件 SHA256 值 | Calculate file SHA256 hash
// 参数 Parameters:
//   - path: 文件路径 | File path
//
// 返回 Returns:
//   - string: 文件的 SHA256 哈希值 | File SHA256 hash
//   - error: 计算过程中的错误 | Error during calculation
func SHA256File(path string) (string, error) {
	return HashFile(path, "sha256")
}

// GetMimeType 根据文件扩展名获取 MIME 类型 | Get MIME type by file extension
// 参数 Parameters:
//   - ext: 文件扩展名（如 ".txt", ".jpg"）| File extension (e.g. ".txt", ".jpg")
//
// 返回 Returns:
//   - string: 对应的 MIME 类型，未知类型返回 "application/octet-stream" | Corresponding MIME type, returns "application/octet-stream" for unknown types
func GetMimeType(ext string) string {
	ext = strings.ToLower(ext)
	mimeTypes := map[string]string{
		".txt":  "text/plain",
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".json": "application/json",
		".xml":  "application/xml",
		".pdf":  "application/pdf",
		".zip":  "application/zip",
		".tar":  "application/x-tar",
		".gz":   "application/gzip",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".bmp":  "image/bmp",
		".svg":  "image/svg+xml",
		".ico":  "image/x-icon",
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".mp4":  "video/mp4",
		".avi":  "video/x-msvideo",
		".mov":  "video/quicktime",
		".wmv":  "video/x-ms-wmv",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".ppt":  "application/vnd.ms-powerpoint",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	}

	if mimeType, ok := mimeTypes[ext]; ok {
		return mimeType
	}
	return "application/octet-stream"
}

// ListFiles 列出目录中的文件 | List files in directory
// 参数 Parameters:
//   - dir: 目录路径 | Directory path
//   - recursive: 是否递归列出子目录中的文件 | Whether to recursively list files in subdirectories
//
// 返回 Returns:
//   - []string: 文件路径列表 | File path list
//   - error: 列出过程中的错误 | Error during listing
func ListFiles(dir string, recursive bool) ([]string, error) {
	var files []string

	if recursive {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				files = append(files, path)
			}
			return nil
		})
		return files, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}

	return files, nil
}

// ListDirs 列出目录中的子目录 | List subdirectories in directory
// 参数 Parameters:
//   - dir: 目录路径 | Directory path
//   - recursive: 是否递归列出所有子目录 | Whether to recursively list all subdirectories
//
// 返回 Returns:
//   - []string: 子目录路径列表 | Subdirectory path list
//   - error: 列出过程中的错误 | Error during listing
func ListDirs(dir string, recursive bool) ([]string, error) {
	var dirs []string

	if recursive {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() && path != dir {
				dirs = append(dirs, path)
			}
			return nil
		})
		return dirs, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, filepath.Join(dir, entry.Name()))
		}
	}

	return dirs, nil
}

// DirSize 计算目录大小 | Calculate directory size
// 参数 Parameters:
//   - dir: 目录路径 | Directory path
//
// 返回 Returns:
//   - int64: 目录总大小（字节）| Total directory size (bytes)
//   - error: 计算过程中的错误 | Error during calculation
func DirSize(dir string) (int64, error) {
	var size int64

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

// CleanDir 清空目录但保留目录本身 | Clean directory contents but keep the directory itself
// 参数 Parameters:
//   - dir: 目录路径 | Directory path
//
// 返回 Returns:
//   - error: 清空过程中的错误 | Error during cleaning
func CleanDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if err = os.RemoveAll(path); err != nil {
			return err
		}
	}

	return nil
}

// TempFile 创建临时文件 | Create temporary file
// 参数 Parameters:
//   - dir: 临时文件目录，空字符串使用系统默认临时目录 | Temp file directory, empty string uses system default
//   - pattern: 文件名模式 | File name pattern
//
// 返回 Returns:
//   - *os.File: 创建的临时文件 | Created temporary file
//   - error: 创建过程中的错误 | Error during creation
func TempFile(dir, pattern string) (*os.File, error) {
	return os.CreateTemp(dir, pattern)
}

// TempDir 创建临时目录 | Create temporary directory
// 参数 Parameters:
//   - dir: 父目录，空字符串使用系统默认临时目录 | Parent directory, empty string uses system default
//   - pattern: 目录名模式 | Directory name pattern
//
// 返回 Returns:
//   - string: 创建的临时目录路径 | Created temporary directory path
//   - error: 创建过程中的错误 | Error during creation
func TempDir(dir, pattern string) (string, error) {
	return os.MkdirTemp(dir, pattern)
}

// FormatSize 格式化文件大小为人类可读的格式 | Format file size to human-readable format
// 参数 Parameters:
//   - size: 文件大小（字节）| File size (bytes)
//
// 返回 Returns:
//   - string: 格式化后的大小字符串（如 "1.5 MB"）| Formatted size string (e.g. "1.5 MB")
func FormatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(size)/float64(div), units[exp])
}

// WriteFile 写入文件内容 | Write file content
func WriteFile(filename string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filename, data, perm)
}

// ReadFile 读取文件内容 | Read file content
func ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

// IsReadable 检查文件是否可读 | Check if file is readable
func IsReadable(filename string) bool {
	file, err := os.Open(filename)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()
	return true
}
