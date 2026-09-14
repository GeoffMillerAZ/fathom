// Package mcp — localize (VB-26, docs/vision/29-verbs-not-lookups.md §3
// wave G; ADR-035): "where does this bug live" as a verb, never a bare
// list. R10 (LocAgent's 92.7% file-level accuracy on a syntax-only graph;
// SHERLOC's finding that agents spend about half their tool budget merely
// locating) is the case for promoting it: fathom's graph is richer and had
// no localize verb at all.
//
// localize runs ONE bounded, budgeted, LOGGED loop over the index: seed
// from whichever of stack_trace/failing_test/error/concern the caller
// gave (parsed frames, test names, quoted literals — the same grep path
// search.go's own engine runs — and concern terms), then expand by graph
// proximity (callers/callees over the typed/syntax edges,
// internal/analysis/callgraph), recency (skipped with a note — no churn
// analysis is wired into QueryInterface today), and open obligations (a
// RANKING signal on a candidate already found by something else, never a
// discovery mechanism on its own — a file cannot be found by "it has
// rules"). Every hop is logged; the answer is a diagnosis OBJECT with a
// citation per candidate naming which hop produced it, never a bare list.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// localizeDefaultBudget / localizeMaxBudget are VB-26's own bounds: a
// caller-supplied budget <= 0 defaults to 12 hops; anything above 40 is
// clamped down to it — never rejected, since a generous caller asking for
// more should still get a bounded, terminating answer.
const (
	localizeDefaultBudget = 12
	localizeMaxBudget     = 40
)

func clampLocalizeBudget(n int) int {
	if n <= 0 {
		return localizeDefaultBudget
	}
	if n > localizeMaxBudget {
		return localizeMaxBudget
	}
	return n
}

// localizeSeedKind names one shape of frontier item the loop below
// processes; the four input-derived kinds are produced once at the start,
// "proximity" is queued DURING the loop when a seed resolves to a real
// symbol.
type localizeSeedKind string

const (
	localizeSeedFileLine  localizeSeedKind = "file_line"
	localizeSeedFunction  localizeSeedKind = "function"
	localizeSeedLiteral   localizeSeedKind = "literal"
	localizeSeedTerm      localizeSeedKind = "term"
	localizeSeedProximity localizeSeedKind = "proximity"
)

// localizeSeed is one frontier item. File/Line are set only for
// localizeSeedFileLine; Label is set only for localizeSeedProximity (a
// human-readable name for the hop log — Value there is the graph's own,
// possibly fully-qualified, key).
type localizeSeed struct {
	Kind  localizeSeedKind
	Value string
	File  string
	Line  int
	Label string
}

// localizeCandidate accumulates every hop's own contribution to one
// (file, symbol) pair — Symbol is "" when a hit named only a file (a
// search match with no enclosing symbol at that line).
type localizeCandidate struct {
	File   string
	Symbol string
	Score  float64
	Tier   string // "typed" | "heuristic"
	Why    []string
}

// localizeHop is one bounded step of the loop, always logged regardless
// of whether it added anything.
type localizeHop struct {
	N     int    `json:"n"`
	Tool  string `json:"tool"`
	Arg   string `json:"arg"`
	Added int    `json:"added"`
}

// ─── seed extraction ───────────────────────────────────────────────────────

// localizeFrameColonRE matches a generic "path:line" stack-frame shape
// (Go, JS/TS, Ruby, Rust, …): a path-shaped token, a colon, a line number.
var localizeFrameColonRE = regexp.MustCompile(`([\w./-]+\.[A-Za-z0-9]+):(\d+)`)

// localizeFramePyRE matches Python's `File "path", line N` frame shape.
var localizeFramePyRE = regexp.MustCompile(`File "([^"]+)", line (\d+)`)

