//go:build server || (!darwin && !windows)

package main

import (
	"context"
	"errors"
	"io"
)

func runNative(context.Context, *host, io.Writer) error {
	return errors.New("this build has no native host; use -browser for the same HTTP workflow")
}
