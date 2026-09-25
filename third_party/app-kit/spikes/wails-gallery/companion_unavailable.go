//go:build !darwin

package main

import (
	"context"
	"errors"
	"io"
	"time"
)

func runCompanion(context.Context, *host, string, io.Writer, func(*host) error, time.Duration) error {
	return errors.New("companion recovery is a macOS-only experiment; use -browser")
}

func runNativeChild(context.Context, io.Writer) error {
	return errors.New("native companion is unavailable on this target; use -browser")
}
