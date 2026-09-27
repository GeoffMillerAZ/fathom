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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/query"
	"github.com/geoffmilleraz/fathom/internal/scip"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
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

	// Optional decision port (ADR-042, WithDecision): decider is nil until
	// WithDecision is called, which keeps every decision-port-aware call
	// site (CompileLayer's prefilter, Obligations*'s rerank) byte-identical
	// to pre-decision-port behavior for every Client that never calls it.
	decider   decision.Decider
	decideCfg config.Decide
	decideRec *decide.Recorder

	// oblMemo is the obligations hot path's memo (RT-143/RT-81,
	// obligations_corpus.go): the layer corpus's paths per store
	// watermark and its guides' contents per stat.
	oblMemo obligationsMemo

	// aggregates memoizes ServiceSymbolAggregates per store watermark
	// (RT-144, aggregates.go).
	aggregates aggregateMemo
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

// SetRehash makes the next ingests read and hash every file, ignoring the
// stat gate (RT-102; `fathom ingest --rehash`).
func (c *Client) SetRehash(on bool) { c.pipe.SetRehash(on) }

// ReingestFile (SU-5/SU-6, docs/plans/substrate.md §A, ADR-041 decisions 3
// and 4) re-extracts ONE file's observations/syntax edges and fingerprint
// — the daemon's POST /refresh-file route and the pre-edit hook's
// per-file freshness check both call through here. relFile is repo-relative
// (forward-slash); the repo root is resolved from the service's recorded
// ServiceMeta (set at the last full Ingest), so a service that has never
// been ingested has nowhere to read the file from and errors loudly rather
// than guessing a path.
func (c *Client) ReingestFile(ctx context.Context, serviceID, relFile string) (fusion.ReingestReport, error) {
	meta, found, err := c.st.GetServiceMeta(ctx, serviceID)
	if err != nil {
		return fusion.ReingestReport{}, fmt.Errorf("client.ReingestFile: get service meta: %w", err)
	}
	if !found || meta.Path == "" {
		return fusion.ReingestReport{}, fmt.Errorf("client.ReingestFile: service %q has no recorded repo path (never ingested)", serviceID)
	}
	return c.pipe.ReingestFile(ctx, c.st, serviceID, meta.Path, relFile)
}

// SourcePackages returns the distinct packages of the symbols anchored at
// file (repo-relative) — one file's rows, never the whole symbol table
// (RT-27).
func (c *Client) SourcePackages(ctx context.Context, serviceID, file string) ([]string, error) {
	pkgs, err := c.st.SourcePackages(ctx, serviceID, file)
	if err != nil {
		return nil, fmt.Errorf("client.SourcePackages: %w", err)
	}
	return pkgs, nil
}

// PackageSymbols returns every symbol of serviceID whose package is one of
// pkgs — those packages' complete node sets, no other package's rows
// (RT-27).
func (c *Client) PackageSymbols(ctx context.Context, serviceID string, pkgs []string) ([]SymbolNode, error) {
	obs, err := c.st.PackageObservations(ctx, serviceID, pkgs)
	if err != nil {
		return nil, fmt.Errorf("client.PackageSymbols: %w", err)
	}
	nodes := make([]SymbolNode, 0, len(obs))
	for _, o := range obs {
		nodes = append(nodes, o.ToSymbolNode())
	}
	return nodes, nil
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

// ImportSCIPResult is ImportSCIP's return: a re-export of scip.Import so
// callers never import internal/scip directly.
type ImportSCIPResult = scip.Import

// scipStoreKeyPrefix returns the store pkg_path NAMESPACE this SCIP index
// file's packages are written under — "scip:<path>::" — mirroring
// internal/fusion/typedcue.go's own "cue:" convention: the shared
// typed_edges/typed_edge_fingerprints tables are keyed only by (serviceID,
// pkgPath), so a producer that wants to reconcile ("this run's package set
// is now exactly these, and no others") without ever touching another
// producer's rows must own a private prefix of that namespace.
//
// Scoped by the SCIP index's own path, not just a bare "scip:" — because
// `fathom ingest` can import SEVERAL SCIP indexes in one run (docs/usage.md:
// --scip is repeatable, or auto-detected from index.scip plus every
// .fathom/scip/*.scip), each commonly covering a disjoint slice of the
// service's packages. Reconciling at the whole-service "scip:" level would
// make one file's transient decode failure — or simply not passing it this
// run — look identical to "this package's source vanished," and delete
// another file's still-current rows. Per-path scoping keeps each index
// file's reconcile self-contained: it only ever purges rows IT previously
// wrote (O116 residual, .fathom/observations.md).
//
// key is scipIndexKey's stable identity for the file, never the raw path
// as given: an absolute path changes whenever the same repository is
// ingested from another checkout or a remapped root, and an explicit
// --scip pointing into a temp directory changes every run — either would
// open a fresh namespace per import and leave every earlier one's rows
// behind forever, the very leak this namespace exists to close.
func scipStoreKeyPrefix(key string) string {
	return "scip:" + key + "::"
}

// scipIndexKey is a SCIP index file's stable identity for
// scipStoreKeyPrefix: its slash-separated path relative to the service root
// when the file lives inside it (the auto-detected index.scip and
// .fathom/scip/*.scip locations), else its base name. Both sides resolve
// symlinks first, so a root reached through a symlinked parent still
// relativizes. Two index files OUTSIDE the root with the same base name
// share a namespace — name external indexes distinctly.
func scipIndexKey(root, path string) string {
	if root != "" {
		abs, err := filepath.Abs(path)
		if err == nil {
			if r, rerr := filepath.EvalSymlinks(root); rerr == nil {
				root = r
			}
			if a, aerr := filepath.EvalSymlinks(abs); aerr == nil {
				abs = a
			}
			if rel, rerr := filepath.Rel(root, abs); rerr == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
				return filepath.ToSlash(rel)
			}
		}
	}
	return filepath.Base(path)
}

