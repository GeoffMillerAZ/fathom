package main

// inittargets.go — `fathom init --harness=<id>` (HP-13): which vehicles
// init writes for one registry descriptor, and nothing more. Repo-scoped
// files only; the harness's own `mcp add` line is printed, never run
// (user-global configuration is ask-first — docs/intent/autonomy.md); a
// descriptor past its demotion clock is refused with the reason.

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// initHarnessNone is the `--harness=none` pseudo-id: stanza + .fathom/
// only, with the pull-tier hook sentences, into AGENTS.md (ADR-059).
const initHarnessNone = "none"

// initTargets is what init writes for one harness.
type initTargets struct {
	// ID is the registry id, or initHarnessNone.
	ID      string
	Display string
	// StanzaID is the registry id whose hook sentences the stanza carries
	// (the generic pull tier for `none`).
	StanzaID string
	// StanzaFiles are the instruction files the stanza region goes into:
	// AGENTS.md, for every harness (ADR-059). A root CLAUDE file gets the
	// import region besides (instructionRegions), whatever the harness.
	StanzaFiles []string
	// Settings are the repo-scoped settings keys that make the harness read
	// AGENTS.md (Gemini CLI's context.fileName), written as wiring keys.
	Settings []harness.InstructionSetting
	// Hooks says whether the hook table is planned (Claude Code only in
	// this build: the one descriptor whose codec is wired).
	Hooks bool
	// MCPAddLine is the descriptor's `mcp add` template ({{url}}); "" for
	// `none`.
	MCPAddLine string
	// HooksNote names why no hook file was written, for the descriptors
	// that document hooks fathom does not yet drive.
	HooksNote string
}

// initTargetsFor resolves --harness into targets. "" ⇒ claude-code. now
// drives the demotion clock (vision 25 §5): an unverified descriptor 60
// days past its codec landing is refused here, before any write.
func initTargetsFor(id string, now time.Time) (initTargets, error) {
	if id == "" {
		id = harness.ClaudeCode
	}
	if id == initHarnessNone {
		return initTargets{
			ID:          initHarnessNone,
			Display:     "no harness",
			StanzaID:    harness.Agents,
			StanzaFiles: []string{harness.AgentsMD},
		}, nil
	}
	d, ok := harness.Lookup(id)
	if !ok {
		return initTargets{}, fmt.Errorf("unknown --harness %q (want %s, or %s)", id, strings.Join(harness.IDs(), ", "), initHarnessNone)
	}
	return initTargetsForDescriptor(d, now)
}

// initTargetsForDescriptor is initTargetsFor's descriptor half: the
// demotion refusal and the vehicle selection, testable against a synthetic
// descriptor whose clock has run out.
func initTargetsForDescriptor(d harness.Descriptor, now time.Time) (initTargets, error) {
	id := d.ID
	if due, reason := harness.DemotionDue(d, now); due {
		return initTargets{}, fmt.Errorf("refusing --harness=%s: %s", id, reason)
	}
	t := initTargets{
		ID:          d.ID,
		Display:     d.Display,
		StanzaID:    d.ID,
		StanzaFiles: d.StanzaFiles(),
		Hooks:       d.HooksWired(),
		MCPAddLine:  d.MCP.AddCommand,
	}
	// AG-6: a harness that reads AGENTS.md only through a settings key gets
	// that key written (a wiring key, receipted) instead of a stanza in its
	// own instruction file.
	for _, f := range d.Instructions {
		if f.Mode == "settings-key" && f.Setting != nil {
			t.Settings = append(t.Settings, *f.Setting)
		}
	}
	switch {
	case d.HooksWired() && !hasInitHookFile(d):
		// HP-34: the descriptor's hooks are real and its codec is wired,
		// but the harness loads them from a file init may not write (Codex
		// 0.153.1 loads only the CODEX_HOME layer, and user-global
		// configuration is ask-first). Writing the documented project file
		// anyway would be wiring that cannot fire, so init writes nothing
		// and prints the entries for the operator to paste.
		t.Hooks = false
		t.HooksNote = unwritableHooksNote(d)
	case d.HooksWired() && d.Unverified():
		// HP-18: the descriptor's own hook file is planned through the
		// wiring engine, and the line says it is unverified here and what
		// gates it (init_harnesshooks.go).
		t.HooksNote = wiredUnverifiedHooksNote(d)
	case d.HooksWired():
		// nothing to say: the hook table is planned through the wiring engine
	case d.HasHooks():
		t.HooksNote = fmt.Sprintf("hooks: not wired (wave 2) — %s documents %d hook binding(s) fathom does not drive in this build; the pull tier (`fathom prime`, the `obligations` MCP tool) carries the same content", d.Display, len(d.Hooks))
	default:
		t.HooksNote = fmt.Sprintf("hooks: none — %s has no hook mechanism; the pull tier (`fathom prime`, the `obligations` MCP tool) and the git-native gates carry fathom here", d.Display)
	}
	return t, nil
}

// printHarnessManualSteps is printManualSteps for a non-Claude-Code
// harness: the descriptor's own `mcp add` line (printed, never run), the
// honest hooks line, the settings-key note where the harness needs one,
// and the harness-neutral gate/config steps.
func printHarnessManualSteps(stdout io.Writer, t initTargets, service, url string) {
	fmt.Fprintln(stdout)
	if t.MCPAddLine != "" {
		fmt.Fprintf(stdout, "MANUAL: register fathom as an MCP server for %s (printed, never written by init):\n", t.Display)
		fmt.Fprintf(stdout, "MANUAL:   %s\n", strings.ReplaceAll(t.MCPAddLine, "{{url}}", strings.TrimRight(url, "/")))
	} else {
		fmt.Fprintf(stdout, "MANUAL: register %s/rpc/agent as a streamable-HTTP MCP server named fathom with your harness's own `mcp add`.\n", strings.TrimRight(url, "/"))
	}
	if t.HooksNote != "" {
		fmt.Fprintf(stdout, "MANUAL: %s\n", t.HooksNote)
	} else if t.ID == initHarnessNone {
		fmt.Fprintln(stdout, "MANUAL: hooks: none written (--harness=none) — the stanza names the pull tier instead")
	}
	if d, ok := harness.Lookup(t.ID); ok {
		for _, f := range d.Instructions {
			switch {
			case f.Mode == "settings-key" && f.Setting != nil:
				fmt.Fprintf(stdout, "note: %s reads %s through %s %s (written above, ADR-059)\n", d.Display, f.Path, f.Setting.File, strings.Join(f.Setting.Key, "."))
			case f.Mode == "settings-key":
				fmt.Fprintf(stdout, "MANUAL: %s reads %s only via a settings key — %s\n", d.Display, f.Path, f.Note)
			}
		}
	}
	fmt.Fprintf(stdout, "MANUAL: capability table for this harness: fathom harness capabilities --harness %s\n", t.StanzaID)
	printGateAndConfigManualSteps(stdout, service)
}
