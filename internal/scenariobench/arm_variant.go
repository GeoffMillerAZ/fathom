// arm_variant.go — DP-14: build a corpus VARIANT from the command line
// (scenariobench run's -arm-clone / -arm-setup / -arm / -arms) instead of
// hand-maintaining a JSON fork for every competitor arm. The decision-port
// A/B (docs/bench/scenarios.md "The decision-port arm") needs the Jev arm
// layered onto an arm the generated LCA corpus already defines in code
// (lca.go's LCAArmFathom) — a corpus JSON fork cannot reach that arm at
// all, since it never exists as a file.
//
// CloneArm and SetupArm mutate an already-loaded Corpus in place; the
// caller (cmd/scenariobench's cmdRun) applies them AFTER the corpus is
// loaded or generated and BEFORE the corpus hash/cell keys are computed
// (CanonicalCorpusSHA256 of the mutated Corpus, not the original file's own
// bytes), so a variant is a genuinely different corpus SHA — cells never
// collide with, and are never borrowed from, the un-cloned base corpus's
// own stored cells. ArmSelection resolves which of a corpus's own arms a
// run should execute. All three are pure functions over Corpus/Arm — no
// I/O, no session-running — so they are testable in isolation from
// cmd/scenariobench's own driver plumbing.
package scenariobench

import (
	"fmt"
	"strings"
)

// CloneArm copies corpus.Arms[base] under a new name newName — every field
// of the base arm, unchanged (a struct-value copy). The base arm's own
// slice/map fields (Setup, AllowedTools, MCP, Env, ...) are never mutated
// in place afterward: SetupArm always allocates a fresh Setup slice for
// whichever arm it targets, so a later -arm-setup on the clone can never
// corrupt the base arm's own Setup by growing a shared backing array.
//
// Every task's own Predictions[base] entry (when present) is copied to
// Predictions[newName] too, so a cloned arm's cells are never silently
// unregistered (SB-10's own registered-prediction rule) or blank in a
// later ledger listing.
//
// Errors when base does not name an existing arm, or newName already
// does — a variant must never silently clobber an existing arm.
func CloneArm(c *Corpus, newName, base string) error {
	if strings.TrimSpace(newName) == "" || strings.TrimSpace(base) == "" {
		return fmt.Errorf("arm-clone: both a new arm name and a base arm name are required (got %q=%q)", newName, base)
	}
	baseArm, ok := c.Arms[base]
	if !ok {
		return fmt.Errorf("arm-clone %s=%s: no arm %q in this corpus", newName, base, base)
	}
	if _, exists := c.Arms[newName]; exists {
		return fmt.Errorf("arm-clone %s=%s: arm %q already exists in this corpus", newName, base, newName)
	}
	if c.Arms == nil {
		c.Arms = map[string]Arm{}
	}
	c.Arms[newName] = baseArm

	for i, task := range c.Tasks {
		pred, ok := task.Predictions[base]
		if !ok {
			continue
		}
		if _, exists := task.Predictions[newName]; exists {
			continue
		}
		np := make(map[string]string, len(task.Predictions)+1)
		for k, v := range task.Predictions {
			np[k] = v
		}
		np[newName] = pred
		task.Predictions = np
		c.Tasks[i] = task
	}
	return nil
}

// SetupArm appends line to arm armName's own Setup list — a shell line
// that runs in the worktree before session 1 (Arm.Setup's own doc
// comment in types.go), e.g. the decision-port A/B's Jev [decide] block
// (DP-14). A fresh slice is always allocated (never an in-place append to
// arm.Setup's existing backing array) so a clone sharing the base arm's
// own Setup slice (CloneArm's shallow struct copy) is never mutated
// through append's occasional in-place growth — setting up a clone must
// never risk corrupting the base arm it was cloned from.
//
// Errors when armName does not name an existing arm.
func SetupArm(c *Corpus, armName, line string) error {
	arm, ok := c.Arms[armName]
	if !ok {
		return fmt.Errorf("arm-setup %s: no arm %q in this corpus", armName, armName)
	}
	setup := make([]string, len(arm.Setup), len(arm.Setup)+1)
	copy(setup, arm.Setup)
	arm.Setup = append(setup, line)
	c.Arms[armName] = arm
	return nil
}

// ArmSelection merges a legacy single -arm value and a comma-separated
// -arms value into one requested set (DP-14): either or both may be
// empty, meaning "every arm" (nil returned); when either names arms, the
// result names EXACTLY those (deduped) — never a superset, never a
// partial match — so a decision-port A/B asking for two named arms runs
// exactly those two, never every arm the corpus happens to define.
func ArmSelection(single, commaList string) map[string]bool {
	out := map[string]bool{}
	if s := strings.TrimSpace(single); s != "" {
		out[s] = true
	}
	for _, a := range strings.Split(commaList, ",") {
		if a = strings.TrimSpace(a); a != "" {
			out[a] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
