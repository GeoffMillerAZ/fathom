package main

// thread.go — continuity threads (vision 21, docs/design/continuity-threads.md).
//
// The thread is the unit of work; sessions are events on it. This file is
// the store and the verbs. The refusal boundary is structural: nothing
// here reads transcript CONTENT (only ids/paths), nothing here calls an
// LLM, and the only surface with a context cost — the brief — carries a
// byte budget enforced by test.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const threadSchemaVersion = 1

// threadBriefBudgetBytes ≈ 600 tokens. The vision's tokenomics gate: a
// continuity feature that cannot say something useful in this space is
// context pollution with a good conscience.
const threadBriefBudgetBytes = 2400

// threadItem is one row of ~/.fathom/threads.jsonl. Latest row per ID wins
// (the ratings-log discipline): corrections and closures are new rows, so
// the history stays auditable.
type threadItem struct {
	SchemaVersion int               `json:"schema_version"`
	ID            string            `json:"id"`
	Thread        string            `json:"thread"`
	Kind          string            `json:"kind"`
	Text          string            `json:"text"`
	Pointers      map[string]string `json:"pointers,omitempty"`
	FiledAt       string            `json:"filed_at"`
	FiledBy       string            `json:"filed_by,omitempty"`

	// CompletesWhen names a CHECKABLE condition; when it holds, the item
	// closes itself with closed_by: "condition" — the anchored-memory
	// self-expiry philosophy, keyed on artifact existence.
	CompletesWhen *threadCondition `json:"completes_when,omitempty"`

	ClosedAt  string `json:"closed_at,omitempty"`
	ClosedBy  string `json:"closed_by,omitempty"`
	ClosedWhy string `json:"closed_why,omitempty"`
}

type threadCondition struct {
	Kind string `json:"kind"` // file_exists | plan_row_checked
	Repo string `json:"repo,omitempty"`
	Path string `json:"path,omitempty"` // file_exists: the file; plan_row_checked: the plan
	Row  string `json:"row,omitempty"`  // plan_row_checked: the {id: …} to look for
}

var threadKinds = []string{
	"decision-pending", "spec-unbuilt", "in-flight",
	"side-task", "handoff-note", "session-end", "pre-compact",
}

func threadsPath() string {
	if p := os.Getenv("FATHOM_THREADS"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".fathom", "threads.jsonl")
}

