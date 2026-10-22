// discoveries.go — RC-25..RC-29, RC-32 (docs/vision/28-receipts.md §4):
// receipts on the hand-off. A discovery is a pointer a tool answer
// established (a symbol id in the same "pkgDir.Name" grammar the `ctx:`
// plan annotation uses, or a repo-relative file path) — never rendered
// text. Every render here resolves pointers FRESH against the live index
// at read time (reusing planscontext.go's ctxIndex seam: openCtxIndex,
// resolveFilePointer, splitPointer) and marks what no longer holds
// (ADR-028) — nothing here stores rendered discovery text.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/discoveryptr"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/statedir"
)

// discoveriesDefaultBudget mirrors the "discoveries" MCP tool's default
// (internal/mcp/discoveries_tool.go) — the same render on both surfaces.
const discoveriesDefaultBudget = 4000

// ─── session windows (design point 2) ───────────────────────────────────

// sessionWindowRow is one JSONL row of logs/sessions.jsonl under root's
// state directory (statedir.Resolve — outside the repo tree by default,
// O63 item 5): SessionStart's own mechanical fact — when a session began,
// in which directory, against which resolved service (best-effort; ""
// when none resolved). Never mined from a transcript; written once, at
// start, by cmdSessionHook.
type sessionWindowRow struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	Cwd           string `json:"cwd"`
	Service       string `json:"service,omitempty"`
	Started       string `json:"started"` // RFC3339
}

// sessionsLogPathFor is logs/sessions.jsonl under root's state directory
// (statedir.Resolve(root's git toplevel, or root itself outside a repo) —
// outside the repo tree by default, O63 item 5; root/workspace/logs/… when
// root's own config opts into the in-tree convention). FATHOM_SESSIONS_LOG
// overrides it directly (tests; an operator replaying a capture against a
// scratch tree) — it wins over even FATHOM_STATE_DIR.
func sessionsLogPathFor(root string) string {
	if p := os.Getenv("FATHOM_SESSIONS_LOG"); p != "" {
		return p
	}
	base := root
	if top := gitToplevel(root); top != "" {
		base = top
	}
	return filepath.Join(statedir.Resolve(base), "logs", "sessions.jsonl")
}

// recordSessionWindow appends row to root's sessions log under a
// cross-process flock (leases.go's own appendLease pattern: a session
// window row must never interleave with a concurrent session's write).
// Best-effort and silent on any failure — a hook must never fail session
// start over a missing log directory.
func recordSessionWindow(root string, row sessionWindowRow) {
	if row.SessionID == "" {
		return
	}
	path := sessionsLogPathFor(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	line, err := json.Marshal(row)
	if err != nil {
		return
	}
	_, _ = f.Write(append(line, '\n'))
}

// readSessionWindows reads every row in root's sessions log. A missing
// file is "no sessions recorded yet" (nil), not an error; a malformed
// line is skipped (best-effort instrument, never a gate).
func readSessionWindows(root string) []sessionWindowRow {
	data, err := os.ReadFile(sessionsLogPathFor(root))
	if err != nil {
		return nil
	}
	var out []sessionWindowRow
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var r sessionWindowRow
		if json.Unmarshal([]byte(line), &r) != nil {
			continue
		}
		out = append(out, r)
	}
	return out
}

// sessionStartFor looks up sessionID's recorded start under root. ok is
// false when no window row names that session (a harness that never fired
// SessionStart for it, or a log rotated/cleared since).
func sessionStartFor(root, sessionID string) (time.Time, bool) {
	if sessionID == "" {
		return time.Time{}, false
	}
	var found time.Time
	ok := false
	for _, r := range readSessionWindows(root) {
		if r.SessionID != sessionID {
			continue
		}
		if t, err := time.Parse(time.RFC3339, r.Started); err == nil {
			found, ok = t, true
		}
	}
	return found, ok
}

