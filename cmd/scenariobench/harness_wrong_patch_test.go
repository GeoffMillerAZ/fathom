package main

import (
	"strings"
	"testing"
)

func TestCmdRun_HelpListsWrongPatchHarness(t *testing.T) {
	help := captureStderr(t, func() { run([]string{"run", "-help"}) })
	for _, want := range []string{"claude|fake|null|oracle|wrong-patch", "wrong-patch (test patch alone or non-gold files, no spend", "killed mutants wait for BX-10 (MineMutants)"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help lacks %q: %s", want, help)
		}
	}
	h, err := resolveHarness("wrong-patch", "", "", "F1", "control")
	if err != nil || h.Name() != "wrong-patch" {
		t.Fatalf("resolve = %v %v", h, err)
	}
}

func TestCmdRun_WrongPatchHarness(t *testing.T) {
	for _, localization := range []bool{true, false} {
		label := "F2"
		if localization {
			label = "F1"
		}
		t.Run(label, func(t *testing.T) { runOfflineFixture(t, "wrong-patch", localization, false) })
	}
}

func TestCmdRun_WrongPatchHarnessVersion(t *testing.T) {
	help := captureStderr(t, func() {
		if rc := run([]string{"run", "-corpus", "unused.json", "-run-id", "fixture", "-harness", "wrong-patch", "-harness-version", "oracle-1"}); rc != 2 {
			t.Errorf("exit=%d", rc)
		}
	})
	if !strings.Contains(help, "-harness wrong-patch requires -harness-version wrong-patch-1") {
		t.Fatalf("mislabeled control not refused: %s", help)
	}
}
