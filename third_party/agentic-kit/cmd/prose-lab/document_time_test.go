package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDocumentWorldTimeGoCUEParity(t *testing.T) {
	cue, err := exec.LookPath("cue")
	if err != nil {
		t.Skip("CUE unavailable; no schema parity claim")
	}
	for _, tc := range []struct {
		value string
		valid bool
	}{
		{"2026-03-04T09:10:00Z", true},
		{"2026-03-04T09:10:00.123456789Z", true},
		{"2026-03-04T09:10:00.1Z", true},
		{"0000-01-01T00:00:00Z", true},
		{"2026-02-30T09:10:00Z", false},
		{"2026-03-04T09:10:00.120Z", false},
		{"2026-03-04T09:10:00+00:00", false},
		{"0001-01-01T00:00:00Z", false},
		{"2026-03-04", false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			_, goErr := parseDocumentOccurredAt(tc.value)
			fact := documentFact{ID: "f-01", Text: "Synthetic observation", Kind: "observed", Required: true, OccurredAt: tc.value}
			path := filepath.Join(t.TempDir(), "fact.json")
			if err := os.WriteFile(path, jsonBytes(fact), 0o600); err != nil {
				t.Fatal(err)
			}
			cueErr := exec.Command(cue, "vet", "schema.cue", path, "-d", "#DocumentFact").Run()
			if (goErr == nil) != tc.valid || (cueErr == nil) != tc.valid {
				t.Fatalf("valid=%t Go=%v CUE=%v", tc.valid, goErr, cueErr)
			}
		})
	}
}
