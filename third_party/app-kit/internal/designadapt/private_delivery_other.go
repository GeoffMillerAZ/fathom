//go:build !darwin && !linux

package designadapt

import (
	"fmt"
	"os"
)

func privateDeliveryReadFlags(label string) (int, error) {
	return 0, fmt.Errorf("%s: secure checks currently support macOS and Linux only", label)
}

func privateDeliveryRegular(os.FileInfo) bool { return false }
