package mcp

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
	"github.com/geoffmilleraz/fathom/internal/refresh"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func ifaceOf[T any]() reflect.Type { return reflect.TypeOf((*T)(nil)).Elem() }

// assertedInterfaces is every interface type internal/mcp's product code
// type-asserts a value to, keyed as the assertion spells it. The scan in
// TestAutoRefreshForwardsEveryClientCapability keeps it exact: an assertion
// to an interface missing here fails the test until it is registered (and
// so checked), and an entry nothing asserts any more fails until removed.
var assertedInterfaces = map[string]reflect.Type{
	"autoRefreshStatsBackend":                    ifaceOf[autoRefreshStatsBackend](),
	"certificationsBackend":                      ifaceOf[certificationsBackend](),
	"claimFeedbackBackend":                       ifaceOf[claimFeedbackBackend](),
	"claimsAnchoredBackend":                      ifaceOf[claimsAnchoredBackend](),
	"claimsListBackend":                          ifaceOf[claimsListBackend](),
	"claimsSummaryBackend":                       ifaceOf[claimsSummaryBackend](),
	"classifiedZeroBackend":                      ifaceOf[classifiedZeroBackend](),
	"compileBackend":                             ifaceOf[compileBackend](),
	"compileStateBackend":                        ifaceOf[compileStateBackend](),
	"conserveBackend":                            ifaceOf[conserveBackend](),
	"docDriftBackend":                            ifaceOf[docDriftBackend](),
	"docLinksAsOfBackend":                        ifaceOf[docLinksAsOfBackend](),
	"docsSearchBackend":                          ifaceOf[docsSearchBackend](),
	"exclusionsBackend":                          ifaceOf[exclusionsBackend](),
	"findingsBackend":                            ifaceOf[findingsBackend](),
	"fleetCallIndex":                             ifaceOf[fleetCallIndex](),
	"fleetCtxBackend":                            ifaceOf[fleetCtxBackend](),
	"fleetNameLookup":                            ifaceOf[fleetNameLookup](),
	"fleetRootStatusBackend":                     ifaceOf[fleetRootStatusBackend](),
	"fleetSummaryBackend":                        ifaceOf[fleetSummaryBackend](),
	"fleetSymbolAggregateBackend":                ifaceOf[fleetSymbolAggregateBackend](),
	"fleetSymbolAggregateErrBackend":             ifaceOf[fleetSymbolAggregateErrBackend](),
	"freshnessEnsurer":                           ifaceOf[freshnessEnsurer](),
	"freshnessProber":                            ifaceOf[freshnessProber](),
	"ingestAtBackend":                            ifaceOf[ingestAtBackend](),
	"ingestStateBackend":                         ifaceOf[ingestStateBackend](),
	"layerCoverageBackend":                       ifaceOf[layerCoverageBackend](),
	"obligationLayerBackend":                     ifaceOf[obligationLayerBackend](),
	"obligationsAsOfBackend":                     ifaceOf[obligationsAsOfBackend](),
	"obligationsBackend":                         ifaceOf[obligationsBackend](),
	"obligationsIndexCheckBackend":               ifaceOf[obligationsIndexCheckBackend](),
	"perLayerCoverageBackend":                    ifaceOf[perLayerCoverageBackend](),
	"pkgDocsSearchBackend":                       ifaceOf[pkgDocsSearchBackend](),
	"rawFallbackBackend":                         ifaceOf[rawFallbackBackend](),
	"rootRawFallbackBackend":                     ifaceOf[rootRawFallbackBackend](),
	"registrySurgeryBackend":                     ifaceOf[registrySurgeryBackend](),
	"reviewClaimsBackend":                        ifaceOf[reviewClaimsBackend](),
	"reviewObligationsBackend":                   ifaceOf[reviewObligationsBackend](),
	"rootObligationsBackend":                     ifaceOf[rootObligationsBackend](),
	"rootObligationsLayerBackend":                ifaceOf[rootObligationsLayerBackend](),
	"scipImportBackend":                          ifaceOf[scipImportBackend](),
	"serviceWatermarker":                         ifaceOf[serviceWatermarker](),
	"sourceFileReader":                           ifaceOf[sourceFileReader](),
	"sourcePathLister":                           ifaceOf[sourcePathLister](),
	"statsBackend":                               ifaceOf[statsBackend](),
	"storeIdentifier":                            ifaceOf[storeIdentifier](),
	"storeWatermarker":                           ifaceOf[storeWatermarker](),
	"suppressionTokener":                         ifaceOf[suppressionTokener](),
	"typedEdgeStatusBackend":                     ifaceOf[typedEdgeStatusBackend](),
	"serviceref.RefReader":                       ifaceOf[serviceref.RefReader](),
	"interface{ Name() string }":                 ifaceOf[interface{ Name() string }](),
	"interface{ RefreshQueue() *refresh.Queue }": ifaceOf[interface{ RefreshQueue() *refresh.Queue }](),
}

// notTheServedBackend names, per interface, the one operand its assertions
// may have without the check: a value that is never the backend a tool is
// served (the wrapper reading the backend it wraps, a fleet backend). An
// assertion of the same interface on any other operand is checked.
var notTheServedBackend = map[string]string{
	"freshnessProber":    "a.QueryInterface", // autoRefreshBackend's own drift probe of the backend it wraps
	"serviceWatermarker": "a.QueryInterface",
	"ingestStateBackend": "a.QueryInterface",
	"fleetNameLookup":    "fleet", // crossservice.go reads the fleet backend, a separate value
}

