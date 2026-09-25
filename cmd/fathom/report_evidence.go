package main

// report_evidence.go — cmdReportEvidence (CV-39, ADR-044 §5): "`fathom
// report evidence` gives, per service: typed share of call edges,
// certificate density (live over declared targets), inventory item
// counts, and whether receipts re-derive. It never ranks languages."
//
//	fathom report evidence [--service S] [--store DB] [--json]
//
// Store-direct (mirrors cmd/fathom/conserve.go and cmd/fathom/certify_
// status.go): opens --store directly, never the daemon. Omitted, --store
// resolves through resolveStorePath ([daemon].store, then the workspace
// default); a store that does not exist, indexes no service, or does not
// index the named --service is refused naming the store — never a table
// of zeros read from an empty file this verb itself created. --service scopes
// to one service; omitted, every service the store's observations table
// knows about is reported, ONE ROW EACH, in store.ListServiceIDs' own
// alphabetical id order — never resorted by typed share, certificate
// density, or any other per-language "how good is this language's
// tooling" merit. The SAME "never ranked" rule applies one level down:
// the per-language tier breakdown for one service (conserve.
// TiersByLanguage) is already alphabetical by language name, and this
// file adds nothing that would reorder it.
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/inventory"
	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/conserve"
	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/receiptverify"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// reportEvidenceSchema is the JSON envelope's wire identity — versioned
// the same way fathom.conserve/v1 is (docs/design/refactor-conservation.md
// §0), so a consumer can tell a future field addition from a breaking
// change.
const reportEvidenceSchema = "fathom.report.evidence/v1"

// evidenceRow is one service's evidence density row.
type evidenceRow struct {
	Service string `json:"service"`

	Edges      int                 `json:"edges"`
	TypedEdges int                 `json:"typed_edges"`
	TypedShare float64             `json:"typed_share"`
	Languages  []conserve.TierInfo `json:"languages,omitempty"`

	CertificatesDeclared int     `json:"certificates_declared"`
	CertificatesLive     int     `json:"certificates_live"`
	CertificateDensity   float64 `json:"certificate_density"`

	InventoryByKind map[string]int `json:"inventory_by_kind,omitempty"`
	InventoryTotal  int            `json:"inventory_total"`

	// ReceiptsRederive is Go-only (design §5): "live" when a sampled Go
	// declaration's location re-derived from a bare git checkout with
	// go/parser alone; "difference: …" when it did not; "not-applicable:
	// …"/"unproven: …" when the check could not even be attempted (no Go
	// sources, no corpus root recorded, not a git checkout) — honestly
	// reported, never defaulted to either true or false (I44's spirit:
	// a missing input is named, not silently treated as a pass).
	ReceiptsRederive string `json:"receipts_rederive"`
}

func cmdReportEvidence(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("report evidence", stderr)
	service := fs.String("service", "", "service ID (default: every service recorded in the store, one row each)")
	storeFlag := fs.String("store", "", "store path (.sqlite/.db); default: [daemon].store in config.toml, else "+defaultStorePath())
	jsonOut := fs.Bool("json", false, "emit the fathom.report.evidence/v1 JSON report instead of a text table")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Resolved the way the other config-aware store readers resolve it
	// (resolveStorePath: [daemon].store, then the workspace default), and
	// REFUSED when nothing is there: store.Open would create an empty
	// SQLite file at the default and every row would print as a measured
	// zero (survey 2026-09-25). A report is only as honest as the store it
	// names, so it names it.
	storePath := *storeFlag
	if storePath == "" {
		storePath = resolveStorePath()
	}
	if _, err := os.Stat(storePath); err != nil {
		fmt.Fprintf(stderr, "error: no store at %s (%v) — pass --store, or set [daemon].store in config.toml; nothing was read\n", storePath, err)
		return 1
	}

	st, err := store.Open(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", storePath, err)
		return 1
	}
	defer st.Close()
	ctx := context.Background()

	known, err := st.ListServiceIDs(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "error: list services in %s: %v\n", storePath, err)
		return 1
	}
	svcs := known
	if *service != "" {
		if !slices.Contains(known, *service) {
			fmt.Fprintf(stderr, "error: service %q is not indexed in store %s (indexed: %s) — its row would be all zeros, not a measurement\n",
				*service, storePath, strings.Join(known, ", "))
			return 1
		}
		svcs = []string{*service}
	}
	if len(svcs) == 0 {
		fmt.Fprintf(stderr, "error: store %s indexes no service — nothing to report\n", storePath)
		return 1
	}

	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()

	rows := make([]evidenceRow, 0, len(svcs))
	for _, svc := range svcs {
		rows = append(rows, computeEvidenceRow(ctx, st, c, svc))
	}

	if *jsonOut {
		b, jerr := json.MarshalIndent(map[string]any{
			"schema":   reportEvidenceSchema,
			"store":    storePath,
			"services": rows,
		}, "", "  ")
		if jerr != nil {
			fmt.Fprintf(stderr, "error: encode JSON: %v\n", jerr)
			return 1
		}
		fmt.Fprintln(stdout, string(b))
		return 0
	}

	fmt.Fprintf(stdout, "report evidence: store %s\n", storePath)
	printEvidenceReport(stdout, rows)
	return 0
}

