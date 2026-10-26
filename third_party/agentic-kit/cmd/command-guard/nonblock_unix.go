//go:build !windows

package main

import "syscall"

const nonblock = syscall.O_NONBLOCK
