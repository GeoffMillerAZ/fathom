package certify

// port.go — PortGaps (TC-20, design doc §5): every exported Go interface
// in the service's index with at least one indexed implementation is a
// "port" target — a certificate-worthy fact even with nothing ever
// declared for it, the SAME "corpus record with nothing declared is
// still a Gap" convention Gaps' own invariant/incident sourcing already
// uses (gaps.go's own doc comment). An interface with ZERO implementers
// is not a port worth certifying: nothing could ever dispatch through
// it, so there is nothing a conformance test would even exercise.
//
// Each implementer's evidence is tiered the SAME way find_symbol's own
// "implementations" field is (internal/mcp/implementations.go's own doc
// comment): typed (a persisted EdgeIfaceDispatch edge) > assertion (a
// `var _ Iface = (*T)(nil)` compile-time assertion) > method_set
// (structural coverage, the fallback). The ladder is MIRRORED here
// rather than imported: internal/mcp already imports internal/certify
// (review_tool.go, verify_plan_tool.go), so certify importing mcp back
// would cycle. This is a second instance of the same evidence pattern,
// exactly like compile.go's own Layer/Kind consts mirror the findings
// layer's claim shape (compile.go's own doc comment) rather than sharing
// a type across an import that cannot exist.
//
// Unlike mcp/implementations.go (which reads source text through a
// connector-backed Sources() call), this reads directly off corpusRoot —
// the service's own indexed path (the SAME parameter Gaps itself takes).
// Interface method NAMES are not indexed as their own symbols at all
// (Go forbids a method whose receiver names an interface type — see
// mcp/implementations.go's implementationsForMethod doc comment), so the
// interface's own file must be parsed; assertion evidence likewise needs
// raw source text no observation carries. Both reads degrade silently on
// any I/O or parse failure (a moved/deleted file, a syntax error) to the
// next-weaker signal, never an error — the same best-effort discipline
// every other corpus read in this package already uses.
import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	readmanifest "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/read-manifest"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// PortImplementation is one implementer PortGaps found for a port
// target, with the evidence tier that backs it.
type PortImplementation struct {
	Name     string
	FilePath string
	Line     int
	Via      string // "typed" | "assertion" | "method_set"
}

// PortTarget is one exported interface PortGaps found with at least one
// indexed implementation. ID is the interface's bare name — the SAME
// identity ResolveTarget's own resolvePort (resolve.go) already matches
// a declared "port" certification's TargetID against (a bare-name match,
// first found wins on a same-name collision across packages — a
// pre-existing, documented simplification this row does not change).
type PortTarget struct {
	ID              string
	Implementations []PortImplementation
	// File/Line: the interface's own declaration site (n.FilePath/n.Line —
	// already read off the index by this same loop).
	File string
	Line int
}

// PortGaps returns every exported Go interface in service's index with
// at least one indexed implementation, sorted by ID. corpusRoot is the
// service's indexed path (ServiceMeta.Path); "" yields no targets — an
// interface's own method names cannot be recovered without its source
// text, so there is nothing safe to report (the same best-effort degrade
// Gaps' own invariant/incident sourcing already uses for an absent
// corpus).
func PortGaps(ctx context.Context, st store.Store, service, corpusRoot string) ([]PortTarget, error) {
	return portGaps(ctx, st, service, corpusRoot, readmanifest.NewSet())
}

