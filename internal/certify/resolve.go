package certify

// resolve.go — ResolveTarget (TC-7, design doc §1's kind→resolution
// table): a certification's kind+id resolves deterministically, or the
// certificate is contradicted ("target does not resolve").
//
// invariant/incident resolve to a DOCUMENT ROW/RECORD, not to a code
// symbol — docs/intent/invariants.md and incidents.md name what fathom
// already tracks as intent corpus facts, and neither one binds to a
// specific function the way a spec id, a governing rule's file#heading,
// or a port interface does. So Target.Symbols stays empty for these two
// kinds: CompileCertifications treats an empty Symbols set as "reach is
// vacuously satisfied once the target itself resolves" (types.go's own
// doc comment) — the certificate's tier still climbs on the test's own
// existence and attested run, it simply has no SEPARATE reach check to
// clear. behaviour resolves against the HEAD capability inventory
// (resolveBehaviour, ADR-044 §2.4): an id that names no inventory key at
// all is now genuinely "target does not resolve" (contradicted), rather
// than always resolving — so characterization/golden tests finally
// certify something checkable — while Target.Symbols STILL stays empty
// (design doc §1: tier never exceeds doc-claimed for this kind, whatever
// it resolves against).
import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/inventory"
	"github.com/geoffmilleraz/fathom/internal/analysis/symbolnav"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/sdd"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// incidentHeadingRe matches an incident record heading, "### INC-21"
// (design doc §1's own grammar; internal/intent's own incidentRe is
// unexported).
var incidentHeadingRe = regexp.MustCompile(`(?m)^###\s+(INC-\d+)\b`)

// ResolveTarget resolves kind+id against service's indexed corpus,
// per design doc §1's table. It never returns a non-nil error for an
// ordinary "does not resolve" outcome (Target.Resolved stays false, with
// Target.Reason set) — only a genuine store/I-O failure is returned as
// an error.
func ResolveTarget(ctx context.Context, st store.Store, service, kind, id string) (Target, error) {
	t := Target{Kind: kind, ID: id}

	switch extractor.CertifyKind(kind) {
	case extractor.CertifyKindBehaviour:
		return resolveBehaviour(ctx, st, service, t)
	case extractor.CertifyKindInvariant:
		return resolveInvariant(ctx, st, service, t)
	case extractor.CertifyKindIncident:
		return resolveIncident(ctx, st, service, t)
	case extractor.CertifyKindSpec:
		return resolveSpec(ctx, st, service, t)
	case extractor.CertifyKindRule:
		return resolveRule(ctx, st, service, t)
	case extractor.CertifyKindPort:
		return resolvePort(ctx, st, service, t)
	default:
		// An unrecognized kind never resolves — same wording as an
		// unknown id within a known kind (design doc §1).
		t.Reason = "target does not resolve"
		return t, nil
	}
}

func resolveInvariant(ctx context.Context, st store.Store, service string, t Target) (Target, error) {
	root, ok, err := serviceRoot(ctx, st, service)
	if err != nil {
		return t, err
	}
	if !ok {
		t.Reason = "target does not resolve"
		return t, nil
	}
	corpus, err := intent.Load(root)
	if err != nil {
		return t, err
	}
	for _, inv := range corpus.Invariants {
		if inv.ID == t.ID {
			t.Resolved = true
			if inv.File != "" {
				t.Files = []string{inv.File}
			}
			return t, nil
		}
	}
	t.Reason = "target does not resolve"
	return t, nil
}

func resolveIncident(ctx context.Context, st store.Store, service string, t Target) (Target, error) {
	root, ok, err := serviceRoot(ctx, st, service)
	if err != nil {
		return t, err
	}
	if !ok {
		t.Reason = "target does not resolve"
		return t, nil
	}
	rel := filepath.Join("docs", "intent", "incidents.md")
	body, rerr := os.ReadFile(filepath.Join(root, rel))
	if rerr != nil {
		t.Reason = "target does not resolve"
		return t, nil
	}
	for _, m := range incidentHeadingRe.FindAllStringSubmatch(string(body), -1) {
		if m[1] == t.ID {
			t.Resolved = true
			t.Files = []string{filepath.ToSlash(rel)}
			return t, nil
		}
	}
	t.Reason = "target does not resolve"
	return t, nil
}

