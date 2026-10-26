package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Exercise the authoring schema and CLI validator on the same boundary values.
// CUE is a required project tool; no provider or network is involved.
func TestSchemaAuthoringBoundaries(t *testing.T) {
	cue, err := exec.LookPath("cue")
	if err != nil {
		t.Fatal("CUE is required for schema/Go contract checks")
	}
	packCase := func(edit func(*Pack)) Pack {
		p := fixture[Pack](t, "review-triage.pack.json")
		edit(&p)
		return p
	}
	inputCase := func(state string) Input {
		in := fixture[Input](t, "case-1.input.json")
		in.CapturedState = state
		return in
	}
	capture := fixture[Capture](t, "case-1.capture.json")
	both := capture
	both.Cause = "timeout"
	neither := capture
	neither.Response = nil
	preflight := capture
	preflight.Response = nil
	preflight.Cause = "no-provider"
	preflight.AttemptCount = 1
	noProvider := preflight
	noProvider.AttemptCount = 0
	timeout := capture
	timeout.Response = nil
	timeout.Cause = "timeout"
	cases := []struct {
		name, definition string
		value            any
		valid            bool
	}{
		{"valid-pack", "#Pack", packCase(func(*Pack) {}), true},
		{"missing-unknown", "#Pack", packCase(func(p *Pack) { p.Questions[0].Options = p.Questions[0].Options[:2] }), false},
		{"duplicate-option", "#Pack", packCase(func(p *Pack) { p.Questions[0].Options[1].Name = p.Questions[0].Options[0].Name }), false},
		{"duplicate-question", "#Pack", packCase(func(p *Pack) { p.Questions = append(p.Questions, p.Questions[0]) }), false},
		{"unknown-favorable", "#Pack", packCase(func(p *Pack) { p.Questions[0].FavorableOptions = []string{"unknown"} }), false},
		{"foreign-favorable", "#Pack", packCase(func(p *Pack) { p.Questions[0].FavorableOptions = []string{"foreign"} }), false},
		{"duplicate-favorable", "#Pack", packCase(func(p *Pack) { p.Questions[0].FavorableOptions = []string{"inspect", "inspect"} }), false},
		{"valid-input", "#Input", inputCase("evidence"), true},
		{"blank-input", "#Input", inputCase(" \t\n"), false},
		{"unicode-blank-input", "#Input", inputCase("\u0085\u00a0\u2003\u2028\u3000\v"), false},
		{"valid-capture", "#Capture", capture, true},
		{"valid-no-provider", "#Capture", noProvider, true},
		{"valid-timeout", "#Capture", timeout, true},
		{"both-outcomes", "#Capture", both, false},
		{"neither-outcome", "#Capture", neither, false},
		{"preflight-dispatch", "#Capture", preflight, false},
	}
	p := fixture[Pack](t, "review-triage.pack.json")
	in := fixture[Input](t, "case-1.input.json")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var goErr error
			switch v := tc.value.(type) {
			case Pack:
				goErr = v.validate()
			case Input:
				goErr = v.validate()
			case Capture:
				_, goErr = check(p, in, v)
			}
			if (goErr == nil) != tc.valid {
				t.Fatalf("Go validity = %t, want %t", goErr == nil, tc.valid)
			}
			b, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "case.json")
			if err := os.WriteFile(path, b, 0600); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command(cue, "vet", "-c", "schema.cue", "-d", tc.definition, path).CombinedOutput()
			if (err == nil) != tc.valid {
				t.Fatalf("CUE validity = %t, want %t: %s", err == nil, tc.valid, out)
			}
		})
	}
}
