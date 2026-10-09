package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// The C# compiler's semantic model as L2's call-site oracle beside a
// scip-dotnet index (LB-F287, calibration/oracles/roslyn calls mode, tier A),
// the way clang-calls stands beside scip-clang (LB-F116), and the disclosure
// of each count C#'s rules write (LB-F286, LB-F287).

const (
	noteCSharpCtorNotCalls          = "scip-dotnet constructor (`.ctor`) references are written where an attribute is applied (`[Obsolete(\"x\")]`, `[Benchmark]`), never at `new X(..)`: an attribute's constructor runs only when reflection reads it, so no call is written there, and they are left out of the call truth (LB-F286)"
	noteCSharpConstructions         = "type references scip-dotnet writes at `new X(..)`, `new X<T>(..)` or `new X { .. }` (it references the type there, not its constructor) are counted as calls to X, the name the dialect records a construction under (LB-F286)"
	noteCSharpCallsAdded            = "call pairs the C# compiler binds at a written call are in the call truth with no callable reference of the index's behind them: a generic method called with explicit type arguments (`Policy.Handle<T>()`, where scip-dotnet 0.2.14 writes no occurrence, O271), a delegate-typed parameter, local, field or property invoked (`callback(x)`), a local function (calibration/oracles/roslyn calls mode, tier A, LB-F287)"
	noteCSharpConstructs            = "constructions the C# compiler binds (`new X(..)`) that no index reference gave are in the call truth (LB-F287)"
	noteCSharpCallsUnresolved       = "of fathom's call pairs are written only where the C# compiler binds nothing in any target framework it compiles (a late-bound `dynamic` call, an error) and are left out of both L2 counts (LB-F287)"
	noteCSharpCallsInactive         = "of fathom's call pairs are written only inside `#if` branches no target framework the C# compiler builds keeps (`#if !HAVE_LINQ` in a library compiled for net20 through net8.0) and are left out of both L2 counts, as clang's preprocessor oracle leaves out C's (LB-F80, LB-F287)"
	noteCSharpFileEdgesMultiDefined = "index file edges are drawn only by references to a symbol more than one document defines (a partial class such as Polly's Policy, declared in 30 files; scip-dotnet also gives a generic type and its non-generic namesake one symbol) and are left out of the file-edge truth: which file such a reference means has no answer, as the import truth already holds, and the harness's first-definer pick was arbitrary (LB-F287)"
	noteCSharpFileEdgesAdded        = "cross-file edges the C# compiler binds, over every target framework, are in the file-edge truth with no index reference behind them (a generic type used as a type argument and a generic method called with type arguments, where scip-dotnet 0.2.14 writes no occurrence, O271; code its one framework does not compile); the compiler draws none to a symbol two files declare (LB-F287)"
	noteCSharpCallShapes            = "call sites the C# compiler binds, by how each is written (static `Type.M(..)`, member `x.M(..)`, bare `M(..)`, extension, generic with type arguments, local_function, delegate `h(..)`, event `E?.Invoke(..)`, new, unresolved)"
	noteCSharpQueryClauses          = "LINQ query clauses the C# compiler binds to a method (`where` to Where, `select` to Select) write no invocation, so no extractor of call sites names them and they are no call pairs of either side (LB-F287)"
)

// csharpCompilerOf reads the C# compiler's call sites, kept lines and query
// clause count from every tool oracle that has them; nil when none does.
func csharpCompilerOf(paths []string) (*langbench.CSharpCompiler, error) {
	cc := &langbench.CSharpCompiler{Sites: map[string][]langbench.ClangSite{}, Active: map[string][][2]int{}}
	found := false
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var probe struct {
			Tool         string              `json:"tool"`
			CallSites    []clangCallSite     `json:"call_sites"`
			ActiveLines  map[string][][2]int `json:"active_lines"`
			QueryClauses int                 `json:"query_clauses"`
			FileEdges    [][2]string         `json:"file_edges"`
			Multi        int                 `json:"file_edges_multi_declared"`
			Shapes       map[string]int      `json:"call_shapes"`
			InLambda     int                 `json:"calls_in_lambda"`
		}
		if err := json.Unmarshal(b, &probe); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if len(probe.CallSites) == 0 && len(probe.ActiveLines) == 0 {
			continue
		}
		found = true
		for _, s := range probe.CallSites {
			cc.Sites[s.Path] = append(cc.Sites[s.Path], langbench.ClangSite{Line: s.Line, Col: s.Col, Name: s.Name, Class: s.Class})
		}
		for f, r := range probe.ActiveLines {
			cc.Active[f] = r
		}
		cc.QueryClauses += probe.QueryClauses
		cc.FileEdges = append(cc.FileEdges, probe.FileEdges...)
		cc.MultiDeclared += probe.Multi
		for s, n := range probe.Shapes {
			if cc.Shapes == nil {
				cc.Shapes = map[string]int{}
			}
			cc.Shapes[s] += n
		}
		cc.InLambda += probe.InLambda
	}
	if !found {
		return nil, nil
	}
	return cc, nil
}

// csharpSiteNotes discloses each count C#'s call-site rules wrote for a subject.
func csharpSiteNotes(name string, counts map[string]int, cc *langbench.CSharpCompiler) []string {
	var out []string
	for _, c := range []struct{ key, note string }{
		{langbench.CntScipCSharpCtorNotCalls, noteCSharpCtorNotCalls},
		{langbench.CntScipCSharpConstructions, noteCSharpConstructions},
		{langbench.CntCSharpCallsAdded, noteCSharpCallsAdded},
		{langbench.CntCSharpConstructs, noteCSharpConstructs},
		{langbench.CntCSharpCallsUnresolved, noteCSharpCallsUnresolved},
		{langbench.CntCallsInactive, noteCSharpCallsInactive},
		{langbench.CntScipCSharpFileEdgesMultiDefined, noteCSharpFileEdgesMultiDefined},
		{langbench.CntCSharpFileEdgesAdded, noteCSharpFileEdgesAdded},
	} {
		if n := counts[c.key]; n > 0 {
			out = append(out, fmt.Sprintf("%s: %d %s", name, n, c.note))
		}
	}
	if cc != nil && cc.QueryClauses > 0 {
		out = append(out, fmt.Sprintf("%s: %d %s", name, cc.QueryClauses, noteCSharpQueryClauses))
	}
	if cc != nil && len(cc.Shapes) > 0 {
		shapes := make([]string, 0, len(cc.Shapes))
		total := 0
		for s, n := range cc.Shapes {
			shapes = append(shapes, fmt.Sprintf("%s %d", s, n))
			total += n
		}
		sort.Strings(shapes)
		out = append(out, fmt.Sprintf("%s: %d %s: %s; %d inside a lambda or anonymous method, each the call of the definition around it (D5) (LB-F287)", name, total, noteCSharpCallShapes, strings.Join(shapes, ", "), cc.InLambda))
	}
	sort.Strings(out)
	return out
}
