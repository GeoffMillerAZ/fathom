package main

// harnesscmd.go — `fathom harness list|capabilities` (HP-8, law 10: the
// verb first, the doc pinned to it). Everything printed here is read from
// internal/harness's registry; nothing is hand-maintained, and an
// unverified descriptor says so on every row.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

func cmdHarness(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: fathom harness list [--json] | capabilities [--harness ID] [--json] | hooks --harness ID [--bin PATH] | ladder [--url BASE] [--json] | matrix [--url U] [--json]")
		return 1
	}
	switch args[0] {
	case "list":
		return cmdHarnessList(args[1:], stdout, stderr)
	case "capabilities":
		return cmdHarnessCapabilities(args[1:], stdout, stderr)
	case "ladder":
		return cmdHarnessLadder(args[1:], stdout, stderr)
	case "matrix":
		return cmdHarnessMatrix(args[1:], stdout, stderr)
	case "hooks":
		return cmdHarnessHooks(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown harness subcommand %q (want list | capabilities | hooks | ladder | matrix)\n", args[0])
		return 1
	}
}

// cmdHarnessLadder prints the tier B/C/D rows (HP-22): the harnesses
// fathom reaches with stanza + MCP + skills and no injection, each with
// its instruction file, skills path, MCP config, `mcp add` line, the
// honest "no injection here" line and the primary source it was read from.
func cmdHarnessLadder(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("harness ladder", stderr)
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL filled into each row's mcp add line")
	jsonOut := fs.Bool("json", false, "emit the rows as JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(harness.LadderRows()); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprint(stdout, harness.RenderLadderTable(*url))
	return 0
}

// cmdHarnessMatrix renders the adoption matrix (RP-18) — the table
// docs/adoption/other-harnesses.md pins between its markers — as Markdown
// or --json rows.
func cmdHarnessMatrix(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("harness matrix", stderr)
	url := fs.String("url", "http://127.0.0.1:7575", "daemon base URL the `mcp add` lines are rendered with")
	jsonOut := fs.Bool("json", false, "emit the rows as JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	now := time.Now()
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(harness.AdoptionRows(*url, now)); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprint(stdout, harness.RenderAdoptionMatrix(*url, now))
	return 0
}

// cmdHarnessList prints one line per registry descriptor (or the exported
// JSON the CUE schema validates, with --json).
func cmdHarnessList(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("harness list", stderr)
	jsonOut := fs.Bool("json", false, "emit the exported descriptor registry as JSON (schema/harness/schema.cue)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *jsonOut {
		data, err := harness.ExportJSON()
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(data))
		return 0
	}
	now := time.Now()
	fmt.Fprintf(stdout, "%-12s %-4s %-6s %-9s %-22s %s\n", "ID", "TIER", "CODEC", "HOOKS", "STANZA", "VERIFIED")
	for _, d := range harness.All() {
		codec := d.Codec
		if codec == "" {
			codec = "-"
		}
		hooks := "none"
		switch {
		case d.HooksWired():
			hooks = "wired"
		case d.HasHooks():
			hooks = "data only"
		}
		verified := d.VerifiedAgainst
		if due, _ := harness.DemotionDue(d, now); due {
			verified = "DEMOTED (unverified past the 60-day clock)"
		}
		fmt.Fprintf(stdout, "%-12s %-4s %-6s %-9s %-22s %s\n",
			d.ID, harness.EffectiveTier(d, now), codec, hooks, strings.Join(d.StanzaFiles(), "+"), verified)
	}
	return 0
}

// cmdHarnessCapabilities renders the honest per-capability table for one
// descriptor (--harness) or every descriptor, as Markdown or --json rows.
func cmdHarnessCapabilities(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("harness capabilities", stderr)
	id := fs.String("harness", "", "registry id (default: every descriptor)")
	jsonOut := fs.Bool("json", false, "emit the rows as JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	var descs []harness.Descriptor
	if *id != "" {
		d, ok := harness.Lookup(*id)
		if !ok {
			fmt.Fprintf(stderr, "error: unknown harness %q (known: %s)\n", *id, strings.Join(harness.IDs(), ", "))
			return 1
		}
		descs = []harness.Descriptor{d}
	} else {
		descs = harness.All()
	}
	now := time.Now()
	if *jsonOut {
		type entry struct {
			ID              string                  `json:"id"`
			Tier            string                  `json:"tier"`
			VerifiedAgainst string                  `json:"verified_against"`
			Rows            []harness.CapabilityRow `json:"capabilities"`
		}
		out := make([]entry, 0, len(descs))
		for _, d := range descs {
			out = append(out, entry{ID: d.ID, Tier: harness.EffectiveTier(d, now), VerifiedAgainst: d.VerifiedAgainst, Rows: harness.CapabilityRows(d, now)})
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}
	for i, d := range descs {
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		fmt.Fprintf(stdout, "## %s (%s) — tier %s — %s\n\n", d.ID, d.Display, harness.EffectiveTier(d, now), d.VerifiedAgainst)
		fmt.Fprint(stdout, harness.RenderCapabilityTable(d.ID, now))
	}
	return 0
}

// harnessHomeDir is the home directory Detect reads "~/" markers under;
// "" when it cannot be resolved (a home marker then never matches).
func harnessHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// cmdHarnessHooks prints the hook entries for a descriptor as the JSON
// its harness expects, ready to paste (HP-34). It exists because of a
// measured fact: Codex CLI 0.153.1 loads hooks ONLY from its user-global
// CODEX_HOME/hooks.json, and fathom does not edit user-global
// configuration unasked (docs/intent/autonomy.md). Printing is the honest
// half of that rule — refusing to write is not a reason to leave the
// operator to hand-assemble eight entries from a descriptor dump.
//
// It writes nothing, anywhere.
func cmdHarnessHooks(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("harness hooks", stderr)
	id := fs.String("harness", "", "registry id whose hook entries to print (required; `fathom harness list` names them)")
	binPath := fs.String("bin", "", "absolute path to the fathom binary the entries should call (default: this executable)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *id == "" {
		fmt.Fprintf(stderr, "error: harness hooks needs --harness ID (one of %s)\n", strings.Join(harness.IDs(), ", "))
		return 1
	}
	d, ok := harness.Lookup(*id)
	if !ok {
		fmt.Fprintf(stderr, "error: unknown --harness %q (want %s)\n", *id, strings.Join(harness.IDs(), ", "))
		return 1
	}
	specs := hookSpecsFor(d.ID)
	if len(specs) == 0 {
		fmt.Fprintf(stderr, "error: %s documents no hook bindings in this build — there is nothing to paste; `fathom harness capabilities --harness %s` shows what reaches it instead\n", d.Display, d.ID)
		return 1
	}
	bin := *binPath
	if bin == "" {
		if exe, err := os.Executable(); err == nil {
			bin = exe
		} else {
			bin = "fathom"
		}
	}

	// Which file, and the unit its timeouts are in, come from the
	// descriptor — never from a literal here.
	var target harness.HookFile
	for _, f := range d.HookFiles {
		if f.Kind != "" {
			target = f
			break
		}
	}
	type entry struct {
		Type    string `json:"type"`
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	}
	type group struct {
		Matcher string  `json:"matcher,omitempty"`
		Hooks   []entry `json:"hooks"`
	}
	byEvent := map[string][]group{}
	var order []string
	for _, sp := range specs {
		timeout := sp.timeout
		if target.TimeoutUnit == "ms" {
			timeout *= 1000
		}
		if _, seen := byEvent[sp.event]; !seen {
			order = append(order, sp.event)
		}
		byEvent[sp.event] = append(byEvent[sp.event], group{
			Matcher: sp.matcher,
			Hooks:   []entry{{Type: "command", Command: bin + " " + sp.sub, Timeout: timeout}},
		})
	}
	// Deterministic output: encoding/json sorts the map keys, and the
	// groups inside each event keep binding order.
	payload := map[string]any{"hooks": byEvent}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if target.Path != "" {
		fmt.Fprintf(stdout, "# %d fathom hook entries for %s — merge into %s (%s scope)\n", len(specs), d.Display, target.Path, target.Scope)
		if target.Trust != "" {
			fmt.Fprintf(stdout, "# before they run: %s\n", target.Trust)
		}
		if d.Env.Home != "" {
			fmt.Fprintf(stdout, "# %s relocates that file\n", d.Env.Home)
		}
		fmt.Fprintln(stdout, "# fathom never writes it: user-global configuration is ask-first (docs/intent/autonomy.md)")
	}
	fmt.Fprintln(stdout, string(raw))
	_ = order
	return 0
}
