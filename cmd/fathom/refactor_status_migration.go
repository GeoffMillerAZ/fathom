package main

// refactor_status_migration.go — CX-13 (design §4, U9 the migration
// tracker): `fathom refactor status --migration "<kind> <from> -> <to>"`
// shows the FLEET-WIDE migration view — CX-10's review.MigrationSites,
// built from per-service typed and lexical reference queries run through
// *pkg/client.Client — one service at a time, never the whole fleet's
// sources loaded together (bounded per service). It only ever reads: this
// command computes and reports, it never edits a single site (the Moderne
// non-goal, refused as CX-X4 in the plan's own "Refused" section).
//
// "Existing" reference queries, reused rather than reinvented:
//   - typed: internal/analysis/callgraph.CallersDetail over the service's
//     own ListAllSymbols/TypedEdges — the SAME resolved-caller machinery
//     cmd/fathom/refactor_plan.go's renderSymbolRefactorPlan already uses
//     (isResolvedKind is shared with that file).
//   - lexical: internal/analysis/references.Find over the service's own
//     Sources — the SAME matcher the "references" MCP tool
//     (internal/mcp/references_tool.go) scans with.
//
// Typed confirmation is per FILE, not per call-expression line: CallersDetail
// names the CALLING FUNCTION's own definition site, not the exact call
// line, so "this file has a resolved typed caller of the target" is the
// honest granularity available here — a lexical occurrence in a file with
// no typed caller stays lexical (design §4: "typed where available,
// lexical otherwise, each tier named").
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/analysis/references"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/review"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// migrationSiteScanMax bounds each per-service, per-pattern lexical scan
// (references.Find's own Options.Max) — a runaway match (a one-character
// "from" against a huge service) stops here rather than holding every line
// of a corpus in memory, the same "bounded per service" discipline
// references_tool.go applies with its own occurrence cap.
const migrationSiteScanMax = 2000

// cmdRefactorStatusMigration is cmdRefactorStatus's --migration branch: it
// takes over BEFORE the --plan/ratchet flow below (that flow stays
// unchanged, CV-34/ADR-044 §4's own purely git+file contract untouched).
func cmdRefactorStatusMigration(migrationText, storePath string, services []string, jsonOut bool, stdout, stderr io.Writer) int {
	m, perr := review.ParseMigration(migrationText)
	if perr != nil {
		fmt.Fprintf(stderr, "error: --migration: %v\n", perr)
		return 2
	}

	st, err := store.Open(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", storePath, err)
		return 2
	}
	defer st.Close()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	ctx := context.Background()

	repos, degraded, berr := refactorStatusMigration(ctx, c, services, m)
	if berr != nil {
		fmt.Fprintf(stderr, "error: %v\n", berr)
		return 2
	}

	rep := migrationStatusReport{
		Schema:       "fathom.refactor.status.migration/v1",
		Migration:    migrationWire{Kind: string(m.Kind), From: m.From, To: m.To},
		Repositories: renderMigrationRepos(repos),
		Degraded:     degraded,
	}

	if jsonOut {
		b, jerr := json.MarshalIndent(rep, "", "  ")
		if jerr != nil {
			fmt.Fprintf(stderr, "error: encode JSON: %v\n", jerr)
			return 2
		}
		fmt.Fprintln(stdout, string(b))
		return 0
	}

	fmt.Fprintf(stdout, "migration status: %s %s -> %s\n", rep.Migration.Kind, rep.Migration.From, rep.Migration.To)
	if len(rep.Repositories) == 0 {
		fmt.Fprintln(stdout, "  no sites found in any service")
	}
	for _, r := range rep.Repositories {
		fmt.Fprintf(stdout, "%s: %d remaining, %d done\n", r.Repository, len(r.Remaining), len(r.Done))
		for _, s := range r.Remaining {
			fmt.Fprintf(stdout, "  - remaining [%s] %s:%d\n", s.Tier, s.File, s.Line)
		}
		for _, s := range r.Done {
			fmt.Fprintf(stdout, "  - done      [%s] %s:%d\n", s.Tier, s.File, s.Line)
		}
	}
	for _, d := range rep.Degraded {
		fmt.Fprintf(stdout, "degraded: %s\n", d)
	}
	return 0
}

