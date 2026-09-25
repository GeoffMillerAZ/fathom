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
// internal/analysis/callgraph), recency (skipped with a note — no
// churn/last-touched analysis is wired into QueryInterface today; the
// unused internal/analysis/churn analyzer this note used to point to was
// retired, CV-M6), and open obligations (a RANKING signal on a candidate
// already found by something else, never a discovery mechanism on its own
// — a file cannot be found by "it has rules"). Every hop is logged; the
// answer is a diagnosis OBJECT with a citation per candidate naming which
// hop produced it, never a bare list.
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
	"unicode"

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

// localizeMaxCandidates / localizeMaxWhy are the payload-cap deliverable
// (ADR-041 D4, docs/bench/readings/lca-run-1-reading.md §3's 882,577-byte
// localize payload in tock-fathom-t2): candidates are capped at 25
// regardless of the byte budget fitToBudget would otherwise allow, and
// every candidate's own citation list is capped at 3 — a candidate with
// ten contributing hops still names only its three most authoritative
// (chronologically earliest, since hops run primary-seed before
// graph-expansion before obligations).
const (
	localizeMaxCandidates = 25
	localizeMaxWhy        = 3
)

// Graph-expansion score weights (ADR-041 D4): closer wins, resolved beats
// heuristic at the same distance, and the per-candidate contribution is
// scaled by how many distinct seeds' walks reached it
// (graphExpansionCandidateScore) — this scheme's own encoding of ranking
// criterion (d), convergence, without appending one why-citation per
// contributing seed.
const (
	localizeGraphDist1Typed     = 3.0
	localizeGraphDist1Heuristic = 2.0
	localizeGraphDist2Typed     = 1.5
	localizeGraphDist2Heuristic = 1.0
)

// graphExpansionScore returns the per-seed-contribution score for a
// candidate reached at distance (1 or 2) with the given tier.
func graphExpansionScore(distance int, tier string) float64 {
	switch {
	case distance <= 1 && tier == "typed":
		return localizeGraphDist1Typed
	case distance <= 1:
		return localizeGraphDist1Heuristic
	case tier == "typed":
		return localizeGraphDist2Typed
	default:
		return localizeGraphDist2Heuristic
	}
}

// graphExpansionCandidateScore scales the base per-distance/tier weight
// by how many distinct seeds' walks reached the candidate (convergence,
// ranking criterion (d)) — fromSeeds < 1 is treated as 1, never a zero or
// negative multiplier.
func graphExpansionCandidateScore(distance int, tier string, fromSeeds int) float64 {
	if fromSeeds < 1 {
		fromSeeds = 1
	}
	return graphExpansionScore(distance, tier) * float64(fromSeeds)
}

// pluralS (summary.go) is reused here for hop/why text ("1 hop" vs
// "2 hops", "1 seed" vs "2 seeds").

// localizeSeedKind names one shape of frontier item the primary loop
// below processes — the four input-derived kinds, produced once at the
// start. Graph expansion (ADR-041 D4, expandSeedsByGraph) runs as its own
// pass AFTER this loop, over every symbol key the loop actually resolved
// — never as a fifth frontier kind queued mid-loop, so the budget spends
// on two predictable aggregate hops (depth 1, depth 2) instead of one per
// resolved seed.
type localizeSeedKind string

const (
	localizeSeedFileLine localizeSeedKind = "file_line"
	localizeSeedFunction localizeSeedKind = "function"
	localizeSeedLiteral  localizeSeedKind = "literal"
	localizeSeedTerm     localizeSeedKind = "term"
)

// localizeSeed is one frontier item. File/Line are set only for
// localizeSeedFileLine.
type localizeSeed struct {
	Kind  localizeSeedKind
	Value string
	File  string
	Line  int
}

