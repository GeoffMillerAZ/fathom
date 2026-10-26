//go:build unix

package main

import "syscall"

const documentReviewNoFollow = syscall.O_NOFOLLOW
