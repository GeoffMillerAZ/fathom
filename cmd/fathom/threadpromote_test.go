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

// ─── thread promote --route (RC-30 remainder) ────────────────────────────

// writeRoutePlanFixture writes a minimal plan file with one row carrying
// ann (the annotation block's INNER text, e.g. "id: RT-1, symbol: x,
// package: pkg/x") and returns its path.
func writeRoutePlanFixture(t *testing.T, dir, ann string) string {
	t.Helper()
	path := filepath.Join(dir, "plan.md")
	body := "# fixture plan\n\n- [ ] Wire the route {" + ann + "}\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// seedRouteFixtureCallLog records two distinct sessions both establishing
// pointer over root's sessions log and call log — routedPointers' own
// two-distinct-session bar (RC-30) — and returns the call-log path.
func seedRouteFixtureCallLog(t *testing.T, root, service, pointer string, now time.Time) string {
	t.Helper()
	t.Setenv("FATHOM_SESSIONS_LOG", filepath.Join(root, "sessions.jsonl"))
	recordSessionWindow(root, sessionWindowRow{SchemaVersion: 1, SessionID: "s1", Cwd: root, Service: service, Started: now.Add(-2 * time.Hour).Format(time.RFC3339)})
	recordSessionWindow(root, sessionWindowRow{SchemaVersion: 1, SessionID: "s2", Cwd: root, Service: service, Started: now.Add(-time.Hour).Format(time.RFC3339)})
	logPath := filepath.Join(root, "calls.jsonl")
	writeDiscoCallLogFixture(t, logPath, []string{
		`{"ts":"` + now.Add(-90*time.Minute).Format(time.RFC3339Nano) + `","service":"` + service + `","pointers":["` + pointer + `"]}`,
		`{"ts":"` + now.Add(-30*time.Minute).Format(time.RFC3339Nano) + `","service":"` + service + `","pointers":["` + pointer + `"]}`,
	})
	return logPath
}

func TestThreadPromoteRoute_WritesRoutePointerOntoPlanRowCtx(t *testing.T) {
	threadEnv(t)
	storePath := seedPlansFixtureStore(t)
	root := t.TempDir()
	now := time.Now().UTC()
	logPath := seedRouteFixtureCallLog(t, root, "payment-service", "payments.Charge", now)
	planPath := writeRoutePlanFixture(t, root, "id: RT-1, symbol: something, package: pkg/x")

	var out, errb bytes.Buffer
	code := cmdThreadPromote([]string{
		"--route", "payments", "--plan", planPath, "--row", "RT-1",
		"--service", "payment-service", "--store", storePath,
		"--repo", root, "--call-log", logPath,
	}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	raw, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	if !strings.Contains(got, "ctx: payments.Charge") {
		t.Fatalf("expected the routed pointer written onto ctx:, got:\n%s", got)
	}
	if !strings.Contains(got, "symbol: something") || !strings.Contains(got, "package: pkg/x") {
		t.Errorf("existing annotation keys must survive untouched:\n%s", got)
	}
	if !strings.Contains(out.String(), "wrote 1 route pointer") {
		t.Errorf("stdout should report what was written: %q", out.String())
	}
}

func TestThreadPromoteRoute_UnionsWithExistingCtx(t *testing.T) {
	threadEnv(t)
	storePath := seedPlansFixtureStore(t)
	root := t.TempDir()
	now := time.Now().UTC()
	logPath := seedRouteFixtureCallLog(t, root, "payment-service", "payments.Charge", now)
	planPath := writeRoutePlanFixture(t, root, `id: RT-2, ctx: "already/here.Pointer"`)

	var out, errb bytes.Buffer
	code := cmdThreadPromote([]string{
		"--route", "payments", "--plan", planPath, "--row", "RT-2",
		"--service", "payment-service", "--store", storePath,
		"--repo", root, "--call-log", logPath,
	}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	got, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "already/here.Pointer") || !strings.Contains(string(got), "payments.Charge") {
		t.Fatalf("promote must UNION with the row's existing ctx:, never replace it:\n%s", got)
	}
}

func TestThreadPromoteRoute_UnreachableIndexRefuses(t *testing.T) {
	threadEnv(t)
	root := t.TempDir()
	now := time.Now().UTC()
	logPath := seedRouteFixtureCallLog(t, root, "svc", "pkg.Thing", now)
	planPath := writeRoutePlanFixture(t, root, "id: RT-3")

	var out, errb bytes.Buffer
	code := cmdThreadPromote([]string{
		"--route", "pkg", "--plan", planPath, "--row", "RT-3",
		"--service", "svc", "--url", "http://127.0.0.1:1",
		"--repo", root, "--call-log", logPath,
	}, &out, &errb)
	if code == 0 {
		t.Fatal("an unreachable index must refuse — never write an unconfirmed route")
	}
	got, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "ctx:") {
		t.Errorf("the plan must be left UNCHANGED on refusal:\n%s", got)
	}
	if !strings.Contains(errb.String(), "unreachable") {
		t.Errorf("stderr should name the unreachable index: %q", errb.String())
	}
}

func TestThreadPromoteRoute_RowNotFoundRefuses(t *testing.T) {
	threadEnv(t)
	storePath := seedPlansFixtureStore(t)
	root := t.TempDir()
	now := time.Now().UTC()
	logPath := seedRouteFixtureCallLog(t, root, "payment-service", "payments.Charge", now)
	planPath := writeRoutePlanFixture(t, root, "id: RT-1")

	var out, errb bytes.Buffer
	code := cmdThreadPromote([]string{
		"--route", "payments", "--plan", planPath, "--row", "NO-SUCH-ROW",
		"--service", "payment-service", "--store", storePath,
		"--repo", root, "--call-log", logPath,
	}, &out, &errb)
	if code == 0 {
		t.Fatal("a missing row id must refuse")
	}
	if !strings.Contains(errb.String(), "not found") {
		t.Errorf("stderr should name the missing row: %q", errb.String())
	}
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
