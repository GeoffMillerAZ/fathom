// Package core defines fathom's shared domain model — the stable data contract
// that the connector, fusion pipeline, store, extractors, and query engine all
// build against. Keeping these types in one internal package prevents drift and
// lets pkg/client re-export them for external consumers.
//
// Evidence tiers map onto agentic-kit's review/scoring foundation (see ADR-007,
// ADR-009); the query layer uses that foundation for confidence ceilings rather
// than re-deriving them here.
package core

import "time"

// SourceKind classifies what a source/observation represents. It is open-ended
// (new extractors add kinds) but the constants below are the well-known ones.
type SourceKind string

const (
	KindGoFunc            SourceKind = "GoFunc"
	KindGoMethod          SourceKind = "GoMethod"
	KindGoType            SourceKind = "GoType"
	KindGoVar             SourceKind = "GoVar"
	KindGoConst           SourceKind = "GoConst"
	KindGoInterface       SourceKind = "GoInterface"
	KindTerraformResource SourceKind = "TerraformResource"
	KindTerraformData     SourceKind = "TerraformData"
	KindTerraformModule   SourceKind = "TerraformModule"
	KindTerraformVariable SourceKind = "TerraformVariable"
	KindTerraformOutput   SourceKind = "TerraformOutput"
	KindTerraformProvider SourceKind = "TerraformProvider"
	KindTerraformLocal    SourceKind = "TerraformLocal"
	KindTerraformVarValue SourceKind = "TerraformVarValue"
	KindK8sWorkload       SourceKind = "K8sWorkload"
	KindK8sContainer      SourceKind = "K8sContainer"
	KindCUEDefinition     SourceKind = "CUEDefinition"
	KindCUEField          SourceKind = "CUEField"
	// KindCUEImport marks an import declaration captured as a reference
	// observation (ADR-021 §1 / LT-4): the extractor emits one node per
	// imported CUE package path so import edges are visible in the index.
	KindCUEImport SourceKind = "CUEImport"
	KindDocChunk  SourceKind = "DocChunk"
	KindModuleDep SourceKind = "ModuleDep"

	// Python
	KindPyFunc  SourceKind = "PythonFunc"
	KindPyClass SourceKind = "PythonClass"
	KindPyVar   SourceKind = "PythonVar"

	// JavaScript / TypeScript
	KindJSFunc      SourceKind = "JSFunc"
	KindJSClass     SourceKind = "JSClass"
	KindJSMethod    SourceKind = "JSMethod"
	KindTSInterface SourceKind = "TSInterface"
	KindTSType      SourceKind = "TSType"
	KindTSEnum      SourceKind = "TSEnum"

	// Java
	KindJavaClass     SourceKind = "JavaClass"
	KindJavaInterface SourceKind = "JavaInterface"
	KindJavaMethod    SourceKind = "JavaMethod"
	KindJavaEnum      SourceKind = "JavaEnum"
	KindJavaRecord    SourceKind = "JavaRecord"
	// KindJavaField and KindJavaImport close the wave P2 coverage gap (the
	// incumbent CGO extractor captured types+methods only): field
	// declarations and import declarations, added without moving Java off
	// its CGO tree-sitter bindings (ADR-021's XT-6b CGO->wazero migration
	// stays a separate, not-yet-triggered slice).
	KindJavaField  SourceKind = "JavaField"
	KindJavaImport SourceKind = "JavaImport"
	// KindJavaAnnotation is new with the LB-Q-java tags-engine port
	// (docs/design/language-breadth.md §13): the incumbent CGO walker never
	// emitted annotation type declarations ("@interface Foo { ... }") at
	// all — a port may add kinds beyond the incumbent, never drop one
	// (§11 step 3), same precedent as CUnion/CMacro for LB-Q-c.
	KindJavaAnnotation SourceKind = "JavaAnnotation"

	// Ruby
	KindRubyClass  SourceKind = "RubyClass"
	KindRubyModule SourceKind = "RubyModule"
	KindRubyMethod SourceKind = "RubyMethod"

	// Kotlin (ADR-021 §4 new-language breadth, wave P2; tree-sitter via
	// wazero/WASM — fwcd/tree-sitter-kotlin, no CGO incumbent to calibrate
	// against). KindKotlinFunction covers both top-level functions and
	// member methods (the grammar has one function_declaration node for
	// both, same precedent as KindPyFunc); KindKotlinProperty is top-level
	// val/var only (module-scope, mirroring KindPyVar).
	KindKotlinClass     SourceKind = "KotlinClass"
	KindKotlinInterface SourceKind = "KotlinInterface"
	KindKotlinObject    SourceKind = "KotlinObject"
	KindKotlinFunction  SourceKind = "KotlinFunction"
	KindKotlinProperty  SourceKind = "KotlinProperty"
	KindKotlinImport    SourceKind = "KotlinImport"

	// HTML (LB-Q-html, language-breadth: new — no incumbent bespoke walker
	// existed for HTML before this dialect). KindHTMLId is an element's
	// `id` attribute (the one HTML kind universal-ctags' HTML parser
	// tags as a definition; PythonClass/KotlinClass precedent for a
	// dialect's primary connector-classification kind).
	KindHTMLId SourceKind = "HTMLId"

	// CSS (LB-Q-css, language-breadth: new — a fleet gap, no incumbent).
	// KindCSSSelector is a rule's selector (the manifest's one
	// required_kind and universal-ctags' only CSS kind); the dialect's
	// other kinds (keyframes, custom properties) have no connector
	// classification use and so get no constant of their own, same
	// precedent as HTML's custom_element/form/field.
	KindCSSSelector SourceKind = "CSSSelector"

	// Rust
	KindRustFn     SourceKind = "RustFn"
	KindRustStruct SourceKind = "RustStruct"
	KindRustEnum   SourceKind = "RustEnum"
	KindRustTrait  SourceKind = "RustTrait"
	KindRustImpl   SourceKind = "RustImpl"
	KindRustMod    SourceKind = "RustMod"

	// C#
	KindCSharpClass     SourceKind = "CSharpClass"
	KindCSharpInterface SourceKind = "CSharpInterface"
	KindCSharpStruct    SourceKind = "CSharpStruct"
	KindCSharpEnum      SourceKind = "CSharpEnum"
	KindCSharpMethod    SourceKind = "CSharpMethod"
	KindCSharpNamespace SourceKind = "CSharpNamespace"

	// C
	KindCFunc    SourceKind = "CFunc"
	KindCStruct  SourceKind = "CStruct"
	KindCEnum    SourceKind = "CEnum"
	KindCTypedef SourceKind = "CTypedef"

	// C++
	KindCppFunc      SourceKind = "CppFunc"
	KindCppClass     SourceKind = "CppClass"
	KindCppStruct    SourceKind = "CppStruct"
	KindCppEnum      SourceKind = "CppEnum"
	KindCppNamespace SourceKind = "CppNamespace"

	// CMake symbols (docs/plans/cpp-toolchain.md CT-4). CMakeLists.txt and
	// *.cmake modules are intent, not artifacts: targets carry
	// target_link_libraries edges in Calls, so "who links core?" is a graph
	// question. build.ninja is generated FROM these and is never indexed
	// (CT-9 refusal).
	KindCMakeProject    SourceKind = "CMakeProject"
	KindCMakeTarget     SourceKind = "CMakeTarget"
	KindCMakeFunction   SourceKind = "CMakeFunction"
	KindCMakeOption     SourceKind = "CMakeOption"
	KindCMakeDependency SourceKind = "CMakeDependency"

	// Templates
	KindTemplComponent SourceKind = "TemplComponent"

	// Helm
	KindHelmChart      SourceKind = "HelmChart"
	KindHelmDependency SourceKind = "HelmDependency"
	KindHelmValue      SourceKind = "HelmValue"
	KindHelmTemplate   SourceKind = "HelmTemplate"

	// Docker
	KindDockerStage    SourceKind = "DockerStage"
	KindComposeService SourceKind = "ComposeService"

	// GitHub Actions
	KindGHAWorkflow SourceKind = "GHAWorkflow"
	KindGHAJob      SourceKind = "GHAJob"
	KindGHAAction   SourceKind = "GHAAction"

	// Protocol Buffers
	KindProtoMessage SourceKind = "ProtoMessage"
	KindProtoService SourceKind = "ProtoService"
	KindProtoRPC     SourceKind = "ProtoRPC"
	KindProtoEnum    SourceKind = "ProtoEnum"

	// GraphQL
	KindGraphQLType  SourceKind = "GraphQLType"
	KindGraphQLField SourceKind = "GraphQLField"

	// TOML
	KindTOMLTable SourceKind = "TOMLTable"
	KindTOMLKey   SourceKind = "TOMLKey"

	// SQL (DDL)
	KindSQLTable    SourceKind = "SQLTable"
	KindSQLView     SourceKind = "SQLView"
	KindSQLIndex    SourceKind = "SQLIndex"
	KindSQLFunction SourceKind = "SQLFunction"
	KindSQLType     SourceKind = "SQLType"

	// JSON
	KindJSONKey SourceKind = "JSONKey"

	// package.json (LB-Q-package-json, language-breadth: new — a doc-engine
	// tool format, no incumbent). KindPackageJSONPackage is the manifest's
	// own entity (the connector-classification kind); the dialect's other
	// kinds (script, bin, export) have no connector-classification use and
	// so get no constant of their own, same precedent as CSS's
	// keyframes/custom_property.
	KindPackageJSONPackage SourceKind = "PackageJSONPackage"

	// pyproject.toml (LB-Q-python-packaging, language-breadth: new — a
	// doc-engine tool format, no incumbent; requirements*.txt and
	// setup.cfg from the same queue entry stay unclassified, §14.2: the
	// doc engine decodes only YAML/JSON/TOML/XML, design §4, and neither
	// format is one of those). KindPyProjectPackage is the manifest's own
	// entity (the connector-classification kind); the dialect's other
	// kinds (script, entrypoint, tool, depgroup) have no
	// connector-classification use and so get no constant of their own,
	// same precedent as package.json's script/bin/export.
	KindPyProjectPackage SourceKind = "PyProjectPackage"

	// Scala
	KindScalaClass  SourceKind = "ScalaClass"
	KindScalaObject SourceKind = "ScalaObject"
	KindScalaTrait  SourceKind = "ScalaTrait"
	KindScalaDef    SourceKind = "ScalaDef"

	// PHP
	KindPhpClass     SourceKind = "PhpClass"
	KindPhpInterface SourceKind = "PhpInterface"
	KindPhpTrait     SourceKind = "PhpTrait"
	KindPhpFunction  SourceKind = "PhpFunction"
	KindPhpMethod    SourceKind = "PhpMethod"
	KindPhpEnum      SourceKind = "PhpEnum"

	// Bash
	KindBashFunction SourceKind = "BashFunction"

	// Lua
	KindLuaFunction SourceKind = "LuaFunction"

	// Kustomize (ADR-021 §2 / TI-3): entries under a kustomization.yaml's
	// resources/patches/images lists, emitted as reference observations —
	// previously parsed-then-discarded because the generic k8s extractor
	// only emits a workload node when metadata.name is present, and
	// kustomization.yaml never has one.
	KindKustomizeResource SourceKind = "KustomizeResource"
	KindKustomizePatch    SourceKind = "KustomizePatch"
	KindKustomizeImage    SourceKind = "KustomizeImage"

	// OpenAPI (ADR-021 §2 / TI-3): detected by a top-level openapi:/swagger:
	// key (see the fusion pass-3 classifier), distinct from the generic k8s
	// manifest shape.
	KindOpenAPIOperation SourceKind = "OpenAPIOperation"
	KindOpenAPISchema    SourceKind = "OpenAPISchema"

	// KindHelmValueRef marks a `.Values.x.y` reference found inside a Helm
	// template (ADR-021 §2 / TI-2) — distinct from KindHelmValue, which marks
	// the key's declaration in values.yaml. Both are queryable via the
	// `references` tool, giving a cheap values-key ↔ template-usage cross-ref
	// without any explicit join at ingest time (TI-7).
	KindHelmValueRef SourceKind = "HelmValueRef"

	// KindImageDockerfileRef is a synthesized cross-reference (ADR-021 §2 /
	// TI-7, cheap tier): a K8s/Compose container image whose base name
	// matches a Dockerfile found in the same repo (by parent-directory
	// name). Tier is always TierPreScan — a same-name match, not a resolved
	// build dependency.
	KindImageDockerfileRef SourceKind = "ImageDockerfileRef"
)

