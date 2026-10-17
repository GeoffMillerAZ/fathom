package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

const genPlanFixture = `# Fixture
Baseline: 8c1e4f2a9b3d
Proof: certified
- [x] Grammar lands [FR-001] {id: G2-1, symbol: ParseMarkdownPlan, package: internal/verify, min_tier: verified}
- [~] Long haul {id: W-2, reason: "kit-side done, donor NOT deleted"}
- [-] Later {id: W-3, disposition: deferred, reason: "waiting on O6", trigger: "skip share <30%", needs: W-2}
- [ ] Open {id: W-4, symbol: GateRequireAll, skip_test: true, skip: "cmd oracle pending", weight: 3, accept: "WHEN asked, THE SYSTEM SHALL gate"}
- [x] Measured {id: W-5, oracle: lexical, path: docs/measurements/m.md, pattern: "p95 ≈ 329", max: 4, min: 1}
- [x] Positional untyped line
- [x] The harness API {id: W-6, symbol: RunHarness, library: true, reason: "gate harness: TestHarness runs it"}
- [x] Certified {id: W-7, symbol: ParseMarkdownPlan, certified_by: "TestA,TestB", strength: golden, needs: "W-5, other.md#X-1", ctx: "docs/a.md ADR-018"}
- [x] Benched {id: W-8, oracle: ledger, path: bench/ledger.jsonl, run_id: r-1, task: t-1, arm: on, min_trials: 3, criterion: H2-1, accept_verdict: pass}
- [x] Retired {id: W-9, gone: "pattern \"oldName\"", paths: "internal/a/, cmd/b.go", retires: "verb:old*, mcp:tool"}
- [x] Callers gone {id: W-10, gone: "callers Old", package: internal/x, tests: allowed, surface_change: "verb:new", oracle: lexical, path: docs/x.md, pattern: "y", eq: 0}
- [ ] Malformed values lint, the typed value round-trips {id: W-11, weight: heavy, max: lots, ctx: ""}
`

func TestGenPlanCue_RoundTrip(t *testing.T) {
	plan := verify.ParseMarkdownPlan("svc", []byte(genPlanFixture))
	doc := renderPlanCUE(plan)

	// Determinism.
	if doc != renderPlanCUE(plan) {
		t.Fatal("renderPlanCUE is not deterministic")
	}
	if !strings.Contains(doc, "Code generated") {
		t.Error("emitted CUE must carry the generated-only header")
	}

	cctx := cuecontext.New()
	if err := vetPlanCUE(cctx, planSchemaDef(t, cctx), doc); err != nil {
		t.Fatalf("emitted CUE fails schema/plan #Plan: %v\n%s", err, doc)
	}
	back, err := planFromCUE([]byte(doc))
	if err != nil {
		t.Fatalf("planFromCUE: %v\n%s", err, doc)
	}
	// The row's Markdown position is not part of the contract, so the
	// typed shadow does not carry it.
	stripMarkdownPosition(&plan)
	if !reflect.DeepEqual(plan, back) {
		t.Fatalf("round-trip loss: %s\ndoc:\n%s", taskDivergence(plan, back), doc)
	}
}

func TestCmdGenPlanCue_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.md")
	outPath := filepath.Join(dir, "plan.cue")
	if err := os.WriteFile(planPath, []byte(genPlanFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := cmdGenPlanCue([]string{"--plan", planPath, "--service", "svc", "--output", outPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "round-trip verified") {
		t.Errorf("stdout = %q, want round-trip confirmation", stdout.String())
	}
	emitted, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(emitted), `disposition: "deferred"`) {
		t.Error("v2 disposition missing from emitted CUE")
	}
	// Missing flags → operational exit 2.
	if code := cmdGenPlanCue(nil, &stdout, &stderr); code != 2 {
		t.Errorf("missing flags: exit %d, want 2", code)
	}
}

// planSchemaDef compiles schema/plan/schema.cue (the gated source, never a
// copy) and returns its #Plan definition: closed, so an emitted key the
// schema does not declare fails the vet.
func planSchemaDef(t *testing.T, cctx *cue.Context) cue.Value {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "schema", "plan", "schema.cue"))
	if err != nil {
		t.Fatalf("schema/plan/schema.cue unreadable (%v) — an unread schema must not pass as a satisfied one", err)
	}
	v := cctx.CompileBytes(src, cue.Filename("schema/plan/schema.cue"))
	if err := v.Err(); err != nil {
		t.Fatalf("schema/plan/schema.cue does not compile: %v", err)
	}
	def := v.LookupPath(cue.ParsePath("#Plan"))
	if !def.Exists() {
		t.Fatal("schema/plan/schema.cue has no #Plan definition")
	}
	return def
}

