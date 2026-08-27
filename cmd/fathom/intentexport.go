package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/intent"
)

// intentexport.go — EP-21's PROPOSE half, printed only.
//
// The router's sensitive-surface table already says which globs matter and
// what evidence a change to them needs. `.fathom/guard.cue` already serves
// exactly that shape before an edit. So the edit-time channel for intent is
// not a second injector — it is this: print the guard entries the router
// implies, for a human to paste. Fathom never writes them (law 15), and a
// proposal a human applies is the custodian's PROPOSE precedent.
func cmdIntentExport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("intent export", stderr)
	root := fs.String("root", ".", "repository root")
	guard := fs.Bool("guard", false, "propose .fathom/guard.cue entries from the router's sensitive surfaces")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if !*guard {
		fmt.Fprintln(stderr, "error: intent export needs --guard (the only export that exists today)")
		return 1
	}
	abs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	c, err := intent.Load(abs)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if len(c.Surfaces) == 0 {
		fmt.Fprintln(stderr, "no sensitive surfaces in the router — nothing to propose")
		return 0
	}

	byID := map[string]intent.Invariant{}
	for _, inv := range c.Invariants {
		byID[inv.ID] = inv
	}

	fmt.Fprintln(stdout, "// PROPOSED guard entries, derived from the router's sensitive-surface table.")
	fmt.Fprintln(stdout, "// Fathom does not write this file (law 15): review and paste what you want.")
	fmt.Fprintf(stdout, "// Source: %s\n\n", c.RouterPath)
	fmt.Fprintln(stdout, "guards: [")
	for _, s := range c.Surfaces {
		note := guardNote(s, byID)
		fmt.Fprintf(stdout, "\t{\n\t\tscopes: [%s]\n\t\tnote: %q\n\t\tmax_rules: 2\n\t},\n",
			quoteJoin(s.Globs), note)
	}
	fmt.Fprintln(stdout, "]")
	return 0
}

// guardNote renders one surface as a pointer, never a copy: the invariant
// IDS and their rungs plus the Done-needs tests. The statements live in
// invariants.md and stay there — a guard note that copies prose becomes a
// second home for the same fact, and drifts.
func guardNote(s intent.Surface, byID map[string]intent.Invariant) string {
	var ids []string
	for id, inv := range byID {
		if strings.Contains(s.Why, id) {
			ids = append(ids, fmt.Sprintf("%s [%s]", id, inv.Rung))
		}
	}
	sortStringsLocal(ids)
	parts := []string{}
	if len(ids) > 0 {
		parts = append(parts, "governed by "+strings.Join(ids, " · "))
	}
	if len(s.Tests) > 0 {
		parts = append(parts, "done needs: "+strings.Join(s.Tests, ", "))
	}
	if len(parts) == 0 {
		parts = append(parts, strings.TrimSpace(s.DoneNeeds))
	}
	return "intent: " + strings.Join(parts, "; ") + " (fathom intent audit)"
}

func quoteJoin(in []string) string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, fmt.Sprintf("%q", s))
	}
	return strings.Join(out, ", ")
}

func sortStringsLocal(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
