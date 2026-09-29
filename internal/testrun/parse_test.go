package testrun

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func failureIDs(r Run) []string {
	out := make([]string, 0, len(r.Failures))
	for _, f := range r.Failures {
		out = append(out, f.ID())
	}
	return out
}

func frameOf(r Run, id string) []Frame {
	for _, f := range r.Failures {
		if f.ID() == id {
			return f.Frames
		}
	}
	return nil
}

// TestParseTestRunGoPlainJSONAndJUnit pins TR-3 (docs/plans/test-run-hook.md,
// design section 3, tier 0): plain `go test`, `go test -json` and JUnit XML
// give the same failing test ids, panics with their project frames, file:line
// frames and build errors. The go fixtures are real `go test` output from a
// three-package module (paths anonymised); the JUnit ones follow gotestsum's
// shape.
func TestParseTestRunGoPlainJSONAndJUnit(t *testing.T) {
	wantFails := []string{
		"example.com/shop/cart.TestPanics",
		"example.com/shop/cart.TestTotal",
		"example.com/shop/money.TestRound",
		"example.com/shop/money.TestTable",
		"example.com/shop/money.TestTable/half",
	}
	wantPkgs := []PackageResult{
		{"example.com/shop/broken", StatusBuildFailed},
		{"example.com/shop/cart", StatusFail},
		{"example.com/shop/empty", StatusNoTests},
		{"example.com/shop/money", StatusFail},
	}
	wantBuild := []BuildError{{Package: "example.com/shop/broken", File: "broken/broken.go", Line: 3, Col: 23, Message: "undefined: undefinedThing"}}
	wantPanic := []Panic{{
		Package: "example.com/shop/cart", Test: "TestPanics",
		Message: "assignment to entry in nil map [recovered, repanicked]",
		Frames:  []Frame{{File: "/work/shop/cart/cart_test.go", Line: 13}},
	}}

	for _, tc := range []struct {
		fixture string
		runner  Runner
	}{
		{"go-plain-fail.txt", RunnerGo},
		{"go-json-fail.jsonl", RunnerGoJSON},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			r := ParseTestRun(fixture(t, tc.fixture))
			if r.Runner != tc.runner {
				t.Errorf("runner = %q, want %q", r.Runner, tc.runner)
			}
			if got := failureIDs(r); !reflect.DeepEqual(got, wantFails) {
				t.Errorf("failures = %q\nwant %q", got, wantFails)
			}
			if got := frameOf(r, "example.com/shop/cart.TestTotal"); !reflect.DeepEqual(got, []Frame{{"cart_test.go", 7}}) {
				t.Errorf("TestTotal frames = %+v", got)
			}
			if got := frameOf(r, "example.com/shop/money.TestTable/half"); !reflect.DeepEqual(got, []Frame{{"money_test.go", 20}}) {
				t.Errorf("TestTable/half frames = %+v", got)
			}
			if !reflect.DeepEqual(r.Packages, wantPkgs) {
				t.Errorf("packages = %+v\nwant %+v", r.Packages, wantPkgs)
			}
			if !reflect.DeepEqual(r.BuildErrors, wantBuild) {
				t.Errorf("build errors = %+v\nwant %+v", r.BuildErrors, wantBuild)
			}
			if !reflect.DeepEqual(r.Panics, wantPanic) {
				t.Errorf("panics = %+v\nwant %+v", r.Panics, wantPanic)
			}
			if !r.Complete {
				t.Error("a whole run whose every package printed its result must read complete")
			}
			if got := r.LeafFailures(); len(got) != 4 {
				t.Errorf("leaf failures = %d, want 4 (TestTable is implied by TestTable/half)", len(got))
			}
		})
	}

	t.Run("verbose output puts the log before the FAIL line", func(t *testing.T) {
		r := ParseTestRun(fixture(t, "go-verbose-fail.txt"))
		if got := failureIDs(r); !reflect.DeepEqual(got, []string{"example.com/shop/cart.TestPanics", "example.com/shop/cart.TestTotal"}) {
			t.Errorf("failures = %q", got)
		}
		if got := frameOf(r, "example.com/shop/cart.TestTotal"); !reflect.DeepEqual(got, []Frame{{"cart_test.go", 7}}) {
			t.Errorf("TestTotal frames = %+v", got)
		}
		if len(r.Panics) != 1 || r.Panics[0].Test != "TestPanics" {
			t.Errorf("panics = %+v", r.Panics)
		}
	})

	for _, name := range []string{"go-plain-pass.txt", "go-json-pass.jsonl"} {
		t.Run(name, func(t *testing.T) {
			r := ParseTestRun(fixture(t, name))
			if len(r.Failures)+len(r.Panics)+len(r.BuildErrors) != 0 {
				t.Errorf("a passing run parsed failures: %+v", r)
			}
			want := []PackageResult{
				{"example.com/shop/cart", StatusOK},
				{"example.com/shop/empty", StatusNoTests},
				{"example.com/shop/money", StatusOK},
			}
			if !reflect.DeepEqual(r.Packages, want) || !r.Complete {
				t.Errorf("packages = %+v complete=%v", r.Packages, r.Complete)
			}
		})
	}

	t.Run("junit", func(t *testing.T) {
		r := ParseTestRun("some runner banner\n" + fixture(t, "junit-fail.xml") + "\ndone\n")
		if r.Runner != RunnerJUnit {
			t.Fatalf("runner = %q", r.Runner)
		}
		want := []string{
			"example.com/shop/broken.TestMain",
			"example.com/shop/money.TestRound",
			"example.com/shop/money.TestTable",
			"example.com/shop/money.TestTable/half",
		}
		if got := failureIDs(r); !reflect.DeepEqual(got, want) {
			t.Errorf("failures = %q\nwant %q", got, want)
		}
		if got := frameOf(r, "example.com/shop/money.TestRound"); !reflect.DeepEqual(got, []Frame{{"money_test.go", 7}}) {
			t.Errorf("TestRound frames = %+v", got)
		}
		if got := frameOf(r, "example.com/shop/broken.TestMain"); !reflect.DeepEqual(got, []Frame{{"broken/broken.go", 3}}) {
			t.Errorf("TestMain frames = %+v", got)
		}
		if r.Summary == nil || *r.Summary != (Summary{Tests: 6, Failures: 3, Errors: 1, Skipped: 1}) {
			t.Errorf("summary = %+v", r.Summary)
		}
		if !r.Complete {
			t.Error("a closed <testsuites> root is a complete summary")
		}
		pass := ParseTestRun(fixture(t, "junit-pass.xml"))
		if len(pass.Failures) != 0 || !pass.Complete || pass.Summary.Tests != 2 {
			t.Errorf("junit pass = %+v", pass)
		}
	})

	t.Run("a cut run is not complete", func(t *testing.T) {
		plain := fixture(t, "go-plain-fail.txt")
		cut := plain[:len(plain)-len("FAIL\texample.com/shop/money\t0.140s\nFAIL\n")]
		if r := ParseTestRun(cut); r.Complete {
			t.Error("output that ends after a failing test but before its package line read complete")
		}
		js := fixture(t, "go-json-pass.jsonl")
		lines := splitLines(js)
		if r := ParseTestRun(joinLines(lines[:len(lines)-3])); r.Complete {
			t.Error("a -json stream missing a started package's final event read complete")
		}
		xml := fixture(t, "junit-pass.xml")
		if r := ParseTestRun(xml[:len(xml)/2]); r.Complete || r.Runner != RunnerJUnit {
			t.Errorf("half a JUnit document: runner %q complete %v", r.Runner, r.Complete)
		}
	})

	t.Run("not a test run", func(t *testing.T) {
		r := ParseTestRun("npm WARN deprecated\n> vitest run\n 3 passed\n")
		if r.Runner != RunnerNone || r.Complete || len(r.Failures) != 0 {
			t.Errorf("unrecognised output = %+v", r)
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		in := fixture(t, "go-json-fail.jsonl")
		a := ParseTestRun(in)
		for i := 0; i < 5; i++ {
			if b := ParseTestRun(in); !reflect.DeepEqual(a, b) {
				t.Fatalf("run %d differs", i)
			}
		}
	})
}

func splitLines(s string) []string {
	var out []string
	for len(s) > 0 {
		i := 0
		for i < len(s) && s[i] != '\n' {
			i++
		}
		out = append(out, s[:i])
		if i < len(s) {
			i++
		}
		s = s[i:]
	}
	return out
}

func joinLines(lines []string) string {
	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	return out
}
