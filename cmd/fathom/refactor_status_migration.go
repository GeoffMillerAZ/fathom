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
// CX-11w and CX-12w (design §4, U9) add what the sites alone do not say. Each
// remaining site's blocker — the layer rule that governs its file against the
// change, or generated code — is named beside it (review.MigrationBlockers;
// refactor_status_blockers.go), and each repository's burn-down and completion
// verdict is printed (review.MigrationBurnDown; refactor_status_burndown.go):
// its gone: rows and their verdicts over time from the verify ledger, and
// conservation run when --conserve-since names a base, never assumed.
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
	"github.com/geoffmilleraz/fathom/internal/console"
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
// conserveSince, when set, is the base ref `fathom conserve` compares each
// repository against for the completion verdict (CX-12w).
func cmdRefactorStatusMigration(migrationText, storePath string, services []string, conserveSince string, jsonOut bool, stdout, stderr io.Writer) int {
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

	view, berr := refactorStatusMigration(ctx, c, m, migrationStatusOpts{Services: services, ConserveSince: conserveSince, Store: st})
	if berr != nil {
		fmt.Fprintf(stderr, "error: %v\n", berr)
		return 2
	}

	rep := migrationStatusReport{
		Schema:        "fathom.refactor.status.migration/v1",
		Migration:     migrationWire{Kind: string(m.Kind), From: m.From, To: m.To},
		ConserveSince: conserveSince,
		Repositories:  renderMigrationRepos(view),
		Degraded:      view.Degraded,
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
		blocked := 0
		for _, s := range r.Remaining {
			if s.Blocker != nil {
				blocked++
			}
		}
		head := fmt.Sprintf("%s: %d remaining, %d done", r.Repository, len(r.Remaining), len(r.Done))
		if blocked > 0 {
			head = fmt.Sprintf("%s: %d remaining (%d blocked), %d done", r.Repository, len(r.Remaining), blocked, len(r.Done))
		}
		fmt.Fprintln(stdout, head)
		for _, s := range r.Remaining {
			line := fmt.Sprintf("  - remaining [%s] %s:%d", s.Tier, s.File, s.Line)
			if s.Blocker != nil {
				line += " — blocked: " + s.Blocker.String()
			}
			fmt.Fprintln(stdout, line)
		}
		for _, s := range r.Done {
			fmt.Fprintf(stdout, "  - done      [%s] %s:%d\n", s.Tier, s.File, s.Line)
		}
		if r.BurnDown != nil {
			printMigrationBurnDown(stdout, r.BurnDown)
		}
	}
	for _, d := range rep.Degraded {
		fmt.Fprintf(stdout, "degraded: %s\n", d)
	}
	return 0
}

// migrationStatusOpts are refactorStatusMigration's inputs beyond the client
// and the migration.
type migrationStatusOpts struct {
	// Services limits the view to these service ids (default: every service
	// the store knows).
	Services []string
	// ConserveSince is the base ref conservation is run against per
	// repository; empty means it is not run and no repository is complete.
	ConserveSince string
	// Store is the store conservation reads.
	Store store.Store
	// Ledger is the verify ledger the burn-down reads (default:
	// verifyLedgerPath()).
	Ledger string
}

// migrationFleetView is refactorStatusMigration's answer: the sites per
// repository (CX-10), each remaining site's blocker (CX-11w), each
// repository's burn-down and completion verdict (CX-12w), and what could
// not be read.
type migrationFleetView struct {
	Repos    []review.RepoMigrationSites
	Blockers []review.MigrationBlocker
	BurnDown []review.RepoMigrationBurnDown
	// GoneRows are each repository's gone: rows with their ledger verdict.
	GoneRows map[string][]migrationBurnRowWire
	// Conservation is each repository's conservation state, in words.
	Conservation map[string]string
	Degraded     []string
}

