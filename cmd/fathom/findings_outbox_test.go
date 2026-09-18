package main

// findings_outbox_test.go — FL-22 (docs/vision/30-findings-loop.md §3.7):
// the session-hook outbox sync. findingsImportRunner is injected in every
// test — never a real fathom binary exec'd.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/statedir"
)

// withFindingsImportRunner swaps findingsImportRunner for the duration of
// the test and restores it on cleanup.
func withFindingsImportRunner(t *testing.T, fn func(exe, store, service, subject, outbox string) ([]byte, int, error)) {
	t.Helper()
	orig := findingsImportRunner
	findingsImportRunner = fn
	t.Cleanup(func() { findingsImportRunner = orig })
}

// indexedDaemon serves list_services naming svc for dir's basename —
// enough for indexedServiceFor to resolve the repo.
func indexedDaemon(t *testing.T, dir, svc string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.Write([]byte("ok"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"services":[{"service":"` + svc + `","symbol_count":1,"languages":["go"],"path":"` + dir + `"}]}}`))
	}))
}

func writeFindingsConfig(t *testing.T, dir, outboxRel, subject, store string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "[daemon]\nstore = " + tomlQuote(store) + "\n\n[findings]\noutbox = " + tomlQuote(outboxRel) + "\nsubject = " + tomlQuote(subject) + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".fathom", "config.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func tomlQuote(s string) string { return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"` }

func TestSyncFindingsOutbox_NoOutboxConfigured_Silent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(statedir.EnvOverride, filepath.Join(dir, "state"))
	if got := syncFindingsOutbox("http://unused.invalid", dir); got != "" {
		t.Errorf("no [findings] section must render nothing, got %q", got)
	}
}

