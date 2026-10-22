package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/harness"
)

// All identities, repositories, logs and state are generated and isolated.
func cbHookEnv(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("FATHOM_HOME", filepath.Join(t.TempDir(), "home"))
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
	t.Setenv("FATHOM_SESSION_STAMPS", filepath.Join(t.TempDir(), "session-stamps"))
	t.Setenv("CLAUDE_PROJECT_DIR", root)
	t.Setenv("FATHOM_DISABLED", "")
	for _, session := range []string{"session-a", "session-b"} {
		writeSessionStamp(sessionStamp{SessionID: session, Source: "startup", At: "2026-10-22T12:00:00Z"})
	}
	return root
}

func cbRule(rule string) map[string]any {
	return map[string]any{"rule": rule, "src": "AGENTS.md#L1-L1", "tier": "doc-claimed"}
}

func cbPost(t *testing.T, url, root, file, session string) string {
	t.Helper()
	raw := cbEdit(t, url, root, file, session, harness.ClaudeCode, &bytes.Buffer{})
	if raw == "" {
		return ""
	}
	var out struct {
		HookSpecificOutput struct {
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	return out.HookSpecificOutput.AdditionalContext
}

func cbDedupePath(session string) string {
	digest := sha256.Sum256([]byte(session))
	return filepath.Join(fathomhome.FathomHome(), "hook-dedupe", hex.EncodeToString(digest[:])+".json")
}

func TestPostEditObligationsDedupeWithinSession(t *testing.T) {
	root := cbHookEnv(t)
	file := writeEditedFile(t, root, "pkg/a.go", "package pkg\n")
	fake := &specRefsFakeDaemon{t: t, obligationsOut: map[string]any{"rules": []any{cbRule("Sort outputs.")}}}
	srv := fake.start()
	defer srv.Close()
	first := cbPost(t, srv.URL, root, file, "session-a")
	if !strings.Contains(first, "Sort outputs.") {
		t.Fatalf("first touch lost rules: %q", first)
	}
	if got := cbPost(t, srv.URL, root, file, "session-a"); got != "" {
		t.Fatalf("unchanged second touch emitted %q", got)
	}
	if got := cbPost(t, srv.URL, root, file, "session-b"); got != first {
		t.Fatalf("new session first touch = %q, want %q", got, first)
	}
	other := writeEditedFile(t, root, "pkg/b.go", "package pkg\n")
	if got := cbPost(t, srv.URL, root, other, "session-a"); strings.Contains(got, "Sort outputs.") || !strings.HasSuffix(got, "(1 rule already shown in this session)") {
		t.Fatalf("new file repeated a delivered rule: %q", got)
	}
	for i := 0; i < 2; i++ {
		if got := cbPost(t, srv.URL, root, file, ""); got != first {
			t.Fatalf("missing session must fail open: %q", got)
		}
	}
	raw, err := os.ReadFile(os.Getenv("FATHOM_HOOKS_LOG"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range bytes.Split(raw, []byte("\n")) {
		var row struct {
			Channel, Kind string
			Meta          map[string]string
		}
		if json.Unmarshal(line, &row) == nil && row.Channel == "hook-dedupe" && row.Kind == "suppressed" && row.Meta["session"] == "session-a" && row.Meta["suppressed_bytes"] == fmt.Sprint(len(first)) && row.Meta["bytes"] == "0" {
			found = true
		}
	}
	if !found {
		t.Fatal("no dedupe beat with session, zero emitted bytes and suppressed byte count")
	}
}

func TestPostEditObligationsReemitAfterCompaction(t *testing.T) {
	for _, source := range []string{"compact", "resume"} {
		t.Run(source, func(t *testing.T) {
			root := cbHookEnv(t)
			file := writeEditedFile(t, root, "pkg/a.go", "package pkg\n")
			fake := &specRefsFakeDaemon{t: t, obligationsOut: map[string]any{"rules": []any{cbRule("Sort outputs.")}}}
			srv := fake.start()
			defer srv.Close()
			first := cbPost(t, srv.URL, root, file, "session-a")
			cbPost(t, srv.URL, root, file, "session-a")
			var out, stderr bytes.Buffer
			input := fmt.Sprintf(`{"session_id":"session-a","source":%q,"cwd":%q}`, source, root)
			if code := cmdSessionHook([]string{"--url", srv.URL, "--dir", root}, strings.NewReader(input), &out, &stderr); code != 0 {
				t.Fatalf("SessionStart = %d: %s", code, stderr.String())
			}
			if got := cbPost(t, srv.URL, root, file, "session-a"); got != first || got == "" {
				t.Fatalf("after %s: %q, want original %q", source, got, first)
			}
			if got := cbPost(t, srv.URL, root, file, "session-a"); got != "" {
				t.Fatalf("reset did not start a new dedupe context: %q", got)
			}
		})
	}
}

func TestPostEditObligationsEmitWhenRulesChange(t *testing.T) {
	root := cbHookEnv(t)
	file := writeEditedFile(t, root, "pkg/a.go", "package pkg\n")
	fake := &specRefsFakeDaemon{t: t, obligationsOut: map[string]any{"rules": []any{cbRule("Sort outputs.")}}}
	srv := fake.start()
	defer srv.Close()
	first := cbPost(t, srv.URL, root, file, "session-a")
	cbPost(t, srv.URL, root, file, "session-a")
	// The prior synchronous request is complete; replace the response before the next request.
	changedFake := &specRefsFakeDaemon{t: t, obligationsOut: map[string]any{"rules": []any{cbRule("Sort outputs."), cbRule("Inject clocks.")}}}
	changedSrv := changedFake.start()
	defer changedSrv.Close()
	changed := cbPost(t, changedSrv.URL, root, file, "session-a")
	if strings.Contains(changed, "Sort outputs.") || !strings.Contains(changed, "Inject clocks.") || !strings.HasSuffix(changed, "(1 rule already shown in this session)") {
		t.Fatalf("changed rules must emit only new rules and the count: %q", changed)
	}
	if got := cbPost(t, changedSrv.URL, root, file, "session-a"); got != "" {
		t.Fatalf("changed rules repeated: %q", got)
	}
	if !strings.Contains(first, "Sort outputs.") {
		t.Fatalf("first delivery lost rules: %q", first)
	}
	if got := cbPost(t, srv.URL, root, file, "session-a"); strings.Contains(got, "Sort outputs.") || !strings.HasSuffix(got, "(1 rule already shown in this session)") {
		t.Fatalf("older rule set repeated delivered rules: %q", got)
	}
}

func cbCwd(t *testing.T, url, root, session string) string {
	t.Helper()
	var out, stderr bytes.Buffer
	input := fmt.Sprintf(`{"session_id":%q,"cwd":%q,"path":%q}`, session, root, root)
	if code := cmdSessionHook([]string{"--event", "cwd-changed", "--url", url, "--dir", root}, strings.NewReader(input), &out, &stderr); code != 0 {
		t.Fatalf("cwd hook = %d: %s", code, stderr.String())
	}
	return out.String()
}

func TestCwdChangedDedupeSameRepo(t *testing.T) {
	root := cbHookEnv(t)
	gitInitTempRepo(t, root)
	fake := &specRefsFakeDaemon{t: t, obligationsOut: map[string]any{}}
	srv := fake.start()
	defer srv.Close()
	first := cbCwd(t, srv.URL, root, "session-a")
	if first == "" {
		t.Fatal("first cwd touch emitted nothing")
	}
	if got := cbCwd(t, srv.URL, root, "session-a"); got != "" {
		t.Fatalf("same root repeated: %q", got)
	}
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := cbCwd(t, srv.URL, sub, "session-a"); got != "" {
		t.Fatalf("subdirectory of the same repository repeated: %q", got)
	}
	other := t.TempDir()
	gitInitTempRepo(t, other)
	if got := cbCwd(t, srv.URL, other, "session-a"); got == "" {
		t.Fatal("new repo must emit")
	}
	if got := cbCwd(t, srv.URL, root, "session-a"); got != first {
		t.Fatalf("return to repo must re-emit: %q", got)
	}
	if got := cbCwd(t, srv.URL, root, "session-b"); got != first {
		t.Fatalf("new session first cwd = %q, want %q", got, first)
	}
}

func TestDedupeStateErrorFailsOpen(t *testing.T) {
	for _, kind := range []string{"malformed", "unwritable", "symlink", "oversized", "readonly", "incomplete"} {
		t.Run(kind, func(t *testing.T) {
			root := cbHookEnv(t)
			file := writeEditedFile(t, root, "pkg/a.go", "package pkg\n")
			fake := &specRefsFakeDaemon{t: t, obligationsOut: map[string]any{"rules": []any{cbRule("Sort outputs.")}}}
			srv := fake.start()
			defer srv.Close()
			first := cbPost(t, srv.URL, root, file, "session-a")
			path := cbDedupePath("session-a")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "incomplete":
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var record map[string]any
				if err := json.Unmarshal(raw, &record); err != nil {
					t.Fatal(err)
				}
				delete(record, "epoch")
				raw, err = json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, raw, 0o600); err != nil {
					t.Fatal(err)
				}
			case "readonly":
				if err := os.Chmod(path, 0o400); err != nil {
					t.Fatal(err)
				}
			case "unwritable":
				if err := os.RemoveAll(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				target := filepath.Join(t.TempDir(), "target")
				if err := os.WriteFile(target, []byte("do not overwrite"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
				defer func() {
					raw, _ := os.ReadFile(target)
					if string(raw) != "do not overwrite" {
						t.Errorf("followed symlink: %q", raw)
					}
				}()
			default:
				data := "invalid JSON"
				if kind == "oversized" {
					data = strings.Repeat("x", (256<<10)+1)
				}
				if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < 2; i++ {
				if got := cbPost(t, srv.URL, root, file, "session-a"); got != first {
					t.Fatalf("%s must emit original: %q", kind, got)
				}
			}
		})
	}
}

func TestDedupeOffSwitch(t *testing.T) {
	root := cbHookEnv(t)
	file := writeEditedFile(t, root, "pkg/a.go", "package pkg\n")
	writeEditedFile(t, root, ".fathom/config.toml", "[hooks]\ndedupe = false\n")
	fake := &specRefsFakeDaemon{t: t, obligationsOut: map[string]any{"rules": []any{cbRule("Sort outputs.")}}}
	srv := fake.start()
	defer srv.Close()
	first := cbPost(t, srv.URL, root, file, "session-a")
	for i := 0; i < 2; i++ {
		if got := cbPost(t, srv.URL, root, file, "session-a"); got != first || got == "" {
			t.Fatalf("off switch must emit in full: %q", got)
		}
	}
	cwd := cbCwd(t, srv.URL, root, "session-a")
	if got := cbCwd(t, srv.URL, root, "session-a"); got != cwd || got == "" {
		t.Fatalf("cwd off switch: %q, want %q", got, cwd)
	}
	if _, err := os.Stat(cbDedupePath("session-a")); !os.IsNotExist(err) {
		t.Fatalf("off switch wrote state: %v", err)
	}
}

func TestDedupeGCRetention(t *testing.T) {
	cbHookEnv(t)
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	old, fresh := cbDedupePath("old"), cbDedupePath("fresh")
	gcTouch(t, old, 10, now.Add(-8*24*time.Hour))
	gcTouch(t, fresh, 10, now.Add(-6*24*time.Hour))
	dry := runGC(gcOptions{Home: t.TempDir(), Now: now})
	found := false
	for _, item := range dry.Items {
		if item.Path == old {
			found = true
			if item.Class != "session-stamps" {
				t.Errorf("class = %q, want existing session-stamps class", item.Class)
			}
		}
	}
	if !found {
		t.Fatal("GC omitted dedupe state older than seven days")
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatalf("dry run deleted state: %v", err)
	}
	runGC(gcOptions{Home: t.TempDir(), Now: now, Apply: true})
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old state remains: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("fresh state removed: %v", err)
	}
}

func TestContextBillSubagentMeasurement(t *testing.T) {
	root := cbHookEnv(t)
	// The parent stamp supplies identity only; a SubagentStart envelope does
	// not prove that the child received SessionStart or the project's stanza.
	writeSessionStamp(sessionStamp{SessionID: "parent", Cwd: root, Identity: &identityResult{Schema: 1, Service: "sample", Symbols: 17}})
	fake := &specRefsFakeDaemon{t: t, obligationsOut: map[string]any{}}
	srv := fake.start()
	defer srv.Close()
	first := subagentPrimer(srv.URL, root, "parent", "")
	second := subagentPrimer(srv.URL, root, "parent", "")
	if first == "" || first != second {
		t.Fatalf("each fresh subagent needs the same primer: %q / %q", first, second)
	}
	if len(first) > subagentPrimerBudget || !strings.Contains(first, "sample") || !strings.Contains(first, "Opening moves") {
		t.Fatalf("primer lost identity/opening moves or exceeds cap: %q", first)
	}
	t.Logf("CB-3 measurement: generated sample service (17 symbols), no discoveries/worktree/outbox: before=%d bytes after=%d bytes; no cut", len(first), len(second))
}

func TestDedupeUnprimedSessionFailsOpen(t *testing.T) {
	cbHookEnv(t)
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		if cbSeen("unprimed", "post-edit", "file", "rules", now, false) {
			t.Fatal("unprimed context suppressed guidance")
		}
	}
	if _, err := os.Stat(cbDedupePath("unprimed")); !os.IsNotExist(err) {
		t.Fatalf("unprimed hook wrote dedupe state: %v", err)
	}
}

func TestDedupeStateContentionFailsOpen(t *testing.T) {
	cbHookEnv(t)
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	if cbSeen("session-a", "post-edit", "file", "rules", now, false) {
		t.Fatal("first touch suppressed")
	}
	f, err := os.OpenFile(cbDedupePath("session-a"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	if cbSeen("session-a", "post-edit", "file", "rules", now, false) {
		t.Error("contending hook suppressed guidance")
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	if !cbSeen("session-a", "post-edit", "file", "rules", now, false) {
		t.Fatal("unlocked record no longer dedupes")
	}
	entries, err := os.ReadDir(filepath.Dir(cbDedupePath("session-a")))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("one session produced %d files", len(entries))
	}
}

func TestDedupeStateExpiryAndCapacity(t *testing.T) {
	cbHookEnv(t)
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	cbSeen("session-a", "post-edit", "file", "rules", now, false)
	if cbSeen("session-a", "post-edit", "file", "rules", now.Add(8*24*time.Hour), false) {
		t.Fatal("expired state suppressed guidance")
	}
	if !cbSeen("session-a", "post-edit", "file", "rules", now.Add(8*24*time.Hour), false) {
		t.Fatal("first touch after expiry not recorded")
	}
	path := cbDedupePath("session-a")
	var st hookDedupeState
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatal(err)
	}
	st.Digests = map[string]hookDedupeEntry{}
	// Fill with real hashed keys, digests, timestamps and counters. Stop
	// at the byte cap, which can bind before the entry-count cap.
	for i := 0; i < hookDedupeMaxEntries; i++ {
		key := hookDigest(fmt.Sprint(i))
		st.Digests[key] = hookDedupeEntry{Digest: hookDigest("rules"), EmittedAt: now, Suppressions: hookDedupeMaxSuppressions}
		raw, err := json.Marshal(st)
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) > hookDedupeMaxBytes {
			delete(st.Digests, key)
			break
		}
	}
	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if cbSeen("session-a", "post-edit", "new-file", "rules", st.At, false) {
		t.Fatal("full state suppressed a new file")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, raw) {
		t.Fatal("full state exceeded its capacity")
	}
}

func TestPostEditObligationsDedupeIsolatesSubagents(t *testing.T) {
	root := cbHookEnv(t)
	file := writeEditedFile(t, root, "pkg/a.go", "package pkg\n")
	fake := &specRefsFakeDaemon{t: t, obligationsOut: map[string]any{"rules": []any{cbRule("Sort outputs.")}}}
	srv := fake.start()
	defer srv.Close()
	run := func(agent string) string {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"session_id": "session-a", "agent_id": agent, "cwd": root, "tool_name": "Edit", "tool_input": map[string]any{"file_path": file, "new_string": "package pkg"}})
		hookStdin(t, string(raw))
		var out, errb bytes.Buffer
		if code := cmdEditHook([]string{"--url", srv.URL}, &out, &errb); code != 0 {
			t.Fatalf("edit hook: %d, %s", code, errb.String())
		}
		return out.String()
	}
	parent := run("")
	if !strings.Contains(parent, "Sort outputs.") {
		t.Fatalf("parent first touch: %q", parent)
	}
	if got := run(""); got != "" {
		t.Fatalf("parent repeated rules: %q", got)
	}
	for _, agent := range []string{"agent-a", "agent-b"} {
		if got := run(agent); got != parent {
			t.Fatalf("%s first touch lost rules seen only by parent: %q", agent, got)
		}
		if got := run(agent); got != "" {
			t.Fatalf("%s repeated rules: %q", agent, got)
		}
	}
}

func TestCwdChangedDedupePreservesFirstTouchIdentity(t *testing.T) {
	root := cbHookEnv(t)
	t.Setenv("FATHOM_IDENT_NO_CACHE", "1")
	gitInitTempRepo(t, root)
	sub := filepath.Join(root, "nested")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path != "/identity" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		service := "outer"
		path := r.URL.Query().Get("path")
		if path == sub {
			service = "nested"
		}
		_ = json.NewEncoder(w).Encode(identityResult{Schema: 1, Service: service, Root: path, Symbols: 17})
	}))
	defer srv.Close()
	first := cwdChangedPrimer(srv.URL, sub)
	if !strings.Contains(first, "nested") {
		t.Fatalf("fixture did not resolve nested identity: %q", first)
	}
	if got := cbCwdPayload(t, srv.URL, sub, "session-a"); got != first {
		t.Fatalf("dedupe changed first-touch identity: %q, want %q", got, first)
	}
	if got := cbCwdPayload(t, srv.URL, sub, "session-a"); got != "" {
		t.Fatalf("unchanged nested identity repeated: %q", got)
	}
	outer := cbCwdPayload(t, srv.URL, root, "session-a")
	if !strings.Contains(outer, "outer") {
		t.Fatalf("changed service within same git root lost: %q", outer)
	}
}

