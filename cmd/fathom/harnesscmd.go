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
		fmt.Fprintln(stderr, "usage: fathom harness list [--json] | capabilities [--harness ID] [--json] | matrix [--url U] [--json]")
		return 1
	}
	switch args[0] {
	case "list":
		return cmdHarnessList(args[1:], stdout, stderr)
	case "capabilities":
		return cmdHarnessCapabilities(args[1:], stdout, stderr)
	case "matrix":
		return cmdHarnessMatrix(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown harness subcommand %q (want list | capabilities | matrix)\n", args[0])
		return 1
	}
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
