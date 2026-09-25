//go:build darwin && server

package main

import (
	"context"
	"errors"
	"io"
)

func runNativeChild(context.Context, io.Writer) error {
	return errors.New("server binary cannot be a native companion; select the contained native build")
}
