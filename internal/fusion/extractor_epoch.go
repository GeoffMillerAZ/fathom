package fusion

// extractor_epoch.go — ExtractorEpoch, the fingerprint salt every extraction change bumps,
// with its per-epoch history, and FingerprintHash, the pass-3 hash gate. Split out of
// pipeline.go unchanged, so the file every language wave edits holds only this.

import (
	"crypto/sha256"
	"encoding/hex"
)

// ExtractorEpoch salts every file fingerprint (the pass-3 hash-gate). Bump it
// in any commit that changes EXTRACTION SEMANTICS (new call-edge forms,
// symbol naming, kind mappings …): the salt change invalidates all stored
// fingerprints, so the next ingest of every service re-extracts every file.
// Without it, hash-gated files keep observations produced by the OLD
// extractor forever — the stale-observation class dogfood O9 caught (the H1
// callgraph/extractor fixes were invisible on the fleet store until forced).
// Epoch 2 = the H1 extractor generation (aliased-selector canonicalization,
// constructor-scoped receiver inference) + this salt's introduction.
// Epoch 3 = the LT-4 CUE extractor v2 generation (docs/plans/lang-truth.md,
// ADR-021 §1): nested-field recursion emitting dotted names
// (#Layer.budgets.max_usd_per_run), package-clause attribution (was
// filename-derived), and import capture. Every already-indexed .cue file's
// stored observations were produced by the OLD top-level-only extractor, so
// the salt bump forces re-extraction on the next ingest rather than leaving
// them silently stale (the exact class dogfood O9 caught for epoch 2).
// Epoch 4 = the ADR-021 §2 tool-identity classifier generation
// (docs/plans/toolident.md): Helm templates/*.yaml now parse via
// placeholder substitution instead of emitting nothing; kustomization.yaml
// emits resource/patch/image symbols instead of nothing (no metadata.name);
// arbitrary compose files and OpenAPI/CRD documents are content-classified
// instead of silently zero-output through the k8s extractor;
// HelmValuesExtractor recurses into nested keys (dotted names) instead of
// top-level-only. Every already-indexed yaml/cue file's stored observations
// were produced by the OLD path/extension-only dispatch, so the salt bump
// forces re-extraction rather than leaving them silently stale.
// Epoch 5 = the ADR-021 §4 tree-sitter engine migration
// (docs/plans/extraction-treesitter.md): Python, JavaScript/TypeScript/JSX/
// TSX, and Ruby now extract via internal/extractor.NewTSExtractor — tree-
// sitter's own C runtime + grammar, compiled to a self-contained wasm32-wasi
// module and executed by the pure-Go wazero runtime (internal/extractor/
// tsengine) — instead of the CGO tree-sitter bindings (ADR-013). Symbol
// naming/qualification is generic containment-based post-processing over
// query captures rather than per-language hand-rolled tree walks; every
// already-indexed .py/.js/.jsx/.ts/.tsx/.rb file's stored observations were
// produced by the OLD CGO extractor, so the salt bump forces re-extraction.
// Epoch 6 (docs/plans/cpp-toolchain.md): C++ names normalize "::" to "."
// (CT-1), .h headers route through the C/C++ sniffing router (CT-3), and
// CMake files extract (CT-4) — stored observations for every .h/.cc/.cpp/
// .cxx/.hpp/.hh file predate all three, so the salt bump forces
// re-extraction.
// Epoch 7 (wave P2, O69/O79): Kotlin (.kt/.kts) now extracts via
// internal/extractor.NewTSExtractor("kotlin") — tree-sitter via wazero/WASM,
// ADR-021 §4 new-language breadth, no CGO incumbent — where every such file
// previously fell through to "no extractor registered" and was fingerprinted
// (never re-sniffed) by that dead-end path; the salt bump forces those files
// through the new extractor on the next ingest instead of leaving them
// silently stuck on a stale "unsupported" fingerprint. The same epoch bump
// also covers JavaExtractor (.java, still CGO/ADR-013) additionally emitting
// KindJavaField and KindJavaImport, which the epoch-6 extractor never
// captured — every already-indexed .java file's stored observations predate
// both new kinds.
// Epoch 8 (LB-F3, docs/design/language-breadth.md §3.3): the tags engine
// (JavaScript, TypeScript, Python, Ruby, Kotlin) now fills EndLine for every
// definition and Signature for every callable; stored observations for those
// files carry neither, so the salt bump re-extracts them.
// Epoch 9 (LB-F10, design §5): the generated/minified gate — a minified or
// generator-marked file of a manifest dialect is no longer extracted; its
// stored observations predate the gate, so the salt bump purges them.
// Epoch 10 (LB-Q-html, docs/design/language-breadth.md §13): HTML (.html/
// .htm) now extracts via internal/extractor.NewTSExtractor("html") — tree-
// sitter via wazero/WASM, a fleet gap with no incumbent — where every such
// file previously fell through to "no extractor registered" and was
// fingerprinted (never re-sniffed) by that dead-end path, the same class
// epoch 7 fixed for Kotlin; the salt bump forces those files through the
// new extractor on the next ingest instead of leaving them silently stuck
// on a stale "unsupported" fingerprint.
// Epoch 11 (LB-Q-css, docs/design/language-breadth.md §13): CSS (.css) now
// extracts via internal/extractor.NewTSExtractor("css") — tree-sitter via
// wazero/WASM, a fleet gap with no incumbent — where every .css file
// previously fell through to "no extractor registered" and was
// fingerprinted (never re-sniffed) by that dead-end path, the same class
// epochs 7 and 10 fixed for Kotlin and HTML; the salt bump forces those
// files through the new extractor on the next ingest instead of leaving
// them silently stuck on a stale "unsupported" fingerprint.
// Epoch 12 (LB-Q-package-json, docs/design/language-breadth.md §13/§4): a
// basename match on "package.json" now routes to the doc-engine
// package-json dialect (package/script/bin/export entities, dependency and
// workspace cross-file references) instead of the generic JSONExtractor's
// top-level-key dump (KindJSONKey, Line always 0) — every already-indexed
// package.json's stored observations were produced by that generic path,
// so the salt bump forces re-extraction rather than leaving them silently
// stale.
// Epoch 13 (LB-Q-python-packaging, docs/design/language-breadth.md §13/§4):
// a basename match on "pyproject.toml" now routes to the doc-engine
// python-packaging dialect (package/script/entrypoint/tool/dependency-group
// entities, an include-group intra-file reference, and dependency/
// build-system cross-file references) instead of the generic
// TOMLExtractor's bare table/key dump (KindTOMLTable/KindTOMLKey) — every
// already-indexed pyproject.toml's stored observations were produced by
// that generic path, so the salt bump forces re-extraction rather than
// leaving them silently stale.
// Epoch 14 (LB-Q-terraform, docs/design/language-breadth.md §13): HCLExtractor
// now emits intra-file Calls (resource/data/module/var/local reference edges,
// including depends_on) on resource/data/module/output/locals symbols, records
// a module's source (local path or registry ref) in its Signature, and .tfvars
// files now extract per-key TerraformVarValue symbols instead of falling
// through to "no extractor registered" — every already-indexed .tf file's
// stored observations were produced by the OLD extractor with no Calls and a
// bare Signature, so the salt bump forces re-extraction rather than leaving
// them silently stale.
// Epoch 15: the doc engine's TOML locator places a table reached only
// through deeper headers ([tool.black.sub]) or dotted keys at the first line
// that writes under it, not at its parent's line — every already-indexed
// pyproject.toml's [tool.*] sections were stored at line 1.
// Epoch 16 (LB-Q-dockerfile, docs/design/language-breadth.md §13): Dockerfile
// (bare "Dockerfile", "*.dockerfile") now extracts via
// internal/extractor.NewTSExtractor("dockerfile") — tree-sitter via
// wazero/WASM (camdencheek/tree-sitter-dockerfile) — retiring
// DockerfileExtractor's line-scanner. DockerStage nodes now carry a real
// Tier (TierVerified, not TierPreScan), an EndLine and a Signature, ARG/ENV
// keys are extracted for the first time (DockerfileArg/DockerfileEnv), and a
// stage's Calls now includes every FROM base reference unconditionally
// (external images too, not only a match against an earlier stage's alias —
// the tags engine's naive same-file call collection convention every other
// tags dialect already follows, ADR-021 §3) — every already-indexed
// Dockerfile/*.dockerfile's stored observations predate all of this, so the
// salt bump forces re-extraction rather than leaving them silently stale.
// Epoch 17 (owner decision D2's re-measurement of TypeScript against
// scip-typescript, 2026-09-28): the typescript dialect now emits abstract
// classes (their members are qualified by them), interface and abstract
// method signatures, and function overload / `declare function` signatures —
// every already-indexed .ts/.tsx file missed those definitions.
// Epoch 18 (EW-15): the Go extractor marks the symbols of a file no default
// build compiles (a //go:build ignore generator) core.ToolKindGoBuildExcluded,
// and a bare reference assigned to the blank identifier (`_ = f`) is no
// longer recorded as a call — every already-indexed .go file predates both.
// Epoch 19 (LB-Q-powershell, docs/design/language-breadth.md §13): PowerShell
// (.ps1 .psm1 .psd1) now extracts via internal/extractor.NewTSExtractor
// ("powershell") — tree-sitter via wazero/WASM, a fleet gap with no incumbent
// — where every such file previously fell through to "no extractor
// registered" and was fingerprinted (never re-sniffed) by that dead-end path,
// the same class epochs 7, 10 and 11 fixed for Kotlin, HTML and CSS; the salt
// bump forces those files through the new extractor on the next ingest.
// Epoch 20 (LB-Q-makefile, docs/design/language-breadth.md §13): Makefile,
// makefile, GNUmakefile, *.mk and *.make now extract via
// internal/extractor.NewTSExtractor("makefile") — tree-sitter via wazero/WASM,
// a fleet gap with no incumbent — where every such file previously fell
// through to "no extractor registered" and was fingerprinted by that
// dead-end path (the epoch 19 class); the salt bump re-reads them.
// Epoch 21 (LB-Q-kotlin): the kotlin dialect now takes its Package from the
// package clause (it was the directory), emits member and primary-constructor
// properties, enum entries (KotlinEnumEntry), type aliases (KotlinTypeAlias) and
// secondary constructors (KotlinConstructor), names a call by its callee's last
// identifier, and parses on a newer grammar commit (`fun interface`, detached
// getters and setters); every already-indexed .kt/.kts file lacks those.
// Epoch 22 (LB-Q-lua): .lua files now extract via
// internal/extractor.NewTSExtractor("lua") — tree-sitter via wazero/WASM —
// retiring the CGO walker: nested declarations, functions assigned to a name, a
// table field or a string key are symbols, every function carries a Signature,
// and a call is named by its callee's last identifier (foo, not M.foo); every
// already-indexed .lua file lacks those, so the salt bump re-extracts it.
// Epoch 23 (LB-Q-gitlab-ci, docs/design/language-breadth.md §13/§4): a
// .gitlab-ci.yml, a file under .gitlab/ci/ and any *.gitlab-ci.yml now extract
// through the doc-engine gitlab-ci dialect (jobs, hidden templates, their
// needs/dependencies/extends references and the cross-file ones that resolve)
// where each was fingerprinted as unclassified YAML (or, with a global
// `services:` list, read as Compose); the salt bump re-reads them.
// Epoch 24 (LB-Q-ruby): the ruby dialect emits constants (RubyConstant, a container
// so `Outer::CONST` reads as Ruby writes it), attr_accessor/attr_reader/attr_writer
// names (RubyProperty, `Owner.name`), aliases (RubyAlias), define_method with a
// literal name and Name = Class.new / Module.new, names a constant reference as a
// call target (`Registry.instance.register` calls Registry, instance and register),
// leaves out `def obj.x` on a local, and .rake, .ru and Rakefile now extract as Ruby;
// every already-indexed .rb file lacks those, so the salt bump re-extracts it.
// Epoch 25 (LB-Q-maven, docs/design/language-breadth.md §13/§4): a pom.xml now
// extracts through the doc-engine maven dialect (the project's artifact, its
// build plugins, parent and dependency references, and the modules, parent and
// dependency edges that resolve to another POM of the repository) where each
// was fingerprinted as an unclassified .xml file; the salt bump re-reads them.
// Epoch 26 (LB-Q-dart): .dart files now extract via
// internal/extractor.NewTSExtractor("dart") — tree-sitter via wazero/WASM, a
// fleet gap with no incumbent: classes, mixins, extensions, enums and their
// constants, typedefs, functions, methods, getters and setters, constructors,
// fields and top-level variables, each callable with a Signature and EndLine and
// its calls named by callee, and `import` directives resolved through
// pubspec.yaml's package name; every such file previously fell through to
// "no extractor registered" (the epoch 19 class), so the salt bump re-reads them.
// Epoch 27 (LB-Q-cargo, docs/design/language-breadth.md §13/§4): a Cargo.toml
// now extracts through the doc-engine cargo dialect (the package, the
// workspace, its features and explicit targets, dependency references, and the
// workspace-member and dependency edges that resolve to another manifest of the
// repository) instead of the generic TOMLExtractor's bare table/key dump
// (KindTOMLTable/KindTOMLKey); every already-indexed Cargo.toml's stored
// observations came from that generic path, so the salt bump re-reads them.
// Epoch 28 (LB-Q-swift) changes no extraction: the wave's swift dialect was
// refused at integration (design §14.2, parse errors on 5.6% of its calibration
// files, refusedDialects), so .swift stays "no extractor registered" and
// disclosed; the number is kept so the wave's history reads in order.
// Epoch 29 (LB-Q-go-mod, docs/design/language-breadth.md §13/§4): a go.mod now
// extracts through the native go-mod dialect (the module directive, a ModuleDep
// per require, a replace per replace directive, and the edges from a require or
// a local replace to another go.mod of the repository) instead of the
// incumbent's require-only line scanner, and a go.work (a workspace, its uses
// and replaces, and the edge from each use to that directory's go.mod), which
// no extractor read before, is indexed; every already-indexed go.mod was
// extracted by the scanner and every go.work fingerprinted as unclassified, so
// the salt bump re-reads them.
// Epoch 30 (LB-Q-msbuild, docs/design/language-breadth.md §13/§4): a .csproj,
// .fsproj, .vbproj, .props or .targets file now extracts through the doc-engine
// msbuild dialect (the file's project entity named by the file, its targets, its
// PackageReferences as Calls, and the ProjectReference edges that resolve to
// another project of the repository) where each was fingerprinted as an
// unclassified .NET build file; the salt bump re-reads them.
// Epoch 31 (O145, LB-F43..LB-F45): the file resolver resolves more imports,
// so more file-resolved edges come out of unchanged files: a TypeScript ESM
// spec `./x.js` (`.mjs`, `.cjs`) now resolves to its source x.ts/.tsx/.d.ts
// (.mts, .cts) as TypeScript's node16/bundler resolution maps it; `.`, `..`
// and a trailing-slash spec are directories (the repository root among them)
// resolved through package.json's main (TypeScript: typings, types, main)
// before the index file, as Node's CommonJS resolution does; and a C# `using
// A = N.T;` or `using static N.T;` resolves to the files declaring type N.T
// (its members bound to A. and, for using static, in scope unqualified). The
// file-resolved pass reuses stored imports for an unchanged fingerprint, so
// the salt bump re-reads every file.
// Epoch 32 (LB-Q-matlab): a `.m` file whose first KiB opens with a function,
// a classdef or a % comment is classified MATLAB by the dialect's match.sniff
// and read by internal/extractor.NewTSExtractor("matlab") — tree-sitter via
// wazero/WASM: classes, their properties and methods, functions (local and
// nested) each with a Signature and an EndLine. Every such file, and every
// other `.m`, was fingerprinted by the no-extractor path (`.m` is never
// claimed by extension: Objective-C shares it), so the salt bump re-reads
// them; a `.m` no sniff settles is now fingerprinted as
// ambiguous_unclassified, as design section 5 says.
// Epoch 33 (LB-Q-kustomize, docs/design/language-breadth.md §13/§4): a
// kustomization.yaml or kustomization.yml now extracts through the doc-engine
// kustomize dialect instead of kustomizeextract.go: the kustomization itself is
// an entity (Kustomization, named by the file), components and the
// configMapGenerator and secretGenerator entries are read (KustomizeResource,
// KustomizeConfigMap, KustomizeSecret), the kustomization refers to each entry
// it names, and a resource, base, component or patch that names another doc-
// engine file of the repository (today another kustomization, by its
// directory; a Kubernetes manifest is not one yet) gets a file-tier edge to
// it; a patch is signed by its target's name and an image by its newName,
// newTag or digest (the doc engine has no concatenation) where the incumbent
// composed Kind/name and newName:newTag.
// Every already-indexed kustomization file was extracted by the incumbent, so
// the salt bump re-reads them.
// Epoch 34 (LB-Q-taskfile, docs/design/language-breadth.md §13/§4): a
// Taskfile.yml, Taskfile.yaml, taskfile.yml, taskfile.yaml, Taskfile.dist.yml,
// Taskfile.dist.yaml, taskfile.dist.yml or taskfile.dist.yaml now extracts
// through the doc-engine taskfile dialect (the file's Taskfile entity named by
// the file, each task named by its key as written, the tasks it names in deps,
// cmds and defer as Calls, and the includes that resolve to another Taskfile of
// the repository) where each was fingerprinted as unclassified YAML; the salt
// bump re-reads them.
// Epoch 35 (LB-Q-emacs-lisp, LB-Q-commonlisp): .el files now extract through
// internal/extractor.NewTSExtractor("emacs-lisp") and .lisp, .lsp and .asd files
// through NewTSExtractor("commonlisp") — tree-sitter via wazero/WASM, two fleet gaps
// with no incumbent. Emacs Lisp: defun, defmacro, defsubst, cl-defun, cl-defmacro
// and cl-defsubst (each with a Signature and an EndLine), defvar and defvar-local
// (a bare `(defvar x)` too), defcustom, defconst, defface, defgroup, define-inline,
// defalias, the derived and minor modes, cl-defgeneric and cl-defmethod, cl-defstruct
// and eieio's defclass, at the top level or below it. Common Lisp: defun, defmacro,
// defgeneric (a `(setf name)` one too) and defmethod (each with a Signature and an
// EndLine), defvar, defparameter, defconstant, deftype, defstruct and defclass, at
// the top level or below it; an .asd file is claimed and defines nothing. Every such
// file was fingerprinted by the no-extractor path, so the salt bump re-reads them.
// Epoch 36 (LB-Q-tcl, LB-Q-vimscript) changes no extraction: the wave's tcl and
// vimscript dialects were refused (design §14.2, parse errors on 62.4% of the
// Tcl calibration files and 3 of the 7 Vim ones, refusedDialects), so .tcl,
// .tk, .tm, .vim and the vimrc basenames stay "no extractor registered" and
// .tcl and .vim stay disclosed; the number is kept so the wave's history reads
// in order, as epoch 28 did for swift.
// Epoch 37 (LB-Q-ada, LB-Q-vhdl, docs/design/language-breadth.md §13): .ads
// and .adb files now extract via internal/extractor.NewTSExtractor("ada") and
// .vhd and .vhdl files via NewTSExtractor("vhdl") — tree-sitter via
// wazero/WASM, fleet gaps with no incumbent. Ada: package specifications and
// bodies, subprograms (bodies, declarations, instantiations, renamings,
// expression functions), tasks, protected objects, entries, types, subtypes,
// exceptions and constants, each callable with a Signature and an EndLine and
// its calls named by callee. VHDL: entities, architectures, packages and
// package bodies, labelled processes, functions and procedures (bodies and
// declarations), types, subtypes, record elements and components. Every such
// file was fingerprinted by the no-extractor path ("no extractor registered",
// the epoch 18 class), so the salt bump re-reads them.
// Epoch 38 (O148, LB-F53): a .cs file whose tree has a syntax error is parsed
// again with each #if group's directive lines blanked, then with each group
// read as its first branch, and the first tree with no error is read
// (csharpConditionalTexts, withCSharpTree): such a file now yields its
// CSharpNamespace symbol, its types' members qualify under the right type,
// and a member declared only in an #elif or #else branch of a file read as
// first branches is no longer extracted. Every such file was fingerprinted by
// the old reading, so the salt bump re-reads them.
// Epoch 39 (LB-Q-git-hooks-config, docs/design/language-breadth.md §13/§4): a
// lefthook config (lefthook.yml, .lefthook.yml, lefthook.yaml, .lefthook.yaml and
// the four -local spellings), a .pre-commit-config.yaml and a .pre-commit-hooks.yaml
// now extract through the doc-engine git-hooks-config dialect (lefthook's hooks and
// their commands, scripts and named jobs; pre-commit's configured hooks and a hook
// repository's hook definitions, by id) where each was fingerprinted as unclassified
// YAML; the salt bump re-reads them. Also LB-Q-cloudformation, under the same bump (one
// per dispatched item): a .yaml, .yml or .json whose content is an AWS CloudFormation or
// SAM template (AWSTemplateFormatVersion, an AWS:: Transform, or a top-level Resources map
// with Namespace::Service types) now extracts through the doc-engine cloudformation dialect
// (the template, its Parameters, Mappings, Conditions, Resources and Outputs, the Ref,
// GetAtt, DependsOn and Condition references a resource or output makes to the template's
// own entities, and a nested stack's or serverless application's local template as a
// file-tier edge) where each YAML one was unclassified and each JSON one was read by the
// generic JSON extractor.
// Epoch 40 (LB-F60, docs/plans/language-breadth.md): a package-level Go const or var
// is now extracted whether or not its name is exported (a func and a type already
// were; the blank identifier is still skipped), so find_symbol and symbol_context
// answer an unexported const or var and the SCIP export and the symbol sets that
// count them now include them. Every .go file was fingerprinted by the exported-only
// reading, so the salt bump re-reads them. Also LB-Q-package-json, under the same
// bump (one per wave, as epoch 39): the package-json dialect gained a references
// block, so a package.json's package entity now calls each dependency name its
// dependencies, devDependencies, peerDependencies and optionalDependencies write
// (the Cargo.toml dialect's shape); every package.json was fingerprinted without
// those calls.
// Epoch 41 (wave lb, F-B call qualification: LB-F65, LB-F66, O260): the python
// dialect stores a call qualified (dialect.json calls.qualify): its receiver chain
// with the called name, argument and index contents dropped ("db.query().filter"),
// and a callee that ends in no name (`handlers[k](x)`, `f()()`) not at all; a
// decorator's calls now belong to the definition it decorates, and a module-level
// assignment's and an annotated class attribute's to the variable they define
// (@calls.to/@calls.from), where each was dropped as outside every function. The Go
// extractor records a method called on a selector chain or on a call's or an
// index's result ("resp.Header.Get", "strings.NewReader().Len"), where it recorded
// none, and no longer records a value no function can be (an operand of a
// comparison or arithmetic, a case value, the base of a further selector) or a
// selector's own field or method name as a call; a type stays a reference. Every .py
// and .go file was fingerprinted with the old call text, so the salt bump re-reads
// them. (The php dialect's qualification ships under the same bump but is not
// dispatched; the CGO phpextract.go still reads .php.) Also LB-F69, under the same
// bump (one per wave): the javascript and typescript dialects define an arrow-function
// or function-expression binding only at module scope (a direct child of the program
// or of an export statement), a method only in a class body, a prototype or member
// assignment only as a module-scope statement, and no object-literal member at all, so
// a call written in a dropped binding now belongs to the definition around it. A store
// ingested before it holds function-local bindings, object-literal methods and
// function-local member assignments as symbols, with their calls attributed to them;
// every .js, .jsx, .mjs, .cjs, .ts, .tsx, .mts and .cts file was fingerprinted by the
// old queries, so the salt bump re-reads them.
// Epoch 42 (wave lc, F-D reference edges: LB-F74, LB-F75, LB-F76, LB-F77, LB-F78): the
// file resolver's file_imports row for a file is now its extractor.FileSpecs (its imports,
// the names it references outside a call and the modules it re-exports, from the
// dialect's new references query, manifest queries.references), where it was its import
// list alone; an import-bound type or identifier reference (a type annotation or hint,
// `new X`, extends, implements or with, a bare `X` or qualified `Y.Z`) is a file-resolved
// edge from the definition around it, a name re-exported through a barrel or a package's
// own imports binds the file defining it, a Ruby constant is looked up lexically, a
// namespace-dir dialect whose package is a clause sees its own package's names (C#: and
// every enclosing namespace's), a protobuf import resolves under buf.yaml and
// buf.work.yaml module roots, and a namespace-dir named import whose path names no file
// binds the package's top-level name or the class's member its path less the last segment
// names. The java dialect's import of a class now binds the class's simple name (a named
// import), where it bound the whole dotted path. Every .py, .js, .jsx, .mjs, .cjs, .ts,
// .tsx, .mts, .cts, .java, .cs, .kt, .kts, .dart, .rb, .rake, .ru, Rakefile and .proto
// file's row was written in the old shape (an older row no longer decodes and is parsed
// again regardless), so the salt bump re-reads them. The php and rust dialects name their
// references queries too, so every .php and .rs file's row takes the new shape as well.
// Under the same bump: PHP properties, class constants and enum cases (LB-F81):
// phpextract.go now emits PhpProperty, PhpConst and PhpEnumCase for them and walks an
// enum's body, so every indexed .php file re-extracts. Rust members (LB-F82):
// rustextract.go now emits a struct's or union's named fields, an enum's variants and a
// struct-like variant's fields, an impl's or trait's associated consts and types, a
// trait's required and default methods, and unions (RustField, RustVariant,
// RustAssocConst, RustAssocType, RustUnion); and use trees (LB-F83): the rust dialect's
// imports.scm composes a nested group's prefixes, so a .rs file's import specs change
// shape. Every .rs file re-extracts. Item cfam (C, C++) changes no extractor output.
//
// Epoch 43 (wave px, renumbered from 42 at its 2026-10-01 merge with wave lc, which took 42
// first: LB-Q-nginx, LB-Q-haproxy, LB-Q-caddyfile, LB-F101; LB-Q-envoy,
// LB-Q-kong, LB-Q-traefik, LB-F104): nginx.conf, haproxy.cfg, Caddyfile and *.caddyfile,
// a .conf an nginx sniff claims, a .cfg a HAProxy sniff claims and a sites-enabled/*,
// sites-available/* or Caddyfile.* file are read by the native proxy dialects, where no
// extractor read them; a .conf or .cfg no sniff claims is counted ambiguous instead of
// reaching dispatch. An Envoy bootstrap or xDS file, a Kong declarative file and a Traefik
// static or file-provider file (YAML or TOML) that a manifest basename or glob names and the
// dialect's content sniff claims now extract through the doc-engine envoy, kong and traefik
// dialects (gatewayDispatch) instead of the generic YAML, JSON or TOML extractor, or none at
// all (an unclassified YAML). A store ingested before it holds those files as bare keys or
// nothing, so the salt bump re-reads them.
//
// Epoch 44 (wave tl, one bump for the wave; renumbered from 43 at its 2026-10-01 merge
// with wave px, which took 43 first, ADR-048 D2): Compose files (compose.yaml,
// compose.yml, docker-compose.yaml, docker-compose.yml) extract through the
// docker-compose doc dialect, which retired composeextract.go and adds a
// ComposeFile root entity per file whose include: and extends.file resolve to the
// included or extended file (LB-F89); a Dockerfile's FROM and COPY --from, a
// compose service's image: and build:, and a Kubernetes container's image: resolve
// to the in-repo Dockerfile that builds the image (LB-X-docker-image-ref, LB-F87);
// a YAML file whose top-level services: is a list is no longer read as Compose
// (O324). OpenAPI and Swagger documents, YAML and JSON, extract through the openapi doc
// dialect, which retired openapiextract.go (LB-Q-openapi). A .tf file joins the
// doc-xref pass and its Calls drop for-expression variables, dynamic-block
// iterators and the provider/providers meta-arguments (LB-F84, LB-F86). A *.gradle
// file extracts through the gradle dialect (LB-F94). Every such file was
// fingerprinted by the old reading, so the salt bump re-reads them.
//
// Epoch 45 (wave cx, renumbered from 43 at its 2026-10-01 merge with wave px, which took 43
// first, and from 44 at its merge with wave tl, which took 44 first). Item calls (LB-F119,
// ADR-048 amendment D1): .c files, and the .h files the CT-3 header router sniffs as C, are
// extracted by the C tags dialect, which lang-parity measured
// at least as good as the CGO cextract.go on libuv and curl (MERGE,
// docs/bench/readings/lang/parity/c.json): a #define is a CMacro and a union a CUnion, which
// cextract.go never emitted, every function carries its signature, calls follow
// dialects/c/tags.scm, and a definition the grammar names by a statement keyword (an `else
// if` opening an #ifdef branch) is none, its calls going to the function around it. Every .c
// and .h file was fingerprinted by the CGO walker, so the salt bump re-reads them. The C++
// dialect is not dispatched (lang-parity refuses it on range_agreement, parity/cpp.json):
// .cc, .cpp, .cxx, .hpp, .hh, .hxx and the router's C++ branch extract as before. Item cinc
// (LB-F121, LB-F122): file-resolve sends a .h to the language the header router sends it,
// reads include roots from in-repo compile databases and binds a call through a header
// prototype, so a .h file's file_imports rows and file-resolved edges are new and C and C++
// file edges change; the bump re-resolves an unchanged repo. Item conan (LB-Q-vcpkg,
// LB-Q-conan): vcpkg.json and vcpkg-configuration.json, read by the generic JSON extractor
// before, extract through the vcpkg doc dialect; conanfile.py (a python file before),
// conanfile.txt, conan.lock and conandata.yml through the conan tags+doc dialect. Item cmake
// (LB-Q-meson, LB-Q-compile-commands): meson.build, meson.options and meson_options.txt
// (unclaimed before) extract through the Meson dialect, and compile_commands.json (generic
// JSON before) through the compile-commands doc dialect; CMakeLists.txt and *.cmake stay on
// cmakeextract.go (lang-parity refused the dialect: level_met none).
//
// Epoch 46 (wave pc, one bump for the wave; ADR-048 D2: the integrator renumbered the items'
// bumps to origin/main's + 1, 45, and to 46 at its merge with wave cx, which took 45 first).
// Item native: a .rego file is read by the native Rego dialect (LB-F131), where no extractor
// read it, and its data imports resolve by the rego-package
// strategy (LB-F132) in the file-resolved pass, which now also serves a native dialect whose
// own reader parses its imports; a store ingested before holds no .rego entity. Item k8s: a
// .yaml or .yml whose every object document is a Kyverno policy, exception or CLI test
// (kyverno.io, policies.kyverno.io, cli.kyverno.io Test) or a Gatekeeper template, constraint
// or gator suite (templates., constraints. and test.gatekeeper.sh) extracts through the kyverno
// or gatekeeper doc dialect (policyDispatch, LB-F134) instead of k8sextract.go, which stored each
// as a K8sWorkload (`ClusterPolicy/<name>`), and the policy-ref pass (LB-F136) runs. Item
// cloud: an AWS IAM policy document (a JSON or YAML root with a Version of 2012-10-17 or
// 2008-10-17 and a Statement), an Azure Policy definition or initiative (properties.policyRule,
// a root policyRule or a bare if/then rule, or properties.policyDefinitions) and a Cloud
// Custodian policies file extract through the aws-iam-policy, azure-policy and cloud-custodian
// doc dialects (cloudPolicyDispatch, LB-F138), instead of the generic JSON extractor or none at
// all (an unclassified YAML); each emits its policy_ref references (LB-F139). A store ingested
// before holds every such file under its old reading, so the salt bump re-reads them.
//
// Epoch 47 (wave iac, one bump for the wave; ADR-048 D2: the integrator gave the three items'
// unbumped changes origin/main's 46 + 1). Item hcl (LB-Q-terragrunt, LB-Q-hcl-tools,
// LB-F141..LB-F144): terragrunt.hcl, root.hcl and a .hcl a Terragrunt sniff claims,
// *.pkr.hcl, *.pkrvars.hcl, *.nomad, *.nomad.hcl, docker-bake*.hcl and a .hcl with a top-level
// job are read by the native terragrunt and hcl-tools dialects, where no extractor read them; a
// .hcl no sniff claims is counted ambiguous; every terraform definition also carries a
// terraform.module label. Item appfw (LB-Q-arm-templates, LB-Q-bicep, LB-Q-serverless,
// LB-Q-pulumi-yaml, LB-Q-aws-cdk, LB-F145..LB-F148): an ARM template or parameters file (by
// content, appFrameworkDispatch), a .bicep or .bicepparam file (the bicep tags dialect),
// serverless.yml, Pulumi.yaml, a Pulumi.<stack>.yaml and cdk.json extract through their
// dialects, where the generic JSON or YAML extractor, or none, read them. Item k8s (LB-Q-argo,
// LB-Q-flux, LB-Q-crossplane, LB-Q-ansible, LB-F149..LB-F152): a .yaml or .yml whose every
// object document is Argo's, Flux's or Crossplane's extracts through its doc dialect
// (FluxResource, ArgoResource, CrossplaneResource; crDispatch, after wave pc's policyDispatch)
// where the Kubernetes reading read it, and Ansible playbooks, role, task, variable and
// YAML-inventory files, unclassified YAML before, through the ansible dialect (ansibleDetects).
// A store ingested before holds every such file under its old reading or none, so the salt
// bump re-reads them.
//
// Epoch 48 (wave le, one bump for the wave; ADR-048 D2: item tsjs owns the bump, items go and
// pycost report their re-extraction needs to the integrator, who folds them in here and
// renumbers to origin/main's + 1 at ship). Item tsjs (LB-F171..LB-F173): a TypeScript or
// JavaScript `new X(...)` is a call to X, call text is compacted (calls.qualify: whitespace,
// argument contents and a non-null `!` dropped, a subscript or a call's result called names no
// callee), and a decorator's call belongs to the definition it decorates (a class's, a
// method's), a class field's initializer or decorator to the class. A .ts, .tsx, .mts, .cts,
// .js, .jsx, .mjs or .cjs file fingerprinted before carried the old calls, so the salt bump
// re-reads it.
//
// Epoch 49 (wave ld, one bump for the wave; ADR-048 D2: the integrator renumbers it to
// origin/main's + 1 at ship). Item dispatch (ADR-048 amendment D1, dispatch at parity): .php
// files are extracted by the PHP tags dialect, which lang-parity measured at least as good as
// the CGO phpextract.go on guzzle and monolog (MERGE, docs/bench/readings/lang/parity/php.json;
// LB-F183): a namespaced file gains a PhpNamespace symbol named by its namespace, every
// definition's Package is the declared namespace (the directory rule only for a file with
// none), every definition carries a signature, an enum's methods are definitions, and calls
// follow dialects/php/tags.scm. Every .php file was fingerprinted by the CGO walker, so the
// salt bump re-reads them. Rust, C#, Java and C++ stay on their CGO incumbents (lang-parity
// refuses each: parity/rust.json, csharp.json, java.json, cpp.json). Item xfile (LB-F195 to
// LB-F198), folded in by the integrator: the .h C++ sniff reads code only (comments, string
// and char literals and #error/#warning text blanked), so a header whose prose or strings
// named C++ now routes to C; the c and cpp dialects name a references query (types, typedef
// names, all-capital macros, qualified names), so every .c, .h, .cc, .cpp, .cxx, .hpp, .hh and
// .hxx file's file_imports row takes the new shape and draws reference edges through the
// include graph; and the rust references query and the crate resolver (rustcrate.go: crate,
// self, super and workspace-crate roots, Cargo [lib]/[[bin]] and tests/examples/benches roots,
// pub use re-exports) change every .rs file's row and edges. The file-resolved pass reuses
// stored rows for an unchanged fingerprint, so the same bump re-reads and re-resolves them.
// Item calls changes no ingest output: ingest reads the C# dialect's references and imports
// queries only, never its calls (O462).
//
// Epoch 50 (wave fx, one bump for the wave; ADR-048 D2: the workers' 49 renumbered to
// origin/main's + 1 by the integrator). Item cue (LB-Q-cue, LB-F211..LB-F215): a .cue file
// now extracts every field CUE declares wherever its struct is written (a conjunction,
// disjunction or default operand, a list element or element type, a comprehension's source or
// body, a let's value, a selector's base), at any depth (the cap was six levels), plus a
// package entity (package/<name>, CUEPackage) per package clause; its Calls hold every
// reference to a field or definition (not only to a #definition), selectors resolved through
// the file's own structs; and an import that is no string literal is no import. The dialect
// manifest now claims .cue, so a generated .cue file (cue get go) is skipped like every
// dialect's. A .cue file fingerprinted before carried the old entities and calls, so the salt
// bump re-reads it. Item timoni (LB-Q-timoni, LB-F216..LB-F218): the native timoni dialect. A .cue file that is a
// Timoni module's (timoni.cue, a file under a timoni.cue other than cue.mod/), a bundle or a
// runtime reads through the Timoni extractor, which carries the CUE extraction's nodes
// unchanged and adds the Timoni entities (module, values, object template, bundle, bundle
// instance, runtime, cluster), values-to-#Config and template-to-#Config calls, and the
// cross-file references: a bundle instance to its local module, #Instance to the object
// templates it renders. Every such .cue file fingerprinted before has none of them; the same
// bump re-reads it. Item fmt (LB-Q-fathom-config, LB-Q-fathom-plan-rows, LB-F219..LB-F222): a
// project's .fathom/config.toml (fathom-config: sections, keys, profiles, surface patterns, the
// keys that name a profile) and a plan under docs/plans/ (fathom-plan-rows: a PlanRow per row
// that carries an id, its needs, symbol, reachable_from and package anchors) were read by the
// generic TOML and Markdown extractors before, so a store ingested earlier holds them as bare
// tables and headings; the same bump re-reads them.
//
// Epoch 51 (wave lh, one bump for the wave; ADR-048 D2: item ts owned the bump at 50, items
// javacs and pkc reported their re-extraction needs to the integrator, who folded them in here
// and renumbered to origin/main's + 1). Item ts (LB-F223..LB-F228): relative-path's TS/JS
// rule resolves a bare or `#` spec as the TypeScript compiler does with the importer's project in view
// (tsresolve.go: the nearest tsconfig.json or jsconfig.json that holds the file, its extends
// chain, paths, baseUrl and rootDirs; an in-repo workspace package by name and a package's own
// name through package.json exports, imports, typings, types and main; an output under outDir
// named back to its source), where every such spec resolved to nothing; the typescript dialect's
// imports query binds `import x = require('./y')`; and an edit to a tsconfig*.json,
// jsconfig*.json, package.json, pnpm-workspace.yaml or lerna.json alone re-runs the pass. A .ts,
// .tsx, .mts, .cts, .js, .jsx, .mjs or .cjs file fingerprinted before kept the edges the old
// rule drew, and the pass waits for an input to change, so the salt bump re-reads and
// re-resolves them. Item javacs (LB-F232, LB-F233; ADR-048 D1): `.cs` files are extracted by
// the C# tags dialect, which lang-parity measured at least as good as the CGO csharpextract.go
// on Polly and Newtonsoft.Json at L1 once the Roslyn dump dated a definition by its name (MERGE,
// docs/bench/readings/lang/parity/csharp.json, O457). A .cs file fingerprinted before holds
// csharpextract.go's nodes: no CSharpProperty, and calls that keep a generic callee's type
// arguments, keep nameof and drop a member call's receiver (LB-F189..LB-F191, O462), so the
// same bump re-reads it through the dialect. The Java dialect's constructor calls (O268) change
// no ingest output while `.java` is still served by javaextract.go (LB-F230, O513). Item pkc
// (LB-F238): the cpp dialect's references query no longer reads the last argument of an
// all-capitals function definition (`TEST(suite, name) { ... }`) as a type, so a C++ file
// (.cc, .cpp, .cxx, .hpp, .hh, .hxx, and a .h the sniff routes to C++) holding such a test
// macro stored a file_imports row whose references bound a header's same-named class; the
// file-resolved pass reuses stored rows for an unchanged fingerprint, so the same bump re-reads
// and re-resolves every C++ file. Its Kotlin and PHP work is bench-only and changes no ingest
// output.
//
// Epoch 52 (wave lx, one bump for the wave, made by the integrator; ADR-048 D2: the wave's 51
// renumbered to origin/main's + 1 when wave lh landed its own 51 first). Item python
// (LB-F241, LB-F242, O526): the tags engine's member pass names a definition a query marks
// @member Owner.name after its class, and the python dialect captures class attributes,
// annotation-only fields (the new PythonField kind), attributes a method sets on self and
// module-level assignments inside an if/for/with/try block, each assignment's calls attached
// to the definition at its node; every .py file fingerprinted before lacks those symbols and
// carries its calls on the old owners. Item dart (LB-F246, LB-F247): a `part` is a re-export
// and a wildcard import of the part and a `part of` a wildcard import of its library, so every
// .dart file's file_imports row takes the new shape, and a callable with a comment between its
// signature and its => body is now extracted. Item rust (LB-F250..LB-F252, O543): the
// references query (read for every .rs file's file_imports row whichever extractor is
// dispatched) reads `#[macro_use] mod x;` as a glob re-export and all-capitals and upper-case
// identifiers inside a macro's token tree as references, rustextract.go emits each
// macro_rules! item as a RustMacro, and the dialect captures an associated const or type alias
// under an impl or trait body only. Item ruby (LB-F254, LB-F255): a bare identifier no local
// binds is a call (the dialect's new locals query), so every .rb, .rake, .ru, Rakefile and
// Gemfile file's calls change. The file-resolved pass reuses stored rows for an unchanged
// fingerprint, so the same bump re-reads and re-resolves them all.
//
// Epoch 53 (wave lf1, one bump for the wave, made by the integrator; ADR-048 D2). Item csharp
// (LB-F286..LB-F290, O594): the C# dialect names every field and event declarator, an event with
// accessors, an enum member, a delegate type and an indexer as a definition named by its type, a
// property by its type too (`Worker.Name`, where it was bare), `new X(..)` is a call to X, the calls
// in a member's initializer or accessors are that member's, and a class is signed with its header,
// so every .cs file fingerprinted before holds the old names and calls. Item java (LB-F301..LB-F303,
// LB-F230, O611): `.java` is served by the Java tags dialect in place of javaextract.go (a
// JavaAnnotation kind, a signature on every definition, constructor calls), and a single-type import
// or the file's own type shadows a same-package class in the file resolver, so every .java file is
// re-read and re-resolved. Item ccpp (LB-F117, LB-F291..LB-F296, O367, O455, O468, O523): C++ is
// served by its tags dialect in place of cppextract.go (namespace-qualified names, macros, aliases
// and unions, data members and enumerators, a member call that keeps its receiver, the cpp-macros
// pre-pass), the C dialect names fields, enumerators and global variables, and an #include is
// followed only where the build's preprocessor can keep it, so every .c, .h, .cc, .cpp, .cxx, .hpp,
// .hh and .hxx file is re-read and re-resolved. Item go changes no ingest output (harness only).
//
// Epoch 54 (wave c1, one bump for the wave, made by the integrator; ADR-048 D2). Item ci
// (LB-F306, LB-F309..LB-F311, LB-Q-composer, O630): a workflow and an action.yml are served by the
// github-actions dialect in place of ghaextract.go (run steps, pin nodes, action definitions),
// composer.json is read as a manifest, and a YAML file no earlier signal claims is offered to the
// late-YAML dialects. Item prom (LB-F319, LB-F320): a Prometheus rule file, configuration or promtool
// test file is claimed by the prometheus dialect, its scrape jobs telemetry identities. Item mesh
// (LB-F313, LB-F314, LB-Q-otel-collector): a Kubernetes container's OTEL_SERVICE_NAME,
// OTEL_RESOURCE_ATTRIBUTES service.name and NEW_RELIC_APP_NAME are TelemetryIdentity nodes, an
// OpenTelemetry Collector configuration is read by its dialect, and every .yaml/.yml path is a route
// candidate. Item incident (TL-5): the docker-compose, serverless and cloudformation dialects emit
// TelemetryIdentity. Every such file fingerprinted before holds the old extraction.
//
// Epoch 55 (wave c2, one bump for the wave, made by the integrator; ADR-048 D2). Items java, node and
// phppy (LB-F321..LB-F336, O694, O696, O697): the route seam's framework readers (Spring MVC, JAX-RS,
// Nest, Fastify, Express, Next.js, Laravel, Symfony, Django and DRF, FastAPI, Flask) write a file's
// service_routes rows and its MOUNT rows; the express-regex reader is retired and the python-regex
// reader yields to the Flask and FastAPI readers, so every .java, .js, .jsx, .mjs, .cjs, .ts, .tsx,
// .mts, .cts, .php and .py file, and a Symfony config/routes YAML file, holds the old rows. Item engine
// (LB-F337..LB-F339, TL-9, O704): the doc engine's value predicates change the github-actions,
// docker-compose, serverless, cloudformation and otel-collector dialects' output (an id'd run step is
// its GHAStep alone, a list-form or OTEL_RESOURCE_ATTRIBUTES identity, an interpolated identity
// refused), so every workflow, action.yml, compose, serverless, CloudFormation and collector file is
// re-read. Item tl4 (TL-4) needs no bump: its log-template pass keeps its own pass state and sweeps
// every candidate on the first ingest that runs it.
//
// Epoch 56 (wave lf2, one bump for the wave, made by the integrator; ADR-048 D2). Item python
// (LB-F346..LB-F348, K18, O527): python's manifest turns on the tags engine's module symbol, so every
// .py file gains its `<module>` definition (the owner of its top-level calls) and its file-resolved
// edges change (an import of a module targets that module's symbol). Item routes2 (LB-F351, LB-F352):
// a Symfony config/routes YAML's directory resource or routing.controllers is a recursive `/**`
// Mount, a file registering @fastify/autoload a `/**?dirs` Mount, and a Fastify plugin under a
// routes/ directory no longer prefixes its own routes with its folders, so those YAML, .js, .jsx,
// .mjs, .cjs, .ts, .tsx, .mts and .cts files hold the old service_routes rows. Items go (K22, K18's
// Go half: the typed pass's environment and the harness only) and langdoc (docs and tooling) need
// no bump.
//
// Epoch 57 (wave c3, one bump for the wave, made by the integrator; ADR-048 D2). Item react
// (LB-F377, LB-F378, O731..O735): the typescript and javascript dialects emit React's kinds
// (JSComponent, JSClassComponent, JSHook, JSProp; a type alias's method now Alias.method) and a JSX
// element as a call, and React Router's routes are service_routes rows, so every .ts, .tsx, .mts,
// .cts, .js, .jsx, .mjs and .cjs file holds the old extraction. Item jenkins (LB-F373..LB-F376): a
// Jenkinsfile, a *.jenkinsfile and a vars/*.groovy fell through to no extractor and are read by the
// jenkinsfile dialect. Item obs (LB-F361..LB-F367, ADR-065): an Alertmanager configuration, a Grafana
// dashboard JSON and provisioning YAML are read by their dialects, a Kubernetes object's embedded
// dashboard, provisioning or alertmanager.yaml is read beside it, and a Prometheus rule group's
// labels are facts. Item cw (LB-F368..LB-F371): a CloudFormation metric filter's metric is a
// CFNMetric and an alarm's metric and log group identity references, and a Terraform
// aws_cloudwatch_log_metric_filter, alarm and aws_lambda_function read likewise. Every such file
// fingerprinted before holds the old extraction.
//
// Epoch 58 (wave h3, one bump for the wave, made by the integrator; ADR-048 D2). Item misc (SG-46,
// SG-47, O735, O774): a module whose exported constants hold a route path ("/app") stores them as
// ConstMethod service_routes rows, and a React Router route whose path is an imported member chain
// (paths.app.users.path) is a deferred row the seam resolves on read, so every .ts, .tsx, .mts, .cts,
// .js, .jsx, .mjs and .cjs file holding either holds the old rows. Items xref (EW-57, EW-58, SR-76:
// the doc-xref pass's Lambda gate keeps its own pass record; the secret withholding is at the
// answering layer) and query (SR-74, SR-75: read side only) need no bump.
//
// Epoch 59 (wave lf3, one bump for the wave, made by the integrator; ADR-048 D2). Item tsjs
// (LB-F381..LB-F385, K18, O721, O732): every .ts, .tsx, .mts, .cts, .js, .jsx, .mjs and .cjs file
// gains its `<module>` (JSModule), a JSConst per module-level const and, in TypeScript, a JSField per
// class field, interface or object type property, enum member and parameter property (JavaScript:
// class fields); top-level calls move to `<module>`, module imports and barrel re-exports become module
// edges, and a CommonJS export of a function or a wrapped one is a JSFunc. Item rust (LB-F387..LB-F390,
// K25 rulings 3 and 4): `.rs` is served by the Rust tags dialect in place of rustextract.go (RustConst,
// RustStatic, RustTypeAlias, an impl named by its head type) and a Rust module path is a file edge to
// the module's file. Item cgo (LB-F391..LB-F394): `.sh` and `.bash` are served by the bash tags dialect
// in place of bashextract.go (BashVariable, a signature and calls per function); `.h` changes its
// dispatch label only. Item scala (LB-F396..LB-F400): `.scala` and `.sc` are served by the Scala tags
// dialect in place of scalaextract.go (ScalaConstructor, ScalaType, ScalaVal, ScalaVar, renamed nested
// containers). Every such file fingerprinted before holds the old extraction.
//
// Epoch 60 (wave lf4, one bump for the wave, made by the integrator; ADR-048 D2). Item tsjs
// (LB-F411, D1, K26, O748): a named or default ES import whose file already reaches the imported
// file stores no module-to-module edge, and each binding import's file_imports row gains `"B":true`
// (a row an older binary wrote decodes without it), so every .ts, .tsx, .mts, .cts, .js, .jsx, .mjs
// and .cjs file holds the old edges and rows. Item shell (LB-F417, LB-F420, O756, O794): an
// extensionless script is read by the dialect its `#!` interpreter names (bash, python, ruby, node),
// where it fell through to no extractor, and `.sql` is served by the sql tags dialect in place of
// sqlextract.go (columns, end lines, a foreign key as a call). Item fileedge (LB-F431..LB-F438): a
// C++ file gains its module symbol (CppModule) and its file-resolved edges; a `.scala` and `.sc`
// definition's package is its file's package clauses and Scala imports are read; Kotlin's member
// imports change its file-resolved edges only. Item web (LB-F423..LB-F425: the CSS rule is the
// harness's, SCSS refused) needs no bump. Every such file fingerprinted before holds the old
// extraction.
//
// Epoch 61 (wave lf5, one bump for the wave, made by the integrator; ADR-048 D2). Item html
// (LB-F441..LB-F446, design 1.8 W1): every .html and .htm file gains its module symbol `<module>`
// (HTMLDocument), which calls the ids its for, form, headers, list, popovertarget, commandfor,
// itemref, ARIA ID-reference and href="#id" attributes name in the same page, and a link carrying a
// query or fragment, or naming a directory, resolves to its page as a file edge; a `./` or `../`
// spec carrying a query or fragment resolves the same way in every relative-path dialect with
// `relative` set (CSS, SCSS, Bicep, CMake, Meson, Ruby). Item css
// (LB-F459..LB-F466, design 1.8 W2): every .css file gains its module symbol (CSSModule), its var()
// and animation-name references and file edges between module symbols; `.scss` is newly dispatched
// to the scss tags dialect (SCSSMixin, SCSSFunction, SCSSVariable, SCSSClass, SCSSModule and the
// rest; @use, @forward and @import under Sass's partial rule). Items tsjs (harness rules only;
// lf5-tsjs-bindings is not merged) and cfam (harness rules only) need no bump. Every such file
// fingerprinted before holds the old extraction.
// Epoch 62 (wave lf6, one bump for the wave, made by the integrator; ADR-048 D2). Item tsjs
// (owner ruling M1, LB-F470, O749): every .js, .jsx, .mjs and .cjs file's module-level let and var
// bindings, each name a destructuring one binds, and its CommonJS value exports (exports.x,
// module.exports.x) are JSConst definitions, their initializer's calls their own. Item shell
// (O793, LB-F494): an extensionless script the shebang rule dispatches is its dialect's file in the
// file-resolve pass, as importer and as target; that changes no extraction, but an unchanged
// service's pass stays gated off until an input changes, so the bump's re-read is what draws those
// scripts' edges. Items tsjs's ruling M2 and cfam's M3 (harness rules only) and markup (a reading
// and a queue entry) need no bump. Every such file fingerprinted before holds the old extraction.
// Epoch 63 (wave lf7, one bump for the wave, made by the integrator; ADR-048 D2). Item tsx (owner
// ruling S3a, LB-F486): every .ts, .tsx, .mts and .cts file's module-level let and var bindings are
// JSConst definitions, as epoch 62 made JavaScript's. Item shell (owner ruling S1, LB-Q-bash L3,
// LB-F520): every .sh and .bash file and every extensionless script a bash or sh shebang dispatches
// has a module symbol for its top level, its source and . statements resolve as imports, and its
// top-level calls are stored only when they bind to a definition of the repository. Items vuesvelte
// and angularastro (LB-F536, LB-F544): .vue, .svelte and .astro files extract through their
// component dialects, each the host of its injected ranges (F2), where each was fingerprinted as an
// unsupported, disclosed file. F1's package strategy "file" changes no shipped dialect's key; no
// PowerShell or Angular extraction ships. Every such file fingerprinted before holds the old
// extraction.
// Epoch 64 (wave lf8, one bump for the wave, made by the integrator; ADR-048 D2). Item k8s
// (LB-F451): every Kubernetes manifest (K8sManifest and K8sCRD) stores its workloads' per-container
// facts as K8sFact rows, a CronJob's job template is a pod path for images, and a document that
// repeats a mapping key is read last-wins (O1034); templated Helm manifests are unchanged. Item
// telemetry (TL-15): a core Service's spec.selector is stored as a route-ref edge to the workloads
// it fronts, so every manifest's route-ref edges re-store. Item cifmt (LB-Q-jenkinsfile, LB-F562):
// a Jenkins pipeline's literal load step is an edge of the doc-xref pass over every Jenkinsfile,
// *.jenkinsfile and vars/*.groovy file, and a pipeline named <x>.Jenkinsfile (a case variant of the
// registered extension, O1042) reaches the jenkinsfile dialect where it was unread; GitHub Actions
// and Helm change no dialect. Item powershell (owner ruling S5, LB-F521..LB-F523, LB-F571): every
// .ps1, .psm1 and .psd1 file has a module symbol with a per-file key, file edges for dot-sourcing,
// Import-Module, `using module` and a manifest's modules, and its top-level calls are stored only
// when they bind to a definition of the repository. Every such file fingerprinted before holds the
// old extraction.
// Epoch 65 (wave lang1, one bump for the wave, made by the integrator; ADR-048 D2). Item lua
// (LB-Q-lua, LB-Q-luau): every Lua file has a <module> symbol and its literal require and
// pcall(require, ...) calls are file edges; Luau files read through the new luau dialect. Item
// graphql (LB-Q-graphql): GraphQL files read through the tree-sitter tags dialect, so operations,
// fragments, extensions, directives and enum values index and a definition's signature is its
// header as written. Every such file fingerprinted before holds the old extraction.
// Epoch 66 (wave lang2, one bump for the wave, made by the integrator). Item md
// (LB-Q-markdown): every .md file's relative links to another .md or .mdx file, and their
// fragments, are file and heading edges of the doc-xref pass, and .mdx files read as Markdown.
// The pass waits for a doc input to change, so the salt bump re-reads a store ingested earlier.
// Item ci (LB-Q-circleci, LB-Q-jupyter): CircleCI configs read through the circleci dialect and
// notebooks through the native jupyter dialect (JSON over the notebook, the python dialect over its
// code cells). Every such file fingerprinted before holds the old extraction.
const ExtractorEpoch = "66"

// FingerprintHash is the pass-3 hash-gate formula: sha256 of the raw file
// bytes salted with ExtractorEpoch, hex-encoded. Every reader that compares
// against stored fingerprints (e.g. Client.ObligationsIndexStale) MUST use
// this function — a plain content hash never matches a stored fingerprint.
func FingerprintHash(content []byte) string {
	sum := sha256.Sum256(append(content, []byte("\x00fathom-extractor-epoch:"+ExtractorEpoch)...))
	return hex.EncodeToString(sum[:])
}
