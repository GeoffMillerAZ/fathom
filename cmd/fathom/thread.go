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
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/verify"
	"github.com/geoffmilleraz/fathom/internal/worktree"
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
	// Commit is the repo HEAD when the item was filed — metadata, never a
	// citation (EP-15). It answers "what was the tree when this was
	// written?", which is what makes a months-old handoff note readable;
	// CC-15 still requires the CITATION to be in the text.
	Commit string `json:"commit,omitempty"`

	// CompletesWhen names a CHECKABLE condition; when it holds, the item
	// closes itself with closed_by: "condition" — the anchored-memory
	// self-expiry philosophy, keyed on artifact existence.
	CompletesWhen *threadCondition `json:"completes_when,omitempty"`

	ClosedAt  string `json:"closed_at,omitempty"`
	ClosedBy  string `json:"closed_by,omitempty"`
	ClosedWhy string `json:"closed_why,omitempty"`
}

type threadCondition struct {
	Kind      string `json:"kind"` // file_exists | plan_row_checked | repo_clean | session_ended
	Repo      string `json:"repo,omitempty"`
	Path      string `json:"path,omitempty"`       // file_exists: the file; plan_row_checked: the plan
	Row       string `json:"row,omitempty"`        // plan_row_checked: the {id: …} to look for
	SessionID string `json:"session_id,omitempty"` // session_ended: the session to watch for
}

var threadKinds = []string{
	"decision-pending", "spec-unbuilt", "in-flight",
	"side-task", "handoff-note", "session-end", "pre-compact",
}

func threadsPath() string {
	return fathomhome.PerFile("FATHOM_THREADS", "threads.jsonl")
}

