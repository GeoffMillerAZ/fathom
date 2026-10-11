// Package mcp — meta-tools, prompts, and resources.
//
// Meta tools (status, staleness_report) are registered in GroupShared.
// status (SR-23, F-11, design §4) merges the retired capabilities/
// server_stats/list_sources tools behind one `section` argument,
// byte-compatible per section (registerStatus below).
//
// MCP protocol endpoints:
//   - prompts/list + prompts/get  (4 canned prompts)
//   - resources/list + resources/read
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/docdrift"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/docindex"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/refresh"
)

// currentGitHead returns HEAD for path, or "" when path is in no work tree
// or HEAD cannot be read. RT-101: read from the ref files without a fork
// (gitro.Head, cached per root by mtime) — this runs on every per-service
// read through auto-refresh.
func currentGitHead(ctx context.Context, path string) string {
	if path == "" {
		return ""
	}
	sha, err := gitro.Head(ctx, path)
	if err != nil {
		return ""
	}
	return sha
}

// ─── registerMeta ─────────────────────────────────────────────────────────────

// registerMeta wires meta-tools, prompts, and resources into s.
func registerMeta(s *Server, backend QueryInterface) {
	registerStatus(s, backend)
	registerStalenessReport(s, backend)
	registerPromptsHandlers(s)
	registerResourcesHandlers(s, backend)
}

// ─── status ───────────────────────────────────────────────────────────────────

// statusToolsSection is status(section:"tools")'s answer — byte-compatible
// with the retired `capabilities` tool (SR-23, F-11, design §4).
func statusToolsSection(ctx context.Context, s *Server) map[string]any {
	descs := s.visibleToolsCtx(ctx)
	names := make([]any, len(descs))
	for i, d := range descs {
		names[i] = d.Name
	}
	return map[string]any{
		"tools":              names,
		"active_surface":     s.effectiveSurface(ctx),
		"active_profile":     s.activeProfile(),
		"tool_count":         len(descs),
		"extraction_engines": extractor.EngineProvenance(),
		"schema_bytes":       s.surfaceSchemaBytes(ctx),
	}
}

// statusDaemonSection is status(section:"daemon")'s answer — byte-compatible
// with the retired `server_stats` tool (SR-23, F-11, design §4).
func statusDaemonSection(ctx context.Context, s *Server, args json.RawMessage) map[string]any {
	uptime := time.Since(s.startTime)
	result := map[string]any{
		"uptime_seconds": uptime.Seconds(),
		"request_count":  s.totalRequests(),
		"started_at":     s.startTime.UTC().Format(time.RFC3339),
	}
	if sb, ok := s.backend.(statsBackend); ok {
		unclassified, oversize := sb.IngestStats()
		result["unclassified_files"] = unclassified
		result["oversize_files"] = oversize
	}
	if cz, ok := s.backend.(classifiedZeroBackend); ok {
		if m := cz.ClassifiedZeroStats(); len(m) > 0 {
			result["classified_zero_files"] = m
		}
	}
	// GL-27 (G6): whether the daemon's compile_layer is usable, and why
	// not, so doctor can FAIL a daemon that started with it disabled
	// instead of the single start-time WARN being the only record.
	if cs, ok := s.backend.(compileStateBackend); ok {
		// All zero is a wrapper whose backend cannot say: omitted, so
		// doctor reads "the daemon predates this" instead of a false off.
		if enabled, configured, reason := cs.CompileLayerState(); enabled || configured || reason != "" {
			result["compile_layer"] = map[string]any{"enabled": enabled, "configured": configured, "reason": reason}
		}
	}
	// Emitted whenever a call logger is wired — an EMPTY block is the
	// truthful "instrument live, zero traffic" state a freshly
	// restarted daemon is in; omission means the instrument itself is
	// absent (nil receiver returns nil). Doctrine 7: an existence
	// check must be able to tell those apart.
	if traffic := s.callLogger.SurfaceTraffic(); traffic != nil {
		result["surface_traffic"] = traffic
	}

	var p struct {
		Service string `json:"service"`
	}
	_ = json.Unmarshal(args, &p) // best-effort: absent/invalid args just skip the service block

	// Typed Go call-edge counters (ADR-021 §3): a Go service whose typed
	// pass never ran (predates ADR-021 §3, or hasn't been re-ingested
	// since) reports all-zero counts here — the visible signal ExtractorEpoch
	// intentionally does NOT force (typed edges are new observations, not
	// a Calls-semantics change, see docs/plans/callgraph-types.md).
	// Note dispatch counts skew high in Go services with many error
	// types: universe/stdlib interface dispatch (error.Error) fans out
	// to every first-party implementer (see internal/fusion/typedcalls.go).
	if p.Service != "" && s.backend != nil {
		edges, err := s.backend.TypedEdges(ctx, p.Service)
		if err == nil {
			counts := map[string]int{"direct": 0, "iface_dispatch": 0, "unresolved": 0}
			total := 0
			for _, e := range edges {
				if e.Kind == core.EdgeInstantiates {
					continue // instantiation facts (EW-15) are not call edges
				}
				total++
				switch e.Kind {
				case "direct":
					counts["direct"]++
				case "iface-dispatch":
					counts["iface_dispatch"]++
				case "unresolved":
					counts["unresolved"]++
				}
			}
			result["typed_edges"] = map[string]any{
				"service":        p.Service,
				"total":          total,
				"direct":         counts["direct"],
				"iface_dispatch": counts["iface_dispatch"],
				"unresolved":     counts["unresolved"],
			}
		}
	}

	return result
}

