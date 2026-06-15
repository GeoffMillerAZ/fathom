// Package core defines sextant's shared domain model — the stable data contract
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
	KindDocChunk          SourceKind = "DocChunk"
	KindModuleDep         SourceKind = "ModuleDep"

	// Python
	KindPyFunc  SourceKind = "PythonFunc"
	KindPyClass SourceKind = "PythonClass"

	// JavaScript / TypeScript
	KindJSFunc   SourceKind = "JSFunc"
	KindJSClass  SourceKind = "JSClass"
	KindJSMethod SourceKind = "JSMethod"

	// Java
	KindJavaClass     SourceKind = "JavaClass"
	KindJavaInterface SourceKind = "JavaInterface"
	KindJavaMethod    SourceKind = "JavaMethod"

	// Ruby
	KindRubyClass  SourceKind = "RubyClass"
	KindRubyModule SourceKind = "RubyModule"
	KindRubyMethod SourceKind = "RubyMethod"

	// Templates
	KindTemplComponent SourceKind = "TemplComponent"
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
	Path    string // same as SourceRef.Path
	Content []byte // raw file bytes
	Lang    string // lowercase language tag: "go", "py", "js", "ts", "java", …
}

// Observation is the atomic evidence record persisted in the store. One symbol,
// fact, or chunk extracted from a source, tagged with provenance.
type Observation struct {
	ServiceID    string
	ConcernID    string // concern scope this observation is relevant to ("" = unscoped)
	SourceRef    string // repo-relative locator (file[:line])
	SourceKind   SourceKind
	EvidenceTier EvidenceTier
	Symbol       string   // symbol name, e.g. "ValidateToken"
	Claim        string   // human/agent-readable fact, e.g. "func ValidateToken(ctx, string) error"
	ContentHash  string   // sha256 of the source content (hash-gate fingerprint)
	Calls        []string // callee symbol keys, for call-graph edges
	Complexity   int      // cyclomatic complexity (0 = not applicable / not computed)
	Line         int      // 1-based line of the symbol's definition in SourceRef; 0 = unknown
	IngestedAt   time.Time
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
// Sources contains only stale entries; TotalSources and FreshSources give the
// full picture without bloating the payload with per-file rows for fresh files.
type StalenessReport struct {
	Sources             []SourceStatus `json:"sources"`
	TotalSources        int            `json:"total_sources"`
	FreshSources        int            `json:"fresh_sources"`
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
