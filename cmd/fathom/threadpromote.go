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
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/verify"
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
	// RC-30 remainder (docs/vision/28-receipts.md §4): "thread promote can
	// write a route onto a plan row's ctx:" — a SEPARATE mode from the
	// thread-scaffold above. --route names a package/concern prefix (the
	// same one `fathom routes <package>` filters by); --plan/--row name
	// the EXISTING row to annotate, never a new plan.
	route := fs.String("route", "", "package/concern prefix: instead of scaffolding a plan, write that route's pointers onto an existing plan row's ctx: annotation (requires --plan and --row)")
	planFile := fs.String("plan", "", "plan file to annotate (required with --route)")
	row := fs.String("row", "", "row id to annotate (required with --route)")
	service := fs.String("service", "", "service to resolve route pointers against (default: this repo's indexed service; --route only)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL (--route only)")
	storePath := fs.String("store", "", "store path, .sqlite/.db, direct mode (--route only)")
	callLog := fs.String("call-log", "", "call log path (default: workspace/logs/mcp-calls.jsonl under --repo; --route only)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *route != "" {
		return cmdThreadPromoteRoute(*route, *planFile, *row, *repo, *service, *url, *storePath, *callLog, stdout, stderr)
	}
	if lead == "" && fs.NArg() == 1 {
		lead = fs.Arg(0)
	}
	if lead == "" {
		fmt.Fprintln(stderr, "usage: fathom thread promote <thread> [--repo R] [--out path]\n"+
			"       fathom thread promote --route <package> --plan <file> --row <id> [--service S] [--url URL | --store DB]")
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

// cmdThreadPromoteRoute implements `fathom thread promote --route <package>
// --plan <file> --row <id>` (RC-30 remainder, docs/vision/28-receipts.md
// §4): derive route's pointers via the SAME derivation `fathom routes`
// uses (routedPointers, scoped to service and the routesWindow lookback,
// filtered to package and resolved fresh against the live index — a
// pointer the index no longer confirms is dropped exactly as cmdRoutes
// drops it), then write them onto the named plan row's `ctx:` annotation
// (verify.UpsertAnnotationField) — UNIONED with whatever pointers the row
// already carries, never replacing them, since ctx: is a discovery pack,
// not a single-owner field.
//
// An unreachable index refuses outright (never silently writes an
// unconfirmed pointer set): the whole point of routing through the index
// at write time, not just at derivation time, is that a stale route must
// never get written down as if it still held.
func cmdThreadPromoteRoute(route, planFile, row, repo, service, url, storePath, callLog string, stdout, stderr io.Writer) int {
	if planFile == "" || row == "" {
		fmt.Fprintln(stderr, "error: --route requires --plan <file> and --row <id>")
		return 1
	}
	rootAbs, err := filepath.Abs(repo)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	scope := threadScopeFor(rootAbs)
	svc := service
	if svc == "" {
		svc = scope.service
	}
	logPath := callLog
	if logPath == "" {
		logPath = callLogPathFor(rootAbs)
	}

	now := time.Now()
	windows := readSessionWindows(rootAbs)
	routed := routedPointers(logPath, svc, windows, now.Add(-routesWindow), rootAbs)

	idx, idxErr := openCtxIndex(svc, storePath, url)
	if idxErr != nil {
		fmt.Fprintf(stderr, "error: index unreachable, cannot confirm route %q's pointers still hold: %v\n", route, idxErr)
		return 1
	}

	var pointers []string
	for _, r := range routed {
		pkg := routePackagePrefix(r.Pointer)
		if pkg != route && !strings.HasPrefix(pkg, route) {
			continue
		}
		if _, res := renderOneDiscoveryLine(idx, idxErr, r.Pointer, 0, r.FromIndex, r.ObservedVerb); res != discoveryResolved {
			continue // dropped: the index no longer confirms it (RC-30)
		}
		pointers = append(pointers, r.Pointer)
	}
	sort.Strings(pointers) // determinism (internal/AGENTS.md)
	if len(pointers) == 0 {
		fmt.Fprintf(stderr, "error: no routed pointer under package %q resolves against the live index — nothing to write\n", route)
		return 1
	}

	raw, err := os.ReadFile(planFile)
	if err != nil {
		fmt.Fprintf(stderr, "error: reading plan %q: %v\n", planFile, err)
		return 1
	}
	plan := verify.ParseMarkdownPlan(svc, raw)
	var task *verify.Task
	for i := range plan.Tasks {
		if plan.Tasks[i].ID == row {
			task = &plan.Tasks[i]
			break
		}
	}
	if task == nil {
		fmt.Fprintf(stderr, "error: row %q not found in %q\n", row, planFile)
		return 1
	}

	merged := mergeCtxPointers(task.Ctx, pointers)
	lines := strings.Split(string(raw), "\n")
	if task.Line < 1 || task.Line > len(lines) {
		fmt.Fprintf(stderr, "error: row %q's recorded line %d is out of range for %q\n", row, task.Line, planFile)
		return 1
	}
	// A wrapped row (G2-5) is edited as the lines the parser read for it:
	// its annotation block may sit on a continuation line, or span two.
	end := task.EndLine
	if end < task.Line || end > len(lines) {
		end = task.Line
	}
	newText, ok := verify.UpsertAnnotationField(strings.Join(lines[task.Line-1:end], "\n"), "ctx", strings.Join(merged, " "))
	if !ok {
		fmt.Fprintf(stderr, "error: row %q's annotation could not be safely edited (a malformed/suspect block) — fix it by hand first\n", row)
		return 1
	}
	lines = append(append(append([]string{}, lines[:task.Line-1]...), strings.Split(newText, "\n")...), lines[end:]...)
	if err := os.WriteFile(planFile, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %d route pointer(s) onto %s row %q's ctx: %s\n", len(pointers), planFile, row, strings.Join(merged, " "))
	return 0
}

// mergeCtxPointers unions existing with additions, deduped, existing
// pointers first in their original order, then any addition not already
// present — a route promotion EXTENDS a row's discovery pack, it never
// replaces what a human or a prior promotion already grounded there.
func mergeCtxPointers(existing, additions []string) []string {
	seen := make(map[string]bool, len(existing)+len(additions))
	out := make([]string, 0, len(existing)+len(additions))
	for _, p := range existing {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, p := range additions {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
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
