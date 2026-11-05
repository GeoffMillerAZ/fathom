# The language library: fathom's language and tool understanding, usable without the rest of fathom

Status: accepted 2026-11-04 (ADR-092, goal G50). Plan: `docs/plans/lang-library.md` (LL-). Evidence:
`docs/assessments/2026-11-04-iac-equivalence.md`, `docs/assessments/2026-11-04-standalone-capabilities.md`. Works beside
`docs/design/iac-facts.md` (ADR-091), whose catalogs this library exposes.

Owner request (2026-11-04): "I do have a use case, I think, that is viable for having the language understanding being its
own capability that can be used from this repo without using the rest of Fathom." The named consumer is the owner's own
policy tool, which checks one baseline against infrastructure source. That tool is not built here.

## 1. Intent that applies, named before designing

- **P20 (build for a named consumer):** the owner named one in writing. Readout is the in-repository second consumer.
- **P21 (split on an enforced boundary):** a closure test makes the boundary enforced now, so a later split stays free.
- **ADR-070 (one module):** holds. None of its revisit triggers fired. Section 7.
- **ADR-063 O11, ADR-069 decision 5:** the policy kit owns the control, the assert, the admission forms and the crosswalk.
  Fathom keeps resource facts. So the evaluator, baselines and crosswalk stay out of this repository.
- **ADR-069 decision 2, ADR-065 decision 2:** secret-bearing values never enter the store. Resource values are opt-in on the
  library, and the product never opts in. Section 5.
- **I51:** memory shedding must still evict compiled grammars. Section 4.
- **I56, I14:** the library never dials. A catalog or a pack arrives as a local file or an `fs.FS`.
- **I1, P2, P23:** deterministic output, bounded work, whole-input refusal at limits.
- **P3 (never silent):** a skipped file, an unread attribute and a refused pack each say why.
- **F97:** forbids a cross-tool identity table before IF-M2 reads. Nothing here builds one.
- **Non-goal, no transformation or migration engine:** holds. The library reads. It does not convert.
- **Compatibility surface:** `pkg/lang` joins it, so a break is a `### Breaking` line.

No revisit trigger has fired. F97's `Promote when` has not fired either: ADR-091's amendment moves CloudFormation forward on
owner direction, and says so.

## 2. What the library is

Bytes and a path go in. Facts come out, with no store, no daemon, no index, no git and no network.

| Layer | Package | Holds | State |
|---|---|---|---|
| Door | `pkg/lang` | `Reader`, routing, facts per file, levels, packs | new door over existing code |
| Resource reads and contracts | `pkg/lang/iac` | resources with attribute paths and value classes, catalogs, binding, attribute checks | new door over `internal/iacfacts` |
| Conformance | `pkg/lang/langtest` | the checks a pack author runs | new |
| CLI | `cmd/fathom-lang` | the same as JSON, for tools not written in Go | new |
| Route | `internal/langroute` | dispatch and content gates, moved out of `internal/fusion` | moved |
| Classify | `internal/langclass` | file classifiers, moved out of `internal/connector`, importing only `core` and the standard library | moved |
| Extraction | `internal/extractor`, `internal/extractor/tsengine`, `internal/promql` | unchanged paths | cuts only |
| Contracts | `internal/iacfacts` | ADR-091's package | new |

Out, and stays product: `internal/store`, ingest, repository resolvers (`fileresolve.go`, `refresolve.go`), `internal/support`,
`internal/langbench`, `calibration/`, fathom's own formats (`docs/plans/*.md` rows and `.fathom/config.toml`).

## 3. The API

```go
r, err := lang.New(
    lang.WithDialects(packFS),      // optional, data packs
)
defer r.Close()

res, err := r.Extract(ctx, lang.File{Path: "main.tf", Src: src, Siblings: paths})
// res.Dialect, res.Language, res.Skipped (binary, oversize, generated, ambiguous, unclassified, not-linked), res.Symbols,
// res.References, res.Unresolved, res.Facts, res.Level, res.Incomplete, res.Notes

for _, d := range r.Dialects() { /* id, kind, engine, matches, measured level and grade */ }
```

Rules:

