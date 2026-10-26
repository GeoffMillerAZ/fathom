//go:build harness_maintenance && !darwin && !linux

package main

import "os"

func maintenanceStorageSupported() bool      { return false }
func maintenanceSingleLink(os.FileInfo) bool { return false }
func maintenanceTryLock(*os.File) error      { return errMaintenanceStoreUnsupported }
func maintenanceUnlock(*os.File) error       { return nil }
