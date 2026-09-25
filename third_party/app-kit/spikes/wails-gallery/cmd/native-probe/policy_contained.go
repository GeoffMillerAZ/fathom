//go:build darwin && !server && nativeprobe && appkit_contained

package main

import "github.com/wailsapp/wails/v3/pkg/application"

const contained = true
const profile = "wails-beta.21-appkit-http-only"

func setPolicy(options *application.WebviewWindowOptions, origin string) {
	options.Mac.HTTPOnlyOrigin = origin
}
