package capability

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadBudgetsThreeStates pins the O22 lesson for the budgets block:
// an absent file, a file with no block, and a malformed block are three
// different outcomes — and only the third is an error.
func TestLoadBudgetsThreeStates(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}

	t.Run("absent file is no budgets, not an error", func(t *testing.T) {
		got, err := LoadBudgets(filepath.Join(dir, "nope.cue"), ScopeProject)
		if err != nil || got != nil {
			t.Fatalf("got %v, %v; want nil, nil", got, err)
		}
	})

	t.Run("empty path is no budgets, not an error", func(t *testing.T) {
		got, err := LoadBudgets("", ScopeUser)
		if err != nil || got != nil {
			t.Fatalf("got %v, %v; want nil, nil", got, err)
		}
	})

	t.Run("a file with only capabilities has no budgets", func(t *testing.T) {
		p := write("caps-only.cue", "capabilities: {\n\tnarrate: on: true\n}\n")
		got, err := LoadBudgets(p, ScopeProject)
		if err != nil || got != nil {
			t.Fatalf("got %v, %v; want nil, nil", got, err)
		}
	})

	t.Run("the scaffold init writes has no budgets", func(t *testing.T) {
		p := write("scaffold.cue", string(ExampleTemplate(Default())))
		got, err := LoadBudgets(p, ScopeProject)
		if err != nil || got != nil {
			t.Fatalf("got %v, %v; want nil, nil", got, err)
		}
		if err := ValidateSchema(p); err != nil {
			t.Fatalf("the scaffold must validate against the schema: %v", err)
		}
	})

	t.Run("a concrete block loads, in tokens, keyed by class", func(t *testing.T) {
		p := write("ok.cue", "capabilities: {\n\tnarrate: on: true\n}\n\nbudgets: {\n\tlist:      3000\n\tcomposite: 8000\n\tmap:       1200\n}\n")
		got, err := LoadBudgets(p, ScopeProject)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := Budgets{"list": 3000, "composite": 8000, "map": 1200}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for k, v := range want {
			if got[k] != v {
				t.Errorf("%s = %d, want %d", k, got[k], v)
			}
		}
		if err := ValidateSchema(p); err != nil {
			t.Errorf("a valid block must also pass the schema: %v", err)
		}
	})

	t.Run("a partial block sets only what it names", func(t *testing.T) {
		p := write("partial.cue", "budgets: list: 900\n")
		got, err := LoadBudgets(p, ScopeUser)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got["list"] != 900 {
			t.Errorf("got %v, want {list:900}", got)
		}
	})

	t.Run("a syntax error is a loud LoadError naming the path", func(t *testing.T) {
		p := write("broken.cue", "budgets: { list: \n")
		_, err := LoadBudgets(p, ScopeProject)
		var le *LoadError
		if !errors.As(err, &le) {
			t.Fatalf("want *LoadError, got %v", err)
		}
		if le.Path != p || le.Scope != ScopeProject {
			t.Errorf("LoadError must carry path and scope: %+v", le)
		}
	})
}

