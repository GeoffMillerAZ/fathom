//go:build darwin && !server && nativeprobe && !appkit_contained

package main

import "github.com/wailsapp/wails/v3/pkg/application"

const contained = false
const profile = "wails-beta.21-baseline"

func setPolicy(_ *application.WebviewWindowOptions, _ string) {}