// portGaps is PortGaps recording every source file it reads in reads, so
// Gaps can name an unreadable one instead of reporting fewer ports (I38).
func portGaps(ctx context.Context, st store.Store, service, corpusRoot string, reads *readmanifest.Set) ([]PortTarget, error) {
	if corpusRoot == "" {
		return nil, nil
	}
	nodes, err := serviceSymbols(ctx, st, service)
	if err != nil {
		return nil, fmt.Errorf("PortGaps: serviceSymbols: %w", err)
	}
	edges, err := st.TypedEdges(ctx, service)
	if err != nil {
		return nil, fmt.Errorf("PortGaps: TypedEdges: %w", err)
	}

	var out []PortTarget
	for _, n := range nodes {
		if n.Kind != core.KindGoInterface || !isExportedName(n.Name) {
			continue
		}
		impls := findPortImplementations(corpusRoot, n, nodes, edges, reads)
		if len(impls) == 0 {
			continue
		}
		out = append(out, PortTarget{ID: n.Name, Implementations: impls, File: n.FilePath, Line: n.Line})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// portRemedy is the remedy line for a "port" gap: it names every
// implementer PortGaps found, with its own evidence tier, so the
// remedy carries the SAME "keep each implementation's evidence tier"
// detail the plan row asks for without any change to the fixed
// GapTarget/GapCertificate wire shape (design doc's own contract for
// the Review Kit adapter).
func portRemedy(id string, impls []PortImplementation) string {
	if len(impls) == 0 {
		// Should not happen (PortGaps never sources a zero-implementer
		// interface), but never fabricate detail this function does not
		// have.
		return fmt.Sprintf("add a conformance-strength certificate for port %s", id)
	}
	parts := make([]string, 0, len(impls))
	for _, im := range impls {
		parts = append(parts, fmt.Sprintf("%s (%s)", im.Name, im.Via))
	}
	return fmt.Sprintf(
		"no conformance certificate for port %s: implementations — %s; add \"// certifies: port %s (conformance)\" above a Test* function that exercises every implementation, or a #Certifies entry in the suite's tests/*.cue manifest",
		id, strings.Join(parts, ", "), id,
	)
}

func isExportedName(name string) bool {
	if name == "" {
		return false
	}
	first := rune(name[0])
	return first >= 'A' && first <= 'Z'
}

// findPortImplementations evidences iface's implementers typed >
// assertion > method_set (this file's own doc comment).
func findPortImplementations(corpusRoot string, iface core.SymbolNode, nodes []core.SymbolNode, edges []core.Edge, reads *readmanifest.Set) []PortImplementation {
	methods := portInterfaceMethodNames(corpusRoot, iface, reads)
	if len(methods) == 0 {
		return nil
	}
	candidates := portMethodSetCandidates(nodes, methods, iface.Name)
	if len(candidates) == 0 {
		return nil
	}
	if typed := portTypedCandidates(edges, nodes, candidates, methods); len(typed) > 0 {
		return renderPortImplementations(typed, "typed")
	}
	if asserted := portAssertionCandidates(corpusRoot, iface.Name, candidates, reads); len(asserted) > 0 {
		return renderPortImplementations(asserted, "assertion")
	}
	return renderPortImplementations(candidates, "method_set")
}

// portInterfaceMethodNames parses iface's own source file (corpusRoot +
// iface.FilePath) and returns the set of method names it directly
// declares. A bare inline `interface{...}` embed is expanded one level;
// a NAMED embed is left unresolved (only makes the match MORE
// conservative, never wrong — a type satisfying the embedded methods too
// still needs to satisfy the ones found here). Any read/parse failure
// returns nil — never a fabricated method set.
func portInterfaceMethodNames(corpusRoot string, iface core.SymbolNode, reads *readmanifest.Set) map[string]bool {
	content, ok := reads.ReadFile(filepath.Join(corpusRoot, filepath.FromSlash(iface.FilePath)))
	if !ok {
		return nil
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, iface.FilePath, content, 0)
	if err != nil {
		return nil
	}
	methods := map[string]bool{}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name == nil || ts.Name.Name != iface.Name {
				continue
			}
			it, ok := ts.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}
			collectPortInterfaceMethods(it, methods)
		}
	}
	if len(methods) == 0 {
		return nil
	}
	return methods
}

func collectPortInterfaceMethods(it *ast.InterfaceType, out map[string]bool) {
	if it == nil || it.Methods == nil {
		return
	}
	for _, field := range it.Methods.List {
		if len(field.Names) > 0 {
			for _, n := range field.Names {
				out[n.Name] = true
			}
			continue
		}
		if embedded, ok := field.Type.(*ast.InterfaceType); ok {
			collectPortInterfaceMethods(embedded, out)
		}
	}
}

