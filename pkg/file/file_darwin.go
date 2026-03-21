//go:build darwin

package file

import (
	"errors"
	"os"
	"syscall"
	"time"
)

// CreationTime 获取文件创建时间（macOS实现）| Get file creation time (macOS implementation)
// macOS 特有的 Birthtimespec 字段 | macOS-specific Birthtimespec field
func CreationTime(path string) (time.Time, error) {
	fileInfo, err := os.Lstat(path)
	if err != nil {
		return time.Time{}, err
	}

	if sysInfo, ok := fileInfo.Sys().(*syscall.Stat_t); ok {
		// macOS 特有的 Birthtimespec 字段，表示文件创建时间 | macOS-specific Birthtimespec field for file creation time
		return time.Unix(sysInfo.Birthtimespec.Sec, sysInfo.Birthtimespec.Nsec), nil
	}

	return time.Time{}, errors.New("unable to determine creation time")
}