// refactorStatusMigration builds the fleet-wide migration view for m: every
// service in opts.Services (or every service the store knows, when none is
// named) contributes its own remaining/done sites via
// migrationSitesForService, and review.MigrationSites (CX-10) groups the
// combined result per repository. Each remaining site's blocker comes from
// review.MigrationBlockers (CX-11w) and each repository's burn-down and
// completion verdict from review.MigrationBurnDown (CX-12w). A service that
// could not be read degrades into a named note (O36's execute-and-flag
// doctrine) rather than aborting the whole fleet-wide call.
func refactorStatusMigration(ctx context.Context, c *client.Client, m review.Migration, opts migrationStatusOpts) (migrationFleetView, error) {
	svcs := opts.Services
	if len(svcs) == 0 {
		var err error
		svcs, err = c.Services(ctx)
		if err != nil {
			return migrationFleetView{}, fmt.Errorf("list services: %w", err)
		}
	}
	sorted := append([]string(nil), svcs...)
	sort.Strings(sorted)

	var view migrationFleetView
	var sites []review.MigrationSite
	var governed []review.GovernedSite
	var generated []review.GeneratedFile
	roots := map[string]string{}
	for _, svc := range sorted {
		read := migrationSitesForService(ctx, c, svc, m)
		view.Degraded = append(view.Degraded, read.Notes...)
		if read.Unread {
			continue
		}
		sites = append(sites, read.Sites...)
		governed = append(governed, read.Blockers.Governed...)
		generated = append(generated, read.Blockers.Generated...)
		roots[svc] = read.Root
	}
	view.Repos = review.MigrationSites(m, sites)
	view.Blockers = review.MigrationBlockers(sites, governed, generated)
	migrationBurnDownFor(ctx, &view, m, opts, sorted, roots)
	return view, nil
}

// migrationBurnDownFor fills view's burn-down: for every read repository
// with a site or a gone: row naming m.From, its rows' verdicts and series
// from the verify ledger, and conservation when opts.ConserveSince is set,
// folded by review.MigrationBurnDown.
func migrationBurnDownFor(ctx context.Context, view *migrationFleetView, m review.Migration, opts migrationStatusOpts, sorted []string, roots map[string]string) {
	// The ledger is read once, and only when a gone: row needs it.
	var hist console.History
	ledgerRead := false
	history := func() console.History {
		if !ledgerRead {
			ledgerRead = true
			ledger := opts.Ledger
			if ledger == "" {
				ledger = verifyLedgerPath()
			}
			h, read := console.ProofHistory(ctx, ledger, console.DefaultHistoryTail)
			if read.State != console.StatePresent {
				view.Degraded = append(view.Degraded, "verify ledger: "+read.Reason+" — every gone: row reads unproven")
			}
			hist = h
		}
		return hist
	}

	hasSites := map[string]bool{}
	for _, r := range view.Repos {
		hasSites[r.Repository] = true
	}
	view.GoneRows = map[string][]migrationBurnRowWire{}
	view.Conservation = map[string]string{}
	series := map[string][]review.MigrationBurnDownPoint{}
	var rows []review.RepoRows
	for _, svc := range sorted {
		root, ok := roots[svc]
		if !ok {
			continue // unread: already a degraded note
		}
		var gone []migrationGoneRow
		if root != "" {
			gone = migrationGoneRows(root, m)
		}
		if !hasSites[svc] && len(gone) == 0 {
			continue
		}
		var h console.History
		if len(gone) > 0 {
			h = history()
		}
		verdicts, points := migrationLedgerRows(h, svc, gone)
		rr := review.RepoRows{Repository: svc, Rows: verdicts}
		wire := make([]migrationBurnRowWire, len(gone))
		for i, g := range gone {
			wire[i] = migrationBurnRowWire{Plan: g.Plan, ID: g.ID, Verdict: verdicts[i].Verdict}
		}
		view.GoneRows[svc] = wire
		series[svc] = points

		switch {
		case len(verdicts) == 0:
			view.Conservation[svc] = "not run: no gone: row to complete"
		case opts.ConserveSince == "":
			rr.ConservationUnproven = "not run — pass --conserve-since <ref> to check that no loss is unexcused"
			view.Conservation[svc] = "not run"
		case root == "":
			rr.ConservationUnproven = "the service records no corpus root to run conserve --since " + opts.ConserveSince + " in"
			view.Conservation[svc] = "not run: no recorded root"
		default:
			rr.Unexcused, rr.Detail, rr.ConservationUnproven = migrationConservation(ctx, opts.Store, svc, root, opts.ConserveSince)
			switch {
			case rr.Unexcused:
				view.Conservation[svc] = "losses"
			case rr.ConservationUnproven != "":
				view.Conservation[svc] = "unproven"
			default:
				view.Conservation[svc] = "conserved since " + opts.ConserveSince
			}
		}
		rows = append(rows, rr)
	}
	view.BurnDown = review.MigrationBurnDown(m, series, rows)
}

