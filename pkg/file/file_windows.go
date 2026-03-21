//go:build windows

package file

import (
	"errors"
	"os"
	"syscall"
	"time"
)

func CreationTime(path string) (time.Time, error) {
	fileInfo, err := os.Lstat(path)
	if err != nil {
		return time.Time{}, err
	}

	if sysInfo, ok := fileInfo.Sys().(*syscall.Win32FileAttributeData); ok {
		nanoseconds := sysInfo.CreationTime.Nanoseconds()
		return time.Unix(0, nanoseconds), nil
	}

	return time.Time{}, errors.New("unable to determine creation time")
}