// cbSeen models a successfully delivered injection for the state-level tests.
func cbSeen(session, channel, target, payload string, now time.Time, reset bool) bool {
	if reset {
		resetHookDedupe(session, "compact", now)
		return false
	}
	d := checkHookInjection(session, channel, target, payload, now)
	defer d.close()
	if !d.suppressed {
		d.commit(now)
	}
	return d.suppressed
}

func cbEdit(t *testing.T, url, root, file, session, id string, output io.Writer) string {
	t.Helper()
	input := map[string]any{"session_id": session, "cwd": root, "tool_name": "Edit", "tool_input": map[string]any{"file_path": file, "new_string": "package pkg"}}
	if id == harness.Codex {
		input["tool_name"] = "apply_patch"
		input["tool_input"] = map[string]any{"command": "*** Begin Patch\n*** Update File: " + file + "\n@@\n-package pkg\n+package pkg\n*** End Patch"}
	}
	raw, _ := json.Marshal(input)
	hookStdin(t, string(raw))
	var stderr bytes.Buffer
	if code := cmdEditHook([]string{"--url", url, "--harness", id}, output, &stderr); code != 0 {
		t.Fatalf("edit hook: %d: %s", code, stderr.String())
	}
	if out, ok := output.(*bytes.Buffer); ok {
		return out.String()
	}
	return ""
}

