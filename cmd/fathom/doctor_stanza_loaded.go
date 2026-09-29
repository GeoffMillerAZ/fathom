package main

// doctor_stanza_loaded.go — PC-25 (docs/vision/26-proof-credibility.md
// §4): the `stanza_loaded` row. stanza_present reads the FILE (I37: the
// stanza is well-formed in AGENTS.md); this row reads the HARNESS: did a
// session actually load it? The evidence is the InstructionsLoaded beat
// `fathom session-hook --event instructions-loaded` writes
// (sessionhook_instructions.go). PASS when a beat names this repo's
// AGENTS.md (arriving through a CLAUDE file's import) or a root CLAUDE
// file; SKIP — never FAIL — when no such beat exists, because absence of a
// beat is absence of evidence: the hook may be freshly wired, the harness
// may predate the event, or the beat log may have rotated. In a repository
// with no root CLAUDE file the SKIP is named (ADR-059, AG-12): Claude Code
// loads AGENTS.md natively there and writes no InstructionsLoaded event,
// so no harness-side evidence can exist; the row never synthesizes a PASS.
// That named SKIP says Claude Code loads the file only where stanza_present
// does not warn that it will not (an ancestor CLAUDE file, the claude-md
// setting, a release before 2.1.277); where it does warn, the SKIP repeats the
// warning and its remedy instead of claiming a load.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
)

// stanzaLoadedFiles are the instruction files, repo-relative, whose load
// counts as "the stanza reached a session": AGENTS.md (the stanza's one
// file, ADR-059 — its beat arrives when a CLAUDE file imports it) and the
// root CLAUDE files an older layout carried it in.
var stanzaLoadedFiles = []string{"AGENTS.md", "CLAUDE.md", ".claude/CLAUDE.md", "CLAUDE.local.md"}

// stanzaLoadedNoClaudeReason is the named SKIP for a repository with no
// root CLAUDE file (AG-12): Claude Code loads its AGENTS.md through the
// agents-md setting, and that load writes no InstructionsLoaded event.
const stanzaLoadedNoClaudeReason = "Claude Code (≥2.1.277) loads AGENTS.md through the agents-md setting without an InstructionsLoaded event; no harness-side evidence exists here; stanza_present covers the file side"

// checkStanzaLoaded reads the InstructionsLoaded beats for root.
func checkStanzaLoaded(root string) doctorCheck {
	const name = "stanza_loaded"
	beats, path, err := loadInstructionsLoadedBeats()
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("beat log %s unreadable (%v) — no InstructionsLoaded evidence either way", path, err)}
	}
	want := resolveSymlinks(root)
	var hits []heartbeat.Beat
	for _, b := range beats {
		fp := b.Meta["file_path"]
		if fp == "" {
			continue
		}
		known := false
		for _, f := range stanzaLoadedFiles {
			rel := filepath.FromSlash(f)
			dir := strings.TrimSuffix(strings.TrimSuffix(filepath.Clean(fp), rel), string(filepath.Separator))
			if strings.HasSuffix(filepath.Clean(fp), string(filepath.Separator)+rel) && resolveSymlinks(dir) == want {
				known = true
				break
			}
		}
		if !known {
			continue
		}
		hits = append(hits, b)
	}
	if len(hits) == 0 && rootClaudeFile(root) == "" {
		return stanzaLoadedNoClaudeSkip(root)
	}
	if len(hits) == 0 {
		return doctorCheck{
			Name:   name,
			Status: doctorSkipped,
			Reason: fmt.Sprintf("no InstructionsLoaded beat for %s/{AGENTS.md,CLAUDE.md} yet (%s) — the harness has not reported loading the stanza; stanza_present still covers the file side", root, stanzaLoadedWiringHint(root)),
			Remedy: "fathom init --path " + root + " (adds the InstructionsLoaded hook row), then start one session in this repo",
		}
	}
	// Newest first, deterministic on ties (log order).
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].At.After(hits[j].At) })
	latest := hits[0]
	reason := fmt.Sprintf("%s loaded by a session (%s, latest %s, %d beat(s))", filepath.Base(latest.Meta["file_path"]), stanzaLoadedReasonLabel(latest), latest.At.UTC().Format(time.RFC3339), len(hits))
	return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
}

// stanzaLoadedNoClaudeSkip is the SKIP for a repository with no root CLAUDE
// file and no beat. It reads the same load path stanza_present reads
// (claudeLoadPath): with no warning it is the named agents-md SKIP; with one,
// the load that SKIP would claim is the load stanza_present denies, so it
// carries stanza_present's machine-side reasons and remedies instead.
func stanzaLoadedNoClaudeSkip(root string) doctorCheck {
	const name = "stanza_loaded"
	reasons, remedies := claudeLoadPath(root, true).warnings()
	if len(reasons) == 0 {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: stanzaLoadedNoClaudeReason}
	}
	return doctorCheck{
		Name:   name,
		Status: doctorSkipped,
		Reason: "no harness-side evidence exists here, and stanza_present warns Claude Code will not load AGENTS.md: " + strings.Join(reasons, "; "),
		Remedy: strings.Join(dedupeStrings(remedies), "; "),
	}
}

// stanzaLoadedReasonLabel renders the beat's load_reason and memory_type
// when recorded — "load_reason unrecorded" otherwise, never a guess.
func stanzaLoadedReasonLabel(b heartbeat.Beat) string {
	reason := b.Meta["load_reason"]
	if reason == "" {
		reason = "load_reason unrecorded"
	}
	if mt := b.Meta["memory_type"]; mt != "" {
		return reason + ", " + mt
	}
	return reason
}

// stanzaLoadedWiringHint names when the InstructionsLoaded row reached
// this repo's project settings ("hook wired at <mtime>?") so a reader can
// tell "wired an hour ago, no session since" from "never wired". The
// mtime is the file's, not the row's — hence the question mark. User
// scope is not read here: doctor's hooks_installed row already covers it.
func stanzaLoadedWiringHint(root string) string {
	p := filepath.Join(root, ".claude", "settings.json")
	data, err := os.ReadFile(p)
	if err != nil {
		return "hook not wired here: .claude/settings.json unreadable"
	}
	if !strings.Contains(string(data), "fathom session-hook --event instructions-loaded") {
		return "hook not wired here: no InstructionsLoaded row in .claude/settings.json"
	}
	fi, err := os.Stat(p)
	if err != nil {
		return "hook wired at an unknown date?"
	}
	return "hook wired at " + fi.ModTime().UTC().Format(time.RFC3339) + "?"
}

// loadInstructionsLoadedBeats reads every instructions-loaded beat from the
// central beat log. A missing log is zero beats, no error — a machine
// that never ran the hook is normal; a present-but-unreadable log is the
// error the caller reports as SKIP.
func loadInstructionsLoadedBeats() ([]heartbeat.Beat, string, error) {
	path := beatLogPath()
	if path == "" {
		return nil, "", nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, path, nil
		}
		return nil, path, err
	}
	defer f.Close()
	beats, err := heartbeat.Load(f, true) // acceptPartial: a torn tail must not blank the history
	if err != nil {
		return nil, path, err
	}
	var out []heartbeat.Beat
	for _, b := range beats {
		if b.Channel == instructionsLoadedChannel {
			out = append(out, b)
		}
	}
	return out, path, nil
}