// computeEvidenceRow gathers svc's evidence-density facts. Every sub-
// computation degrades to an honest zero/empty/"unproven" on its own
// missing input rather than failing the whole row — a service with no
// typed pass, no certifications and no recorded corpus root still gets a
// row, naming what it does not have.
func computeEvidenceRow(ctx context.Context, st store.Store, c *client.Client, svc string) evidenceRow {
	row := evidenceRow{Service: svc}

	edges, _ := st.TypedEdges(ctx, svc) // best effort: absent for an unindexed/untyped service
	row.Edges = len(edges)
	for _, e := range edges {
		if core.EdgeConfidenceOf(e) == core.EdgeConfidenceResolved {
			row.TypedEdges++
		}
	}
	if row.Edges > 0 {
		row.TypedShare = round4(float64(row.TypedEdges) / float64(row.Edges))
	}

	root := ""
	if meta, found, merr := st.GetServiceMeta(ctx, svc); merr == nil && found {
		root = meta.Path
	}

	// Languages touched: read from the STORE's own indexed symbols
	// (FilePath), never from re-reading the corpus off disk — a service
	// ingested elsewhere, or whose recorded root no longer exists on this
	// machine, still gets an honest per-language tier breakdown this way.
	var langPaths []string
	if nodes, nerr := c.ListAllSymbols(ctx, svc); nerr == nil {
		seen := map[string]bool{}
		for _, n := range nodes {
			if !seen[n.FilePath] {
				seen[n.FilePath] = true
				langPaths = append(langPaths, n.FilePath)
			}
		}
	}
	row.Languages = conserve.TiersByLanguage(ctx, conserve.Input{Service: svc, Store: st, ChangedFiles: langPaths})

	// Inventory and the receipts sample both need actual file CONTENT —
	// unlike the tier breakdown above, they degrade honestly to empty/
	// unproven when the corpus is not readable from here (a remote or
	// moved service root), rather than falling back to store metadata.
	sources, srcErr := c.Sources(ctx, svc)
	if srcErr != nil {
		sources = nil // best effort: an unreadable/unresolvable corpus reports empty, never fails the row
	}

	if gaps, gerr := certify.Gaps(ctx, st, svc, root, nil); gerr == nil {
		row.CertificatesDeclared = gaps.Summary.Declared
		row.CertificatesLive = gaps.Summary.Live
		if gaps.Summary.Declared > 0 {
			row.CertificateDensity = round4(float64(gaps.Summary.Live) / float64(gaps.Summary.Declared))
		}
	}

	files := make([]inventory.File, 0, len(sources))
	for _, f := range sources {
		files = append(files, inventory.File{Path: f.Path, Content: f.Content})
	}
	items := inventory.Extract(files, evidenceSurfacePatterns(root))
	row.InventoryByKind = map[string]int{}
	for _, it := range items {
		row.InventoryByKind[it.Kind]++
	}
	row.InventoryTotal = len(items)

	row.ReceiptsRederive = receiptsRederiveStatus(ctx, root, sources)
	return row
}

// evidenceSurfacePatterns best-effort reads root's `.fathom/config.toml`
// for `[[surface.pattern]]` entries (CV-17) — the same read cmd/fathom/
// conserve.go's plane (internal/conserve/surface.go's own repoSurface
// Patterns, unexported there) performs, duplicated here at this small
// size rather than exporting a one-line function across a package
// boundary for a single caller. A missing/unreadable config, or an empty
// root, yields no patterns — the built-in inventory kinds keep working
// regardless.
func evidenceSurfacePatterns(root string) []inventory.Pattern {
	if root == "" {
		return nil
	}
	loaded, err := config.Load("", filepath.Join(root, ".fathom", "config.toml"))
	if err != nil {
		return nil
	}
	src := config.SurfacePatterns(loaded.Config)
	if len(src) == 0 {
		return nil
	}
	out := make([]inventory.Pattern, len(src))
	for i, p := range src {
		out[i] = inventory.Pattern{Kind: p.Kind, Glob: p.Glob, Regex: p.Regex}
	}
	return out
}

