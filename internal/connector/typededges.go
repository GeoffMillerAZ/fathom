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
	"scss": "file",
	// astro (LB-Q-astro, wave lf7): its frontmatter's and scripts' imports
	// (F2) resolve by the relative-path strategy; an import of a component
	// is an edge between the files' module symbols.
	"astro": "file",

	// LB-Q-vue, LB-Q-svelte (wave lf7): a component's <script> imports (F2).
	"vue":    "file",
	"svelte": "file",

	// meson (LB-Q-meson): subdir('x') imports x/meson.build through the
	// relative-path strategy, which lang-oracle's resolver runs (a name a file
	// uses that an imported meson.build defines resolves at file granularity),
	// but ingest's file-resolve pass is keyed by file extension and meson.build,
	// meson.options and meson_options.txt have none (O379), so no meson edge is
	// stored and "syntax" is the honest baseline claim (ADR-027).
	"meson": "syntax",

	// syntax: call names by syntax only — every other registered
	// language, CUE included (see the doc comment above).
	"cue":        "syntax",
	"php":        "syntax",
	"sh":         "syntax",
	"powershell": "syntax",
	"makefile":   "syntax",
	"gradle":     "syntax",
	"lua":        "syntax",
	"luau":       "syntax",
	"elixir":     "syntax",
	"jupyter":    "syntax",
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

	// vcpkg.json (LB-Q-vcpkg): a dependency on another port of the repository
	// (and an overlay-ports entry naming a port directory, design §4) becomes
	// a file-tier doc-xref edge at ingest (fusion.ResolveDocXrefs) but is not
	// a file-resolver or SCIP producer, so "syntax" is the honest baseline
	// claim, same as Cargo.toml above.
	"vcpkg": "syntax",

	// Conan (LB-Q-conan): a recipe's requires are references (calls) only; no
	// cross-file edge is declared (the tags engine's recipes are not entities
	// the doc-xref pass can resolve, O370), so "syntax" is the honest baseline.
	"conan": "syntax",

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

	// nginx, HAProxy and Caddyfile (LB-Q-nginx, LB-Q-haproxy, LB-Q-caddyfile):
	// native proxy dialects whose includes and imports become file-tier
	// doc-xref edges and whose route targets are route_ref references, but
	// are not a file-resolver or SCIP producer, so "syntax" is the honest
	// baseline claim, same as go.mod above.
	"nginx": "syntax",

	// Terragrunt and the HashiCorp HCL tool configs (LB-Q-terragrunt,
	// LB-Q-hcl-tools): native hclsyntax dialects whose includes, dependencies,
	// local module sources and image references become file-tier doc-xref
	// edges at ingest (fusion.ResolveDocXrefs), not a file-resolver or SCIP
	// producer, so "syntax" is the honest baseline claim, same as nginx above.
	"terragrunt": "syntax",
	"hcl-tools":  "syntax",

	"haproxy":   "syntax",
	"caddyfile": "syntax",

	// Rego (LB-Q-rego, LB-F131, LB-F132): a native dialect whose own reader
	// parses its imports; `import data.x.y` resolves by the rego-package
	// strategy and a call or reference through it is a file-resolved edge, so
	// the per-file resolver is its producer, as for HTML and CSS above.
	"rego": "file",

	// Jenkins pipelines (LB-Q-jenkinsfile): a step's name by syntax; a shell
	// step and a vars/ step call become file-tier doc-xref edges at ingest
	// (fusion.jenkinsXrefs, LB-F374, LB-F375), not a file-resolver or SCIP
	// producer, so "syntax" is the honest baseline claim, same as Gradle.
	"jenkinsfile": "syntax",

	// MSBuild (LB-Q-msbuild): a ProjectReference to another project of the
	// repository becomes a file-tier doc-xref edge at ingest
	// (fusion.ResolveDocXrefs) but is not a file-resolver or SCIP producer,
	// so "syntax" is the honest baseline claim, same as pom.xml and
	// Cargo.toml above.
	"msbuild": "syntax",

	// Envoy, Kong and Traefik configuration (LB-Q-envoy, LB-Q-kong,
	// LB-Q-traefik): a route's cluster or service, a listener's route
	// configuration and an xDS or file-provider path become file-tier doc-xref
	// edges at ingest (fusion.ResolveDocXrefs); a cluster's, service's or
	// upstream's host is a route-host target for the route_ref layer
	// (fusion.ResolveRouteRefs). None is a file-resolver or SCIP producer, so
	// "syntax" is the honest baseline claim, same as Kustomize and Taskfile.
	"envoy":   "syntax",
	"kong":    "syntax",
	"traefik": "syntax",

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

	// Azure Bicep (LB-Q-bicep): a tags-engine dialect with an import strategy
	// (relative-path), so a module's path and a compile-time import resolve to
	// the file through the file resolver: "file", as CSS and HTML above.
	"bicep": "file",

	// Serverless Framework, Pulumi YAML and AWS CDK (LB-Q-serverless,
	// LB-Q-pulumi-yaml, LB-Q-aws-cdk): doc-engine tool formats whose
	// references stay within the file; a `${file()}` include is a file-tier
	// doc-xref edge at most, so "syntax" is the honest baseline claim.
	"serverless":  "syntax",
	"pulumi-yaml": "syntax",
	"aws-cdk":     "syntax",
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
