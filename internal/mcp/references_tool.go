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
// confidence code, GUARANTEED by trimming the text (never the location or
// confidence, which are exact facts) until the marshaled row fits — rather
// than a fixed byte split per field that could silently drift from
// encoding/json's actual output.
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
// byte cap, and confidence collapsed to a single character. includeService
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
		Description: "Fleet-wide occurrence search: find every mention of a query string across code, docs, and config for one or more services. query must be an identifier (\"FindMe\"), a dotted path (\"pkg.Sym\" or \"Type.Method\"), or a declared regular expression (regex: true) — a free-text fragment (e.g. \"func (s *\") is refused, naming the three accepted forms. Returns structured occurrences (text matches, tier=pre-scan) plus symbol definitions found in the index (tier from the symbol's own evidence tier). Definitions are exact-name matches by default; include_heuristic: true also lists symbols whose name merely contains the query. Occurrences are unaffected. Each occurrence is rendered compact for the wire — {loc: \"file:line\", text (trimmed to 60 chars), c: \"r\"|\"h\"} at 100 bytes or under — with service, tier (always pre-scan) and depth (always 1) lifted to the envelope (occurrence_service, occurrence_tier, occurrence_depth) when uniform across every occurrence rather than repeated per row; a search whose results span several services keeps a per-row \"svc\" instead. Definitions keep the fuller {service,symbol,file,line,tier,depth,edge_confidence} shape. Default max is 25 occurrences; the elided block names how to get more. Every row carries depth (0 = a definition, 1 = a mention) and edge_confidence (resolved = the mention's enclosing symbol has a typed call edge to the definition, or the definition is at the verified tier; heuristic = text match only), summarised in by_depth. Reach for it when you need every place a name appears — docs and config included — not just the call graph. Required: query.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "String to search for: an identifier, a dotted path (\"pkg.Sym\" or \"Type.Method\"), or a regex (with regex: true). Alias: \"symbol\" (additive — same field, either name works; both may be given only if they agree).",
				},
				"symbol": map[string]any{
					"type":        "string",
					"description": "Alias for \"query\" (P1-1: the hot-tool quartet's primary argument is named differently per tool; this lets a caller reuse the same field name across all four).",
				},
				"services": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Service IDs to search (default: all known services)",
				},
				"word": map[string]any{
					"type":        "boolean",
					"description": "Whole-word match only",
				},
				"regex": map[string]any{
					"type":        "boolean",
					"description": "Treat query as a regular expression",
				},
				"case_insensitive": map[string]any{
					"type":        "boolean",
					"description": "Case-insensitive matching",
				},
				"max": map[string]any{
					"type":        "integer",
					"description": "Maximum total occurrences to return (default 25)",
				},
				"include_heuristic": map[string]any{
					"type":        "boolean",
					"description": "Also list definitions whose name merely CONTAINS the query (\"FooBar\" for \"Foo\"). Default false; occurrences are unaffected.",
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
			// half's whole precision loss (docs/bench/README.md).
			IncludeHeuristic bool `json:"include_heuristic"`
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

		var defs []defEntry
		var occs []occEntry
		truncated := false

		for _, svc := range svcs {
			// ── definitions: symbol index lookup ─────────────────────────────
			nodes, err := backend.ListAllSymbols(ctx, svc)
			if err != nil {
				return nil, &rpcError{Code: ErrCodeInternal, Message: "references: list symbols " + svc + ": " + err.Error()}
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
				}
			}

			// ── occurrences: source file scan ─────────────────────────────────
			if truncated {
				// Already at cap; skip further source scans.
				continue
			}
			sources, err := backend.Sources(ctx, svc)
			if err != nil {
				return nil, &rpcError{Code: ErrCodeInternal, Message: "references: sources " + svc + ": " + err.Error()}
			}
			found, err := references.Find(p.Query, sources, opts)
			if err != nil {
				return nil, &rpcError{Code: ErrCodeParams, Message: "references: " + err.Error()}
			}
			// Typed-edge confirmation for mentions (BD-2): built lazily,
			// only when this service has both definitions of the query and
			// typed edges to confirm a mention against.
			var mentionConf func(file string, line int) core.EdgeConfidence
			if len(defKeys) > 0 && len(found) > 0 {
				if typed, tErr := backend.TypedEdges(ctx, svc); tErr == nil && len(typed) > 0 {
					mentionConf = mentionConfidence(nodes, typed, defKeys)
				}
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

		resp := map[string]any{
			"query":       p.Query,
			"definitions": defs,
			"occurrences": wireOccs,
			"count":       len(wireOccs),
			// Which definition matching answered (BP-26): "resolved" =
			// exact-name definitions only, "all" = plus substring hits.
			"edge_filter": map[bool]string{true: "all", false: "resolved"}[p.IncludeHeuristic],
			"truncated":   truncated || el != nil,
			"by_depth":    withDepthBuckets(infos),
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
		return resp, nil
	})
}
