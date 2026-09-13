package mcp

import (
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// ─── output schemas (RP-11, docs/plans/reach-parity.md) ─────────────────────
//
// Every agent-surface tool advertises an `outputSchema` in tools/list and
// returns its result as `structuredContent` beside the text block on
// tools/call (MCP 2025-06-18+), so a non-Claude client can machine-check
// the envelope. The schemas are GENERATED from Go result types by
// reflection (outputSchemaFor → schemaOf): the tools that already return a
// typed struct (query_symbols, memory_search) contribute it directly; the
// map-returning tools declare a result type below whose fields mirror the
// keys the handler emits — `omitempty` marks a key the handler emits
// conditionally, everything else is `required`. The spec's rule is
// "Servers MUST provide structured results that conform to this schema",
// so TestOutputSchemasMatchLiveResults calls every tool and validates its
// live result against the schema it advertises; a key the handler adds
// but the type omits fails that test, never the client.
//
// Root schemas stay `type: object` (the 2025-11-25 restriction) and permit
// additional properties: fields are additive by contract (law 4), and a
// client pinned to an older schema must keep parsing a newer response.

// findCallersResult mirrors find_callers' payload (tools.go). Callers is
// the SB-47 compact wire shape; CallerService/CallerTier/CallerDepth/
// CallerEdgeConfidence are the envelope lift, present only when every
// listed caller shares one (see uniformCallerFields).
type findCallersResult struct {
	Callers              []callerEntry  `json:"callers"`
	ByDepth              []depthBucket  `json:"by_depth"`
	TransitiveCallers    []depthRow     `json:"transitive_callers,omitempty"`
	TypedCallers         []typedEdgeRef `json:"typed_callers,omitempty"`
	CrossServiceCallers  []any          `json:"cross_service_callers,omitempty"`
	CallerService        string         `json:"caller_service,omitempty"`
	CallerTier           string         `json:"caller_tier,omitempty"`
	CallerDepth          int            `json:"caller_depth,omitempty"`
	CallerEdgeConfidence string         `json:"caller_edge_confidence,omitempty"`
}

// typedEdgeRef is one kind-labeled typed edge ({key, kind}).
type typedEdgeRef struct {
	Key  string `json:"key"`
	Kind string `json:"kind"`
}

type entrypointsResult struct {
	Entrypoints []symbolNodeDTO `json:"entrypoints"`
}

type outlineResult struct {
	Symbols []symbolNodeDTO `json:"symbols"`
}

type findSymbolResult struct {
	Symbols         []symbolNodeDTO       `json:"symbols"`
	MatchedVia      string                `json:"matched_via"`
	DidYouMean      []any                 `json:"did_you_mean,omitempty"`
	Implementations []implementationEntry `json:"implementations,omitempty"`
}

type ingestResult struct {
	Service           string `json:"service"`
	OK                bool   `json:"ok"`
	UnclassifiedFiles int    `json:"unclassified_files,omitempty"`
	OversizeFiles     int    `json:"oversize_files,omitempty"`
}

type listServicesResult struct {
	Services []listServicesEntry `json:"services"`
	Count    int                 `json:"count"`
}

type listServicesEntry struct {
	Service     string   `json:"service"`
	SymbolCount int      `json:"symbol_count"`
	Languages   []string `json:"languages"`
	Path        string   `json:"path,omitempty"`
}

type obligationsResult struct {
	Service             string `json:"service"`
	File                string `json:"file,omitempty"`
	Rules               []any  `json:"rules,omitempty"`
	CompiledAtWatermark string `json:"compiled_at_watermark,omitempty"`
	Displaced           int    `json:"displaced,omitempty"`
	DisplacedRepo       int    `json:"displaced_repo,omitempty"`
	DisplacedModule     int    `json:"displaced_module,omitempty"`
	ModuleNotes         []any  `json:"module_notes,omitempty"`
	IndexStale          bool   `json:"index_stale,omitempty"`
	ClaimID             string `json:"claim_id,omitempty"`
	Action              string `json:"action,omitempty"`
}

type packContextResult struct {
	Included []packContextItem `json:"included"`
	Tokens   int               `json:"tokens"`
	Dropped  int               `json:"dropped"`
	Budget   int               `json:"budget"`
}

type packContextItem struct {
	ID    string  `json:"id"`
	Text  string  `json:"text"`
	Score float64 `json:"score"`
}

type readSourceResult struct {
	File       string       `json:"file"`
	Lines      []sourceLine `json:"lines"`
	StartLine  int          `json:"start_line"`
	EndLine    int          `json:"end_line"`
	TotalLines int          `json:"total_lines"`
	ClampedEOF bool         `json:"clamped_eof"`
	Note       string       `json:"note,omitempty"`
	Elided     *elided      `json:"elided,omitempty"`
}

type referencesResult struct {
	Query             string                `json:"query"`
	Definitions       []referenceDefinition `json:"definitions"`
	Occurrences       []referenceOccurrence `json:"occurrences"`
	Count             int                   `json:"count"`
	EdgeFilter        string                `json:"edge_filter"`
	Truncated         bool                  `json:"truncated"`
	ByDepth           []depthBucket         `json:"by_depth"`
	Elided            *elided               `json:"elided,omitempty"`
	OccurrenceTier    string                `json:"occurrence_tier,omitempty"`
	OccurrenceDepth   int                   `json:"occurrence_depth,omitempty"`
	OccurrenceService string                `json:"occurrence_service,omitempty"`
	// Exact/ExactAvailable (SB-45): whether occurrences answered from the
	// typed pass only, and whether every scanned service could actually
	// confirm against one (false when any fell back — see Degraded).
	Exact          bool `json:"exact"`
	ExactAvailable bool `json:"exact_available,omitempty"`
	// Degraded (SB-45, O36 — ADR-014 execute-and-flag): services this call
	// could not fully answer for — an unreadable root, or no typed
	// reference sites to confirm exact mode against — named with why,
	// never silently dropped and never aborting the whole call.
	Degraded []referenceDegradedService `json:"degraded,omitempty"`
}

// referenceDegradedService is one references degraded-service row (O36,
// SB-45).
type referenceDegradedService struct {
	Service string `json:"service"`
	Reason  string `json:"reason"`
}

type referenceDefinition struct {
	Service        string `json:"service"`
	Symbol         string `json:"symbol"`
	File           string `json:"file"`
	Line           int    `json:"line"`
	Tier           string `json:"tier"`
	Depth          int    `json:"depth"`
	EdgeConfidence string `json:"edge_confidence"`
}

// referenceOccurrence is one compact occurrence row on the wire (SB-1): a
// combined "file:line" location, text trimmed to fit the 100-byte cap, a
// one-character confidence code, and an optional per-row service — set
// only when a response's occurrences span more than one service (the
// common single-service case lifts it to the envelope instead; see
// renderReferenceOccurrence).
type referenceOccurrence struct {
	Loc  string `json:"loc"`
	Text string `json:"text"`
	C    string `json:"c"`
	Svc  string `json:"svc,omitempty"`
}

type repoMapResult struct {
	Map    string `json:"map"`
	Budget int    `json:"budget"`
}

type stalenessReportResult struct {
	Service             string   `json:"service"`
	SourceCount         int      `json:"source_count"`
	Sources             []string `json:"sources"`
	IndexedCommit       string   `json:"indexed_commit"`
	CurrentCommit       string   `json:"current_commit"`
	ReingestRecommended bool     `json:"reingest_recommended"`
	// TypedEdges (BP-35): {known, packages, failed, reason} — whether this
	// daemon's typed Go call-edge pass ran for the service, and why not
	// when it did not. Absent when the backend cannot report it.
	TypedEdges any `json:"typed_edges,omitempty"`
}

type symbolContextResult struct {
	Def                 symbolNodeDTO `json:"def"`
	Callers             []string      `json:"callers"`
	Callees             []string      `json:"callees"`
	RelatedTests        []string      `json:"related_tests"`
	Snippet             []sourceLine  `json:"snippet,omitempty"`
	SnippetTruncated    bool          `json:"snippet_truncated,omitempty"`
	TypedEdges          any           `json:"typed_edges,omitempty"`
	CrossServiceCallees []any         `json:"cross_service_callees,omitempty"`
	CrossServiceCallers []any         `json:"cross_service_callers,omitempty"`
}

type verifyPlanResult struct {
	Service string            `json:"service"`
	Summary verifyPlanSummary `json:"summary"`
	Tasks   []verifyPlanTask  `json:"tasks"`
}

type verifyPlanSummary struct {
	Total   int `json:"total"`
	Pass    int `json:"pass"`
	Fail    int `json:"fail"`
	Unknown int `json:"unknown"`
}

type verifyPlanTask struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Label    string `json:"label"`
	Criteria []any  `json:"criteria"`
}