// portMethodSetCandidates returns every indexed type (other than iface
// itself) whose GoMethod symbols structurally cover every name in
// methods.
func portMethodSetCandidates(nodes []core.SymbolNode, methods map[string]bool, ifaceName string) map[string]core.SymbolNode {
	typeMethods := map[string]map[string]bool{}
	firstMethod := map[string]core.SymbolNode{}
	for _, n := range nodes {
		if n.Kind != core.KindGoMethod {
			continue
		}
		recv, meth := splitPortReceiverMethod(n.Name)
		if recv == "" || recv == ifaceName {
			continue
		}
		if typeMethods[recv] == nil {
			typeMethods[recv] = map[string]bool{}
		}
		typeMethods[recv][meth] = true
		if cur, ok := firstMethod[recv]; !ok || n.Line < cur.Line {
			firstMethod[recv] = n
		}
	}
	typeDecls := map[string]core.SymbolNode{}
	for _, n := range nodes {
		if n.Name == ifaceName {
			continue
		}
		if n.Kind == core.KindGoType || n.Kind == core.KindGoInterface {
			if _, ok := typeDecls[n.Name]; !ok {
				typeDecls[n.Name] = n
			}
		}
	}
	out := map[string]core.SymbolNode{}
	for typeName, have := range typeMethods {
		covers := true
		for m := range methods {
			if !have[m] {
				covers = false
				break
			}
		}
		if !covers {
			continue
		}
		if decl, ok := typeDecls[typeName]; ok {
			out[typeName] = decl
			continue
		}
		out[typeName] = core.SymbolNode{Name: typeName, FilePath: firstMethod[typeName].FilePath, Line: firstMethod[typeName].Line}
	}
	return out
}

func splitPortReceiverMethod(name string) (recv, method string) {
	for i := 0; i < len(name); i++ {
		if name[i] == '.' {
			return name[:i], name[i+1:]
		}
	}
	return "", name
}

// portTypedCandidates narrows candidates to those confirmed by a
// persisted EdgeIfaceDispatch edge targeting one of iface's own methods
// on that candidate type.
func portTypedCandidates(edges []core.Edge, nodes []core.SymbolNode, candidates map[string]core.SymbolNode, methods map[string]bool) map[string]core.SymbolNode {
	if len(edges) == 0 || len(candidates) == 0 {
		return nil
	}
	byKey := map[string]core.SymbolNode{}
	for _, n := range nodes {
		if n.Kind == core.KindGoMethod {
			byKey[n.Key()] = n
		}
	}
	out := map[string]core.SymbolNode{}
	for _, e := range edges {
		if e.Kind != core.EdgeIfaceDispatch {
			continue
		}
		node, ok := byKey[e.ToSymbol]
		if !ok {
			continue
		}
		recv, meth := splitPortReceiverMethod(node.Name)
		if !methods[meth] {
			continue
		}
		if decl, ok := candidates[recv]; ok {
			out[recv] = decl
		}
	}
	return out
}

// portAssertionCandidates scans each CANDIDATE's own declaring file (the
// idiomatic location for a `var _ Iface = (*T)(nil)` assertion; iface's
// own file is scanned too, on the chance the assertion lives beside the
// interface instead) for a compile-time assertion naming iface,
// returning the subset of candidates it confirms. A file that fails to
// read is skipped, never an error — the caller falls back to
// method_set.
func portAssertionCandidates(corpusRoot, ifaceName string, candidates map[string]core.SymbolNode, reads *readmanifest.Set) map[string]core.SymbolNode {
	re := regexp.MustCompile(`var\s+_\s+(?:[A-Za-z_]\w*\.)?` + regexp.QuoteMeta(ifaceName) + `\s*=\s*(?:\(\*(\w+)\)|(\w+))`)
	out := map[string]core.SymbolNode{}
	seen := map[string]bool{}
	for _, decl := range candidates {
		if decl.FilePath == "" || seen[decl.FilePath] {
			continue
		}
		seen[decl.FilePath] = true
		content, ok := reads.ReadFile(filepath.Join(corpusRoot, filepath.FromSlash(decl.FilePath)))
		if !ok {
			continue
		}
		for _, m := range re.FindAllStringSubmatch(string(content), -1) {
			t := m[1]
			if t == "" {
				t = m[2]
			}
			if d, ok := candidates[t]; ok {
				out[t] = d
			}
		}
	}
	return out
}

func renderPortImplementations(m map[string]core.SymbolNode, via string) []PortImplementation {
	out := make([]PortImplementation, 0, len(m))
	for name, n := range m {
		out = append(out, PortImplementation{Name: name, FilePath: n.FilePath, Line: n.Line, Via: via})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
