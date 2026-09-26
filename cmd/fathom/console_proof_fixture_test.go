package main

// console_proof_fixture_test.go — the fixture fleet's fourth service,
// proof-service: a real git repository built at test time with dated
// commits, so the Proof pages (console wave C, OC-21..OC-26) have what the
// three fauxtown services cannot give them — plans under docs/plans with a
// git history, tests that reach the claimed symbols (one directly, one
// through a helper), a lexical-oracle row, an intent corpus with an
// attested certificate, and a change range for the conservation report.
//
// The repository is built once per test binary, inside the fixture store's
// own once (console_fixture_test.go), and ingested in local mode beside the
// fauxtown services. It carries no go.mod, like them: the typed pass does
// not apply, and the syntax graph is what verify reads.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

const proofServiceName = "proof-service"

// The fixture fleet is small enough to verify in full inside one request:
// the pages' wait budgets are raised so every test sees the gate's
// verdicts, never a page still verifying (production waits a bounded time
// and says what is still pending).
func init() {
	consolePlansPortfolioWait = time.Minute
	consolePlansPlanWait = time.Minute
}

// consoleFixtureRepo is a fixture service built as a git repository rather
// than read from fauxtown: Build writes and commits it under dir and
// returns its root; After runs once the store holds it.
type consoleFixtureRepo struct {
	Build func(dir string) (string, error)
	After func(root, storePath string) error
}

// consoleFixtureRepos are the fixture services that are real repositories.
var consoleFixtureRepos = map[string]consoleFixtureRepo{
	proofServiceName: {Build: buildProofServiceRepo, After: attestProofService},
}

// consoleFixtureRepoRoots records each built repository's root.
var consoleFixtureRepoRoots = map[string]string{}

// proofServiceRoot is proof-service's root in the fixture store.
func proofServiceRoot(t *testing.T) string {
	t.Helper()
	consoleFixtureStorePath(t)
	root := consoleFixtureRepoRoots[proofServiceName]
	if root == "" {
		t.Fatal("proof-service was not built")
	}
	return root
}

// The proof-service commits, oldest first. Each writes its files (a nil
// value deletes one) and commits at its date.
var proofServiceCommits = []struct {
	when  time.Time
	msg   string
	files map[string]string
}{
	{
		when: time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC), msg: "settle: first cut",
		files: map[string]string{
			"settle.go":                 proofSettleV1,
			"settle_test.go":            proofSettleTestV1,
			"README.md":                 proofReadmeV1,
			"docs/plans/settlement.md":  proofPlanV1,
			"docs/intent/README.md":     "# Router\n\ndocs/intent/invariants.md and docs/intent/incidents.md are the living docs.\n",
			"docs/intent/invariants.md": proofInvariants,
			"docs/intent/incidents.md":  proofIncidents,
			"tests/proof.cue":           proofManifest,
		},
	},
	{
		when: time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC), msg: "settle: claim S-1, scope grows",
		files: map[string]string{"docs/plans/settlement.md": proofPlanV2},
	},
	{
		when: time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC), msg: "reconcile: add, plan grows",
		files: map[string]string{
			"settle.go":                proofSettleV2,
			"settle_test.go":           proofSettleTestV2,
			"README.md":                proofReadmeV2,
			"docs/plans/settlement.md": proofPlanV3,
			"docs/plans/audit.md":      proofAuditPlan,
		},
	},
	{
		when: time.Date(2026, 9, 22, 9, 0, 0, 0, time.UTC), msg: "settle: reject negative amounts",
		files: map[string]string{"settle.go": proofSettleV3},
	},
}

// buildProofServiceRepo writes and commits proof-service under dir.
func buildProofServiceRepo(dir string) (string, error) {
	root := filepath.Join(dir, proofServiceName)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	if err := proofGit(root, proofServiceCommits[0].when, "init", "-q"); err != nil {
		return "", err
	}
	for _, c := range proofServiceCommits {
		for rel, body := range c.files {
			full := filepath.Join(root, rel)
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return "", err
			}
			if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
				return "", err
			}
		}
		if err := proofGit(root, c.when, "add", "-A"); err != nil {
			return "", err
		}
		if err := proofGit(root, c.when, "commit", "-q", "-m", c.msg); err != nil {
			return "", err
		}
	}
	consoleFixtureRepoRoots[proofServiceName] = root
	return root, nil
}