// assertedConcreteSelectors are the package-qualified asserted types that
// are not interfaces (a selector's kind is not visible to the parser).
var assertedConcreteSelectors = map[string]bool{"time.Time": true}

// TestAutoRefreshForwardsEveryClientCapability closes the O7 forwarding
// class (O7, O1068, O1072; EW-80): every daemon serves through
// autoRefreshBackend, whose embedded QueryInterface promotes only that
// interface's own methods, so an optional capability *client.Client has and
// the wrapper does not forward is silently stripped on every real daemon
// (O1072: review_diff's findings_persisted:false). It scans internal/mcp's
// product files for every interface a value is type-asserted to and
// requires the wrapper to satisfy each one the client satisfies.
func TestAutoRefreshForwardsEveryClientCapability(t *testing.T) {
	asserted, checked := scanAssertedInterfaces(t)
	clientT := reflect.TypeOf((*client.Client)(nil))
	wrapperT := reflect.TypeOf((*autoRefreshBackend)(nil))
	for _, name := range asserted {
		if !checked[name] {
			continue
		}
		it := assertedInterfaces[name]
		if clientT.Implements(it) && !wrapperT.Implements(it) {
			var missing []string
			for i := 0; i < it.NumMethod(); i++ {
				if m := it.Method(i); !hasMethod(wrapperT, m.Name) {
					missing = append(missing, m.Name)
				}
			}
			t.Errorf("*autoRefreshBackend does not satisfy %s, which *client.Client does: the auto-refresh wrapper strips the capability on every daemon (the O7 class); forward %s",
				name, strings.Join(missing, ", "))
		}
	}
}

func hasMethod(t reflect.Type, name string) bool {
	_, ok := t.MethodByName(name)
	return ok
}

// scanAssertedInterfaces parses the package's non-test files and returns
// the registry keys of every interface a type assertion or type switch
// names, failing on an interface the registry does not hold and on a
// registry entry nothing asserts. checked holds the interfaces asserted on
// at least one operand notTheServedBackend does not exempt.
func scanAssertedInterfaces(t *testing.T) (names []string, checked map[string]bool) {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var parsed []*ast.File
	declared := map[string]bool{} // interface types declared in the package
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		af, err := parser.ParseFile(fset, f, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		parsed = append(parsed, af)
		ast.Inspect(af, func(n ast.Node) bool {
			if ts, ok := n.(*ast.TypeSpec); ok {
				if _, ok := ts.Type.(*ast.InterfaceType); ok {
					declared[ts.Name.Name] = true
				}
			}
			return true
		})
	}
	if len(parsed) == 0 {
		t.Fatal("no product files scanned")
	}
	spell := func(e ast.Expr) string {
		var b strings.Builder
		if err := printer.Fprint(&b, fset, e); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	seen := map[string]bool{}
	checked = map[string]bool{}
	note := func(e, operand ast.Expr) {
		name := ""
		switch x := e.(type) {
		case *ast.Ident:
			if declared[x.Name] {
				name = x.Name
			}
		case *ast.InterfaceType:
			name = spell(x)
		case *ast.SelectorExpr:
			if s := spell(x); !assertedConcreteSelectors[s] {
				name = s
			}
		}
		if name == "" {
			return
		}
		seen[name] = true
		if notTheServedBackend[name] != spell(operand) {
			checked[name] = true
		}
	}
	for _, af := range parsed {
		ast.Inspect(af, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.TypeAssertExpr:
				if x.Type != nil {
					note(x.Type, x.X)
				}
			case *ast.TypeSwitchStmt:
				operand := typeSwitchOperand(x)
				for _, c := range x.Body.List {
					for _, e := range c.(*ast.CaseClause).List {
						note(e, operand)
					}
				}
			}
			return true
		})
	}
	var out []string
	for name := range seen {
		if _, ok := assertedInterfaces[name]; !ok {
			t.Errorf("internal/mcp type-asserts to %s, which assertedInterfaces does not hold: register it (or, for a non-interface selector, assertedConcreteSelectors) so the wrapper is checked against it", name)
			continue
		}
		out = append(out, name)
	}
	for name := range assertedInterfaces {
		if !seen[name] {
			t.Errorf("assertedInterfaces holds %s, which no product file asserts any more: remove it", name)
		}
	}
	for name := range notTheServedBackend {
		if _, ok := assertedInterfaces[name]; !ok {
			t.Errorf("notTheServedBackend names %s, which assertedInterfaces does not hold", name)
		}
	}
	sort.Strings(out)
	return out, checked
}

// typeSwitchOperand is the value a type switch switches on (x in
// `switch v := x.(type)`).
func typeSwitchOperand(sw *ast.TypeSwitchStmt) ast.Expr {
	var e ast.Expr
	switch a := sw.Assign.(type) {
	case *ast.AssignStmt:
		e = a.Rhs[0]
	case *ast.ExprStmt:
		e = a.X
	}
	if ta, ok := e.(*ast.TypeAssertExpr); ok {
		return ta.X
	}
	return e
}
