// Package client is the public Go embedding API for fathom.
//
// It exposes a thin Client that wires together a Connector, a fusion Pipeline,
// a query Engine, and a Store so that callers never import internal packages
// directly.
//
// Three named modes are available via NewConstantClient:
//
//   - ModeLocal  — reads source from a local directory tree.
//   - ModeFaux   — reads from the in-repo testdata/fauxtown fixture.
//   - ModeGitHub — placeholder; returns an error until implemented.
package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/query"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/worktree"
)

// SourceFile is a single fetched source file. It is a re-export of
// core.SourceFile so callers never import internal packages.
type SourceFile = core.SourceFile

// Mode selects which connector the Client uses to acquire source.
type Mode int

const (
	// ModeLocal reads source from a local directory path on disk.
	ModeLocal Mode = iota
	// ModeFaux reads from the in-repo testdata/fauxtown fixture.
	ModeFaux
	// ModeGitHub reads via the gh CLI (requires gh to be installed).
	ModeGitHub
)

// ServiceRef identifies a service/repo to ingest. It is a re-export of
// core.ServiceRef so callers never import internal packages.
type ServiceRef = core.ServiceRef

// Query is a concern-scoped lookup. It is a re-export of core.Query.
type Query = core.Query

// QueryResult is the typed answer to a Query. It is a re-export of
// core.QueryResult.
type QueryResult = core.QueryResult

// Report summarises a completed Ingest run. It is a re-export of
// fusion.Report.
type Report = fusion.Report

// ReadFailure names one file an Ingest could not read — Report.ReadFailures
// lists them sorted by Path. It is a re-export of fusion.ReadFailure.
type ReadFailure = fusion.ReadFailure

// ReadFailuresError is the error Ingest returns beside a complete Report
// when at least one file could not be read (IT-14). Refused marks the O16
// guard (IT-15): the ingest could read nothing and the store already held
// evidence for the service, so nothing was written. Hint() carries the
// operator hint for the failure class. It is a re-export of
// fusion.ReadFailuresError.
type ReadFailuresError = fusion.ReadFailuresError

// SymbolNode is a symbol with location and outgoing call edges. It is a
// re-export of core.SymbolNode.
type SymbolNode = core.SymbolNode

// Edge is a directed, kind-labeled call edge produced by the Go typed
// enrichment pass (ADR-021 §3, internal/fusion.EnrichTypedEdges). It is a
// re-export of core.Edge.
type Edge = core.Edge

// Client wraps the fathom pipeline and query engine so callers can ingest
// services and query symbols without importing internal packages.
//
// Client is safe for concurrent use by multiple goroutines once constructed.
type Client struct {
	pipe   *fusion.Pipeline
	engine *query.Engine
	st     store.Store
	conn   connector.Connector // retained for Sources()

	// Optional LLM config for semantic-layer compilation (WithLLM).
	llm        llmclient.LLMClient
	llmModel   string
	llmInRate  float64
	llmOutRate float64
}

// New constructs a Client from an explicit connector and store. Callers that
// want automatic mode-based wiring should use NewConstantClient instead.
func New(conn connector.Connector, st store.Store) *Client {
	pipe := fusion.New(conn, st, fusion.Config{Concurrency: 4})
	eng := query.New(st)
	return &Client{pipe: pipe, engine: eng, st: st, conn: conn}
}

// NewConstantClient opens (or creates) a store at storePath — SQLite for a
// ".sqlite"/".db" path (ADR-020) — and wires the connector selected by mode.
// Pass "" for storePath to use an in-memory SQLite database.
//
// For ModeFaux the storePath may be "" (memory) or any writable path; the
// connector always reads from the in-repo testdata/fauxtown fixture.
// For ModeLocal the connector reads from svc.Path at Ingest time.
// For ModeGitHub the connector reads via the gh CLI; svc.Repo must be set.
func NewConstantClient(mode Mode, storePath string) (*Client, error) {
	var conn connector.Connector
	switch mode {
	case ModeLocal:
		conn = connector.NewLocalConnector()
	case ModeFaux:
		conn = connector.NewFauxConnector()
	case ModeGitHub:
		return nil, fmt.Errorf("client: ModeGitHub is not yet implemented")
	default:
		return nil, fmt.Errorf("client: unknown mode %d", mode)
	}

	st, err := store.Open(storePath)
	if err != nil {
		return nil, fmt.Errorf("client: open store %q: %w", storePath, err)
	}

	return New(conn, st), nil
}

