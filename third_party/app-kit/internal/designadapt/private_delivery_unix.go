//go:build darwin || linux

package designadapt

import (
	"os"
	"syscall"
)

func privateDeliveryReadFlags(_ string) (int, error) {
	return os.O_RDONLY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK, nil
}

func privateDeliveryRegular(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return info.Mode().IsRegular() && ok && stat.Nlink == 1
}
