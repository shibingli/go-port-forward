//go:build linux

package disk

import (
	"fmt"
	"syscall"
)

// getDiskSpace 获取磁盘空间信息(Linux实现) | Get disk space information (Linux implementation)
func getDiskSpace(path string) (*SpaceInfo, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil, fmt.Errorf("failed to get disk space for %s: %w", path, err)
	}

	// 计算空间信息 | Calculate space information
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free
	usedPercent := 0.0
	if total > 0 {
		usedPercent = float64(used) / float64(total) * 100
	}

	return &SpaceInfo{
		Total:       total,
		Free:        free,
		Used:        used,
		UsedPercent: usedPercent,
	}, nil
}
