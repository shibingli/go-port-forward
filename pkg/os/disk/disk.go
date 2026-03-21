// Package disk 提供磁盘空间检查功能
// Package disk provides disk space checking functionality
package disk

import (
	"fmt"
)

// SpaceInfo 磁盘空间信息 | Disk space information
type SpaceInfo struct {
	// Total 总空间(字节) | Total space in bytes
	Total uint64 `json:"total"`
	// Free 可用空间(字节) | Free space in bytes
	Free uint64 `json:"free"`
	// Used 已用空间(字节) | Used space in bytes
	Used uint64 `json:"used"`
	// UsedPercent 使用百分比 | Used percentage
	UsedPercent float64 `json:"used_percent"`
}

// GetDiskSpace 获取指定路径的磁盘空间信息 | Get disk space information for the specified path
// path: 要检查的路径 | Path to check
// 返回磁盘空间信息和错误 | Returns disk space information and error
func GetDiskSpace(path string) (*SpaceInfo, error) {
	return getDiskSpace(path)
}

// HasEnoughSpace 检查磁盘是否有足够的空间 | Check if disk has enough space
// path: 要检查的路径 | Path to check
// requiredBytes: 需要的字节数 | Required bytes
// 返回是否有足够空间和错误 | Returns whether there is enough space and error
func HasEnoughSpace(path string, requiredBytes uint64) (bool, error) {
	info, err := GetDiskSpace(path)
	if err != nil {
		return false, err
	}

	// 保留10%的空间作为缓冲 | Reserve 10% space as buffer
	safeSpace := uint64(float64(info.Free) * 0.9)
	return safeSpace >= requiredBytes, nil
}

// FormatBytes 格式化字节数为人类可读的格式 | Format bytes to human-readable format
// bytes: 字节数 | Bytes
// 返回格式化后的字符串 | Returns formatted string
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// GetDiskSpaceWithFormat 获取格式化的磁盘空间信息 | Get formatted disk space information
// path: 要检查的路径 | Path to check
// 返回格式化的信息字符串和错误 | Returns formatted information string and error
func GetDiskSpaceWithFormat(path string) (string, error) {
	info, err := GetDiskSpace(path)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Total: %s, Used: %s (%.2f%%), Free: %s",
		FormatBytes(info.Total),
		FormatBytes(info.Used),
		info.UsedPercent,
		FormatBytes(info.Free),
	), nil
}

// getDiskSpace 获取磁盘空间信息的平台特定实现 | Platform-specific implementation of getting disk space
// 此函数在disk_windows.go和disk_linux.go中实现 | This function is implemented in disk_windows.go and disk_linux.go
