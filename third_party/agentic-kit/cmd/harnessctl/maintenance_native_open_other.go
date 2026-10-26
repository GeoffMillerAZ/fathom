//go:build harness_maintenance && !darwin && !linux

package main

import (
	"errors"
	"os"
)

func maintenanceNativeOpenSource(string) (*os.File, error) {
	return nil, errors.New("unsupported native host")
}

func maintenanceNativeRootOwned(os.FileInfo) bool     { return false }
func maintenanceNativeAncestorOwned(os.FileInfo) bool { return false }
