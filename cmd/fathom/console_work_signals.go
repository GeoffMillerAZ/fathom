package main

// console_work_signals.go — the readers /work adds to its proof join
// (docs/design/work-intelligence.md §3): the CI summary files a
// repository names (WI-2w) and the friction map's activity record (WI-6w)
// — search and lookup calls from the MCP call log, the edits that end a
// search run from the rule-outcome log, the discovery pointers earlier
// sessions established, claimed-unproven anchors from the verify ledger
// and the plans' contracts, and the reverts WI-4w read from git. Counts
// and tool events only, never transcript prose (GL-X4, ADR-041 decision
// 8). Every read is bounded (a tail), every file it could not read or
// parse is named on the page, and a figure it did not read is an absence
// with its reason — never a zero (I38).

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// workCIMaxBytes bounds one CI summary file's read.
const workCIMaxBytes = 8 << 20

// workRepoConfig reads the repository's own .fathom/config.toml (its
// `[zones]` and `[work]` tables). An absent file is an empty config.
func workRepoConfig(root string) (config.Config, error) {
	if root == "" {
		return config.Config{}, nil
	}
	loaded, err := config.Load("", workConfigPath(root))
	if err != nil {
		return config.Config{}, err
	}
	return loaded.Config, nil
}

func workConfigPath(root string) string { return filepath.Join(root, ".fathom", "config.toml") }

// workCIPath is a named CI summary's path: absolute as written, else
// relative to the repository root.
func workCIPath(root, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, p)
}

// workCIRuns reads each CI summary file cfg names (console.ParseCISummary)
// and binds every run to the proving commit it built, in the spelling the
// proof history used (a ledger abbreviates HEAD; a workflow run carries
// the full head_sha). It returns the runs and one line per file saying
// what was read or why not. A run bound to no proving commit is kept: U1
// then names the changes it could not bind.
func workCIRuns(root string, sums []config.CISummary, cfgErr error, changes []console.ProvenChange) ([]console.CIRun, []string) {
	if cfgErr != nil {
		return nil, []string{"CI minutes: .fathom/config.toml could not be read: " + cfgErr.Error()}
	}
	if len(sums) == 0 {
		return nil, []string{"CI minutes: no CI summary file named ([[work.ci_summaries]] in .fathom/config.toml)"}
	}
	spellings := map[string]bool{}
	for _, c := range changes {
		if c.Commit != "" {
			spellings[c.Commit] = true
		}
	}
	var runs []console.CIRun
	var read []string
	for _, s := range sums {
		path := workCIPath(root, s.Path)
		fi, err := os.Stat(path)
		switch {
		case err != nil:
			read = append(read, fmt.Sprintf("%s: not read: %v", s.Path, err))
			continue
		case fi.Size() > workCIMaxBytes:
			read = append(read, fmt.Sprintf("%s: not read: %d bytes, over the %d MiB bound", s.Path, fi.Size(), workCIMaxBytes>>20))
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			read = append(read, fmt.Sprintf("%s: not read: %v", s.Path, err))
			continue
		}
		parsed, err := console.ParseCISummary(s.Path, data, s.Commit)
		if err != nil {
			read = append(read, fmt.Sprintf("%s: not parsed: %v", s.Path, err))
			continue
		}
		minutes, bound := 0.0, 0
		var commits []string
		for _, r := range parsed {
			minutes += r.Minutes
			commits = append(commits, shortCommit(r.Commit))
			matched := false
			for sp := range spellings {
				if strings.HasPrefix(sp, r.Commit) || strings.HasPrefix(r.Commit, sp) {
					rr := r
					rr.Commit = sp
					runs = append(runs, rr)
					matched = true
				}
			}
			if matched {
				bound++
			} else {
				runs = append(runs, r)
			}
		}
		read = append(read, fmt.Sprintf("%s: %s, %.1f CI minutes, built %s (%d bound to a proving commit)",
			s.Path, plural(len(parsed), "run"), minutes, strings.Join(commits, ", "), bound))
	}
	return runs, read
}