func TestSyncFindingsOutbox_ImportsAndAdvancesCursor(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, "state")
	t.Setenv(statedir.EnvOverride, stateDir)
	writeFindingsConfig(t, dir, "outbox.json", "org/repo/comp", filepath.Join(dir, "store.sqlite"))
	if err := os.WriteFile(filepath.Join(dir, "outbox.json"), []byte(`{"findings":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := indexedDaemon(t, dir, filepath.Base(dir))
	defer srv.Close()

	var gotArgs []string
	withFindingsImportRunner(t, func(exe, store, service, subject, outbox string) ([]byte, int, error) {
		gotArgs = []string{store, service, subject, outbox}
		return []byte(`{"schema_version":1,"delivered":3,"duplicate":1,"stale":0,"rejected":[]}`), 0, nil
	})

	got := syncFindingsOutbox(srv.URL, dir)
	want := "findings outbox: imported 3 (duplicate 1, stale 0, rejected 0)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if gotArgs[1] != strings.ToLower(filepath.Base(dir)) {
		t.Errorf("service arg = %q, want %q", gotArgs[1], filepath.Base(dir))
	}
	if gotArgs[2] != "org/repo/comp" {
		t.Errorf("subject arg = %q, want org/repo/comp", gotArgs[2])
	}

	cursor, err := os.ReadFile(filepath.Join(statedir.Resolve(dir), findingsOutboxCursorFile))
	if err != nil {
		t.Fatalf("cursor was not written: %v", err)
	}
	if len(strings.TrimSpace(string(cursor))) != 64 {
		t.Errorf("cursor = %q, want a hex sha256 digest", cursor)
	}

	// A second call with the SAME outbox content must not re-run the
	// import (never more than one attempt per session — the digest cursor
	// is the gate).
	withFindingsImportRunner(t, func(exe, store, service, subject, outbox string) ([]byte, int, error) {
		t.Fatal("findings import must not run again for an unchanged outbox")
		return nil, 0, nil
	})
	if got := syncFindingsOutbox(srv.URL, dir); got != "" {
		t.Errorf("unchanged outbox must render nothing on the second call, got %q", got)
	}
}

func TestSyncFindingsOutbox_RejectionStillAdvancesCursor(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(statedir.EnvOverride, filepath.Join(dir, "state"))
	writeFindingsConfig(t, dir, "outbox.json", "org/repo/comp", filepath.Join(dir, "store.sqlite"))
	if err := os.WriteFile(filepath.Join(dir, "outbox.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := indexedDaemon(t, dir, filepath.Base(dir))
	defer srv.Close()

	withFindingsImportRunner(t, func(exe, store, service, subject, outbox string) ([]byte, int, error) {
		return []byte(`{"delivered":1,"duplicate":0,"stale":0,"rejected":[{"index":0,"finding_id":"f1","reason":"bad"}]}`), 1, nil
	})
	got := syncFindingsOutbox(srv.URL, dir)
	if !strings.Contains(got, "rejected 1") {
		t.Errorf("got %q, want it to name the rejection count", got)
	}
	if _, err := os.ReadFile(filepath.Join(statedir.Resolve(dir), findingsOutboxCursorFile)); err != nil {
		t.Errorf("a rejection (exit 1) must still advance the cursor: %v", err)
	}
}

func TestSyncFindingsOutbox_TimeoutDoesNotAdvanceCursor(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(statedir.EnvOverride, filepath.Join(dir, "state"))
	writeFindingsConfig(t, dir, "outbox.json", "org/repo/comp", filepath.Join(dir, "store.sqlite"))
	if err := os.WriteFile(filepath.Join(dir, "outbox.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := indexedDaemon(t, dir, filepath.Base(dir))
	defer srv.Close()

	withFindingsImportRunner(t, func(exe, store, service, subject, outbox string) ([]byte, int, error) {
		return nil, -1, context.DeadlineExceeded
	})
	got := syncFindingsOutbox(srv.URL, dir)
	if !strings.Contains(got, "sync failed") {
		t.Errorf("got %q, want a sync-failed line", got)
	}
	if _, err := os.ReadFile(filepath.Join(statedir.Resolve(dir), findingsOutboxCursorFile)); err == nil {
		t.Error("a timeout must NOT advance the cursor")
	}

	// Retried on the next call (cursor untouched) — a different runner
	// this time proves the retry actually happens.
	withFindingsImportRunner(t, func(exe, store, service, subject, outbox string) ([]byte, int, error) {
		return []byte(`{"delivered":1,"duplicate":0,"stale":0,"rejected":[]}`), 0, nil
	})
	got2 := syncFindingsOutbox(srv.URL, dir)
	if !strings.Contains(got2, "imported 1") {
		t.Errorf("got %q, want the retried import to succeed", got2)
	}
}

func TestSyncFindingsOutbox_MissingOutboxFile_Silent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(statedir.EnvOverride, filepath.Join(dir, "state"))
	writeFindingsConfig(t, dir, "does-not-exist.json", "org/repo/comp", filepath.Join(dir, "store.sqlite"))
	if got := syncFindingsOutbox("http://unused.invalid", dir); got != "" {
		t.Errorf("a missing outbox file must render nothing, got %q", got)
	}
}

func TestSyncFindingsOutbox_NoStoreConfigured(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(statedir.EnvOverride, filepath.Join(dir, "state"))
	if err := os.MkdirAll(filepath.Join(dir, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".fathom", "config.toml"), []byte("[findings]\noutbox = \"outbox.json\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "outbox.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := syncFindingsOutbox("http://unused.invalid", dir)
	if !strings.Contains(got, "sync failed") || !strings.Contains(got, "store") {
		t.Errorf("got %q, want a sync-failed line naming the missing store", got)
	}
}

// TestFindingsImportRunner_DefaultReportsRealExitCode proves the DEFAULT
// findingsImportRunner correctly plumbs a real subprocess's exit code and
// stdout through exec.ExitError — against `sh`, never the fathom binary
// itself. sh's argv here is exactly what the runner always builds
// ("findings" "import" --store ... --service ... --subject ... --file ...
// --json), so this drives it through a tiny script that echoes JSON and
// exits 1, the same shape a real `fathom findings import` rejection takes.
func TestFindingsImportRunner_DefaultReportsRealExitCode(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh on PATH")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-fathom.sh")
	body := "#!/bin/sh\necho '{\"delivered\":1}'\nexit 1\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	stdout, code, err := findingsImportRunner(script, "store", "svc", "subj", "outbox")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if !strings.Contains(string(stdout), `"delivered":1`) {
		t.Errorf("stdout = %q, want it to carry the script's own output", stdout)
	}
}