// statusSourcesSection is status(section:"sources")'s answer — byte-
// compatible with the retired `list_sources` tool (SR-23, F-11,
// design §4). A required service, unlike the other two sections: it lists
// THAT service's own files.
func statusSourcesSection(ctx context.Context, backend QueryInterface, service string) (map[string]any, *rpcError) {
	if backend == nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
	}
	if service == "" {
		return nil, &rpcError{Code: ErrCodeParams, Message: `status: section "sources" requires service`}
	}
	nodes, err := serviceSymbols(ctx, backend, service)
	if err != nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
	}
	seen := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		if n.FilePath != "" {
			seen[n.FilePath] = struct{}{}
		}
	}
	sources := make([]string, 0, len(seen))
	for f := range seen {
		sources = append(sources, f)
	}
	sort.Strings(sources)
	return map[string]any{"sources": stringsToAny(sources)}, nil
}

// registerStatus is the SR-23 merge (F-11, design §4): one tool answering
// what the retired capabilities/server_stats/list_sources tools each
// answered, byte-compatible per section — `list_services` (37,155 calls,
// hooks and the verify CLI) stays a separate tool, unaffected.
func registerStatus(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "status",
		Description: "One status endpoint for the retired capabilities/server_stats/list_sources tools: tools (this surface's callable set, active surface/profile, schema_bytes), daemon (uptime, request count, ingest/typed-edge counters), or sources (a service's ingested file list). Required: section.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"section": map[string]any{"type": "string", "enum": []string{"tools", "daemon", "sources"}, "description": `Which retired tool's answer to return: "tools" (capabilities), "daemon" (server_stats), "sources" (list_sources, requires service)`},
				"service": map[string]any{"type": "string", "description": "Service / repo identifier — required for section \"sources\"; optional for \"daemon\" (adds typed_edges)"},
			},
			"required": []string{"section"},
		},
		group: GroupShared,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		var p struct {
			Section string `json:"section"`
			Service string `json:"service"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "status: invalid args"}
		}
		switch p.Section {
		case "tools":
			return statusToolsSection(ctx, s), nil
		case "daemon":
			return statusDaemonSection(ctx, s, args), nil
		case "sources":
			return statusSourcesSection(ctx, backend, p.Service)
		default:
			return nil, &rpcError{Code: ErrCodeParams, Message: `status: section must be "tools", "daemon", or "sources"`}
		}
	})
}

// ─── server_stats (retired, SR-23 — see registerStatus/statusDaemonSection) ───

// statsBackend is the optional backend capability for the ADR-021 §2 / TI-6
// process-lifetime unclassified/oversize counters (fusion.Pipeline.Stats,
// via pkg/client.Client.IngestStats). Test fakes may omit it — server_stats
// and the ingest tool simply omit those fields when it is absent.
type statsBackend interface {
	IngestStats() (unclassified, oversize int64)
}

// exclusionsBackend is the optional backend capability that names the
// directories the connector's default cache list pruned from the most
// recent discovery of a service (RT-145, I38) — carried on the ingest
// tool's response as default_ignored_dirs, as the CLI summary does.
type exclusionsBackend interface {
	DefaultIgnoredDirs(service string) []string
}

// compileStateBackend is the optional backend capability reporting whether
// compile_layer can spend (an LLM client was built) and, when not, whether a
// provider was configured and the reason (pkg/client.Client.CompileLayerState).
type compileStateBackend interface {
	CompileLayerState() (enabled, configured bool, reason string)
}

// classifiedZeroBackend is the optional backend capability for the per-tool
// classified-but-zero-symbols counters (ADR-021 §2, critic F5) — files that
// classified as a known tool but whose extractor emitted zero symbols.
type classifiedZeroBackend interface {
	ClassifiedZeroStats() map[string]int64
}

// typedEdgeStatusBackend is the optional backend capability for the
// per-service typed Go call-edge pass outcome (BP-35,
// internal/fusion/typedenv.go, via pkg/client.Client.TypedEdgeStatus).
// staleness_report reports it so "is the graph I am querying typed?" is
// answerable from the same call that answers "is it fresh?" — the two
// questions failed together on 2026-09-12 and only one of them was
// askable.
type typedEdgeStatusBackend interface {
	TypedEdgeStatus(service string) (fusion.TypedEdgeStatus, bool)
}

// fleetRootStatusBackend is the optional FleetBackend capability (DG-18,
// diagnostics sweep) for reporting that a service's recorded corpus root
// (ServiceMeta.Path, set at ingest time) no longer exists on disk.
// list_services surfaces this as a structured "root_missing": true field
// plus remedy instead of an error line — a dangling service record (e.g.
// a cleaned-up scratch clone) is common enough that a human reading
// list_services should be told what to do about it, not just that
// something failed.
type fleetRootStatusBackend interface {
	RootMissing(service string) (missing bool, remedy string)
}

// autoRefreshStatsBackend is the optional backend capability for the
// auto-refresh wrapper's process-lifetime re-ingest counter (O108,
// autorefresh.go). Present only when the backend passed to NewServer* is
// (or wraps) an *autoRefreshBackend — i.e. serve-http's default
// --auto-refresh=true configuration. Consumed by cmd/fathom's /metrics as
// fathom_autorefresh_total, via Server.AutoRefreshTotal.
type autoRefreshStatsBackend interface {
	AutoRefreshTotal() int64
}

// stalenessDocDrift computes staleness_report's additive doc_drift section
// (SR-28, F-56, design §4): the retired stale_docs tool's own scan — a
// Markdown inline code span or link target naming a symbol or file the
// index no longer knows — folded into the one staleness answer instead of
// a second tool call. nodes is the caller's already-fetched symbol list
// (serviceSymbols), reused rather than queried twice.
//
// Errors and an empty corpus are both silent (nil): a scan the tree budget
// cut short, or a backend with no source access at all, must never turn
// "is my index stale" into a failure — the caller's own source_count/
// sources fields already disclose a partial scan when one happened.
func stalenessDocDrift(ctx context.Context, backend QueryInterface, service string, nodes []core.SymbolNode) []any {
	files, _, rpcErr := fetchSources(ctx, backend, service)
	if rpcErr != nil {
		return nil
	}
	knownSymbols := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		knownSymbols[n.Name] = true
	}
	knownFiles := make(map[string]bool, len(files))
	for _, f := range files {
		knownFiles[f.Path] = true
	}
	var allDrifts []docdrift.Drift
	for _, f := range files {
		if f.Lang != "md" {
			continue
		}
		allDrifts = append(allDrifts, docdrift.Analyze(f.Path, string(f.Content), knownSymbols, knownFiles)...)
	}
	sort.Slice(allDrifts, func(i, j int) bool {
		if allDrifts[i].File != allDrifts[j].File {
			return allDrifts[i].File < allDrifts[j].File
		}
		if allDrifts[i].Line != allDrifts[j].Line {
			return allDrifts[i].Line < allDrifts[j].Line
		}
		return allDrifts[i].Ref < allDrifts[j].Ref
	})
	result := make([]any, len(allDrifts))
	for i, d := range allDrifts {
		result[i] = map[string]any{"file": d.File, "line": d.Line, "ref": d.Ref}
	}
	return result
}

// docDriftBackend is the optional backend capability KR-7's doc_link_drift
// needs: docindex.StalenessDocDrift over the service's still-open doc_links
// (KR-6), the same optional-capability pattern docsSearchBackend (KR-5)
// already uses. pkg/client implements it; a backend that omits it (a bare
// fake in a unit test, or any backend not wired to a store) simply gets no
// doc_link_drift key — never an empty list standing in for "checked, found
// nothing" (I38), since "checked" is exactly what did not happen.
type docDriftBackend interface {
	DocLinkDrift(ctx context.Context, service string) ([]docindex.DriftedDocLink, error)
}

// renderDocLinkDrift is KR-7's response shape for staleness_report's
// doc_link_drift key: one object per drifted span, naming where it lives,
// what it names, what changed, and — when the commit that moved the
// target is on record — which one and when. §11: never a "wrong" verdict,
// only what moved.
func renderDocLinkDrift(drifted []docindex.DriftedDocLink) []any {
	out := make([]any, len(drifted))
	for i, d := range drifted {
		row := map[string]any{
			"path":         d.Path,
			"heading":      d.HeadingPath,
			"target_kind":  d.TargetKind,
			"target":       d.Target,
			"line":         d.Line,
			"linked_hash":  d.LinkedHash,
			"current_hash": d.CurrentHash,
		}
		if d.ChangedCommit != "" {
			row["changed_commit"] = d.ChangedCommit
			row["changed_at"] = d.ChangedAt
		}
		out[i] = row
	}
	return out
}

// ─── staleness_report ─────────────────────────────────────────────────────────

func registerStalenessReport(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "staleness_report",
		Description: "Staleness summary: source files, last-ingested state, git-commit drift. Reach for it when an answer looks wrong. Required: service.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"as_of":   map[string]any{"type": "string", "description": "RFC3339 timestamp or indexed commit sha: answer the claims-by-layer and doc-link sections as they stood then (GL-17)"},
			},
			"required": []string{"service"},
		},
		group: GroupShared,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			AsOf    string `json:"as_of"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "staleness_report: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "staleness_report: service required"}
		}
		asOf, aerr := asOfFilter(ctx, backend, p.Service, p.AsOf, "staleness_report")
		if aerr != nil {
			return nil, aerr
		}
		// Derive a simple "sources seen" summary from ListAllSymbols.
		nodes, err := serviceSymbols(ctx, backend, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		seen := make(map[string]struct{}, len(nodes))
		for _, n := range nodes {
			if n.FilePath != "" {
				seen[n.FilePath] = struct{}{}
			}
		}
		sources := make([]string, 0, len(seen))
		for f := range seen {
			sources = append(sources, f)
		}
		sort.Strings(sources)

		// Fetch commit-drift metadata (best-effort; empty strings if unavailable).
		indexedCommit := ""
		currentCommit := ""
		reingestRecommended := false
		if repoPath, ic, found, metaErr := backend.ServiceMeta(ctx, p.Service); metaErr == nil && found {
			indexedCommit = ic
			// Attempt to resolve current HEAD using the stored path.
			if repoPath != "" {
				currentCommit = currentGitHead(ctx, repoPath)
			}
			if indexedCommit != "" && currentCommit != "" && indexedCommit != currentCommit {
				reingestRecommended = true
			}
		}

		// RC-19: no budget line existed for staleness_report before this
		// row — measured 46,567 B for a large service's source list on
		// this repo's own index. source_count stays the TRUE total (an
		// honest summary is never truncated); only the listed "sources"
		// rows are capped.
		listedSources, elidedCount := fitToolBudget(sources, defaultCharBudget)
		resp := map[string]any{
			"service":              p.Service,
			"source_count":         len(sources),
			"sources":              stringsToAny(listedSources),
			"indexed_commit":       indexedCommit,
			"current_commit":       currentCommit,
			"reingest_recommended": reingestRecommended,
		}
		if line := elisionLine(elidedCount, "read_source or outline on the file you doubt (source_count is the full total)"); line != "" {
			resp["elision"] = line
		}
		// SR-28 (F-56, design §4): stale_docs' own scan, folded in.
		resp["doc_drift"] = stalenessDocDrift(ctx, backend, p.Service, nodes)
		// KR-7 (docs/design/knowledge-retrieval.md §6): doc_links-based
		// drift, a DIFFERENT and additive signal from doc_drift above --
		// doc_drift (SR-28) flags a span naming something the index no
		// longer knows at ALL; doc_link_drift flags a span whose linked
		// target's content changed (or vanished), attributed to the commit
		// that moved it when one is on record. Separate keys on purpose:
		// SR-28's existing shape and callers are untouched. Omitted
		// entirely (never an empty list standing in for "checked, found
		// nothing") when the backend has no doc-link-drift capability —
		// docDriftBackend's own comment explains why that is not the same
		// as "nothing drifted" (I38).
		//
		// GL-17: an as_of call leaves doc_link_drift out — drift compares a
		// link's frozen hash with the target as it stands NOW, so it has no
		// dated answer — and stalenessAsOf names the doc links that held
		// then instead.
		if ddb, ok := backend.(docDriftBackend); ok && !asOf.active() {
			if drifted, err := ddb.DocLinkDrift(ctx, p.Service); err == nil {
				resp["doc_link_drift"] = renderDocLinkDrift(drifted)
			}
		}
		// RT-99 (§7.1): a re-ingest the refresh queue holds is named, so
		// reingest_recommended is read beside "it is already on its way".
		if q := s.RefreshQueue(); q != nil {
			switch q.StateOf(p.Service) {
			case refresh.StateQueued:
				resp["refresh"] = "queued"
			case refresh.StateRunning:
				resp["refresh"] = "refreshing"
			}
			// I38: a queued refresh's failure (a post-commit request has
			// no caller to hear it) is named until a success clears it.
			if f, failed := q.LastFailure(p.Service); failed {
				resp["refresh_failure"] = fmt.Sprintf("%s refresh failed at %s: %s", joinReasons(f.Reasons), f.At.UTC().Format(time.RFC3339), f.Err)
			}
		}
		// BP-35: the typed-edge pass outcome for this service, when the
		// backend can report it. `known: false` is the honest "no ingest
		// of this service has run in this daemon process" state — a
		// reader must not read an absent block as a passing one.
		if teb, ok := backend.(typedEdgeStatusBackend); ok {
			st, known := teb.TypedEdgeStatus(p.Service)
			block := map[string]any{"known": known}
			if known {
				block["packages"] = st.Packages
				block["failed"] = st.Failed
				if st.Reason != "" {
					block["reason"] = st.Reason
				}
				// DG-9: same discovery-gap disclosure as list_services'
				// typed_edges block (platform_tools.go) — doctor's
				// typed_edges row reads THIS block, not list_services',
				// so a discovery gap must travel through here to reach
				// doctor at all.
				if st.DiscoveryGaps > 0 {
					block["discovery_gaps"] = st.DiscoveryGaps
					if len(st.DiscoveryGapFiles) > 0 {
						block["discovery_gap_files"] = st.DiscoveryGapFiles
					}
				}
				// finding [4]/DG-10: Persisted/RecordedAt exist SPECIFICALLY so a
				// caller -- doctor's checkTypedEdges, in particular -- can tell a
				// store-replayed (pre-restart) answer apart from one this daemon
				// process just computed. Before this fix, THIS was the one
				// surface doctor actually reads (cmd/fathom/doctor_typed.go), and
				// it dropped both fields, so a restart-stale PASS was reported
				// identically to a genuinely fresh one -- the exact "an
				// instrument may not degrade toward healthy" failure ADR-027 was
				// written to prevent, reintroduced through the very mechanism
				// (DG-10) built to fix restart-driven blind spots.
				if st.Persisted {
					block["persisted"] = true
					block["recorded_at"] = st.RecordedAt.UTC().Format(time.RFC3339)
				}
			}
			resp["typed_edges"] = block
		}
		// Optional: append per-layer claims summary when backend supports it.
		// GL-17: a dated call counts the claims valid at the instant instead
		// (status counts are today's verdicts, not what stood then).
		if asOf.active() {
			stalenessAsOf(ctx, backend, p.Service, asOf, resp)
		} else if csb, ok := backend.(claimsSummaryBackend); ok {
			if summary, err := csb.ClaimsSummary(ctx, p.Service); err == nil && len(summary) > 0 {
				resp["layers"] = summary
			}
		}
		return resp, nil
	})
}