// attestProofService ingests one passing run of the proof suite bound to
// HEAD, so TestSettle_Balances's "certifies: invariant I1" marker compiles
// to a live certificate while I2 and INC-1 stay gaps.
func attestProofService(root, storePath string) error {
	head, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return fmt.Errorf("proof-service HEAD: %w", err)
	}
	results := filepath.Join(filepath.Dir(root), "proof-run.json")
	if err := os.WriteFile(results, []byte(proofRunEnvelope(strings.TrimSpace(string(head)))), 0o644); err != nil {
		return err
	}
	var out, errb bytes.Buffer
	if code := cmdTestsIngest([]string{"--service", proofServiceName, "--store", storePath, "--manifest", root, "--results", results}, &out, &errb); code != 0 {
		return fmt.Errorf("tests ingest proof-service: %d %s", code, errb.String())
	}
	return nil
}

func proofGit(dir string, when time.Time, args ...string) error {
	cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}, args...)...)
	cmd.Dir = dir
	ts := when.Format(time.RFC3339)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.com",
		"GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.com",
		"GIT_AUTHOR_DATE="+ts, "GIT_COMMITTER_DATE="+ts,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %v: %v: %s", args, err, out)
	}
	return nil
}

func proofRunEnvelope(commit string) string {
	return `{
  "suite": {"component": "proof", "name": "unit", "class": "unit"},
  "key": "proof-1",
  "verdict": "pass",
  "cases": [
    {"id": "1", "name": "TestSettle_Balances", "status": "pass", "duration": "1ms", "attempts": 1},
    {"id": "2", "name": "TestReconcile_Matches", "status": "pass", "duration": "1ms", "attempts": 1}
  ],
  "totals": {"run": 2, "passed": 2, "failed": 0, "skipped": 0, "flaked": 0},
  "timing": {"startedAt": "2026-09-24T00:00:00Z", "wallClock": "2ms"},
  "env": {"toolchainDigest": "x", "runnerClass": "y", "arch": "arm64", "os": "darwin", "commit": "` + commit + `", "dirty": false},
  "artifacts": []
}`
}

const proofManifest = `
component: "proof"
suites: unit: {
	class:  "unit"
	runner: "go-test"
	inputs: paths: ["."]
	needs: []
	runtime: {}
	flake: {}
	tier: "blocking"
}
`

const proofInvariants = "# Invariants\n\n| # | Statement | Enforced by |\n|---|---|---|\n| I1 | A settled ledger balances to zero | test (`TestSettle_Balances`) |\n| I2 | A reconciliation never drops an entry | test (`TestReconcile_Matches`) |\n"

const proofIncidents = "# Incidents\n\n## Open\n\n### INC-1\n\nA settlement ran twice.\n"

const proofSettleV1 = `package proof

// Settle balances the ledger for one amount.
func Settle(amount int) int {
	if err := validate(amount); err != nil {
		return 0
	}
	return amount
}

func validate(amount int) error {
	return nil
}
`

const proofSettleV2 = proofSettleV1 + `
// Reconcile compares two ledgers entry by entry.
func Reconcile(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Audit exports the settlement trail.
func Audit() string {
	return "trail"
}
`

const proofSettleV3 = `package proof

import "errors"

// Settle balances the ledger for one amount.
func Settle(amount int) int {
	if err := validate(amount); err != nil {
		return 0
	}
	if amount < 0 {
		return -amount
	}
	return amount
}

func validate(amount int) error {
	if amount == 0 {
		return errors.New("zero amount")
	}
	return nil
}

// Reconcile compares two ledgers entry by entry.
func Reconcile(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Audit exports the settlement trail.
func Audit() string {
	return "trail"
}
`

const proofSettleTestV1 = `package proof

import "testing"

// certifies: invariant I1 (asserts)
func TestSettle_Balances(t *testing.T) {
	if Settle(5) != 5 {
		t.Fatal("unbalanced")
	}
}
`

const proofSettleTestV2 = proofSettleTestV1 + `
func TestReconcile_Matches(t *testing.T) {
	if !reconcileBoth([]int{1}, []int{1}) {
		t.Fatal("mismatch")
	}
}

func reconcileBoth(a, b []int) bool {
	return Reconcile(a, b)
}
`

const proofReadmeV1 = "# proof-service\n\nThe fixture ledger.\n"

const proofReadmeV2 = "# proof-service\n\nThe fixture ledger.\n\n## Settlement window\n\nSettlement runs nightly.\n\nlatency: measured at 0.4 s over 30 runs.\n"

