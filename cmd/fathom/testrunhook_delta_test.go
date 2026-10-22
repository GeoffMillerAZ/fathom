package main

// testrunhook_delta_test.go — RO-23 (docs/plans/readouts.md phase 2): under
// [readouts] mode = rich a failing run's line compares it with the
// session's previous test-run rows (RO-35's `head`, `tree` and `scope`):
// tests fixed, still failing (since which HEAD), new, and `not run` for a
// previous failure outside this run's scope, a possible flake when one test
// failed and passed at the same HEAD and tree, `of the first 10` past the
// row's id cap, and `not compared` for a row missing a key. Driven through
// the readout hook, the entry that runs the test-run tier while [readouts]
// mode is on.

import (
	"fmt"
	"strings"
	"testing"
)

const (
	deltaMoney = "example.com/shop/money"
	deltaCart  = "example.com/shop/cart"
)

// goFailOutput is a `go test` output in which pkg fails the named tests.
func goFailOutput(pkg string, tests ...string) string {
	var b strings.Builder
	for _, name := range tests {
		fmt.Fprintf(&b, "--- FAIL: %s (0.00s)\n    x_test.go:7: boom\n", name)
	}
	fmt.Fprintf(&b, "FAIL\nFAIL\t%s\t0.140s\n", pkg)
	return b.String()
}

// goOKOutput is the result line of a package that passed.
func goOKOutput(pkg string) string { return "ok  \t" + pkg + "\t0.120s\n" }

// deltaStep is one run of a delta case: an optional working-tree edit (a
// new money.go body, which moves `tree`), an optional commit after it
// (which moves `head`), then the command through the readout hook.
type deltaStep struct {
	edit   string
	commit bool
	cmd    string
	output string
	pass   bool
}

