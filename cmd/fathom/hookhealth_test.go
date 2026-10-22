package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

func TestHookHealthTransitionsAndRecovery(t *testing.T) {
	root, now := readinessEnv(t)
	cap := harness.CapObligationsPostEdit
	old := now.Add(-24 * time.Hour)
	if err := updateHookHealth(root, harness.ClaudeCode, cap, true, "", old); err != nil {
		t.Fatal(err)
	}
	if err := updateHookHealth(root, harness.ClaudeCode, cap, false, "not-ready:no-instructions", now); err != nil {
		t.Fatal(err)
	}
	s, err := readHookHealth(root, harness.ClaudeCode, now)
	stamp := s.Channels[cap]
	if err != nil || stamp.LastState != "not-ready:no-instructions" || !stamp.ChangedAt.Equal(now) || !stamp.LastEmitAt.Equal(old) {
		t.Fatalf("stamp=%+v err=%v", stamp, err)
	}
	if err := updateHookHealth(root, harness.ClaudeCode, cap, true, "", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	s, err = readHookHealth(root, harness.ClaudeCode, now.Add(time.Minute))
	stamp = s.Channels[cap]
	if err != nil || stamp.LastState != "ready" || !stamp.ChangedAt.Equal(now.Add(time.Minute)) || !stamp.LastEmitAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("recovery=%+v err=%v", stamp, err)
	}
	other, err := readHookHealth(t.TempDir(), harness.ClaudeCode, now)
	if err != nil || len(other.Channels) != 0 {
		t.Fatalf("cross-root state=%+v err=%v", other, err)
	}
}

func TestHookHealthInactiveLineAndOffByChoice(t *testing.T) {
	root, now := readinessEnv(t)
	if err := os.WriteFile(filepath.Join(os.Getenv("FATHOM_HOME"), "config.toml"), []byte("[readouts]\nmode = 'rich'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	line := inactiveFeaturesLine(root, harness.ClaudeCode, now)
	want := "1 feature inactive: readouts"
	if line != want {
		t.Fatalf("line = %q, want %q", line, want)
	}
	if err := os.WriteFile(filepath.Join(os.Getenv("FATHOM_HOME"), "config.toml"), []byte("[readouts]\nmode = 'off'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := inactiveFeaturesLine(root, harness.ClaudeCode, now); strings.Contains(got, "readouts") {
		t.Fatalf("off-by-choice channel: %q", got)
	}
}

func TestHookHealthBoundedAndFailOpen(t *testing.T) {
	root, now := readinessEnv(t)
	path := hookHealthPath(root, harness.ClaudeCode)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"bad json", strings.Repeat("x", hookDedupeMaxBytes+1)} {
		if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		if line := inactiveFeaturesLine(root, harness.ClaudeCode, now); line != "" {
			t.Fatalf("state error must fail open: %q", line)
		}
		if allowed := channelReady(harness.CapSubagentPrime, readinessInput{root: root, harnessID: harness.ClaudeCode, now: now}, "subagent-hook", "parent"); !allowed {
			t.Fatal("stamp error withheld today's payload")
		}
	}
}

func TestHookHealthPrimerGatingAndCap(t *testing.T) {
	root, now := readinessEnv(t)
	if err := os.WriteFile(filepath.Join(os.Getenv("FATHOM_HOME"), "config.toml"), []byte("[readouts]\nmode = 'rich'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	isolateHookRun(t)
	clock := nowFunc
	nowFunc = func() time.Time { return now }
	t.Cleanup(func() { nowFunc = clock })
	transport := http.DefaultTransport
	http.DefaultTransport = testRunRoundTrip(func(r *http.Request) (*http.Response, error) {
		body := "ok"
		if r.URL.Path != "/healthz" {
			body = `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"services\":[]}"}]}}`
		}
		if r.URL.Path == "/identity" {
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = transport })
	for _, source := range []string{"startup", "clear", "", "resume", "compact"} {
		var out, errb bytes.Buffer
		input, _ := json.Marshal(map[string]string{"source": source})
		if code := cmdSessionHook([]string{"--dir", root, "--url", "http://daemon.invalid"}, bytes.NewReader(input), &out, &errb); code != 0 {
			t.Fatalf("code=%d stderr=%s", code, &errb)
		}
		var envelope struct {
			Hook struct {
				Text string `json:"additionalContext"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		want := 1
		if source == "resume" || source == "compact" {
			want = 0
		}
		if got := strings.Count(envelope.Hook.Text, "feature inactive:"); got != want {
			t.Fatalf("%q: got %d flags: %q", source, got, envelope.Hook.Text)
		}
		rows := readBeats(t, beatLogPath(), "payload-cost")
		if want == 1 {
			last := rows[len(rows)-1]
			if last.Meta["inactive_features_bytes"] != strconv.Itoa(len(inactiveFeaturesLine(root, harness.ClaudeCode, now))) {
				t.Fatalf("inactive bytes unmeasured: %+v", last)
			}
		}
		if len(envelope.Hook.Text) > sessionPrimerBudget {
			t.Fatalf("primer over cap: %d", len(envelope.Hook.Text))
		}
	}
	line := inactiveFeaturesLine(root, harness.ClaudeCode, now)
	primer, _, _ := composePrimer(strings.Repeat("identity ", 400), root, now, unreadRoots, line)
	if len(primer) > sessionPrimerBudget || !strings.Contains(primer, "truncated") {
		t.Fatalf("unnamed primer cut: %q", primer)
	}
}

func TestHookHealthInvocationAndComponentDelivery(t *testing.T) {
	root, now := readinessEnv(t)
	clock := nowFunc
	nowFunc = func() time.Time { return now }
	t.Cleanup(func() { nowFunc = clock })
	var out bytes.Buffer
	w, finish := beginAdviceHealth(root, harness.ClaudeCode, harness.CapObligationsPostEdit, &out)
	noteAdviceComponent(harness.CapClaimGate, true)
	if _, err := io.WriteString(w, "advice"); err != nil {
		t.Fatal(err)
	}
	finish()
	s, err := readHookHealth(root, harness.ClaudeCode, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, cap := range []harness.Capability{harness.CapObligationsPostEdit, harness.CapClaimGate} {
		if s.Channels[cap].LastState != "ready" || !s.Channels[cap].LastEmitAt.Equal(now) {
			t.Fatalf("%s: %+v", cap, s.Channels[cap])
		}
	}
}

func TestHookHealthLocalReadinessRecoversWithoutAnotherInvocation(t *testing.T) {
	root, now := readinessEnv(t)
	if err := updateHookHealth(root, harness.ClaudeCode, harness.CapSubagentPrime, false, "not-ready:no-maps", now); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(threadItem{ID: "map:example", Kind: "map", FiledAt: now.Format(time.RFC3339)})
	if err := os.WriteFile(threadsPath(), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if line := inactiveFeaturesLine(root, harness.ClaudeCode, now); line != "" {
		t.Fatalf("recovered channel flagged: %q", line)
	}
}