const proofPlanV1 = `# Settlement ledger (fixture plan)

- [ ] Settle balances the ledger {id: S-1, symbol: Settle, reachable_from: TestSettle_Balances}
- [ ] Reconcile compares two ledgers {id: S-2, symbol: Reconcile}
- [ ] The runbook names the settlement window {id: S-3, oracle: lexical, path: README.md, pattern: "^## Settlement window$"}
`

const proofPlanV2 = `# Settlement ledger (fixture plan)

- [x] Settle balances the ledger {id: S-1, symbol: Settle, reachable_from: TestSettle_Balances}
- [ ] Reconcile compares two ledgers {id: S-2, symbol: Reconcile}
- [ ] The runbook names the settlement window {id: S-3, oracle: lexical, path: README.md, pattern: "^## Settlement window$"}
- [ ] Audit trail exported {id: S-5, symbol: Audit}
- [-] Multi-currency settlement {id: S-7, disposition: deferred, reason: "no second currency yet", trigger: "a second currency ships"}
`

const proofPlanV3 = `# Settlement ledger (fixture plan)

## A. Core

- [x] Settle balances the ledger {id: S-1, symbol: Settle, reachable_from: TestSettle_Balances}
- [x] Reconcile compares two ledgers {id: S-2, symbol: Reconcile}
- [x] The runbook names the settlement window {id: S-3, oracle: lexical, path: README.md, pattern: "^## Settlement window$"}
- [x] A settler that was never written {id: S-4, symbol: NoSuchSettler}
- [ ] Audit trail exported {id: S-5, symbol: Audit}
- [~] Nightly batch in progress {id: S-6, symbol: NightlyBatch}

## B. Measurements

- [x] (op) S-M1: settlement latency measured {id: S-M1, oracle: lexical, path: README.md, pattern: "^latency: measured", skip: "measured: README.md"}
- [ ] (op) S-M2: thirty days of settlements without drift {id: S-M2, skip: "operator measurement: 30-day window"}

## Deferred

- [-] Multi-currency settlement {id: S-7, disposition: deferred, reason: "no second currency yet", trigger: "a second currency ships"}
- [-] Blockchain anchoring {id: S-8, disposition: refused, reason: "out of scope for a ledger"}
`

const proofAuditPlan = `# Audit (fixture plan)

- [x] Validation guards every settlement {id: AU-1, symbol: validate, skip_test: true, skip: "fixture: exercised through Settle"}
- [ ] Audit retention policy {id: AU-2, symbol: Audit}
`

// pinConsoleVerbClocks pins the verbs' own clocks to the fixture's, as the
// fixture already does for the fleet report: a page and its verb must be
// compared at one instant.
func pinConsoleVerbClocks(t *testing.T, f *consoleFixture) {
	t.Helper()
	prev := certifyGapsNow
	certifyGapsNow = func() time.Time { return f.now }
	t.Cleanup(func() { certifyGapsNow = prev })
}

// runVerbJSON runs a chip's exact command as consoleVerbArgs reads it —
// with the fixture's --url unless the chip runs as shown, and without it
// when the verb defines no --url (the Evidence pages' store and ledger
// verbs) — and returns its decoded JSON. doctor's exit 1 is its verdict (a
// check failed), not a failure to run.
func runVerbJSON(t *testing.T, f *consoleFixture, verb string) any {
	t.Helper()
	args, dir := consoleVerbArgs(t, f, verb)
	var out, errb bytes.Buffer
	code := runIn(t, dir, args, &out, &errb)
	if n := len(args); code != 0 && n >= 2 && args[n-2] == "--url" && strings.Contains(errb.String(), "flag provided but not defined: -url") {
		out.Reset()
		errb.Reset()
		code = runIn(t, dir, args[:n-2], &out, &errb)
	}
	if code != 0 && !(code == 1 && strings.HasPrefix(verb, "fathom doctor ")) {
		t.Fatalf("%s exited %d: %s", verb, code, errb.String())
	}
	var v any
	if err := json.Unmarshal(out.Bytes(), &v); err != nil {
		t.Fatalf("%s: %v\n%s", verb, err, out.String())
	}
	return v
}

