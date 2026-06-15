// Package client is the public Go embedding API for sextant.
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
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/sextant/internal/connector"
	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/fusion"
	"github.com/geoffmilleraz/sextant/internal/query"
	"github.com/geoffmilleraz/sextant/internal/store"
)

// SourceFile is a single fetched source file. It is a re-export of
// core.SourceFile so callers never import internal packages.
type SourceFile = core.SourceFile

// langFromPath derives a lowercase language tag from a file's extension.
// Returns "" for unrecognised extensions.
func langFromPath(path string) string {
	base := filepath.Base(path)
	// Special basenames first.
	switch base {
	case "go.sum", "go.mod":
		return "go"
	case "Gemfile.lock":
		return "ruby"
	case "Cargo.lock":
		return "rust"
	case "requirements.txt":
		return "py"
	case "package-lock.json":
		return "js"
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "py"
	case ".js", ".jsx":
		return "js"
	case ".ts", ".tsx":
		return "ts"
	case ".java":
		return "java"
	case ".rb":
		return "ruby"
	case ".rs":
		return "rust"
	case ".tf":
		return "terraform"
	case ".md":
		return "md"
	case ".yaml", ".yml":
		return "yaml"
	}
	return ""
}

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

// SymbolNode is a symbol with location and outgoing call edges. It is a
// re-export of core.SymbolNode.
type SymbolNode = core.SymbolNode

// Client wraps the sextant pipeline and query engine so callers can ingest
// services and query symbols without importing internal packages.
//
// Client is safe for concurrent use by multiple goroutines once constructed.
type Client struct {
	pipe   *fusion.Pipeline
	engine *query.Engine
	st     store.Store
	conn   connector.Connector // retained for Sources()
}

// New constructs a Client from an explicit connector and store. Callers that
// want automatic mode-based wiring should use NewConstantClient instead.
func New(conn connector.Connector, st store.Store) *Client {
	pipe := fusion.New(conn, st, fusion.Config{Concurrency: 4})
	eng := query.New(st)
	return &Client{pipe: pipe, engine: eng, st: st, conn: conn}
}

// NewConstantClient opens (or creates) a DuckDB store at storePath and
// wires the connector selected by mode. Pass "" for storePath to use an
// in-memory DuckDB database.
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
func (c *Client) Ingest(ctx context.Context, svc ServiceRef) (Report, error) {
	return c.pipe.Ingest(ctx, svc)
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
		files = append(files, SourceFile{
			Path:    ref.Path,
			Content: data,
			Lang:    langFromPath(ref.Path),
		})
	}
	return files, nil
}

// IngestService re-ingests the named service using a minimal ServiceRef
// (ID == Path) and satisfies the mcp.QueryInterface.IngestService contract.
// Use Ingest for full control over the ServiceRef.
func (c *Client) IngestService(ctx context.Context, service string) error {
	_, err := c.pipe.Ingest(ctx, ServiceRef{ID: service, Path: service})
	return err
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