type capabilitiesResult struct {
	Tools             []string `json:"tools"`
	ActiveSurface     string   `json:"active_surface"`
	ActiveProfile     string   `json:"active_profile"`
	ToolCount         int      `json:"tool_count"`
	ExtractionEngines any      `json:"extraction_engines"`
	SchemaBytes       int      `json:"schema_bytes"`
}

// agentResultTypes maps each agent-surface tool to its result type — the
// single source outputSchemaFor generates from. Adding a tool to
// companionTools without a row here fails TestOutputSchemasMatchLiveResults.
var agentResultTypes = map[string]reflect.Type{
	"capabilities":     reflect.TypeOf(capabilitiesResult{}),
	"entrypoints":      reflect.TypeOf(entrypointsResult{}),
	"find_callers":     reflect.TypeOf(findCallersResult{}),
	"find_symbol":      reflect.TypeOf(findSymbolResult{}),
	"ingest":           reflect.TypeOf(ingestResult{}),
	"list_services":    reflect.TypeOf(listServicesResult{}),
	"memory_search":    reflect.TypeOf(memorySearchResult{}),
	"obligations":      reflect.TypeOf(obligationsResult{}),
	"outline":          reflect.TypeOf(outlineResult{}),
	"pack_context":     reflect.TypeOf(packContextResult{}),
	"query_symbols":    reflect.TypeOf(symbolQueryResult{}),
	"read_source":      reflect.TypeOf(readSourceResult{}),
	"references":       reflect.TypeOf(referencesResult{}),
	"repo_map":         reflect.TypeOf(repoMapResult{}),
	"staleness_report": reflect.TypeOf(stalenessReportResult{}),
	"symbol_context":   reflect.TypeOf(symbolContextResult{}),
	"verify_plan":      reflect.TypeOf(verifyPlanResult{}),
}

