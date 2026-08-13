package main

// thread_test.go — continuity threads (TH-1..4). The load-bearing
// obligations from docs/design/continuity-threads.md §5.

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func threadEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("FATHOM_THREADS", filepath.Join(dir, "threads.jsonl"))
	t.Setenv("FATHOM_SESSION_STAMPS", filepath.Join(dir, "stamps"))
	return dir
}

func seedItem(t *testing.T, id, kind, text string, extra func(*threadItem)) {
	t.Helper()
	it := threadItem{
		SchemaVersion: 1, ID: id, Thread: "tt", Kind: kind, Text: text,
		FiledAt: time.Date(2026, 8, 13, 1, 0, 0, 0, time.UTC).Format(time.RFC3339),
	}
	if extra != nil {
		extra(&it)
	}
	if err := appendThreadItem(it); err != nil {
		t.Fatal(err)
	}
}

// TestThreadBriefComposesDerivedAndDeposited pins TH-1's read side: the
// brief carries deposited items ranked by kind value, and the store's
// history stays append-only under closure.
func TestThreadBriefComposesDerivedAndDeposited(t *testing.T) {
	threadEnv(t)
	now := time.Date(2026, 8, 13, 2, 0, 0, 0, time.UTC)
	seedItem(t, "th-note", "handoff-note", "V rework: suppression at the query layer", nil)
	seedItem(t, "th-side", "side-task", "purge blast-radius reproduced", func(it *threadItem) {
		it.Pointers = map[string]string{"session_id": "cafebabe-0000"}
	})

	out := renderThreadBrief(loadThreadItems(threadsPath()), t.TempDir(), threadBriefBudgetBytes, now)
	if !strings.Contains(out, "V rework") || !strings.Contains(out, "purge blast-radius") {
		t.Fatalf("both deposited items must appear:\n%s", out)
	}
	if strings.Index(out, "V rework") > strings.Index(out, "purge blast-radius") {
		t.Error("a handoff note outranks a pointer — value-per-byte order")
	}
	if !strings.Contains(out, "session cafebabe") {
		t.Errorf("a side-task carries its session ADDRESS, not its content:\n%s", out)
	}
	if !strings.Contains(out, "fathom thread list") {
		t.Error("the brief must say where the rest lives")
	}
}

// TestBriefBudgetAndSourceGating pins TH-3, both halves: the byte bound is
// HARD and disclosed, and resume/compact get nothing at all.
func TestBriefBudgetAndSourceGating(t *testing.T) {
	threadEnv(t)
	now := time.Date(2026, 8, 13, 2, 0, 0, 0, time.UTC)
	for i := 0; i < 40; i++ {
		seedItem(t, "th-bulk"+itoa(i), "handoff-note",
			strings.Repeat("a fairly long line of continuity text ", 3)+itoa(i), nil)
	}

	t.Run("the budget is a hard bound with a named cut", func(t *testing.T) {
		out := renderThreadBrief(loadThreadItems(threadsPath()), t.TempDir(), threadBriefBudgetBytes, now)
		if len(out) > threadBriefBudgetBytes {
			t.Fatalf("brief is %d bytes against a %d budget — the ONE surface with a context cost broke its bound",
				len(out), threadBriefBudgetBytes)
		}
		if !strings.Contains(out, "more: fathom thread list") {
			t.Error("a truncated brief must say how many were cut and where they live (law 3)")
		}
	})

	t.Run("resume and compact print nothing", func(t *testing.T) {
		for _, src := range []string{"resume", "compact"} {
			var out, errb bytes.Buffer
			if code := cmdThreadBrief([]string{"--source", src}, &out, &errb); code != 0 {
				t.Fatalf("exit %d: %s", code, errb.String())
			}
			if out.Len() != 0 {
				t.Errorf("source=%s must inject NOTHING — its context survives:\n%s", src, out.String())
			}
		}
	})
}

