package main

import (
	"strings"
	"testing"
)

func TestOfflineHarnessVersionReservedBothDirections(t *testing.T) {
	controls := []string{"null", "oracle", "wrong-patch"}
	for _, owner := range controls {
		for _, name := range append(append([]string{}, controls...), "claude", "fake") {
			t.Run(owner+"/"+name, func(t *testing.T) {
				err := validateOfflineHarnessVersion(name, owner+"-1")
				if owner == name {
					if err != nil {
						t.Fatal(err)
					}
				} else if err == nil {
					t.Fatalf("%s accepted reserved %s-1: %v", name, owner, err)
				}
			})
		}
		t.Run(owner+"/unreserved", func(t *testing.T) {
			if err := validateOfflineHarnessVersion(owner, "custom-1"); err == nil {
				t.Fatal("control accepted another version")
			}
		})
	}
	for _, name := range []string{"claude", "fake"} {
		if err := validateOfflineHarnessVersion(name, "custom-1"); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCmdRunRejectsImpersonatedControl(t *testing.T) {
	stderr := captureStderr(t, func() {
		if rc := run([]string{"run", "-corpus", "unused.json", "-run-id", "fixture", "-harness", "claude", "-harness-version", "oracle-1"}); rc != 2 {
			t.Errorf("exit=%d", rc)
		}
	})
	if !strings.Contains(stderr, "reserved for -harness oracle") {
		t.Fatalf("not refused before corpus or harness: %s", stderr)
	}
}