// outputSchemaFor returns the JSON Schema (2025-11-25 `outputSchema` shape:
// an object root with properties/required) generated from the tool's Go
// result type, or nil for a tool without a declared result type (the long
// tail on bare /rpc advertises none rather than a guessed one).
func outputSchemaFor(name string) map[string]any {
	t, ok := agentResultTypes[name]
	if !ok {
		return nil
	}
	return schemaOf(t)
}

var timeType = reflect.TypeOf(time.Time{})

// schemaOf renders a Go type as JSON Schema: structs become objects whose
// properties follow the json tags (omitempty/omitzero → optional; embedded
// structs inline, matching encoding/json), slices arrays, maps objects with
// additionalProperties, time.Time an RFC 3339 string, interfaces the
// unconstrained schema `{}`. Property names are emitted sorted so the
// schema — and therefore the per-session schema budget — is byte-stable.
func schemaOf(t reflect.Type) map[string]any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch {
	case t == timeType:
		return map[string]any{"type": "string", "format": "date-time"}
	case t.Kind() == reflect.Struct:
		props := map[string]any{}
		var required []string
		collectFields(t, props, &required)
		sort.Strings(required)
		out := map[string]any{"type": "object", "properties": props}
		if len(required) > 0 {
			out["required"] = required
		}
		return out
	case t.Kind() == reflect.Slice || t.Kind() == reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return map[string]any{"type": "string"}
		}
		return map[string]any{"type": "array", "items": schemaOf(t.Elem())}
	case t.Kind() == reflect.Map:
		return map[string]any{"type": "object", "additionalProperties": schemaOf(t.Elem())}
	case t.Kind() == reflect.String:
		return map[string]any{"type": "string"}
	case t.Kind() == reflect.Bool:
		return map[string]any{"type": "boolean"}
	case t.Kind() >= reflect.Int && t.Kind() <= reflect.Uintptr:
		return map[string]any{"type": "integer"}
	case t.Kind() == reflect.Float32 || t.Kind() == reflect.Float64:
		return map[string]any{"type": "number"}
	default:
		return map[string]any{}
	}
}

