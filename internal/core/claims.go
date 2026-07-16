package core

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

// claims.go — the semantic-layer domain model (ADR-016, ADR-017).
//
// A SemanticClaim is the unit of compiled knowledge: a governed restatement
// of a governing source, anchored by citations (ClaimLink rel="cites") and
// joined to code by scope globs (rel="scopes"). Claims live in additive
// tables BESIDE observations — never inside them: re-ingest purges
// observations destructively per source, and the kind-blind contradiction
// scan would false-positive against them.

// ClaimStatus is the freshness/veracity state of a claim. Claims are never
// deleted by compilation — they are status-flipped, so audit history and
// "that rule went stale" (as opposed to silent absence) are both preserved.
type ClaimStatus string

const (
	// ClaimFresh — citations verified at last sweep; served normally.
	ClaimFresh ClaimStatus = "fresh"
	// ClaimStale — a cited line range changed since compile; served flagged,
	// never silently dropped and never silently trusted.
	ClaimStale ClaimStatus = "stale"
	// ClaimContradicted — a citation no longer resolves at all (file or
	// range gone); served flagged, strongest drift signal short of refutation.
	ClaimContradicted ClaimStatus = "contradicted"
	// ClaimSuperseded — a newer compile of the same subject produced a
	// replacement claim; not served.
	ClaimSuperseded ClaimStatus = "superseded"
	// ClaimRefuted — a human (or machine) verdict rejected the claim via
	// claim_events; not served. Human corroboration never promotes tier —
	// refutation is the only verdict with serving consequences.
	ClaimRefuted ClaimStatus = "refuted"
)

// Valid reports whether s is a known ClaimStatus.
func (s ClaimStatus) Valid() bool {
	switch s {
	case ClaimFresh, ClaimStale, ClaimContradicted, ClaimSuperseded, ClaimRefuted:
		return true
	}
	return false
}

// Served reports whether a claim in this status may appear in a payload
// (flagged or not). Superseded and refuted claims are never served.
func (s ClaimStatus) Served() bool {
	return s == ClaimFresh || s == ClaimStale || s == ClaimContradicted
}

// SemanticClaim is one compiled, governed claim.
type SemanticClaim struct {
	// ClaimID is content-addressed: sha256 over (layer, kind, subject,
	// repro key) — stable across recompiles of unchanged inputs, so every
	// deferred mechanism (lifecycle ledger, feedback, links) can attach to
	// it non-destructively. Compute with ClaimID().
	ClaimID   string
	ServiceID string

	Layer        string   // e.g. "obligations"
	LayerVersion string   // layer definition version, e.g. "v1"
	Kind         string   // v1: "obligation"; later: "finding", "failure-mode", …
	Subject      string   // open ref: the governing source section, e.g. "CLAUDE.md#deterministic-excellence-gate"
	Rule         string   // the claim text served to agents (budgeted at compile)
	Scope        []string // sorted glob(s) over code paths this claim governs

	// Tier is the evidence tier (v1 always TierDocClaimed: the rule text is
	// a citation-grounded restatement; only the scope mapping is inferred).
	Tier EvidenceTier
	// TierProfile names the tier vocabulary this claim's Tier belongs to
	// (ADR-017: data now, machinery at its revival evidence). V1: "engineering".
	TierProfile string

	Status ClaimStatus

	// Provenance (flattened; the full aiprovenance retrofit attaches by ClaimID).
	Model         string // model ID that compiled the claim ("" for deterministic passes)
	PromptVersion string // versioned prompt identity, e.g. "obligations/v1"
	ReproKey      string // provenance repro key: InputDigest is the default
	InputDigest   string // sha256 of the source section the claim was compiled from

	// CompiledAtWatermark is the service corpus watermark at compile time
	// (git HEAD or manifest hash — the honest "as of" every payload carries).
	CompiledAtWatermark string
	CompiledAt          time.Time
}

// ClaimID computes the content-addressed identity for a claim.
func ClaimID(layer, kind, subject, reproKey string) string {
	h := sha256.Sum256([]byte(layer + "\x00" + kind + "\x00" + subject + "\x00" + reproKey))
	return hex.EncodeToString(h[:])
}

