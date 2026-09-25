//go:build !server && (windows || (darwin && !appkit_contained))

package main

import (
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func requireNativeContainment() error {
	return errors.New("native experiment requires the prepared macOS HTTP-only patch and appkit_contained build tag; use -browser for independent HTTP delivery")
}

func configureNativeWindow(_ *application.WebviewWindowOptions, _ string) {}