func appendThreadItem(it threadItem) error {
	// Stamp HEAD once, here, so every write path (note, point, the
	// lifecycle hooks, promote) carries it without each remembering to —
	// the MI-4 lesson: a field every caller must copy by hand is a field
	// that goes missing.
	if it.Commit == "" {
		if repo := it.Pointers["repo_root"]; repo != "" {
			it.Commit = gitHeadShort(repo)
		} else if cwd, err := os.Getwd(); err == nil {
			it.Commit = gitHeadShort(cwd)
		}
	}
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
// FiledAt then ID (house determinism). Exploration maps' own rows
// (mapstore.go) share the log but are not thread items: no brief, list or
// handoff ever serves them.
func loadThreadItems(path string) []threadItem {
	all := loadAllThreadRows(path)
	out := all[:0:0]
	for _, it := range all {
		if !isMapItem(it) {
			out = append(out, it)
		}
	}
	return out
}

// loadAllThreadRows is loadThreadItems' fold over every row, the maps' own
// included.
func loadAllThreadRows(path string) []threadItem {
	data, err := readThreadsFile(path)
	if err != nil {
		return nil
	}
	return foldThreadRows(parseThreadRows(data))
}

// readThreadsFile is the single-read seam for a thread-log snapshot.
var readThreadsFile = os.ReadFile

func foldThreadRows(rows []threadItem) []threadItem {
	latest := map[string]threadItem{}
	var order []string
	for _, it := range rows {
		if hookDeadlineSpent() {
			return nil
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
		if it.ClosedAt == "" && it.CompletesWhen != nil && threadConditionHolds(*it.CompletesWhen, repoRoot, items) {
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

func threadConditionHolds(c threadCondition, repoRoot string, items []threadItem) bool {
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
		//fathom:tenancy-allow exec-noctx one-shot CLI verb with no request context; the git/child call ends with the process
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
		// The plan grammar's own parser, so a wrapped row (its id on a
		// continuation line, G2-5) is found by id like any other.
		for _, t := range verify.ParseMarkdownPlan("", data).Tasks {
			if t.ID == c.Row {
				return t.Claimed
			}
		}
		return false
	case "session_ended":
		// IT-9: a pre-compact pointer's session survives until a
		// session-end note for that SAME session_id lands in the log —
		// never any session-end note (that would close a compaction
		// pointer from an entirely different, still-open session).
		if c.SessionID == "" {
			return false
		}
		for _, it := range items {
			if it.Kind == "session-end" && it.Pointers["session_id"] == c.SessionID {
				return true
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
	case "session_ended":
		return "a session-end note for the same session"
	}
	return c.Kind
}

// ─── repo scope (TH-6) ───────────────────────────────────────────────────

// threadScope is the repo a verb runs FROM: its identity (RC-31,
// docs/vision/28-receipts.md F5) every NEW deposit is tagged with, the
// legacy basename kept only as the read-side fallback for items filed
// before RC-31, plus the indexed service id when the daemon resolves one
// — the thread a note files on by default. On 2026-08-23 the open set
// held 30 items, ~25 of them other repos' handoffs: the tag existed, the
// default was the defect; F5 then found the tag ITSELF collapsing two
// unrelated worktrees that happened to share a directory basename.
type threadScope struct {
	identity string // repoIdentity(root) — origin URL, else absolute path
	repo     string // lower-cased basename of the repo root (legacy tag)
	service  string // resolved service id, "" when nothing resolves
}

// threadScopeFor resolves the scope for root: identity and basename
// always; the service via the same daemon-backed identity every hook uses
// (candidatesWithDaemon's cached round trip), fail-open — a down daemon
// degrades to basename-only scoping, never to an error.
func threadScopeFor(root string) threadScope {
	sc := threadScope{identity: repoIdentity(root), repo: strings.ToLower(filepath.Base(root))}
	hc := &http.Client{Timeout: 1500 * time.Millisecond}
	if cand, ok, _ := resolveIndexedCandidateCached(hc, defaultFathomURL(), root); ok {
		sc.service = strings.ToLower(cand.name)
	}
	return sc
}

// repoIdentity (RC-31) is the identity a thread deposit is scoped by: the
// normalized `origin` remote URL when root has one, else root's own
// lowercased absolute path. Two worktrees of the SAME repo share an
// origin and so share continuity (a feature, not a leak); two UNRELATED
// repos that merely happen to share a directory basename — F5's
// contamination: "two tasks pinned to one commit shared a worktree
// basename and each other's thread notes" — no longer collide, because
// the basename is never this function's answer.
func repoIdentity(repoRoot string) string {
	if url := gitOriginURL(repoRoot); url != "" {
		return normalizeOriginURL(url)
	}
	abs, err := filepath.Abs(repoRoot)
	if err != nil {
		abs = repoRoot
	}
	return strings.ToLower(abs)
}

// gitOriginURL returns repoRoot's `origin` remote URL, "" when it has none
// (not a git repo, or no `origin` configured) — repoIdentity's file-path
// fallback then applies.
func gitOriginURL(repoRoot string) string {
	//fathom:tenancy-allow exec-noctx one-shot CLI verb with no request context; the git/child call ends with the process
	out, err := exec.Command("git", "-C", repoRoot, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// normalizeOriginURL lowercases url and strips its scheme, any embedded
// user@ auth, and a trailing ".git" or "/", so "https://github.com/x/y.git",
// "git@github.com:x/y.git" and "ssh://git@github.com/x/y/" all normalize
// to the identical "github.com/x/y" identity.
func normalizeOriginURL(url string) string {
	u := strings.ToLower(strings.TrimSpace(url))
	if remote, local := worktree.NormalizeOrigin(u); !local {
		return remote
	}
	u = strings.TrimPrefix(u, "file://")
	return strings.TrimSuffix(strings.TrimSuffix(u, "/"), ".git")
}

// defaultThread is where a note lands when --thread is not given: the
// repo's own service id, "general" only when no service resolves.
func (s threadScope) defaultThread() string {
	if s.service != "" {
		return s.service
	}
	return "general"
}

// owns reports whether an item belongs to this repo's continuity. An item
// carrying RC-31's "repo_identity" tag (every deposit filed from here on)
// is matched EXACTLY against it — the fix for F5: two unrelated repos
// sharing a directory basename no longer collide. An older item with no
// identity tag falls back to the pre-RC-31 rule unchanged (basename or
// service match on the tag or the thread name); an UNTAGGED item (no repo
// pointer at all — older rows, hand-seeded rows) is never hidden: hiding
// what cannot be attributed would be a silent cut (law 3).
func (s threadScope) owns(it threadItem) bool {
	if id := it.Pointers["repo_identity"]; id != "" {
		return id == s.identity
	}
	tag := it.Pointers["repo"]
	if tag == "" {
		return true
	}
	if tag == s.repo || it.Thread == s.repo {
		return true
	}
	return s.service != "" && (tag == s.service || it.Thread == s.service)
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
	thread := fs.String("thread", "", `thread name (default: this repo's indexed service id, else "general")`)
	kind := fs.String("kind", "handoff-note", "item kind: "+strings.Join(threadKinds, "|"))
	completes := fs.String("completes-when", "", `self-closing condition: "file_exists:<path>" or "plan_row_checked:<plan>#<ROW-ID>"`)
	repo := fs.String("repo", ".", "repository this item belongs to")
	// Exploration maps (XM-1, XM-14): with --map the note is an anchored map
	// entry, its text the why line.
	mapID := fs.String("map", "", "file an exploration-map entry on this map (created on first use) instead of a note; needs --anchor")
	anchor := fs.String("anchor", "", "with --map: the entry's anchor, a symbol key or path#Lx-Ly (hashed now, re-resolved at every read)")
	why := fs.String("why", "", "with --map: the one-line why (default: the note text)")
	agent := fs.String("agent", "", "with --map: the appending sub-agent's id, recorded as the entry's provenance")
	storePath := fs.String("store", "", "with --map: resolve a symbol anchor from this store (default: the daemon)")
	url := fs.String("url", defaultFathomURL(), "with --map: fathom daemon URL for a symbol anchor")
	service := fs.String("service", "", "with --map: the service a symbol anchor resolves in (default: the one the daemon names for this repository)")
	mapRefs := fs.String("map-ref", "", "reference these exploration maps (space- or comma-separated ids) from the note: the session brief re-surfaces them when work on the thread resumes")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if lead == "" && fs.NArg() == 1 {
		lead = fs.Arg(0)
	}
	if *mapID != "" && *mapRefs != "" {
		fmt.Fprintln(stderr, "error: --map files an entry on a map and --map-ref references maps from a note; use one")
		return 1
	}
	if *mapID != "" {
		return cmdThreadNoteMap(mapEntryFlags{
			mapID: *mapID, anchor: *anchor, why: *why, lead: lead, agent: *agent,
			thread: *thread, completes: *completes, repo: *repo, store: *storePath, url: *url, service: *service,
		}, stdout, stderr)
	}
	if strings.TrimSpace(lead) == "" {
		fmt.Fprintln(stderr, "usage: fathom thread note \"text\" [--thread T] [--kind K] [--completes-when …] [--map-ref IDS]")
		fmt.Fprintln(stderr, "       fathom thread note --map ID --anchor <symbol|path#Lx-Ly> --why \"…\" [--agent ID] [--completes-when …]")
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
	name := *thread
	if name == "" {
		name = threadScopeFor(root).defaultThread()
	}
	it := threadItem{
		SchemaVersion: threadSchemaVersion,
		ID:            newThreadID(now, lead),
		Thread:        name,
		Kind:          *kind,
		Text:          strings.TrimSpace(lead),
		Pointers:      map[string]string{"repo": strings.ToLower(filepath.Base(root)), "repo_identity": repoIdentity(root)},
		FiledAt:       now.UTC().Format(time.RFC3339),
		FiledBy:       threadFiler(),
		CompletesWhen: cond,
	}
	if sid := currentSessionID(); sid != "" {
		it.Pointers["session_id"] = sid
	}
	if refs := strings.FieldsFunc(*mapRefs, func(r rune) bool { return r == ',' || r == ' ' }); len(refs) > 0 {
		set := loadMapSet(root, now)
		var uniq []string
		for _, id := range refs {
			if _, known := set.maps[id]; !known {
				fmt.Fprintf(stderr, "error: --map-ref %q: no such map (`fathom map list` names them)\n", id)
				return 1
			}
			if !sliceContains(uniq, id) {
				uniq = append(uniq, id)
			}
		}
		it.Pointers[mapsPointerKey] = strings.Join(uniq, " ")
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
	thread := fs.String("thread", "", `thread name (default: this repo's indexed service id, else "general")`)
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
	name := *thread
	if name == "" {
		name = threadScopeFor(root).defaultThread()
	}
	it := threadItem{
		SchemaVersion: threadSchemaVersion,
		ID:            newThreadID(now, lead),
		Thread:        name,
		Kind:          "side-task",
		Text:          strings.TrimSpace(lead),
		Pointers:      map[string]string{"session_id": sid, "repo": strings.ToLower(filepath.Base(root)), "repo_identity": repoIdentity(root)},
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
	thread := fs.String("thread", "", "only this thread, wherever it was filed")
	all := fs.Bool("all", false, "everything: closed and dormant items, and every repo's")
	allRepos := fs.Bool("all-repos", false, "open items from every repo, not only this one")
	repo := fs.String("repo", ".", "repository (scope, and condition checks)")
	jsonOut := fs.Bool("json", false, "emit the listed items as JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	root, _ := filepath.Abs(*repo)
	items := evaluateThreadConditions(loadThreadItems(threadsPath()), root, time.Now())
	listJSON := threadListJSON{Kind: "list", Items: []threadListItemJSON{}} // RP-16: thread_json.go
	if len(items) == 0 {
		if *jsonOut {
			return jsonExit(renderThreadJSON(stdout, listJSON), stderr)
		}
		fmt.Fprintln(stdout, "no thread items — `fathom thread note` files one; hooks deposit mechanically at /clear and compaction")
		return 0
	}
	// TH-6: this repo first. A named --thread is asked for by name, so it
	// crosses repos (threads span repos, law 13); --all-repos and --all
	// widen the default. What is hidden is COUNTED, never silently gone.
	scope := threadScopeFor(root)
	crossRepo := *all || *allRepos || *thread != ""
	shown, hidden := 0, 0
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
		where := ""
		if !scope.owns(it) {
			if !crossRepo {
				hidden++
				continue
			}
			where = "  @" + it.Pointers["repo"]
		}
		ptr := ""
		if sid := it.Pointers["session_id"]; sid != "" {
			ptr = "  ↪ session " + sid[:min(8, len(sid))]
		}
		if *jsonOut {
			listJSON.Items = append(listJSON.Items, threadListItemOf(it, status))
		} else {
			fmt.Fprintf(stdout, "  %-10s %-9s [%s] %-16s %s%s%s\n", it.ID, status, it.Kind, it.Thread, it.Text, where, ptr)
		}
		shown++
	}
	if *jsonOut {
		listJSON.HiddenOtherRepos = hidden
		return jsonExit(renderThreadJSON(stdout, listJSON), stderr)
	}
	if shown == 0 {
		fmt.Fprintln(stdout, "nothing open here — closed and dormant items with --all")
	}
	if hidden > 0 {
		fmt.Fprintf(stdout, "  %d open item(s) in other repos — fathom thread list --all-repos\n", hidden)
	}
	return 0
}

// jsonExit maps an encoder error to an exit code, naming it on stderr.
func jsonExit(err error, stderr io.Writer) int {
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
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

// threadFiler labels who filed a thread item: "agent" when a harness's env
// contract (hookharness.go) says a harness is running this process.
func threadFiler() string {
	if harnessPresent() {
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
	repo := fs.String("repo", ".", "repository (scope, and derived in-flight state)")
	allRepos := fs.Bool("all-repos", false, "ride other repos' open items along instead of counting them")
	jsonOut := fs.Bool("json", false, "emit the brief as JSON: source, budget_bytes, bytes, text")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *source == "resume" || *source == "compact" {
		if *jsonOut { // RP-16: thread_json.go — an empty brief is still an envelope
			return jsonExit(renderThreadJSON(stdout, threadBriefJSON{Kind: "brief", Source: *source, BudgetBytes: *budget}), stderr)
		}
		return 0 // context survives; say nothing
	}
	root, _ := filepath.Abs(*repo)
	now := time.Now()
	items := loadThreadItems(threadsPath())
	scope := threadScopeFor(root)
	var mapLines []string
	if threadsHoldMaps() {
		items = evaluateThreadConditions(items, root, now)
		mapLines = briefMapLines(items, root, now)
	}
	out := renderThreadBriefScoped(items, scope, root, *budget, now, *allRepos, mapLines...)
	if *jsonOut {
		return jsonExit(renderThreadJSON(stdout, threadBriefJSON{Kind: "brief", Source: *source, BudgetBytes: *budget, Bytes: len(out), Text: out}), stderr)
	}
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
//
// TH-6: scope is the repo's basename (the tag every deposit carries);
// other repos' substantive items are COUNTED on one line, not listed —
// renderThreadBriefScoped takes a daemon-resolved scope and can ride them
// along on request.
func renderThreadBrief(items []threadItem, repoRoot string, budgetBytes int, now time.Time, mapLines ...string) string {
	scope := threadScope{identity: repoIdentity(repoRoot), repo: strings.ToLower(filepath.Base(repoRoot))}
	return renderThreadBriefScoped(items, scope, repoRoot, budgetBytes, now, false, mapLines...)
}

// renderThreadBriefScoped is renderThreadBrief with an explicit scope and
// the --all-repos switch. Pure: no daemon, no clock but the one passed.
//
// mapLines (XM-12) are the exploration-map bullets the caller chose
// (briefMapLines): they follow this repo's deposited items and, like every
// bullet, are cut from the bottom when the budget is spent.
func renderThreadBriefScoped(items []threadItem, scope threadScope, repoRoot string, budgetBytes int, now time.Time, allRepos bool, mapLines ...string) string {
	items = evaluateThreadConditions(items, repoRoot, now)

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
		case scope.owns(it):
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
		ptr += mapRefsSuffix(it) // XM-13: the maps this note references
		lines = append(lines, fmt.Sprintf("  • (%s/%s) %s%s", it.Thread, it.Kind, it.Text, ptr))
		// RC-29 (docs/vision/28-receipts.md §4): a session-end note's own
		// pointers, resolved FRESH at brief-render time — never the text
		// that was true when the note was filed (ADR-028).
		if it.Kind == "session-end" {
			if disc := sessionEndDiscoveriesLine(it, repoRoot); disc != "" {
				lines = append(lines, "    "+disc)
			}
		}
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
	// 2b) XM-12: at most three exploration maps, never a full list.
	lines = append(lines, mapLines...)
	// 3) Other repos' substantive items are a COUNT by default (TH-6: they
	//    were burying this repo's continuity in every brief) — never a
	//    silent cut — and ride along only on --all-repos.
	if allRepos {
		for _, it := range foreign {
			add(it)
		}
	} else if len(foreign) > 0 {
		lines = append(lines, fmt.Sprintf("  • %d open item(s) in other repos — fathom thread list --all-repos", len(foreign)))
	}
	// …and foreign telemetry collapses to one line, total.
	if foreignMech > 0 {
		lines = append(lines, fmt.Sprintf("  • %d other repo(s) left sessions with work in flight: fathom thread list --all-repos", foreignMech))
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
		//fathom:tenancy-allow exec-noctx one-shot CLI verb with no request context; the git/child call ends with the process
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

// threadRunner runs `fathom thread` (registered below; EW-45).
type threadRunner struct{}

func (threadRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdThread(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "thread", Group: "govern", Order: 10, Run: threadRunner{},
		Summary: "Continuity threads: brief | note | point | list | close | promote | handoff — the work, not the session (vision 21)"})
}