// ─── prompts ──────────────────────────────────────────────────────────────────

// promptDef is a canned prompt definition.
type promptDef struct {
	Name        string
	Description string
	Arguments   []promptArg
	// tools lists every tool the template's workflow instructs the model to
	// call; a scoped endpoint hides prompts whose tools it gates.
	tools []string
	// template is called with the arguments map to produce the messages array.
	template func(args map[string]string) []map[string]any
}

// promptArg describes one argument to a canned prompt.
type promptArg struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// cannedPrompts is fathom's built-in prompts: empty since SR-20 (surface
// reckoning F-78) removed the four canned workflow prompts the plan row
// names (each 0 lifetime calls). registerPromptsHandlers stays wired below
// and serves an empty prompts/list — the MCP prompts capability is kept,
// the canned content is not.
var cannedPrompts = []promptDef{}

// promptIndex is a fast-lookup map built once at package init.
//
//fathom:tenancy-allow pkg-map prompt index built once from the canned prompt list at init and only read after: not a cache
var promptIndex map[string]*promptDef

func init() {
	promptIndex = make(map[string]*promptDef, len(cannedPrompts))
	for i := range cannedPrompts {
		promptIndex[cannedPrompts[i].Name] = &cannedPrompts[i]
	}
}

// registerPromptsHandlers wires prompts/list and prompts/get.
func registerPromptsHandlers(s *Server) {
	s.Register("prompts/list", func(ctx context.Context, _ json.RawMessage) (any, *rpcError) {
		items := make([]any, 0, len(cannedPrompts))
		for i := range cannedPrompts {
			p := &cannedPrompts[i]
			if !s.promptOnScope(ctx, p) {
				continue
			}
			args := make([]any, len(p.Arguments))
			for j, a := range p.Arguments {
				args[j] = map[string]any{
					"name":        a.Name,
					"description": a.Description,
					"required":    a.Required,
				}
			}
			items = append(items, map[string]any{
				"name":        p.Name,
				"description": p.Description,
				"arguments":   args,
			})
		}
		return map[string]any{"prompts": items}, nil
	})

	s.Register("prompts/get", func(ctx context.Context, params json.RawMessage) (any, *rpcError) {
		var p struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "prompts/get: invalid params"}
		}
		if p.Name == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "prompts/get: name required"}
		}
		def, ok := promptIndex[p.Name]
		if !ok {
			return nil, &rpcError{Code: ErrCodeNotFound, Message: fmt.Sprintf("prompts/get: unknown prompt %q", p.Name)}
		}
		if !s.promptOnScope(ctx, def) {
			return nil, &rpcError{Code: ErrCodeNotFound, Message: fmt.Sprintf("prompts/get: prompt %q uses tools that are not on the %s surface; POST /rpc serves the full surface", p.Name, scopeFrom(ctx).surface)}
		}
		args := p.Arguments
		if args == nil {
			args = map[string]string{}
		}
		messages := def.template(args)
		return map[string]any{
			"description": def.Description,
			"messages":    messages,
		}, nil
	})
}