// ImportSCIP decodes a SCIP index file (ADR-041 decision 5, SU-9,
// docs/plans/substrate.md; internal/scip.ImportIndexFile) and writes the
// typed edges it implies for serviceID, grouped per package — the same
// store.PutTypedEdges package-granular replace unit
// internal/fusion.EnrichTypedEdges already uses for the native Go pass, so
// re-importing one changed file's worth of SCIP output never disturbs
// another package's rows. It never touches the syntax-extracted symbol
// table, only typed_edges.
//
// O115: internal/scip.ImportIndex maps every SCIP definition against
// serviceID's OWN already-extracted symbol table (never the SCIP index's
// own DisplayName, which a real producer never sets), so this reads that
// table first — always available in practice, since ingest.go only calls
// ImportSCIP after the syntax pass has written serviceID.
//
// O116 residual (.fathom/observations.md): O116 itself fixed the case where
// a package's source is still present but now computes to zero edges
// (still named in imp.Packages, with an empty Edges slice, so the loop
// below still calls PutTypedEdges and clears it). This does NOT cover a
// package that VANISHES ENTIRELY from the new decode — deleted, renamed, or
// the producer stops emitting it — because such a package is never named
// in imp.Packages at all, so the per-package loop never even sees it, and
// its old rows survive forever. Fixed here by writing every package under
// this path's own "scip:<path>::" namespace (scipStoreKeyPrefix) and then
// reconciling: list every stored package in that namespace and delete any
// this import did not just (re)write. Never touches a bare (Go typed pass)
// or "cue:"-prefixed (CUE typed pass) pkg_path, and never touches another
// SCIP index file's own "scip:<other path>::" rows.
func (c *Client) ImportSCIP(ctx context.Context, serviceID, path string) (ImportSCIPResult, error) {
	nodes, err := c.ListAllSymbols(ctx, serviceID)
	if err != nil {
		return scip.Import{}, fmt.Errorf("client.ImportSCIP: list symbols: %w", err)
	}
	imp, err := scip.ImportIndexFile(serviceID, path, nodes)
	if err != nil {
		return scip.Import{}, fmt.Errorf("client.ImportSCIP: %w", err)
	}
	root, _, _, err := c.ServiceMeta(ctx, serviceID)
	if err != nil {
		return imp, fmt.Errorf("client.ImportSCIP: service meta: %w", err)
	}
	prefix := scipStoreKeyPrefix(scipIndexKey(root, path))
	keep := make(map[string]bool, len(imp.Packages))
	for _, pkg := range imp.Packages {
		key := prefix + pkg.Package
		keep[key] = true
		if err := c.st.PutTypedEdges(ctx, serviceID, key, pkg.Edges); err != nil {
			return imp, fmt.Errorf("client.ImportSCIP: put typed edges for %q: %w", pkg.Package, err)
		}
	}
	stored, err := c.st.ListTypedEdgePackages(ctx, serviceID)
	if err != nil {
		return imp, fmt.Errorf("client.ImportSCIP: list typed edge packages: %w", err)
	}
	for _, id := range stored {
		if !strings.HasPrefix(id, prefix) || keep[id] {
			continue
		}
		if err := c.st.DeleteTypedEdgePackage(ctx, serviceID, id); err != nil {
			return imp, fmt.Errorf("client.ImportSCIP: delete vanished package %q: %w", id, err)
		}
	}
	return imp, nil
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

// SourcePaths lists the service's source file paths WITHOUT reading any
// file — connector discovery only. A question answered by paths (which
// languages a repository holds) must use this, never Sources: Sources
// fetches every file's contents, and on list_services' path that read whole
// repositories into memory per caller (2026-09-26, ~30 GB).
func (c *Client) SourcePaths(ctx context.Context, service string) ([]string, error) {
	svcRef := core.ServiceRef{ID: service, Path: service}
	if meta, found, err := c.st.GetServiceMeta(ctx, service); err == nil && found && meta.Path != "" {
		svcRef.Path = meta.Path
	}
	refs, err := c.conn.Discover(ctx, svcRef)
	if err != nil {
		return nil, fmt.Errorf("client.SourcePaths discover %q: %w", service, err)
	}
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ref.Path)
	}
	return out, nil
}