// vetPlanCUE unifies an emitted plan-CUE document with #Plan and validates
// it concretely.
func vetPlanCUE(cctx *cue.Context, def cue.Value, doc string) error {
	v := cctx.CompileString(doc)
	if err := v.Err(); err != nil {
		return err
	}
	return def.Unify(v).Validate(cue.Concrete(true))
}

// taskDivergence names the first task whose round trip differs and the
// fields that differ, so a red gate says which key the schema or the
// renderer dropped.
func taskDivergence(md, back verify.Plan) string {
	if md.Service != back.Service || md.Baseline != back.Baseline || md.Proof != back.Proof ||
		!reflect.DeepEqual(md.DeletedRows, back.DeletedRows) {
		return "plan header (service, baseline, proof)"
	}
	if len(md.Tasks) != len(back.Tasks) {
		return "task count " + strconv.Itoa(len(md.Tasks)) + " → " + strconv.Itoa(len(back.Tasks))
	}
	for i := range md.Tasks {
		a, b := reflect.ValueOf(md.Tasks[i]), reflect.ValueOf(back.Tasks[i])
		var fields []string
		for f := 0; f < a.NumField(); f++ {
			if !reflect.DeepEqual(a.Field(f).Interface(), b.Field(f).Interface()) {
				fields = append(fields, a.Type().Field(f).Name)
			}
		}
		if len(fields) > 0 {
			return md.Tasks[i].ID + " loses " + strings.Join(fields, ", ")
		}
	}
	return "no field differs (DeepEqual disagrees)"
}

// TestPlanSchema_IV6Negatives asserts schema/plan/testdata/invalid.cue's
// cases 8 to 10 (commented there so live `cue vet` stays green): the keys
// IV-6 declared are typed, not merely admitted.
func TestPlanSchema_IV6Negatives(t *testing.T) {
	cctx := cuecontext.New()
	src, err := os.ReadFile(filepath.Join("..", "..", "schema", "plan", "schema.cue"))
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		`bad: #Task & {id: "T-8", expect: {symbol: "X", certified_by: ["TestX"], strength: "strong"}}`,
		`bad: #Task & {id: "T-9", needs: "T-1"}`,
		`bad: #Task & {id: "T-10", depends_on: ["T-1"]}`,
	} {
		v := cctx.CompileString(string(src) + "\n" + bad)
		if v.Err() == nil && v.Validate() == nil {
			t.Errorf("%s: vetted clean, want a schema failure", bad)
		}
	}
}

// TestGenPlanCue_RoundTripsEveryActivePlan is IV-6's gate (ADR-074 decision
// 5, ADR-018 decision 3): every active plan under docs/plans/ (README.md and
// archive/ excluded) renders to CUE that vets against the closed
// schema/plan #Plan, and that CUE rebuilds the parsed plan with no loss.
// Baseline 2026-10-17: 49 of 75 plans lost data, and no gate saw it.
func TestGenPlanCue_RoundTripsEveryActivePlan(t *testing.T) {
	cctx := cuecontext.New()
	def := planSchemaDef(t, cctx)

	// The vet is closed: a key #Plan does not declare fails it, so a
	// parser key the schema never learned cannot pass silently.
	stray := "package plan\nservice: \"svc\"\ntasks: [{id: \"T-1\", undeclared_key: \"x\"}]\n"
	if vetPlanCUE(cctx, def, stray) == nil {
		t.Fatal("an undeclared key vetted clean — #Plan is not closed, so this gate proves nothing")
	}

	dir := filepath.Join("..", "..", "docs", "plans")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("docs/plans unreadable (%v) — an unread plan must not pass as a clean one", err)
	}
	var lost []string
	read := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") || name == "README.md" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("docs/plans/%s unreadable (%v) — an unread plan must not pass as a clean one", name, err)
		}
		read++
		plan := verify.ParseMarkdownPlan("fathom", raw)
		doc := renderPlanCUE(plan)
		if err := vetPlanCUE(cctx, def, doc); err != nil {
			lost = append(lost, fmt.Sprintf("docs/plans/%s: emitted CUE fails schema/plan #Plan: %v", name, err))
			continue
		}
		back, err := planFromCUE([]byte(doc))
		if err != nil {
			lost = append(lost, fmt.Sprintf("docs/plans/%s: emitted CUE does not decode: %v", name, err))
			continue
		}
		stripMarkdownPosition(&plan)
		if !reflect.DeepEqual(plan, back) {
			lost = append(lost, fmt.Sprintf("docs/plans/%s: %s", name, taskDivergence(plan, back)))
		}
	}
	if read == 0 {
		t.Fatal("no top-level plan was read — the round trip would pass vacuously")
	}
	if len(lost) > 0 {
		t.Fatalf("%d of %d active plans lose data through gen-plan-cue (ADR-018 decision 3 claims none):\n%s",
			len(lost), read, strings.Join(lost, "\n"))
	}
}