// TestLoadBudgetsRefusals pins every way a budgets block can lie: an
// unknown class (with the nearest known one suggested), a non-positive or
// non-integer value, and the phase-O optional-field inversion. Every
// problem is reported, not just the first.
func TestLoadBudgetsRefusals(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}

	t.Run("unknown class is refused and the nearest known one suggested", func(t *testing.T) {
		p := write("typo.cue", "budgets: { lists: 900 }\n")
		_, err := LoadBudgets(p, ScopeProject)
		var ue *UnknownBudgetClassError
		if !errors.As(err, &ue) {
			t.Fatalf("want UnknownBudgetClassError, got %v", err)
		}
		if ue.Class != "lists" || ue.Suggest != "list" {
			t.Errorf("got %+v, want class lists, suggest list", ue)
		}
		if serr := ValidateSchema(p); serr == nil {
			t.Error("the schema is closed over the known classes; an unknown key must fail validation too")
		}
	})

	t.Run("zero and negative are refused", func(t *testing.T) {
		for _, body := range []string{"budgets: list: 0\n", "budgets: map: -5\n"} {
			p := write("nonpos.cue", body)
			if _, err := LoadBudgets(p, ScopeProject); err == nil || !strings.Contains(err.Error(), "> 0") {
				t.Errorf("%q: want a > 0 refusal, got %v", body, err)
			}
			if serr := ValidateSchema(p); serr == nil {
				t.Errorf("%q: the schema must refuse a non-positive budget", body)
			}
		}
	})

	t.Run("a non-integer is refused", func(t *testing.T) {
		for _, body := range []string{"budgets: list: \"900\"\n", "budgets: list: 1.5\n", "budgets: list: true\n"} {
			p := write("notint.cue", body)
			if _, err := LoadBudgets(p, ScopeProject); err == nil || !strings.Contains(err.Error(), "whole number of tokens") {
				t.Errorf("%q: want a whole-number refusal, got %v", body, err)
			}
			if serr := ValidateSchema(p); serr == nil {
				t.Errorf("%q: the schema must refuse a non-integer budget", body)
			}
		}
	})

	t.Run("an optional class declaration is refused with the fix in the message", func(t *testing.T) {
		p := write("optional.cue", "budgets: { list?: 900 }\n")
		_, err := LoadBudgets(p, ScopeProject)
		if err == nil {
			t.Fatal("an optional-field declaration must be refused, not silently ignored")
		}
		if !strings.Contains(err.Error(), "without the question mark") {
			t.Errorf("the message must say how to fix it: %v", err)
		}
	})

	t.Run("an optional block declaration is refused too", func(t *testing.T) {
		p := write("optional-block.cue", "budgets?: { list: 900 }\n")
		_, err := LoadBudgets(p, ScopeProject)
		if err == nil || !strings.Contains(err.Error(), "without the question mark") {
			t.Fatalf("budgets?: sets nothing and must say so, got %v", err)
		}
	})

	t.Run("every problem is reported, sorted", func(t *testing.T) {
		p := write("many.cue", "budgets: { list: 0, foo: 1, map: \"x\" }\n")
		_, err := LoadBudgets(p, ScopeProject)
		var le *LoadError
		if !errors.As(err, &le) {
			t.Fatalf("want *LoadError, got %v", err)
		}
		if len(le.Errs) != 3 {
			t.Fatalf("want 3 problems reported, got %d: %v", len(le.Errs), err)
		}
		for i := 1; i < len(le.Errs); i++ {
			if le.Errs[i-1].Error() > le.Errs[i].Error() {
				t.Errorf("errors must be sorted: %q > %q", le.Errs[i-1], le.Errs[i])
			}
		}
	})
}

// TestLoadBudgetResolutionScopes reads both scopes at once: a broken file
// yields no budgets for THAT scope plus a loud error, and never poisons
// the other scope.
func TestLoadBudgetResolutionScopes(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "home", ".fathom", UserFileName)
	proj := filepath.Join(dir, "repo", ".fathom", ProjectFileName)
	for p, body := range map[string]string{
		user: "budgets: { list: 900, map: 1200 }\n",
		proj: "budgets: { list: 3000 }\n",
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	res, errs := LoadBudgetResolution(user, proj)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if res.User["list"] != 900 || res.User["map"] != 1200 || res.Project["list"] != 3000 {
		t.Errorf("got %+v", res)
	}

	// Both absent: a zero value, no errors.
	res, errs = LoadBudgetResolution(filepath.Join(dir, "no-user.cue"), filepath.Join(dir, "no-proj.cue"))
	if len(errs) != 0 || res.User != nil || res.Project != nil {
		t.Errorf("absent files must yield a zero resolution: %+v, %v", res, errs)
	}

	// A broken project file is reported and leaves the user scope intact.
	if err := os.WriteFile(proj, []byte("budgets: { nope: 1 }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, errs = LoadBudgetResolution(user, proj)
	if len(errs) != 1 {
		t.Fatalf("want exactly one error for the broken project file, got %v", errs)
	}
	if res.Project != nil || res.User["list"] != 900 {
		t.Errorf("a broken scope yields nothing for that scope and does not poison the other: %+v", res)
	}
}

// TestBudgetClassesAreSortedAndClosed: the class list is the closed set
// design §3 sizes, and it is sorted because it is rendered into error
// messages and the scaffold.
func TestBudgetClassesAreSortedAndClosed(t *testing.T) {
	want := []string{"composite", "list", "map"}
	if len(BudgetClasses) != len(want) {
		t.Fatalf("got %v, want %v", BudgetClasses, want)
	}
	for i := range want {
		if BudgetClasses[i] != want[i] {
			t.Errorf("BudgetClasses[%d] = %q, want %q", i, BudgetClasses[i], want[i])
		}
	}
}