// consoleVerbArgs is how the parity test runs a page's verb against the
// fixture: a daemon verb gets the fixture's --url (its default, the local
// daemon, is the page's own in the normal setup); a store-direct verb
// (certify, conserve) runs exactly as its chip reads — the chip must name
// the daemon's --store itself, since as copied it would otherwise read its
// default store, not the page's. A verb that reads the working tree from
// its directory (verify, conserve, plans with an explicit --plan) runs in
// the repository's root, as an operator runs it. A chip's quoting is a
// shell's (shellFields).
func consoleVerbArgs(t *testing.T, f *consoleFixture, verb string) (args []string, dir string) {
	t.Helper()
	fields := shellFields(verb)[1:]
	name := fields[0]
	if len(fields) > 1 && !strings.HasPrefix(fields[1], "-") {
		name += " " + fields[1]
	}
	switch name {
	case "certify status", "certify gaps", "conserve":
		if !strings.Contains(verb, " --store "+consoleFixtureStorePath(t)+" ") {
			t.Fatalf("`%s` opens a store directly but its chip names no --store %s: copied as shown it reads another store", verb, consoleFixtureStorePath(t))
		}
		args = append([]string{}, fields...)
	case "metrics traffic", "scope list", "report evidence", "fleet receipts", "intent audit", "doctor":
		// The workspace tabs' verbs (console wave E) name every input they
		// read — the root, the store, the daemon, the window — and run
		// exactly as the chip reads.
		args = append([]string{}, fields...)
	default:
		args = append(append([]string{}, fields...), "--url", f.ts.URL)
	}
	svc, hasPlan := "", false
	for i, fld := range fields {
		if fld == "--service" && i+1 < len(fields) {
			svc = fields[i+1]
		}
		hasPlan = hasPlan || fld == "--plan"
	}
	if root := consoleFixtureRepoRoots[svc]; root != "" && (name == "conserve" || name == "verify" || ((name == "plans status" || name == "plans next") && hasPlan)) {
		dir = root
	}
	return args, dir
}

// runIn runs the CLI in dir (the current directory when empty) and restores
// the working directory before returning.
func runIn(t *testing.T, dir string, args []string, stdout, stderr *bytes.Buffer) int {
	t.Helper()
	if dir != "" {
		prev, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(dir); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chdir(prev) }()
	}
	return run(args, stdout, stderr)
}

// assertProofMetricsMatchVerbs holds the proof ledger's shared metrics to
// their verbs on the fixture fleet (OC-9): plans.claimed_unproven and
// plans.proven are `plans status --json` summed over every service with
// plans; certify.gaps and certify.live are `certify gaps --json`'s summary
// summed over every service.
func assertProofMetricsMatchVerbs(t *testing.T, f *consoleFixture, value func(console.MetricID) console.Value[int]) {
	t.Helper()
	var unproven, proven int
	var out, errb bytes.Buffer
	if code := run([]string{"plans", "status", "--service", proofServiceName, "--json", "--url", f.ts.URL}, &out, &errb); code != 0 {
		t.Fatalf("plans status: %d %s", code, errb.String())
	}
	var rollups []planRollup
	if err := json.Unmarshal(out.Bytes(), &rollups); err != nil {
		t.Fatal(err)
	}
	for _, r := range rollups {
		unproven += r.ClaimedUnproven
		proven += r.Proven
	}
	if v := value(console.MetricPlansUnproven); v.State != console.StatePresent || v.V != unproven || unproven == 0 {
		t.Errorf("plans.claimed_unproven = %+v, want plans status's %d", v, unproven)
	}
	if v := value(console.MetricPlansProven); v.V != proven {
		t.Errorf("plans.proven = %+v, want plans status's %d", v, proven)
	}
	var gaps, live int
	for _, svc := range consoleFixtureServices {
		out.Reset()
		errb.Reset()
		if code := run([]string{"certify", "gaps", "--service", svc, "--json", "--store", consoleFixtureStorePath(t)}, &out, &errb); code != 0 {
			t.Fatalf("certify gaps %s: %d %s", svc, code, errb.String())
		}
		var doc certifyGapsPayload
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			t.Fatal(err)
		}
		gaps += doc.Summary.Gaps
		live += doc.Summary.Live
	}
	if v := value(console.MetricCertGaps); v.V != gaps || gaps == 0 {
		t.Errorf("certify.gaps = %+v, want certify gaps's %d", v, gaps)
	}
	if v := value(console.MetricCertLive); v.V != live {
		t.Errorf("certify.live = %+v, want certify gaps's %d", v, live)
	}
}