// ─── pointer aggregation (shared: discoveries + routes) ─────────────────

// discoveryPointerAgg is one pointer's aggregate over a call-log +
// observed-discoveries window: how many rows established it, the most
// recent timestamp, and (RC-56) whether an MCP call-log row or filed
// thread item ever named it (FromIndex) vs. only the discovery observer
// having seen it (ObservedVerb: "read"/"grepped" — vision 28 §4, "renders
// with a read/grep verb … so the reader knows it was located, not
// established by the index"). A pointer named by BOTH keeps FromIndex —
// it WAS established by a tool answer, so it renders exactly as before.
type discoveryPointerAgg struct {
	Pointer      string
	Count        int
	Last         time.Time
	FromIndex    bool
	ObservedVerb string
}

// callLogPointerRow is the minimal shape read back from a call-log JSONL
// row (internal/mcp/calllog.go's callLogEntry) — just enough to bucket by
// service, time, and pointer.
type callLogPointerRow struct {
	TS       time.Time
	Pointers []string
}

// readCallLogPointerRows reads logPath's JSONL rows, keeping only those
// naming service (when non-empty) with ts >= since and at least one
// pointer. A missing or malformed log degrades to empty, never an error.
func readCallLogPointerRows(logPath, service string, since time.Time) []callLogPointerRow {
	// Every generation in the window (RT-126): a rotation hides no
	// pointer, and a generation wholly before since is never opened.
	rc, err := openCallLogGenerationsSince(logPath, since)
	if err != nil {
		return nil
	}
	defer rc.Close()
	var out []callLogPointerRow
	sc := bufio.NewScanner(rc)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var row struct {
			TS       string   `json:"ts"`
			Service  string   `json:"service"`
			Origin   string   `json:"origin"`
			Pointers []string `json:"pointers"`
		}
		if json.Unmarshal([]byte(line), &row) != nil || len(row.Pointers) == 0 || mcp.IsConsoleRead(row.Origin) {
			continue
		}
		if service != "" && row.Service != service {
			continue
		}
		ts, terr := time.Parse(time.RFC3339Nano, row.TS)
		if terr != nil || ts.Before(since) {
			continue
		}
		out = append(out, callLogPointerRow{TS: ts, Pointers: row.Pointers})
	}
	return out
}

// gatherDiscoveryPointers folds readCallLogPointerRows plus this scope's
// filed thread items (a "discoveries" pointer set an agent explicitly
// filed, vision 28 §4's "no model distills anything" write path) into a
// ranked, deduped pointer list: most recent first, then most frequent,
// then lexical (determinism, internal/AGENTS.md).
func gatherDiscoveryPointers(logPath, service string, since time.Time, scope threadScope, root string) []discoveryPointerAgg {
	agg := map[string]*discoveryPointerAgg{}
	bump := func(ptr string, ts time.Time, fromIndex bool, observedVerb string) {
		ptr = strings.TrimSpace(ptr)
		if ptr == "" {
			return
		}
		e, ok := agg[ptr]
		if !ok {
			e = &discoveryPointerAgg{Pointer: ptr}
			agg[ptr] = e
		}
		e.Count++
		if ts.After(e.Last) {
			e.Last = ts
		}
		if fromIndex {
			e.FromIndex = true
		}
		if observedVerb != "" && e.ObservedVerb == "" {
			e.ObservedVerb = observedVerb
		}
	}
	for _, row := range readCallLogPointerRows(logPath, service, since) {
		for _, p := range row.Pointers {
			bump(p, row.TS, true, "")
		}
	}
	for _, it := range loadThreadItems(threadsPath()) {
		if !scope.owns(it) {
			continue
		}
		disc := it.Pointers["discoveries"]
		if disc == "" {
			continue
		}
		filedAt, ferr := time.Parse(time.RFC3339, it.FiledAt)
		if ferr != nil || filedAt.Before(since) {
			continue
		}
		for _, p := range strings.Fields(disc) {
			bump(p, filedAt, true, "")
		}
	}
	// RC-56: the discovery observer's own log — a planner that only grepped
	// or read still contributes pointers here, tagged with the verb that
	// located them rather than the index tool that would otherwise have.
	for _, row := range readObservedDiscoveryRows(root, since) {
		verb := observedDiscoveryVerb(row.Tool)
		for _, p := range row.Pointers {
			bump(p, row.TS, false, verb)
		}
	}
	out := make([]discoveryPointerAgg, 0, len(agg))
	for _, e := range agg {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Last.Equal(out[j].Last) {
			return out[i].Last.After(out[j].Last)
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Pointer < out[j].Pointer
	})
	return out
}