// EvidenceTier is the provenance/confidence tier of an observation. Values
// mirror agentic-kit's scoring tiers; the query engine resolves ceilings via
// that foundation. A code fact (TierVerified) never outranks-by-tier a doc claim.
type EvidenceTier string

const (
	TierVerified   EvidenceTier = "verified"    // extracted directly from code (ceiling 5)
	TierPreScan    EvidenceTier = "pre-scan"    // pattern matched, not confirmed (ceiling 4)
	TierDocClaimed EvidenceTier = "doc-claimed" // stated in documentation (ceiling 2)
	TierSMEStated  EvidenceTier = "sme-stated"  // said by a human expert (ceiling 2)
	TierAssumed    EvidenceTier = "assumed"     // inferred, no direct evidence (ceiling 1)
)

// ServiceRef identifies a service/repo unit to ingest.
type ServiceRef struct {
	ID   string // logical service id, e.g. "payment-service"
	Path string // local path or repo-relative root the connector resolves
	Repo string // optional VCS coordinate (e.g. "org/repo") for remote connectors
}

// SourceRef points at a single fetchable source (a file) within a service.
type SourceRef struct {
	Service string // ServiceRef.ID this source belongs to
	Path    string // repo-relative path, e.g. "internal/auth/token.go"
	Kind    SourceKind
}

