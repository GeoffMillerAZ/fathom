// routes.go — RC-30 (docs/vision/28-receipts.md §4): long-lived discovery
// paths. A route is a pointer established in TWO OR MORE distinct
// sessions over the last 30 days — attributed by workspace/logs/
// sessions.jsonl's own recorded windows (discoveries.go), since the call
// log carries no session id of its own. Resolved fresh, grouped by
// package prefix, dropped the moment the index no longer confirms them.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// routesWindow is the 30-day lookback RC-30 pins.
const routesWindow = 30 * 24 * time.Hour

// routedPointer is one pointer that crossed the two-distinct-session bar.
// FromIndex/ObservedVerb carry the same RC-56 provenance
// discoveryPointerAgg does, so a route established only by grepping (never
// by an index tool) still renders with its "grepped"/"read" verb instead
// of a wrong "stale (unresolved)".
type routedPointer struct {
	Pointer      string
	Sessions     int
	FromIndex    bool
	ObservedVerb string
}

// routedPointerAgg is routedPointers' internal accumulator: which
// sessions named a pointer, and whether an index tool ever did (vs. only
// the discovery observer).
type routedPointerAgg struct {
	sessions     map[string]bool
	fromIndex    bool
	observedVerb string
}

// sessionForTimestamp attributes ts to the most recent session window that
// had already started by ts — the best available approximation absent an
// explicit session id on every call-log row (design note, RC-30): a
// session's own window is [Started, next session's Started).
func sessionForTimestamp(windows []sessionWindowRow, ts time.Time) string {
	best := ""
	var bestStart time.Time
	for _, w := range windows {
		t, err := time.Parse(time.RFC3339, w.Started)
		if err != nil || t.After(ts) {
			continue
		}
		if best == "" || t.After(bestStart) {
			best, bestStart = w.SessionID, t
		}
	}
	return best
}

// routedPointers folds logPath's rows (service-scoped, within `since`)
// plus root's observed-discoveries log (RC-56: "fathom routes counts
// observed pointers too" — attributed directly by each row's own
// session_id, no sessionForTimestamp approximation needed) into the
// pointers established in >= 2 distinct sessions, ranked by session count
// then lexical.
func routedPointers(logPath, service string, windows []sessionWindowRow, since time.Time, root string) []routedPointer {
	agg := map[string]*routedPointerAgg{}
	touch := func(p, sid string, fromIndex bool, observedVerb string) {
		if sid == "" {
			return
		}
		e, ok := agg[p]
		if !ok {
			e = &routedPointerAgg{sessions: map[string]bool{}}
			agg[p] = e
		}
		e.sessions[sid] = true
		if fromIndex {
			e.fromIndex = true
		}
		if observedVerb != "" && e.observedVerb == "" {
			e.observedVerb = observedVerb
		}
	}
	for _, row := range readCallLogPointerRows(logPath, service, since) {
		sid := sessionForTimestamp(windows, row.TS)
		for _, p := range row.Pointers {
			touch(p, sid, true, "")
		}
	}
	for _, row := range readObservedDiscoveryRows(root, since) {
		verb := observedDiscoveryVerb(row.Tool)
		for _, p := range row.Pointers {
			touch(p, row.SessionID, false, verb)
		}
	}
	var out []routedPointer
	for p, e := range agg {
		if len(e.sessions) >= 2 {
			out = append(out, routedPointer{Pointer: p, Sessions: len(e.sessions), FromIndex: e.fromIndex, ObservedVerb: e.observedVerb})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Sessions != out[j].Sessions {
			return out[i].Sessions > out[j].Sessions
		}
		return out[i].Pointer < out[j].Pointer
	})
	return out
}

// routePackagePrefix groups a pointer by its package/directory: a file
// pointer's own directory (checked via resolveFilePointer, the same
// file-first disambiguation planscontext.go's renderContextPack applies,
// since a bare pointer's dots could be a file extension OR a symbol's
// last-dot split), else a symbol pointer's pkgHint half (splitPointer).
func routePackagePrefix(ptr string) string {
	filePath, _ := stripDiscoveryCitation(ptr)
	if _, ok := resolveFilePointer("", filePath); ok {
		dir := filepath.Dir(filePath)
		if dir == "." {
			return "(root)"
		}
		return dir
	}
	pkgHint, _ := splitPointer(ptr)
	if pkgHint == "" {
		return "(unscoped)"
	}
	return pkgHint
}

