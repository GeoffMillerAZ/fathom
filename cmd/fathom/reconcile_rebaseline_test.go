package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestRebaselineUsageSaysWhatAndWhen (CL-9): `fathom fleet reconcile --help`
// says what --rebaseline does (re-records a drifted entry's baseline digest
// as the operator's change; fathom never merges it) and when it applies
// (only with --apply, only to drifted entries, a skill only when fathom
// shipped that render, FA-24).
func TestRebaselineUsageSaysWhatAndWhen(t *testing.T) {
	ix := loadHelpIndex(t)
	usage := ""
	for _, f := range ix.proveHelp("fleet", "reconcile").Flags {
		if f.Name == "rebaseline" {
			usage = f.Usage
		}
	}
	for _, want := range []string{
		"only with --apply",
		"DRIFTED",
		"MCP entry, hook entry or owned skill",
		"re-records its baseline digest",
		"as your change",
		"fathom never merges",
		"only when its bytes are a render fathom shipped",
		"Entries that have not drifted are left alone",
		"any other skill edit is refused and stays DRIFT",
	} {
		if !strings.Contains(usage, want) {
			t.Errorf("--rebaseline usage %q lacks %q", usage, want)
		}
	}
	helpSandbox(t)
	var out, errb bytes.Buffer
	if rc := runWithStdin([]string{"fleet", "reconcile", "--help"}, &recordingReader{}, &out, &errb); rc != 0 || !strings.Contains(out.String(), usage) {
		t.Errorf("fleet reconcile --help: rc %d, stdout lacks the --rebaseline usage:\n%s", rc, out.String())
	}
}