1. **Own types.** Public types are defined in `pkg/lang`, with no internal type in an exported signature. Fathom's ingest fields
   (`Service`, `Complexity`) stay out. A symbol carries `Kind` (the dialect's own name, for example `TerraformResource`) and
   `Canonical` (the closed vocabulary in `internal/extractor/canonical.go`). `Canonical` values are compatibility surface and only
   grow. `Kind` values are not, or language breadth would freeze. Every kind an extractor emits has a `Canonical`: a manifest
   declares it, the native table in `canonical.go` holds the kinds no manifest declares (Go, and the generic JSON and TOML readers),
   and a kind with no canonical meaning (`K8sFact`, `TerraformProvider`) is on an explicit list with its reason. A test fails on an
   empty `Canonical` anywhere else.
2. **`Siblings`** is the optional path list that YAML classification reads. Empty is legal and is disclosed when it changes a verdict.
3. **Errors, never panics.** `lang.New` returns the error a bad variant gives. `Extract` prevents what it can, and a panic in a
   built-in extractor that it did not prevent is that file's failure: an Incomplete reading with a one-line note ("the reader
   failed on this input"), so one file never ends a batch. A stack overflow is fatal and cannot be recovered, and a parser
   whose cost grows with the square of its input cannot be interrupted, so such a parser is handed a file only after a guard
   has counted what it recurses on or spends its time on, and a file past a bound reads Incomplete with the reason. The count
   is the parser's own: the HCL and CUE guards read the tokens of the lexer the parser reads (brackets, prefix operators,
   conditionals, splats, field labels, the chain of selectors, calls and indexes applied to a bracket, and in HCL the template
   directives a body leaves open and the literal pieces of one template; a `{for ...}` expression, whose newlines the parser
   ignores, is not reset at a newline). The CUE guard scans a copy in which `@` is an operator, since the scanner reads an
   attribute by recursion and no guard that drives it survives, and counts lexical errors, which the parser keeps in a list it
   searches linearly. The proto guard reads the tokens of its scanner and counts brackets, `-` and a comment where a literal is
   expected, and runs of tokens no statement ends. nginx and Caddyfile refuse in the parser, which carries the block depth. A
   second reading of the raw text drifts from the parser, so none is kept for those. A TOML document of more than 40,000 keys is
   refused too: go-toml's repeat check is quadratic (60,000 keys took 4 s). A guard is a belief about a parser, so a shape no
   guard counts is not promised refused: each belief has a test over a family of shapes at the size the guard must refuse and
   the size of a flat file it must read.
   There is a fuzz target, and a test cuts every dialect fixture at four points and fails on a panic.
4. **Concurrency.** A `Reader` is safe for concurrent use and deterministic: equal inputs give byte-equal JSON.
5. **Compilation cache.** The wazero runtime is process-wide, so the on-disk cache is a package function, `lang.EnableCompilationCache(dir)`. It errors on a second directory and refuses a group- or world-writable one, because the cache holds machine code (LB-F26). It is off by default.
6. **Limits.** The product's limits apply (2 MiB file, 64 MiB module, binary and generated sniffs). `WithLimits` may lower them. No
   option raises a module's memory cap.
