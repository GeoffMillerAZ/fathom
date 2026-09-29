package main

// enforce.go — ADR-038 items 3-4: wiring internal/guardspec's
// deterministic pre-write checks into the PreToolUse guard hook
// (edithook_pre.go). Mirrors guard.go's own local-fast-path discipline: a
// target no enforce:true rule governs costs one local file read plus a
// handful of pure-Go checks, no daemon call — the same "instant silence on
// a miss" contract guarded-edits' advisory half already provides. This is
// the ONLY place a hook verb ever asks for HookOutput.Deny.

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
)

// enforceDecision is what evaluateEnforcedFile/evaluateEnforcedCommand
// found: Denied=false means no enforce:true rule governing this target was
// violated (or none govern it at all, or enforcement is switched off) —
// the caller proceeds to the ordinary advisory path unchanged.
//
// RuleID/CheckID/File name the FIRST violation (the one Reason renders) so
// the deny path can write its `blocked` outcome (recordBlockedOutcome,
// PR-10); they are zero whenever Denied is false. File is the repo-relative
// path the violation fired on, "" for a command guard.
type enforceDecision struct {
	Denied  bool
	Reason  string
	RuleID  string
	CheckID string
	File    string
}

// newEnforceDecision is the one place a violation becomes a denial, so a
// decision can never carry a Reason without the identity behind it.
func newEnforceDecision(v guardspec.EnforceViolation, file string) enforceDecision {
	return enforceDecision{Denied: true, Reason: formatDenyReason(v), RuleID: v.RuleID, CheckID: v.CheckID, File: file}
}

// loadLayerRuleSet reads <root>/.fathom/guard.cue's layer_rules —
// guard.go's loadGuardSet, mirrored for the OTHER half of the same file.
// ok is false on an absent or malformed file (fail-open, same discipline:
// `fathom doctor` is the loud, non-hot-path place a malformed file gets
// surfaced).
func loadLayerRuleSet(root string) (guardspec.LayerRuleSet, bool) {
	path := filepath.Join(root, ".fathom", "guard.cue")
	if _, err := os.Stat(path); err != nil {
		return guardspec.LayerRuleSet{}, false
	}
	lrs, err := guardspec.LoadLayerRules(path)
	if err != nil {
		return guardspec.LayerRuleSet{}, false
	}
	return lrs, true
}

// enforceEnabled is the escape hatch (ADR-038 item 3): FATHOM_ENFORCE=off
// (env, checked first, always wins) or a loaded `[guards] enforce = false`
// (project config layer, then user) turns every enforce:true rule back to
// advisory. Fails OPEN on a config read error — the same direction
// capabilityOffAt takes and for the same reason: this runs inside a hook,
// where silently WITHDRAWING a safety feature because a file failed to
// parse is worse than keeping it; a malformed config is reported loudly by
// `fathom doctor` instead.
func enforceEnabled(root string) bool {
	if os.Getenv("FATHOM_ENFORCE") == "off" {
		return false
	}
	if root == "" {
		return true
	}
	loaded, err := config.Load(config.DefaultUserPath(), filepath.Join(root, ".fathom", "config.toml"))
	if err != nil {
		return true // fail open; see the doc comment
	}
	if loaded.Config.Guards.Enforce != nil && !*loaded.Config.Guards.Enforce {
		return false
	}
	return true
}

// evaluateEnforcedFile runs every enforce:true guard and layer rule
// governing repoRelPath against the proposed edit and returns the FIRST
// violation as a single-line deny reason (design decision, ADR-038 item
// 3: never a wall of text) — nil-equivalent (Denied:false) when nothing
// governs the target, nothing is violated, or enforcement is off.
func evaluateEnforcedFile(root, repoRelPath, newContent, oldContent string) enforceDecision {
	if !enforceEnabled(root) {
		return enforceDecision{}
	}
	var violations []guardspec.EnforceViolation
	if gs, ok := loadGuardSet(root); ok {
		for _, g := range gs.Guards {
			violations = append(violations, guardspec.EvaluateGuardEnforcement(g, repoRelPath, newContent, oldContent)...)
		}
	}
	if lrs, ok := loadLayerRuleSet(root); ok {
		modPrefix, _ := guardspec.ModulePrefix(root)
		for _, r := range lrs.Rules {
			violations = append(violations, guardspec.EvaluateLayerRuleEnforcement(r, modPrefix, repoRelPath, newContent)...)
		}
	}
	if len(violations) == 0 {
		return enforceDecision{}
	}
	return newEnforceDecision(violations[0], repoRelPath)
}

// evaluateEnforcedCommand is evaluateEnforcedFile's command-guard sibling:
// only a PROJECT guard.cue is consulted (never the user-scope file — law
// 13, guardspec.LoadUserGuards' own forbidden-fields list already refuses
// enforce/forbidden_paths/checks there, so a user-scope guard can never
// reach this path at all).
func evaluateEnforcedCommand(root, command string) enforceDecision {
	if !enforceEnabled(root) {
		return enforceDecision{}
	}
	gs, ok := loadGuardSet(root)
	if !ok {
		return enforceDecision{}
	}
	g, matched := matchingCommandGuard(gs, command)
	if !matched {
		return enforceDecision{}
	}
	if v := guardspec.EvaluateCommandEnforcement(g); v != nil {
		return newEnforceDecision(*v, "")
	}
	return enforceDecision{}
}

// formatDenyReason renders one EnforceViolation as the single line shown
// in place of the tool call: the rule, its citation, and the remedy.
func formatDenyReason(v guardspec.EnforceViolation) string {
	return fmt.Sprintf("fathom guard (%s, %s): %s — remedy: %s", v.RuleID, v.Cite, v.Message, v.Remedy)
}

// evaluateEnforcedFileAtProjDir resolves the FIRST serviceCandidates root
// filePath relativizes under (guard.go's own matching loops use the same
// order) and runs evaluateEnforcedFile against it — the local, no-daemon
// identity guess is enough here: enforcement never needs the daemon-
// resolved authoritative identity renderPreEdit's advisory path fetches on
// a guard hit, because every check this file runs is pure local text.
func evaluateEnforcedFileAtProjDir(projDir, filePath, newContent, oldContent string) enforceDecision {
	for _, cand := range serviceCandidates(projDir) {
		rel, ok := relativeToRoot(cand.root, filePath)
		if !ok {
			continue
		}
		return evaluateEnforcedFile(cand.root, rel, newContent, oldContent)
	}
	return enforceDecision{}
}

// evaluateEnforcedCommandAtProjDir mirrors cmdEditHookPreCommand's own
// candidate walk: every candidate root is tried in order, the first
// enforced match wins.
func evaluateEnforcedCommandAtProjDir(projDir, command string) enforceDecision {
	for _, cand := range serviceCandidates(projDir) {
		if d := evaluateEnforcedCommand(cand.root, command); d.Denied {
			return d
		}
	}
	return enforceDecision{}
}
