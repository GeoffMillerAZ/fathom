package main

// discovery_test.go — instrument B, the counting-only discovery observer
// (docs/design/production-tokenomics.md §4).

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	heartbeat "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/heartbeat"
)

// withTempBeatLog points FATHOM_HOOKS_LOG at a fresh t.TempDir() file so
// tests never write to the real per-user hooks beat log.
func withTempBeatLog(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", path)
	return path
}

// readBeatsForTest reads back every beat written to the current
// FATHOM_HOOKS_LOG (set via withTempBeatLog) — nil when the file was
// never created (no beats emitted).
func readBeatsForTest(t *testing.T) []heartbeat.Beat {
	t.Helper()
	path := os.Getenv("FATHOM_HOOKS_LOG")
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	beats, err := heartbeat.Load(f, true)
	if err != nil {
		t.Fatalf("heartbeat.Load: %v", err)
	}
	return beats
}

func TestFirstCommandToken(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		{"", ""},
		{"grep -rn foo .", "grep"},
		{"rg --hidden foo", "rg"},
		{"  find . -name '*.go'  ", "find"},
		{"/usr/bin/grep -n foo bar", "grep"},
		{"FOO=bar grep -n foo bar", "grep"},
		{"env grep -n foo", "grep"},
		{"go test ./...", "go"},
		{"git status", "git"},
	}
	for _, c := range cases {
		if got := firstCommandToken(c.command); got != c.want {
			t.Errorf("firstCommandToken(%q) = %q, want %q", c.command, got, c.want)
		}
	}
}

func TestEmitDiscoveryBeatIfShellSearch(t *testing.T) {
	t.Run("grep-like command emits a beat", func(t *testing.T) {
		withTempBeatLog(t)
		emitDiscoveryBeatIfShellSearch("grep -rn foo .", "", false)
		beats := readBeatsForTest(t)
		if len(beats) != 1 || beats[0].Channel != "discovery" || beats[0].Kind != "shell-search" {
			t.Fatalf("beats = %+v", beats)
		}
	})
	t.Run("non-search command emits nothing", func(t *testing.T) {
		withTempBeatLog(t)
		emitDiscoveryBeatIfShellSearch("go test ./...", "", false)
		if beats := readBeatsForTest(t); len(beats) != 0 {
			t.Fatalf("beats = %+v, want none", beats)
		}
	})
}

// TestCmdEditHookPre_DiscoveryTools_ZeroDaemonCalls is the load-bearing
// proof for instrument B: Grep/Glob/Read PreToolUse invocations must never
// touch the daemon, regardless of what's configured — this is a pure
// local counting path (docs/design/production-tokenomics.md §4's own
// "no daemon call" requirement, mirroring the phase-O guard's proof
// pattern in edithook_pre_test.go).
func TestCmdEditHookPre_DiscoveryTools_ZeroDaemonCalls(t *testing.T) {
	withTempBeatLog(t)
	dir := t.TempDir()

	for _, tool := range []string{"Grep", "Glob", "Read"} {
		t.Run(tool, func(t *testing.T) {
			srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "the daemon must never be queried by the discovery observer", 500)
			})
			defer srv.Close()

			hookInput := fmt.Sprintf(`{"tool_name":%q,"tool_input":{"file_path":"whatever.go"},"cwd":%q}`, tool, dir)
			hookStdin(t, hookInput)

			var out, errBuf bytes.Buffer
			code := runWithStdin([]string{"edit-hook", "--pre", "--url=" + srv.URL}, nil, &out, &errBuf)
			if code != 0 {
				t.Fatalf("exit = %d, want 0", code)
			}
			if out.Len() != 0 {
				t.Errorf("expected silence (nothing injected), got %q", out.String())
			}
			if n := *calls; n != 0 {
				t.Errorf("daemon queried %d time(s), want 0", n)
			}
		})
	}
}

// TestCmdEditHookPre_DiscoveryTools_EmitsBeat proves each tool emits its
// own coarse-kind beat, keeping Read in its own bucket (never folded into
// "grep" — the design's own honesty requirement).
func TestCmdEditHookPre_DiscoveryTools_EmitsBeat(t *testing.T) {
	dir := t.TempDir()
	for tool, wantKind := range map[string]string{"Grep": "grep", "Glob": "glob", "Read": "read"} {
		t.Run(tool, func(t *testing.T) {
			withTempBeatLog(t)
			hookInput := fmt.Sprintf(`{"tool_name":%q,"tool_input":{"file_path":"whatever.go"},"cwd":%q}`, tool, dir)
			hookStdin(t, hookInput)
			var out, errBuf bytes.Buffer
			runWithStdin([]string{"edit-hook", "--pre", "--url=http://127.0.0.1:1"}, nil, &out, &errBuf)

			beats := readBeatsForTest(t)
			if len(beats) != 1 || beats[0].Channel != "discovery" || beats[0].Kind != wantKind {
				t.Fatalf("beats = %+v, want one discovery/%s beat", beats, wantKind)
			}
		})
	}
}

// TestCmdEditHookPre_BashSearchCommand_EmitsShellSearchAndStillGuards
// proves the shell-search classification runs ALONGSIDE (not instead of)
// the existing command-guard matching — both are local-only, so nothing
// about O-9's zero-daemon-call contract changes.
func TestCmdEditHookPre_BashSearchCommand_EmitsShellSearchAndStillGuards(t *testing.T) {
	withTempBeatLog(t)
	dir := t.TempDir()
	writeGuardCue(t, dir, `guards: [
	{
		note: "grep carefully here"
		command_scopes: ["grep*"]
	},
]`)
	hookInput := fmt.Sprintf(`{"tool_name":"Bash","tool_input":{"command":"grep -rn foo ."},"cwd":%q}`, dir)
	hookStdin(t, hookInput)
	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--pre", "--url=http://127.0.0.1:1"}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if out.Len() == 0 {
		t.Error("expected the command guard to still fire")
	}

	beats := readBeatsForTest(t)
	found := false
	for _, b := range beats {
		if b.Channel == "discovery" && b.Kind == "shell-search" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a discovery/shell-search beat among %+v", beats)
	}
}