// SourceFile is a single fetched source file with its raw content and a
// language hint derived from the file extension.  It is the unit passed to
// per-file analyzers in the source-scanning tools.
type SourceFile struct {
	Path      string // same as SourceRef.Path
	Content   []byte // raw file bytes
	Lang      string // lowercase language tag: "go", "py", "js", "ts", "java", …
	Generated bool   // true when the first lines match the "Code generated ... DO NOT EDIT" convention (ADR-021 §2 / TI-5) — see connector.IsGenerated
}

// LangBinary and LangUnclassified are the two well-known SourceFile.Lang
// sentinel values that keep language reporting honest (ADR-021 §1):
//
//   - LangBinary marks content that fails connector.IsBinary's NUL-byte sniff.
//     Binary files are excluded from extraction and line counting; cloc
//     reports them in a files/bytes-only bucket, never as counted lines.
//   - LangUnclassified marks a file whose path matches no entry in the
//     connector's extension→language registry (no extractor is registered
//     for it). It is always reported explicitly — never silently folded into
//     an "other" bucket — so a gap in language coverage stays visible.
const (
	LangBinary       = "binary"
	LangUnclassified = "unclassified"
)

// Observation is the atomic evidence record persisted in the store. One symbol,
// fact, or chunk extracted from a source, tagged with provenance.
type Observation struct {
	ServiceID    string
	ConcernID    string // concern scope this observation is relevant to ("" = unscoped)
	SourceRef    string // repo-relative locator (file[:line])
	SourceKind   SourceKind
	EvidenceTier EvidenceTier
	Symbol       string   // symbol name, e.g. "ValidateToken"
	Package      string   // package / namespace / receiver of the symbol ("" = none)
	Claim        string   // human/agent-readable fact, e.g. "func ValidateToken(ctx, string) error"
	ContentHash  string   // sha256 of the source content (hash-gate fingerprint)
	Calls        []string // callee symbol keys, for call-graph edges
	Complexity   int      // cyclomatic complexity (0 = not applicable / not computed)
	Line         int      // 1-based line of the symbol's definition in SourceRef; 0 = unknown
	// EndLine is the 1-based last line covered by this observation in
	// SourceRef; 0 = unknown/not computed (T5-5, docs/plans/trust-and-
	// tokenomics.md — kills the T08 class where a DocChunk's single heading
	// Line forced downstream arithmetic to GUESS a section's extent). Set
	// by: the Markdown extractor (KindDocChunk, line before the next
	// heading or EOF for the last one); the Go extractor (func/method/type/
	// const/var, from the declaration's own ast.Node.End()); the CGO
	// tree-sitter extractors — C, C++, C#, Java, Lua, PHP, Rust, Scala —
	// and the CUE, HCL/Terraform, and CMake extractors (from the parsed
	// node's own end position, or — CMake only — the scanned invocation's
	// closing paren); the Templ extractor (KindTemplComponent, via the same
	// next-declaration adjacency technique as Markdown, since it has no
	// AST). A single-line declaration gets EndLine == Line, not
	// 0 — 0 means "not computed" (see docs/plans/extraction-treesitter.md
	// for the query-based wazero engine covering .py/.js/.jsx/.ts/.tsx/.rb,
	// which does not compute it yet: the engine's Node snapshot carries no
	// end row, only start). Extractors not listed here (SQL, GraphQL,
	// Protocol Buffers) either have no AST end position at hand (regex
	// line-scan) or the library in use records only a token's start.
	EndLine    int `json:"end_line,omitempty"`
	IngestedAt time.Time

	// ToolKind is the pass-3 classifier's verdict on what tool the source
	// file actually is (ADR-021 §2), e.g. "k8s", "k8s-crd", "compose",
	// "helm-template-yaml", "cue-module", "timoni" — set for files that pass
	// through the yaml/cue-family classifier; "" for every other language,
	// which is unambiguous by extension and never needs classification.
	ToolKind string `json:"tool_kind,omitempty"`
	// Generated marks an observation extracted from a file whose header
	// matches the "Code generated ... DO NOT EDIT" convention (ADR-021 §2 /
	// TI-5). Generated files are still extracted normally; this flag lets
	// INDEX-side consumers (symbol queries, review tooling reading
	// observations) separate generated symbols without re-fetching source.
	// Presentation-layer tools that already hold file content (cloc via
	// Client.Sources) recompute the same signal live with
	// connector.IsGenerated — the same live-computed pattern as
	// SourceFile.Lang — rather than reading it from here.
	Generated bool `json:"generated,omitempty"`
}

