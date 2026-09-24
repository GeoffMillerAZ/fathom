package core

// ReceiptKind is the closed set of claim shapes `fathom receipt verify`
// (VB-20, docs/plans/verbs.md; ADR-034 receipts) knows how to re-derive
// from a bare git checkout with go/types and git alone.
type ReceiptKind string

const (
	// ReceiptKindCallers re-derives find_callers' claim: every caller of
	// Symbol, compared against go/types' own call graph.
	ReceiptKindCallers ReceiptKind = "callers"
	// ReceiptKindReferences re-derives references' claim: every file
	// mentioning Symbol, compared against go/types' identifier resolution
	// (file granularity — the level go/types' Uses map itself carries).
	ReceiptKindReferences ReceiptKind = "references"
	// ReceiptKindSymbolLocation re-derives a definition-site claim
	// (find_symbol, or a references definition row): File/Line for
	// Symbol, compared against a fresh go/parser pass over File.
	ReceiptKindSymbolLocation ReceiptKind = "symbol_location"
	// ReceiptKindCertification re-derives a certifications-layer claim's
	// reach (vision 33, ADR-043 §4; docs/plans/test-certifications.md
	// TC-13): that Test's call graph, independently walked with go/types,
	// reaches every symbol named in Reached. A reached test is not a
	// passing test (ADR-043 decision 5) — `fathom receipt verify` reports
	// the run itself as "unverified (no run)" unless --run-tests is
	// given, in which case it actually runs Test and reports pass/fail.
	ReceiptKindCertification ReceiptKind = "certification"
)

// Receipt is the saved envelope of one agent-surface answer (ADR-034
// receipts; VB-20, docs/vision/29-verbs-not-lookups.md §3 wave E): the
// caller's own query plus the facts the answer carried, anchored to
// IndexCommit — everything a stranger needs to re-derive the SAME claim
// from a bare checkout with go/types and git alone, no fathom daemon or
// store involved. A caller builds one by saving the envelope an answer
// carried (VB-19's resolution/index_commit fields) alongside the query
// that produced it.
type Receipt struct {
	// Kind selects which re-derivation `fathom receipt verify` runs.
	Kind ReceiptKind `json:"kind"`
	// Tool names the agent-surface tool that produced this receipt.
	// Documentation only — Kind, not Tool, drives verification.
	Tool string `json:"tool,omitempty"`
	// Service is the fathom service ID the original query named.
	// Documentation only — verification runs against --repo, a path the
	// verifier is told separately, since a bare checkout carries no
	// fathom service registry to resolve it from.
	Service string `json:"service,omitempty"`
	// Symbol is the queried symbol, in go/types' own vocabulary: a bare
	// function name ("Charge") or "Recv.Method" ("Server.Handle") — the
	// same form a fully-qualified fathom SymbolKey's trailing segment
	// reduces to (see receiptverify.BareSymbolName).
	Symbol string `json:"symbol"`
	// IndexCommit anchors the claim to the commit the daemon answered
	// from (the envelope's own `index_commit`, VB-19) — the commit
	// `receipt verify` checks out to re-derive it.
	IndexCommit string `json:"index_commit"`

	// Callers is set when Kind is ReceiptKindCallers: every caller's
	// qualified name the original find_callers answer returned.
	Callers []string `json:"callers,omitempty"`

	// Files is set when Kind is ReceiptKindReferences: every file the
	// original references answer's occurrences+definitions named.
	Files []string `json:"files,omitempty"`

	// File/Line are set when Kind is ReceiptKindSymbolLocation: the
	// definition site the original answer named for Symbol.
	File string `json:"file,omitempty"`
	Line int    `json:"line,omitempty"`

	// Test is set when Kind is ReceiptKindCertification: the certifying
	// test, in the same "dir/pkg.Name" vocabulary as Symbol (a Test
	// function is never a method, so exactly one dot separates the
	// repo-relative package directory from the function name) — the form
	// `receipt verify --run-tests` builds its `go test <pkg>` invocation
	// from.
	Test string `json:"test,omitempty"`
	// Target is set when Kind is ReceiptKindCertification: the
	// certification's target id (e.g. "invariant:I35"). Documentation
	// only — not re-derived.
	Target string `json:"target,omitempty"`
	// Strength is set when Kind is ReceiptKindCertification: the
	// declared certification strength (design doc §3: exists, reaches,
	// covers, kills). Documentation only — not re-derived.
	Strength string `json:"strength,omitempty"`
	// Reached is set when Kind is ReceiptKindCertification: the symbol
	// keys the original certification found Test's call graph reaching.
	// `receipt verify` confirms every one is independently reachable
	// from Test with go/types; the first one that is not names the
	// difference.
	Reached []string `json:"reached,omitempty"`
	// RunDigest is set when Kind is ReceiptKindCertification: a content
	// digest of the attested run the certification's state was last
	// computed from. Documentation only — `receipt verify --run-tests`
	// runs Test fresh rather than trusting this digest.
	RunDigest string `json:"run_digest,omitempty"`
}