// refactorStatusMigration builds the fleet-wide migration view for m: every
// service in services (or every service the store knows, when services is
// empty) contributes its own remaining/done sites via
// migrationSitesForService, and review.MigrationSites (CX-10) groups the
// combined result per repository. A service that could not be read
// degrades into a named note (O36's execute-and-flag doctrine) rather than
// aborting the whole fleet-wide call.
func refactorStatusMigration(ctx context.Context, c *client.Client, services []string, m review.Migration) (repos []review.RepoMigrationSites, degraded []string, err error) {
	svcs := services
	if len(svcs) == 0 {
		svcs, err = c.Services(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("list services: %w", err)
		}
	}
	sorted := append([]string(nil), svcs...)
	sort.Strings(sorted)

	var sites []review.MigrationSite
	for _, svc := range sorted {
		s, note := migrationSitesForService(ctx, c, svc, m)
		if note != "" {
			degraded = append(degraded, note)
			continue
		}
		sites = append(sites, s...)
	}
	return review.MigrationSites(m, sites), degraded, nil
}

// migrationSitesForService reads ONE service's own sources (never another
// service's, never the whole fleet at once) and returns every occurrence
// of m's From (OnTarget:false, "remaining") and To (OnTarget:true, "done")
// value, tiered typed/lexical. note is non-empty, and sites nil, only when
// this service could not be read at all.
func migrationSitesForService(ctx context.Context, c *client.Client, svc string, m review.Migration) (sites []review.MigrationSite, note string) {
	sources, err := c.Sources(ctx, svc)
	if err != nil {
		return nil, fmt.Sprintf("%s: read corpus sources: %v", svc, err)
	}

	fromPat, toPat, perr := migrationScanPatterns(m)
	if perr != nil {
		return nil, fmt.Sprintf("%s: %v", svc, perr)
	}

	opts := references.Options{Regex: true, Max: migrationSiteScanMax}
	fromOccs, ferr := references.Find(fromPat, sources, opts)
	if ferr != nil {
		return nil, fmt.Sprintf("%s: scan for %q: %v", svc, m.From, ferr)
	}
	toOccs, terr := references.Find(toPat, sources, opts)
	if terr != nil {
		return nil, fmt.Sprintf("%s: scan for %q: %v", svc, m.To, terr)
	}

	var typedFromFiles, typedToFiles map[string]bool
	if m.Kind == review.MigrationSymbol {
		typedFromFiles, typedToFiles = typedMigrationCallerFiles(ctx, c, svc, m)
	}

	for _, o := range fromOccs {
		tier := review.SiteTierLexical
		if typedFromFiles[o.File] {
			tier = review.SiteTierTyped
		}
		sites = append(sites, review.MigrationSite{Repository: svc, File: o.File, Line: o.Line, Tier: tier, OnTarget: false})
	}
	for _, o := range toOccs {
		tier := review.SiteTierLexical
		if typedToFiles[o.File] {
			tier = review.SiteTierTyped
		}
		sites = append(sites, review.MigrationSite{Repository: svc, File: o.File, Line: o.Line, Tier: tier, OnTarget: true})
	}
	return sites, ""
}

// typedMigrationCallerFiles resolves m's bare From/To symbol names against
// svc's own call graph (ListAllSymbols + TypedEdges, best-effort: a
// service with no typed pass yet simply contributes no typed upgrade) and
// returns the files holding a RESOLVED caller of each — the same
// isResolvedKind filter refactor_plan.go's renderSymbolRefactorPlan
// already applies.
func typedMigrationCallerFiles(ctx context.Context, c *client.Client, svc string, m review.Migration) (fromFiles, toFiles map[string]bool) {
	nodes, err := c.ListAllSymbols(ctx, svc)
	if err != nil || len(nodes) == 0 {
		return nil, nil
	}
	typed, _ := c.TypedEdges(ctx, svc) // best effort: empty for a service with no typed pass yet

	byKey := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byKey[n.Key()] = n
	}
	g := callgraph.Build(nodes)
	callgraph.AddTypedEdges(g, typed)

	return typedCallerFiles(g, byKey, bareSymbolName(m.From)), typedCallerFiles(g, byKey, bareSymbolName(m.To))
}

