//go:build !darwin

package macbundle

import (
	"fmt"
	"os"
	"runtime"
)

func rejectQuarantined(_ *os.File) error {
	return fmt.Errorf("macOS quarantine inspection unsupported on %s", runtime.GOOS)
}