// localizeFuncNameRE matches a bare or dotted identifier immediately
// before a "(" — "FuncName(" or "pkg.Func(" — the function-name shape a
// stack trace or an error message carries.
var localizeFuncNameRE = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\(`)

// localizeQuotedRE matches a quoted literal (double, single or backtick
// quoted), at least 2 characters, the seed shape for the search hop.
var localizeQuotedRE = regexp.MustCompile("\"([^\"]{2,})\"|`([^`]{2,})`|'([^']{2,})'")

// failingTestNameRE extracts a go test name (optionally with a "/"
// subtest path) out of whatever `go test` runner formatting the caller
// pasted verbatim — "--- FAIL: TestBalance (0.00s)", "=== RUN
// TestBalance/sub", "FAIL: TestBalance" — and out of a bare name too: it
// just finds the "TestXxx[/subpath]" token wherever it sits, ignoring
// every other character around it, so no per-prefix-shape parsing is
// needed at all.
var failingTestNameRE = regexp.MustCompile(`Test[A-Za-z0-9_]*(?:/[^\s()]+)?`)

// localizeWordRE matches a candidate concern TERM: an identifier-shaped
// word of at least 4 characters.
var localizeWordRE = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]{3,}`)

// localizeStopwords are common English function words excluded from
// concern-term seeding — never a claim about the corpus, just noise
// reduction over free-form prose.
var localizeStopwords = map[string]bool{
	"this": true, "that": true, "with": true, "from": true, "into": true,
	"when": true, "what": true, "does": true, "not": true, "have": true,
	"has": true, "are": true, "was": true, "were": true, "been": true,
	"the": true, "and": true, "for": true, "its": true, "which": true,
	"about": true, "should": true, "would": true, "could": true,
	"there": true, "their": true, "than": true, "then": true,
}

// dedupSeeds keeps the FIRST occurrence of each (Kind, Value/File+Line)
// pair, preserving input order — the seed order IS the loop's own
// determinism, so this never sorts, only drops repeats.
func dedupSeeds(in []localizeSeed) []localizeSeed {
	seen := make(map[string]bool, len(in))
	out := make([]localizeSeed, 0, len(in))
	for _, s := range in {
		var key string
		if s.Kind == localizeSeedFileLine {
			key = string(s.Kind) + ":" + s.File + ":" + strconv.Itoa(s.Line)
		} else {
			key = string(s.Kind) + ":" + s.Value
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	return out
}

// localizeLastSegment returns the part of a dotted name after its last
// "." — "pkg.Func" -> "Func" — since FindSymbol looks up bare names.
func localizeLastSegment(s string) string {
	if i := strings.LastIndex(s, "."); i >= 0 {
		return s[i+1:]
	}
	return s
}

func localizeSeedsFromFrames(text string) []localizeSeed {
	var out []localizeSeed
	for _, m := range localizeFrameColonRE.FindAllStringSubmatch(text, -1) {
		line, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		out = append(out, localizeSeed{Kind: localizeSeedFileLine, File: m[1], Line: line})
	}
	for _, m := range localizeFramePyRE.FindAllStringSubmatch(text, -1) {
		line, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		out = append(out, localizeSeed{Kind: localizeSeedFileLine, File: m[1], Line: line})
	}
	for _, m := range localizeFuncNameRE.FindAllStringSubmatch(text, -1) {
		out = append(out, localizeSeed{Kind: localizeSeedFunction, Value: localizeLastSegment(m[1])})
	}
	return out
}

// localizeSeedsFor builds the seed set for one of the four input shapes.
// stack_trace and error both seed from frames (file:line, function
// names); error additionally seeds from quoted literals (an error string
// often quotes the offending value); failing_test strips whatever `go
// test` runner formatting the caller pasted verbatim (failingTestNameRE
// — "--- FAIL: TestBalance (0.00s)", "=== RUN   TestBalance/sub", "FAIL:
// TestBalance", or a bare name all reduce to the same test name) and
// seeds the base test name (subtests split at the first "/") as BOTH a
// symbol lookup and a literal (a non-Go test framework may not be in the
// typed index at all); concern seeds from its own significant words.
func localizeSeedsFor(kind, text string) []localizeSeed {
	var out []localizeSeed
	switch kind {
	case "stack_trace":
		out = localizeSeedsFromFrames(text)
	case "failing_test":
		name := strings.TrimSpace(text)
		if m := failingTestNameRE.FindString(name); m != "" {
			name = m
		}
		if name != "" {
			base := name
			if i := strings.Index(base, "/"); i >= 0 {
				base = base[:i]
			}
			out = append(out, localizeSeed{Kind: localizeSeedFunction, Value: base})
			out = append(out, localizeSeed{Kind: localizeSeedLiteral, Value: name})
		}
	case "error":
		out = localizeSeedsFromFrames(text)
		for _, m := range localizeQuotedRE.FindAllStringSubmatch(text, -1) {
			for _, g := range m[1:] {
				if g != "" {
					out = append(out, localizeSeed{Kind: localizeSeedLiteral, Value: g})
					break
				}
			}
		}
	case "concern":
		for _, w := range localizeWordRE.FindAllString(text, -1) {
			if localizeStopwords[strings.ToLower(w)] {
				continue
			}
			out = append(out, localizeSeed{Kind: localizeSeedTerm, Value: w})
		}
	}
	return dedupSeeds(out)
}

