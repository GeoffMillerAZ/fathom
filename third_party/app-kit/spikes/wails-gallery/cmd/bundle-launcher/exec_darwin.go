//go:build darwin

package main

import "syscall"

func execParent(path string, argv, environment []string) error {
	return syscall.Exec(path, argv, environment)
}
