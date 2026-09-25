//go:build !server && darwin && appkit_contained

package main

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestContainedNativeWindowHasExactHTTPOrigin(t *testing.T) {
	var options application.WebviewWindowOptions
	configureNativeWindow(&options, "http://127.0.0.1:12345")
	if options.Mac.HTTPOnlyOrigin != "http://127.0.0.1:12345" || requireNativeContainment() != nil {
		t.Fatal("explicit native profile not configured")
	}
}