// Ingest runs the full fusion pipeline for svc and returns a Report.
//
// Worktree re-root refusal (PC-33, I30, O22): when the store already
// records a root for svc.ID and svc.Path is a linked git worktree of that
// root (or any .claude/worktrees/ path), the ingest is refused with a
// *worktree.RerootError naming both paths — the same check the daemon's
// `ingest` tool applies, so the CLI's store-direct path cannot move a
// service onto an agent's worktree either. A path that does not exist as a
// directory (a faux-mode fixture name) is never a worktree.
func (c *Client) Ingest(ctx context.Context, svc ServiceRef) (Report, error) {
	if err := core.ValidServiceID(svc.ID); err != nil {
		return Report{}, fmt.Errorf("ingest: %w", err)
	}
	if err := c.refuseWorktreeReroot(ctx, svc.ID, svc.Path); err != nil {
		return Report{}, fmt.Errorf("ingest: %w", err)
	}
	return c.pipe.Ingest(ctx, svc)
}

// refuseWorktreeReroot applies worktree.CheckReroot against the recorded
// root for service, when there is one and path is an existing directory.
func (c *Client) refuseWorktreeReroot(ctx context.Context, service, path string) error {
	meta, found, err := c.st.GetServiceMeta(ctx, service)
	if err != nil || !found || meta.Path == "" {
		return nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return nil
	}
	return worktree.CheckReroot(service, meta.Path, abs)
}

// IngestBatch ingests each service in svcs sequentially, collecting Reports.
// The first error aborts the batch; partial results are returned alongside the
// error so callers can inspect what succeeded.
func (c *Client) IngestBatch(ctx context.Context, svcs []ServiceRef) ([]Report, error) {
	reports := make([]Report, 0, len(svcs))
	for _, svc := range svcs {
		r, err := c.Ingest(ctx, svc)
		if err != nil {
			return reports, fmt.Errorf("client.IngestBatch %q: %w", svc.ID, err)
		}
		reports = append(reports, r)
	}
	return reports, nil
}

// QuerySymbols executes a concern-scoped query and returns ranked symbols.
func (c *Client) QuerySymbols(ctx context.Context, q Query) (QueryResult, error) {
	return c.engine.Query(ctx, q)
}

// FindCallers returns all symbols whose call list includes symbol (matched by
// bare name or SymbolKey suffix).
func (c *Client) FindCallers(ctx context.Context, serviceID, symbol string) ([]SymbolNode, error) {
	return c.engine.FindCallers(ctx, serviceID, symbol)
}

// FindSymbol returns all symbols matching name exactly or as a SymbolKey suffix
// for the given service.
func (c *Client) FindSymbol(ctx context.Context, serviceID, name string) ([]SymbolNode, error) {
	return c.engine.FindSymbol(ctx, serviceID, name)
}

// BatchQuery runs each query in qs and returns results in the same order.
// The first error aborts the batch; partial results are returned alongside the
// error.
func (c *Client) BatchQuery(ctx context.Context, qs []Query) ([]QueryResult, error) {
	results := make([]QueryResult, 0, len(qs))
	for i, q := range qs {
		r, err := c.QuerySymbols(ctx, q)
		if err != nil {
			return results, fmt.Errorf("client.BatchQuery[%d]: %w", i, err)
		}
		results = append(results, r)
	}
	return results, nil
}

// ListAllSymbols returns every SymbolNode stored for serviceID, with no
// concern or keyword filtering.  This is the "list everything" API used by
// inspect and by the query fallback path.
func (c *Client) ListAllSymbols(ctx context.Context, serviceID string) ([]SymbolNode, error) {
	obs, err := c.st.QueryObservations(ctx, serviceID, "")
	if err != nil {
		return nil, fmt.Errorf("client.ListAllSymbols: %w", err)
	}
	nodes := make([]SymbolNode, 0, len(obs))
	for _, o := range obs {
		nodes = append(nodes, o.ToSymbolNode())
	}
	return nodes, nil
}

// TypedEdges returns every typed Go call edge (ADR-021 §3) stored for
// serviceID — direct, interface-dispatch (one row per implementer), and
// unresolved. Empty/nil for a non-Go service, or a Go service whose typed
// pass hasn't run yet (e.g. it predates ADR-021 §3, or its first ingest
// under the new pass hasn't happened).
func (c *Client) TypedEdges(ctx context.Context, serviceID string) ([]Edge, error) {
	edges, err := c.st.TypedEdges(ctx, serviceID)
	if err != nil {
		return nil, fmt.Errorf("client.TypedEdges: %w", err)
	}
	return edges, nil
}