// workCIWatermarks is the memo key part for the files cfg names, so a
// changed summary is read again.
func workCIWatermarks(root string, cfg config.Config) string {
	var b strings.Builder
	for _, s := range cfg.Work.CISummaries {
		b.WriteString(s.Path + "@" + fileWatermark(workCIPath(root, s.Path)) + ";")
	}
	return b.String()
}

// ─── the friction map's activity record (WI-6w) ─────────────────────────────

// Bounds on the friction reads (tenancy: every read a tail).
const (
	workCallTailBytes       = 8 << 20 // the call log's newest bytes
	workThreadTailBytes     = 4 << 20 // the thread log's newest bytes
	workPlanMaxBytes        = 4 << 20 // one plan file
	workMaxUnprovenReadings = 500     // claimed-unproven readings anchored
)

// workIndex is what /work reads from the index: a symbol pointer's
// defining file (claimed-unproven anchors). nil when the page has no
// store: each read it would have answered is named absent.
type workIndex interface {
	SymbolFile(ctx context.Context, service, symbol string) (string, bool)
}

// storeWorkIndex answers workIndex from the daemon's store.
type storeWorkIndex struct{ st store.Store }

// SymbolFile resolves a "package.Symbol" pointer (splitPointer's grammar)
// to the one file defining it: the name's observations, narrowed to the
// package directory when one is given. Several candidate files are not
// guessed between.
func (x storeWorkIndex) SymbolFile(ctx context.Context, service, symbol string) (string, bool) {
	pkg, name := splitPointer(symbol)
	obs, err := x.st.ObservationsForSymbol(ctx, service, name)
	if err != nil {
		return "", false
	}
	files := map[string]bool{}
	for _, o := range obs {
		if o.Symbol != name && !strings.HasSuffix(o.Symbol, "."+name) {
			continue
		}
		f, _, _ := strings.Cut(o.SourceRef, ":")
		if f == "" || (pkg != "" && path.Dir(f) != strings.TrimSuffix(pkg, "/")) {
			continue
		}
		files[f] = true
	}
	if len(files) != 1 {
		return "", false
	}
	for f := range files {
		return f, true
	}
	return "", false
}

// workAt renders a timestamp at a fixed width, so a call-log ts
// (nanoseconds), a rule-outcome ts (seconds) and a commit time order
// correctly as strings. One that does not parse is kept as written.
func workAt(ts string) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return ts
	}
	return t.UTC().Format("2006-01-02T15:04:05.000000000Z")
}

// workToolKind is a call's friction kind: a text search, a symbol or
// reference lookup, or "" for a call friction does not count.
func workToolKind(tool string) string {
	switch tool {
	case "search", "localize":
		return console.ActSearch
	case "find_symbol", "query_symbols", "references", "find_callers", "symbol_context", "outline", "read_source":
		return console.ActLookup
	}
	return ""
}

// workSourceExts are the extensions a discovery pointer names a file by
// even when the file is gone from the tree (a reverted or deleted file).
var workSourceExts = map[string]bool{
	"go": true, "ts": true, "tsx": true, "js": true, "jsx": true, "mjs": true, "cjs": true, "py": true, "rs": true,
	"java": true, "kt": true, "rb": true, "php": true, "cs": true, "c": true, "h": true, "cc": true, "cpp": true,
	"hpp": true, "swift": true, "dart": true, "scala": true, "ex": true, "exs": true, "sh": true, "sql": true,
	"proto": true, "cue": true, "md": true, "json": true, "yaml": true, "yml": true, "toml": true, "tf": true,
}

