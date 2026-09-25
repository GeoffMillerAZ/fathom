//go:build !server && darwin && appkit_contained

package main

import "github.com/wailsapp/wails/v3/pkg/application"

func requireNativeContainment() error { return nil }

func configureNativeWindow(options *application.WebviewWindowOptions, origin string) {
	options.Mac.HTTPOnlyOrigin = origin
}
