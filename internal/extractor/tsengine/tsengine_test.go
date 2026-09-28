package tsengine

import (
	"bytes"
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The critic-B1 regression pair. Pathological input: `({[` x 50,000 (150KB —
// under the 2MB extraction cap, exactly a truncated minified bundle's shape)
// wedged the original engine for 52.9s (x200,000 for 11+ minutes) where the
// CGO incumbent finishes in ~49ms. Root cause, established by direct
// measurement during the fix (updating the critic's initial parse-phase
// attribution): the PARSE of this input completes in ~70ms — it is the
// QUERY CURSOR's scan of the resulting error tree that runs 45s+ unbounded.
// Both phases now carry a guest-side budget (grammars/budget.c progress
// callbacks) plus the host-side hard backstop for any time spent between
// progress checks (see ParseHardBackstop's doc).

// TestParser_GuestBudgetBoundsPathologicalParse: a parse budget expiring
// inside the parse's checked phase cancels via tree-sitter's own mechanism;
// the Parser SURVIVES (a cancelled parse only requires the reset Parse
// performs at entry). A 1ms budget guarantees the deadline is already
// expired at the parse's first progress check regardless of machine speed.
func TestParser_GuestBudgetBoundsPathologicalParse(t *testing.T) {
	savedTimeout, savedBackstop := ParseTimeout, ParseHardBackstop
	ParseTimeout = 1 * time.Millisecond
	ParseHardBackstop = 10 * time.Minute // effectively disabled — isolate the guest path
	defer func() { ParseTimeout, ParseHardBackstop = savedTimeout, savedBackstop }()

	ctx := context.Background()
	p, err := NewParser(ctx, "javascript", "tree_sitter_javascript")
	if err != nil {
		t.Fatalf("NewParser: %v", err)
	}
	defer p.Close()

	pathological := bytes.Repeat([]byte("({["), 50000) // 150KB

	start := time.Now()
	_, err = p.Parse(pathological)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Parse of pathological input returned no error — the guest budget did not fire")
	}
	if elapsed > 30*time.Second {
		t.Fatalf("Parse took %v — the guest cancel did not bound the parse", elapsed)
	}

	// The SAME parser must recover after a guest-side cancel.
	ParseTimeout = 10 * time.Second
	tree, err := p.Parse([]byte("function ok() { return 1; }\n"))
	if err != nil {
		t.Fatalf("Parse on the same parser after guest cancel: %v", err)
	}
	tree.Close()
}

// TestQuery_GuestBudgetBoundsPathologicalQueryRun: the load-bearing bound —
// the query scan over the pathological input's error tree is where the 45s+
// actually went. The guest budget cancels it; cancellation surfaces as an
// ERROR (never a silently truncated match set, since the C API reports
// cancellation as ordinary exhaustion); the Parser survives.
//
// The bound is asserted in PROGRESS, not time, so the test measures the
// cancel and not the machine: the guest reads its clock once when the scan
// is armed and once per progress callback (every 100 tree-sitter query
// operations), so a test clock that advances exactly 1ms per reading turns
// a budget of B ms into exactly B callbacks, whatever the load, -race or
// core type. (Wall- and CPU-time versions of this test flaked under the
// pre-push gate: the same cancel point cost 8s of CPU unloaded and 34.5s on
// a loaded efficiency core.) The whole unbounded scan is ~3,000 callbacks
// (measured; ~43s of CPU). The test proves the ratio itself: a scan whose
// clock is held still is still running at stallReads callbacks, and the
// budgeted scan stopped at budgetMS — at most a tenth of that.
func TestQuery_GuestBudgetBoundsPathologicalQueryRun(t *testing.T) {
	const budgetMS = 5
	const stallReads = 10 * budgetMS

	savedTimeout, savedBackstop := ParseTimeout, ParseHardBackstop
	ParseTimeout = time.Hour // let the parse complete (it takes ~70ms)
	// Effectively disabled: it is wall clock, so under load it could abandon
	// a healthy scan mid-way and fail this test for the machine's sake. A
	// cancel that never fires shows as the whole scan completing with no
	// error (~43s+ of CPU), which the run helper rejects.
	ParseHardBackstop = 10 * time.Minute
	defer func() { ParseTimeout, ParseHardBackstop = savedTimeout, savedBackstop }()

	// The test clock: +1ms per reading, or held still while stalled until
	// the stallReads'th reading, when it jumps an hour ahead (so the stalled
	// scan's own budget can then cancel it).
	var reads, now atomic.Int64
	var stalled atomic.Bool
	guestNanotime = func() int64 {
		n := reads.Add(1)
		if stalled.Load() {
			if n >= stallReads {
				return now.Add(int64(time.Hour))
			}
			return now.Load()
		}
		return now.Add(int64(time.Millisecond))
	}
	defer func() { guestNanotime = nil }()

	ctx := context.Background()
	p, err := NewParser(ctx, "javascript", "tree_sitter_javascript")
	if err != nil {
		t.Fatalf("NewParser: %v", err)
	}
	defer p.Close()

	pathological := bytes.Repeat([]byte("({["), 50000)
	tree, err := p.Parse(pathological)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer tree.Close()

	q, err := p.CompileQuery(`(function_declaration name: (identifier) @name) @definition.function
(call_expression function: (_) @reference.call)`)
	if err != nil {
		t.Fatalf("CompileQuery: %v", err)
	}
	defer q.Close()

	run := func(label string) int64 {
		t.Helper()
		reads.Store(0)
		_, err := q.Run(tree)
		got := reads.Load()
		if err == nil {
			t.Fatalf("%s: Run over the pathological tree returned no error after %d clock readings — the query budget did not fire", label, got)
		}
		if !strings.Contains(err.Error(), "query aborted after") {
			t.Fatalf("%s: Run error = %v, want the guest budget's cancel (not the hard backstop)", label, err)
		}
		return got
	}

	// 1. The scan is long: with the clock held still it is still running
	// at the stallReads'th progress check.
	stalled.Store(true)
	if got := run("stalled clock"); got != stallReads {
		t.Fatalf("stalled clock: cancelled after %d readings, want exactly %d — the scan ended before that many progress checks, so a %d-callback budget proves nothing", got, stallReads, budgetMS)
	}

	// 2. The budget cancels it at exactly budgetMS progress checks (+1: the
	// reading that arms the deadline) — a tenth of what the scan provably
	// needs, and ~1/600 of the whole scan.
	stalled.Store(false)
	ParseTimeout = budgetMS * time.Millisecond
	if got := run("budgeted"); got != budgetMS+1 {
		t.Fatalf("budgeted: cancelled after %d clock readings, want exactly %d (arm + %d progress checks)", got, budgetMS+1, budgetMS)
	}

	// The SAME parser must still work for a healthy file.
	ParseTimeout = 10 * time.Second
	tree2, err := p.Parse([]byte("function ok() { return 1; }\n"))
	if err != nil {
		t.Fatalf("Parse after query cancel: %v", err)
	}
	defer tree2.Close()
	matches, err := q.Run(tree2)
	if err != nil {
		t.Fatalf("Run after query cancel: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("healthy file produced no matches after query cancel")
	}
}

// TestEngine_HardBackstopAbandonsRunawayCall: when a guest budget can never
// fire (simulated with an effectively-infinite ParseTimeout — the real-world
// case is time spent between progress checks), the host-side backstop
// abandons the runaway call within its bound. The abandoned Parser is dead;
// a fresh one must work immediately while the runaway burns out in the
// background (its reaper closes the module when tree-sitter finishes).
func TestEngine_HardBackstopAbandonsRunawayCall(t *testing.T) {
	savedTimeout, savedBackstop := ParseTimeout, ParseHardBackstop
	ParseTimeout = time.Hour // no guest budget will ever fire
	// The setup parse (~70ms of work) must not race a 2s wall clock on a
	// loaded machine; the 2s backstop is armed only for the runaway scan.
	ParseHardBackstop = 10 * time.Minute
	defer func() { ParseTimeout, ParseHardBackstop = savedTimeout, savedBackstop }()

	ctx := context.Background()
	p, err := NewParser(ctx, "javascript", "tree_sitter_javascript")
	if err != nil {
		t.Fatalf("NewParser: %v", err)
	}
	defer p.Close()

	pathological := bytes.Repeat([]byte("({["), 50000)
	tree, err := p.Parse(pathological) // completes in ~70ms, under the backstop
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer tree.Close()

	q, err := p.CompileQuery(`(call_expression function: (_) @reference.call)`)
	if err != nil {
		t.Fatalf("CompileQuery: %v", err)
	}
	defer q.Close()

	ParseHardBackstop = 2 * time.Second
	start := time.Now()
	_, err = q.Run(tree) // the 45s+ scan; guest budget disabled -> backstop must fire
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Run returned no error — the hard backstop did not fire")
	}
	if elapsed > 30*time.Second {
		t.Fatalf("Run took %v — the 2s hard backstop did not bound the call", elapsed)
	}

	// The abandoned parser is dead — a fresh Parser must work immediately.
	ParseHardBackstop = 10 * time.Minute
	p2, err := NewParser(ctx, "javascript", "tree_sitter_javascript")
	if err != nil {
		t.Fatalf("NewParser after abandonment: %v", err)
	}
	defer p2.Close()
	tree2, err := p2.Parse([]byte("function ok2() { return 2; }\n"))
	if err != nil {
		t.Fatalf("Parse on fresh parser after abandonment: %v", err)
	}
	tree2.Close()
}

func TestNewParser_UnknownGrammar(t *testing.T) {
	_, err := NewParser(context.Background(), "cobol", "tree_sitter_cobol")
	if err == nil {
		t.Fatal("expected error for unknown grammar, got nil")
	}
}

func TestNewParser_UnknownEntryPoint(t *testing.T) {
	_, err := NewParser(context.Background(), "python", "tree_sitter_nope")
	if err == nil {
		t.Fatal("expected error for unknown entry point, got nil")
	}
}

func TestParser_ParsePython(t *testing.T) {
	ctx := context.Background()
	p, err := NewParser(ctx, "python", "tree_sitter_python")
	if err != nil {
		t.Fatalf("NewParser: %v", err)
	}
	defer p.Close()

	src := []byte("class Bar:\n    def baz(self):\n        return 1\n\ndef foo(x):\n    return x + 1\n")
	tree, err := p.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer tree.Close()

	root, err := tree.Root()
	if err != nil {
		t.Fatalf("Root: %v", err)
	}
	if root.StartByte != 0 {
		t.Errorf("root.StartByte = %d, want 0", root.StartByte)
	}
	if int(root.EndByte) != len(src) {
		t.Errorf("root.EndByte = %d, want %d", root.EndByte, len(src))
	}

	q, err := p.CompileQuery(`
(function_definition name: (identifier) @name) @definition.function
(class_definition name: (identifier) @name) @definition.class
`)
	if err != nil {
		t.Fatalf("CompileQuery: %v", err)
	}
	defer q.Close()

	matches, err := q.Run(tree)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(matches) != 3 {
		t.Fatalf("len(matches) = %d, want 3 (Bar class, baz method, foo function): %+v", len(matches), matches)
	}

	var gotNames []string
	for _, m := range matches {
		for _, c := range m.Captures {
			if c.Name == "name" {
				gotNames = append(gotNames, c.Node.Text(src))
			}
		}
	}
	want := map[string]bool{"Bar": true, "baz": true, "foo": true}
	if len(gotNames) != 3 {
		t.Fatalf("captured names = %v, want 3 entries from %v", gotNames, want)
	}
	for _, n := range gotNames {
		if !want[n] {
			t.Errorf("unexpected captured name %q", n)
		}
	}
}

func TestParser_CompileQuery_BadQuery(t *testing.T) {
	ctx := context.Background()
	p, err := NewParser(ctx, "python", "tree_sitter_python")
	if err != nil {
		t.Fatalf("NewParser: %v", err)
	}
	defer p.Close()

	_, err = p.CompileQuery("(this_is_not_a_real_node_kind)")
	if err == nil {
		t.Fatal("expected error for malformed query, got nil")
	}
	if !strings.Contains(err.Error(), "query compile failed") {
		t.Errorf("error = %v, want a %q message", err, "query compile failed")
	}
}

func TestParser_Determinism(t *testing.T) {
	ctx := context.Background()
	src := []byte("def a():\n    pass\n\ndef b():\n    pass\n\ndef c():\n    pass\n")

	run := func() []string {
		p, err := NewParser(ctx, "python", "tree_sitter_python")
		if err != nil {
			t.Fatalf("NewParser: %v", err)
		}
		defer p.Close()
		tree, err := p.Parse(src)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		defer tree.Close()
		q, err := p.CompileQuery(`(function_definition name: (identifier) @name) @definition.function`)
		if err != nil {
			t.Fatalf("CompileQuery: %v", err)
		}
		defer q.Close()
		matches, err := q.Run(tree)
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		var names []string
		for _, m := range matches {
			for _, c := range m.Captures {
				if c.Name == "name" {
					names = append(names, c.Node.Text(src))
				}
			}
		}
		return names
	}

	first := run()
	for i := 0; i < 5; i++ {
		got := run()
		if len(got) != len(first) {
			t.Fatalf("run %d: got %v, want %v", i, got, first)
		}
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("run %d: got %v, want %v", i, got, first)
			}
		}
	}
}

func TestGrammars(t *testing.T) {
	names := Grammars()
	want := map[string]bool{"javascript": true, "typescript": true, "python": true, "ruby": true, "kotlin": true, "html": true, "css": true, "c": true, "csharp": true, "dockerfile": true, "sql": true, "bash": true, "java": true, "cpp": true}
	if len(names) != len(want) {
		t.Fatalf("Grammars() = %v, want %d entries", names, len(want))
	}
	for _, n := range names {
		if !want[n] {
			t.Errorf("unexpected grammar %q", n)
		}
	}
}

func TestParser_TypeScriptBothEntryPoints(t *testing.T) {
	ctx := context.Background()
	for _, entry := range []string{"tree_sitter_typescript", "tree_sitter_tsx"} {
		p, err := NewParser(ctx, "typescript", entry)
		if err != nil {
			t.Fatalf("NewParser(%s): %v", entry, err)
		}
		tree, err := p.Parse([]byte("interface Foo { x: number }\n"))
		if err != nil {
			p.Close()
			t.Fatalf("Parse(%s): %v", entry, err)
		}
		tree.Close()
		p.Close()
	}
}