// migrationServiceRead is what one service contributes to the fleet view.
type migrationServiceRead struct {
	Sites []review.MigrationSite
	// Blockers are the generated and governed facts for the remaining
	// sites (CX-11w).
	Blockers migrationBlockerFacts
	// Root is the service's recorded corpus root, "" when it records none.
	Root string
	// Notes are named degradations; Unread is set, and Sites nil, only when
	// this service could not be read at all.
	Notes  []string
	Unread bool
}

// migrationSitesForService reads ONE service's own sources (never another
// service's, never the whole fleet at once) and returns every occurrence
// of m's From (OnTarget:false, "remaining") and To (OnTarget:true, "done")
// value, tiered typed/lexical, with the facts that block its remaining
// sites.
func migrationSitesForService(ctx context.Context, c *client.Client, svc string, m review.Migration) migrationServiceRead {
	unread := func(format string, a ...any) migrationServiceRead {
		return migrationServiceRead{Unread: true, Notes: []string{svc + ": " + fmt.Sprintf(format, a...)}}
	}
	sources, err := c.Sources(ctx, svc)
	if err != nil {
		return unread("read corpus sources: %v", err)
	}

	fromPat, toPat, perr := migrationScanPatterns(m)
	if perr != nil {
		return unread("%v", perr)
	}

	opts := references.Options{Regex: true, Max: migrationSiteScanMax}
	fromOccs, ferr := references.Find(fromPat, sources, opts)
	if ferr != nil {
		return unread("scan for %q: %v", m.From, ferr)
	}
	toOccs, terr := references.Find(toPat, sources, opts)
	if terr != nil {
		return unread("scan for %q: %v", m.To, terr)
	}

	var nodes []core.SymbolNode
	var typedFromFiles, typedToFiles map[string]bool
	if m.Kind == review.MigrationSymbol {
		nodes, _ = c.ListAllSymbols(ctx, svc)
		typedFromFiles, typedToFiles = typedMigrationCallerFiles(ctx, c, svc, m, nodes)
	}

	var sites []review.MigrationSite
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

	root, _, _, _ := c.ServiceMeta(ctx, svc)
	read := migrationServiceRead{Sites: sites, Root: root}
	read.Blockers = migrationBlockerFactsFor(svc, root, m, sites, sources, nodes)
	read.Notes = read.Blockers.Notes
	return read
}

// typedMigrationCallerFiles resolves m's bare From/To symbol names against
// svc's own call graph (its symbols already listed + TypedEdges, best-effort: a
// service with no typed pass yet simply contributes no typed upgrade) and
// returns the files holding a RESOLVED caller of each — the same
// isResolvedKind filter refactor_plan.go's renderSymbolRefactorPlan
// already applies.
func typedMigrationCallerFiles(ctx context.Context, c *client.Client, svc string, m review.Migration, nodes []core.SymbolNode) (fromFiles, toFiles map[string]bool) {
	if len(nodes) == 0 {
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
	Schema        string              `json:"schema"`
	Migration     migrationWire       `json:"migration"`
	ConserveSince string              `json:"conserve_since,omitempty"`
	Repositories  []migrationRepoWire `json:"repositories"`
	Degraded      []string            `json:"degraded,omitempty"`
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
	// BurnDown is the repository's burn-down and completion verdict
	// (CX-12w); absent for a repository with no gone: row and no site to
	// speak of.
	BurnDown *migrationBurnDownWire `json:"burn_down,omitempty"`
}

type migrationSiteWire struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Tier string `json:"tier"`
	// Blocker is set on a remaining site that is blocked (CX-11w).
	Blocker *migrationBlockerWire `json:"blocker,omitempty"`
}

// migrationBlockerWire is why a remaining site is blocked: "governed" cites
// the layer rule's id, "generated" has none.
type migrationBlockerWire struct {
	Reason string `json:"reason"`
	Rule   string `json:"rule,omitempty"`
}

func (b migrationBlockerWire) String() string {
	if b.Reason == string(review.BlockedGoverned) {
		return "governed by rule " + b.Rule
	}
	return b.Reason + " code"
}