// Sources discovers and fetches every source file for the named service using
// the client's connector. It calls Discover(svc) then Fetch for each ref and
// returns the results as []SourceFile sorted by path.
//
// The path is resolved from the service_meta recorded at ingest time when
// available — so source-scanning tools work regardless of the server's
// working directory. It falls back to the service ID as path so connectors
// that only need the ID (like FauxConnector) keep working.
//
// RT-146: at most two Sources reads run at once process-wide
// (acquireSourcesSlot); a hook-origin ctx (WithHookOrigin) never waits and
// gets ErrSourcesBusy. The fetch loop stops when ctx ends.
func (c *Client) Sources(ctx context.Context, service string) ([]core.SourceFile, error) {
	release, err := acquireSourcesSlot(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	// A request abandoned mid-read (its scope's abandon fires) stops the
	// walk and the fetch loop too, not only the wait for a slot.
	if abandon := abandonOf(ctx); abandon != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		defer cancel()
		go func() {
			select {
			case <-abandon:
				cancel()
			case <-ctx.Done():
			}
		}()
	}
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
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		// RT-153: the producer governor, before each file. A halted walk is
		// an error, never a silently shorter corpus.
		if err := tenancy.ProducerAllowsCtx(ctx, tenancy.ClassTree); err != nil {
			return nil, fmt.Errorf("client.Sources %q: %w", service, err)
		}
		data, err := c.conn.Fetch(ctx, ref)
		if errors.Is(err, tenancy.ErrOverBudget) {
			return nil, fmt.Errorf("client.Sources %q: %w", service, err)
		}
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
// service_meta at the last ingest (or, for a first ingest killed before its
// final commit, in its ingest_state row) — NOT the service ID — so it reads
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
	// RT-153/RT-173: an ingest is ingest-class work the producer governor
	// halts at refuse and the exit rung can halt and quarantine.
	ctx, done := tenancy.BeginIngest(ctx, service)
	defer done()
	ref := ServiceRef{ID: service, Path: service}
	if p := c.corpusPath(ctx, service); p != "" {
		ref.Path = p
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
	ctx, done := tenancy.BeginIngest(ctx, service) // RT-153/RT-173
	defer done()
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

// DefaultIgnoredDirs names, repo-relative and sorted, each directory the
// connector's default cache list pruned from its most recent discovery of
// service (RT-145) — nil when none, or when the connector does not record
// exclusions. The MCP ingest tool carries it as default_ignored_dirs.
func (c *Client) DefaultIgnoredDirs(service string) []string {
	if er, ok := c.conn.(connector.ExclusionReporter); ok {
		return er.Exclusions(service).DefaultIgnored
	}
	return nil
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

// TypedEdgeStatus returns the last typed-edge pass outcome this Client's
// pipeline recorded for service (BP-35, internal/fusion/typedenv.go), and
// whether any ingest of that service has run in this process at all. A
// daemon whose Go toolchain environment is broken reports Failed with the
// named reason here, instead of serving a syntax-only graph that every
// surface renders as healthy. Consumed by /statusz, the mcp
// staleness_report tool, and doctor's typed_edges row.
func (c *Client) TypedEdgeStatus(service string) (fusion.TypedEdgeStatus, bool) {
	return c.pipe.TypedEdgeStatus(service)
}

// TypedPassTotal returns the process-lifetime count of typed-edge passes
// this Client's pipeline has actually run (O108) — see
// fusion.Pipeline.TypedPassTotal. Consumed by cmd/fathom's /metrics as
// fathom_typed_pass_total.
func (c *Client) TypedPassTotal() int64 {
	return c.pipe.TypedPassTotal()
}

// TypedPassInflight returns 1 while a typed-edge pass is currently running
// anywhere in this process, else 0 — see fusion.TypedPassInflight (W-N,
// docs/plans/daemon-footprint.md). The semaphore it reflects is
// process-wide, not per-Pipeline, so this is a package-level read, not a
// forward to c.pipe. Consumed by cmd/fathom's /metrics as
// fathom_typed_pass_inflight.
func (c *Client) TypedPassInflight() int64 {
	return fusion.TypedPassInflight()
}

// TypedPassWaitedTotal returns the process-lifetime count of typed-edge
// passes that had to wait for another pass already holding the
// process-wide typed-pass slot — see fusion.TypedPassWaitedTotal (W-N).
// Consumed by cmd/fathom's /metrics as fathom_typed_pass_waited_total.
func (c *Client) TypedPassWaitedTotal() int64 {
	return fusion.TypedPassWaitedTotal()
}

// ServiceWatermark returns the connector's *current* freshness token for the
// service's corpus (git HEAD SHA, or a content-addressed manifest hash for
// non-git corpora — ADR-017), resolved from the path recorded in service_meta
// at the last ingest. Comparing it against the stored IndexedCommit detects
// corpus drift without assuming git. Empty string means freshness is unknown.
//
// RT-110 (design §6, ADR-017 amended): a service whose stored watermark is a
// git SHA is compared only against its HEAD, read from the ref files
// (gitro.Head); when HEAD cannot be read the answer is "" with an error
// naming freshness as unknown — never the connector's content manifest,
// which reads the whole corpus on the request path.
func (c *Client) ServiceWatermark(ctx context.Context, service string) (string, error) {
	svcRef := core.ServiceRef{ID: service, Path: service}
	meta, found, err := c.st.GetServiceMeta(ctx, service)
	if err == nil && found && meta.Path != "" {
		svcRef.Path = meta.Path
	}
	if err == nil && found && gitro.IsObjectName(meta.IndexedCommit) {
		head, herr := gitro.Head(ctx, svcRef.Path)
		if herr != nil {
			return "", fmt.Errorf("service %q: %w", service, herr)
		}
		return head, nil
	}
	return c.conn.Watermark(ctx, svcRef)
}

// Store returns the underlying store.Store Client was constructed over —
// an escape hatch for a caller that needs a capability Client itself does
// not re-export (ADR-044 wave 2: internal/mcp's review_diff wiring builds
// an internal/conserve.Input, whose Store field wants the real
// store.Store interface, not Client's own curated subset of it). Callers
// outside pkg/client should prefer Client's own methods when one exists;
// this exists for the packages (internal/mcp, cmd/fathom) that already
// import internal/store directly for other reasons.
func (c *Client) Store() store.Store { return c.st }

// StoreWatermark returns the STORE's freshness token for serviceID: the git
// commit recorded at the last ingest joined to that ingest's timestamp.
//
// It is deliberately not ServiceWatermark. ServiceWatermark asks the
// connector what the TREE looks like right now, which moves whenever a file
// is edited; this one moves when — and only when — an ingest replaced the
// service's observations. That is the key derived data may be memoized
// under (invariant I21, bench-pyramid BP-28): a hit can never serve a graph
// built from rows the store has since replaced, and an unchanged index does
// not invalidate on every keystroke.
//
// Returns "" with no error for a service the store has no metadata for; a
// caller must treat "" as "cannot memoize", never as a key.
func (c *Client) StoreWatermark(ctx context.Context, serviceID string) (string, error) {
	meta, ok, err := c.st.GetServiceMeta(ctx, serviceID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	return fmt.Sprintf("%s@%d", meta.IndexedCommit, meta.IndexedAt), nil
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
	c.pipe.Close() // RT-107: the pipeline's grammar modules
	return c.st.Close()
}
