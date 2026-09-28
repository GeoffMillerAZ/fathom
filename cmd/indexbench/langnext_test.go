package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestCmdLangNext: the driver's facts (§15) under D6 — open queue items in
// order, each eligible or with the rows still blocking it.
func TestCmdLangNext(t *testing.T) {
	var out, errb bytes.Buffer
	if code := cmdLangNext([]string{"-root", "../..", "-all"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) == 0 || !strings.Contains(out.String(), "cpp") {
		t.Fatalf("lang-next -all lists no cpp line:\n%s", out.String())
	}
	for _, l := range lines {
		if !strings.Contains(l, "\teligible") && !strings.Contains(l, "\tblocked: ") {
			t.Fatalf("line %q is neither eligible nor blocked", l)
		}
	}
}
