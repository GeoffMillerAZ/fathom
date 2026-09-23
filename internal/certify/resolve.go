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
// clear. behaviour never resolves to anything at all (design doc §1: tier
// never exceeds doc-claimed — CompileCertifications enforces that cap).
import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/symbolnav"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/intent"
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
		// Declared only — nothing to resolve, nothing to reach. The
		// caller (CompileCertifications) caps this kind's tier at
		// doc-claimed regardless of test existence/reach/run.
		t.Resolved = true
		return t, nil
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
