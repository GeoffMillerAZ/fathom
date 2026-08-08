package main

// edithook_pre.go — `fathom edit-hook --pre`: the PreToolUse guard hook
// (docs/design/guarded-edits.md). Same binary, same service-identity
// resolution, same daemon lookup as the PostToolUse path (edithook.go) —
// the difference is WHEN it fires and that it is silent by construction
// for every file/command that no guard names.
//
// The "instant silence" contract (design §2.2) is structural, not just
// documented: the local guard.cue glob/pattern check (guard.go) runs
// BEFORE any HTTP call is made — candidatesWithDaemon (the one call that
// hits the daemon) is only reached once a guard has already matched.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// cmdEditHookPre implements the --pre branch of cmdEditHook. toolName
// selects file-guard matching (Edit/Write/MultiEdit — anything that isn't
// literally "Bash", "Grep", "Glob", or "Read") vs command-guard matching
// (Bash, design §2.5) vs the counting-only discovery observer (Grep/Glob/
// Read, and Bash argv starting with a search binary —
// docs/design/production-tokenomics.md §4, instrument B).
func cmdEditHookPre(toolName, filePath, command, cwd, sessionID, url string, stdout io.Writer) int {
	switch toolName {
	case "Bash":
		emitDiscoveryBeatIfShellSearch(command, sessionID)
		return cmdEditHookPreCommand(command, cwd, sessionID, url, stdout)
	case "Grep", "Glob", "Read":
		return cmdEditHookPreDiscovery(toolName, filePath, cwd, sessionID)
	default:
		return cmdEditHookPreFile(filePath, cwd, sessionID, url, stdout)
	}
}

// ─── file guards (Edit/Write/MultiEdit) ──────────────────────────────────