// collectFields walks t's exported fields into props/required, inlining
// anonymous embedded structs the way encoding/json does.
func collectFields(t reflect.Type, props map[string]any, required *[]string) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "-" || (!f.IsExported() && !f.Anonymous) {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		if f.Anonymous && name == "" {
			ft := f.Type
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				collectFields(ft, props, required)
				continue
			}
		}
		if name == "" {
			name = f.Name
		}
		props[name] = schemaOf(f.Type)
		optional := strings.Contains(opts, "omitempty") || strings.Contains(opts, "omitzero") || f.Type.Kind() == reflect.Pointer
		if !optional {
			*required = append(*required, name)
		}
	}
}

// validateAgainstSchema checks a decoded JSON value against a schemaOf
// schema: required properties present, declared properties of the right
// JSON type, arrays and nested objects recursively. Extra properties are
// allowed (additive contract). Returns the first violation as a path +
// message, or "" when the value conforms. Used by the schema test and by
// nothing at dispatch time — the server never validates its own output on
// the hot path.
func validateAgainstSchema(path string, schema map[string]any, v any) string {
	typ, _ := schema["type"].(string)
	switch typ {
	case "object":
		m, ok := v.(map[string]any)
		if !ok {
			return path + ": want object, got " + jsonKind(v)
		}
		if req, ok := schema["required"].([]string); ok {
			for _, r := range req {
				if _, present := m[r]; !present {
					return path + ": required property \"" + r + "\" missing"
				}
			}
		}
		props, _ := schema["properties"].(map[string]any)
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if ps, ok := props[k].(map[string]any); ok {
				if m[k] == nil {
					continue // null for an optional/nullable field
				}
				if msg := validateAgainstSchema(path+"."+k, ps, m[k]); msg != "" {
					return msg
				}
			} else if ap, ok := schema["additionalProperties"].(map[string]any); ok && m[k] != nil {
				if msg := validateAgainstSchema(path+"."+k, ap, m[k]); msg != "" {
					return msg
				}
			}
		}
	case "array":
		arr, ok := v.([]any)
		if !ok {
			return path + ": want array, got " + jsonKind(v)
		}
		items, _ := schema["items"].(map[string]any)
		for i, it := range arr {
			if it == nil {
				continue
			}
			if msg := validateAgainstSchema(path+"["+itoa(i)+"]", items, it); msg != "" {
				return msg
			}
		}
	case "string":
		if _, ok := v.(string); !ok {
			return path + ": want string, got " + jsonKind(v)
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return path + ": want boolean, got " + jsonKind(v)
		}
	case "integer", "number":
		if _, ok := v.(float64); !ok {
			return path + ": want " + typ + ", got " + jsonKind(v)
		}
	}
	return ""
}

func jsonKind(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return reflect.TypeOf(v).String()
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

// compile-time: the typed result structs the handlers already return are
// the ones the schema table names.
var (
	_ = core.EdgeConfidenceResolved
	_ symbolQueryResult
	_ memorySearchResult
)