// SymbolNode is a symbol with its location and outgoing call edges — the unit
// produced by extractors and returned by queries.
type SymbolNode struct {
	Service  string
	Package  string
	Name     string
	Kind     SourceKind
	FilePath string
	Line     int
	// EndLine mirrors Observation.EndLine (T5-5): the 1-based last line
	// covered by this node in FilePath; 0 = unknown/not computed. See
	// Observation.EndLine's doc comment for why this exists and which
	// extractors set it.
	EndLine    int
	Signature  string
	Calls      []string // callee keys (see SymbolKey)
	Tier       EvidenceTier
	Complexity int // cyclomatic complexity (0 = not applicable / not computed)

	// ToolKind, when non-empty, is the extractor's PER-SYMBOL tool-identity
	// verdict, overriding the fusion classifier's per-FILE verdict on the
	// resulting Observation (ADR-021 §2). Tool identity is a per-document
	// fact for multi-document YAML streams — a file holding a CRD and a
	// Deployment is "k8s-crd" for one document and "k8s" for the other —
	// so the k8s extractor, which sees document boundaries, tags each node;
	// extractors that leave it empty inherit the file-level verdict.
	ToolKind string
}

// EdgeKind classifies HOW a typed call edge was resolved by the Go typed
// enrichment pass (internal/fusion.EnrichTypedEdges, ADR-021 §3). It is the
// empty string for edges that predate typed resolution — the ad hoc,
// syntax-only edges internal/analysis/archconformance and internal/mcp's
// DOT/arch-conformance handlers still build directly from Observation.Calls
// never set it — so a reader can tell typed (compiler-grade) evidence from
// raw syntax by checking Kind != "".
type EdgeKind string