type cbFailedWriter struct{}

func (cbFailedWriter) Write([]byte) (int, error) { return 0, errors.New("injected write failure") }

type cbEncodeFailure struct{ harness.Codec }

func (cbEncodeFailure) EncodeOutput(harness.Event, harness.HookOutput) ([]byte, error) {
	return nil, errors.New("injected encode failure")
}

func TestPostEditDedupeProvenHarnesses(t *testing.T) {
	for _, id := range []string{harness.ClaudeCode, harness.Codex, harness.GeminiCLI, "copilot", "cursor"} {
		t.Run(id, func(t *testing.T) {
			root := cbHookEnv(t)
			file := writeEditedFile(t, root, "pkg/a.go", "package pkg\n")
			fake := &specRefsFakeDaemon{t: t, obligationsOut: map[string]any{"rules": []any{cbRule("Sort outputs.")}}}
			srv := fake.start()
			defer srv.Close()
			first := cbEdit(t, srv.URL, root, file, "session-a", id, &bytes.Buffer{})
			if !strings.Contains(first, "Sort outputs.") {
				t.Fatalf("first injection missing: %q", first)
			}
			if id == harness.ClaudeCode || id == harness.Codex {
				if got := cbEdit(t, srv.URL, root, file, "session-a", id, &bytes.Buffer{}); got != "" {
					t.Fatalf("unchanged payload repeated: %q", got)
				}
				if _, err := os.Stat(cbDedupePath("session-a")); err != nil {
					t.Fatalf("proven harness did not record delivery: %v", err)
				}
				return
			}
			if got := cbEdit(t, srv.URL, root, file, "session-a", id, &bytes.Buffer{}); got != first {
				t.Fatalf("unproven harness suppressed: %q", got)
			}
			if _, err := os.Stat(cbDedupePath("session-a")); !os.IsNotExist(err) {
				t.Fatalf("unproven harness wrote state: %v", err)
			}
		})
	}
}

