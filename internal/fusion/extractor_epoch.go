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
// Epoch 43 (wave tl, one bump for the wave): Compose files (compose.yaml,
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
const ExtractorEpoch = "43"

// FingerprintHash is the pass-3 hash-gate formula: sha256 of the raw file
// bytes salted with ExtractorEpoch, hex-encoded. Every reader that compares
// against stored fingerprints (e.g. Client.ObligationsIndexStale) MUST use
// this function — a plain content hash never matches a stored fingerprint.
func FingerprintHash(content []byte) string {
	sum := sha256.Sum256(append(content, []byte("\x00fathom-extractor-epoch:"+ExtractorEpoch)...))
	return hex.EncodeToString(sum[:])
}