const (
	// EdgeDirect is a statically resolved call: a plain function call, or a
	// method call whose receiver's concrete type is known at the call site
	// (types.Selection resolves to exactly one method).
	EdgeDirect EdgeKind = "direct"
	// EdgeIfaceDispatch is one edge to ONE possible implementer of an
	// interface-typed method call. A single call site with N implementers in
	// scope emits N such edges (one per implementer), never a single
	// ambiguous edge — sum the edges sharing a From to get "reached via any
	// of N" (the honesty upgrade over the old refusal, ADR-021 §3).
	EdgeIfaceDispatch EdgeKind = "iface-dispatch"
	// EdgeUnresolved marks a call site the typed pass could not resolve to a
	// concrete target: a func value, a higher-order call, or any call whose
	// static type doesn't reduce to a single method-set member. Recorded
	// explicitly — never silently dropped — so a missing edge always means
	// "not a call", not "the resolver gave up quietly".
	EdgeUnresolved EdgeKind = "unresolved"
	// EdgeFuncValue marks a function or method referenced as a VALUE, not
	// called at this site: mux.HandleFunc(p, h), a route-table literal
	// {p, h}, an http.HandlerFunc(h) conversion, middleware(h), a method
	// value s.handleX (VS-6, docs/plans/verify-spine.md). An enclosing
	// function that only ever REGISTERS a handler this way still reaches it
	// — find_callers/TestImpact walk these edges exactly like any other
	// callee, regardless of Kind. A method value through an interface-typed
	// receiver fans out like EdgeIfaceDispatch: one EdgeFuncValue edge per
	// in-scope implementer, never a single ambiguous edge.
	EdgeFuncValue EdgeKind = "func-value"
	// EdgeFileResolved marks an edge from internal/fusion's per-file
	// import/module-scope resolver (ADR-041 decision 5, docs/plans/
	// substrate.md SU-11: TypeScript/JavaScript/Python when no SCIP index
	// is present for the service) — a call site whose callee is bound by a
	// parsed import statement (resolved to a file in the corpus) or
	// defined at module scope in the same file. This is STRICTLY WEAKER
	// evidence than a compiler/type-checker-confirmed edge: no type
	// information, no overload resolution, just import-graph plus
	// name-in-scope matching (stack-graphs-style, not stack-graphs-grade).
	// It is tier pre-scan and MUST count as heuristic, never resolved —
	// core.EdgeConfidenceOf special-cases this kind explicitly rather than
	// falling into its "any other non-empty Kind is resolved" default, and
	// every consumer (find_callers/resolved_nav/blast_radius) inherits that
	// via EdgeConfidenceOf rather than switching on Kind itself.
	EdgeFileResolved EdgeKind = "file-resolved"
)

