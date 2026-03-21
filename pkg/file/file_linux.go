//go:build linux

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

	if sysInfo, ok := fileInfo.Sys().(*syscall.Stat_t); ok {
		return time.Unix(sysInfo.Ctim.Sec, sysInfo.Ctim.Nsec), nil
	}

	return time.Time{}, errors.New("unable to determine creation time")
}