type migrationBurnRowWire struct {
	Plan    string `json:"plan"`
	ID      string `json:"id"`
	Verdict string `json:"verdict"`
}

type migrationPointWire struct {
	At        string `json:"at"`
	Claimed   int    `json:"claimed"`
	Proven    int    `json:"proven"`
	Remaining int    `json:"remaining"`
}

// migrationBurnDownWire is one repository's burn-down: its gone: rows and
// their verdicts, the series over time, whether the migration is complete
// there and, when it is not, why; and the state of conservation.
type migrationBurnDownWire struct {
	Rows         []migrationBurnRowWire `json:"rows"`
	Series       []migrationPointWire   `json:"series"`
	Complete     bool                   `json:"complete"`
	Reason       string                 `json:"reason,omitempty"`
	Conservation string                 `json:"conservation"`
}

func printMigrationBurnDown(w io.Writer, b *migrationBurnDownWire) {
	proven := 0
	for _, r := range b.Rows {
		if r.Verdict == review.RowProven {
			proven++
		}
	}
	fmt.Fprintf(w, "  burn-down: %d of %d gone: row(s) proven\n", proven, len(b.Rows))
	for _, r := range b.Rows {
		fmt.Fprintf(w, "    %s#%s  %s\n", r.Plan, r.ID, r.Verdict)
	}
	for _, p := range b.Series {
		fmt.Fprintf(w, "    %s  %d of %d claimed proven (%d remaining)\n", p.At, p.Proven, p.Claimed, p.Remaining)
	}
	fmt.Fprintf(w, "  conservation: %s\n", b.Conservation)
	if b.Complete {
		fmt.Fprintln(w, "  complete: yes")
		return
	}
	fmt.Fprintf(w, "  complete: no — %s\n", b.Reason)
}

// renderMigrationRepos builds the per-repository wire rows: every
// repository with a site or a burn-down, sorted by name, each remaining site
// carrying its blocker.
func renderMigrationRepos(v migrationFleetView) []migrationRepoWire {
	blockerOf := map[string]migrationBlockerWire{}
	for _, b := range v.Blockers {
		blockerOf[siteKey(b.Site)] = migrationBlockerWire{Reason: string(b.Reason), Rule: b.Rule}
	}
	byRepo := map[string]*migrationRepoWire{}
	get := func(name string) *migrationRepoWire {
		if r, ok := byRepo[name]; ok {
			return r
		}
		r := &migrationRepoWire{Repository: name, Remaining: []migrationSiteWire{}, Done: []migrationSiteWire{}}
		byRepo[name] = r
		return r
	}
	for _, r := range v.Repos {
		w := get(r.Repository)
		w.Remaining = renderMigrationSites(r.Remaining, blockerOf)
		w.Done = renderMigrationSites(r.Done, nil)
	}
	for _, b := range v.BurnDown {
		w := get(b.Repository)
		rows := v.GoneRows[b.Repository]
		if rows == nil {
			rows = []migrationBurnRowWire{}
		}
		series := make([]migrationPointWire, len(b.Points))
		for i, p := range b.Points {
			series[i] = migrationPointWire{At: p.At, Claimed: p.Claimed, Proven: p.Proven, Remaining: p.Remaining()}
		}
		w.BurnDown = &migrationBurnDownWire{Rows: rows, Series: series, Complete: b.Complete, Reason: b.Reason, Conservation: v.Conservation[b.Repository]}
	}
	names := make([]string, 0, len(byRepo))
	for n := range byRepo {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]migrationRepoWire, 0, len(names))
	for _, n := range names {
		out = append(out, *byRepo[n])
	}
	return out
}

func siteKey(s review.MigrationSite) string {
	return fmt.Sprintf("%s\x00%s\x00%d", s.Repository, s.File, s.Line)
}

func renderMigrationSites(sites []review.MigrationSite, blockers map[string]migrationBlockerWire) []migrationSiteWire {
	out := make([]migrationSiteWire, 0, len(sites))
	for _, s := range sites {
		w := migrationSiteWire{File: s.File, Line: s.Line, Tier: string(s.Tier)}
		if b, ok := blockers[siteKey(s)]; ok {
			b := b
			w.Blocker = &b
		}
		out = append(out, w)
	}
	return out
}
