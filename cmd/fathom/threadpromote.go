package main

// threadpromote.go — TH-5: the two verbs that connect threads to the rest
// of the lifecycle.
//
// promote: intent graduates into the plan ledger — the airlock feeds the
// gate, nothing skips it. handoff: the brief rendered as a harness-neutral
// artifact, because continuity that lives outside every context window is
// exactly what makes multi-agent handoff free.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// cmdThreadPromote scaffolds a gate-ready plan from a thread's open items.
//
// The scaffold is HONEST about what it is: rows carry stable ids but no
// symbol contracts — grounding a contract needs the index and a human or
// agent who knows the shape of the work (the fathom-plan skill's job).
// A row cannot be claimed until then, which is exactly right: promote
// moves intent into the ledger, not into "done".
func cmdThreadPromote(args []string, stdout, stderr io.Writer) int {
	var lead string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		lead, args = args[0], args[1:]
	}
	fs := newFlagSet("thread promote", stderr)
	repo := fs.String("repo", ".", "repository whose docs/plans/ receives the scaffold")
	out := fs.String("out", "", "output path (default docs/plans/<thread>.md)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if lead == "" && fs.NArg() == 1 {
		lead = fs.Arg(0)
	}
	if lead == "" {
		fmt.Fprintln(stderr, "usage: fathom thread promote <thread> [--repo R] [--out path]")
		return 1
	}

	items := loadThreadItems(threadsPath())
	var open []threadItem
	for _, it := range items {
		if it.Thread == lead && it.ClosedAt == "" {
			open = append(open, it)
		}
	}
	if len(open) == 0 {
		fmt.Fprintf(stderr, "error: thread %q has no open items — nothing to promote (fathom thread list --all)\n", lead)
		return 1
	}

	root, _ := filepath.Abs(*repo)
	slug := planSlug(lead)
	dest := *out
	if dest == "" {
		dest = filepath.Join(root, "docs", "plans", slug+".md")
	}
	if _, err := os.Stat(dest); err == nil {
		// Overwriting a plan would destroy gate history; the scaffold is a
		// birth certificate, not an update mechanism.
		fmt.Fprintf(stderr, "error: %s already exists — promote scaffolds NEW plans; edit the existing one directly\n", dest)
		return 1
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s — promoted from the continuity thread %q\n\n", slug, lead)
	b.WriteString("> Scaffolded by `fathom thread promote` — intent graduating into the gate's\n")
	b.WriteString("> ledger. Rows carry stable ids but NO contracts yet: ground each with a\n")
	b.WriteString("> real {symbol, package, reachable_from} (the fathom-plan skill) before any\n")
	b.WriteString("> box is claimed. `fathom verify` gates claims exactly as everywhere else;\n")
	b.WriteString("> promote moves intent into the ledger, never into \"done\".\n>\n")
	fmt.Fprintf(&b, "> Source items carry their session pointers; the transcripts stay on disk.\n\n")
	upper := strings.ToUpper(planIDPrefix(slug))
	n := 0
	for _, it := range open {
		n++
		line := strings.TrimSpace(it.Text)
		ptr := ""
		if sid := it.Pointers["session_id"]; sid != "" {
			ptr = fmt.Sprintf(" (origin: session %s, filed %s)", sid[:min(8, len(sid))], it.FiledAt[:10])
		}
		fmt.Fprintf(&b, "- [ ] %s%s {id: %s-%d}\n", line, ptr, upper, n)
	}
	b.WriteString("\n")

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := os.WriteFile(dest, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	// The promoted items point at their plan and will CLOSE THEMSELVES as
	// its rows are checked — the conveyor advancing on its own.
	now := time.Now()
	rel, rerr := filepath.Rel(root, dest)
	if rerr != nil {
		rel = dest
	}
	for i, it := range open {
		_ = appendThreadItem(threadItem{
			SchemaVersion: threadSchemaVersion, ID: it.ID,
			Thread: it.Thread, Kind: it.Kind, Text: it.Text, Pointers: it.Pointers, FiledAt: it.FiledAt,
			CompletesWhen: &threadCondition{
				Kind: "plan_row_checked", Repo: root, Path: rel,
				Row: fmt.Sprintf("%s-%d", upper, i+1),
			},
		})
		_ = now
	}
	fmt.Fprintf(stdout, "promoted %d item(s) from thread %q → %s\n", n, lead, dest)
	fmt.Fprintln(stdout, "next: ground each row's contract (fathom-plan), then build; the thread items close themselves as rows are checked")
	return 0
}

var slugRE = regexp.MustCompile(`[^a-z0-9]+`)

func planSlug(thread string) string {
	s := slugRE.ReplaceAllString(strings.ToLower(thread), "-")
	return strings.Trim(s, "-")
}

func planIDPrefix(slug string) string {
	parts := strings.Split(slug, "-")
	if len(parts) == 1 {
		if len(slug) > 4 {
			return slug[:4]
		}
		return slug
	}
	var b strings.Builder
	for _, p := range parts {
		if p != "" {
			b.WriteByte(p[0])
		}
	}
	return b.String()
}

// cmdThreadHandoff renders the continuity state as a harness-neutral
// markdown artifact: the same composition as the brief, unbudgeted,
// because a handoff is read by an agent that has NO other context — the
// budget discipline protects a session that already has one.
func cmdThreadHandoff(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("thread handoff", stderr)
	repo := fs.String("repo", ".", "repository (for derived in-flight state)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	root, _ := filepath.Abs(*repo)
	items := evaluateThreadConditions(loadThreadItems(threadsPath()), root, time.Now())

	var b strings.Builder
	b.WriteString("# Continuity handoff\n\n")
	b.WriteString("Generated by `fathom thread handoff` — the open work, harness-neutral.\n")
	b.WriteString("Session transcripts referenced by id live on the originating machine.\n\n")
	byThread := map[string][]threadItem{}
	var names []string
	for _, it := range items {
		if it.ClosedAt != "" {
			continue
		}
		if _, ok := byThread[it.Thread]; !ok {
			names = append(names, it.Thread)
		}
		byThread[it.Thread] = append(byThread[it.Thread], it)
	}
	sortStringsAsc(names)
	if len(names) == 0 {
		b.WriteString("Nothing open. Derived state only:\n\n")
	}
	for _, name := range names {
		fmt.Fprintf(&b, "## %s\n\n", name)
		for _, it := range byThread[name] {
			ptr := ""
			if sid := it.Pointers["session_id"]; sid != "" {
				ptr = " — session " + sid
			}
			dormant := ""
			if threadDormant(it, time.Now()) {
				dormant = " (dormant)"
			}
			fmt.Fprintf(&b, "- **%s**%s: %s%s\n", it.Kind, dormant, it.Text, ptr)
		}
		b.WriteString("\n")
	}
	if wip := derivedInFlight(root); wip != "" {
		b.WriteString("## Derived in-flight state\n\n- " + wip + "\n\n")
	}
	b.WriteString("Open plan work: `fathom plans next --service <svc>` · full store: `fathom thread list --all`\n")
	fmt.Fprint(stdout, b.String())
	return 0
}

func sortStringsAsc(xs []string) {
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j] < xs[j-1]; j-- {
			xs[j], xs[j-1] = xs[j-1], xs[j]
		}
	}
}