// cmdRoutes implements `fathom routes [package|concern]`: every routed
// pointer (>= 2 distinct sessions, last 30 days), resolved fresh and
// dropped when the index no longer confirms it, grouped by package prefix
// and optionally filtered to one named prefix.
func cmdRoutes(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("routes", stderr)
	root := fs.String("root", ".", "repository root")
	service := fs.String("service", "", "service to resolve pointers against (default: this repo's indexed service)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	storePath := fs.String("store", "", "store path, .sqlite/.db (direct mode; bypasses daemon)")
	callLog := fs.String("call-log", "", "call log path (default: workspace/logs/mcp-calls.jsonl under --root)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human list")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	var filterPrefix string
	if fs.NArg() > 0 {
		filterPrefix = fs.Arg(0)
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
	windows := readSessionWindows(rootAbs)
	routed := routedPointers(logPath, svc, windows, now.Add(-routesWindow), rootAbs)

	idx, idxErr := openCtxIndex(svc, *storePath, *url)
	type resolvedRoute struct {
		Pkg      string
		Line     string
		Sessions int
		Resolved bool
	}
	var resolved []resolvedRoute
	for _, r := range routed {
		line, res := renderOneDiscoveryLine(idx, idxErr, r.Pointer, 0, r.FromIndex, r.ObservedVerb)
		if res == discoveryStale {
			continue // dropped: the index confirmed it's genuinely gone (RC-30)
		}
		// MAJOR fix: an unreachable index (res == discoveryUnreachable) is
		// KEPT, not dropped — the index never said this pointer was gone,
		// it simply couldn't be asked. Silently dropping it here is exactly
		// how "no routes yet" got printed for a healthy set of candidates.
		pkg := routePackagePrefix(r.Pointer)
		if filterPrefix != "" && pkg != filterPrefix && !strings.HasPrefix(pkg, filterPrefix) {
			continue
		}
		resolved = append(resolved, resolvedRoute{Pkg: pkg, Line: line, Sessions: r.Sessions, Resolved: res == discoveryResolved})
	}
	sort.Slice(resolved, func(i, j int) bool {
		if resolved[i].Pkg != resolved[j].Pkg {
			return resolved[i].Pkg < resolved[j].Pkg
		}
		if resolved[i].Sessions != resolved[j].Sessions {
			return resolved[i].Sessions > resolved[j].Sessions
		}
		return resolved[i].Line < resolved[j].Line
	})

	if *jsonOut {
		type row struct {
			Package  string `json:"package"`
			Line     string `json:"line"`
			Sessions int    `json:"sessions"`
			Resolved bool   `json:"resolved"`
		}
		rows := make([]row, len(resolved))
		for i, r := range resolved {
			rows[i] = row{Package: r.Pkg, Line: r.Line, Sessions: r.Sessions, Resolved: r.Resolved}
		}
		out := map[string]any{"service": svc, "window_days": int(routesWindow.Hours() / 24), "routes": rows}
		if idxErr != nil {
			// MAJOR fix: name the operational fact instead of letting an
			// empty/partial "routes" array read as "nothing routes here".
			out["index_unreachable"] = idxErr.Error()
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		return 0
	}
	if len(resolved) == 0 {
		if idxErr != nil {
			// MAJOR fix: an unreachable index means "unknown", never "no
			// routes yet" — that line is a positive claim the index never
			// made (a routed candidate may well exist and simply
			// couldn't be confirmed this call).
			fmt.Fprintf(stdout, "routes unknown — index unreachable: %v\n", idxErr)
			return 0
		}
		fmt.Fprintln(stdout, "no routes yet — a route needs the same pointer established in 2+ sessions over the last 30 days")
		return 0
	}
	pkg := ""
	for _, r := range resolved {
		if r.Pkg != pkg {
			pkg = r.Pkg
			fmt.Fprintf(stdout, "%s\n", pkg)
		}
		fmt.Fprintf(stdout, "  %s (in %d sessions)\n", r.Line, r.Sessions)
	}
	return 0
}

// routesSummaryCount is the SessionStart primer's cheap existence check
// (RC-30: "names routes in ONE line only when >= 1 exists"): the count of
// distinct routed pointers over the 30-day window, WITHOUT the live-index
// resolution pass `fathom routes` itself does — a primer line must stay
// free (no daemon round trip beyond the one buildSessionContext already
// makes), so this counts CANDIDATES and can over-count a since-moved
// pointer by one until the operator actually runs `fathom routes`.
func routesSummaryCount(root, service string, now time.Time) int {
	if service == "" {
		return 0
	}
	windows := readSessionWindows(root)
	logPath := callLogPathFor(root)
	return len(routedPointers(logPath, service, windows, now.Add(-routesWindow), root))
}

// routesSummaryLine renders the one-line primer addition, "" when no
// route exists yet.
func routesSummaryLine(root, service string, now time.Time) string {
	n := routesSummaryCount(root, service, now)
	if n == 0 {
		return ""
	}
	pkgs := map[string]bool{}
	windows := readSessionWindows(root)
	logPath := callLogPathFor(root)
	for _, r := range routedPointers(logPath, service, windows, now.Add(-routesWindow), root) {
		pkgs[routePackagePrefix(r.Pointer)] = true
	}
	return fmt.Sprintf("routes known for %d package(s) — fathom routes", len(pkgs))
}