// discoveryCitationRE matches an obligations-style citation suffix
// ("path#Lstart-Lend", internal/semantic.Obligation.Src's own format).
var discoveryCitationRE = regexp.MustCompile(`^(.+)#(L\d+-L\d+)$`)

// stripDiscoveryCitation splits a citation-shaped pointer into its file
// path and the line-range suffix; any other pointer returns unchanged.
func stripDiscoveryCitation(ptr string) (filePath, citation string) {
	if m := discoveryCitationRE.FindStringSubmatch(ptr); m != nil {
		return m[1], m[2]
	}
	return ptr, ""
}

// discoveryResolution distinguishes HOW a pointer's render was produced —
// MAJOR fix: an unreachable/erroring index is an operational fact, never
// the positive claim "stale" makes (the index was never consulted, so it
// never said the pointer was gone).
type discoveryResolution int

const (
	discoveryResolved    discoveryResolution = iota // resolved live (file or symbol)
	discoveryStale                                  // index reachable; genuinely not found
	discoveryUnreachable                            // the index itself could not be consulted
)

// renderOneDiscoveryLine resolves ptr FRESH via idx/resolveFilePointer
// (planscontext.go's own seam) and renders one line — "read <path>" for a
// file, "symbol <ptr> → file:line" for a resolved symbol, "<ptr> —
// unresolved (index unreachable: <err>)" when the index itself could not
// be consulted, "<ptr> — stale (unresolved)" when the index WAS reachable
// and genuinely does not know the pointer, or (RC-56) "read <ptr>"/
// "grepped <ptr>" for a pointer the discovery OBSERVER saw but no index
// tool ever established (fromIndex false, observedVerb set) and that
// resolves as neither a file nor a symbol — a directory a Grep searched,
// say. Mirrors internal/mcp's renderDiscoveryPointer exactly (same
// grammar, different package: the two surfaces share the POINTER SHAPE,
// not code).
func renderOneDiscoveryLine(idx ctxIndex, idxErr error, ptr string, count int, fromIndex bool, observedVerb string) (line string, res discoveryResolution) {
	freq := ""
	if count > 1 {
		freq = fmt.Sprintf(" (%d)", count)
	}
	filePath, citation := stripDiscoveryCitation(ptr)
	if lines, ok := resolveFilePointer("", filePath); ok {
		suffix := ""
		if citation != "" {
			suffix = "#" + citation
		}
		_ = lines
		return fmt.Sprintf("read %s%s%s", filePath, suffix, freq), discoveryResolved
	}
	if !fromIndex && observedVerb != "" {
		// Came only from observation (vision 28 §4): the observer located it
		// — a grepped directory, say — even though it is not a file on disk
		// right now. This needs no index lookup at all (it was never a
		// symbol claim), so it renders BEFORE the idxErr/idx checks below —
		// an unreachable index must never turn an observed pointer into a
		// spurious "unresolved (index unreachable)" line.
		return fmt.Sprintf("%s %s%s", observedVerb, ptr, freq), discoveryResolved
	}
	if idxErr != nil {
		return fmt.Sprintf("%s — unresolved (index unreachable: %v)%s", ptr, idxErr, freq), discoveryUnreachable
	}
	if idx != nil {
		pkgHint, name := splitPointer(ptr)
		if sctx, ok := idx.resolveSymbol(pkgHint, name); ok {
			return fmt.Sprintf("symbol %s → %s:%d%s", ptr, sctx.Def.FilePath, sctx.Def.Line, freq), discoveryResolved
		}
	}
	return fmt.Sprintf("%s — stale (unresolved)%s", ptr, freq), discoveryStale
}

