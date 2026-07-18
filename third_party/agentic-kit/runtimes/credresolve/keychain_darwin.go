//go:build darwin

package credresolve

import (
	"bytes"
	"errors"
	"os/exec"
)

// ErrKeychainUnavailable reports that the platform keychain cannot be read.
// Inject a ReadKeychain returning this error to disable the Keychain step.
var ErrKeychainUnavailable = errors.New("credresolve: keychain unavailable")

// defaultReadKeychain reads a generic keychain item's password payload via
// the macOS `security` CLI. The payload is returned verbatim (trailing
// newline trimmed) and is never logged.
func defaultReadKeychain(service string) ([]byte, error) {
	path, err := exec.LookPath("security")
	if err != nil {
		return nil, ErrKeychainUnavailable
	}
	out, err := exec.Command(path, "find-generic-password", "-s", service, "-w").Output()
	if err != nil {
		// Item absent, access denied, or locked keychain — treat as not found.
		// Deliberately do not include command output in the error: it could
		// contain partial secret material.
		return nil, ErrKeychainUnavailable
	}
	return bytes.TrimRight(out, "\n"), nil
}