// Observations returns every stored observation for serviceID, with no
// concern filtering. It powers fleet-level backends that need raw evidence
// rows (e.g. cross-service drift and caller analysis).
func (c *Client) Observations(ctx context.Context, serviceID string) ([]Observation, error) {
	obs, err := c.st.QueryObservations(ctx, serviceID, "")
	if err != nil {
		return nil, fmt.Errorf("client.Observations: %w", err)
	}
	return obs, nil
}

// Services returns the sorted distinct service IDs that have been ingested into
// the store.  An empty store returns a nil slice with no error.
func (c *Client) Services(ctx context.Context) ([]string, error) {
	ids, err := c.st.ListServiceIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("client.Services: %w", err)
	}
	return ids, nil
}

// Sources discovers and fetches every source file for the named service using
// the client's connector. It calls Discover(svc) then Fetch for each ref and
// returns the results as []SourceFile sorted by path.
//
// The path is resolved from the service_meta recorded at ingest time when
// available — so source-scanning tools work regardless of the server's
// working directory. It falls back to the service ID as path so connectors
// that only need the ID (like FauxConnector) keep working.
func (c *Client) Sources(ctx context.Context, service string) ([]core.SourceFile, error) {
	svcRef := core.ServiceRef{ID: service, Path: service}
	if meta, found, err := c.st.GetServiceMeta(ctx, service); err == nil && found && meta.Path != "" {
		svcRef.Path = meta.Path
	}

	refs, err := c.conn.Discover(ctx, svcRef)
	if err != nil {
		return nil, fmt.Errorf("client.Sources discover %q: %w", service, err)
	}

	files := make([]SourceFile, 0, len(refs))
	for _, ref := range refs {
		data, err := c.conn.Fetch(ctx, ref)
		if err != nil {
			// Skip files that cannot be fetched (e.g. permission errors).
			continue
		}
		// Language is live-computed on every call (ADR-021 §1 / LT-1): the
		// shared connector registry is the single source of truth, so cloc
		// and every other source-scanning tool see every registered
		// language plus an explicit "unclassified" bucket instead of a
		// silent "other". Binary detection is content-based and always wins
		// over the extension-derived language (LT-2): a NUL-bearing file
		// never contributes to line counts, regardless of its extension.
		lang := connector.LanguageForPath(ref.Path)
		if connector.IsBinary(data) {
			lang = core.LangBinary
		}
		files = append(files, SourceFile{
			Path:      ref.Path,
			Content:   data,
			Lang:      lang,
			Generated: connector.IsGenerated(ref.Path, data),
		})
	}
	return files, nil
}

// IngestService re-ingests the named service and satisfies the
// mcp.QueryInterface.IngestService contract (used by the ingest MCP tool and by
// auto-refresh). It re-ingests from the filesystem path recorded in
// service_meta at the last ingest — NOT the service ID — so the re-ingest reads
// the real source and re-records the same path. Re-ingesting from the bare ID
// (a non-path) re-walked the wrong directory and clobbered ServiceMeta.path to
// empty, which silently froze the daemon's index (auto-refresh / staleness /
// ingest all depend on that path). Falls back to the ID only when no path is
// recorded (e.g. FauxConnector). Use Ingest for full control over the ServiceRef.
//
// ADR-016 post-ingest sweep: after a successful ingest, if the service has any
// compiled claims, SweepClaims is run best-effort (error ignored). This is the
// $0-token citation-freshness pass — status flips happen without any LLM call.
func (c *Client) IngestService(ctx context.Context, service string) error {
	ref := ServiceRef{ID: service, Path: service}
	if meta, found, err := c.st.GetServiceMeta(ctx, service); err == nil && found && meta.Path != "" {
		ref.Path = meta.Path
	}
	if _, err := c.pipe.Ingest(ctx, ref); err != nil {
		return err
	}
	// Best-effort post-ingest sweep: run only when claims exist.
	if claims, err := c.st.ClaimsForService(ctx, service, ""); err == nil && len(claims) > 0 {
		_, _ = c.SweepClaims(ctx, service)
	}
	return nil
}