// ─── path matching ──────────────────────────────────────────────────────────

// localizePathsMatch reports whether a and b name the same file across an
// absolute-vs-repo-relative mismatch — equal, or one a "/"-separated
// suffix of the other (mirrors scenariobench's sameFile).
func localizePathsMatch(a, b string) bool {
	if a == b {
		return true
	}
	return strings.HasSuffix(a, "/"+b) || strings.HasSuffix(b, "/"+a)
}

// findFileByPathSuffix returns the first (in SORTED key order, for
// determinism) indexed file path suffix-matching target, and its
// Line-sorted symbols.
func findFileByPathSuffix(byFile map[string][]core.SymbolNode, target string) (string, []core.SymbolNode, bool) {
	target = filepath.ToSlash(target)
	keys := make([]string, 0, len(byFile))
	for k := range byFile {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if localizePathsMatch(k, target) {
			return k, byFile[k], true
		}
	}
	return "", nil, false
}

// ─── the loop ───────────────────────────────────────────────────────────────

// runLocalize is registerLocalize's own testable core.
func runLocalize(ctx context.Context, s *Server, backend QueryInterface, service, kind, text string, budgetIn int) map[string]any {
	allowed := clampLocalizeBudget(budgetIn)

	// ADR-014 execute-and-flag: an unindexed service is never a silent
	// empty — every hop would degrade to added:0 anyway (no symbols, no
	// corpus root to grep), so say so directly instead of making the
	// caller infer "not indexed" from a diagnosis that looks identical to
	// "no bug found here".
	if _, _, found, _ := backend.ServiceMeta(ctx, service); !found {
		return map[string]any{
			"candidates": []map[string]any{},
			"hops":       []localizeHop{},
			"budget":     map[string]any{"allowed": allowed, "used": 0},
			"note":       fmt.Sprintf("service %q is not indexed — nothing to localize against; ingest it first", service),
		}
	}

	used := 0
	var hops []localizeHop
	candidates := map[string]*localizeCandidate{}
	var candOrder []string

	upsert := func(file, symbol, tier string, score float64, why string) {
		key := file + "\x00" + symbol
		c, ok := candidates[key]
		if !ok {
			c = &localizeCandidate{File: file, Symbol: symbol, Tier: tier}
			candidates[key] = c
			candOrder = append(candOrder, key)
		}
		c.Score += score
		if tier == "typed" {
			c.Tier = "typed"
		}
		c.Why = append(c.Why, why)
	}

	navs := s.loadNavGraphs(ctx, backend, service)
	byFile := map[string][]core.SymbolNode{}
	if navs != nil {
		byFile = groupSymbolsByFile(navs.Nodes)
	}
	byKey := map[string]core.SymbolNode{}
	if navs != nil {
		for _, n := range navs.Nodes {
			byKey[n.Key()] = n
		}
	}

	seenProximity := map[string]bool{}
	frontier := localizeSeedsFor(kind, text)

	for len(frontier) > 0 && used < allowed {
		item := frontier[0]
		frontier = frontier[1:]
		used++
		added := 0
		hop := localizeHop{N: used}

		switch item.Kind {
		case localizeSeedFileLine:
			hop.Tool = "stack_frame"
			hop.Arg = fmt.Sprintf("%s:%d", item.File, item.Line)
			if matchPath, syms, ok := findFileByPathSuffix(byFile, item.File); ok {
				if sym, found := enclosingSymbol(syms, item.Line, nil); found {
					upsert(matchPath, sym.Name, "typed", 10, "stack frame "+hop.Arg)
					added++
					if key := sym.Key(); !seenProximity[key] {
						seenProximity[key] = true
						frontier = append(frontier, localizeSeed{Kind: localizeSeedProximity, Value: key, Label: sym.Name})
					}
				} else {
					upsert(matchPath, "", "heuristic", 6, "stack frame "+hop.Arg+" (file; no enclosing symbol at that line)")
					added++
				}
			}

		case localizeSeedFunction:
			hop.Tool = "find_symbol"
			hop.Arg = item.Value
			found, _ := backend.FindSymbol(ctx, service, item.Value)
			sort.Slice(found, func(i, j int) bool {
				if found[i].FilePath != found[j].FilePath {
					return found[i].FilePath < found[j].FilePath
				}
				return found[i].Name < found[j].Name
			})
			for _, n := range found {
				upsert(n.FilePath, n.Name, "typed", 8, "named "+item.Value)
				added++
				if key := n.Key(); !seenProximity[key] {
					seenProximity[key] = true
					frontier = append(frontier, localizeSeed{Kind: localizeSeedProximity, Value: key, Label: n.Name})
				}
			}

		case localizeSeedLiteral, localizeSeedTerm:
			hop.Tool = "search"
			hop.Arg = item.Value
			if root, err := resolveCorpusRoot(ctx, backend, service); err == nil {
				ignore, ignorePatterns, _, ierr := connector.LoadIgnoreScopedForWithPatterns(root, service)
				if ierr == nil {
					rawHits, _, serr := searchRun(ctx, root, regexp.QuoteMeta(item.Value), "", false, 0, ignore, ignorePatterns)
					if serr == nil {
						sort.Slice(rawHits, func(i, j int) bool {
							if rawHits[i].File != rawHits[j].File {
								return rawHits[i].File < rawHits[j].File
							}
							return rawHits[i].Line < rawHits[j].Line
						})
						label, score := "matched literal", 5.0
						if item.Kind == localizeSeedTerm {
							label, score = "matched concern term", 3.0
						}
						seenFile := map[string]bool{}
						for _, rh := range rawHits {
							if seenFile[rh.File] {
								continue
							}
							seenFile[rh.File] = true
							sym := ""
							if syms, ok := byFile[rh.File]; ok {
								if n, found := enclosingSymbol(syms, rh.Line, nil); found {
									sym = n.Name
								}
							}
							upsert(rh.File, sym, "heuristic", score, fmt.Sprintf("%s %q via search", label, item.Value))
							added++
						}
					}
				}
			}

		case localizeSeedProximity:
			hop.Tool = "graph_proximity"
			hop.Arg = item.Label
			if navs != nil {
				graph, _ := navs.pick(true) // both typed and heuristic edges: a proximity hop must not silently drop a syntax-only neighbor
				edges := append(append([]callgraph.EdgeInfo{}, callgraph.CallersDetail(graph, item.Value)...), callgraph.CalleesDetail(graph, item.Value)...)
				for _, e := range edges {
					n, ok := byKey[e.Key]
					if !ok {
						continue
					}
					tier := "heuristic"
					if e.Kind != "" {
						tier = "typed"
					}
					upsert(n.FilePath, n.Name, tier, 3, fmt.Sprintf("graph neighbor of %s (%s)", item.Label, tier))
					added++
				}
			}
		}

		hop.Added = added
		hops = append(hops, hop)
	}

	// Recency (VB-26: "the churn/last-touched analysis if present in the
	// store, else skip with a note"): internal/analysis/churn exists but
	// is wired into no QueryInterface method today, so there is nothing
	// to consult — one honest, logged, zero-yield hop names why, rather
	// than silently dropping the dimension the plan row names.
	if used < allowed {
		used++
		hops = append(hops, localizeHop{N: used, Tool: "recency", Arg: "no churn/last-touched analysis wired into the store — skipped", Added: 0})
	}

	// Obligations: a RANKING signal on a candidate already found by
	// something else — never a discovery mechanism (a file is not "found"
	// by carrying rules). One hop per unique candidate file, sorted for
	// determinism, bounded by whatever budget remains.
	if ob, ok := backend.(obligationsBackend); ok {
		filesSeen := map[string]bool{}
		var files []string
		for _, key := range candOrder {
			f := candidates[key].File
			if f == "" || filesSeen[f] {
				continue
			}
			filesSeen[f] = true
			files = append(files, f)
		}
		sort.Strings(files)
		for _, file := range files {
			if used >= allowed {
				break
			}
			used++
			added := 0
			if obl, oerr := ob.Obligations(ctx, service, file, 1); oerr == nil && len(obl.Rules) > 0 {
				added = 1
				why := fmt.Sprintf("governed: %s (%s)", obl.Rules[0].Rule, obl.Rules[0].Src)
				for _, key := range candOrder {
					c := candidates[key]
					if c.File != file {
						continue
					}
					c.Score += 2
					c.Why = append(c.Why, why)
				}
			}
			hops = append(hops, localizeHop{N: used, Tool: "obligations", Arg: file, Added: added})
		}
	}

	out := make([]*localizeCandidate, 0, len(candOrder))
	for _, key := range candOrder {
		out = append(out, candidates[key])
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Symbol < out[j].Symbol
	})

	candOut := make([]map[string]any, 0, len(out))
	for _, c := range out {
		candOut = append(candOut, map[string]any{
			"file": c.File, "symbol": c.Symbol, "score": c.Score, "tier": c.Tier, "why": c.Why,
		})
	}

	resp := map[string]any{
		"candidates": candOut,
		"hops":       hops,
		"budget":     map[string]any{"allowed": allowed, "used": used},
	}
	if _, indexCommit, found, _ := backend.ServiceMeta(ctx, service); found && indexCommit != "" {
		resp["index_commit"] = indexCommit
	}
	return resp
}