// TestConditionClosesItem pins the self-expiry: a plan_row_checked item
// closes itself, attributed, when the box flips.
func TestConditionClosesItem(t *testing.T) {
	threadEnv(t)
	repo := t.TempDir()
	plan := filepath.Join(repo, "docs", "plans", "p.md")
	if err := os.MkdirAll(filepath.Dir(plan), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan, []byte("- [ ] build the thing {id: TT-1, symbol: x, package: y}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedItem(t, "th-cond", "spec-unbuilt", "the thing is specced, unbuilt", func(it *threadItem) {
		it.CompletesWhen = &threadCondition{Kind: "plan_row_checked", Repo: repo, Path: "docs/plans/p.md", Row: "TT-1"}
	})
	now := time.Date(2026, 8, 13, 3, 0, 0, 0, time.UTC)

	items := evaluateThreadConditions(loadThreadItems(threadsPath()), repo, now)
	if items[0].ClosedAt != "" {
		t.Fatal("the box is unchecked; the item must stay open")
	}

	if err := os.WriteFile(plan, []byte("- [x] build the thing {id: TT-1, symbol: x, package: y}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	items = evaluateThreadConditions(loadThreadItems(threadsPath()), repo, now)
	if items[0].ClosedAt == "" {
		t.Fatal("the box flipped; the item must close itself")
	}
	if items[0].ClosedBy != "condition" {
		t.Errorf("closed_by = %q, want condition — attribution is what makes self-closure auditable", items[0].ClosedBy)
	}
	// And the closure is DURABLE: a fresh load sees it without re-evaluating.
	reloaded := loadThreadItems(threadsPath())
	if reloaded[0].ClosedAt == "" {
		t.Error("the closure must be a durable row, not a per-read computation")
	}
	if reloaded[0].Text == "" {
		t.Error("the closure row must not erase the item's text (latest-wins carries fields forward)")
	}
}

// TestSessionEndDepositsOnClear pins TH-2: a /clear with work in flight
// deposits ONE mechanical item; a clean empty session deposits NOTHING.
func TestSessionEndDepositsOnClear(t *testing.T) {
	threadEnv(t)
	t.Setenv("FATHOM_CHANNELS", "")

	runHook := func(t *testing.T, root, reason string) {
		t.Helper()
		in := map[string]any{"session_id": "sess-end-1", "cwd": root, "reason": reason}
		raw, _ := json.Marshal(in)
		defer pipeStdin(t, string(raw))()
		var out, errb bytes.Buffer
		if code := cmdSessionEndHook(nil, &out, &errb); code != 0 {
			t.Fatalf("exit %d: %s", code, errb.String())
		}
	}
	gitRepo := func(t *testing.T) string {
		t.Helper()
		root := t.TempDir()
		for _, args := range [][]string{{"init", "-q"}, {"commit", "-q", "--allow-empty", "-m", "x"}} {
			cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.email=t@t", "-c", "user.name=t"}, args...)...)
			if err := cmd.Run(); err != nil {
				t.Skipf("git unavailable: %v", err)
			}
		}
		return root
	}

	t.Run("clean session deposits nothing", func(t *testing.T) {
		runHook(t, gitRepo(t), "clear")
		if items := loadThreadItems(threadsPath()); len(items) != 0 {
			t.Fatalf("a clean, empty session must deposit NOTHING — silence is fine; got %+v", items)
		}
	})

	t.Run("work in flight deposits one pointer", func(t *testing.T) {
		root := gitRepo(t)
		if err := os.WriteFile(filepath.Join(root, "wip.go"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		runHook(t, root, "clear")
		items := loadThreadItems(threadsPath())
		if len(items) != 1 {
			t.Fatalf("want exactly one deposit, got %d", len(items))
		}
		it := items[0]
		if it.Kind != "session-end" || !strings.Contains(it.Text, "uncommitted") {
			t.Errorf("the deposit must be the mechanical fact: %+v", it)
		}
		if it.Pointers["session_id"] != "sess-end-1" {
			t.Error("the deposit must point at the session that left the work")
		}
	})

	t.Run("a resume-shaped end deposits nothing", func(t *testing.T) {
		before := len(loadThreadItems(threadsPath()))
		root := gitRepo(t)
		if err := os.WriteFile(filepath.Join(root, "wip.go"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		runHook(t, root, "resume")
		if got := len(loadThreadItems(threadsPath())); got != before {
			t.Error("reason=resume keeps its context; depositing there duplicates what survives")
		}
	})
}

// TestStampJoinLabelsLedgerRows pins the cohort labeling: the SessionStart
// stamp reaches the ledger row, and a missing stamp yields "" — unknown,
// never guessed.
func TestStampJoinLabelsLedgerRows(t *testing.T) {
	threadEnv(t)
	writeSessionStamp(sessionStamp{
		SchemaVersion: 1, SessionID: "s-fresh", Source: "clear",
		BriefInjected: true, BriefBytes: 512, At: "2026-08-13T02:00:00Z",
	})

	now := time.Date(2026, 8, 13, 2, 30, 0, 0, time.UTC)
	e := buildSessionLedgerEntry("s-fresh", "repo", transcriptSummary{}, nil, now)
	if e.SessionSource != "clear" || !e.BriefInjected || e.BriefBytes != 512 {
		t.Fatalf("the stamp must label the row: %+v", e)
	}
	anon := buildSessionLedgerEntry("s-unstamped", "repo", transcriptSummary{}, nil, now)
	if anon.SessionSource != "" || anon.BriefInjected {
		t.Errorf("a missing stamp is UNKNOWN, never guessed: %+v", anon)
	}
}

// TestThreadBetEvaluation pins TH-4: cohort admission, the registered
// floor, the pricing-assumption disclosure, and the kill riding in the
// verdict.
func TestThreadBetEvaluation(t *testing.T) {
	mk := func(source string, brief bool, in, out, cache, turns, firstEdit int) sessionLedgerEntry {
		fe := firstEdit
		return sessionLedgerEntry{
			SessionSource: source, BriefInjected: brief,
			TokensIn: &in, TokensOut: &out, TokensCacheRead: &cache, TurnCount: &turns, FirstEditTurn: &fe,
		}
	}

	t.Run("below the floor there is no verdict", func(t *testing.T) {
		rows := []sessionLedgerEntry{mk("clear", true, 100, 100, 0, 20, 5)}
		rep := threadBetCheck(rows)
		if rep.Verdict != "collecting" {
			t.Fatalf("verdict = %q, want collecting — n=1 is far below the registered 20", rep.Verdict)
		}
		if !strings.Contains(rep.Detail, "1 of 20") {
			t.Errorf("the counts must be named: %s", rep.Detail)
		}
	})

	t.Run("unlabeled rows join no cohort", func(t *testing.T) {
		rows := []sessionLedgerEntry{mk("", false, 100, 100, 0, 20, 5)}
		rep := threadBetCheck(rows)
		if rep.FreshUnits != 0 || rep.ResumedUnits != 0 {
			t.Fatal("unknown is not a cohort (law 3)")
		}
	})

	t.Run("met and not-met carry their consequences", func(t *testing.T) {
		var rows []sessionLedgerEntry
		for i := 0; i < 20; i++ {
			rows = append(rows, mk("clear", true, 1000, 1000, 0, 20, 5)) // 100/turn, warm-up 5
		}
		for i := 0; i < 5; i++ {
			rows = append(rows, mk("resume", false, 1000, 1000, 20000*10, 20, 8)) // 100 + 10000/turn cache
		}
		rep := threadBetCheck(rows)
		if rep.Verdict != "met" {
			t.Fatalf("verdict = %q (%s), want met", rep.Verdict, rep.Detail)
		}
		if !strings.Contains(rep.Detail, "10%") {
			t.Errorf("the cache-read weighting is a PRICING ASSUMPTION and must be named: %s", rep.Detail)
		}

		// Flip it: fresh warms up slower → the registered kill must speak.
		rows = rows[:0]
		for i := 0; i < 20; i++ {
			rows = append(rows, mk("clear", true, 1000, 1000, 0, 20, 50))
		}
		for i := 0; i < 5; i++ {
			rows = append(rows, mk("resume", false, 1000, 1000, 0, 20, 8))
		}
		rep = threadBetCheck(rows)
		if rep.Verdict != "not-met" {
			t.Fatalf("verdict = %q, want not-met", rep.Verdict)
		}
		if !strings.Contains(rep.Detail, "REMOVED") {
			t.Errorf("the registered kill rides in the verdict so it cannot become a memory: %s", rep.Detail)
		}
	})
}
