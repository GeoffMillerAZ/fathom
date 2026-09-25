package main

import (
	"runtime/debug"
	"testing"
)

func TestBuildVersionDoesNotInventARelease(t *testing.T) {
	for _, tc := range []struct {
		name string
		info *debug.BuildInfo
		want string
	}{
		{"no build info", nil, "devel"},
		{"development", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, "devel"},
		{"Go module dirty identity", &debug.BuildInfo{Main: debug.Module{Version: "v0.0.0-20260918091806-0123456789ab+dirty"}, Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "0123456789abcdef"}, {Key: "vcs.modified", Value: "true"}}}, "v0.0.0-20260918091806-0123456789ab+dirty"},
		{"module release", &debug.BuildInfo{Main: debug.Module{Version: "v0.2.0"}}, "v0.2.0"},
		{"dirty source", &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "0123456789abcdef"}, {Key: "vcs.modified", Value: "true"}}}, "devel+0123456789ab.dirty"},
		{"clean source", &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "0123456789abcdef"}}}, "devel+0123456789ab"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := describeBuild(tc.info); got != tc.want {
				t.Fatalf("version = %q, want %q", got, tc.want)
			}
		})
	}
}
