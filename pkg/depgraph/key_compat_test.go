package depgraph

import (
	"crypto/sha256"
	"testing"
)

func TestInputKeySourceCompatibility(t *testing.T) {
	// Fixed inputs and expected keys come from depgraph/key_test.go's
	// TestInputKeyCanonicalizationGoldens in the pinned source snapshot.
	mainDigest := sha256.Sum256([]byte("package main\n"))
	runDigest := sha256.Sum256([]byte("#!/bin/sh\necho acme\n"))
	linkDigest := sha256.Sum256([]byte("main.go"))

	for _, tc := range []struct {
		name           string
		epoch, runMode uint32
		want           Key
	}{
		{"base", 1, ModeExec, "e4c7c6231e869032c7599b844d7eca22bf28205ae84cebd11cd1eb992a97a662"},
		{"no exec", 1, 0, "1d1cdd2dc13bb6fd60471489eeb1576393b13b839d427badc901ca0e71480189"},
		{"epoch 2", 2, ModeExec, "eb4ed30e11b5f4f784dcfa2d341c12adcb60d3956805c0353f3f79ac90830cc9"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := KeyInputs{
				Domain:   "cikit/nodekey/v1",
				Epoch:    tc.epoch,
				Task:     "build",
				Platform: "linux/amd64",
				Sources: []SourceRef{
					{Path: "src/main.go", Digest: mainDigest[:]},
					{Path: "src/run.sh", Mode: tc.runMode, Digest: runDigest[:]},
					{Path: "src/link", Mode: ModeSymlink, Digest: linkDigest[:]},
				},
				Deps:                []string{"bbbb", "aaaa"},
				ToolchainLockDigest: "lock-digest-1",
				CueLang:             "v0.16.1",
				ToolchainPackages:   map[string]string{"go": "nix-go-1", "cue": "nix-cue-1"},
				Env:                 map[string]string{"CGO_ENABLED": "0", "GOFLAGS": "-trimpath"},
				Args:                []string{"./...", "-count=1"},
				SecretNames:         []string{"registry-token", "api-key"},
			}
			if got := InputKey(in); got != tc.want {
				t.Errorf("InputKey = %s, pinned source key = %s", got, tc.want)
			}
		})
	}
}