7. **Measured levels.** `Level` and `Dialects()` report what readings proved, from `docs/bench/readings/lang/*.json`, through a generated
   and embedded table with a drift test. A pack dialect, and a dialect a Go extractor declares, always reads `unmeasured` in this
   version: no mechanism loads a pack's reading, and a claim to the contrary would be untrue. `Dialects()` lists only what `Extract`
   reads: a dialect refused by its plan row (swift, tcl, vimscript), one only the product reads (fathom's own formats), and one
   whose family is not linked (CUE) are not listed, and a test makes every listed dialect read a sample of its own.
8. **Incomplete.** `Result.Incomplete` is true when an engine could not fully read the file, and `Notes` carries a one-line reason
   with no stack. A policy tool that must not pass a file it could not read refuses on it. The doc engine, the HCL reader, the
   Kubernetes (an embedded document too), Helm, generic JSON and TOML, proto, notebook, nginx, Caddy, HAProxy, Rego and go.mod
   readers, a tree-sitter injected range that failed, and a Go extractor's error set it. Two kinds of engine report nothing:
   the tree-sitter grammars recover from a syntax error without reporting it (the pinned grammar blobs export no `has_error`),
   and the line scanners (CMake, templ) have no failure concept. For them `false` means no engine reported a failure, which
   the type documents. The decode failure travels on an internal field (`DocResult.Incomplete`,
   `ReportingExtractor`) that only the door reads, so the product's behavior and node output do not change: routing parity is
   byte-identical.
9. **References.** A `Reference` has `To`, the target exactly as the dialect names it (a Go call `pkg.Func`, a Terraform `var.name`,
   a CloudFormation logical id), and `ToKey`, the `Key` of the one symbol of the same file that `To` equals by `Key` or `Name`,
   else empty (another file, an ambiguous name, a cross-file reference). `Edge` is the relation the dialect declared (`ref`,
   `depends_on`, `uses`, `references`), `call` for a call by a code reader (Go, the tree-sitter dialects, templ, Rego, notebooks), and `reference` for any other engine
   that declares none (a CMake link, a Markdown link). `Line` is
   where the reference is written, 0 when the engine records none (calls read by the Go and tree-sitter engines). `Symbol.Key` is
   `Package.Name`, and only a Go call target is already one.
10. **Context and the deadline.** `Extract` checks its context on entry, after dispatch settles on an extractor, and after the
    extractor returns, never inside a parse: a deadline shorter than one parse is returned when that parse ends, not at once, and
    the result of a file read past its deadline is not returned. The engine's own 10 s parse and query deadline is the one stated
    exception to byte-equal output: a file that runs past it reads as incomplete with the budget named, and a slower machine can
    fail where a faster one does not. So does the grammar module's 64 MiB memory limit, which a 690 KB Python file of 30,000
    functions exceeds: it reads incomplete with the limit named, and no option raises it.
11. **Value domains.** `Dialect` is "" only when nothing read the file. The generic JSON and TOML readers have ids
    (`generic-json`, `generic-toml`), listed by `Dialects()` as unmeasured, so a file they read has a `Dialect`. `Level` is "" on a skipped file and a level on a read one. A file
    owned by a basename (`envoy.json`, `kong.json`) whose content check fails is labelled by the reader that read it.
12. **Withheld values.** The door withholds the places a credential is found, and says so (ADR-069 decision 2, ADR-065
    decision 2). One pass over the finished `Result` drops the userinfo of a URL wherever a string from the file lands (a name,
    a key, a reference target or its raw spelling, an unresolved target, a fact), keeping an ssh account that has no password and
    the image of a `docker` or `oci` reference; the userinfo ends at the first `/`, `\`, `?` or `#`, so the host of a URL is
    never changed. It masks a string literal bound to a name or a key (`=`, `:=`, `:`), whatever the name is, where it is a
    parameter's default (inside parentheses), a rule's value or a fact's value, and returns the program of a task and not its
    text, because a script holds a credential in more shapes than a pattern covers. The value of a credential flag and an
    Authorization header are masked in the rest of a `Signature`. A credential in another place (a name, a label, a literal that
    is an argument, a block's own attributes) is returned as written, so the door promises the places and not the absence.
    Iac resource values are separate and opt in (section 5).

## 4. The cuts

Measured by `go list -deps` at `c26ced479`: importing `internal/extractor` links 227 non-standard packages, 12 of them fathom's.

| Leak | Fix | Why not simpler |
|---|---|---|
| `extractor` imports `verify` through `planrowsdialect.go` | The plan-row extractor moves to its own package and the product registers it | It is a fathom format, so a library has no use for it |
| `tsengine` imports `memo`, which imports `callgraph` and `repofacts` | `graphstore.go` leaves `memo` for its own package, `internal/graphmemo`, so `memo` imports only `tenancy` and `tsengine` keeps using it | A port or a plain `sync.Map` would need every `main` to install it, or would break I51: `compiled.go` lets shedding evict grammars |
| `tsengine` imports `fathomhome` | `EnableCompilationCache` takes a directory. The product computes it | |
| Dispatch lives on `fusion.Pipeline` | `internal/langroute.Router` owns it. `Pipeline` embeds a router, and `ExtractForBench` delegates | `fusion` imports store and llm |
| Classifiers live in `connector` | They move to the leaf `internal/langclass`. `connector` keeps forwarding functions | Forwarding through `langroute` would link wazero and the grammars into `internal/store` and `docindex` |
| `register.go` panics | Router construction returns errors. `fusion.New` keeps its panic for its own callers | |
| `extractor` imports CUE in four files | Those move behind a registration, and `pkg/lang/cue` adds them. `Dialects()` filters what is not linked, and `Skipped` gains `not-linked` | Without it 215 external packages stay. A simulated closure with it reads 76 of the 80-package budget |

The extractor stays at its path. 299 plan rows anchor to `internal/extractor` and `internal/fusion`, and language lanes work in those
directories. A package inside the module may import `internal/...`, so nothing about being public requires a wholesale move.

The cuts do move some code, and a moved function behind a claimed row would read `claimed-but-absent`. Measured: LL-4 touches 21 claimed rows, and LL-1 and LL-7 touch 6. **Each of those commits re-anchors every row it moves, in the same commit**, so the gate stays honest.

**Proof that routing did not change:** nodes for every dialect fixture and the fauxtown corpus, captured at the base commit, stay
byte-identical after the move, through both `ExtractForBench` and `ReingestFile`, which is a second dispatch with its own order. `DispatchTable` and `docs/languages.md` do not change. If the move cannot be made without Pipeline
state, the plan stops at LL-5 and says so.

## 5. Resource reads

Extraction today keeps a resource's type and name and discards its attributes (`hclextract.go`). A policy tool, and ADR-091's attribute
checks, need more. Resource reads are the one new capability.

```go
set, err := iac.ReadFile(path, src)                  // one file
set, err := iac.ReadModule(fsys, dir)                // one Terraform directory: variable defaults and locals
```

```go
type Resource struct {
    Tool, Mode, Type, Name, Provider string // terraform, managed, aws_s3_bucket, logs, aws.west
    Span         Span
    Attrs        []Attr
    Multiplicity Multiplicity // single, count, for_each or condition, with the expression's class
}
type Attr struct {
    Path  string // versioning[*].enabled, rule[*].id
    Class Class
    Value any    // only with iac.WithValues()
    Refs  []string
    Span  Span
}
```

| Class | Meaning | A check may treat it as |
|---|---|---|
| `literal` | written as a constant | a fact |
| `defaulted` | resolves only through a variable default or a local chain ending in one | an assumption an operator can override |
| `reference` | names another resource, data source, module output or parameter | a relation |
| `expression` | computed, with no static answer | unknown |
| `unknown` | the reader could not classify it | unknown |

Absence is not a class. "This attribute is unset" comes from comparing a read with a contract, because only a contract knows what could
have been set.

**Paths.** Every nested Terraform block segment takes `[*]`, one block or several, because tofu lists blocks and one schema attribute
must have one spelling. A resource's own `lifecycle` block is meta-configuration and takes none.

**Syntax errors.** A broken expression reads `unknown` and counts in `SyntaxErrors`: an attribute on a line a syntax error points at is
`unknown`, never what the parser's recovery guessed.

**Absence is never silent.** A CloudFormation `Properties` or Pulumi `properties` the reader cannot walk (an intrinsic, an alias, a list, an
interpolation, a scalar) emits one `unknown` attribute at the root, with an empty path, and counts in `SyntaxErrors`. A resource entry that is
not read (`Fn::ForEach`, an alias, a merge-only entry, a non-mapping, one without a type) is named in `ResourceSet.Dropped`. A repeated key and
a YAML document after the first count in `SyntaxErrors`.

**Module files.** `ReadModule` follows OpenTofu: a `.tofu` file shadows the `.tf` file of the same name, which is not read. An override file
(`override.tf`, `*_override.tf` and the `.tofu` spellings) adds no resource. Each attribute or nested block it names reads `unknown` in the
resource it overrides (an empty override block names its block too, and OpenTofu replaces the base's blocks of that type whole, so an unknown
base attribute may be absent), and every attribute the override writes under that name is added as `unknown` when the base has none at that
path. A variable default or local it replaces reads `unknown`, and the resource is read once. The reader does not merge attributes.

**Coverage.** Terraform (blocks, `dynamic` as `[*]`, `count`, `for_each`), CloudFormation (`Properties`, intrinsic functions, `Condition`,
parameter defaults) and Pulumi YAML. Pulumi TypeScript and Python stay with IF-22, lane #200. Module calls read as a resource-like
relation and are never expanded. Helm is not rendered.

**Values.** `Value` is empty unless the caller passes `iac.WithValues()`. Even then, the reader itself withholds what its source marks secret, before any contract is bound:
the default of a `sensitive` or `ephemeral` Terraform variable, a CloudFormation `NoEcho` parameter, a Pulumi `secret` configuration entry and
`fn::secret`. A literal on an attribute a bound contract marks sensitive or write-only stays withheld too. In both cases a check still learns
that a literal or default is there, which is what "no hardcoded secret" asks. A value past `MaxValueBytes` (1 MiB, counted as a consumer would
serialise it: string bytes, mapping keys and `ignore_changes` names included) refuses the whole input, and so do the values of one read past `MaxTotalValueBytes` (16 MiB) in all. The
product never calls `WithValues`, proven by a test that fails on any such call outside `pkg/lang`, `cmd/fathom-lang` and tests, and a
planted sentinel appears in no store row.

**What it never does:** run `init`, a plan, a provider or a program.

**Oracles (shared code with none of them):**

| Tool | Oracle | Reads |
|---|---|---|
| Terraform | `tofu show -json -config`, `constant_value`, `references` or empty per attribute (tier A, offline after a pinned provider mirror) | path recall and class agreement on three oracle classes |
| CloudFormation | cfn-lint's own decode (tier A) | path recall and class agreement |
| Pulumi YAML | `calibration/oracles/pulumi-yaml-go` (tier A) | path recall and class agreement |

Bar: path precision at least 0.99, path recall at least 0.95, class agreement at least 0.98. A miss withholds that tool from the door.

The Terraform oracle is coarser than the classes. A probe of OpenTofu 1.13 read `var.x`, `["a", var.x]` and `"pre-${var.x}"` all as `references`, folded `true ? "a" : "b"` to a constant, read `upper("x")` as empty, and dropped `dynamic` blocks, `lifecycle` and unknown attributes. So the reading maps `literal` to constant, `reference` and `defaulted` to references, and `expression` to references or empty. A folded conditional counts as agreement, and so do a few other expressions tofu folds that the reader keeps as `expression`: `!true`, `1 + 2`, a `for` over constants and a negation of anything but a number literal. A negative number literal (`-1`, `-1.5`, and one inside a tuple or object) is `literal`, as tofu reports `constant_value`. `defaulted` is graded separately against `root_module.variables`, and locals are ungraded and disclosed. `dynamic`, `lifecycle` and paths outside the schema leave the denominators and are counted. Subjects are limited to providers in the oracle's pin list.

## 6. Packs

Dialects are already data, and the private `loadDialects(fs.FS)` already takes a filesystem. `lang.WithDialects(fs.FS)` exposes it.

| Extension | How | State |
|---|---|---|
| A doc-engine dialect (YAML, JSON, TOML and the other decoders) | `dialect.json` plus `doc.json` | LL rows |
| A tags dialect over a grammar fathom embeds | `dialect.json` plus queries | LL rows |
| A Go extractor | `lang.WithExtractor(match, e)` | LL rows |
| A catalog | `iac.OpenCatalog(path)` | IF rows |
| A new grammar | a wasm blob and an ABI check | deferred, row LL-40, with F29 |

**The built-in router is process-wide and immutable.** `dialectRules()` is a process-wide Once read by free dispatch functions, so per-Reader built-ins are not realistic. Each Reader adds a pack layer with its own classifier rules, tried after the specific built-ins and before the generic fallback for each extension, in `id` order. A plain "built-ins first" would kill the main case, because the generic path claims every `.json` and `.toml`. A pack claims by glob, basename or sniff, and may not claim a file whose extension a built-in owns outright: a glob such as `**/*` or a basename such as `main.tf` is accepted and never routed to the pack for a `.tf`, `.go` or `.md` file, and a test holds that claim table for packs and Go extractors.

Rules: a pack is merged with the built-ins and then validated by the same `checkDialect` and cross-dialect rules, and refused whole with a reason. Its queries are compiled at `New`, because `checkDialect` only stats them and "refused whole" would otherwise be false. `CanonicalKind` and the query readers learn the pack's FS. `SensitiveKind` does not: the door returns no source snippet for a sensitive entity to withhold, so a pack's `sensitive` flag has no reader in this version and the dead accessors were removed. Refused whole, with a reason: a glob that is not a valid pattern, an empty tags query, a tags capture of a kind the manifest does not declare, a variant that names its own tags query without an id (every variant is compiled), and any file or directory under `dialects/` that no manifest names (a typo'd `dialect.jsn` included). A pack's `queries.injections` are refused in this version. Queries run under the same query budget. A pack is data and runs no code on the host. `pkg/lang/langtest.CheckPack` runs validation, required
kinds and the pack's own goldens. `fathom-lang --pack DIR` gives tools not written in Go the same packs.

The product loads no pack. A repository-controlled query running in a daemon is a different trust decision, not taken here.

## 7. One module

| Measure, at `c26ced479` | Result |
|---|---|
| Consumer importing one leaf package | compiles 1 fathom package |
| Go version forced on that consumer | 1.26.2 |
| Pins raised by module graph pruning | hcl 2.20 to 2.24, x/net .30 to .61, cue .10 to .16.1 |
| Module zip per version | 49.9 MiB |

A nested module would shrink the zip and the version pressure. It would also make fathom require a tagged sibling, which
`TestRepositoryIsSelfContained` refuses, and a `replace` would break `go install`, so every language commit would become tag then bump. **Wins
if** an outside consumer reports a real version conflict. The closure test keeps that split free: everything the door reaches is already known.

## 8. Quality bar for standing alone

Each is a plan row, in the gate.

1. A closure test fails if `pkg/lang` reaches store, fusion, connector, MCP, `net/http`, or more than the budgeted package count (79: the consumer bar of 80 includes the consumer's own package).
2. A no-dial and no-write test: the library opens no socket and writes no file unless a cache directory was given.
   `net` stays in the closure through wazero, so the test watches behavior, and the closure test forbids `net/http`. A second test
   reads the source of every fathom package the door links: a file-system write may appear only in a file that writes a path its
   caller gave, with no absolute path as a literal.
3. An API snapshot from `go doc -all`, so a break is deliberate.
4. An outside module under `pkg/lang/testdata/consumer` builds offline, so importing it from outside is proven, not assumed. It
   commits its own tidied `go.mod` and `go.sum` and builds with `-mod=readonly` and `GOPROXY=off`, so it holds on a module cache
   that holds only what `go mod download` of the root module fetches.
5. Fuzz targets on `Extract`, `ReadFile` and `OpenCatalog`, with seeds in the normal test run.
6. A concurrent determinism test under `-race`.
7. A use-case doc with `## Maintained`, runnable examples in godoc, and a change note.
8. `fathom-lang`'s JSON is compatibility surface for tools not written in Go, pinned by a golden test.
9. A CLI review in `docs/app-kit.md`: stdlib flags and JSON output are a reasoned divergence, because the binary's closure is the point.
10. `task deploy` installs `fathom-lang`, and release builds publish it.

## 9. What is left to the policy tool

Baselines, the assert language (ADR-063 O11 names CEL), verdicts, adapters from a contract to a control, and the crosswalk. The numbers it will need
are in `docs/assessments/2026-11-04-iac-equivalence.md`.

## 10. Measurement and kill

| Row | Reads | Bar | If it misses |
|---|---|---|---|
| LL-M1 | closure and size of an outside consumer | at most 80 non-standard packages with no CUE, and a stripped binary (`-ldflags="-s -w"`) at most 30 MiB. Today it reads 227 and 30.0 MiB | Name what holds it and revisit the cuts |
| LL-M2 | routing parity against the base commit | byte-identical on every fixture | Stop: the move is wrong |
| LL-M3 | Terraform resource reads against `tofu show -json -config` | section 5's bar | Withhold Terraform reads |
| LL-M4 | CloudFormation resource reads against cfn-lint's decode | section 5's bar | Withhold CloudFormation reads |
| LL-M5 | Pulumi YAML resource reads against its loader | section 5's bar | Withhold Pulumi YAML reads |
| LL-M6 | unknown rate: 20 rules, each an attribute check, on 10 Terraform AWS and 10 CloudFormation roots, with Pulumi YAML reported separately. The unit is (rule, matching resource), and no matching resource is n/a. Unknown means `expression` or `unknown`, with `defaulted` reported apart, split by reusable module against deployed root. A script scores from resource reads, with the rules and the subject rule committed before reading, and a 50-pair audit by hand | unknown under 30% on Terraform and under 30% on CloudFormation | Report it. The door stays, and the guidance says source checks do not answer there |
| LL-M7 | a module outside this repository builds against a tagged release through the Go proxy | builds and runs | Fix the release before claiming G50 |

Prediction, recorded before any reading: LL-M1 passes only with CUE split. LL-M3 reads high on paths, and class agreement misses first on expressions the oracle folds or empties. LL-M6 reads under the bar on deployed roots and CloudFormation, and over it on reusable modules whose values all come from variables.

## 11. Two horizons

Built now: the door, reads for three tools, packs of data. Kept open for free: a nested module (the closure is enforced), other doors under `pkg/` (F99), and a
crosswalk beside the catalogs (a key of source, version, type and path, with no migration). Not built: a crosswalk, an evaluator, a converter, a policy
format, product-side packs, new grammars.

## Decision models

None applies. What a file declares, which class an attribute has and whether a pack is valid are all read or computed, so a model could only guess at what the
source records (I62). Provider `none` is the whole feature, and not importing the package is the opt-out.
