//go:build darwin

package macbundle

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func rejectQuarantined(file *os.File) error {
	_, err := unix.Fgetxattr(int(file.Fd()), "com.apple.quarantine", nil)
	if err == nil {
		return fmt.Errorf("quarantined executable refused")
	}
	if errors.Is(err, unix.ENOATTR) {
		return nil
	}
	return fmt.Errorf("inspect quarantine metadata: %w", err)
}