func appendThreadItem(it threadItem) error {
	p := threadsPath()
	if p == "" {
		return fmt.Errorf("cannot resolve the threads path (no home directory)")
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	line, err := json.Marshal(it)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

// loadThreadItems folds the log, latest row per id winning, sorted by
// FiledAt then ID (house determinism).
func loadThreadItems(path string) []threadItem {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	latest := map[string]threadItem{}
	var order []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var it threadItem
		if json.Unmarshal([]byte(line), &it) != nil || it.ID == "" {
			continue // a malformed line is skipped, never guessed at
		}
		if _, seen := latest[it.ID]; !seen {
			order = append(order, it.ID)
		}
		prev, seen := latest[it.ID]
		if seen {
			// A later row may be a partial closure record: carry forward
			// fields it does not restate.
			if it.Text == "" {
				it.Text = prev.Text
			}
			if it.Thread == "" {
				it.Thread = prev.Thread
			}
			if it.Kind == "" {
				it.Kind = prev.Kind
			}
			if it.FiledAt == "" {
				it.FiledAt = prev.FiledAt
			}
			if it.Pointers == nil {
				it.Pointers = prev.Pointers
			}
			if it.CompletesWhen == nil {
				it.CompletesWhen = prev.CompletesWhen
			}
		}
		latest[it.ID] = it
	}
	out := make([]threadItem, 0, len(order))
	for _, id := range order {
		out = append(out, latest[id])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].FiledAt != out[j].FiledAt {
			return out[i].FiledAt < out[j].FiledAt
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// evaluateThreadConditions closes any open item whose condition now holds.
// Read-time, no sweep; each closure is a durable, attributed row.
func evaluateThreadConditions(items []threadItem, repoRoot string, now time.Time) []threadItem {
	out := make([]threadItem, 0, len(items))
	for _, it := range items {
		if it.ClosedAt == "" && it.CompletesWhen != nil && threadConditionHolds(*it.CompletesWhen, repoRoot) {
			it.ClosedAt = now.UTC().Format(time.RFC3339)
			it.ClosedBy = "condition"
			it.ClosedWhy = describeCondition(*it.CompletesWhen) + " now holds"
			_ = appendThreadItem(threadItem{
				SchemaVersion: threadSchemaVersion, ID: it.ID,
				ClosedAt: it.ClosedAt, ClosedBy: it.ClosedBy, ClosedWhy: it.ClosedWhy,
			})
		}
		out = append(out, it)
	}
	return out
}

func threadConditionHolds(c threadCondition, repoRoot string) bool {
	base := c.Repo
	if base == "" {
		base = repoRoot
	}
	switch c.Kind {
	case "file_exists":
		if c.Path == "" {
			return false
		}
		_, err := os.Stat(filepath.Join(base, c.Path))
		return err == nil
	case "repo_clean":
		if c.Repo == "" {
			return false
		}
		out, err := exec.Command("git", "-C", c.Repo, "status", "--porcelain").Output()
		return err == nil && strings.TrimSpace(string(out)) == ""
	case "plan_row_checked":
		if c.Path == "" || c.Row == "" {
			return false
		}
		data, err := os.ReadFile(filepath.Join(base, c.Path))
		if err != nil {
			return false
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "{id: "+c.Row+",") || strings.Contains(line, "{id: "+c.Row+"}") {
				return strings.HasPrefix(strings.TrimSpace(line), "- [x]")
			}
		}
		return false
	}
	return false
}

func describeCondition(c threadCondition) string {
	switch c.Kind {
	case "file_exists":
		return c.Path + " exists"
	case "plan_row_checked":
		return c.Path + "#" + c.Row + " is checked"
	case "repo_clean":
		return "the repository tree is clean again"
	}
	return c.Kind
}

// ─── the verbs ───────────────────────────────────────────────────────────

func cmdThread(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: fathom thread brief|note|point|list|close …")
		return 1
	}
	switch args[0] {
	case "brief":
		return cmdThreadBrief(args[1:], stdout, stderr)
	case "note":
		return cmdThreadNote(args[1:], stdout, stderr)
	case "point":
		return cmdThreadPoint(args[1:], stdout, stderr)
	case "list":
		return cmdThreadList(args[1:], stdout, stderr)
	case "close":
		return cmdThreadClose(args[1:], stdout, stderr)
	case "promote":
		return cmdThreadPromote(args[1:], stdout, stderr)
	case "handoff":
		return cmdThreadHandoff(args[1:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "unknown thread verb %q (brief|note|point|list|close|promote|handoff)\n", args[0])
	return 1
}

func newThreadID(now time.Time, text string) string {
	h := sha256.Sum256([]byte("thread\x00" + now.UTC().Format(time.RFC3339Nano) + "\x00" + text))
	return "th-" + hex.EncodeToString(h[:])[:8]
}

func cmdThreadNote(args []string, stdout, stderr io.Writer) int {
	// `thread note "text" --flag v` is the natural form, and Go's
	// flag package stops at the first positional — so the leading argument
	// is lifted before parsing (the rate-verb lesson, applied here before
	// it bites).
	var lead string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		lead, args = args[0], args[1:]
	}
	fs := newFlagSet("thread note", stderr)
	thread := fs.String("thread", "general", "thread name")
	kind := fs.String("kind", "handoff-note", "item kind: "+strings.Join(threadKinds, "|"))
	completes := fs.String("completes-when", "", `self-closing condition: "file_exists:<path>" or "plan_row_checked:<plan>#<ROW-ID>"`)
	repo := fs.String("repo", ".", "repository this item belongs to")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if lead == "" && fs.NArg() == 1 {
		lead = fs.Arg(0)
	}
	if strings.TrimSpace(lead) == "" {
		fmt.Fprintln(stderr, "usage: fathom thread note \"text\" [--thread T] [--kind K] [--completes-when …]")
		return 1
	}
	if !sliceContains(threadKinds, *kind) {
		fmt.Fprintf(stderr, "error: unknown kind %q (want %s)\n", *kind, strings.Join(threadKinds, "|"))
		return 1
	}
	cond, cerr := parseCondition(*completes, *repo)
	if cerr != nil {
		fmt.Fprintf(stderr, "error: %v\n", cerr)
		return 1
	}
	now := time.Now()
	root, _ := filepath.Abs(*repo)
	it := threadItem{
		SchemaVersion: threadSchemaVersion,
		ID:            newThreadID(now, lead),
		Thread:        *thread,
		Kind:          *kind,
		Text:          strings.TrimSpace(lead),
		Pointers:      map[string]string{"repo": strings.ToLower(filepath.Base(root))},
		FiledAt:       now.UTC().Format(time.RFC3339),
		FiledBy:       threadFiler(),
		CompletesWhen: cond,
	}
	if sid := currentSessionID(); sid != "" {
		it.Pointers["session_id"] = sid
	}
	if err := appendThreadItem(it); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "filed %s on thread %q (%s)\n", it.ID, it.Thread, it.Kind)
	return 0
}

func cmdThreadPoint(args []string, stdout, stderr io.Writer) int {
	// `thread point "text" --flag v` is the natural form, and Go's
	// flag package stops at the first positional — so the leading argument
	// is lifted before parsing (the rate-verb lesson, applied here before
	// it bites).
	var lead string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		lead, args = args[0], args[1:]
	}
	fs := newFlagSet("thread point", stderr)
	thread := fs.String("thread", "general", "thread name")
	session := fs.String("session", "", "session id the detail lives in (default: this session)")
	repo := fs.String("repo", ".", "repository")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if lead == "" && fs.NArg() == 1 {
		lead = fs.Arg(0)
	}
	if lead == "" {
		fmt.Fprintln(stderr, "usage: fathom thread point \"what happened\" [--session S]")
		return 1
	}
	sid := *session
	if sid == "" {
		sid = currentSessionID()
	}
	if sid == "" {
		// A pointer with nothing to point at is prose pretending to be an
		// address — same refusal shape as an unjoinable rating.
		fmt.Fprintln(stderr, "error: no session id to point at (the harness did not export one and --session was not given)")
		return 1
	}
	now := time.Now()
	root, _ := filepath.Abs(*repo)
	it := threadItem{
		SchemaVersion: threadSchemaVersion,
		ID:            newThreadID(now, lead),
		Thread:        *thread,
		Kind:          "side-task",
		Text:          strings.TrimSpace(lead),
		Pointers:      map[string]string{"session_id": sid, "repo": strings.ToLower(filepath.Base(root))},
		FiledAt:       now.UTC().Format(time.RFC3339),
		FiledBy:       threadFiler(),
	}
	if err := appendThreadItem(it); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "pointer %s filed: the detail stays in session %s — nothing was copied\n", it.ID, sid[:min(8, len(sid))])
	return 0
}