func TestPostEditDedupeCommitsAfterStdout(t *testing.T) {
	root := cbHookEnv(t)
	file := writeEditedFile(t, root, "pkg/a.go", "package pkg\n")
	fake := &specRefsFakeDaemon{t: t, obligationsOut: map[string]any{"rules": []any{cbRule("Sort outputs.")}}}
	srv := fake.start()
	defer srv.Close()
	cbEdit(t, srv.URL, root, file, "session-a", harness.ClaudeCode, cbFailedWriter{})
	if got := cbEdit(t, srv.URL, root, file, "session-a", harness.ClaudeCode, &bytes.Buffer{}); !strings.Contains(got, "Sort outputs.") {
		t.Fatalf("failed stdout recorded an emit: %q", got)
	}
	if got := cbEdit(t, srv.URL, root, file, "session-a", harness.ClaudeCode, &bytes.Buffer{}); got != "" {
		t.Fatalf("successful stdout not recorded: %q", got)
	}
}

func TestCwdDedupeCommitsAfterDelivery(t *testing.T) {
	for _, failure := range []string{"encode", "stdout"} {
		t.Run(failure, func(t *testing.T) {
			root := cbHookEnv(t)
			fake := &specRefsFakeDaemon{t: t, obligationsOut: map[string]any{}}
			srv := fake.start()
			defer srv.Close()
			raw := fmt.Sprintf(`{"session_id":"session-a","cwd":%q}`, root)
			codec := harness.ClaudeCodec()
			var output io.Writer = &bytes.Buffer{}
			if failure == "encode" {
				codec = cbEncodeFailure{codec}
			} else {
				output = cbFailedWriter{}
			}
			runCwdChangedEvent(codec, srv.URL, strings.NewReader(raw), output, func(harness.HookInput) string { return root })
			if got := cbCwd(t, srv.URL, root, "session-a"); got == "" {
				t.Fatal("failed delivery recorded cwd emit")
			}
		})
	}
}

