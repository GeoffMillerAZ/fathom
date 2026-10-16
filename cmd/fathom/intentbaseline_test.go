package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	heartbeat "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/heartbeat"
)

// EP-24 / K5: "the session acted on the attention line" is derived from a
// BEAT, never from anything the transcript says. An agent announcing it will
// check the corpus is narrative, and narrative is never evidence (law 14).
func TestIntentLineActedDerivedFromBeats(t *testing.T) {
	const sid = "s-1"
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	beat := func(channel string) heartbeat.Beat {
		return heartbeat.Beat{Channel: channel, At: now, Meta: map[string]string{"session": sid}}
	}

	cases := []struct {
		name  string
		beats []heartbeat.Beat
		want  bool
	}{
		{"no beats at all", nil, false},
		{"only unrelated channels", []heartbeat.Beat{beat("discovery"), beat("payload-cost")}, false},
		{"the verb ran", []heartbeat.Beat{beat("intent-cli")}, true},
		{"doctor ran", []heartbeat.Beat{beat("doctor")}, true},
		{"another session's verb does not count", []heartbeat.Beat{{
			Channel: "intent-cli", At: now, Meta: map[string]string{"session": "someone-else"},
		}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, got := sumSessionBeats(tc.beats, sid); got != tc.want {
				t.Fatalf("intent_acted = %v, want %v", got, tc.want)
			}
		})
	}
}

// The session stamp carries the cohort label: did this session SEE the line?
// Without it K5's question ("did seeing it change anything?") is unanswerable.
func TestSessionStampCarriesIntentLine(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FATHOM_SESSION_STAMPS", dir)
	writeSessionStamp(sessionStamp{SchemaVersion: 1, SessionID: "abc", IntentLine: true, At: "now"})
	if got := readSessionStamp("abc"); !got.IntentLine {
		t.Fatal("intent_line did not survive the stamp round-trip")
	}
	writeSessionStamp(sessionStamp{SchemaVersion: 1, SessionID: "def", At: "now"})
	if got := readSessionStamp("def"); got.IntentLine {
		t.Fatal("a session with no attention line must not be labeled as having seen one")
	}
}

// K1's baseline is a MEASUREMENT or a stated reason — never a silent zero.
func TestIntentBaselineMeasuresOrExplains(t *testing.T) {
	now := time.Now()

	if _, _, note := intentBaseline(t.TempDir(), "", now, 90); !strings.Contains(note, "no decisions home") {
		t.Fatalf("a repo with no decisions home must say so, got %q", note)
	}

	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	write := func(body string) {
		if err := os.WriteFile(filepath.Join(root, "ADR.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// One record arrives proposed, then is accepted — a measurable answer.
	write("# Decisions\n\n## ADR-001 — a thing\n\n**Status.** Proposed.\n")
	git("add", "ADR.md")
	git("commit", "-qm", "one")
	write("# Decisions\n\n## ADR-001 — a thing\n\n**Status.** Accepted.\n")
	git("add", "ADR.md")
	git("commit", "-qm", "two")

	days, _, note := intentBaseline(root, "ADR.md", now, 90)
	if len(days) != 1 {
		t.Fatalf("one answered record expected, got %d (%s)", len(days), note)
	}
	if !strings.Contains(note, "1 record(s) answered") {
		t.Fatalf("the note must name what was measured, got %q", note)
	}

	// A window that excludes every commit reports the absence, not a zero.
	if d, _, n := intentBaseline(root, "ADR.md", now.AddDate(1, 0, 0), 90); len(d) != 0 || !strings.Contains(n, "lookback") {
		t.Fatalf("an empty window must be named: days=%v note=%q", d, n)
	}
}
