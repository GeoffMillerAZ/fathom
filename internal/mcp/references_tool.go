package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/analysis/references"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// referencesDefaultMax is the default occurrence cap (SB-1,
// docs/plans/strengths-bench.md; O31/O32): the first replicated home bench
// run found references paying 1.90x grep's tool-result bytes, with the
// worst task (a two-line grep) costing 18.6x its answer's size because a
// junk query kept exploring at 200 occurrences/call, 6-7KB each. 25 is
// enough to recognize a hit pattern and decide the next move; the elided
// block names how to get more.
const referencesDefaultMax = 25

// occWireMaxBytes bounds one rendered occurrence on the wire (SB-1): a
// combined "file:line" location, trimmed text, and a one-character
// confidence code, held by trimming the text (never the location or
// confidence, which are exact facts) until the marshaled row fits — rather
// than a fixed byte split per field that could silently drift from
// encoding/json's actual output. The bound is conditional on the location:
// the fixed envelope {"c":"h","loc":"","text":""} is 28 bytes, so a
// file:line longer than 72 bytes (fewer with a per-row svc) cannot fit
// even with no text. Such a row keeps its location whole, carries an empty
// text, and exceeds the bound by exactly what the location costs (BP-41,
// TestReferencesOccurrenceLongPathKeepsLocationWhole).
const occWireMaxBytes = 100

// occTextMaxChars is the independent, tighter ceiling on occurrence text
// itself: even a short location must never carry a whole source line.
const occTextMaxChars = 60

var (
	// referenceIdentifierRe matches a bare Go-ish identifier: "FindMe".
	referenceIdentifierRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	// referenceDottedPathRe matches a dotted path: "pkg.Sym", "Type.Method",
	// or a slash-qualified package path before the dot ("internal/mcp.Server").
	referenceDottedPathRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_/]*\.[A-Za-z_][A-Za-z0-9_]*$`)
)

// refuseNonSymbolQuery reports why query is refused, or "" when it is
// acceptable (SB-2, O31: the junk query "func (s *" turned one grep-shaped
// slip into a 27-heuristic-line, several-KB scan that kept exploring).
// references accepts exactly three forms when regex is not declared: a
// bare identifier, a dotted path ("pkg.Sym" or "Type.Method"), or — the
// third form — a caller-declared regular expression, which is never
// second-guessed here once regexDeclared is true.
func refuseNonSymbolQuery(query string, regexDeclared bool) string {
	if regexDeclared {
		return ""
	}
	if referenceIdentifierRe.MatchString(query) || referenceDottedPathRe.MatchString(query) {
		return ""
	}
	return fmt.Sprintf("references: query %q is not one of the three accepted forms — an identifier (e.g. \"FindMe\"), a dotted path (\"pkg.Sym\" or \"Type.Method\"), or a declared regular expression (regex: true)", query)
}

// interfaceMethodQuery reports whether query names a method exactly one
// indexed interface declares — bare ("PutObservations") or dotted with the
// interface's own name ("Store.PutObservations") — returning that
// interface's node and the bare method name. Go forbids a method whose
// receiver names an interface type, so an interface's own methods are
// NEVER indexed as their own GoMethod symbol (only KindGoInterface for the
// interface type itself); this is the only place "is query an interface
// method" can be answered — by parsing each candidate interface's own
// declared method set (interfaceMethodNames), never by a definition
// lookup. Ambiguous (more than one interface declares the bare method) or
// no match returns ok=false — a conservative false negative is safe, a
// wrong interface's implementers is not (RC-21, docs/vision/28-receipts.md
// §3.4).
func interfaceMethodQuery(sources []core.SourceFile, nodes []core.SymbolNode, query string) (iface core.SymbolNode, method string, ok bool) {
	ifaceName, bareMethod := "", query
	if i := strings.LastIndexByte(query, '.'); i >= 0 {
		ifaceName, bareMethod = query[:i], query[i+1:]
	}
	var matches []core.SymbolNode
	for _, n := range nodes {
		if n.Kind != core.KindGoInterface {
			continue
		}
		if ifaceName != "" && n.Name != ifaceName {
			continue
		}
		if interfaceMethodNames(sources, n)[bareMethod] {
			matches = append(matches, n)
		}
	}
	if len(matches) != 1 {
		return core.SymbolNode{}, "", false
	}
	return matches[0], bareMethod, true
}

// confidenceChar collapses an edge_confidence value to a single wire
// character: "r" for resolved, "h" for anything else (heuristic).
func confidenceChar(conf string) string {
	if conf == string(core.EdgeConfidenceResolved) {
		return "r"
	}
	return "h"
}

// truncateForWire trims s to at most maxChars runes (never splitting a
// multi-byte rune), first trimming surrounding whitespace so a truncated
// line never starts or ends mid-indent, and appends an ellipsis when it
// actually cut something.
func truncateForWire(s string, maxChars int) string {
	s = strings.TrimSpace(s)
	if maxChars <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxChars {
		return s
	}
	runes := []rune(s)
	if maxChars <= 1 {
		return string(runes[:maxChars])
	}
	return strings.TrimRight(string(runes[:maxChars-1]), " ") + "…"
}

// defEntry is one references definition row — hoisted to package scope
// (was a local type inside registerReferences) so referencesSummary (wave
// L1) can share it without a second, hand-copied shape.
type defEntry struct {
	Service string `json:"service"`
	Symbol  string `json:"symbol"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Tier    string `json:"tier"`
	// Depth/EdgeConfidence (BD-2): a definition is depth 0; its
	// confidence is the index's own evidence tier (verified =
	// resolved, anything weaker = heuristic).
	Depth          int    `json:"depth"`
	EdgeConfidence string `json:"edge_confidence"`
}

