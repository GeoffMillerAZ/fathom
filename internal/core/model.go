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

	// Ruby
	KindRubyClass  SourceKind = "RubyClass"
	KindRubyModule SourceKind = "RubyModule"
	KindRubyMethod SourceKind = "RubyMethod"

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
	IngestedAt   time.Time

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
	Service    string
	Package    string
	Name       string
	Kind       SourceKind
	FilePath   string
	Line       int
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

// Edge is a directed call/reference edge between two symbols.
type Edge struct {
	FromService string
	FromFile    string
	FromSymbol  string
	ToSymbol    string
}

// CrossRepoEdge is the atomic unit of cross-repo knowledge (platform graph).
type CrossRepoEdge struct {
	FromService string
	FromFile    string
	ToService   string
	ToKey       string
	ToFile      string
}

// Query is a concern-scoped lookup. ConcernID is mandatory (ADR-006).
type Query struct {
	ServiceID  string
	ConcernID  string
	Keywords   []string
	Kinds      []SourceKind // optional filter
	TopN       int          // 0 => default
	SemanticWt float64      // 0 => keyword-only
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