func TestTestDeltaFromHookLogRows(t *testing.T) {
	const rich = "[readouts]\nmode = \"rich\"\n"
	elevenTests := []string{"TestA", "TestB", "TestC", "TestD", "TestE", "TestF", "TestG", "TestH", "TestI", "TestJ", "TestK"}
	cases := []struct {
		name     string
		cfg      string
		steps    []deltaStep
		want     []string // in the last step's output; {hN} is the Nth commit's short sha
		absent   []string
		rowDelta string // the last row's `delta` key; "" means absent
	}{
		{
			name: "fixed, still failing and new; a fix that moved the tree is no flake",
			cfg:  rich,
			steps: []deltaStep{
				{edit: "v1", commit: true, cmd: "go test ./...", output: goFailOutput(deltaMoney, "TestA", "TestB") + goOKOutput(deltaCart)},
				{edit: "v2", cmd: "go test ./...", output: goFailOutput(deltaMoney, "TestB", "TestC") + goOKOutput(deltaCart)},
			},
			want:     []string{"delta vs the last run: fixed money.TestA; still failing money.TestB (since {h0}); new money.TestC"},
			absent:   []string{"flake", "not compared"},
			rowDelta: "compared",
		},
		{
			name: "still failing names the HEAD its streak began at",
			cfg:  rich,
			steps: []deltaStep{
				{edit: "v1", commit: true, cmd: "go test ./money/", output: goFailOutput(deltaMoney, "TestB")},
				{edit: "v2", commit: true, cmd: "go test ./money/", output: goFailOutput(deltaMoney, "TestB")},
				{edit: "v3", cmd: "go test ./money/", output: goFailOutput(deltaMoney, "TestB")},
			},
			want:     []string{"still failing money.TestB (since {h0})"},
			absent:   []string{"{h1}", "flake"},
			rowDelta: "compared",
		},
		{
			name: "failed then passed at the same HEAD and tree is a possible flake",
			cfg:  rich,
			steps: []deltaStep{
				{edit: "v1", commit: true, cmd: "go test ./money/", output: goFailOutput(deltaMoney, "TestA", "TestB")},
				{cmd: "go test ./money/", output: goFailOutput(deltaMoney, "TestB")},
			},
			want:     []string{"fixed money.TestA (possible flake); still failing money.TestB (since {h0})"},
			rowDelta: "compared",
		},
		{
			name: "passed then failed at the same HEAD and tree is a possible flake",
			cfg:  rich,
			steps: []deltaStep{
				{edit: "v1", commit: true, cmd: "go test ./money/", output: goOKOutput(deltaMoney), pass: true},
				{cmd: "go test ./money/", output: goFailOutput(deltaMoney, "TestA")},
			},
			want:     []string{"delta vs the last run: new money.TestA (possible flake)"},
			rowDelta: "compared",
		},
		{
			name: "a previous run of another package scope is not compared",
			cfg:  rich,
			steps: []deltaStep{
				{edit: "v1", commit: true, cmd: "go test ./...", output: goFailOutput(deltaMoney, "TestA") + goOKOutput(deltaCart)},
				{cmd: "go test ./cart/", output: goFailOutput(deltaCart, "TestC")},
			},
			want:     []string{"TestC"},
			absent:   []string{"delta vs", "fixed", "new cart."},
			rowDelta: "not-compared",
		},
		{
			name: "a previous run with another filter is not compared",
			cfg:  rich,
			steps: []deltaStep{
				{edit: "v1", commit: true, cmd: "go test ./money/", output: goFailOutput(deltaMoney, "TestA", "TestB")},
				{edit: "v2", cmd: "go test ./money/ -run TestB", output: goFailOutput(deltaMoney, "TestB")},
			},
			want:     []string{"TestB"},
			absent:   []string{"delta vs", "fixed"},
			rowDelta: "not-compared",
		},
		{
			name: "a previous row at the id cap compares only the first 10",
			cfg:  rich,
			steps: []deltaStep{
				{edit: "v1", commit: true, cmd: "go test ./money/", output: goFailOutput(deltaMoney, elevenTests...)},
				{edit: "v2", cmd: "go test ./money/", output: goFailOutput(deltaMoney, "TestA", "TestL")},
			},
			want:     []string{"delta vs the last run (of the first 10 it logged): fixed money.TestB, money.TestC, money.TestD, +6 more; still failing money.TestA (since {h0}); 1 not compared"},
			absent:   []string{"new money."},
			rowDelta: "compared",
		},
		{
			name: "a previous row missing a key is not compared",
			cfg:  rich,
			steps: []deltaStep{
				{edit: "v1", cmd: "go test ./money/", output: goFailOutput(deltaMoney, "TestA")}, // no commit yet: no head, no tree
				{commit: true, cmd: "go test ./money/", output: goFailOutput(deltaMoney, "TestA")},
			},
			want:     []string{"TestA"},
			absent:   []string{"delta vs", "still failing"},
			rowDelta: "not-compared",
		},
		{
			name: "the session's first run carries no delta",
			cfg:  rich,
			steps: []deltaStep{
				{edit: "v1", commit: true, cmd: "go test ./money/", output: goFailOutput(deltaMoney, "TestA")},
			},
			want:     []string{"TestA"},
			absent:   []string{"delta vs"},
			rowDelta: "not-compared",
		},
		{
			name: "outside rich mode the line carries no delta and the row no key",
			cfg:  "[readouts]\nmode = \"observe\"\n",
			steps: []deltaStep{
				{edit: "v1", commit: true, cmd: "go test ./money/", output: goFailOutput(deltaMoney, "TestA", "TestB")},
				{edit: "v2", cmd: "go test ./money/", output: goFailOutput(deltaMoney, "TestB")},
			},
			want:   []string{"TestB"},
			absent: []string{"delta vs"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := foldEnv(t, c.cfg)
			var heads []string
			var out string
			for i, s := range c.steps {
				if s.edit != "" {
					writeRepoFile(t, repo, "money/money.go", "package money\n\n// "+s.edit+"\n")
				}
				if s.commit {
					heads = append(heads, commitAll(t, repo))
				}
				event, payload := "tool-failure", testRunFailurePayload(repo, s.cmd, s.output)
				if s.pass {
					event, payload = "post-tool", testRunPostPayload(repo, s.cmd, s.output)
				}
				_, out, _, _ = runReadoutHook(t, []string{"--event", event}, payload)
				if rows := foldRows(t); len(rows) != i+1 {
					t.Fatalf("step %d: %d test-run rows, want %d", i, len(rows), i+1)
				}
			}
			expand := func(s string) string {
				for i, h := range heads {
					s = strings.ReplaceAll(s, fmt.Sprintf("{h%d}", i), h[:7])
				}
				return s
			}
			for _, w := range c.want {
				if !strings.Contains(out, expand(w)) {
					t.Errorf("line lacks %q:\n%s", expand(w), out)
				}
			}
			for _, a := range c.absent {
				if strings.Contains(out, expand(a)) {
					t.Errorf("line carries %q:\n%s", expand(a), out)
				}
			}
			rows := foldRows(t)
			if got := rows[len(rows)-1]["delta"]; got != c.rowDelta {
				t.Errorf("row delta = %q, want %q", got, c.rowDelta)
			}
			if !strings.Contains(out, "full output: workspace/tmp/test-run/") {
				t.Errorf("line lost the saved-output path:\n%s", out)
			}
		})
	}
}