// localizeInputKinds is the four mutually-exclusive input shapes, in the
// order validation reports them.
var localizeInputKinds = []string{"stack_trace", "failing_test", "error", "concern"}

func registerLocalize(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "localize",
		Description: "The answer grep cannot give when you don't yet know which file: one bounded, budgeted, LOGGED walk of the index from a stack trace, a failing test name, an error string, or a concern — never a bare list, a diagnosis object. Exactly one of stack_trace, failing_test, error, concern must be given (checked at call time, not the schema's own required list). Seeds from parsed stack frames (file:line, function names), quoted literals (the same grep engine `search` runs), or a concern's own significant terms; expands by graph proximity (callers/callees over the typed/syntax call graph — internal/analysis/callgraph), open obligations (a governed file ranks higher, cited, never a discovery path on its own), and recency (skipped with a note today — no churn analysis is wired in yet). Every hop is logged (tool, arg, how many candidates it added) whether or not it found anything. Response: candidates (file, symbol, score, tier — typed when a call-graph or index lookup confirmed it, heuristic when only a text match did — and why, the citation strings naming which hop produced it), hops, budget ({allowed, used}), and index_commit when the backend can name one. An unindexed service returns an empty diagnosis with a note naming it, never a silent empty that reads as \"no bug found\". Deterministic: identical input against an unchanged index returns byte-identical results. Reach for it when you have a crash, a failing test, or a described problem and do not yet know which file it lives in — a literal string with a KNOWN location is search's job instead. Required: service.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":      map[string]any{"type": "string", "description": "Service / repo identifier"},
				"stack_trace":  map[string]any{"type": "string", "description": "A stack trace or traceback, verbatim — file:line frames and function names are parsed out as seeds"},
				"failing_test": map[string]any{"type": "string", "description": "A failing test's name (subtests like TestFoo/bar are split at the first \"/\" for the symbol lookup)"},
				"error":        map[string]any{"type": "string", "description": "An error message, verbatim — quoted literals and any embedded file:line frames are parsed out as seeds"},
				"concern":      map[string]any{"type": "string", "description": "A free-text description of the area or problem — significant terms are searched"},
				"budget":       map[string]any{"type": "integer", "description": "Maximum hops (default 12, max 40) — always a floor under a bounded, terminating answer, never rejected for being too high"},
			},
			"required": []string{"service"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service     string `json:"service"`
			StackTrace  string `json:"stack_trace"`
			FailingTest string `json:"failing_test"`
			Error       string `json:"error"`
			Concern     string `json:"concern"`
			Budget      int    `json:"budget"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "localize: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "localize: service required"}
		}
		vals := map[string]string{
			"stack_trace": p.StackTrace, "failing_test": p.FailingTest,
			"error": p.Error, "concern": p.Concern,
		}
		var present []string
		for _, k := range localizeInputKinds {
			if strings.TrimSpace(vals[k]) != "" {
				present = append(present, k)
			}
		}
		if len(present) != 1 {
			return nil, &rpcError{Code: ErrCodeParams, Message: fmt.Sprintf("localize: exactly one of stack_trace, failing_test, error, concern is required, got %d (%s)", len(present), strings.Join(present, ", "))}
		}
		kind := present[0]
		return runLocalize(ctx, s, backend, p.Service, kind, vals[kind], p.Budget), nil
	})
}
