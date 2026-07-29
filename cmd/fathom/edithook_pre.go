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
// literally "Bash") vs command-guard matching (Bash, design §2.5).
func cmdEditHookPre(toolName, filePath, command, cwd, sessionID, url string, stdout io.Writer) int {
	if toolName == "Bash" {
		return cmdEditHookPreCommand(command, cwd, url, stdout)
	}
	return cmdEditHookPreFile(filePath, cwd, sessionID, url, stdout)
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
	hc := &http.Client{Timeout: 2 * time.Second}
	svcName, resolvedRel, ok := resolveServiceAndRel(hc, url, projDir, filePath)
	if !ok {
		emitBeat("edit-hook", "answered", map[string]string{"emitted": "false", "phase": "pre", "target": "file"})
		return 0
	}
	repoRelFile = resolvedRel

	obl, indexStale, ok := fetchObligations(url, svcName, repoRelFile, guard.MaxRules)
	if !ok {
		emitBeat("edit-hook", "error", map[string]string{"phase": "pre", "target": "file"})
		return 0
	}

	// include_modules gates DISPLAY only (design §2.1's per-guard toggle) —
	// the daemon already computed module rules whenever a binding exists;
	// this guard's config decides whether the PRE-edit preview shows them.
	displayed := obl.Rules
	if !guard.IncludeModules {
		displayed = filterOutModuleRules(obl.Rules)
	}

	var sb strings.Builder
	if guard.Note != "" {
		fmt.Fprintf(&sb, "⚖️ fathom guard — %s: %s", repoRelFile, guard.Note)
	}
	for _, r := range displayed {
		if sb.Len() == 0 {
			fmt.Fprintf(&sb, "⚖️ fathom guard — %s:", repoRelFile)
		}
		writeRuleBullet(&sb, r)
	}
	if obl.Displaced > 0 {
		fmt.Fprintf(&sb, "\n⚠️ %d more eligible rule(s) not shown (cap)", obl.Displaced)
	}
	if indexStale {
		sb.WriteString("\n⚠️ index is behind your working tree for this file")
	}
	for _, n := range obl.ModuleNotes {
		fmt.Fprintf(&sb, "\n⚠️ module %s: %s", n.Module, n.Note)
	}

	if sb.Len() == 0 {
		emitBeat("edit-hook", "answered", map[string]string{"emitted": "false", "phase": "pre", "target": "file"})
		return 0 // guard matched but had nothing to say (no note, no rules, not stale)
	}

	// Stamp what was ACTUALLY displayed — a rule hidden by
	// include_modules:false must remain visible to the post-hook, not be
	// silently suppressed forever (design §2.3's dedup is "already shown
	// to the agent," never "already computed by the daemon").
	writeGuardStamp(sessionID, svcName, repoRelFile, ruleKeysOf(displayed), indexStale, time.Now())
	reportModuleBeats(displayed, "pre")

	out, _ := json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "PreToolUse",
			"additionalContext": sb.String(),
		},
	})
	emitBeat("edit-hook", "answered", map[string]string{"emitted": "true", "phase": "pre", "target": "file"})
	fmt.Fprintln(stdout, string(out))
	return 0
}

// ─── command guards (Bash, design §2.5) ──────────────────────────────────

func cmdEditHookPreCommand(command, cwd, url string, stdout io.Writer) int {
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
	if !matched || note == "" {
		// Command guards serve only their note (design decision: there is
		// no per-file obligations lookup for a command — no daemon call at
		// all on this path). "matched but no note" has nothing to inject.
		emitBeat("edit-hook", "answered", map[string]string{"emitted": "false", "phase": "pre", "target": "command"})
		return 0
	}

	sb := fmt.Sprintf("⚖️ fathom guard — command: %s", note)
	out, _ := json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "PreToolUse",
			"additionalContext": sb,
		},
	})
	emitBeat("edit-hook", "answered", map[string]string{"emitted": "true", "phase": "pre", "target": "command"})
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