func TestDedupeRefreshBackstops(t *testing.T) {
	for _, channel := range []string{"post-edit", "cwd-changed"} {
		t.Run(channel, func(t *testing.T) {
			cbHookEnv(t)
			now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
			cbSeen("session-a", channel, "target", "payload", now, false)
			if !cbSeen("session-a", channel, "target", "payload", now.Add(29*time.Minute), false) {
				t.Fatal("before expiry did not dedupe")
			}
			if cbSeen("session-a", channel, "target", "payload", now.Add(30*time.Minute), false) {
				t.Fatal("30 minute backstop did not re-emit")
			}
			for i := 0; i < 10; i++ {
				if !cbSeen("session-a", channel, "target", "payload", now.Add(30*time.Minute), false) {
					t.Fatalf("suppression %d not recorded", i+1)
				}
			}
			if cbSeen("session-a", channel, "target", "payload", now.Add(30*time.Minute), false) {
				t.Fatal("10 suppression backstop did not re-emit")
			}
		})
	}
}

func TestDedupeEmptyPayloadAndInvalidConfig(t *testing.T) {
	root := cbHookEnv(t)
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	cbSeen("session-a", "post-edit", "file", "", now, false)
	if _, err := os.Stat(cbDedupePath("session-a")); !os.IsNotExist(err) {
		t.Fatalf("empty payload took state: %v", err)
	}
	for _, config := range []string{"[hooks", "[hooks]\ntest_run = \"invalid\"\n"} {
		writeEditedFile(t, root, ".fathom/config.toml", config)
		if hookDedupeEnabled(root) {
			t.Fatalf("invalid config enabled dedupe: %q", config)
		}
	}
}

