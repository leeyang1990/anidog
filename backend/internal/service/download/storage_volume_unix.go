//go:build !windows

package download

import (
	"fmt"
	"syscall"
)

func storageVolume(path string) (string, error) {
	var stat syscall.Stat_t
	if err := syscall.Stat(path, &stat); err != nil {
		return "", err
	}
	return fmt.Sprint(stat.Dev), nil
}
