package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/intent"
)

// fathomHomeResolvers is every per-user path the ledgers, logs, stamps and
// daemon records resolve through, with the per-file variable that overrides
// it ("" for a path that has none) and the path relative to the fathom home.
func fathomHomeResolvers(t *testing.T) []struct {
	name, envVar, rel string
	resolve           func() string
} {
	t.Helper()
	purges := func() string { p, _ := purgesPath(); return p }
	daemonState := func() string { p, _ := daemonStateDir(); return p }
	logDir := func() string { p, _ := daemonLogDir(); return p }
	return []struct {
		name, envVar, rel string
		resolve           func() string
	}{
		{"verify ledger", "FATHOM_VERIFY_LEDGER", "verify-ledger.jsonl", verifyLedgerPath},
		{"intent ledger", "FATHOM_INTENT_LEDGER", "intent-ledger.jsonl", intent.LedgerPath},
		{"session ledger", "FATHOM_SESSION_LEDGER", "session-ledger.jsonl", sessionLedgerPath},
		{"hooks beat log", "FATHOM_HOOKS_LOG", "hooks.jsonl", beatLogPath},
		{"assess ledger", "FATHOM_ASSESS_LEDGER", "assess-ledger.jsonl", assessLedgerPath},
		{"preflight ledger", "FATHOM_PREFLIGHT_LEDGER", "preflight-outcomes.jsonl", preflightLedgerPath},
		{"purges log", "FATHOM_PURGES", "purges.jsonl", purges},
		{"rule outcomes", "FATHOM_RULE_OUTCOME_LOG", "rule-outcomes.jsonl", ruleOutcomeLogPath},
		{"session ratings", "FATHOM_SESSION_RATINGS", "session-ratings.jsonl", ratingsPath},
		{"session stamps", "FATHOM_SESSION_STAMPS", "session-stamps", sessionStampDir},
		{"threads", "FATHOM_THREADS", "threads.jsonl", threadsPath},
		{"console views", "FATHOM_CONSOLE_VIEWS", "console-views.jsonl", consoleViewsLogPath},
		{"arm state", "FATHOM_ARM_STATE", "arm.json", armStatePath},
		{"fleet snapshots", "", "fleet", fleetSnapshotDir},
		{"daemon logs", "", "logs", logDir},
		{"daemon state", "", "daemon", daemonState},
	}
}

// TestFathomHomeRelocatesLedgersAndPerFileVarsWin is PE-26: FATHOM_HOME
// moves every per-user path under one directory, and a per-file variable
// still names its own file and wins over it.
func TestFathomHomeRelocatesLedgersAndPerFileVarsWin(t *testing.T) {
	realHome, scratch := t.TempDir(), t.TempDir()
	t.Setenv("HOME", realHome)
	for _, r := range fathomHomeResolvers(t) {
		if r.envVar != "" {
			t.Setenv(r.envVar, "")
		}
	}

	t.Setenv(fathomhome.Env, "")
	for _, r := range fathomHomeResolvers(t) {
		if got, want := r.resolve(), filepath.Join(realHome, ".fathom", r.rel); got != want {
			t.Errorf("%s without FATHOM_HOME = %q, want %q", r.name, got, want)
		}
	}

	t.Setenv(fathomhome.Env, scratch)
	if got := fathomhome.FathomHome(); got != scratch {
		t.Fatalf("FathomHome() = %q, want %q", got, scratch)
	}
	for _, r := range fathomHomeResolvers(t) {
		if got, want := r.resolve(), filepath.Join(scratch, r.rel); got != want {
			t.Errorf("%s under FATHOM_HOME = %q, want %q", r.name, got, want)
		}
	}

	for _, r := range fathomHomeResolvers(t) {
		if r.envVar == "" {
			continue
		}
		own := filepath.Join(t.TempDir(), "own-"+filepath.Base(r.rel))
		t.Setenv(r.envVar, own)
		if got := r.resolve(); got != own {
			t.Errorf("%s: per-file %s = %q must win over FATHOM_HOME, got %q", r.name, r.envVar, own, got)
		}
		t.Setenv(r.envVar, "")
	}
}

// TestFathomHomeMovesNothingIntoTheRealHome: with FATHOM_HOME set, the real
// home directory is never a parent of any resolved per-user path.
func TestFathomHomeMovesNothingIntoTheRealHome(t *testing.T) {
	realHome, scratch := t.TempDir(), t.TempDir()
	t.Setenv("HOME", realHome)
	t.Setenv(fathomhome.Env, scratch)
	for _, r := range fathomHomeResolvers(t) {
		if r.envVar != "" {
			t.Setenv(r.envVar, "")
		}
	}
	for _, r := range fathomHomeResolvers(t) {
		if p := r.resolve(); p == "" || !isUnder(p, scratch) {
			t.Errorf("%s = %q, want a path under %s", r.name, p, scratch)
		}
	}
	if entries, _ := os.ReadDir(realHome); len(entries) != 0 {
		t.Errorf("resolving paths created %d entries under the real home", len(entries))
	}
}

func isUnder(p, dir string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !filepath.IsAbs(rel) && len(rel) > 0 && rel[0] != '.'
}

