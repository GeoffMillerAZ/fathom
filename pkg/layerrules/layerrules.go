// Package layerrules is the PUBLIC re-export of internal/guardspec's
// layer-rule types and internal/guardspec/layerrules' go/packages-backed
// evaluator (VB-13/VB-14, docs/vision/29-verbs-not-lookups.md §3 wave C;
// ADR-035 decision 3).
//
// It exists for exactly one reason: `fathom rules gen-test` emits a Go
// test file into the ADOPTING repo — which, for any repo other than
// fathom itself, is a DIFFERENT module and can never import an
// `internal/...` package path (the Go compiler enforces this; a generated
// test importing internal/guardspec directly builds only inside fathom's
// own tree, breaking every external adopter — VB-15's whole "stranger"
// premise). This package is the same shape as pkg/client's re-export of
// internal/core/internal/semantic for external consumers (AGENTS.md "The
// domain model is the keystone ... pkg/client re-exports for external
// consumers"), scoped narrowly to what a generated layer_rules_test.go
// needs.
package layerrules

import (
	"github.com/geoffmilleraz/fathom/internal/guardspec"
	intlayerrules "github.com/geoffmilleraz/fathom/internal/guardspec/layerrules"
)

// LayerRuleKind selects which structural check a LayerRule performs.
type LayerRuleKind = guardspec.LayerRuleKind

// Severity controls whether a violated rule fails the generated test.
type Severity = guardspec.Severity

const (
	ImportDirection       = guardspec.ImportDirection
	NoCycles              = guardspec.NoCycles
	Boundary              = guardspec.Boundary
	CrossServiceDirection = guardspec.CrossServiceDirection

	SeverityError = guardspec.SeverityError
	SeverityWarn  = guardspec.SeverityWarn
)

// LayerRule, LayerRuleSet, Violation and LayerRuleIndex mirror
// internal/guardspec's decoded/evaluation types exactly (type aliases, not
// copies — a guardspec.LayerRule and a layerrules.LayerRule are the same
// type).
type (
	LayerRule    = guardspec.LayerRule
	LayerRuleSet = guardspec.LayerRuleSet
	Violation    = guardspec.Violation
)

// LoadLayerRules reads and decodes a `.fathom/guard.cue` file's
// layer_rules — see guardspec.LoadLayerRules.
func LoadLayerRules(path string) (LayerRuleSet, error) {
	return guardspec.LoadLayerRules(path)
}

// Check re-reads ruleFile's layer rules and evaluates every one against
// root's TRUE go/packages import graph — see
// internal/guardspec/layerrules.Check for the full contract (which
// severities are reported, and why cross_service_direction is always
// skipped from a single checkout).
func Check(root, ruleFile string) (violations []Violation, ruleIDsSkipped []string, err error) {
	return intlayerrules.Check(root, ruleFile)
}
