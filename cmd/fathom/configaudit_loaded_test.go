package main

// configaudit_loaded_test.go — RO-41 (docs/plans/readouts.md, design §20 H11):
// `fathom config audit --loaded` counts, per instruction file loaded in the
// window, its bytes at HEAD, its loads and the served claims compiled from it.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func loadedGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestConfigAuditLoadedCountsServedClaimsPerInstructionFile(t *testing.T) {
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	orig := configAuditLoadedNow
	configAuditLoadedNow = func() time.Time { return now }
	t.Cleanup(func() { configAuditLoadedNow = orig })

	mustWrite(t, filepath.Join(root, "CLAUDE.md"), strings.Repeat("x", 400))
	mustWrite(t, filepath.Join(root, "docs", "plain.md"), strings.Repeat("y", 80))
	mustWrite(t, filepath.Join(root, ".claude", "rules", "r.md"), strings.Repeat("z", 40))
	loadedGit(t, root, "init", "-q")
	loadedGit(t, root, "add", ".")
	loadedGit(t, root, "commit", "-q", "-m", "x")

	beatLog := filepath.Join(t.TempDir(), "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", beatLog)
	ruleLog := filepath.Join(t.TempDir(), "rule-outcomes.jsonl")
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", ruleLog)

	var bl strings.Builder
	beat := func(ago time.Duration, file string) {
		fmt.Fprintf(&bl, `{"channel":"instructions-loaded","kind":"loaded","at":%q,"meta":{"file_path":%q,"memory_type":"Project","load_reason":"session_start"}}`+"\n",
			now.Add(-ago).Format(time.RFC3339Nano), filepath.Join(root, file))
	}
	beat(10*24*time.Hour, "CLAUDE.md")
	beat(9*24*time.Hour, "CLAUDE.md")
	beat(8*24*time.Hour, "docs/plain.md")
	beat(7*24*time.Hour, ".claude/rules/r.md")
	mustWrite(t, beatLog, bl.String())

	var rl strings.Builder
	row := func(ago time.Duration, claim, event string) {
		fmt.Fprintf(&rl, `{"schema_version":1,"ts":%q,"rule_id":%q,"event":%q}`+"\n", now.Add(-ago).Format(time.RFC3339Nano), claim, event)
	}
	row(20*24*time.Hour, "c1", "served") // before the window: not counted
	row(6*24*time.Hour, "c1", "served")
	row(5*24*time.Hour, "c1", "served")
	row(4*24*time.Hour, "c2", "served")
	row(3*24*time.Hour, "c2", "corroborated") // not a served row
	mustWrite(t, ruleLog, rl.String())

	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "svc", Path: root}); err != nil {
		t.Fatal(err)
	}
	at := now.AddDate(0, -1, 0)
	mk := func(id, subject string) core.SemanticClaim {
		return core.SemanticClaim{ServiceID: "svc", ClaimID: id, Layer: "obligations", LayerVersion: "v1", Kind: "obligation",
			Subject: subject, Rule: "rule " + id, Scope: []string{"**"}, Tier: core.TierDocClaimed, Status: core.ClaimFresh, CompiledAt: at, ValidFrom: at}
	}
	if err := st.PutClaims(ctx, []core.SemanticClaim{mk("c1", "CLAUDE.md#a"), mk("c2", "CLAUDE.md#b"), mk("c3", ".claude/rules/r.md#q")}, nil); err != nil {
		t.Fatal(err)
	}
	st.Close()

	var out, errb bytes.Buffer
	code := run([]string{"config", "audit", "--loaded", "--repo", root, "--store", storePath, "--json"}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	var rep configAuditLoadedReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("json: %v\n%s", err, out.String())
	}
	if want := now.Add(-10 * 24 * time.Hour).Format(time.RFC3339Nano); rep.WindowStart != want {
		t.Errorf("window start %q, want first beat %q", rep.WindowStart, want)
	}
	got := map[string]configAuditLoadedFile{}
	var order []string
	for _, f := range rep.Files {
		got[f.File] = f
		order = append(order, f.File)
	}
	if len(order) != 3 || order[0] != ".claude/rules/r.md" || order[1] != "CLAUDE.md" || order[2] != "docs/plain.md" {
		t.Fatalf("files not the 3 loaded, sorted: %v", order)
	}
	if f := got["CLAUDE.md"]; f.Bytes != 400 || f.EstTokens != 100 || f.Loads != 2 || f.Served != 3 || f.Status != "served" {
		t.Errorf("CLAUDE.md: %+v", f)
	}
	if f := got[".claude/rules/r.md"]; f.Loads != 1 || f.Served != 0 || f.Status != "0 served in 10d" {
		t.Errorf("r.md: %+v", f)
	}
	if f := got["docs/plain.md"]; f.Status != "not compiled" || f.Loads != 1 {
		t.Errorf("plain.md: %+v", f)
	}
	out.Reset()
	if code := run([]string{"config", "audit", "--loaded", "--repo", root, "--store", storePath}, &out, &errb); code != 0 {
		t.Fatalf("text exit %d", code)
	}
	txt := out.String()
	for _, w := range []string{"est. tokens", "0 served in 10d", "not compiled", "3 served"} {
		if !strings.Contains(txt, w) {
			t.Errorf("text lacks %q:\n%s", w, txt)
		}
	}
}

func TestConfigAuditLoadedNativeFileHasNoEvent(t *testing.T) {
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	mustWrite(t, filepath.Join(root, "AGENTS.md"), strings.Repeat("a", 40))
	mustWrite(t, filepath.Join(root, "docs", "d.md"), "d")
	loadedGit(t, root, "init", "-q")
	loadedGit(t, root, "add", ".")
	loadedGit(t, root, "commit", "-q", "-m", "x")
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", filepath.Join(t.TempDir(), "r.jsonl"))
	var out, errb bytes.Buffer
	if code := run([]string{"config", "audit", "--loaded", "--repo", root, "--store", filepath.Join(t.TempDir(), "none.sqlite"), "--json"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d %s", code, errb.String())
	}
	var rep configAuditLoadedReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Files) != 1 || rep.Files[0].File != "AGENTS.md" || rep.Files[0].Loads != 0 || !rep.Files[0].Native || rep.Files[0].Status != "loaded natively, no event" {
		t.Fatalf("%+v", rep.Files)
	}
}
