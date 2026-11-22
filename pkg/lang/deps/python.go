package deps

import (
	"context"
	"errors"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

func init() { admissions["python"] = measured } // DEP-M2

var pythonImports = sync.OnceValues(func() (*extractor.TSExtractor, error) {
	return extractor.NewTSExtractor("python")
})

// The shared import captures handle static named and aliased imports. This
// query accounts for forms outside that binding-oriented query and detects
// constructs that can change what imports mean at runtime.
const pythonDependencyQuery = `
[(ERROR) (MISSING)] @error
(import_from_statement
  module_name: [(dotted_name) (relative_import)] @star.module
  (wildcard_import)) @star.statement
(future_import_statement) @future
(identifier) @identifier
(attribute object: (identifier) @receiver attribute: (identifier) @member)
(comment) @comment
`

const pythonAttributionQuery = `
(import_statement name: [
  (dotted_name) @site.module
  (aliased_import name: (dotted_name) @site.module)
]) @site.whole
(import_from_statement module_name: (_) @site.module) @site.from
`

func readPythonDependencies(ctx context.Context, repo repositoryFS, src []byte, resolver Resolver, out *result) {
	if !utf8.Valid(src) {
		out.addUnknown(Unknown{Code: UnknownUnsupportedConstruct, DecidableBy: []DecidableBy{DecideToolchain}})
		return
	}
	importer, err := pythonImports()
	if err != nil {
		pythonParseGap(out, err)
		return
	}
	specs, err := importer.ExtractImports(out.Path, src)
	if err != nil {
		pythonParseGap(out, err)
		return
	}
	if ctx.Err() != nil {
		return
	}
	matches, err := pythonQueryMatches(ctx, src, "dependency-python", pythonDependencyQuery, false)
	if err != nil {
		pythonParseGap(out, err)
		return
	}

	paths := pythonPaths{repo: repo, directories: make(map[string]pythonDirectory), declarations: make(map[string]map[string]bool)}
	resolve := func(raw, member string, line int, declarationAllowed bool) {
		raw = pythonCleanModule(raw)
		if len(raw) > maxIdentityBytes || raw == "" {
			out.addUnknown(Unknown{Code: UnknownUnsupportedConstruct, Line: line, DecidableBy: []DecidableBy{DecideToolchain}})
			return
		}
		if resolver != nil {
			out.resolveExtension(ctx, resolver, ResolveRequest{
				Repo: repo, From: out.Path, Raw: raw, Kind: KindImport, Line: line,
			})
			return
		}
		paths.resolve(ctx, out, raw, member, line, declarationAllowed)
	}
	dynamic := func(line int) {
		out.addUnknown(Unknown{Code: UnknownDynamicImport, Line: line, DecidableBy: []DecidableBy{DecideObservation, DecideDeclaration}})
	}

	sysNames := map[string]bool{"sys": true}
	for _, spec := range specs {
		if ctx.Err() != nil {
			return
		}
		resolve(spec.Module, spec.ExportedName, spec.Line,
			spec.Kind == extractor.ImportNamed && pythonIdentifier(spec.LocalName) && !pythonReservedDeclaration(spec.LocalName))
		head, _, _ := strings.Cut(spec.Module, ".")
		switch head {
		case "builtins", "imp", "importlib", "pkg_resources", "pkgutil", "runpy", "zipimport":
			dynamic(spec.Line)
		case "sys":
			if spec.Kind == extractor.ImportWhole {
				sysNames[spec.LocalName] = true
			}
			switch spec.ExportedName {
			case "meta_path", "modules", "path", "path_hooks", "path_importer_cache":
				dynamic(spec.Line)
			}
		}
	}

	invalidSyntax := false
	for _, match := range matches {
		node := match.Captures[0].Node
		line := node.Line()
		switch match.Captures[0].Name {
		case "error":
			invalidSyntax = true
			out.addUnknown(Unknown{Code: UnknownParseError, Line: line, DecidableBy: []DecidableBy{DecideToolchain}})
		case "star.statement":
			resolve(match.Captures[1].Node.Text(src), "*", line, false)
			dynamic(line)
		case "future":
			resolve("__future__", "", line, false)
		case "identifier":
			switch node.Text(src) {
			case "__builtins__", "__getattr__", "__import__", "__loader__", "__package__", "__path__", "__spec__",
				"declare_namespace", "eval", "exec", "extend_path", "getattr", "globals", "import_module", "locals", "setattr":
				dynamic(line)
			}
		case "receiver":
			member := match.Captures[1].Node
			switch member.Text(src) {
			case "meta_path", "path_hooks", "path_importer_cache", "spec_from_file_location", "exec_module", "load_module":
				dynamic(member.Line())
			case "modules", "path":
				if sysNames[node.Text(src)] {
					dynamic(member.Line())
				}
			}
		case "comment":
			if line <= 2 && strings.Contains(node.Text(src), "coding") {
				invalidSyntax = true
				// The Go parser reads UTF-8. A PEP 263 cookie may
				// give these same bytes a different meaning.
				out.addUnknown(Unknown{Code: UnknownUnsupportedConstruct, Line: line, DecidableBy: []DecidableBy{DecideToolchain}})
			}
		}
	}
	paths.finishFromLists(ctx, out, src, invalidSyntax)
}

func pythonQueryMatches(ctx context.Context, src []byte, key, pattern string, types bool) ([]tsengine.Match, error) {
	lease, err := tsengine.AcquireParser(ctx, "dependency-python", "python", "tree_sitter_python")
	if err != nil {
		return nil, err
	}
	defer lease.Release()
	parser := lease.Parser()
	query, ok := parser.Cached(key).(*tsengine.Query)
	if !ok {
		query, err = parser.CompileQuery(pattern)
		if err != nil {
			return nil, err
		}
		if types {
			query.CaptureNodeTypes()
		}
		parser.SetCached(key, query)
	}
	tree, err := parser.Parse(src)
	if err != nil {
		lease.Discard()
		return nil, err
	}
	defer tree.Close()
	matches, err := query.Run(tree)
	if err != nil {
		// Release the tree before discarding its parser.
		return nil, err
	}
	return matches, nil
}

func pythonParseGap(out *result, err error) {
	code := UnknownParseError
	var budget *tsengine.BudgetError
	if errors.As(err, &budget) {
		code = UnknownReadLimit
	}
	out.addUnknown(Unknown{Code: code, DecidableBy: []DecidableBy{DecideToolchain}})
}
