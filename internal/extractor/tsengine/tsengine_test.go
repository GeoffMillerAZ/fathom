package tsengine

import (
	"bytes"
	"context"
	"strings"
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
func TestQuery_GuestBudgetBoundsPathologicalQueryRun(t *testing.T) {
	savedTimeout, savedBackstop := ParseTimeout, ParseHardBackstop
	ParseTimeout = time.Hour // let the parse complete (it takes ~70ms)
	ParseHardBackstop = 10 * time.Minute
	defer func() { ParseTimeout, ParseHardBackstop = savedTimeout, savedBackstop }()

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

	// The bound is asserted on the CPU the scan burned, not on wall clock.
	// The guest budget is checked by tree-sitter's progress callback every
	// ~100 query operations, and on this tree an operation's cost grows as
	// the scan proceeds: a 300ms budget is observed at ~8s of guest CPU
	// (measured 8.25s, with or without RT-107's memory cap; a 1ms budget
	// fires at ~28ms), against ~43-53s for the whole unbounded scan. That is
	// CPU work, so on a loaded machine its wall time stretches with the
	// share of a core the test gets — the pre-push gate saw 49.7s wall for
	// the same ~8s — and a wall-clock bound here measured the machine, not
	// the cancel. The wall-clock guarantee under any load is the host-side
	// backstop's job (TestEngine_HardBackstopAbandonsRunawayCall); this test
	// isolates the guest cancel, so it bounds the work the cancel allows.
	// RUSAGE_SELF is process-wide: this must not run beside another wasm
	// scan (no test in this package is parallel, and none before this one
	// leaves an abandoned runaway burning).
	ParseTimeout = 300 * time.Millisecond
	cpu0 := processCPU(t)
	start := time.Now()
	_, err = q.Run(tree)
	elapsed := time.Since(start)
	burned := processCPU(t) - cpu0

	if err == nil {
		t.Fatal("Run over the pathological tree returned no error — the query budget did not fire")
	}
	t.Logf("query cancel: %v CPU, %v wall", burned, elapsed)
	// 30s, the bound this test always used, now in CPU: the cancelled scan
	// is ~8s on a performance core (11.8s measured with 3x the cores busy),
	// the whole scan 43s+ and more on a slower core, so 30s still separates
	// "the cancel fired" from "the scan ran out".
	if burned > 30*time.Second {
		t.Fatalf("Run burned %v CPU (%v wall) — the query cancel did not bound the scan (the whole unbounded scan is ~43-53s)", burned, elapsed)
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
	ParseHardBackstop = 2 * time.Second
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
	want := map[string]bool{"javascript": true, "typescript": true, "python": true, "ruby": true, "kotlin": true}
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