func cmdThreadList(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("thread list", stderr)
	thread := fs.String("thread", "", "only this thread")
	all := fs.Bool("all", false, "include closed and dormant items")
	repo := fs.String("repo", ".", "repository (for condition checks)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	root, _ := filepath.Abs(*repo)
	items := evaluateThreadConditions(loadThreadItems(threadsPath()), root, time.Now())
	if len(items) == 0 {
		fmt.Fprintln(stdout, "no thread items — `fathom thread note` files one; hooks deposit mechanically at /clear and compaction")
		return 0
	}
	shown := 0
	for _, it := range items {
		if *thread != "" && it.Thread != *thread {
			continue
		}
		if it.ClosedAt != "" && !*all {
			continue
		}
		status := "open"
		if it.ClosedAt != "" {
			status = "closed(" + it.ClosedBy + ")"
		} else if threadDormant(it, time.Now()) {
			if !*all {
				continue
			}
			status = "dormant"
		}
		ptr := ""
		if sid := it.Pointers["session_id"]; sid != "" {
			ptr = "  ↪ session " + sid[:min(8, len(sid))]
		}
		fmt.Fprintf(stdout, "  %-10s %-9s [%s] %-16s %s%s\n", it.ID, status, it.Kind, it.Thread, it.Text, ptr)
		shown++
	}
	if shown == 0 {
		fmt.Fprintln(stdout, "nothing open — closed and dormant items with --all")
	}
	return 0
}

func cmdThreadClose(args []string, stdout, stderr io.Writer) int {
	// `thread close "text" --flag v` is the natural form, and Go's
	// flag package stops at the first positional — so the leading argument
	// is lifted before parsing (the rate-verb lesson, applied here before
	// it bites).
	var lead string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		lead, args = args[0], args[1:]
	}
	fs := newFlagSet("thread close", stderr)
	why := fs.String("why", "", "one line on the outcome")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if lead == "" && fs.NArg() == 1 {
		lead = fs.Arg(0)
	}
	if lead == "" {
		fmt.Fprintln(stderr, "usage: fathom thread close <id> [--why …]")
		return 1
	}
	id := lead
	items := loadThreadItems(threadsPath())
	var found bool
	for _, it := range items {
		if it.ID == id {
			found = true
			if it.ClosedAt != "" {
				fmt.Fprintf(stdout, "%s is already closed (%s)\n", id, it.ClosedBy)
				return 0
			}
		}
	}
	if !found {
		fmt.Fprintf(stderr, "error: no item %q (fathom thread list --all shows every id)\n", id)
		return 1
	}
	err := appendThreadItem(threadItem{
		SchemaVersion: threadSchemaVersion, ID: id,
		ClosedAt: time.Now().UTC().Format(time.RFC3339), ClosedBy: threadFiler(), ClosedWhy: *why,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "closed %s\n", id)
	return 0
}

// threadDormant: untouched for 14 days. Dormant items leave the brief but
// never vanish — visible via list --all (law 3: aging is visible, not
// silent deletion).
func threadDormant(it threadItem, now time.Time) bool {
	t, err := time.Parse(time.RFC3339, it.FiledAt)
	if err != nil {
		return false
	}
	return now.Sub(t) > 14*24*time.Hour
}

func threadFiler() string {
	if os.Getenv("CLAUDECODE") != "" || os.Getenv("CLAUDE_CODE_SESSION_ID") != "" {
		return "agent"
	}
	return "human"
}

func parseCondition(spec, repo string) (*threadCondition, error) {
	if spec == "" {
		return nil, nil
	}
	root, _ := filepath.Abs(repo)
	kind, rest, ok := strings.Cut(spec, ":")
	if !ok {
		return nil, fmt.Errorf(`completes-when %q: want "file_exists:<path>" or "plan_row_checked:<plan>#<ROW-ID>"`, spec)
	}
	switch kind {
	case "file_exists":
		return &threadCondition{Kind: kind, Repo: root, Path: rest}, nil
	case "plan_row_checked":
		plan, row, ok := strings.Cut(rest, "#")
		if !ok || row == "" {
			return nil, fmt.Errorf(`plan_row_checked wants "<plan-path>#<ROW-ID>", got %q`, rest)
		}
		return &threadCondition{Kind: kind, Repo: root, Path: plan, Row: row}, nil
	}
	return nil, fmt.Errorf("unknown condition kind %q", kind)
}

func sliceContains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// ─── the brief ───────────────────────────────────────────────────────────

// cmdThreadBrief renders the ≤budget continuity brief, or NOTHING on
// resume/compact — their context survives, and injecting there is the
// schema-tax mistake in a new costume.
func cmdThreadBrief(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("thread brief", stderr)
	budget := fs.Int("budget-bytes", threadBriefBudgetBytes, "hard byte budget for the brief")
	source := fs.String("source", "startup", "session start source: startup|clear|resume|compact|fork")
	repo := fs.String("repo", ".", "repository (for derived in-flight state)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *source == "resume" || *source == "compact" {
		return 0 // context survives; say nothing
	}
	root, _ := filepath.Abs(*repo)
	out := renderThreadBrief(loadThreadItems(threadsPath()), root, *budget, time.Now())
	fmt.Fprint(stdout, out)
	return 0
}

// renderThreadBrief is the pure, budget-enforced core (TH-3's test surface).
//
// Composition order after the first field report (2026-08-13, the
// idp-and-warrants session): the reader said, verbatim, that "the durable
// spec plane — git log, the plan files, and fathom verify printing which
// rows are claimed vs proven — is doing the real continuity work", while
// the v1 brief led with other repos' dirty-file telemetry. So the brief
// now LEADS with the current repo's derived spec state (the thing that
// demonstrably restored context), then this repo's deposited items, then
// one collapsed line for everything foreign — and when no substantive
// handoff exists, it says how to make the next one better instead of
// presenting telemetry as a handoff.
func renderThreadBrief(items []threadItem, repoRoot string, budgetBytes int, now time.Time) string {
	items = evaluateThreadConditions(items, repoRoot, now)
	repo := strings.ToLower(filepath.Base(repoRoot))

	kindRank := map[string]int{
		"handoff-note": 0, "decision-pending": 1, "in-flight": 2,
		"spec-unbuilt": 3, "side-task": 4, "session-end": 5, "pre-compact": 6,
	}
	mechanical := func(k string) bool { return k == "session-end" || k == "pre-compact" }

	var local, foreign []threadItem
	foreignMech := 0
	for _, it := range items {
		if it.ClosedAt != "" || threadDormant(it, now) {
			continue
		}
		switch {
		case it.Pointers["repo"] == repo || it.Thread == repo:
			local = append(local, it)
		case mechanical(it.Kind):
			foreignMech++ // another repo's telemetry is ONE line, never a list
		default:
			foreign = append(foreign, it)
		}
	}
	rank := func(xs []threadItem) {
		sort.SliceStable(xs, func(i, j int) bool {
			if kindRank[xs[i].Kind] != kindRank[xs[j].Kind] {
				return kindRank[xs[i].Kind] < kindRank[xs[j].Kind]
			}
			if xs[i].FiledAt != xs[j].FiledAt {
				return xs[i].FiledAt > xs[j].FiledAt
			}
			return xs[i].ID < xs[j].ID
		})
	}
	rank(local)
	rank(foreign)

	var lines []string
	add := func(it threadItem) {
		ptr := ""
		if sid := it.Pointers["session_id"]; sid != "" {
			ptr = " [session " + sid[:min(8, len(sid))] + "]"
		}
		lines = append(lines, fmt.Sprintf("  • (%s/%s) %s%s", it.Thread, it.Kind, it.Text, ptr))
	}

	lines = append(lines, "🧵 fathom continuity — the work, not the session:")
	// 1) The durable spec plane first: what demonstrably restores context.
	if ps := derivedPlansSummary(repoRoot); ps != "" {
		lines = append(lines, "  • (plans) "+ps)
	}
	if wip := derivedInFlight(repoRoot); wip != "" {
		lines = append(lines, "  • (derived) "+wip)
	}
	// 2) This repo's deposited items, substantive kinds first.
	substantive := false
	for _, it := range local {
		if !mechanical(it.Kind) {
			substantive = true
		}
		add(it)
	}
	// 3) Foreign substantive items ride along (threads span repos)…
	for _, it := range foreign {
		add(it)
	}
	// …but foreign telemetry collapses to one line, total.
	if foreignMech > 0 {
		lines = append(lines, fmt.Sprintf("  • %d other repo(s) left sessions with work in flight: fathom thread list --all", foreignMech))
	}
	if len(lines) == 1 {
		return "" // nothing to say costs nothing — the budget's best case
	}
	// 4) A thin brief teaches instead of pretending: telemetry is not a
	// handoff, and the next session deserves better than this one got.
	if !substantive {
		lines = append(lines, "  no handoff notes were filed for this repo — before /clear, leave one: fathom thread note \"where the work stands\"")
	}
	lines = append(lines, "  open plan work: fathom plans next --service <svc> · detail: fathom thread list")

	// The budget is a HARD bound: drop whole bullets from the bottom until
	// it fits, and say how many were dropped (law 3 — never a silent cut).
	fits := func(ls []string, dropped int) string {
		out := strings.Join(ls, "\n") + "\n"
		if dropped > 0 {
			out = strings.Join(ls, "\n") + fmt.Sprintf("\n  … and %d more: fathom thread list\n", dropped)
		}
		return out
	}
	dropped := 0
	for len(lines) > 1 {
		if len(fits(lines, dropped)) <= budgetBytes {
			return fits(lines, dropped)
		}
		cut := len(lines) - 2
		if cut <= 0 {
			break
		}
		lines = append(lines[:cut], lines[len(lines)-1])
		dropped++
	}
	out := fits(lines, dropped)
	if len(out) > budgetBytes {
		return "" // pathological budget: silence beats an over-budget brief
	}
	return out
}

// derivedPlansSummary reads docs/plans/*.md locally: how much of this
// repo's spec plane is claimed vs still open. It cannot know PROVEN (that
// is the gate's, and needs the daemon) — and says so rather than implying
// checked means proven.
func derivedPlansSummary(root string) string {
	entries, err := os.ReadDir(filepath.Join(root, "docs", "plans"))
	if err != nil {
		return ""
	}
	plans, rows, checked := 0, 0, 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "README.md" {
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(root, "docs", "plans", e.Name()))
		if rerr != nil {
			continue
		}
		counted := false
		for _, line := range strings.Split(string(data), "\n") {
			t := strings.TrimSpace(line)
			if !strings.Contains(t, "{id: ") {
				continue
			}
			switch {
			case strings.HasPrefix(t, "- [x]"):
				rows++
				checked++
				counted = true
			case strings.HasPrefix(t, "- [ ]"):
				rows++
				counted = true
			}
		}
		if counted {
			plans++
		}
	}
	if rows == 0 {
		return ""
	}
	return fmt.Sprintf("%d plan(s), %d gated row(s): %d checked, %d open — `fathom verify` distinguishes claimed from PROVEN",
		plans, rows, checked, rows-checked)
}

// derivedInFlight reads git for the mechanical in-flight facts: stashes,
// a non-default WIP branch, dirty files. One line, cheap, derivable — the
// store never duplicates what git already knows.
func derivedInFlight(root string) string {
	run := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	var parts []string
	if n := run("stash", "list"); n != "" {
		parts = append(parts, fmt.Sprintf("%d stash(es)", len(strings.Split(n, "\n"))))
	}
	if b := run("rev-parse", "--abbrev-ref", "HEAD"); b != "" && b != "main" && b != "master" {
		parts = append(parts, "on branch "+b)
	}
	if s := run("status", "--porcelain"); s != "" {
		parts = append(parts, fmt.Sprintf("%d dirty file(s)", len(strings.Split(s, "\n"))))
	}
	if len(parts) == 0 {
		return ""
	}
	return "git in-flight: " + strings.Join(parts, ", ")
}