func TestPreCompactResetsDedupeEveryWiredHarness(t *testing.T) {
	for _, id := range harness.IDs() {
		d, _ := harness.Lookup(id)
		if _, ok := d.Binding(harness.EventPreCompact); !ok {
			continue
		}
		t.Run(id, func(t *testing.T) {
			root := cbHookEnv(t)
			now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
			cbSeen("session-a", "post-edit", "file", "rules", now, false)
			hookStdin(t, fmt.Sprintf(`{"session_id":"session-a","cwd":%q,"trigger":"auto"}`, root))
			var out, errb bytes.Buffer
			if code := cmdPreCompactHook([]string{"--harness", id}, &out, &errb); code != 0 {
				t.Fatalf("compact: %d: %s", code, errb.String())
			}
			if cbSeen("session-a", "post-edit", "file", "rules", now, false) {
				t.Fatal("pre-compact left context suppressed")
			}
		})
	}
}

func TestDedupeInvalidConfigWithoutDedupeKey(t *testing.T) {
	root := cbHookEnv(t)
	for _, config := range []string{"[hooks", "[hooks]\ntest_run = \"invalid\"\n"} {
		writeEditedFile(t, root, ".fathom/config.toml", config)
		if hookDedupeEnabled(root) {
			t.Errorf("invalid config enabled dedupe: %q", config)
		}
	}
}