// renderDiscoveryLines renders ptrs in order, cutting whole lines from the
// bottom once budget bytes is reached and naming the cut (law 3) — the
// same elision shape planscontext.go's budgetContextPack uses.
//
// CRITICAL fix: resolution happens LAZILY, one pointer at a time, inside
// this same loop — never a separate eager pass over every pointer before
// the budget is applied. In HTTP mode each resolution is one network round
// trip (openCtxIndex's httpCtxIndex), so resolving pointers the budget will
// never render was the bug: a large or slow pointer set could blow well
// past a caller's own deadline (subagentDiscoveries) before a single byte
// of output existed.
func renderDiscoveryLines(idx ctxIndex, idxErr error, ptrs []discoveryPointerAgg, budget int) string {
	if len(ptrs) == 0 {
		return ""
	}
	var b strings.Builder
	included := 0
	for _, p := range ptrs {
		// Budget already full from prior lines: stop BEFORE resolving the
		// next pointer, not after — the whole point of laziness.
		if budget > 0 && b.Len() >= budget && included > 0 {
			break
		}
		line, _ := renderOneDiscoveryLine(idx, idxErr, p.Pointer, p.Count, p.FromIndex, p.ObservedVerb)
		row := line + "\n"
		if budget > 0 && b.Len()+len(row) > budget && included > 0 {
			break
		}
		b.WriteString(row)
		included++
	}
	if dropped := len(ptrs) - included; dropped > 0 {
		fmt.Fprintf(&b, "…%d more: fathom discoveries --budget %d\n", dropped, budget*2)
	}
	return b.String()
}

// callLogPathFor is the default call log discoveries reads: the daemon's
// own default (logs/mcp-calls.jsonl under root's state directory —
// statedir.Resolve, O63 item 5), matching defaultCallLogPath()'s
// convention without depending on process cwd.
func callLogPathFor(root string) string {
	base := root
	if top := gitToplevel(root); top != "" {
		base = top
	}
	return filepath.Join(statedir.Resolve(base), "logs", "mcp-calls.jsonl")
}

// parseSinceFlag accepts an RFC3339 timestamp or a bare Go duration
// ("2h", "45m"), the latter meaning "now minus that duration" — the same
// ergonomic a `--since` flag wants without a date-math dependency.
func parseSinceFlag(s string, now time.Time) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		return now.Add(-d), nil
	}
	return time.Time{}, fmt.Errorf("want an RFC3339 timestamp or a duration like \"2h\"")
}

// ─── fathom discoveries (RC-26) ──────────────────────────────────────────