// scratchRunFixture is a faux-store `fathom verify` target: an ingested
// store and a passing plan, both under t.TempDir().
func scratchRunFixture(t *testing.T) (store, plan string) {
	t.Helper()
	store = filepath.Join(t.TempDir(), "scratch.sqlite")
	var ing bytes.Buffer
	if code := run([]string{"ingest", "--service", "payment-service", "--path", fauxPaymentPath(),
		"--store", store, "--mode", "faux"}, &ing, &ing); code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}
	plan = filepath.Join(t.TempDir(), "docs", "plans", "scratch.md")
	if err := os.MkdirAll(filepath.Dir(plan), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# Scratch plan\n\n- [x] Charge handler {id: SC-1, symbol: Charge, skip_test: true, skip: \"fixture\"}\n"
	if err := os.WriteFile(plan, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return store, plan
}

// scratchEnv points HOME at a stand-in for the operator's real home and
// clears every per-file variable, returning that home.
func scratchEnv(t *testing.T) (realHome string) {
	t.Helper()
	realHome = t.TempDir()
	t.Setenv("HOME", realHome)
	for _, v := range []string{"FATHOM_VERIFY_LEDGER", "FATHOM_INTENT_LEDGER", "FATHOM_SESSION_LEDGER", "FATHOM_HOOKS_LOG", fathomhome.Env} {
		t.Setenv(v, "")
	}
	return realHome
}

// TestScratchRunUnderFathomHomeLeavesRealLedgersUntouched is PE-28: a
// scratch `fathom verify` and `fathom intent audit` under FATHOM_HOME
// append to the scratch directory's ledgers and create nothing under the
// real home's ~/.fathom (O121: scratch runs filled the real ledgers).
func TestScratchRunUnderFathomHomeLeavesRealLedgersUntouched(t *testing.T) {
	realHome := scratchEnv(t)
	store, plan := scratchRunFixture(t)
	scratch := t.TempDir()
	t.Setenv(fathomhome.Env, scratch)

	var out, errb bytes.Buffer
	if code := run([]string{"verify", "--service", "payment-service", "--plan", plan, "--store", store, "--quiet"}, &out, &errb); code != 0 {
		t.Fatalf("scratch verify exited %d\nstdout=%s\nstderr=%s", code, out.String(), errb.String())
	}
	root := t.TempDir()
	writeCorpusFixture(t, root)
	out.Reset()
	if code := run([]string{"intent", "audit", "--json", "--root", root, "--url", "http://127.0.0.1:1", "--service", "svc"}, &out, &errb); code != 0 {
		t.Fatalf("scratch intent audit exited %d: %s", code, errb.String())
	}

	for _, name := range []string{"verify-ledger.jsonl", "intent-ledger.jsonl"} {
		if fi, err := os.Stat(filepath.Join(scratch, name)); err != nil || fi.Size() == 0 {
			t.Errorf("the scratch run must append to %s under FATHOM_HOME (stat: %v)", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(realHome, ".fathom")); !os.IsNotExist(err) {
		var names []string
		_ = filepath.WalkDir(filepath.Join(realHome, ".fathom"), func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				names = append(names, p)
			}
			return nil
		})
		t.Errorf("a scratch run under FATHOM_HOME wrote under the real ~/.fathom (stat: %v): %v", err, names)
	}
}

// TestLedgerFlagNamesOneLedger: `--ledger <path>` on `fathom verify` and on
// `fathom intent audit` names that run's ledger file, beating both
// FATHOM_HOME and the per-file variable.
func TestLedgerFlagNamesOneLedger(t *testing.T) {
	realHome := scratchEnv(t)
	store, plan := scratchRunFixture(t)
	scratch, own := t.TempDir(), t.TempDir()
	t.Setenv(fathomhome.Env, scratch)
	envVerify, envIntent := filepath.Join(t.TempDir(), "envvar-verify.jsonl"), filepath.Join(t.TempDir(), "envvar-intent.jsonl")
	t.Setenv("FATHOM_VERIFY_LEDGER", envVerify)
	t.Setenv("FATHOM_INTENT_LEDGER", envIntent)
	verifyOwn, intentOwn := filepath.Join(own, "v.jsonl"), filepath.Join(own, "i.jsonl")

	var out, errb bytes.Buffer
	if code := run([]string{"verify", "--ledger", verifyOwn, "--service", "payment-service", "--plan", plan, "--store", store, "--quiet"}, &out, &errb); code != 0 {
		t.Fatalf("verify --ledger exited %d\nstdout=%s\nstderr=%s", code, out.String(), errb.String())
	}
	root := t.TempDir()
	writeCorpusFixture(t, root)
	out.Reset()
	if code := run([]string{"intent", "audit", "--ledger", intentOwn, "--json", "--root", root, "--url", "http://127.0.0.1:1", "--service", "svc"}, &out, &errb); code != 0 {
		t.Fatalf("intent audit --ledger exited %d: %s", code, errb.String())
	}
	for _, p := range []string{verifyOwn, intentOwn} {
		if fi, err := os.Stat(p); err != nil || fi.Size() == 0 {
			t.Errorf("--ledger %s: no rows appended (stat: %v)", p, err)
		}
	}
	for _, p := range []string{
		envVerify, envIntent,
		filepath.Join(scratch, "verify-ledger.jsonl"), filepath.Join(scratch, "intent-ledger.jsonl"),
		filepath.Join(realHome, ".fathom"),
	} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("--ledger must win: %s was written (stat: %v)", p, err)
		}
	}
}
