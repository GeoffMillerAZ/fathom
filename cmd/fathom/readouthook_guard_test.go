package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// readoutGuardFixture holds a guard without requires (a push note) and a
// go test guard requiring -race, its entry on line 6.
const readoutGuardFixture = `guards: [
	{
		command_scopes: ["git push *"]
		note: "Pushes run the gate."
	},
	{
		command_scopes: ["go test *"]
		requires: ["-race"]
		note: "The gate runs go test -race."
	},
]
`

// TestReadoutNamesTheGuardTheCommandMissed: under [readouts] mode = rich,
// a command matching a guard's command_scopes that lacks one of its
// requires tokens gets one line beside its output naming the guard's
// pattern, the missing token, the guard's file:line citation and its note.
// The token is matched on the parsed argv of the matching command, never a
// substring of the raw line; a guard without requires and observe mode add
// nothing.
func TestReadoutNamesTheGuardTheCommandMissed(t *testing.T) {
	repo := t.TempDir()
	readoutGitRepo(t, repo, map[string]string{
		"go.mod":            "module example.com/shop\n\ngo 1.22\n",
		".fathom/guard.cue": readoutGuardFixture,
	})
	const passed = "ok  \texample.com/shop\t0.012s\n"
	run := func(t *testing.T, command string) (string, []readoutTestRow) {
		t.Helper()
		code, out, _, _ := runReadoutHook(t, []string{"--event", "post-tool"}, readoutPostToolEnvelope(t, t.TempDir(), "cd "+repo+" && "+command, passed))
		if code != 0 {
			t.Fatalf("exit %d", code)
		}
		if out == "" {
			return "", readoutRows(t)
		}
		var doc struct {
			Spec map[string]any `json:"hookSpecificOutput"`
		}
		if strings.Count(strings.TrimSpace(out), "\n") != 0 || json.Unmarshal([]byte(out), &doc) != nil {
			t.Fatalf("stdout %q: want one JSON document", out)
		}
		for k := range doc.Spec {
			if k != "hookEventName" && k != "additionalContext" {
				t.Errorf("hook output carries %q: the line is advice beside the output, never a block", k)
			}
		}
		ctx, _ := doc.Spec["additionalContext"].(string)
		return ctx, readoutRows(t)
	}

	for _, command := range []string{"go test ./...", "go test ./... && echo -race"} {
		t.Run("missing/"+command, func(t *testing.T) {
			readoutHookEnv(t, "[readouts]\nmode = \"rich\"\n")
			line, rows := run(t, command)
			want := `this command lacks -race, which the guard at .fathom/guard.cue:6 ("go test *") requires: The gate runs go test -race.`
			if !strings.Contains(line, want) {
				t.Fatalf("context %q: want %q", line, want)
			}
			if strings.Contains(line, "git push") || strings.Contains(line, "Pushes run the gate") {
				t.Errorf("context %q names the guard without requires", line)
			}
			if len(rows) != 1 || rows[0].Meta["rich"] != readoutRichGuardKind || rows[0].Meta["emitted"] != "true" {
				t.Fatalf("rows = %+v, want one row naming the guard line as emitted", rows)
			}
		})
	}

	for _, command := range []string{
		"go test -race ./...",
		"go test -race=true ./...",
		"GOFLAGS=-race go test ./...",
		"git push origin main",
		"go vet ./...",
		`echo "go test ./..."`,
	} {
		t.Run("silent/"+command, func(t *testing.T) {
			readoutHookEnv(t, "[readouts]\nmode = \"rich\"\n")
			if line, _ := run(t, command); strings.Contains(line, "lacks") {
				t.Fatalf("%q: context %q, want no guard line", command, line)
			}
		})
	}

	t.Run("observe", func(t *testing.T) {
		readoutHookEnv(t, "[readouts]\nmode = \"observe\"\n")
		if line, _ := run(t, "go test ./..."); line != "" {
			t.Fatalf("observe adds no line, got %q", line)
		}
	})
}