func cmdEditHookPreFile(filePath, cwd, sessionID, url string, stdout io.Writer) int {
	if filePath == "" {
		return 0
	}
	if !filepath.IsAbs(filePath) {
		a, err := filepath.Abs(filePath)
		if err != nil {
			return 0
		}
		filePath = a
	}
	projDir := resolveProjDir(cwd)
	emitBeat("edit-hook", "seen", map[string]string{"phase": "pre", "target": "file"})

	// LOCAL fast path — no daemon call yet. serviceCandidates is pure
	// path math; loadGuardSet reads one local file per candidate.
	var (
		guard       guardspec.Guard
		matched     bool
		repoRelFile string
	)
	for _, cand := range serviceCandidates(projDir) {
		rel, ok := relativeToRoot(cand.root, filePath)
		if !ok {
			continue
		}
		gs, gok := loadGuardSet(cand.root)
		if !gok {
			continue
		}
		if g, gmatched := matchingFileGuard(gs, rel); gmatched {
			guard, matched, repoRelFile = g, true, rel
			break
		}
	}
	if !matched {
		emitBeat("edit-hook", "answered", map[string]string{"emitted": "false", "phase": "pre", "target": "file"})
		return 0 // no guard names this file (or it's out-of-repo entirely) — zero daemon calls
	}

	// A guard matched: NOW resolve authoritative service identity (the
	// one HTTP call this path ever makes on a hit) and fetch obligations,
	// capped at the guard's own budget.
	//
	// Critic finding 5 (phase-O merge, cheap): resolveServiceAndRel and
	// fetchObligations are two SEQUENTIAL HTTP calls; each previously
	// carried its own independent 2s client timeout, so a slow daemon
	// could push this path's worst case to ~4s against the design's
	// stated <=2s pre-edit budget (docs/design/guarded-edits.md §2.2:
	// "Same <=2 s hook budget"). preHookBudget is split across both calls
	// instead: the second call gets whatever remains of the shared
	// budget (floored so a fast first call still leaves the second a
	// meaningful attempt, rather than a guaranteed-to-fail sliver).
	const preHookBudget = 2 * time.Second
	const minSecondCallBudget = 200 * time.Millisecond
	callStart := time.Now()
	hc := &http.Client{Timeout: preHookBudget}
	svcName, resolvedRel, ok := resolveServiceAndRel(hc, url, projDir, filePath)
	if !ok {
		emitBeat("edit-hook", "answered", map[string]string{"emitted": "false", "phase": "pre", "target": "file"})
		return 0
	}
	// Critic finding 8 (phase-O merge, note-only unless trivial): the
	// LOCAL fast-path loop above already matched a guard against
	// serviceCandidates' (daemon-unaware) root guess; this daemon-aware
	// pass is the AUTHORITATIVE identity resolution and can in principle
	// disagree with it (e.g. a dogfood-O15-style rename where the
	// indexed service's recorded path differs from every basename
	// candidate). When they agree (the overwhelmingly common case),
	// repoRelFile is unchanged; when they disagree, the daemon-resolved
	// value below is used for the actual obligations lookup regardless —
	// the guard already matched against a real root under the SAME
	// project directory, so serving against the authoritative root is
	// correct, just worth noting as a real, if rare, edge case rather
	// than silently assuming the two resolutions always name the same
	// root.
	repoRelFile = resolvedRel

	remaining := preHookBudget - time.Since(callStart)
	if remaining < minSecondCallBudget {
		remaining = minSecondCallBudget
	}
	obl, indexStale, ok := fetchObligations(url, svcName, repoRelFile, guard.MaxRules, remaining)
	if !ok {
		emitBeat("edit-hook", "error", map[string]string{"phase": "pre", "target": "file"})
		return 0
	}

	// include_modules gates DISPLAY only (design §2.1's per-guard toggle) —
	// the daemon already computed module rules whenever a binding exists;
	// this guard's config decides whether the PRE-edit preview shows them.
	displayed := obl.Rules
	hiddenModuleRules := 0
	if !guard.IncludeModules {
		displayed = filterOutModuleRules(obl.Rules)
		hiddenModuleRules = len(obl.Rules) - len(displayed)
	}

	var sb strings.Builder
	writeHeaderIfNeeded := func() {
		if sb.Len() == 0 {
			fmt.Fprintf(&sb, "⚖️ fathom guard — %s:", repoRelFile)
		}
	}
	if guard.Note != "" {
		fmt.Fprintf(&sb, "⚖️ fathom guard — %s: %s", repoRelFile, guard.Note)
	}
	for _, r := range displayed {
		writeHeaderIfNeeded()
		writeRuleBullet(&sb, r)
	}
	// Critic finding 7 (phase-O merge): include_modules:false was the
	// design's one SILENT suppression — module rules are fetched
	// (consuming the shared server-side budget) then dropped with no
	// line at all. Never silent, per the program's own doctrine: name
	// the count.
	if hiddenModuleRules > 0 {
		writeHeaderIfNeeded()
		fmt.Fprintf(&sb, "\n⚠️ %d module rule(s) hidden (include_modules:false)", hiddenModuleRules)
	}
	if obl.Displaced > 0 {
		writeHeaderIfNeeded()
		fmt.Fprintf(&sb, "\n⚠️ %d more eligible rule(s) not shown (cap)", obl.Displaced)
	}
	if indexStale {
		writeHeaderIfNeeded()
		sb.WriteString("\n⚠️ index is behind your working tree for this file")
	}
	for _, n := range obl.ModuleNotes {
		writeHeaderIfNeeded()
		fmt.Fprintf(&sb, "\n⚠️ module %s: %s", n.Module, n.Note)
	}

	if sb.Len() == 0 {
		emitBeat("edit-hook", "answered", map[string]string{"emitted": "false", "phase": "pre", "target": "file"})
		return 0 // guard matched but had nothing to say (no note, no rules, not stale)
	}
	emitPayloadCost("pre-edit-guard", sb.String(), len(displayed), "pre", sessionID)

	// Stamp what was ACTUALLY displayed — a rule hidden by
	// include_modules:false must remain visible to the post-hook, not be
	// silently suppressed forever (design §2.3's dedup is "already shown
	// to the agent," never "already computed by the daemon").
	writeGuardStamp(sessionID, svcName, repoRelFile, ruleKeysOf(displayed), indexStale, time.Now())
	reportModuleBeats(displayed, "pre", sessionID)

	out, _ := json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "PreToolUse",
			"additionalContext": sb.String(),
		},
	})
	emitBeat("edit-hook", "answered", map[string]string{"emitted": "true", "phase": "pre", "target": "file"})
	recordStoryRow(projDir, "guard-served-file", nil)
	fmt.Fprintln(stdout, string(out))
	return 0
}