// resolveBehaviour resolves a `certifies: behaviour <key>` target against
// service's HEAD capability inventory (CV-22, ADR-044 §2.4): t.ID must
// name an existing `internal/analysis/inventory.Extract` item key (e.g.
// `route:GET /v1/x`) — an id that matches nothing is "target does not
// resolve" (contradicted, ResolveTarget's caller sets
// core.ClaimContradicted), so a characterization/golden test finally
// certifies something checkable instead of an arbitrary free-text label.
// Target.Symbols stays empty regardless (types.go's own doc comment: this
// kind's tier never exceeds doc-claimed).
//
// The inventory is built from every file the store already knows about
// for service (serviceSymbols' FilePath set, unioned with
// ListSourceRefs — the fullest known-file list a store affords without a
// bare directory walk), read fresh off disk under the service's recorded
// corpus root: the store persists SYMBOLS, never raw file content
// (internal/store has no content-serving method), so this is the same
// "re-read a known file from disk" pattern resolveIncident already uses
// for a single file, generalised to the whole known corpus. An unreadable
// or since-deleted file contributes nothing rather than failing the
// whole resolve.
func resolveBehaviour(ctx context.Context, st store.Store, service string, t Target) (Target, error) {
	root, ok, err := serviceRoot(ctx, st, service)
	if err != nil {
		return t, err
	}
	if !ok {
		t.Reason = "target does not resolve"
		return t, nil
	}

	paths, err := knownServiceFiles(ctx, st, service)
	if err != nil {
		return t, err
	}
	files := make([]inventory.File, 0, len(paths))
	for _, rel := range paths {
		content, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if rerr != nil {
			continue
		}
		files = append(files, inventory.File{Path: rel, Content: content})
	}

	items := inventory.Extract(files, projectSurfacePatterns(root))
	for _, it := range items {
		if it.Key == t.ID {
			t.Resolved = true
			return t, nil
		}
	}
	t.Reason = "target does not resolve"
	return t, nil
}

