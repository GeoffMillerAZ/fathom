package langbench

import (
	"os"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// queueSchemaSrc returns schema/langqueue's definitions (and, when withValid,
// the positive testdata) as one CUE source with the package clauses hoisted.
func queueSchemaSrc(t *testing.T, withValid bool) string {
	t.Helper()
	files := []string{"../../schema/langqueue/schema.cue"}
	if withValid {
		files = append(files, "../../schema/langqueue/testdata/valid.cue")
	}
	var parts []string
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		parts = append(parts, strings.Replace(string(b), "package langqueue\n", "", 1))
	}
	return strings.Join(parts, "\n")
}

const queueBase = `{id: "go", name: "Go", kind: "language", target_level: "L4",
	grammar_source: "tree-sitter/tree-sitter-go", licence: "MIT",
	oracle: ["ctags"], calibration_subjects: ["spf13/cobra"], depends_on: [],
	wave: 1, engine: "tags", plan_row: "LB-Q-go", worker_model: "sonnet"}`

// TestQueueSchema (LB-F108): the positive testdata validates concretely and
// every documented negative in testdata/invalid.cue is refused.
func TestQueueSchema(t *testing.T) {
	cctx := cuecontext.New()
	v := cctx.CompileString(queueSchemaSrc(t, true))
	if err := v.Err(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	if err := v.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("valid.cue should validate concretely: %v", err)
	}
	schema := queueSchemaSrc(t, false)
	ok := cctx.CompileString(schema + "\nok: #QueueEntry & " + queueBase + "\nok2: #QueueEntry & " + queueBase + " & {tier: \"T1\"}" + "\nok7: #QueueEntry & " + queueBase + " & {tier: \"T7\"}")
	if err := ok.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("the negatives' base must be valid (without tier, T1, and T7 observability as code): %v", err)
	}
	cases := map[string]string{
		"tier T8":                `{tier: "T8"}`,
		"tier is an oracle tier": `{tier: "A"}`,
		"tooling at L4":          `{kind: "tooling", target_level: "L4"}`,
		"plan_row mismatch":      `{plan_row: "LB-Q-golang"}`,
		"unknown field":          `{stars: 5}`,
		"uppercase id":           `{id: "Go", plan_row: "LB-Q-Go"}`,
		"engine off list":        `{engine: "scip"}`,
		"level past L4":          `{target_level: "L5"}`,
		"wave zero":              `{wave: 0}`,
		"empty name":             `{name: ""}`,
		"kind off list":          `{kind: "format"}`,
	}
	for name, patch := range cases {
		bad := cctx.CompileString(schema + "\nbad: #QueueEntry & " + queueBase + " & " + patch)
		if bad.Err() == nil && bad.Validate(cue.Concrete(true)) == nil {
			t.Errorf("%s: schema accepted %s", name, patch)
		}
	}
	// the documented negatives must be listed in invalid.cue
	inv, err := os.ReadFile("../../schema/langqueue/testdata/invalid.cue")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`tier: "T8"`, `tier: "A"`, `kind: "tooling"`, `plan_row`, `unknown field`} {
		if !strings.Contains(string(inv), want) {
			t.Errorf("invalid.cue does not document %q", want)
		}
	}
}

// TestQueueSchemaCommittedQueue (LB-F108): the committed queue vets
// concretely against #Queue through cuelang.org/go, so the check runs even
// when the cue CLI is absent.
func TestQueueSchemaCommittedQueue(t *testing.T) {
	b, err := os.ReadFile("../../docs/plans/language-breadth.queue.json")
	if err != nil {
		t.Fatal(err)
	}
	cctx := cuecontext.New()
	schema := cctx.CompileString(queueSchemaSrc(t, false))
	if err := schema.Err(); err != nil {
		t.Fatal(err)
	}
	data := cctx.CompileBytes(b) // a top-level JSON list is CUE-compatible as is
	if err := data.Err(); err != nil {
		t.Fatal(err)
	}
	got := schema.LookupPath(cue.ParsePath("#Queue")).Unify(data)
	if err := got.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("docs/plans/language-breadth.queue.json does not satisfy #Queue: %v", err)
	}
}
