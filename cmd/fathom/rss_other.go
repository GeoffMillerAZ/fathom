//go:build !linux && !(darwin && cgo)

package main

// rss_other.go — DG-16's fallback leg: any platform without a proven
// resident-memory reading (darwin built without cgo, or any GOOS besides
// darwin/linux). Per the owner's amendment: omit the /metrics lines
// entirely on such a build — never print a fabricated zero standing in for
// "unknown."
func processResidentMemoryBytes() (int64, bool) { return 0, false }

func processPhysFootprintBytes() (int64, bool) { return 0, false }
