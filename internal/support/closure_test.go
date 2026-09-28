package support

import "testing"

// TestSupportRegistriesMatchTargets proves SM-7's three closure rules
// (design §3.1) independently, then a clean-closure case that must report
// no violations at all.
func TestSupportRegistriesMatchTargets(t *testing.T) {
	t.Run("registry entry with no target row fails", func(t *testing.T) {
		registry := []RegistryEntry{{Kind: KindClient, ID: "orphan"}}
		violations := CheckClosure(registry, nil, nil, nil)
		if len(violations) != 1 || violations[0].ID != "orphan" {
			t.Fatalf("violations = %v, want exactly one for the orphaned registry entry", violations)
		}
	})

	t.Run("target row with no registry entry and a CLAIMED plan row fails", func(t *testing.T) {
		targets := []ClosureRef{{Kind: KindEcosystem, ID: "cargo", PlanRow: "DP-9"}}
		claimed := map[string]bool{"DP-9": true}
		violations := CheckClosure(nil, targets, nil, claimed)
		if len(violations) != 1 || violations[0].ID != "cargo" {
			t.Fatalf("violations = %v, want exactly one for the claimed-but-unregistered target", violations)
		}
	})

	t.Run("target row with no registry entry and an OPEN plan row is declared, not a failure", func(t *testing.T) {
		targets := []ClosureRef{{Kind: KindEcosystem, ID: "cargo", PlanRow: "DP-9"}}
		violations := CheckClosure(nil, targets, nil, map[string]bool{"DP-9": false})
		if len(violations) != 0 {
			t.Fatalf("violations = %v, want none (an open owning row is declared, not a failure)", violations)
		}
		// Also true when the row is simply absent from claimedRows.
		if v := CheckClosure(nil, targets, nil, nil); len(v) != 0 {
			t.Fatalf("violations = %v, want none when claimedRows has no entry at all", v)
		}
	})

	t.Run("reading names an entry in neither registry nor targets fails", func(t *testing.T) {
		readings := []ClosureRef{{Kind: KindRules, ID: "ghost"}}
		violations := CheckClosure(nil, nil, readings, nil)
		if len(violations) != 1 || violations[0].ID != "ghost" {
			t.Fatalf("violations = %v, want exactly one for the unknown reading", violations)
		}
	})

	t.Run("a reading matching a target-only (declared) entry is not a violation", func(t *testing.T) {
		targets := []ClosureRef{{Kind: KindRules, ID: "cursor-rules", PlanRow: "GOV-1"}}
		readings := []ClosureRef{{Kind: KindRules, ID: "cursor-rules"}}
		if v := CheckClosure(nil, targets, readings, nil); len(v) != 0 {
			t.Fatalf("violations = %v, want none", v)
		}
	})

	t.Run("clean closure via the real Target shape: registry == targets, nothing claimed-orphaned", func(t *testing.T) {
		targets := []Target{
			{Kind: KindClient, ID: "cursor", Name: "Cursor", PlanRow: "SM-13", Wave: 1},
			{Kind: KindClient, ID: "claude-code", Name: "Claude Code", PlanRow: "SM-12", Wave: 1},
		}
		registry := []RegistryEntry{
			{Kind: KindClient, ID: "cursor"},
			{Kind: KindClient, ID: "claude-code"},
		}
		claimed := map[string]bool{"SM-12": true, "SM-13": false}
		if v := CheckClosure(registry, targetRefs(targets), nil, claimed); len(v) != 0 {
			t.Fatalf("violations = %v, want none (both registered)", v)
		}
	})
}
