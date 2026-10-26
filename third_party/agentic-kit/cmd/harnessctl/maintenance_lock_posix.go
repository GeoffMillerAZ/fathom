//go:build harness_maintenance && (darwin || linux)

package main

import (
	"errors"
	"os"
	"syscall"
)

func maintenanceStorageSupported() bool { return true }
func maintenanceSingleLink(info os.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && st.Nlink == 1
}
func maintenanceTryLock(file *os.File) error {
	e := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if e == nil {
		return nil
	}
	if errors.Is(e, syscall.EWOULDBLOCK) || errors.Is(e, syscall.EAGAIN) {
		return errMaintenanceStoreBusy
	}
	return errMaintenanceStoreUnsupported
}
func maintenanceUnlock(file *os.File) error {
	if syscall.Flock(int(file.Fd()), syscall.LOCK_UN) != nil {
		return errMaintenanceStoreIncomplete
	}
	return nil
}