func cmdDiscoveries(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("discoveries", stderr)
	sinceFlag := fs.String("since", "", "only pointers established at/after this time (RFC3339, or a duration like \"2h\"); default: all recorded history")
	budget := fs.Int("budget", discoveriesDefaultBudget, "byte budget for the rendered list")
	jsonOut := fs.Bool("json", false, "emit JSON instead of the rendered text")
	service := fs.String("service", "", "service to resolve pointers against (default: this repo's indexed service)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	storePath := fs.String("store", "", "store path, .sqlite/.db (direct mode; bypasses daemon)")
	callLog := fs.String("call-log", "", "call log path (default: workspace/logs/mcp-calls.jsonl under --root)")
	root := fs.String("root", ".", "repository root")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	rootAbs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	scope := threadScopeFor(rootAbs)
	svc := *service
	if svc == "" {
		svc = scope.service
	}
	logPath := *callLog
	if logPath == "" {
		logPath = callLogPathFor(rootAbs)
	}
	now := time.Now()
	var since time.Time
	if *sinceFlag != "" {
		t, perr := parseSinceFlag(*sinceFlag, now)
		if perr != nil {
			fmt.Fprintf(stderr, "error: --since: %v\n", perr)
			return 1
		}
		since = t
	}

	ptrs := gatherDiscoveryPointers(logPath, svc, since, scope, rootAbs)
	idx, idxErr := openCtxIndex(svc, *storePath, *url)
	rendered := renderDiscoveryLines(idx, idxErr, ptrs, *budget)

	if *jsonOut {
		type ptrJSON struct {
			Pointer  string `json:"pointer"`
			Count    int    `json:"count"`
			LastSeen string `json:"last_seen"`
		}
		pj := make([]ptrJSON, len(ptrs))
		for i, p := range ptrs {
			pj[i] = ptrJSON{Pointer: p.Pointer, Count: p.Count, LastSeen: p.Last.UTC().Format(time.RFC3339)}
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"service": svc, "since": since.UTC().Format(time.RFC3339), "budget": *budget,
			"pointers": pj, "rendered": rendered,
		})
		return 0
	}
	if rendered == "" {
		fmt.Fprintln(stdout, "no discoveries recorded")
		return 0
	}
	fmt.Fprint(stdout, rendered)
	return 0
}

// ─── the SubagentStart discoveries brief (RC-28) ─────────────────────────

// subagentDiscoveriesBudget is the discoveries chunk's own render budget —
// deliberately smaller than the 1200-byte TOTAL subagentPrimer enforces,
// leaving room for the base ≤600-byte primer ahead of it; the outer cut
// still applies as the hard backstop.
const subagentDiscoveriesBudget = 900

// subagentDiscoveriesMaxPointers caps how many pointers are even
// considered for resolution, BEFORE rendering — gatherDiscoveryPointers
// already orders recency-then-frequency, so this keeps only the most
// relevant candidates and bounds worst-case resolution work (one HTTP
// round trip per pointer in daemon mode) independent of how many pointers
// a busy session happened to establish.
const subagentDiscoveriesMaxPointers = 12

// subagentDiscoveriesDeadline bounds subagentDiscoveries' ENTIRE
// resolution pass — opening the index plus every pointer lookup — CRITICAL
// fix: this runs inside the harness's SubagentStart hook (~5s timeout)
// AFTER subagentPrimer has already built the constant identity+opening-
// moves primer. Past this deadline the discoveries brief is abandoned so
// the hook always returns promptly and the base primer ships alone,
// rather than an unreachable or slow index silently costing the whole
// primer (the base primer WITH it).
const subagentDiscoveriesDeadline = 2 * time.Second

// subagentDiscoveries renders what a SubagentStart hands the new sub-agent from
// the parent's state: the one map id the parent filed, when it filed any
// (XM-8, subagentMapLine), then "last session established: …" for a subagent
// starting under root, resolved fresh against service's live index — the
// parent session's discoveries since its own recorded start (sessionStartFor),
// falling back to now-2h when this repo never recorded that session (an older
// harness build, or a rotated log). The map line LEADS: the primer's cap
// (subagentDiscoveriesPrimerBudget, 1,200 bytes total) cuts the tail, so the
// brief yields bytes before the id ever does. room is how many bytes of that
// cap the chunk may fill (the base primer already spent the rest); with a map
// line the brief is rendered to what is left after it, and with none the
// brief's own budget applies as it always did. room <= 0 means the caller sets
// no total. "" when nothing was established (silence costs nothing —
// subagentPrimer.go's own doctrine for every advisory line here), OR when the
// brief's resolution did not finish within subagentDiscoveriesDeadline
// (CRITICAL fix: the base primer must still ship on time) and no map was filed.
func subagentDiscoveries(baseURL, root, service, parentSessionID, agentID string, room int, now time.Time) string {
	if service == "" {
		return ""
	}
	mapLine := subagentMapLine(hookMapRoot(root), parentSessionID, agentID, now)
	budget := subagentDiscoveriesBudget
	if mapLine != "" && room > 0 {
		left := room - len(mapLine) - 1 - len(subagentBriefLead) - 1 - subagentBriefFooterReserve
		if left < budget {
			budget = left
		}
	}
	var brief string
	if budget >= subagentBriefMinBudget {
		brief = subagentBrief(baseURL, root, service, parentSessionID, budget, now)
	}
	switch {
	case mapLine == "":
		return brief
	case brief == "":
		return mapLine
	}
	return mapLine + "\n" + brief
}