// knownServiceFiles returns the sorted, deduplicated union of every file
// path the store has EVER recorded evidence for under service: every
// indexed symbol's FilePath (serviceSymbols) plus ListSourceRefs (which
// additionally covers fingerprint-only files with no registered
// extractor, e.g. a .proto or a plain-text env file that produced no
// symbol of its own).
func knownServiceFiles(ctx context.Context, st store.Store, service string) ([]string, error) {
	nodes, err := serviceSymbols(ctx, st, service)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		if n.FilePath != "" {
			set[n.FilePath] = true
		}
	}
	refs, err := st.ListSourceRefs(ctx, service)
	if err != nil {
		return nil, err
	}
	for _, r := range refs {
		if r != "" {
			set[r] = true
		}
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

// projectSurfacePatterns best-effort reads root's OWN `.fathom/config.toml`
// for `[[surface.pattern]]` entries (CV-17) — project layer only,
// deliberately skipping the operator's personal user-level config.toml:
// resolveBehaviour scans a SERVICE's corpus, not the caller's own
// environment, and a repo's configured surface patterns are a property of
// that repo, not of whoever happens to be running the resolve. A missing
// or unreadable config.toml (no project file at all is the ordinary case)
// yields no patterns rather than an error — the built-in kinds still work.
func projectSurfacePatterns(root string) []inventory.Pattern {
	loaded, err := config.Load("", projectdir.Config.Path(root))
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

func resolveSpec(ctx context.Context, st store.Store, service string, t Target) (Target, error) {
	nodes, err := serviceSymbols(ctx, st, service)
	if err != nil {
		return t, err
	}
	// sdd.ExtractRefs matches the bracketed doc convention ("[US-042]") and
	// returns the brackets WITH the id; the `// certifies: spec US-042`
	// marker grammar (design doc §1) never writes the brackets — strip
	// them before comparing so both spellings name the same target.
	want := strings.Trim(t.ID, "[]")
	var symbols, files []string
	for _, n := range nodes {
		for _, ref := range sdd.ExtractRefs(n.Signature + " " + n.Name) {
			if strings.Trim(ref, "[]") == want {
				symbols = append(symbols, n.Key())
				files = append(files, n.FilePath)
				break
			}
		}
	}
	if len(symbols) == 0 {
		t.Reason = "target does not resolve"
		return t, nil
	}
	t.Resolved = true
	t.Symbols = sortedUnique(symbols)
	t.Files = sortedUnique(files)
	return t, nil
}

func resolveRule(ctx context.Context, st store.Store, service string, t Target) (Target, error) {
	claims, err := st.ClaimsForService(ctx, service, "obligations")
	if err != nil {
		return t, err
	}
	var match *core.SemanticClaim
	for i := range claims {
		if claims[i].Subject == t.ID && claims[i].Status.Served() {
			match = &claims[i]
			break
		}
	}
	if match == nil {
		t.Reason = "target does not resolve"
		return t, nil
	}
	t.Resolved = true
	t.Files = sortedUnique(append([]string(nil), match.Scope...))

	nodes, err := serviceSymbols(ctx, st, service)
	if err != nil {
		return t, err
	}
	fileSet := toSet(t.Files)
	var symbols []string
	for _, n := range nodes {
		if fileSet[n.FilePath] {
			symbols = append(symbols, n.Key())
		}
	}
	t.Symbols = sortedUnique(symbols)
	return t, nil
}

func resolvePort(ctx context.Context, st store.Store, service string, t Target) (Target, error) {
	nodes, err := serviceSymbols(ctx, st, service)
	if err != nil {
		return t, err
	}
	bareName := t.ID
	if i := strings.LastIndex(bareName, "."); i >= 0 {
		bareName = bareName[i+1:]
	}
	var iface *core.SymbolNode
	for i := range nodes {
		if nodes[i].Kind == core.KindGoInterface && nodes[i].Name == bareName {
			n := nodes[i]
			iface = &n
			break
		}
	}
	if iface == nil {
		t.Reason = "target does not resolve"
		return t, nil
	}
	t.Resolved = true
	symbols := []string{iface.Key()}
	files := []string{iface.FilePath}
	for _, implName := range symbolnav.Implementations(nodes, bareName) {
		for _, n := range nodes {
			if n.Name == implName {
				symbols = append(symbols, n.Key())
				files = append(files, n.FilePath)
			}
		}
	}
	t.Symbols = sortedUnique(symbols)
	t.Files = sortedUnique(files)
	return t, nil
}

// serviceRoot resolves service's corpus root from its ingest metadata.
// ok is false (never an error) when the service has no recorded metadata
// or an empty path — a normal "cannot resolve" outcome, not a store
// failure.
func serviceRoot(ctx context.Context, st store.Store, service string) (string, bool, error) {
	meta, found, err := st.GetServiceMeta(ctx, service)
	if err != nil {
		return "", false, err
	}
	if !found || meta.Path == "" {
		return "", false, nil
	}
	return meta.Path, true, nil
}

// serviceSymbols returns every indexed symbol node for service.
func serviceSymbols(ctx context.Context, st store.Store, service string) ([]core.SymbolNode, error) {
	obs, err := st.QueryObservations(ctx, service, "")
	if err != nil {
		return nil, err
	}
	nodes := make([]core.SymbolNode, 0, len(obs))
	for _, o := range obs {
		nodes = append(nodes, o.ToSymbolNode())
	}
	return nodes, nil
}

func sortedUnique(ss []string) []string {
	if len(ss) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(ss))
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
