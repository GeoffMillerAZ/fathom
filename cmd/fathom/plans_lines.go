package main

// plans_lines.go — `fathom plans lines [--json]` (SV-2, ADR-060 decision 2,
// vision 41 §4.1): one repository's local branches, linked worktrees and
// stashes, each classed landed, at risk, idle or active, with its age. For a
// landed line it prints the command that would remove it. The claims-rows and
// touches-rows classes were removed when SV-M1 read CUT (ADR-060 amendment
// 2026-10-02). It runs no git write, deletes nothing and rewrites no history
// (I63, P15): every git read goes through gitro, and the removal command is
// text for the operator.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/worklines"
)

// linesGitRunner runs the verb's git reads; a test swaps in a runner that
// refuses any write.
var linesGitRunner worklines.Runner = gitro.Output

func cmdPlansLines(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("plans lines", stderr)
	root := fs.String("root", "", "a working tree of the repository (default: the current directory)")
	integration := fs.String("integration", "main", "the integration branch lines are measured against")
	idleDays := fs.Int("idle-days", 14, "days without a commit before a line with no other fact reads idle")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	dir := *root
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		dir = wd
	}
	rep, err := worklines.ClassifyLines(context.Background(), worklines.Options{
		Root:        dir,
		Integration: *integration,
		Run:         linesGitRunner,
		IdleAfter:   time.Duration(*idleDays) * 24 * time.Hour,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rep)
		return 0
	}
	renderPlansLines(stdout, rep)
	return 0
}

func renderPlansLines(w io.Writer, rep worklines.Report) {
	tip := rep.IntegrationTip
	if len(tip) > 12 {
		tip = tip[:12]
	}
	fmt.Fprintf(w, "fathom plans lines — %s (integration %s at %s)\n", rep.Root, rep.Integration, tip)
	if len(rep.Lines) == 0 {
		fmt.Fprintln(w, "no lines besides the integration branch")
	}
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "class\tline\tage\tnote")
	for _, l := range rep.Lines {
		name := l.Name
		if l.Kind != worklines.KindBranch {
			name = l.Kind + " " + name
		}
		var note []string
		if l.Landed != "" {
			note = append(note, l.Landed+"; would remove: "+l.Removal)
		}
		note = append(note, l.AtRisk...)
		if l.Kind == worklines.KindBranch && l.Worktree != "" {
			note = append(note, "worktree "+l.Worktree)
		}
		fmt.Fprintf(tw, "%s\t%s\t%dd\t%s\n", l.Class, name, l.AgeDays, strings.Join(note, "; "))
	}
	_ = tw.Flush()
	var parts []string
	for _, c := range worklines.Classes {
		if n := rep.Counts[c]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", c, n))
		}
	}
	if len(parts) > 0 {
		fmt.Fprintf(w, "lines: %s\n", strings.Join(parts, ", "))
	}
	fmt.Fprintln(w, "fathom ran no git write; a removal command is for you to run (I63)")
}
