//go:build windows

package disk

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpace = kernel32.NewProc("GetDiskFreeSpaceExW")
)

// getDiskSpace 获取磁盘空间信息(Windows实现) | Get disk space information (Windows implementation)
func getDiskSpace(path string) (*SpaceInfo, error) {
	// 转换路径为UTF-16 | Convert path to UTF-16
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("failed to convert path to UTF-16: %w", err)
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64

	// 调用Windows API | Call Windows API
	ret, _, err := getDiskFreeSpace.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)

	if ret == 0 {
		return nil, fmt.Errorf("failed to get disk space for %s: %w", path, err)
	}

	// 计算空间信息 | Calculate space information
	used := totalBytes - totalFreeBytes
	usedPercent := 0.0
	if totalBytes > 0 {
		usedPercent = float64(used) / float64(totalBytes) * 100
	}

	return &SpaceInfo{
		Total:       totalBytes,
		Free:        freeBytesAvailable,
		Used:        used,
		UsedPercent: usedPercent,
	}, nil
}