// Edge is a directed call/reference edge between two symbols.
//
// FromSymbol/ToSymbol hold "pkg.Name"-style strings and Kind is "" for the
// legacy syntax-only producers (archconformance, the mcp ad hoc edges built
// straight from Observation.Calls). The Go typed pass
// (internal/fusion.EnrichTypedEdges) populates FromSymbol/ToSymbol with full
// canonical SymbolKeys ("service::pkg.Name") and Kind; these are the only
// Edge values the store round-trips (store.PutTypedEdges / TypedEdges).
type Edge struct {
	FromService string
	FromFile    string
	FromSymbol  string
	ToSymbol    string
	Kind        EdgeKind // "" for legacy/syntax-only edges; set for typed edges
}

// CrossRepoEdgeKind classifies WHAT KIND of cross-repo relationship an edge
// records. It is additive (vision 02 §4.7: edge kinds grow one at a time)
// and defaults to the zero value for every CrossRepoEdge ever produced
// before this field existed, so no existing producer or consumer changes
// behavior.
type CrossRepoEdgeKind string

const (
	// CrossRepoEdgeCall is the original cross-repo relationship
	// (platformlink.BuildFromSymbols): a caller in FromService invokes a
	// symbol ("pkg.Name") that resolves to a definition in ToService. The
	// zero value, named explicitly so a reader need not infer "empty means
	// call".
	CrossRepoEdgeCall CrossRepoEdgeKind = ""
	// CrossRepoEdgeServiceRef (VB-30, docs/vision/29-verbs-not-lookups.md
	// §I; internal/analysis/serviceref) is a client->handler join:
	// FromService issues an outbound HTTP call whose normalized (method,
	// path template) matches a route ToService registers. ToKey carries
	// "METHOD /normalized/template" — the same "METHOD path" convention
	// internal/mcp's traverse tool already uses for graph.KindRoute — never
	// a bare symbol name. Always EdgeConfidenceHeuristic
	// (CrossRepoEdgeConfidenceOf): a normalized-string match across
	// repositories is never confirmed by a compiler-grade resolver.
	CrossRepoEdgeServiceRef CrossRepoEdgeKind = "service_ref"
)