func cbCwdPayload(t *testing.T, url, root, session string) string {
	t.Helper()
	raw := cbCwd(t, url, root, session)
	if raw == "" {
		return ""
	}
	var out struct {
		SystemMessage      string `json:"systemMessage"`
		HookSpecificOutput struct {
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	if out.SystemMessage != "" {
		return out.SystemMessage
	}
	return out.HookSpecificOutput.AdditionalContext
}

func TestHookDedupeUsesInjectedClock(t *testing.T) {
	root := cbHookEnv(t)
	file := writeEditedFile(t, root, "pkg/a.go", "package pkg\n")
	fake := &specRefsFakeDaemon{t: t, obligationsOut: map[string]any{"rules": []any{cbRule("Sort outputs.")}}}
	srv := fake.start()
	defer srv.Close()
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	old := nowFunc
	nowFunc = func() time.Time { return now }
	t.Cleanup(func() { nowFunc = old })
	for _, channel := range []string{"post-edit", "cwd-changed"} {
		t.Run(channel, func(t *testing.T) {
			emit := func() string {
				if channel == "cwd-changed" {
					return cbCwd(t, srv.URL, root, "session-a")
				}
				return cbPost(t, srv.URL, root, file, "session-a")
			}
			if emit() == "" {
				t.Fatal("missing first injection")
			}
			for i := 0; i < 10; i++ {
				if got := emit(); got != "" {
					t.Fatalf("suppression %d emitted %q", i+1, got)
				}
			}
			if emit() == "" {
				t.Fatal("suppression backstop did not emit")
			}
			if emit() != "" {
				t.Fatal("full emit did not reset backstops")
			}
			now = now.Add(30 * time.Minute)
			if emit() == "" {
				t.Fatal("injected 30-minute backstop did not emit")
			}
		})
	}
}

func TestDedupeCheckWithoutDeliveryLeavesNoEntry(t *testing.T) {
	cbHookEnv(t)
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	for _, channel := range []string{"post-edit", "cwd-changed"} {
		pending := checkHookInjection("session-a", channel, "target", "payload", now)
		if pending.suppressed {
			t.Fatal("first touch suppressed")
		}
		pending.close() // An encode error or a process exiting before stdout.
		if cbSeen("session-a", channel, "target", "payload", now, false) {
			t.Fatal("undelivered payload recorded")
		}
	}
}

func TestDedupeLogSupportsContextBillBar(t *testing.T) {
	root := cbHookEnv(t)
	file := writeEditedFile(t, root, "pkg/a.go", "package pkg\n")
	fake := &specRefsFakeDaemon{t: t, obligationsOut: map[string]any{"rules": []any{cbRule("Sort outputs.")}}}
	srv := fake.start()
	defer srv.Close()
	for _, channel := range []string{"post-edit", "cwd-changed"} {
		t.Run(channel, func(t *testing.T) {
			emit := func() {
				if channel == "cwd-changed" {
					cbCwd(t, srv.URL, root, "session-a")
				} else {
					cbPost(t, srv.URL, root, file, "session-a")
				}
			}
			emit()
			emit()
			if err := os.WriteFile(cbDedupePath("session-a"), []byte("invalid JSON"), 0o600); err != nil {
				t.Fatal(err)
			}
			emit()
			beats, _ := readBeatLog(beatLogPath())
			var rows []map[string]string
			for _, b := range beats {
				if b.Channel == "hook-dedupe" && b.Meta["channel"] == channel {
					rows = append(rows, b.Meta)
				}
			}
			if len(rows) != 3 {
				t.Fatalf("audit rows: %v", rows)
			}
			for i, row := range rows {
				if row["harness"] != harness.ClaudeCode || row["session"] != "session-a" || len(row["key"]) != 64 {
					t.Fatalf("audit join fields missing: %v", row)
				}
				if (row["emitted"] == "false") != (i == 1) {
					t.Fatalf("delivery outcome %d: %v", i, row)
				}
				if i == 1 {
					if row["bytes"] != "0" || row["suppressed_bytes"] != rows[0]["bytes"] || row["first_touch"] != "false" || row["state_error"] != "false" {
						t.Fatalf("suppression accounting: %v", row)
					}
				} else if row["bytes"] == "0" || row["suppressed_bytes"] != "0" {
					t.Fatalf("emission accounting: %v", row)
				}
			}
			if rows[0]["first_touch"] != "true" || rows[2]["state_error"] != "true" {
				t.Fatalf("first-touch/error audit: %v", rows)
			}
			resetHookDedupe("session-a", "compact", nowFunc())
		})
	}
}

func TestPostEditDedupeIncludesTestsToRun(t *testing.T) {
	root := cbHookEnv(t)
	gitInitTempRepo(t, root)
	file := writeEditedFile(t, root, "pkg/a.go", "package pkg\n")
	writeEditedFile(t, root, ".fathom/config.toml", "[hooks]\ntests_to_run = true\n")
	tests := []string{"pkg/pkg.TestFirst"}
	srv := reachDaemon(t, root, &tests, false)
	first := cbPost(t, srv.URL, root, file, "session-a")
	if !strings.Contains(first, "tests to run: TestFirst") {
		t.Fatalf("missing addition: %q", first)
	}
	if got := cbPost(t, srv.URL, root, file, "session-a"); got != "" {
		t.Fatalf("complete injection repeated: %q", got)
	}
	tests = []string{"pkg/pkg.TestSecond"}
	if got := cbPost(t, srv.URL, root, file, "session-a"); !strings.Contains(got, "tests to run: TestSecond") {
		t.Fatalf("changed addition suppressed: %q", got)
	}
}