// workPointerIsFile tells a file pointer ("internal/pay/a.go") from a
// symbol pointer ("internal/pay.Settle"): the two share one grammar, so a
// pointer is a file when it exists as one under root or ends in a source
// extension. Answers are cached per call.
func workPointerIsFile(root string) func(string) bool {
	cache := map[string]bool{}
	return func(p string) bool {
		if v, ok := cache[p]; ok {
			return v
		}
		isFile := false
		if ext := strings.TrimPrefix(path.Ext(p), "."); workSourceExts[ext] {
			isFile = true
		} else if root != "" && !filepath.IsAbs(p) {
			if fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err == nil && fi.Mode().IsRegular() {
				isFile = true
			}
		}
		cache[p] = isFile
		return isFile
	}
}

// workCallEvents reads the call log's newest workCallTailBytes and keeps
// this service's search and lookup calls as activity events, each with
// the first symbol and file pointer its result carried. A call is
// sequenced into search runs by its caller's session; a row naming none
// is its own run of one (counted, never chained to an unrelated call).
// Every pointer a call returned becomes a discovery pointer established
// when its session's last call in the tail landed, so a session never
// "rediscovers" what it found itself.
func workCallEvents(logPath string, sr serviceRoot) ([]console.ActivityEvent, []console.DiscoveryPointer, string) {
	if logPath == "" {
		return nil, nil, "search and lookup events: no call log resolved for this page"
	}
	data, err := readFileTail(logPath, workCallTailBytes)
	if os.IsNotExist(err) {
		return nil, nil, "search and lookup events: no call log at " + logPath
	} else if err != nil {
		return nil, nil, "search and lookup events: " + logPath + " could not be read: " + err.Error()
	}
	isFile := workPointerIsFile(sr.Root)
	type established struct {
		key  string
		ptrs []string
	}
	var events []console.ActivityEvent
	var found []established
	lastAt := map[string]string{}
	sequenced, unsequenced := 0, 0
	for i, line := range strings.Split(string(data), "\n") {
		e, kind := decodeCallLogLine([]byte(line))
		if kind != callLogLineEntry || !strings.EqualFold(e.Service, sr.Service) {
			continue
		}
		var extra struct {
			Caller *struct {
				Session string `json:"session"`
			} `json:"caller"`
			Pointers []string `json:"pointers"`
		}
		_ = json.Unmarshal([]byte(line), &extra)
		at := workAt(e.TS)
		key := fmt.Sprintf("row:%d", i)
		if extra.Caller != nil && extra.Caller.Session != "" {
			key = "s:" + extra.Caller.Session
		}
		if at > lastAt[key] {
			lastAt[key] = at
		}
		if len(extra.Pointers) > 0 {
			found = append(found, established{key, extra.Pointers})
		}
		ak := workToolKind(e.Tool)
		if ak == "" {
			continue
		}
		ev := console.ActivityEvent{Session: key, At: at, Kind: ak}
		for _, p := range extra.Pointers {
			switch {
			case isFile(p):
				if ev.File == "" {
					ev.File = p
				}
			case ev.Symbol == "":
				ev.Symbol = p
			}
		}
		events = append(events, ev)
		if strings.HasPrefix(key, "s:") {
			sequenced++
		} else {
			unsequenced++
		}
	}
	var pointers []console.DiscoveryPointer
	for _, f := range found {
		for _, p := range f.ptrs {
			pointers = append(pointers, console.DiscoveryPointer{Symbol: p, At: lastAt[f.key]})
		}
	}
	rows := "rows name"
	if unsequenced == 1 {
		rows = "row names"
	}
	return events, pointers, fmt.Sprintf("search and lookup events: %d from the call log %s (the newest %d MiB), %d sequenced by session into runs an edit in the rule-outcome log ends (%d %s no session and %s not sequenced; a turn's answer is not recorded, so a run may span turns)",
		len(events), logPath, workCallTailBytes>>20, sequenced, unsequenced, rows, map[bool]string{true: "is", false: "are"}[unsequenced == 1])
}

