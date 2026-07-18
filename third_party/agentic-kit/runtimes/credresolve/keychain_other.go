//go:build !darwin

package credresolve

import "errors"

// ErrKeychainUnavailable reports that the platform keychain cannot be read.
// Inject a ReadKeychain returning this error to disable the Keychain step.
var ErrKeychainUnavailable = errors.New("credresolve: keychain unavailable")

// defaultReadKeychain is a stub on platforms without a supported keychain;
// the resolver falls through to the Claude Code credential files.
func defaultReadKeychain(string) ([]byte, error) {
	return nil, ErrKeychainUnavailable
}