// typedCallerFiles returns the set of files holding a RESOLVED typed
// caller of bare, or nil when bare does not resolve in g at all (nothing
// declared under that name — not yet renamed, or already fully retired).
func typedCallerFiles(g *callgraph.Graph, byKey map[string]core.SymbolNode, bare string) map[string]bool {
	if bare == "" {
		return nil
	}
	canon := callgraph.Canonical(g, bare)
	if canon == "" {
		return nil
	}
	if _, ok := byKey[canon]; !ok {
		return nil
	}
	out := map[string]bool{}
	for _, ci := range callgraph.CallersDetail(g, canon) {
		if !isResolvedKind(ci.Kind) {
			continue // heuristic/unresolved is not a typed reference (design §4's own store-query basis)
		}
		if cn, ok := byKey[ci.Key]; ok {
			out[cn.FilePath] = true
		}
	}
	return out
}

// migrationScanPatterns turns m into a (from, to) pair of regular
// expressions references.Find (Options.Regex:true) can scan with, per
// CX-9's own five-kind vocabulary. An unsupported kind is refused by name
// rather than silently matching nothing — CX-9's ParseMigration already
// guarantees Kind is one of these five, so this only ever fires if that
// contract is ever loosened without this file being updated too.
func migrationScanPatterns(m review.Migration) (from, to string, err error) {
	switch m.Kind {
	case review.MigrationSymbol:
		return wordBoundaryPattern(bareSymbolName(m.From)), wordBoundaryPattern(bareSymbolName(m.To)), nil
	case review.MigrationImport:
		return quotedLiteralPattern(m.From), quotedLiteralPattern(m.To), nil
	case review.MigrationRoute, review.MigrationConfig, review.MigrationPkg:
		return wordBoundaryPattern(m.From), wordBoundaryPattern(m.To), nil
	default:
		return "", "", fmt.Errorf("unsupported migration kind %q", m.Kind)
	}
}

// bareSymbolName reduces a possibly-qualified symbol ("Recv.Method") to
// its trailing name — a call site reads "recv.method(", never the
// receiver-qualified declaration form (mirrors refactor_plan.go's own
// patternWord derivation).
func bareSymbolName(s string) string {
	if i := strings.LastIndex(s, "."); i >= 0 {
		return s[i+1:]
	}
	return s
}

func wordBoundaryPattern(s string) string {
	return `\b` + regexp.QuoteMeta(s) + `\b`
}

// quotedLiteralPattern matches s's own quoted literal in source text,
// using \x22 hex escapes rather than a literal '"' byte — the same
// quote-avoidance renderImportRefactorPlan uses, so this pattern is safe
// to embed unmodified in a plan's own annotation block if a caller does so.
func quotedLiteralPattern(s string) string {
	return `\x22` + regexp.QuoteMeta(s) + `\x22`
}

// ─── wire shapes ────────────────────────────────────────────────────────

type migrationStatusReport struct {
	Schema       string              `json:"schema"`
	Migration    migrationWire       `json:"migration"`
	Repositories []migrationRepoWire `json:"repositories"`
	Degraded     []string            `json:"degraded,omitempty"`
}

type migrationWire struct {
	Kind string `json:"kind"`
	From string `json:"from"`
	To   string `json:"to"`
}

type migrationRepoWire struct {
	Repository string              `json:"repository"`
	Remaining  []migrationSiteWire `json:"remaining"`
	Done       []migrationSiteWire `json:"done"`
}

type migrationSiteWire struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Tier string `json:"tier"`
}

func renderMigrationRepos(repos []review.RepoMigrationSites) []migrationRepoWire {
	out := make([]migrationRepoWire, 0, len(repos))
	for _, r := range repos {
		out = append(out, migrationRepoWire{
			Repository: r.Repository,
			Remaining:  renderMigrationSites(r.Remaining),
			Done:       renderMigrationSites(r.Done),
		})
	}
	return out
}

func renderMigrationSites(sites []review.MigrationSite) []migrationSiteWire {
	out := make([]migrationSiteWire, 0, len(sites))
	for _, s := range sites {
		out = append(out, migrationSiteWire{File: s.File, Line: s.Line, Tier: string(s.Tier)})
	}
	return out
}
