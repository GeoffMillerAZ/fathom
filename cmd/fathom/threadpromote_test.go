package main

// threadpromote_test.go — TH-5: intent graduates into the gate's ledger,
// and the conveyor keeps moving on its own afterwards.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestThreadPromoteScaffoldsGateReadyPlan(t *testing.T) {
	threadEnv(t)
	repo := t.TempDir()
	seedItem(t, "th-p1", "decision-pending", "suppression moves to the query layer", func(it *threadItem) {
		it.Thread = "purge-rework"
		it.Pointers = map[string]string{"session_id": "cafebabe-1234"}
	})
	seedItem(t, "th-p2", "spec-unbuilt", "raw-bytes probe joins the verdict type", func(it *threadItem) {
		it.Thread = "purge-rework"
	})
	seedItem(t, "th-other", "handoff-note", "unrelated thread stays out", func(it *threadItem) {
		it.Thread = "other"
	})

	var out, errb bytes.Buffer
	if code := cmdThreadPromote([]string{"purge-rework", "--repo", repo}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	plan := filepath.Join(repo, "docs", "plans", "purge-rework.md")
	raw, err := os.ReadFile(plan)
	if err != nil {
		t.Fatalf("the scaffold must land in docs/plans/: %v", err)
	}
	got := string(raw)

	if !strings.Contains(got, "suppression moves to the query layer") ||
		!strings.Contains(got, "raw-bytes probe joins the verdict") {
		t.Fatalf("both open items must become rows:\n%s", got)
	}
	if strings.Contains(got, "unrelated thread") {
		t.Error("only the named thread's items promote")
	}
	if !strings.Contains(got, "{id: PR-1}") || !strings.Contains(got, "{id: PR-2}") {
		t.Errorf("rows must carry stable ids the gate can track:\n%s", got)
	}
	if strings.Contains(got, "- [x]") {
		t.Fatal("promote moves intent into the ledger, NEVER into done — no row may be born checked")
	}
	if !strings.Contains(got, "ground each") && !strings.Contains(got, "NO contracts") {
		t.Errorf("the scaffold must say plainly that contracts are not yet grounded:\n%s", got)
	}
	if !strings.Contains(got, "session cafebabe") {
		t.Errorf("origin pointers ride along:\n%s", got)
	}

	t.Run("promoted items self-close when their rows are checked", func(t *testing.T) {
		items := evaluateThreadConditions(loadThreadItems(threadsPath()), repo, time.Now())
		for _, it := range items {
			if it.ID == "th-p1" && it.ClosedAt != "" {
				t.Fatal("the row is unchecked; the item must stay open")
			}
		}
		checked := strings.Replace(got, "- [ ] suppression moves to the query layer", "- [x] suppression moves to the query layer", 1)
		if err := os.WriteFile(plan, []byte(checked), 0o644); err != nil {
			t.Fatal(err)
		}
		items = evaluateThreadConditions(loadThreadItems(threadsPath()), repo, time.Now())
		var closed bool
		for _, it := range items {
			if it.ID == "th-p1" {
				closed = it.ClosedAt != "" && it.ClosedBy == "condition"
			}
		}
		if !closed {
			t.Fatal("checking the promoted row must close its source item — the conveyor advancing on its own")
		}
	})

	t.Run("promote refuses to overwrite an existing plan", func(t *testing.T) {
		var o, e bytes.Buffer
		if code := cmdThreadPromote([]string{"purge-rework", "--repo", repo}, &o, &e); code == 0 {
			t.Fatal("overwriting a plan would destroy gate history; promote is a birth certificate")
		}
	})
}

func TestThreadHandoffIsHarnessNeutral(t *testing.T) {
	threadEnv(t)
	seedItem(t, "th-h1", "handoff-note", "the next agent needs the V verdicts", func(it *threadItem) {
		it.Thread = "purge-rework"
		it.Pointers = map[string]string{"session_id": "cafebabe-9999"}
	})
	var out, errb bytes.Buffer
	if code := cmdThreadHandoff([]string{"--repo", t.TempDir()}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	got := out.String()
	if !strings.Contains(got, "# Continuity handoff") || !strings.Contains(got, "purge-rework") {
		t.Fatalf("the handoff must group by thread:\n%s", got)
	}
	if !strings.Contains(got, "session cafebabe-9999") {
		t.Error("the FULL session id travels — a foreign agent cannot expand a prefix")
	}
	if strings.Contains(got, "claude") || strings.Contains(got, "Claude") {
		t.Error("harness-neutral means no harness names in the artifact")
	}
}
