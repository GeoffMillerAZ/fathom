package main

import (
	"runtime/debug"
	"strings"
)

// Build identity is descriptive, not a release or exact working-tree attestation.
// Verification receipts additionally hash the actual executable.
func serverBuildVersion() string {
	info, _ := debug.ReadBuildInfo()
	return describeBuild(info)
}

func describeBuild(info *debug.BuildInfo) string {
	if info == nil {
		return "devel"
	}
	version := info.Main.Version
	if version == "" || version == "(devel)" {
		version = "devel"
	}
	var revision string
	dirty := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			dirty = setting.Value == "true"
		}
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	if revision != "" && !strings.Contains(version, revision) {
		if strings.Contains(version, "+") {
			version += "." + revision
		} else {
			version += "+" + revision
		}
	}
	if dirty && !strings.HasSuffix(version, "+dirty") && !strings.HasSuffix(version, ".dirty") {
		version += ".dirty"
	}
	return version
}