// IngestServiceAt ingests service from an explicit corpus root — the
// first-time-onboarding path the `ingest` MCP tool exposes so a new repo can
// join the index without touching the daemon's store from a second process
// (single-writer lock). path must be an absolute, existing directory; after
// the first ingest records ServiceMeta, plain IngestService (and
// auto-refresh) keep the service fresh from the recorded path.
func (c *Client) IngestServiceAt(ctx context.Context, service, path string) error {
	if err := core.ValidServiceID(service); err != nil {
		return fmt.Errorf("ingest at: %w", err)
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("ingest at: path %q must be absolute", path)
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("ingest at: path %q is not an existing directory", path)
	}
	if err := c.refuseWorktreeReroot(ctx, service, path); err != nil {
		return fmt.Errorf("ingest at: %w", err)
	}
	if _, err := c.pipe.Ingest(ctx, ServiceRef{ID: service, Path: path}); err != nil {
		return err
	}
	if claims, err := c.st.ClaimsForService(ctx, service, ""); err == nil && len(claims) > 0 {
		_, _ = c.SweepClaims(ctx, service)
	}
	return nil
}

// RemoveService deletes every stored row for serviceID — registry surgery
// (store.Store.RemoveService). Exposed on the client so the daemon's
// remove_service verb reaches it through the same delegation chain every
// other capability uses.
func (c *Client) RemoveService(ctx context.Context, serviceID string) (bool, error) {
	return c.st.RemoveService(ctx, serviceID)
}

// RenameService atomically re-keys every service_id-scoped row
// (store.Store.RenameService) — typed refusals pass through verbatim.
func (c *Client) RenameService(ctx context.Context, oldID, newID string) error {
	return c.st.RenameService(ctx, oldID, newID)
}

// IngestStats returns the process-lifetime unclassified/oversize counters
// (ADR-021 §2 / TI-6) accumulated across every Ingest this Client's pipeline
// has run — see fusion.Pipeline.Stats. Consumed by the mcp server's
// server_stats tool and (as a before/after delta around one call) the ingest
// tool's own response, via the optional statsBackend capability.
func (c *Client) IngestStats() (unclassified, oversize int64) {
	return c.pipe.Stats()
}

// ClassifiedZeroStats returns the process-lifetime per-tool
// classified-but-zero-symbols counters (ADR-021 §2, critic F5) — see
// fusion.Pipeline.ClassifiedZeroStats. Consumed by the mcp server_stats
// tool via the optional classifiedZeroBackend capability.
func (c *Client) ClassifiedZeroStats() map[string]int64 {
	return c.pipe.ClassifiedZeroStats()
}

// ServiceWatermark returns the connector's *current* freshness token for the
// service's corpus (git HEAD SHA, or a content-addressed manifest hash for
// non-git corpora — ADR-017), resolved from the path recorded in service_meta
// at the last ingest. Comparing it against the stored IndexedCommit detects
// corpus drift without assuming git. Empty string means freshness is unknown.
func (c *Client) ServiceWatermark(ctx context.Context, service string) (string, error) {
	svcRef := core.ServiceRef{ID: service, Path: service}
	if meta, found, err := c.st.GetServiceMeta(ctx, service); err == nil && found && meta.Path != "" {
		svcRef.Path = meta.Path
	}
	return c.conn.Watermark(ctx, svcRef)
}

// ServiceMeta returns the stored ingest metadata (path, indexed commit) for
// serviceID.  found is false when no metadata has been recorded yet.
// This satisfies the mcp.QueryInterface.ServiceMeta contract.
func (c *Client) ServiceMeta(ctx context.Context, serviceID string) (path, indexedCommit string, found bool, err error) {
	meta, ok, storeErr := c.st.GetServiceMeta(ctx, serviceID)
	if storeErr != nil {
		return "", "", false, storeErr
	}
	if !ok {
		return "", "", false, nil
	}
	return meta.Path, meta.IndexedCommit, true, nil
}

// ListCommits returns commit history for serviceID, newest-first, as CommitInfo
// presentation records (Date is RFC3339, FilesTouchedCount is len(Files)).
func (c *Client) ListCommits(ctx context.Context, serviceID string) ([]store.CommitInfo, error) {
	commits, err := c.st.ListCommits(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	out := make([]store.CommitInfo, len(commits))
	for i, cm := range commits {
		dateStr := ""
		if cm.Date != 0 {
			dateStr = time.Unix(0, cm.Date).UTC().Format(time.RFC3339)
		}
		out[i] = store.CommitInfo{
			Hash:              cm.Hash,
			Date:              dateStr,
			Author:            cm.Author,
			Message:           cm.Message,
			Files:             cm.Files,
			FilesTouchedCount: len(cm.Files),
			SpecIDs:           cm.SpecIDs,
		}
	}
	return out, nil
}

// Close releases all resources held by the client (including the store).
func (c *Client) Close() error {
	return c.st.Close()
}
