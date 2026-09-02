package main

// init_harnesshooks.go — `fathom init --harness=<id>` hook-file wiring for
// the non-Claude descriptors whose codec has landed (HP-18): the
// descriptor names the repo-scoped hook file it documents (Codex
// `.codex/hooks.json`, Gemini CLI `.gemini/settings.json`), both are the
// Claude-settings nesting the existing json-hook editor addresses
// (hooks.<Event>[{matcher, hooks:[{type,command,timeout}]}]), so every
// entry is a wiring.Key with a receipt `fleet undo` reverses (I13, law
// 15). User-global files are never written (autonomy: ask-first).
//
// The stanza for such a harness says what is true on the day it is
// written: the hook is wired but unverified and runs only after the
// harness's own trust step, so the pull tier stays named as the fallback.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// codexHookKey is the wiring key for one Codex hook spec in
// `<repo>/.codex/hooks.json`. The verified shape (hooks.md, 2026-09-02)
// is `{"hooks": {<Event>: [{matcher, hooks: [{type, command, timeout}]}]}}`
// — the same three-level nesting as Claude Code's settings — so
// KindJSONHook is reused unchanged and no new editor kind is needed: the
// identity is (event, matcher, marker), exactly as hookKeyFor's.
func codexHookKey(spec hookSpec, command string) wiring.Key {
	d, _ := harness.Lookup(harness.Codex)
	f, _ := d.InitHookFile()
	return harnessHookKeyFor(f, spec, command)
}

// harnessHookKeyFor builds the json-hook key for one spec in one
// descriptor hook file, converting the binding's seconds into the file's
// documented timeout unit (Gemini CLI's settings.json takes milliseconds).
func harnessHookKeyFor(f harness.HookFile, spec hookSpec, command string) wiring.Key {
	timeout := spec.timeout
	if f.TimeoutUnit == "ms" {
		timeout = spec.timeout * 1000
	}
	return wiring.Key{
		File:    f.Path,
		Kind:    wiring.KindJSONHook,
		Path:    []string{spec.event, spec.matcher},
		Marker:  spec.marker,
		Body:    command,
		Timeout: timeout,
	}
}

// harnessHookKeys plans the hook entries `init --harness=<id>` adds for a
// non-Claude descriptor whose hooks are wired: one key per binding, into
// the descriptor's project hook file, add-only (an entry already present
// in its matcher group — by marker — is left alone, exactly as the Claude
// Code path does). A descriptor that names no writable project hook file
// plans nothing and says so in the notes. Never a user-global file.
func harnessHookKeys(root, id, binPath string) ([]wiring.Key, []string) {
	d, ok := harness.Lookup(id)
	if !ok || !d.HooksWired() {
		return nil, nil
	}
	f, ok := d.InitHookFile()
	if !ok {
		return nil, []string{fmt.Sprintf("hooks: %s documents no repo-scoped hook file this build can write — nothing written; wire its user-scope file by hand if you want them", d.Display)}
	}
	present := harnessHookEntriesPresent(root, f, hookSpecsFor(id))
	var keys []wiring.Key
	for _, hs := range hookSpecsFor(id) {
		if present[hookSpecKeyOf(hs)] {
			continue // add-only
		}
		keys = append(keys, harnessHookKeyFor(f, hs, binPath+" "+hs.sub))
	}
	var notes []string
	if len(keys) == 0 {
		notes = append(notes, fmt.Sprintf("ok %s (every fathom hook already present)", f.Path))
	}
	return keys, notes
}

// harnessHookEntriesPresent reads the descriptor hook file under root and
// reports, per spec, whether an entry carrying the spec's marker already
// sits in its matcher group. Absent, unreadable or unparsable files yield
// an empty map — "present" is the only claim made, from bytes read.
func harnessHookEntriesPresent(root string, f harness.HookFile, specs []hookSpec) map[hookSpecKey]bool {
	out := map[hookSpecKey]bool{}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path)))
	if err != nil {
		return out
	}
	var parsed map[string]any
	if json.Unmarshal(data, &parsed) != nil {
		return out
	}
	hooksObj, _ := parsed["hooks"].(map[string]any)
	for _, spec := range specs {
		if hookMarkerPresent(hooksObj, spec) {
			out[hookSpecKeyOf(spec)] = true
		}
	}
	return out
}

// wiredUnverifiedHooksNote is the honest hooks line init prints for a
// descriptor whose hooks it wrote but no real session has verified.
func wiredUnverifiedHooksNote(d harness.Descriptor) string {
	f, ok := d.InitHookFile()
	if !ok {
		return fmt.Sprintf("hooks: %s documents no repo-scoped hook file this build can write — nothing written", d.Display)
	}
	note := fmt.Sprintf("hooks: %d fathom entries written to %s (unverified on this machine — no %s session has run them here)", len(d.Hooks), f.Path, d.Display)
	if f.Trust != "" {
		note += "; before they run: " + f.Trust
	}
	return note
}

// unverifiedHookSentences renders the stanza's two hook paragraphs for a
// harness whose hooks are wired but unverified (HP-11, rule 3): the text
// names the channel that IS wired, the trust step that gates it, and the
// pull-tier fallback — never a promise the harness has not kept yet.
func unverifiedHookSentences(d harness.Descriptor, intentOn bool) (obligations, continuity string) {
	f, _ := d.InitHookFile()
	trust := "once the fathom entries in " + f.Path + " are trusted"
	if f.Trust != "" {
		trust = "once you " + trustLeadClause(f.Trust)
	}
	if intentOn {
		obligations = "Obligations — governing rules for the file you are about to touch, and\n" +
			"where `docs/intent/` exists its matched invariants — are injected on every\n" +
			"edit by the fathom edit hook in " + f.Path + " " + trust + "\n" +
			"(" + d.Display + "; unverified here); until then call the `obligations` MCP tool\n" +
			"before you edit. Treat them as binding, not advisory."
	} else {
		obligations = "Obligations — governing rules for the file you are about to touch — are\n" +
			"injected on every edit by the fathom edit hook in " + f.Path + " " + trust + "\n" +
			"(" + d.Display + "; unverified here); until then call the `obligations` MCP tool\n" +
			"before you edit. Treat them as binding, not advisory."
	}
	continuity = "Continuity: the fathom SessionStart entry in " + f.Path + " prints the\n" +
		"identity, freshness, live-example, thread-brief, and custodian primer " + trust + ";\n" +
		"run `fathom prime` at session start if nothing was injected. File what the next\n" +
		"session must know: `fathom thread note \"…\"`."
	return obligations, continuity
}

// trustLeadClause keeps a trust note's leading imperative (up to the first
// " and " or ";"), so the stanza stays one sentence per paragraph.
func trustLeadClause(s string) string {
	for _, sep := range []string{"; ", " and "} {
		if i := strings.Index(s, sep); i > 0 {
			s = s[:i]
		}
	}
	return s
}