// ─── resources ────────────────────────────────────────────────────────────────

const (
	manifestURI      = "fathom:///manifest"
	serviceURIPrefix = "fathom:///service/"
)

// registerResourcesHandlers wires resources/list and resources/read.
func registerResourcesHandlers(s *Server, backend QueryInterface) {
	s.Register("resources/list", func(ctx context.Context, _ json.RawMessage) (any, *rpcError) {
		resources := []any{
			map[string]any{
				"uri":         manifestURI,
				"name":        "Fathom Manifest",
				"description": "Server manifest: registered tools, active profile, and known services.",
				"mimeType":    "application/json",
			},
		}
		// Add per-service resources when backend is available.
		if backend != nil {
			svcs, err := backend.Services(ctx)
			if err == nil {
				for _, svc := range svcs {
					resources = append(resources, map[string]any{
						"uri":         serviceURIPrefix + svc,
						"name":        "Service: " + svc,
						"description": "Symbol summary for service " + svc,
						"mimeType":    "application/json",
					})
				}
			}
		}
		return map[string]any{"resources": resources}, nil
	})

	s.Register("resources/read", func(ctx context.Context, params json.RawMessage) (any, *rpcError) {
		var p struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "resources/read: invalid params"}
		}
		if p.URI == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "resources/read: uri required"}
		}

		switch {
		case p.URI == manifestURI:
			return readManifest(s, ctx, backend)
		case strings.HasPrefix(p.URI, serviceURIPrefix):
			svcID := strings.TrimPrefix(p.URI, serviceURIPrefix)
			return readServiceResource(ctx, backend, svcID)
		default:
			return nil, &rpcError{Code: ErrCodeNotFound, Message: fmt.Sprintf("resources/read: unknown resource %q", p.URI)}
		}
	})
}