// subagentBriefLead heads the discoveries brief.
const subagentBriefLead = "last session established:"

// subagentBriefFooterReserve is what renderDiscoveryLines' closing "…N more"
// line can add past its budget, held back so a brief that shares the cap with a
// map line ends on a whole line and not on the primer's byte cut.
const subagentBriefFooterReserve = 56

// subagentBriefMinBudget is the least room worth rendering a brief into: under
// it not even one pointer line fits whole, so the map line ships alone rather
// than a brief the cap would clip mid-line.
const subagentBriefMinBudget = 60

// subagentBrief serves only the parent's own, session-current range evidence.
// Old rows without range hashes or caller identity are intentionally ineligible.
// Verification is local: an unreachable index cannot leak errors into a brief.
func subagentBrief(baseURL, root, service, parentSessionID string, budget int, now time.Time) string {
	ctx, cancel := context.WithTimeout(context.Background(), subagentDiscoveriesDeadline)
	defer cancel()

	since, ok := sessionStartFor(root, parentSessionID)
	if !ok || service == "" {
		return ""
	}
	type candidate struct {
		ptr discoveryptr.Pointer
		ts  time.Time
	}
	var candidates []candidate
	rc, err := openCallLogGenerationsSince(callLogPathFor(root), since)
	if err == nil {
		defer rc.Close()
		sc := bufio.NewScanner(rc)
		sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for sc.Scan() {
			if ctx.Err() != nil || hookDeadlineSpent() {
				return ""
			}
			var row struct {
				TS      string `json:"ts"`
				Service string `json:"service"`
				Origin  string `json:"origin"`
				Caller  struct {
					Session string  `json:"session"`
					AgentID *string `json:"agent_id"`
				} `json:"caller"`
				Ranges []discoveryptr.Pointer `json:"range_pointers"`
			}
			if json.Unmarshal(sc.Bytes(), &row) != nil || row.Service != service || row.Origin != "" || row.Caller.Session != parentSessionID || (row.Caller.AgentID == nil || *row.Caller.AgentID != "") {
				continue
			}
			ts, err := time.Parse(time.RFC3339Nano, row.TS)
			if err != nil || ts.Before(since) || ts.After(now) {
				continue
			}
			for _, ptr := range row.Ranges {
				candidates = append(candidates, candidate{ptr, ts})
			}
		}
	}
	for _, row := range readObservedDiscoveryRows(root, since) {
		if ctx.Err() != nil || hookDeadlineSpent() {
			return ""
		}
		if row.SessionID != parentSessionID || row.AgentID != "" || row.TS.After(now) || (row.Service != "" && row.Service != service) {
			continue
		}
		for _, ptr := range row.RangePointers {
			candidates = append(candidates, candidate{ptr, row.TS})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if !candidates[i].ts.Equal(candidates[j].ts) {
			return candidates[i].ts.After(candidates[j].ts)
		}
		return candidates[i].ptr.Ref() < candidates[j].ptr.Ref()
	})
	var b strings.Builder
	seen := map[string]bool{}
	count := 0
	for _, c := range candidates {
		if ctx.Err() != nil || hookDeadlineSpent() {
			return ""
		}
		p := c.ptr
		if seen[p.Ref()] || discoveryptr.NativeInstructions(p.Path) {
			continue
		}
		if _, ok := p.Current(root); !ok {
			continue
		}
		line := "read " + p.Ref() + "\n"
		if b.Len()+len(line) > budget || count >= subagentDiscoveriesMaxPointers {
			break
		}
		seen[p.Ref()] = true
		b.WriteString(line)
		count++
	}
	if b.Len() == 0 {
		return ""
	}
	return subagentBriefLead + "\n" + strings.TrimRight(b.String(), "\n")
}

