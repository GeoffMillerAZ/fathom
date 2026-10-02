package connector

// typededges.go — the per-language typed-edge PRODUCER table (ADR-041
// decision 5, SU-13, docs/plans/substrate.md): the code-side fact
// `fathom capabilities`'s typed_edge_producers field reports, and the
// table TestLanguagesTrackerMatchesRegistry (internal/connector/
// languages_test.go) cross-checks against docs/languages.md's own bolded
// level word so the doc and the code can never drift silently.
//
// This reports what MECHANISM currently exists in code for a language —
// never an aspirational target. CUE is the case that matters: docs/
// languages.md names "native" as CUE's TARGET level (tracked as SU-10,
// still open), but internal/fusion.EnrichCUEEdges does not exist yet, so
// the honest fact here is "syntax" — ADR-027's rule that an instrument
// may not degrade toward healthy applies to what fathom claims about
// itself as much as to what it claims about a service it indexes.
var typedEdgeProducers = map[string]string{
	// native: fathom resolves types itself.
	"go": "native",

	// scip: fathom's generic `fathom ingest --scip` import mechanism
	// (internal/scip.ImportIndex, SU-9) exists and works for any SCIP
	// index regardless of which external indexer produced it — the tool
	// named after the colon is the delegate docs/languages.md tracks for
	// this language, per ADR-041 decision 5's order, not a claim that
	// mechanism has actually been run for any particular service.
	"ts":     "scip:scip-typescript",
	"js":     "scip:scip-typescript",
	"py":     "scip:scip-python",
	"java":   "scip:scip-java",
	"kotlin": "scip:scip-java",
	"scala":  "scip:scip-java",
	"rust":   "scip:rust-analyzer",
	"c":      "scip:scip-clang",
	"cpp":    "scip:scip-clang",
	"ruby":   "scip:scip-ruby",
	"dart":   "scip:scip-dart",
	"cs":     "scip:scip-dotnet",

	// file: fusion.ResolveFileEdges' relative-path strategy resolves this
	// dialect's src/href/@import specs to in-repo files (tier pre-scan,
	// LB-F11) — no SCIP producer and no native pass. (Swift's swift-module
	// strategy would too, but LB-Q-swift is refused and .swift is not
	// classified: design §14.2.)
	"html": "file",
	"css":  "file",

	// meson (LB-Q-meson): subdir('x') imports x/meson.build through the
	// relative-path strategy, so a subproject or dependency a file names that a
	// subdirectory's meson.build declares resolves to it at file granularity
	// (tier pre-scan, LB-F11).
	"meson": "file",

	// syntax: call names by syntax only — every other registered
	// language, CUE included (see the doc comment above).
	"cue":        "syntax",
	"php":        "syntax",
	"sh":         "syntax",
	"powershell": "syntax",
	"makefile":   "syntax",
	"lua":        "syntax",
	"emacs-lisp": "syntax",
	"commonlisp": "syntax",
	"ada":        "syntax",
	"vhdl":       "syntax",
	"sql":        "syntax",
	"terraform":  "syntax",
	"yaml":       "syntax",
	"md":         "syntax",
	"dockerfile": "syntax",
	"cmake":      "syntax",
	"proto":      "syntax",
	"graphql":    "syntax",
	"toml":       "syntax",
	"json":       "syntax",
	"templ":      "syntax",

	// package.json (LB-Q-package-json): its cross_file selectors
	// (workspace-member globs, dependency edges, design §4) are not wired
	// into fusion.ResolveFileEdges or any SCIP producer, so "syntax" is
	// the honest baseline claim, same as JSON/TOML/CUE above (ADR-027: a
	// mechanism is claimed only once it exists in code).
	"package-json": "syntax",

	// pyproject.toml (LB-Q-python-packaging): its cross_file selectors
	// (dependency, optional-dependency, dependency-group and build-system
	// edges, design §4) are not wired into fusion.ResolveFileEdges or any
	// SCIP producer, so "syntax" is the honest baseline claim, same as
	// package.json above (ADR-027: a mechanism is claimed only once it
	// exists in code).
	"python-packaging": "syntax",

	// pom.xml (LB-Q-maven): its cross_file selectors (modules, parent and
	// dependency edges to another POM of the repository, design §4) become
	// file-tier doc-xref edges at ingest (fusion.ResolveDocXrefs) but are
	// not a file-resolver or SCIP producer, so "syntax" is the honest
	// baseline claim, same as package.json and pyproject.toml above.
	"maven": "syntax",

	// Cargo.toml (LB-Q-cargo): its cross_file selectors (workspace members
	// and a dependency on another package of the repository, design §4)
	// become file-tier doc-xref edges at ingest (fusion.ResolveDocXrefs) but
	// are not a file-resolver or SCIP producer, so "syntax" is the honest
	// baseline claim, same as package.json, pyproject.toml and pom.xml above.
	"cargo": "syntax",

	// compile_commands.json (LB-Q-compile-commands): a translation unit's
	// include directories, macros and output are its own references; no
	// cross-file selector is declared (L2), so "syntax" is the honest baseline
	// claim, same as the manifests above.
	"compile-commands": "syntax",

	// go.mod and go.work (LB-Q-go-mod): a require to another module of the
	// repository, a replace to a local directory and a go.work use become
	// file-tier doc-xref edges at ingest (fusion.ResolveDocXrefs) but are not a
	// file-resolver or SCIP producer, so "syntax" is the honest baseline claim,
	// same as Cargo.toml above. (The Go language itself stays "native".)
	"go-mod": "syntax",

	// MSBuild (LB-Q-msbuild): a ProjectReference to another project of the
	// repository becomes a file-tier doc-xref edge at ingest
	// (fusion.ResolveDocXrefs) but is not a file-resolver or SCIP producer,
	// so "syntax" is the honest baseline claim, same as pom.xml and
	// Cargo.toml above.
	"msbuild": "syntax",

	// Kustomize (LB-Q-kustomize): a resource, base, component or patch that
	// names another doc-engine file of the repository (today another
	// kustomization, by its directory: LB-Q-kustomize-rest) becomes a
	// file-tier doc-xref edge at ingest (fusion.ResolveDocXrefs) but is not a
	// file-resolver or SCIP producer, so "syntax" is the honest baseline
	// claim, same as pom.xml and Cargo.toml above. (A kustomization file was
	// language "yaml" before: it shared that tag's producer.)
	"kustomize": "syntax",

	// Taskfile (LB-Q-taskfile): an `includes:` entry that names another
	// Taskfile of the repository becomes a file-tier doc-xref edge at ingest
	// (fusion.ResolveDocXrefs) but is not a file-resolver or SCIP producer, so
	// "syntax" is the honest baseline claim, same as pom.xml and MSBuild above.
	"taskfile": "syntax",

	// Git hook managers (LB-Q-git-hooks-config): lefthook's hooks, commands,
	// scripts and jobs and pre-commit's hooks are definitions only (L1); no
	// reference or cross-file edge is drawn, so "syntax" is the honest baseline
	// claim, same as Taskfile above.
	"git-hooks-config": "syntax",
}

// TypedEdgeProducer returns lang's typed-edge producer string — "native",
// "scip:<tool>", "file", or "syntax" — or "" when lang is not a
// registered language tag (RegisteredExtensions/LanguageForPath).
func TypedEdgeProducer(lang string) string { return typedEdgeProducers[lang] }

// TypedEdgeProducers returns the full lang -> producer map, one entry per
// distinct Lang tag RegisteredExtensions() classifies — `fathom
// capabilities`'s typed_edge_producers field (SU-13).
func TypedEdgeProducers() map[string]string {
	out := map[string]string{}
	for _, ext := range RegisteredExtensions() {
		lang := LanguageForPath(ext)
		if lang == "" {
			continue
		}
		if p, ok := typedEdgeProducers[lang]; ok {
			out[lang] = p
		}
	}
	return out
}
