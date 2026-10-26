package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

func TestOnceHasOneWinnerUnderConcurrency(t *testing.T) {
	ta := newTestApp(t)
	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ta.once("s1", "advice-x") {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("once fired %d times across concurrent callers, want 1", wins.Load())
	}
}

func TestSessionKeyAndSafeNameStayInsideStateDir(t *testing.T) {
	for _, s := range []string{"../../etc", "", strings.Repeat("a", 500), "a/b\\c"} {
		if k := sessionKey(s); len(k) != 16 || strings.ContainsAny(k, "/\\.") {
			t.Errorf("sessionKey(%q) = %q", s, k)
		}
	}
	for _, s := range []string{"..", ".", "", "a/../../b", "x y"} {
		if n := safeName(s); n == "." || n == ".." || strings.ContainsAny(n, "/\\ ") {
			t.Errorf("safeName(%q) = %q", s, n)
		}
	}
}

func TestPruneRemovesOnlyStaleSessions(t *testing.T) {
	ta := newTestApp(t)
	root := filepath.Join(ta.stateDir(), "sessions")
	stale, fresh := filepath.Join(root, "stale"), filepath.Join(root, "fresh")
	for _, d := range []string{stale, fresh} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	old := fixedNow.Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	ta.pruneDaily(root)
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("stale session dir survived pruning")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Error("fresh session dir was pruned")
	}
	if err := os.MkdirAll(stale, 0o700); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(stale, old, old)
	ta.pruneDaily(root) // same day: the stamp makes this a no-op
	if _, err := os.Stat(stale); err != nil {
		t.Error("pruning ran twice on the same day")
	}
}

func TestLedgerConcurrentAppendsStayWholeLines(t *testing.T) {
	ta := newTestApp(t)
	cfg := builtinPolicy().Ledger
	var wg sync.WaitGroup
	for i := range 64 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ta.appendLedger(cfg, ledgerRecord{Host: hostClaude, Kind: kindSpawn, BriefChars: i, Notes: []string{strings.Repeat("n", 500)}})
		}()
	}
	wg.Wait()
	raw, err := os.ReadFile(ta.ledgerPath())
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 64 {
		t.Fatalf("%d lines, want 64", len(lines))
	}
	for _, l := range lines {
		var r ledgerRecord
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			t.Fatalf("torn ledger line: %v", err)
		}
	}
}

func TestLedgerRotatesAndKeepsNewest(t *testing.T) {
	ta := newTestApp(t)
	cfg := LedgerConfig{Enabled: true, MaxBytes: 200, KeepRotated: 2}
	for i := range 12 {
		now := fixedNow.Add(time.Duration(i) * time.Second)
		ta.now = func() time.Time { return now }
		ta.appendLedger(cfg, ledgerRecord{Host: hostClaude, Kind: kindSpawn, Model: strings.Repeat("m", 100)})
	}
	files := ta.ledgerFiles()
	if len(files) != 3 { // two rotated plus the active file
		t.Fatalf("ledger files = %v, want 2 rotated + active", files)
	}
	if _, err := os.Stat(ta.ledgerPath() + ".lock"); !os.IsNotExist(err) {
		t.Fatal("rotation left its lock behind")
	}
}

func TestStaleRotationLockIsCleared(t *testing.T) {
	ta := newTestApp(t)
	cfg := LedgerConfig{Enabled: true, MaxBytes: 10, KeepRotated: 1}
	ta.appendLedger(cfg, ledgerRecord{Host: hostClaude, Kind: kindSpawn})
	lock := ta.ledgerPath() + ".lock"
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	old := fixedNow.Add(-2 * rotationLockTTL)
	_ = os.Chtimes(lock, old, old)
	ta.appendLedger(cfg, ledgerRecord{Host: hostClaude, Kind: kindSpawn})
	if len(ta.ledgerFiles()) != 2 {
		t.Fatalf("a stale lock blocked rotation: %v", ta.ledgerFiles())
	}
}

func TestClipIsRuneSafe(t *testing.T) {
	s := strings.Repeat("é", 50) + strings.Repeat("ü", 50)
	got := clip(s, 10, 10, "|")
	if !utf8.ValidString(got) || got != strings.Repeat("é", 10)+"|"+strings.Repeat("ü", 10) {
		t.Fatalf("clip = %q", got)
	}
	if clip("short", 10, 10, "|") != "short" {
		t.Fatal("clip changed a string within budget")
	}
}

func TestContainBashSplitsBudgetAndSurvivesOddShapes(t *testing.T) {
	ta := newTestApp(t)
	cfg := MainThreadConfig{BashMaxChars: 2000, HeadChars: 800, TailChars: 800}
	in := hookInput{SessionID: "s", ToolResponse: json.RawMessage(`{"stdout":"","stderr":"` + strings.Repeat("e", 5000) + `"}`)}
	c, ok := ta.containBash(cfg, in)
	if !ok {
		t.Fatal("stderr-only output over budget was not contained")
	}
	var resp map[string]string
	_ = json.Unmarshal(c.Response, &resp)
	// stdout is empty, so stderr inherits the whole 1600-character budget.
	if !strings.Contains(resp["stderr"], "1600 of 5000 characters kept inline") || c.Kept != 1600 {
		t.Fatalf("stderr-only containment: kept %d, stderr %q", c.Kept, resp["stderr"][:120])
	}
	for name, raw := range map[string]string{"array": `[1]`, "no streams": `{"output":"x"}`, "non-string": `{"stdout":5}`} {
		if _, ok := ta.containBash(cfg, hookInput{ToolResponse: json.RawMessage(raw)}); ok {
			t.Errorf("%s: contained an unknown shape", name)
		}
	}
	ta.vars["AGENT_HOOKS_STATE"] = "/dev/null/cannot-create"
	big := hookInput{ToolResponse: json.RawMessage(`{"stdout":"` + strings.Repeat("x", 5000) + `"}`)}
	if _, ok := ta.containBash(cfg, big); ok {
		t.Error("contained output whose full text could not be saved (the evidence link would be severed)")
	}
}