// ─── command guards (Bash, design §2.5) ──────────────────────────────────

func cmdEditHookPreCommand(command, cwd, sessionID, url string, stdout io.Writer) int {
	if command == "" {
		return 0
	}
	projDir := resolveProjDir(cwd)
	emitBeat("edit-hook", "seen", map[string]string{"phase": "pre", "target": "command"})

	var note string
	matched := false
	for _, cand := range serviceCandidates(projDir) {
		gs, gok := loadGuardSet(cand.root)
		if !gok {
			continue
		}
		if g, gmatched := matchingCommandGuard(gs, command); gmatched {
			note, matched = g.Note, true
			break
		}
	}
	// Scoped config (docs/design/scoped-config.md deliverable 2): a
	// user-scope guard (~/.fathom/guard.cue, note+command_scopes only) is
	// consulted when no PROJECT guard matched — project scope always wins
	// on conflict (law 13), so the user-scope operator's own danger list
	// only ever ADDS coverage, never overrides a project's own command
	// guard for the same pattern. Its Note already carries the "[user] "
	// tag (guardspec.LoadUserGuards) — no separate rendering branch needed.
	if !matched {
		if ugs, uok := loadUserGuardSet(); uok {
			if g, gmatched := matchingCommandGuard(ugs, command); gmatched {
				note, matched = g.Note, true
			}
		}
	}
	if !matched || note == "" {
		// Command guards serve only their note (design decision: there is
		// no per-file obligations lookup for a command — no daemon call at
		// all on this path). "matched but no note" has nothing to inject.
		emitBeat("edit-hook", "answered", map[string]string{"emitted": "false", "phase": "pre", "target": "command"})
		return 0
	}

	sb := fmt.Sprintf("⚖️ fathom guard — command: %s", note)
	emitPayloadCost("pre-edit-guard", sb, 0, "pre", sessionID)
	out, _ := json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "PreToolUse",
			"additionalContext": sb,
		},
	})
	emitBeat("edit-hook", "answered", map[string]string{"emitted": "true", "phase": "pre", "target": "command"})
	recordStoryRow(projDir, "guard-served-command", nil)
	fmt.Fprintln(stdout, string(out))
	return 0
}

// ─── shared helpers ───────────────────────────────────────────────────────

func resolveProjDir(cwd string) string {
	projDir := os.Getenv("CLAUDE_PROJECT_DIR")
	if projDir == "" {
		projDir = cwd
	}
	if projDir == "" {
		projDir, _ = os.Getwd()
	}
	return projDir
}

// resolveServiceAndRel mirrors the post-hook's own candidate resolution
// (edithook.go): daemon-resolved identity first, first candidate that
// relativizes wins. ok=false only when filePath relativizes under NO
// candidate root at all (should not happen here since the local pass
// already found one, but this is the authoritative daemon-aware pass,
// which can in principle disagree e.g. on a dogfood O15 rename).
func resolveServiceAndRel(hc *http.Client, url, projDir, filePath string) (svcName, rel string, ok bool) {
	for _, cand := range candidatesWithDaemon(hc, url, projDir) {
		if r, rok := relativeToRoot(cand.root, filePath); rok {
			return strings.ToLower(cand.name), r, true
		}
	}
	return "", "", false
}

// filterOutModuleRules returns rules with every module-tagged entry
// (r.Module != "") removed — include_modules:false's display filter.
func filterOutModuleRules(rules []semantic.Obligation) []semantic.Obligation {
	out := make([]semantic.Obligation, 0, len(rules))
	for _, r := range rules {
		if r.Module == "" {
			out = append(out, r)
		}
	}
	return out
}

// ruleKeysOf computes guardstamp.go's ruleKey for every rule in rules.
func ruleKeysOf(rules []semantic.Obligation) []string {
	out := make([]string, len(rules))
	for i, r := range rules {
		out[i] = ruleKey(r.Rule, r.Src, r.Module)
	}
	return out
}
