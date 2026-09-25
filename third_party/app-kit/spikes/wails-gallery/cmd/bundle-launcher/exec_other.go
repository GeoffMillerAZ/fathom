//go:build !darwin

package main

import "errors"

func execParent(string, []string, []string) error {
	return errors.New("private bundle execution is only implemented for macOS")
}