// receiptsRederiveStatus answers "whether receipts re-derive (Go only)"
// (design §5) for one service: it samples the alphabetically first
// non-test Go source file's first function declaration, builds a
// core.ReceiptKindSymbolLocation receipt for it against root's current
// HEAD, and re-derives it from a BARE git checkout with go/parser alone
// (receiptverify.Verify, VB-20) — proving the receipt-verification
// pipeline this service's own claims could be checked through actually
// works end to end, not merely that fathom asserts something. Every
// precondition that cannot be met (no corpus root recorded, no Go
// sources, no go.mod, not a git checkout, no declaration to sample)
// degrades to a named, honest "unproven"/"not-applicable" — never a
// silent true or false.
func receiptsRederiveStatus(ctx context.Context, root string, sources []core.SourceFile) string {
	if root == "" {
		return "unproven: no corpus root recorded"
	}
	var goFiles []core.SourceFile
	for _, f := range sources {
		if f.Lang == "go" && !strings.HasSuffix(f.Path, "_test.go") {
			goFiles = append(goFiles, f)
		}
	}
	if len(goFiles) == 0 {
		return "not-applicable: no Go sources"
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return "unproven: no go.mod at the corpus root"
	}
	out, err := certify.ExecGit(root, "rev-parse", "HEAD")
	if err != nil {
		return "unproven: not a git checkout"
	}
	sha := strings.TrimSpace(string(out))

	sort.Slice(goFiles, func(i, j int) bool { return goFiles[i].Path < goFiles[j].Path })
	for _, f := range goFiles {
		spans, perr := publicloc.FuncSpans(f.Content, f.Path)
		if perr != nil || len(spans) == 0 {
			continue
		}
		sort.Slice(spans, func(i, j int) bool { return spans[i].Start < spans[j].Start })
		span := spans[0]
		r := core.Receipt{
			Kind: core.ReceiptKindSymbolLocation, Symbol: span.Name,
			File: f.Path, Line: span.Start, IndexCommit: sha,
		}
		outcome, verr := receiptverify.Verify(ctx, root, r)
		if verr != nil {
			return fmt.Sprintf("unproven: %v", verr)
		}
		if outcome.OK {
			return "live"
		}
		return fmt.Sprintf("difference: %s", outcome.Diff)
	}
	return "unproven: no Go declarations found"
}

// round4 rounds to 4 decimal places — enough precision for a share/
// density ratio to be meaningful without inviting float noise into JSON
// output that gets diffed or golden-tested.
func round4(f float64) float64 {
	return float64(int(f*10000+0.5)) / 10000
}

// printEvidenceReport renders rows as a short, scannable text table — one
// line per service, mirroring cmd/fathom/conserve.go's printConserveReport
// shape.
func printEvidenceReport(w io.Writer, rows []evidenceRow) {
	for _, r := range rows {
		fmt.Fprintf(w, "report evidence: %s: typed %d/%d (%.0f%%), certificates %d/%d live (%.0f%%), inventory %d item(s), receipts: %s\n",
			r.Service, r.TypedEdges, r.Edges, r.TypedShare*100,
			r.CertificatesLive, r.CertificatesDeclared, r.CertificateDensity*100,
			r.InventoryTotal, r.ReceiptsRederive)
		if len(r.Languages) > 0 {
			fmt.Fprint(w, "    languages:")
			for _, t := range r.Languages {
				fmt.Fprintf(w, " %s=%s", t.Language, t.Tier)
			}
			fmt.Fprintln(w)
		}
		if len(r.InventoryByKind) > 0 {
			kinds := make([]string, 0, len(r.InventoryByKind))
			for k := range r.InventoryByKind {
				kinds = append(kinds, k)
			}
			sort.Strings(kinds)
			fmt.Fprint(w, "    inventory:")
			for _, k := range kinds {
				fmt.Fprintf(w, " %s=%d", k, r.InventoryByKind[k])
			}
			fmt.Fprintln(w)
		}
	}
}
