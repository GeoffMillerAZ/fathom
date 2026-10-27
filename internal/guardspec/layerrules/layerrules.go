// Package layerrules is the runtime helper `fathom rules gen-test`'s
// generated test imports (VB-14, docs/vision/29-verbs-not-lookups.md §3
// wave C): it re-reads a repo's `.fathom/guard.cue` layer_rules and
// evaluates them with go/packages' TRUE import graph at `go test` time —
// no fathom daemon, no fathom index, so the generated test proves a rule
// with the same authority `go vet` or `go build` would, and fails
// `task validate` and any CI exactly when the committed source violates it.
//
// This is deliberately a SEPARATE producer from
// internal/guardspec.BuildIndexFromSymbols (the call-derived graph that
// backs live obligations serving): go/packages is accurate but too slow
// for the hot edit path, and the index's call-derived graph is fast but
// blind to a type-only/embed-only import. Both build a
// guardspec.LayerRuleIndex keyed by the SAME convention — a package's
// repo-relative directory ("internal/store") — so one rule's from/to/
// scope/allow/guarded globs mean the same thing to both.
package layerrules

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/geoffmilleraz/fathom/internal/depgraph"
	"github.com/geoffmilleraz/fathom/internal/goenv"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
)

// BuildIndex loads root's real Go import graph via go/packages and returns
// a guardspec.LayerRuleIndex over it, keyed by repo-relative package
// directory. CrossService is always nil: a single-module go/packages load
// cannot see another service, so cross_service_direction rules never fire
// from this producer — that kind is a live-obligations-only input (the
// fleet's typed edges), and the generated test reports it as such (see
// Check).
func BuildIndex(root string) (guardspec.LayerRuleIndex, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return guardspec.LayerRuleIndex{}, fmt.Errorf("layerrules: %w", err)
	}
	cfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedImports | packages.NeedModule,
		Dir:   abs,
		Env:   loadEnv(),
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		if mod, ok := goenv.MissingModule(err.Error()); ok {
			return guardspec.LayerRuleIndex{}, fmt.Errorf("layerrules: loading %s: %s is not in the module cache and the load runs go offline (GOPROXY=off, ADR-083): %s: %w", abs, missingName(mod), goenv.Remedy(abs), err)
		}
		return guardspec.LayerRuleIndex{}, fmt.Errorf("layerrules: loading %s: %w", abs, err)
	}
	if len(pkgs) == 0 {
		return guardspec.LayerRuleIndex{}, fmt.Errorf("layerrules: loading %s: no packages (is there a go.mod?)", abs)
	}
	// I38: a dependency the offline load could not fetch leaves its
	// importers' edges out; a rule evaluated over that partial graph would
	// pass for the wrong reason, so the load fails and names it.
	if missing := missingPackages(pkgs); len(missing) > 0 {
		return guardspec.LayerRuleIndex{}, fmt.Errorf("layerrules: loading %s: %s not in the module cache and the load runs go offline (GOPROXY=off, ADR-083): %s", abs, strings.Join(missing, ", "), goenv.Remedy(abs))
	}

	var modulePath string
	for _, p := range pkgs {
		if p.Module != nil {
			modulePath = p.Module.Path
			break
		}
	}
	if modulePath == "" {
		return guardspec.LayerRuleIndex{}, fmt.Errorf("layerrules: %s: no package reports a module (GO111MODULE off?)", abs)
	}

	g := depgraph.NewGraph()
	for _, p := range pkgs {
		fromDir, ok := trimModule(p.PkgPath, modulePath)
		if !ok {
			continue
		}
		for imp := range p.Imports {
			toDir, ok := trimModule(imp, modulePath)
			if !ok || toDir == fromDir {
				continue
			}
			g.AddEdge(fromDir, toDir)
		}
	}
	return guardspec.LayerRuleIndex{PackageEdges: g}, nil
}

// loadEnv is the go command's environment for BuildIndex: this process's,
// GOWORK=off (the subject is its own module; a go.work above it, a
// developer's and never the subject's, must not redirect the load — the
// convention of internal/contextbench/publicloc.LoadGoTruth), then the
// offline overrides (ADR-083, goenv.Apply: GOPROXY=off and GOSUMDB=off
// unless the user's own `[ingest] go_network` says otherwise,
// GOTELEMETRY=off, no -mod=mod) with GOTOOLCHAIN=local last.
func loadEnv() []string {
	return goenv.Apply(append(os.Environ(), "GOWORK=off"))
}

// missingPackages returns, sorted and deduplicated, the packages among pkgs
// and their imports whose load error is the go command's offline refusal.
func missingPackages(pkgs []*packages.Package) []string {
	found := map[string]bool{}
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			if strings.Contains(e.Msg, goenv.LookupRefused) {
				path := p.PkgPath
				if path == "" {
					path = p.ID
				}
				found[path] = true
			}
		}
	})
	out := make([]string, 0, len(found))
	for p := range found {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// missingName is the module a whole-load refusal names, or a stand-in when
// it names none.
func missingName(mod string) string {
	if mod == "" {
		return "a required module"
	}
	return mod
}

// trimModule reports pkgPath's directory relative to modulePath ("." for
// the module root itself), and false when pkgPath is not local to
// modulePath (an external dependency).
func trimModule(pkgPath, modulePath string) (string, bool) {
	if pkgPath == modulePath {
		return ".", true
	}
	prefix := modulePath + "/"
	if !strings.HasPrefix(pkgPath, prefix) {
		return "", false
	}
	return strings.TrimPrefix(pkgPath, prefix), true
}

// Check re-reads ruleFile's layer rules and evaluates every one against
// root's true go/packages import graph, returning every violation from an
// "error"-severity rule (a "warn" rule's violations are silently omitted —
// callers that also want warnings should call Load/BuildIndex/
// guardspec.EvaluateLayerRule directly). A cross_service_direction rule
// always evaluates to zero violations here (see BuildIndex's doc comment)
// — it is reported, not silently dropped: RuleIDsSkipped names any such
// rule so the generated test can say so instead of a false "clean".
func Check(root, ruleFile string) (violations []guardspec.Violation, ruleIDsSkipped []string, err error) {
	rules, err := guardspec.LoadLayerRules(ruleFile)
	if err != nil {
		return nil, nil, fmt.Errorf("layerrules: load %s: %w", ruleFile, err)
	}
	idx, err := BuildIndex(root)
	if err != nil {
		return nil, nil, err
	}
	for _, r := range rules.Rules {
		if r.Kind == guardspec.CrossServiceDirection || r.Kind == guardspec.CallDirection {
			ruleIDsSkipped = append(ruleIDsSkipped, r.ID)
			continue
		}
		if r.Severity != guardspec.SeverityError {
			continue
		}
		violations = append(violations, guardspec.EvaluateLayerRule(r, idx)...)
	}
	return violations, ruleIDsSkipped, nil
}