// ClaimLink is a typed edge from a claim to a target. The rel vocabulary is
// open (ADR-016); v1 materializes only "cites" and "scopes".
type ClaimLink struct {
	ClaimID string
	Rel     string // "cites" | "scopes" (reserved: supports, contradicts, supersedes)
	// TargetRef: for "cites", "path#Lstart-Lend" into the governing corpus;
	// for "scopes", the code glob. Always repo-relative (ADR-017).
	TargetRef string
	// TargetHash: for "cites", sha256 of the exact cited lines (the sweep
	// re-hashes these to detect drift). Empty for "scopes".
	TargetHash string
}

const (
	RelCites  = "cites"
	RelScopes = "scopes"
)

// CiteRef builds a citation TargetRef. Lines are 1-based and inclusive.
func CiteRef(path string, startLine, endLine int) string {
	return path + "#L" + itoa(startLine) + "-L" + itoa(endLine)
}

// ParseCiteRef splits a citation TargetRef into path and 1-based inclusive
// line range. ok is false when ref is not a well-formed citation.
func ParseCiteRef(ref string) (path string, start, end int, ok bool) {
	i := strings.LastIndex(ref, "#L")
	if i < 0 {
		return "", 0, 0, false
	}
	path = ref[:i]
	rangePart := ref[i+2:]
	j := strings.Index(rangePart, "-L")
	if j < 0 {
		return "", 0, 0, false
	}
	start, ok1 := atoi(rangePart[:j])
	end, ok2 := atoi(rangePart[j+2:])
	if !ok1 || !ok2 || path == "" || start < 1 || end < start {
		return "", 0, 0, false
	}
	return path, start, end, true
}

// HashLines returns the sha256 of the 1-based inclusive line range of content,
// with lines joined by "\n" (line endings normalized). This is the citation
// hash contract shared by the compiler and the sweep.
func HashLines(content []byte, start, end int) (string, bool) {
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	if start < 1 || end < start || end > len(lines) {
		return "", false
	}
	h := sha256.Sum256([]byte(strings.Join(lines[start-1:end], "\n")))
	return hex.EncodeToString(h[:]), true
}

// CompileRun is the per-run cost/audit ledger row (real usage, never
// heuristics — ADR-016 §4.1).
type CompileRun struct {
	RunID            string // sha256 of (service, layer, corpus watermark, started-at)
	ServiceID        string
	Layer            string
	CorpusWatermark  string
	Model            string
	PromptVersion    string
	DryRun           bool
	TokensIn         int
	TokensOut        int
	CostUSD          float64
	DurationMS       int64
	SectionsTotal    int // sections in the corpus
	SectionsCompiled int // sections that missed the digest gate (spent tokens)
	SectionsFailed   int // sections skipped for a malformed/truncated LLM response
	ClaimsEmitted    int
	ClaimsRejected   int // failed compile-time anchor/scope verification
	RanAt            time.Time

	// Refused is true when the run's pre-flight cost estimate exceeded the
	// layer's max_usd_per_run budget: the run never started (no LLM calls,
	// no claims written) but the estimate is still ledgered here — refusals
	// are recorded, not silent (ADR-016 §4.1, M1-7 enforceBudgets).
	Refused bool
	// RefusalReason explains Refused or BudgetStopped (empty otherwise).
	RefusalReason string
	// BudgetStopped is true when the run's running token total crossed
	// max_tokens_per_run mid-corpus: the run stopped cleanly after storing
	// whatever it had already compiled. Sections not yet reached are
	// untouched and resume via the digest gate on the next invocation.
	BudgetStopped bool
}

// ClaimEvent is one append-only human/machine verdict on a claim (the refute
// channel the truth lens demanded — ADR-016 §4.1).
type ClaimEvent struct {
	ClaimID string
	Verdict string // "refuted" | "corroborated"
	Author  string
	Note    string
	At      time.Time
}

func itoa(n int) string { return strconv.Itoa(n) }

func atoi(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	return n, err == nil
}
