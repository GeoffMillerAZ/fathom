package capability

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// Budgets is one scope's `budgets` block: tool-class id → default answer
// budget in TOKENS (docs/design/budgeted-answers.md §3, BA-9). A key absent
// from the map means that scope said nothing about the class, which is
// different from setting it. The block is CLOSED over BudgetClasses:
//
//	budgets: {
//		list:      1500 // query_symbols, references
//		composite: 4000 // pack_context
//		map:       2500 // repo_map
//	}
//
// Each class sizes exactly the tools internal/mcp/budget_defaults.go
// (toolBudgetClasses) maps to it, named beside it above — and nothing
// else. find_symbol, find_callers, symbol_context and outline are NOT
// budgeted by class: no value here changes them.
//
// It shares the file with the `capabilities` block but not the loader:
// [LoadBudgets] reads only this block, [Registry.LoadSettings] only the
// other, so each keeps its own refusals and neither can drop the other's
// lines quietly.
type Budgets map[string]int

// BudgetClasses are the tool classes design §3 sizes, sorted. A key outside
// this set is refused at load (and by the embedded schema) with the nearest
// known class suggested — a typo that silently leaves a budget at its
// built-in is the same defect class as a capability-id typo.
var BudgetClasses = []string{"composite", "list", "map"}

// BudgetResolution holds both scopes' budgets with their provenance intact.
// Precedence belongs to the consumer (internal/mcp.budgetDefaults resolves
// project → user → built-in, law 13: a user scope may default a class, only
// the project governs it).
type BudgetResolution struct {
	User    Budgets
	Project Budgets
}

// UnknownBudgetClassError names a budgets key that is not a tool class,
// suggesting the nearest one when a typo is plausible.
type UnknownBudgetClassError struct {
	Class   string
	Suggest string
}

func (e *UnknownBudgetClassError) Error() string {
	if e.Suggest != "" {
		return fmt.Sprintf("budgets: unknown tool class %q — did you mean %q?", e.Class, e.Suggest)
	}
	return fmt.Sprintf("budgets: unknown tool class %q (known: %s)", e.Class, strings.Join(BudgetClasses, ", "))
}

// OptionalBudgetError is the phase-O trust-boundary inversion for the
// budgets block: `list?: 900` — or `budgets?: {...}` — declares that the
// field may exist and sets nothing, while reading as a setting.
type OptionalBudgetError struct {
	Field string
}

func (e *OptionalBudgetError) Error() string {
	return fmt.Sprintf("budgets: %q is declared OPTIONAL (%s?:) — in a configuration file that declares the field "+
		"may exist rather than setting it, so this line has no effect; write %q: without the question mark",
		e.Field, e.Field, e.Field)
}

// LoadBudgets reads one scope's `budgets` block from a capabilities.cue.
//
// The same three states as [Registry.LoadSettings] — the O22 lesson:
//
//   - the path is empty or the file ABSENT: no budgets, no error.
//   - the file has no `budgets` block: no budgets, no error.
//   - the block is PRESENT and wrong: a loud *LoadError naming every
//     problem — an unknown class, a non-positive or non-integer value, or
//     a field declared optional. Nothing is dropped quietly.
func LoadBudgets(path string, scope Scope) (Budgets, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("capabilities: read %s: %w", path, err)
	}

	cctx := cuecontext.New()
	v := cctx.CompileBytes(data, cue.Filename(path))
	if v.Err() != nil {
		return nil, &LoadError{Path: path, Scope: scope, Errs: []error{v.Err()}}
	}

	// Find the block by iterating WITH optionals, so `budgets?:` is refused
	// rather than read past as absent.
	top, terr := v.Fields(cue.Optional(true))
	if terr != nil {
		return nil, &LoadError{Path: path, Scope: scope, Errs: []error{fmt.Errorf("budgets: %w", terr)}}
	}
	var root cue.Value
	found := false
	for top.Next() {
		if top.Selector().Unquoted() != "budgets" {
			continue
		}
		if top.IsOptional() {
			return nil, &LoadError{Path: path, Scope: scope, Errs: []error{&OptionalBudgetError{Field: "budgets"}}}
		}
		root, found = top.Value(), true
		break
	}
	if !found {
		return nil, nil
	}

	fields, ferr := root.Fields(cue.Optional(true))
	if ferr != nil {
		return nil, &LoadError{
			Path: path, Scope: scope,
			Errs: []error{fmt.Errorf("budgets: must be a struct of tool class → tokens: %w", ferr)},
		}
	}
	out := Budgets{}
	var errs []error
	for fields.Next() {
		name := fields.Selector().Unquoted()
		if fields.IsOptional() {
			errs = append(errs, &OptionalBudgetError{Field: name})
			continue
		}
		if !isBudgetClass(name) {
			errs = append(errs, &UnknownBudgetClassError{Class: name, Suggest: nearest(name, BudgetClasses)})
			continue
		}
		n, nerr := fields.Value().Int64()
		if nerr != nil {
			errs = append(errs, fmt.Errorf("budgets: %s must be a whole number of tokens: %w", name, nerr))
			continue
		}
		if n <= 0 {
			errs = append(errs, fmt.Errorf("budgets: %s must be > 0 tokens, got %d", name, n))
			continue
		}
		out[name] = int(n)
	}
	if len(errs) > 0 {
		sort.SliceStable(errs, func(i, j int) bool { return errs[i].Error() < errs[j].Error() })
		return nil, &LoadError{Path: path, Scope: scope, Errs: errs}
	}
	return out, nil
}

func isBudgetClass(name string) bool {
	i := sort.SearchStrings(BudgetClasses, name)
	return i < len(BudgetClasses) && BudgetClasses[i] == name
}

// LoadBudgetResolution reads both scopes' budgets blocks. A broken file
// yields no budgets for THAT scope plus a loud error, and never poisons the
// other scope — a partial read that looks complete is the defect class
// this package exists to remove.
func LoadBudgetResolution(userPath, projectPath string) (BudgetResolution, []error) {
	var errs []error
	res := BudgetResolution{}
	if b, err := LoadBudgets(userPath, ScopeUser); err != nil {
		errs = append(errs, err)
	} else {
		res.User = b
	}
	if b, err := LoadBudgets(projectPath, ScopeProject); err != nil {
		errs = append(errs, err)
	} else {
		res.Project = b
	}
	return res, errs
}