// ─── the session-end pointer set (RC-29) ─────────────────────────────────

// sessionEndPointersMax is "the session's top 8 pointers" (vision 28 §4).
const sessionEndPointersMax = 8

// sessionEndPointers returns the session's top pointers (recency then
// frequency) since its own recorded start, for the machine-minted
// session-end thread note to carry in its Pointers map
// (cmdSessionEndHook, lifecyclehooks.go). Falls back to now-2h when this
// session's start was never recorded. nil when nothing was established —
// a clean/read-only session carries no discoveries, which is not an error.
func sessionEndPointers(root, sessionID string, now time.Time) []string {
	since := now.Add(-2 * time.Hour)
	if t, ok := sessionStartFor(root, sessionID); ok {
		since = t
	}
	scope := threadScopeFor(root)
	svc := scope.service
	logPath := callLogPathFor(root)
	agg := gatherDiscoveryPointers(logPath, svc, since, scope, root)
	if len(agg) == 0 {
		return nil
	}
	if len(agg) > sessionEndPointersMax {
		agg = agg[:sessionEndPointersMax]
	}
	out := make([]string, len(agg))
	for i, p := range agg {
		out[i] = p.Pointer
	}
	return out
}

// discoveriesPointerKey is the threadItem.Pointers map key a filed
// discoveries set lives under — space-separated, the same "pointers, not
// prose" grammar the plan grammar's `ctx:` field already uses.
const discoveriesPointerKey = "discoveries"

// sessionEndDiscoveriesLine renders "last session established: …" from a
// session-end threadItem's stored Pointers["discoveries"] set, resolved
// FRESH against service's live index at brief-render time — never from
// what was true when the note was filed (ADR-028). "" when the item
// carries no discoveries pointers, or none resolve to anything a reader
// could act on.
func sessionEndDiscoveriesLine(it threadItem, root string) string {
	disc := it.Pointers[discoveriesPointerKey]
	if disc == "" {
		return ""
	}
	ptrs := strings.Fields(disc)
	if len(ptrs) == 0 {
		return ""
	}
	scope := threadScopeFor(root)
	idx, idxErr := openCtxIndex(scope.service, "", defaultFathomURL())
	agg := make([]discoveryPointerAgg, len(ptrs))
	for i, p := range ptrs {
		agg[i] = discoveryPointerAgg{Pointer: p, Count: 1, FromIndex: true}
	}
	rendered := renderDiscoveryLines(idx, idxErr, agg, sessionEndBriefPointerBudget)
	rendered = strings.TrimRight(rendered, "\n")
	if rendered == "" {
		return ""
	}
	return "last session established: " + strings.ReplaceAll(rendered, "\n", "; ")
}

// sessionEndBriefPointerBudget bounds the pointer line INSIDE the thread
// brief's existing threadBriefBudgetBytes (2400) total — small on purpose,
// since it is one bullet among several.
const sessionEndBriefPointerBudget = 400

// discoveriesRunner runs `fathom discoveries` (registered below; EW-45).
type discoveriesRunner struct{}

func (discoveriesRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdDiscoveries(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "discoveries", Group: "hooks", Order: 30, Run: discoveriesRunner{},
		Summary: "Receipts on the hand-off: this repo's recent call-log pointers, resolved fresh against the live index"})
}