// localizeCandidate accumulates every hop's own contribution to one
// (file, symbol) pair — Symbol is "" when a hit named only a file (a
// search match with no enclosing symbol at that line). GraphDistance is
// ADR-041 D4's own field: the shortest graph-expansion distance (1 or 2)
// any seed reached this candidate at; 0 covers BOTH "this candidate IS a
// seed itself" and "graph expansion never touched it" — the spec's own
// "0 seed" reading makes the two indistinguishable on the wire by design.
type localizeCandidate struct {
	File          string
	Symbol        string
	Score         float64
	Tier          string // "typed" | "heuristic"
	Why           []string
	GraphDistance int
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

// ─── reformulation (ADR-041 D4) ─────────────────────────────────────────────

// reformulateIdentRE matches an identifier-shaped token of at least 3
// characters — splitIdentifier below decides whether it actually carries
// camelCase/snake_case shape (more than one word) or is just a plain
// word, which reformulateSeeds skips.
var reformulateIdentRE = regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9_]{2,}\b`)

// splitIdentifier splits an identifier into its constituent words —
// snake_case on "_", camelCase/PascalCase on a lower-to-upper transition
// — returning a single-element slice when s carries neither shape (a
// plain word or a single already-lowercase token), the signal
// reformulateSeeds uses to skip it as not identifier-shaped.
func splitIdentifier(s string) []string {
	if strings.Contains(s, "_") {
		var parts []string
		for _, p := range strings.Split(s, "_") {
			if p != "" {
				parts = append(parts, p)
			}
		}
		return parts
	}
	runes := []rune(s)
	var parts []string
	var cur []rune
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) && unicode.IsLower(runes[i-1]) {
			parts = append(parts, string(cur))
			cur = nil
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		parts = append(parts, string(cur))
	}
	return parts
}

// reformulateSeeds (ADR-041 D4) derives seeds from free text when
// ordinary extraction (localizeSeedsFor) found nothing to seed from at
// all — NEVER a model call: every identifier-shaped token
// (camelCase/snake_case, splitIdentifier finding more than one word)
// becomes both a function-name seed (the identifier itself, run through
// the SAME find_symbol hop a function seed always runs through) and a
// concern-term seed per significant word it splits into; every quoted
// literal becomes a literal seed; every path-shaped token's file
// basename (extension stripped) becomes a term seed. Returns the derived
// seeds (deduped, first-occurrence order) and the plain derived term
// strings the response's own `reformulated_terms` names. Deterministic:
// same text, same output, every time — no ranking or model call here,
// only regex and string splitting.
func reformulateSeeds(text string) ([]localizeSeed, []string) {
	var out []localizeSeed
	var terms []string
	seenTerm := map[string]bool{}
	add := func(kind localizeSeedKind, v string) {
		if v == "" {
			return
		}
		out = append(out, localizeSeed{Kind: kind, Value: v})
		if !seenTerm[v] {
			seenTerm[v] = true
			terms = append(terms, v)
		}
	}

	for _, m := range reformulateIdentRE.FindAllString(text, -1) {
		parts := splitIdentifier(m)
		if len(parts) < 2 {
			continue // a plain word, not an identifier shape
		}
		add(localizeSeedFunction, m)
		for _, p := range parts {
			if len(p) >= 3 && !localizeStopwords[strings.ToLower(p)] {
				add(localizeSeedTerm, p)
			}
		}
	}
	for _, m := range localizeQuotedRE.FindAllStringSubmatch(text, -1) {
		for _, g := range m[1:] {
			if g != "" {
				add(localizeSeedLiteral, g)
				break
			}
		}
	}
	for _, m := range localizeFrameColonRE.FindAllStringSubmatch(text, -1) {
		base := filepath.Base(m[1])
		base = strings.TrimSuffix(base, filepath.Ext(base))
		if len(base) >= 3 {
			add(localizeSeedTerm, base)
		}
	}
	return dedupSeeds(out), terms
}

// ─── graph expansion (ADR-041 D4) ───────────────────────────────────────────

// graphExpansion is one symbol expandSeedsByGraph found while walking the
// call graph outward from a set of seed keys — the SECOND and THIRD hop
// of localize's own walk (the FIRST hop is the seed extraction/scoring
// above; expandSeedsByGraph only ever walks from a symbol a seed hop
// already resolved). Key is the graph's own canonical key; Distance is 1
// or 2 hops from the nearest contributing seed (never 0 — a seed is
// never re-emitted as its own expansion candidate); Tier is "typed" when
// the SHORTEST path found used only typed edges, "heuristic" otherwise;
// Via is that shortest path's own seed-to-Key chain of canonical keys;
// FromSeeds is every distinct origin seed (sorted) whose walk reached Key
// at ANY distance — the convergence signal ranking criterion (d) uses.
type graphExpansion struct {
	Key       string
	Distance  int
	Tier      string
	Via       []string
	FromSeeds []string
}

// graphNeighbors is one symbol's direct callers AND callees, combined —
// the bidirectional "proximity" shape localize's own graph walk has
// always used (a caller and a callee are both evidence something is
// nearby, not merely code that calls forward).
func graphNeighbors(g *callgraph.Graph, key string) []callgraph.EdgeInfo {
	return append(append([]callgraph.EdgeInfo{}, callgraph.CallersDetail(g, key)...), callgraph.CalleesDetail(g, key)...)
}

// expandSeedsByGraph walks callers and callees of each of seeds, in the
// order given, out to depth hops (2 for localize's own D4 walk).
// Resolved (typed) edges are preferred: a seed whose OWN direct
// callers/callees include at least one typed edge walks typed edges
// only, at every hop of that seed's own walk; a seed with NONE (its
// direct neighbourhood is entirely syntax-only) falls back to walking
// both kinds, and every candidate that walk reaches is labeled
// "heuristic" — "resolved edges first; heuristic edges only when a seed
// has no resolved edges, and label them". Deterministic: seeds are
// walked in the given order, candidates are returned sorted by Key, and
// a candidate reached more than once keeps the shortest (then
// most-typed, then first-found-by-seed-order) path — never a later,
// longer or less-confident one.
func expandSeedsByGraph(seeds []string, graph *callgraph.Graph, depth int) []graphExpansion {
	if graph == nil || depth <= 0 {
		return nil
	}
	results := map[string]*graphExpansion{}
	var order []string

	type frontierNode struct {
		key  string
		path []string
	}

	for _, seed := range seeds {
		canon := callgraph.Canonical(graph, seed)
		if canon == "" {
			continue
		}
		useHeuristic := true
		for _, e := range graphNeighbors(graph, canon) {
			if e.Kind != "" {
				useHeuristic = false
				break
			}
		}

		visited := map[string]bool{canon: true}
		frontier := []frontierNode{{canon, []string{canon}}}
		for d := 1; d <= depth && len(frontier) > 0; d++ {
			var next []frontierNode
			for _, f := range frontier {
				for _, e := range graphNeighbors(graph, f.key) {
					if !useHeuristic && e.Kind == "" {
						continue // resolved-first: this seed has typed edges, so skip heuristic-only ones
					}
					if visited[e.Key] {
						continue
					}
					visited[e.Key] = true
					tier := "heuristic"
					if e.Kind != "" {
						tier = "typed"
					}
					path := append(append([]string{}, f.path...), e.Key)
					next = append(next, frontierNode{e.Key, path})

					rec, ok := results[e.Key]
					if !ok {
						rec = &graphExpansion{Key: e.Key, Distance: d, Tier: tier, Via: path}
						results[e.Key] = rec
						order = append(order, e.Key)
					} else if d < rec.Distance || (d == rec.Distance && tier == "typed" && rec.Tier == "heuristic") {
						rec.Distance = d
						rec.Tier = tier
						rec.Via = path
					}
					seen := false
					for _, s := range rec.FromSeeds {
						if s == canon {
							seen = true
							break
						}
					}
					if !seen {
						rec.FromSeeds = append(rec.FromSeeds, canon)
					}
				}
			}
			frontier = next
		}
	}

	sort.Strings(order)
	out := make([]graphExpansion, 0, len(order))
	for _, k := range order {
		rec := results[k]
		sort.Strings(rec.FromSeeds)
		out = append(out, *rec)
	}
	return out
}

// ─── the loop ───────────────────────────────────────────────────────────────

// runLocalize is registerLocalize's own testable core. The second return
// value is its answer-first summary sentence (wave L1) — computed here,
// alongside the response it describes, rather than re-derived by the
// caller from an already-built map.
func runLocalize(ctx context.Context, s *Server, backend QueryInterface, service, kind, text string, budgetIn int) (map[string]any, string) {
	allowed := clampLocalizeBudget(budgetIn)

	// ADR-014 execute-and-flag: an unindexed service is never a silent
	// empty — every hop would degrade to added:0 anyway (no symbols, no
	// corpus root to grep), so say so directly instead of making the
	// caller infer "not indexed" from a diagnosis that looks identical to
	// "no bug found here".
	if _, _, found, _ := backend.ServiceMeta(ctx, service); !found {
		note := fmt.Sprintf("service %q is not indexed — nothing to localize against; ingest it first", service)
		return map[string]any{
			"candidates": []map[string]any{},
			"hops":       []localizeHop{},
			"budget":     map[string]any{"allowed": allowed, "used": 0},
			"note":       note,
		}, capSummary(note)
	}

	used := 0
	var hops []localizeHop
	candidates := map[string]*localizeCandidate{}
	var candOrder []string

	upsert := func(file, symbol, tier string, score float64, why string, distance int) {
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
		if distance > 0 && (c.GraphDistance == 0 || distance < c.GraphDistance) {
			c.GraphDistance = distance
		}
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

	// seedGraphKeys is every symbol key the primary loop below actually
	// resolved (a stack frame's enclosing symbol, a find_symbol hit, or a
	// search hit's own enclosing symbol) — the walk origins ADR-041 D4's
	// graph-expansion pass runs from, AFTER this loop, once per resolved
	// symbol rather than once per hop.
	var seedGraphKeys []string
	seenSeedKey := map[string]bool{}
	addSeedKey := func(k string) {
		if k == "" || seenSeedKey[k] {
			return
		}
		seenSeedKey[k] = true
		seedGraphKeys = append(seedGraphKeys, k)
	}

	frontier := localizeSeedsFor(kind, text)
	reformulated := false
	var reformulatedTerms []string
	// ADR-041 D4: seed extraction found nothing at all to work with —
	// derive seeds by identifier splitting, quoted literals and file
	// basenames (reformulateSeeds), never a model call. Logged as its own
	// hop, even when it finds nothing either.
	if len(frontier) == 0 && used < allowed {
		used++
		derived, terms := reformulateSeeds(text)
		hops = append(hops, localizeHop{N: used, Tool: "reformulate", Arg: fmt.Sprintf("%d derived term(s) from identifier-splitting/literals/basenames", len(terms)), Added: len(derived)})
		if len(derived) > 0 {
			frontier = derived
			reformulated = true
			reformulatedTerms = terms
		}
	}

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
					upsert(matchPath, sym.Name, "typed", 10, "stack frame "+hop.Arg, 0)
					added++
					addSeedKey(sym.Key())
				} else {
					upsert(matchPath, "", "heuristic", 6, "stack frame "+hop.Arg+" (file; no enclosing symbol at that line)", 0)
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
				upsert(n.FilePath, n.Name, "typed", 8, "named "+item.Value, 0)
				added++
				addSeedKey(n.Key())
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
							var symKey string
							if syms, ok := byFile[rh.File]; ok {
								if n, found := enclosingSymbol(syms, rh.Line, nil); found {
									sym = n.Name
									symKey = n.Key()
								}
							}
							upsert(rh.File, sym, "heuristic", score, fmt.Sprintf("%s %q via search", label, item.Value), 0)
							added++
							addSeedKey(symKey)
						}
					}
				}
			}
		}

		hop.Added = added
		hops = append(hops, hop)
	}

	// Graph expansion (ADR-041 D4, hops 2 and 3): every symbol the primary
	// loop above actually resolved becomes a walk origin — callers and
	// callees over the typed/heuristic graph, to depth 2. Two aggregate
	// hops (never one per seed, so the budget spends predictably): depth
	// 1 then depth 2, each logged once regardless of how many seeds or
	// candidates it touched.
	if navs != nil && len(seedGraphKeys) > 0 {
		graph, _ := navs.pick(true) // both typed and heuristic: expandSeedsByGraph itself decides, per seed, whether heuristic edges are in play
		byDistance := map[int][]graphExpansion{}
		for _, ge := range expandSeedsByGraph(seedGraphKeys, graph, 2) {
			byDistance[ge.Distance] = append(byDistance[ge.Distance], ge)
		}
		for d := 1; d <= 2; d++ {
			if used >= allowed {
				break
			}
			used++
			added := 0
			for _, ge := range byDistance[d] {
				n, ok := byKey[ge.Key]
				if !ok {
					continue
				}
				names := make([]string, 0, len(ge.Via))
				for _, k := range ge.Via {
					if vn, ok := byKey[k]; ok {
						names = append(names, vn.Name)
					} else {
						names = append(names, k)
					}
				}
				why := fmt.Sprintf("graph path via: %s (%s, %d hop%s, %d seed%s)",
					strings.Join(names, " → "), ge.Tier, ge.Distance, pluralS(ge.Distance), len(ge.FromSeeds), pluralS(len(ge.FromSeeds)))
				upsert(n.FilePath, n.Name, ge.Tier, graphExpansionCandidateScore(ge.Distance, ge.Tier, len(ge.FromSeeds)), why, ge.Distance)
				added++
			}
			hops = append(hops, localizeHop{N: used, Tool: "graph_expand", Arg: fmt.Sprintf("depth %d from %d seed%s", d, len(seedGraphKeys), pluralS(len(seedGraphKeys))), Added: added})
		}
	}

	// Recency (VB-26: "the churn/last-touched analysis if present in the
	// store, else skip with a note"): no churn/last-touched analysis is
	// wired into any QueryInterface method today (the unused
	// internal/analysis/churn analyzer this note used to point to was
	// retired, CV-M6), so there is nothing to consult — one honest,
	// logged, zero-yield hop names why, rather than silently dropping the
	// dimension the plan row names.
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
		why := c.Why
		if len(why) > localizeMaxWhy {
			why = why[:localizeMaxWhy]
		}
		row := map[string]any{
			"file": c.File, "symbol": c.Symbol, "score": c.Score, "tier": c.Tier, "why": why,
		}
		if c.GraphDistance > 0 {
			row["graph_distance"] = c.GraphDistance
		}
		candOut = append(candOut, row)
	}

	// Payload cap (ADR-041 D4): capped at localizeMaxCandidates regardless
	// of the byte budget, THEN fit to the standard byte budget on top —
	// the 882,577-byte single payload lca-run-1 hit
	// (docs/bench/readings/lca-run-1-reading.md §3) never happens again
	// even before the byte cap engages. candidates_total is the count
	// before either cut, so a caller can always see what was elided.
	totalCandidates := len(candOut)
	capped := candOut
	if len(capped) > localizeMaxCandidates {
		capped = capped[:localizeMaxCandidates]
	}
	kept, _ := fitToBudget(capped, 0)

	resp := map[string]any{
		"candidates":       kept,
		"candidates_total": totalCandidates,
		"hops":             hops,
		"budget":           map[string]any{"allowed": allowed, "used": used},
	}
	if el := elisionFor(len(kept), totalCandidates, "score desc, then file, then symbol", "budget"); el != nil {
		resp["elided"] = el
	}
	if reformulated {
		resp["reformulated"] = true
		resp["reformulated_terms"] = reformulatedTerms
	}
	if _, indexCommit, found, _ := backend.ServiceMeta(ctx, service); found && indexCommit != "" {
		resp["index_commit"] = indexCommit
	}
	return resp, localizeSummary(out)
}

// localizeSummary renders localize's answer-first sentence (wave L1): the
// top candidate with its tier and why — built from out, the exact
// already-sorted (score desc, then file, then symbol) slice candOut above
// renders, so the summary can never name a candidate that outranks what
// the body actually lists first.
func localizeSummary(ranked []*localizeCandidate) string {
	if len(ranked) == 0 {
		return "no candidates found"
	}
	top := ranked[0]
	loc := top.File
	if top.Symbol != "" {
		loc += " (" + top.Symbol + ")"
	}
	summary := fmt.Sprintf("top candidate: %s — tier %s", loc, top.Tier)
	if len(top.Why) > 0 {
		summary += "; " + top.Why[0]
	}
	return capSummary(summary)
}

// localizeInputKinds is the four mutually-exclusive input shapes, in the
// order validation reports them.
var localizeInputKinds = []string{"stack_trace", "failing_test", "error", "concern"}

func registerLocalize(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "localize",
		Description: "Bounded walk from a stack trace, failing test, error, or concern to the file(s) a bug likely lives in. Reach for it when you don't yet know which file. Required: service.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":      map[string]any{"type": "string", "description": "Service / repo identifier"},
				"stack_trace":  map[string]any{"type": "string", "description": "Stack trace, verbatim — frames parsed as seeds"},
				"failing_test": map[string]any{"type": "string", "description": "Failing test's name (TestFoo/bar splits at /)"},
				"error":        map[string]any{"type": "string", "description": "Error message, verbatim — seeds are parsed out"},
				"concern":      map[string]any{"type": "string", "description": "Free-text description of the area or problem"},
				"budget":       map[string]any{"type": "integer", "description": "Maximum hops (default 12, max 40)"},
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
		body, summary := runLocalize(ctx, s, backend, p.Service, kind, vals[kind], p.Budget)
		return withSummary(summary, body), nil
	})
}