// CrossRepoEdge is the atomic unit of cross-repo knowledge (platform graph).
type CrossRepoEdge struct {
	FromService string
	FromFile    string
	ToService   string
	ToKey       string
	ToFile      string
	// Kind classifies the edge (additive; "" == CrossRepoEdgeCall, the
	// original and only kind before VB-30).
	Kind CrossRepoEdgeKind
}

// Query is a concern-scoped lookup. ConcernID is mandatory (ADR-006).
type Query struct {
	ServiceID string
	ConcernID string
	Keywords  []string
	Kinds     []SourceKind // optional filter
	TopN      int          // 0 => default
}

// QueryResult is the typed answer to a Query.
type QueryResult struct {
	Nodes          []SymbolNode
	Staleness      StalenessReport
	Contradictions []Contradiction
}

// StalenessReport tells callers whether the underlying data is fresh (ADR-008).
// It is constant-size by design — it rides on every query answer, so it must
// never dwarf the answer itself: Sources holds at most a few example rows (the
// oldest stale sources); the counts and OldestIngestedAt carry the full
// picture. The full source list and commit drift live behind the
// staleness_report tool.
type StalenessReport struct {
	Sources             []SourceStatus `json:"sources,omitempty"` // oldest stale sources, capped (examples, not the full list)
	StaleSources        int            `json:"stale_sources"`
	TotalSources        int            `json:"total_sources"`
	FreshSources        int            `json:"fresh_sources"`
	OldestIngestedAt    time.Time      `json:"oldest_ingested_at,omitzero"` // ingest low-water mark across all sources
	ReingestRecommended bool           `json:"reingest_recommended"`
}

// SourceStatus is the freshness of one ingested source.
type SourceStatus struct {
	SourceRef    string
	LastIngested time.Time
	Stale        bool
}

// Contradiction reports two observations asserting incompatible claims about the
// same symbol (surfaced via agentic-kit evidence semantics at the query layer).
type Contradiction struct {
	Symbol string
	A      Observation
	B      Observation
}