// occEntry is one references occurrence in its full, internal shape — the
// working representation registerReferences builds and by_depth/applyBudget
// operate over, before renderReferenceOccurrence compacts it for the wire.
type occEntry struct {
	Service string `json:"service"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Text    string `json:"text"`
	Tier    string `json:"tier"`
	// Depth/EdgeConfidence (BD-2): a mention is depth 1 — one hop from the
	// definition; resolved only when the symbol enclosing the mention has
	// a TYPED call edge to a definition of the query in the same service,
	// heuristic for a bare text match.
	Depth          int    `json:"depth"`
	EdgeConfidence string `json:"edge_confidence"`
}

// occWireSize is the marshaled byte size of one rendered occurrence.
func occWireSize(w referenceOccurrence) int {
	b, err := json.Marshal(w)
	if err != nil {
		return occWireMaxBytes + 1
	}
	return len(b)
}

// renderReferenceOccurrence renders one references occurrence for the wire
// at up to occWireMaxBytes (100) bytes (SB-1, O31/O32): a combined
// "file:line" location, the occurrence text trimmed to at most
// occTextMaxChars (60) runes and then further shrunk if needed to hit the
// byte cap, and confidence collapsed to a single character. A location too
// long to leave room empties the text and keeps the row over the cap
// rather than cut the path (see occWireMaxBytes). includeService
// is set only when the response's occurrences span more than one service —
// service, tier and depth are otherwise CONSTANT across every occurrence
// row (a mention is always depth 1, tier pre-scan) and are lifted to the
// response envelope by registerReferences instead of repeated per row.
func renderReferenceOccurrence(o occEntry, includeService bool) referenceOccurrence {
	w := referenceOccurrence{
		Loc:  o.File + ":" + strconv.Itoa(o.Line),
		Text: truncateForWire(o.Text, occTextMaxChars),
		C:    confidenceChar(o.EdgeConfidence),
	}
	if includeService {
		w.Svc = o.Service
	}
	for occWireSize(w) > occWireMaxBytes && len(w.Text) > 0 {
		n := utf8.RuneCountInString(w.Text) - 1
		if n < 0 {
			n = 0
		}
		w.Text = truncateForWire(w.Text, n)
	}
	return w
}

// registerReferences wires the "references" tool into the investigation group.
// It performs a fleet-wide occurrence search across all source files for a
// query string, returning structured occurrences (text matches, tier pre-scan)
// plus high-confidence symbol definitions (verified tier).
func registerReferences(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "references",
		Description: "Fleet-wide occurrence search: identifier, dotted path, or regex. Returns matches plus definitions. Reach for it when you need every place a name appears. Required: query.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Identifier, dotted path, or regex (regex: true). Alias: \"symbol\".",
				},
				"symbol": map[string]any{
					"type":        "string",
					"description": "Alias for \"query\" — same field.",
				},
				"services": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Service IDs to search (default: all)",
				},
				"word": map[string]any{
					"type": "boolean",
				},
				"regex": map[string]any{
					"type": "boolean",
				},
				"case_insensitive": map[string]any{
					"type": "boolean",
				},
				"max": map[string]any{
					"type":        "integer",
					"description": "Maximum occurrences to return (default 25)",
				},
				"include_heuristic": map[string]any{
					"type":        "boolean",
					"description": "Also list name-contains matches too. Default false.",
				},
				"exact": map[string]any{
					"type":        "boolean",
					"description": "Keep only typed-confirmed hits",
				},
			},
			"required": []string{"query"},
		},
		group:   GroupInvestigation,
		aliases: map[string]string{"symbol": "query"},
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}

		var p struct {
			Query           string   `json:"query"`
			Services        []string `json:"services"`
			Word            bool     `json:"word"`
			Regex           bool     `json:"regex"`
			CaseInsensitive bool     `json:"case_insensitive"`
			Max             int      `json:"max"`
			// IncludeHeuristic (BP-26) restores the substring-containment
			// definition matching below. It is opt-in because a hit on
			// "FooBar" for query "Foo" is not a reference to Foo: the L0
			// oracle counted those separately and they are the definition
			// half's whole precision loss (docs/bench/README.md). SB-45
			// additively reuses it to add back the bare text occurrences
			// exact mode drops.
			IncludeHeuristic bool `json:"include_heuristic"`
			// Exact (SB-45) is a pointer so "not given" (nil, defaulted
			// below) is distinguishable from an explicit false.
			Exact *bool `json:"exact"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "references: invalid args"}
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(args, &raw); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "references: invalid args"}
		}
		query, aliasErr := resolveAliasedString(raw, "references", "query", "symbol")
		if aliasErr != nil {
			return nil, aliasErr
		}
		p.Query = query
		if p.Query == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "references: query required"}
		}
		if msg := refuseNonSymbolQuery(p.Query, p.Regex); msg != "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: msg}
		}

		maxOccs := p.Max
		if maxOccs <= 0 {
			maxOccs = referencesDefaultMax
		}

		// Resolve service list.
		svcs := p.Services
		if len(svcs) == 0 {
			all, err := backend.Services(ctx)
			if err != nil {
				return nil, &rpcError{Code: ErrCodeInternal, Message: "references: list services: " + err.Error()}
			}
			svcs = all
		}
		sort.Strings(svcs)

		opts := references.Options{
			Word:            p.Word,
			Regex:           p.Regex,
			CaseInsensitive: p.CaseInsensitive,
		}

		// SB-45: fetch each queried service's typed edges up front — once,
		// both to decide exact's default (true when ANY queried service's
		// typed pass produced edges) and to reuse below instead of asking
		// twice per service.
		typedByService := make(map[string][]core.Edge, len(svcs))
		exactDefault := false
		for _, svc := range svcs {
			if typed, tErr := backend.TypedEdges(ctx, svc); tErr == nil {
				typedByService[svc] = typed
				if len(typed) > 0 {
					exactDefault = true
				}
			}
		}
		exact := exactDefault
		if p.Exact != nil {
			exact = *p.Exact
		}

		// degradedEntry (O36, ADR-014 execute-and-flag): a service this
		// call could not fully answer for — its root vanished, or (SB-45)
		// its store holds no typed reference sites to confirm a mention
		// against — named with why, never silently dropped and never
		// aborting the whole fleet-wide call.
		type degradedEntry struct {
			Service string `json:"service"`
			Reason  string `json:"reason"`
		}

		var defs []defEntry
		var occs []occEntry
		var degraded []degradedEntry
		var implRows []implementationEntry
		truncated := false
		exactAvailable := true
		// RC-20: tallies definitions by package as they are found, so a
		// generic name (huge fan-in across many packages/services) can be
		// summarized instead of listed, without a second pass over nodes.
		byPkg := make(map[string]int)

		for _, svc := range svcs {
			// ── definitions: symbol index lookup ─────────────────────────────
			nodes, err := serviceSymbols(ctx, backend, svc)
			if err != nil {
				// O36: a service whose root vanished (or is otherwise
				// unreadable) must not fail the whole fleet-wide call —
				// skip it and name it in degraded.
				degraded = append(degraded, degradedEntry{Service: svc, Reason: "list symbols: " + err.Error()})
				continue
			}
			var defKeys []string
			for _, n := range nodes {
				isDef := false
				if n.Name == p.Query {
					isDef = true
				} else if p.IncludeHeuristic && !p.Word && !p.Regex {
					// Substring match on name — opt-in since BP-26.
					if strings.Contains(n.Name, p.Query) {
						isDef = true
					}
				}
				if isDef {
					tier := string(n.Tier)
					if tier == "" {
						tier = string(core.TierVerified)
					}
					conf := core.EdgeConfidenceHeuristic
					if tier == string(core.TierVerified) {
						conf = core.EdgeConfidenceResolved
					}
					defs = append(defs, defEntry{
						Service:        svc,
						Symbol:         n.Name,
						File:           n.FilePath,
						Line:           n.Line,
						Tier:           tier,
						Depth:          0,
						EdgeConfidence: string(conf),
					})
					defKeys = append(defKeys, n.Key())
					pkg := n.Package
					if pkg == "" {
						pkg = svc
					}
					byPkg[pkg]++
				}
			}

			// ── occurrences: source file scan ─────────────────────────────────
			if truncated {
				// Already at cap; skip further source scans.
				continue
			}
			sources, srcCov, err := collectSources(ctx, backend, svc)
			if err == nil && srcCov.LowerBound() {
				degraded = append(degraded, degradedEntry{Service: svc, Reason: partialNote(srcCov)})
			}
			if err != nil {
				// O36: same doctrine as ListAllSymbols above — a service
				// whose sources cannot be discovered degrades, never
				// aborts the whole call.
				degraded = append(degraded, degradedEntry{Service: svc, Reason: "sources: " + err.Error()})
				continue
			}

			// RC-21: query names an interface method (typed edges know the
			// receiver is an interface, but the method itself is never
			// indexed as its own symbol — see interfaceMethodQuery) — list
			// the implementing types first, ahead of the occurrences below.
			if implRows == nil {
				if iface, method, ok := interfaceMethodQuery(sources, nodes, p.Query); ok {
					if rows := implementationsForMethod(ctx, backend, svc, iface, method, nodes); len(rows) > 0 {
						implRows = rows
					}
				}
			}

			found, err := references.Find(p.Query, sources, opts)
			if err != nil {
				return nil, &rpcError{Code: ErrCodeParams, Message: "references: " + err.Error()}
			}
			// Typed-edge confirmation for mentions (BD-2): built lazily,
			// only when this service has both definitions of the query and
			// typed edges to confirm a mention against.
			typed := typedByService[svc]
			var mentionConf func(file string, line int) core.EdgeConfidence
			if len(defKeys) > 0 && len(typed) > 0 && len(found) > 0 {
				mentionConf = mentionConfidence(nodes, typed, defKeys)
			}
			// SB-45: exact mode keeps only occurrences mentionConf marks
			// resolved. A service that cannot even attempt confirmation
			// (no typed edges, or no indexed definition of the query here)
			// has no typed reference sites to filter to — fall back to its
			// unfiltered occurrences and say so, rather than silently
			// returning none for it (execute-and-flag, ADR-014).
			serviceExact := exact && mentionConf != nil
			if exact && mentionConf == nil && len(found) > 0 {
				exactAvailable = false
				reason := fmt.Sprintf("no typed reference sites for %q in %s", p.Query, svc)
				switch {
				case len(typed) == 0:
					reason += " (no typed edges)"
				case len(defKeys) == 0:
					reason += " (no indexed definition)"
				}
				degraded = append(degraded, degradedEntry{Service: svc, Reason: reason})
			}
			// Append the WHOLE service's matches — never break mid-service —
			// so totalOccs below is an honest count of what was actually
			// found in every service the scan reached, not a moving target
			// frozen mid-scan (a count over an incomplete manifest must be
			// reported as a lower bound, never silently rounded down further
			// than the scan itself already stopped).
			for _, occ := range found {
				conf := core.EdgeConfidenceHeuristic
				if mentionConf != nil {
					conf = mentionConf(occ.File, occ.Line)
				}
				if serviceExact && conf != core.EdgeConfidenceResolved && !p.IncludeHeuristic {
					// Exact mode: drop the bare text hit unless the caller
					// explicitly asked for heuristic occurrences back too.
					continue
				}
				occs = append(occs, occEntry{
					Service:        svc,
					File:           occ.File,
					Line:           occ.Line,
					Text:           occ.Text,
					Tier:           string(core.TierPreScan),
					Depth:          1,
					EdgeConfidence: string(conf),
				})
			}
			if len(occs) >= maxOccs {
				truncated = true
			}
		}

		// RC-20: a definition fan-in this large (a generic name matched
		// across many packages/services) would never fit even a paged
		// answer — return the shape of the answer instead of rows.
		// Occurrences are already hard-capped at maxOccs above, so
		// definitions (uncapped until now) are the runaway signal.
		if need, sum := scopeRequired(len(defs), scopeRequiredBudget, byPkg); need {
			msg := fmt.Sprintf(
				"%d definitions named %q exceed the budget — too many to page through; scope with \"services\", or a dotted path (\"pkg.%s\") instead of a bare identifier.",
				sum.Count, p.Query, p.Query,
			)
			return withSummary(capSummary(fmt.Sprintf("%d definitions named %q exceed the budget — too many to list; scope required", sum.Count, p.Query)), map[string]any{
				"query":          p.Query,
				"definitions":    []any{},
				"occurrences":    []any{},
				"count":          sum.Count,
				"edge_filter":    map[bool]string{true: "all", false: "resolved"}[p.IncludeHeuristic],
				"truncated":      true,
				"by_depth":       []any{},
				"resolution":     resolutionMix{},
				"exact":          exact,
				"scope_required": true,
				"top_packages":   sum.TopPackages,
				"message":        msg,
			}), nil
		}

		// SB-1/BA-1: ONE disclosure for both caps. totalOccs is every
		// occurrence found in the services actually scanned (a lower bound
		// on the whole fleet when the scan itself stopped early); occs is
		// then cut to maxOccs and, within that, to the byte budget — a
		// caller sees exactly one elided block naming how many of the
		// KNOWN total were dropped, never a `truncated:true` with nothing
		// to act on.
		totalOccs := len(occs)
		if len(occs) > maxOccs {
			occs = occs[:maxOccs]
		}
		occs, _ = fitToBudget(occs, budgetDefaults("references", s.budgets))
		var el *elided
		if len(occs) < totalOccs {
			el = elisionFor(len(occs), totalOccs,
				"service order, then source-scan order",
				"re-call with a larger \"max\", a larger budget, narrow \"services\", or tighten the query")
		}
		truncated = truncated || el != nil
		// by_depth counts what the caller sees: every definition plus the
		// occurrences that survived BOTH caps (maxOccs above, the budget
		// here), so the summary never claims rows the response elided.
		infos := make([]depthInfo, 0, len(defs)+len(occs))
		for _, d := range defs {
			infos = append(infos, depthInfo{Depth: d.Depth, Confidence: core.EdgeConfidence(d.EdgeConfidence)})
		}
		for _, o := range occs {
			infos = append(infos, depthInfo{Depth: o.Depth, Confidence: core.EdgeConfidence(o.EdgeConfidence)})
		}

		// SB-1: render occurrences compact for the wire, lifting service to
		// the envelope when every kept occurrence shares one (tier and
		// depth are ALWAYS uniform for occurrences — pre-scan/1 — and are
		// lifted unconditionally below).
		svcUniform := true
		uniformSvc := ""
		for i, o := range occs {
			if i == 0 {
				uniformSvc = o.Service
			} else if o.Service != uniformSvc {
				svcUniform = false
			}
		}
		wireOccs := make([]referenceOccurrence, 0, len(occs))
		for _, o := range occs {
			wireOccs = append(wireOccs, renderReferenceOccurrence(o, !svcUniform))
		}

		byDepth := withDepthBuckets(infos)
		resp := map[string]any{
			"query":       p.Query,
			"definitions": defs,
			"occurrences": wireOccs,
			"count":       len(wireOccs),
			// Which definition matching answered (BP-26): "resolved" =
			// exact-name definitions only, "all" = plus substring hits.
			"edge_filter": map[bool]string{true: "all", false: "resolved"}[p.IncludeHeuristic],
			"truncated":   truncated || el != nil,
			"by_depth":    byDepth,
			// resolution (VB-19): the same by_depth counts, summed to one
			// envelope-level typed/heuristic mix.
			"resolution": resolutionFromDepthBuckets(byDepth),
			// SB-45: whether occurrences answered from the typed pass only.
			"exact": exact,
		}
		// index_commit (VB-19): lifted only when every SCANNED service (not
		// just the ones with a surviving definition/occurrence) resolves to
		// the same commit — references is fleet-wide, so a mixed-commit
		// scan has no single honest watermark to report, mirroring this
		// tool's own occurrence_service uniform-lift discipline above.
		if ic, uniform := uniformIndexCommit(ctx, backend, svcs); uniform && ic != "" {
			resp["index_commit"] = ic
		}
		if exact {
			// exact_available is false when at least one scanned service
			// had no typed reference sites to confirm against and fell
			// back to its unfiltered occurrences (named in degraded).
			resp["exact_available"] = exactAvailable
		}
		if len(degraded) > 0 {
			resp["degraded"] = degraded
		}
		if len(implRows) > 0 {
			// RC-21: same per-tool budget occurrences fit inside — a handful
			// of implementing types never approaches it, but an interface
			// with an unusually large implementer set must not blow past
			// the 16 KB discipline this reused it to protect in the first
			// place.
			implRows, _ = fitToBudget(implRows, budgetDefaults("references", s.budgets))
			resp["implementations"] = implRows
		}
		if len(occs) > 0 {
			resp["occurrence_tier"] = string(core.TierPreScan)
			resp["occurrence_depth"] = 1
			if svcUniform {
				resp["occurrence_service"] = uniformSvc
			}
		}
		if el != nil {
			resp["elided"] = el
		}
		return withSummary(referencesSummary(p.Query, defs, wireOccs), resp), nil
	})
}

// referencesSummary renders references' answer-first sentence (wave L1):
// counts by kind (definitions, occurrences) with the first three locations
// — built from defs and wireOccs, the exact (post-budget) lists the
// "definitions"/"occurrences" fields return.
func referencesSummary(query string, defs []defEntry, occs []referenceOccurrence) string {
	summary := fmt.Sprintf("%d definition%s, %d occurrence%s of %q", len(defs), pluralS(len(defs)), len(occs), pluralS(len(occs)), query)
	var locs []string
	for _, d := range defs {
		if len(locs) >= 3 {
			break
		}
		locs = append(locs, fmt.Sprintf("%s:%d", d.File, d.Line))
	}
	for _, o := range occs {
		if len(locs) >= 3 {
			break
		}
		locs = append(locs, o.Loc)
	}
	if len(locs) > 0 {
		summary += "; first: " + strings.Join(locs, ", ")
	}
	return capSummary(summary)
}
