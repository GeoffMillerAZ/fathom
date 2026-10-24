package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/wiring"
)

func runHookReconcile(t *testing.T, root string, apply bool) string {
	t.Helper()
	args := []string{"--repo", root, "--url", "://unavailable"}
	if apply {
		args = append(args, "--apply")
	}
	var out, errb strings.Builder
	if code := cmdFleetReconcile(args, &out, &errb); code != 0 {
		t.Fatalf("reconcile exit %d: %s%s", code, &out, &errb)
	}
	return out.String()
}

func writeHookLedgerVersion(t *testing.T, root string, version int) {
	t.Helper()
	ledger, err := wiring.LoadLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	ledger.Version = version
	body, err := json.Marshal(ledger)
	if err != nil {
		t.Fatal(err)
	}
	path := wiring.LedgerPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileAdoptionSurvivesOtherWriters(t *testing.T) {
	for _, initial := range []string{"absent", "v1"} {
		for _, writer := range []string{"RecordWritten", "Init"} {
			t.Run(initial+"/"+writer, func(t *testing.T) {
				t.Setenv("HOME", t.TempDir())
				root := t.TempDir()
				writeSettings(t, root, []byte(`{"hooks":{"PostToolUse":[{"matcher":"Edit|Write|MultiEdit","hooks":[{"type":"command","command":"fathom edit-hook","timeout":5}]}]}}`))
				if initial == "v1" {
					writeHookLedgerVersion(t, root, 1)
				}
				switch writer {
				case "RecordWritten":
					label := ".claude/skills/fathom-example/SKILL.md"
					body := "<!-- fathom:owned -->\nExample skill\n"
					writeRepoFile(t, root, label, body)
					recorded, err := wiring.RecordWritten(root, []wiring.SkillFile{{Path: filepath.Join(root, label), Label: label, Shipped: []byte(body)}}, "2026-01-01T00:00:00Z")
					if err != nil || len(recorded) != 1 {
						t.Fatalf("RecordWritten = %v, %v", recorded, err)
					}
				case "Init":
					var out, errb strings.Builder
					// Init writes before contacting the daemon. Exercise its real
					// writers without a listener or a dependency on a live daemon.
					if code := cmdInit([]string{"--path", root, "--url", "://unavailable", "--skills=claude"}, &out, &errb); code != 2 || !strings.Contains(errb.String(), "unreachable") {
						t.Fatalf("init exit %d: %s%s", code, &out, &errb)
					}
				}
				ledger, err := wiring.LoadLedger(root)
				if err != nil || !ledger.NeedsHookAdoption() {
					t.Errorf("writer consumed hook adoption: version=%d, err=%v", ledger.Version, err)
				}
				if dry := runHookReconcile(t, root, false); strings.Contains(dry, "collision:") || !strings.Contains(dry, "would adopt 1 fathom keys into the ledger") {
					t.Errorf("legacy hook adoption lost after writer:\n%s", dry)
				}
			})
		}
	}
}

func TestReconcileMigrationWithoutAdoptions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	runHookReconcile(t, root, true)
	writeHookLedgerVersion(t, root, 1)
	runHookReconcile(t, root, false)
	ledger, err := wiring.LoadLedger(root)
	if err != nil || ledger.Version != 1 {
		t.Fatalf("dry run changed migration: version=%d, err=%v", ledger.Version, err)
	}
	runHookReconcile(t, root, true)
	ledger, err = wiring.LoadLedger(root)
	if err != nil || ledger.NeedsHookAdoption() {
		t.Fatalf("zero-adoption migration not saved: version=%d, err=%v", ledger.Version, err)
	}
	before, err := os.ReadFile(wiring.LedgerPath(root))
	if err != nil {
		t.Fatal(err)
	}
	out := runHookReconcile(t, root, true)
	after, err := os.ReadFile(wiring.LedgerPath(root))
	if err != nil || string(before) != string(after) || strings.Contains(out, "receipt:") {
		t.Fatalf("completed migration wrote again: err=%v\n%s", err, out)
	}
}

func TestReconcileCollisionIsNotDrift(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	runHookReconcile(t, root, true)
	hs := fathomHookSpecsAt(root)[0]
	k := hookKeyFor(hs, hookCommand("fathom", hs.sub))
	ledger, err := wiring.LoadLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	ledger.Forget(k.File, k.ID())
	if err := wiring.SaveLedger(root, ledger); err != nil {
		t.Fatal(err)
	}
	settings := readRepoFile(t, root, k.File)
	oldCommand, err := json.Marshal(k.Body)
	if err != nil {
		t.Fatal(err)
	}
	newCommand, err := json.Marshal("./scripts/wrapper 'fathom " + hs.sub + "'")
	if err != nil {
		t.Fatal(err)
	}
	settings = strings.Replace(settings, string(oldCommand), string(newCommand), 1)
	writeSettings(t, root, []byte(settings))
	for _, apply := range []bool{false, true} {
		out := runHookReconcile(t, root, apply)
		if strings.Count(out, "collision:") != 1 || !strings.Contains(out, "0 drifted") {
			t.Errorf("collision must be printed without drift:\n%s", out)
		}
		if got := readRepoFile(t, root, k.File); got != settings {
			t.Fatal("collision changed settings")
		}
	}
}

func TestReconcileUnreadableHooksKeepMigrationPending(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	writeHookLedgerVersion(t, root, 1)
	writeSettings(t, root, []byte("{"))
	if _, _, migrate := planShimHookKeys(root); migrate {
		t.Fatal("unreadable hook settings authorized migration")
	}
	var out, errb strings.Builder
	if code := cmdFleetReconcile([]string{"--repo", root, "--url", "://unavailable", "--apply"}, &out, &errb); code != 1 || !strings.Contains(out.String(), "BLOCKED") {
		t.Fatalf("unreadable hooks not blocked: exit=%d, %s%s", code, &out, &errb)
	}
	ledger, err := wiring.LoadLedger(root)
	if err != nil || !ledger.NeedsHookAdoption() {
		t.Fatalf("unreadable hooks consumed migration: version=%d, err=%v", ledger.Version, err)
	}
}
