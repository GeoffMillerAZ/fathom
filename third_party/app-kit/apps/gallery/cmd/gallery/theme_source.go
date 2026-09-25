package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
)

func readThemeSource(path string) ([]byte, error) {
	// Refuse ordinary FIFOs/devices before Open can block. This owner-selected
	// file is not a directory controlled by a hostile concurrent writer.
	before, err := os.Stat(path)
	if err != nil || !before.Mode().IsRegular() || before.Size() > 64<<10 {
		return nil, errors.New("selected theme source must be a regular file no larger than 64KiB")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read selected theme source: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 64<<10 {
		return nil, errors.New("selected theme source must be a regular file no larger than 64KiB")
	}
	body, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	if err != nil || len(body) == 0 || len(body) > 64<<10 {
		return nil, errors.New("selected theme source is empty, too large or unreadable")
	}
	return body, nil
}

// Bind this unpublished local guide to the actual running implementation, not
// merely a HEAD that omits working-tree changes. This is not a released module pin.
func executableIdentity() (string, error) {
	name, err := os.Executable()
	if err != nil {
		return "", err
	}
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("local-gallery-binary:sha256:%x", h.Sum(nil)), nil
}