// workRuleEdits turns the rule-outcome log's edit-hook events — a rule
// served, complied with, violated or blocked at a file in a session —
// into the edit events that end a search run.
func workRuleEdits(rules []console.RuleEvent) []console.ActivityEvent {
	var out []console.ActivityEvent
	for _, r := range rules {
		switch r.Event {
		case "served", "complied", "violated", "blocked":
			if r.Session != "" && r.File != "" {
				out = append(out, console.ActivityEvent{Session: "s:" + r.Session, At: workAt(r.At), Kind: console.ActEdit, File: r.File})
			}
		}
	}
	return out
}

// workRevertEvents turns the later commits WI-4w found reverting a
// proving commit into revert events on that commit's files (each revert
// commit and file once, whichever spelling of the proving commit the
// history used).
func workRevertEvents(commits map[string]console.CommitFacts) ([]console.ActivityEvent, string) {
	if len(commits) == 0 {
		return nil, "reverts: no proving commit was read"
	}
	keys := make([]string, 0, len(commits))
	for k := range commits {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	seen := map[string]bool{}
	reverted := map[string]bool{}
	unknown := 0
	var out []console.ActivityEvent
	for _, k := range keys {
		cf := commits[k]
		if !cf.RevertsKnown {
			unknown++
			continue
		}
		for _, rv := range cf.Reverts {
			reverted[rv.Commit] = true
			for _, f := range cf.Files {
				if seen[rv.Commit+"\x00"+f] {
					continue
				}
				seen[rv.Commit+"\x00"+f] = true
				out = append(out, console.ActivityEvent{Session: "commit:" + rv.Commit, At: workAt(rv.At), Kind: console.ActRevert, File: f,
					Note: fmt.Sprintf("%s: commit %s reverted proving commit %s", rv.At, shortCommit(rv.Commit), shortCommit(cf.Commit))})
			}
		}
	}
	note := fmt.Sprintf("reverts: %s of a proving commit's files, from the git log WI-4w reads", plural(len(reverted), "revert"))
	if unknown > 0 {
		note += fmt.Sprintf(" (%d proving commit spellings whose reverts were not read)", unknown)
	}
	return out, note
}

// workFiledDiscoveries reads the discovery pointers agents filed on this
// repository's threads (`thread note --pointer discoveries=…`) from the
// thread log's newest workThreadTailBytes.
func workFiledDiscoveries(ctx context.Context, sr serviceRoot) ([]console.DiscoveryPointer, string) {
	p := threadsPath()
	if p == "" {
		return nil, "no thread log"
	}
	data, err := readFileTail(p, workThreadTailBytes)
	if os.IsNotExist(err) {
		return nil, "no thread log at " + p
	} else if err != nil {
		return nil, p + " could not be read: " + err.Error()
	}
	scope := threadScope{repo: strings.ToLower(filepath.Base(sr.Root)), service: strings.ToLower(sr.Service)}
	if out, err := gitro.Output(ctx, sr.Root, "remote", "get-url", "origin"); err == nil && strings.TrimSpace(string(out)) != "" {
		scope.identity = normalizeOriginURL(string(out))
	} else if abs, err := filepath.Abs(sr.Root); err == nil {
		scope.identity = strings.ToLower(abs)
	}
	var out []console.DiscoveryPointer
	for _, line := range strings.Split(string(data), "\n") {
		var it threadItem
		if json.Unmarshal([]byte(line), &it) != nil || it.Pointers["discoveries"] == "" || !scope.owns(it) {
			continue
		}
		for _, ptr := range strings.Fields(it.Pointers["discoveries"]) {
			out = append(out, console.DiscoveryPointer{Symbol: ptr, At: workAt(it.FiledAt)})
		}
	}
	return out, fmt.Sprintf("%s filed on this repository's threads (%s, the newest %d MiB)", plural(len(out), "discovery pointer"), p, workThreadTailBytes>>20)
}

// workUnprovenAnchors anchors each claimed-unproven reading in the proof
// history to its row's contract — the plan's symbol and package, as a
// "package.Symbol" pointer — and, through the index, the file defining
// it. At most workMaxUnprovenReadings, newest rows first.
func workUnprovenAnchors(ctx context.Context, sr serviceRoot, h console.History, idx workIndex) ([]console.UnprovenAnchor, string) {
	type plan struct {
		expect map[string]verify.Expect
		err    string
	}
	plans := map[string]*plan{}
	readPlan := func(p string) *plan {
		if pl, ok := plans[p]; ok {
			return pl
		}
		pl := &plan{}
		plans[p] = pl
		full := p
		if !filepath.IsAbs(full) {
			full = filepath.Join(sr.Root, filepath.FromSlash(p))
		}
		fi, err := os.Stat(full)
		switch {
		case err != nil:
			pl.err = err.Error()
			return pl
		case fi.Size() > workPlanMaxBytes:
			pl.err = "over the size bound"
			return pl
		}
		md, err := os.ReadFile(full)
		if err != nil {
			pl.err = err.Error()
			return pl
		}
		pl.expect = map[string]verify.Expect{}
		for _, t := range verify.ParseMarkdownPlan(sr.Service, md).Tasks {
			pl.expect[t.ID] = t.Expect
		}
		return pl
	}
	var out []console.UnprovenAnchor
	readings, unanchored, resolved, cut := 0, 0, 0, 0
	for _, rh := range h.Rows {
		if !strings.EqualFold(rh.Service, sr.Service) {
			continue
		}
		for _, r := range rh.Readings {
			if r.State != console.HistUnproven {
				continue
			}
			if readings >= workMaxUnprovenReadings {
				cut++
				continue
			}
			readings++
			ex := readPlan(rh.Plan).expect[rh.ID]
			if ex.Symbol == "" {
				unanchored++
				continue
			}
			sym := ex.Symbol
			if ex.Package != "" {
				sym = strings.TrimSuffix(ex.Package, "/") + "." + ex.Symbol
			}
			a := console.UnprovenAnchor{Row: rh.ID, Symbol: sym}
			if idx != nil {
				if f, ok := idx.SymbolFile(ctx, sr.Service, sym); ok {
					a.File = f
					resolved++
				}
			}
			out = append(out, a)
		}
	}
	note := fmt.Sprintf("claimed-unproven anchors: %s from the verify ledger's history, anchored by their plans' contracts", plural(readings, "reading"))
	if idx != nil {
		note += fmt.Sprintf(", %d resolved to a file through the index", resolved)
	} else {
		note += " (no index to resolve their files: counted by symbol only)"
	}
	if unanchored > 0 {
		note += fmt.Sprintf("; %d carry no symbol (their plan or row was not read, or names none)", unanchored)
	}
	if cut > 0 {
		note += fmt.Sprintf("; %d older readings beyond the %d bound not read", cut, workMaxUnprovenReadings)
	}
	return out, note
}

// workFrictionInputs gathers FrictionSignals' inputs beyond the rule
// outcomes and names every source it read.
func workFrictionInputs(ctx context.Context, callLog string, sr serviceRoot, h console.History, commits map[string]console.CommitFacts,
	rules []console.RuleEvent, zones console.Zones, idx workIndex) (console.FrictionInputs, []string) {
	events, callPointers, callNote := workCallEvents(callLog, sr)
	filed, filedNote := workFiledDiscoveries(ctx, sr)
	reverts, revertNote := workRevertEvents(commits)
	unproven, unprovenNote := workUnprovenAnchors(ctx, sr, h, idx)
	in := console.FrictionInputs{Rules: rules, Zones: zones, Unproven: unproven}
	in.Events = append(append(events, workRuleEdits(rules)...), reverts...)
	in.Pointers = append(callPointers, filed...)
	pointerNote := fmt.Sprintf("rediscovery pointers: %d from the call log's discovery pointers (each established when its session's last call landed) and %s; "+
		"the session ledger's repeated-read figure is a per-session count that names no file, so it adds none", len(callPointers), filedNote)
	return in, []string{callNote, pointerNote, unprovenNote, revertNote}
}