// readManifest returns the fathom:///manifest resource.
func readManifest(s *Server, ctx context.Context, backend QueryInterface) (any, *rpcError) {
	descs := s.visibleToolsCtx(ctx)
	toolNames := make([]string, len(descs))
	for i, d := range descs {
		toolNames[i] = d.Name
	}
	var svcs []string
	if backend != nil {
		var err error
		svcs, err = backend.Services(ctx)
		if err != nil {
			svcs = nil
		}
	}
	payload := map[string]any{
		"server":         "fathom",
		"version":        "1.0.0",
		"tools":          toolNames,
		"active_surface": s.effectiveSurface(ctx),
		"active_profile": s.activeProfile(),
		"services":       svcs,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "manifest marshal: " + err.Error()}
	}
	return map[string]any{
		"contents": []any{
			map[string]any{
				"uri":      manifestURI,
				"mimeType": "application/json",
				"text":     string(b),
			},
		},
	}, nil
}

// readServiceResource returns the fathom:///service/<id> resource.
func readServiceResource(ctx context.Context, backend QueryInterface, svcID string) (any, *rpcError) {
	if backend == nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
	}
	nodes, err := serviceSymbols(ctx, backend, svcID)
	if err != nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
	}
	// Collect distinct file paths and kinds.
	files := make(map[string]struct{}, len(nodes))
	kinds := make(map[string]int, 16)
	for _, n := range nodes {
		if n.FilePath != "" {
			files[n.FilePath] = struct{}{}
		}
		kinds[string(n.Kind)]++
	}
	fileList := make([]string, 0, len(files))
	for f := range files {
		fileList = append(fileList, f)
	}
	sort.Strings(fileList)

	payload := map[string]any{
		"service":      svcID,
		"symbol_count": len(nodes),
		"file_count":   len(files),
		"files":        fileList,
		"kinds":        kinds,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "service resource marshal: " + err.Error()}
	}
	uri := serviceURIPrefix + svcID
	return map[string]any{
		"contents": []any{
			map[string]any{
				"uri":      uri,
				"mimeType": "application/json",
				"text":     string(b),
			},
		},
	}, nil
}
