// Package memory implements anchored memory (docs/design/anchored-memory.md,
// phase W of docs/plans/ambient-fathom.md) — the only memory fathom will
// store, and the refusal that keeps it from becoming a junk drawer.
//
// A memory is one falsifiable sentence ABOUT this codebase, bound to at least
// one anchor (a symbol, a file, a path glob, a commit, or another memory) and
// backed by at least one piece of structural evidence. Two rules carry the
// whole design:
//
//   - A memory with NO anchor is REFUSED, with the reason — never stored at a
//     low tier. An unanchored claim is a preference, and fathom cannot expire
//     a preference, so it declines to remember one (§2.1).
//   - The evidence tier is DERIVED from the evidence's structural kind and
//     the confidence is capped through agentic-kit's scoring.ApplyCeiling at
//     the write path. The caller's claimed tier is recorded as an audit trail
//     and otherwise not trusted (law 14, §2.2).
//
// Anchor drift is evaluated at READ time (EvaluateState) from hashes fathom
// already maintains — there is no sweep, no timer, and nothing to miss while
// the daemon is down. See state.go.
//
// Determinism: every list this package returns is sorted; every time value
// comes from an injected clock. No function here calls time.Now().
//
// Concurrency: Memory, Anchor and Evidence are plain values, safe to share.
// DriftCache is safe for concurrent use via sync.Mutex. FileStore serializes
// its own reads and writes the same way.
package memory

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// AnchorKind names what a memory is ABOUT. The vocabulary is closed: an
// unrecognized kind is refused at the write path rather than silently stored
// as something fathom cannot evaluate for drift.
type AnchorKind string

const (
	// AnchorSymbol binds to a symbol in the index. The sharpest anchor and
	// the default; drifts when the symbol's content hash changes.
	AnchorSymbol AnchorKind = "symbol"
	// AnchorFile binds to one source_ref; drifts when the file's content
	// hash changes.
	AnchorFile AnchorKind = "file"
	// AnchorPath binds to a glob; drifts when any matching file changes.
	// The loosest anchor — see LooseAnchorWarning.
	AnchorPath AnchorKind = "path"
	// AnchorCommit binds to a commit sha. A historical fact: it never
	// drifts, and it never orphans either.
	AnchorCommit AnchorKind = "commit"
	// AnchorClaim binds to another claim's (or memory's) id; drifts when
	// that claim's state changes.
	AnchorClaim AnchorKind = "claim"
)

// anchorSpecificity orders anchor kinds from sharpest (0) to loosest. It is
// the "specificity" term of the deterministic surfacing order (tier, then
// specificity, then id) the injection path uses.
var anchorSpecificity = map[AnchorKind]int{
	AnchorSymbol: 0,
	AnchorFile:   1,
	AnchorCommit: 2,
	AnchorClaim:  3,
	AnchorPath:   4,
}

// ValidAnchorKind reports whether k is a recognized anchor kind.
func ValidAnchorKind(k AnchorKind) bool {
	_, ok := anchorSpecificity[k]
	return ok
}

// Anchor is what a memory is about.
type Anchor struct {
	Kind AnchorKind `json:"kind"`
	Ref  string     `json:"ref"`
}

// Key is the anchor's stable identity, "kind:ref". It is also the wire form
// ParseAnchor accepts and the key the drift cache is indexed by.
func (a Anchor) Key() string { return string(a.Kind) + ":" + a.Ref }

// String renders the anchor as its Key.
func (a Anchor) String() string { return a.Key() }

// Specificity returns the anchor's sharpness rank (lower = sharper). An
// unknown kind sorts last, behind every known kind.
func (a Anchor) Specificity() int {
	if s, ok := anchorSpecificity[a.Kind]; ok {
		return s
	}
	return len(anchorSpecificity)
}

// ParseAnchor parses "kind:ref" (e.g. "symbol:parseConfig"). The ref may
// itself contain colons; only the first separates kind from ref.
func ParseAnchor(s string) (Anchor, error) {
	kind, ref, ok := strings.Cut(strings.TrimSpace(s), ":")
	if !ok {
		return Anchor{}, fmt.Errorf("anchor %q is not in kind:ref form (e.g. symbol:parseConfig)", s)
	}
	a := Anchor{Kind: AnchorKind(strings.TrimSpace(kind)), Ref: strings.TrimSpace(ref)}
	if !ValidAnchorKind(a.Kind) {
		return Anchor{}, fmt.Errorf("unknown anchor kind %q (want %s)", kind, strings.Join(anchorKindNames(), " | "))
	}
	if a.Ref == "" {
		return Anchor{}, fmt.Errorf("anchor %q has an empty ref", s)
	}
	return a, nil
}

func anchorKindNames() []string {
	names := make([]string, 0, len(anchorSpecificity))
	for k := range anchorSpecificity {
		names = append(names, string(k))
	}
	sort.Strings(names)
	return names
}

// LooseAnchorWarning is the review note attached to a path anchor. A memory
// anchored to "**/*.go" never drifts usefully, which makes it
// indistinguishable from an unanchored note (§2.1). It is a warning, not a
// refusal: a deliberately broad anchor is sometimes right, but it must be
// visible in review.
const LooseAnchorWarning = "path anchors are the loosest kind and rarely drift usefully — prefer a symbol or file anchor"

// EvidenceKind is the closed, STRUCTURAL vocabulary of what makes a memory
// credible. The kind — never the caller's claimed tier — determines the
// evidence tier, and therefore the confidence ceiling (law 14).
type EvidenceKind string

const (
	// EvidenceMachineCheck: a machine check (a test, a gate, an analyzer
	// run) enforces the claim. → verified.
	EvidenceMachineCheck EvidenceKind = "machine-check"
	// EvidenceIndex: derived from the index — a real call edge, a real
	// type, a real symbol. → pre-scan.
	EvidenceIndex EvidenceKind = "index"
	// EvidenceDoc: an ADR, a design doc or a comment says so. → doc-claimed.
	EvidenceDoc EvidenceKind = "doc"
	// EvidenceHuman: a person asserted it. → sme-stated.
	EvidenceHuman EvidenceKind = "human"
	// EvidenceModel: a model inferred it. → assumed, the floor.
	EvidenceModel EvidenceKind = "model"
)

// evidenceTier maps each structural evidence kind to the tier it can support.
// This map IS the structural derivation law 14 demands: nothing else in this
// package may promote a memory above what its evidence kind allows.
var evidenceTier = map[EvidenceKind]core.EvidenceTier{
	EvidenceMachineCheck: core.TierVerified,
	EvidenceIndex:        core.TierPreScan,
	EvidenceDoc:          core.TierDocClaimed,
	EvidenceHuman:        core.TierSMEStated,
	EvidenceModel:        core.TierAssumed,
}

// ValidEvidenceKind reports whether k is a recognized evidence kind.
func ValidEvidenceKind(k EvidenceKind) bool {
	_, ok := evidenceTier[k]
	return ok
}

// TierForEvidenceKind returns the tier a single piece of evidence of kind k
// can support. ok is false for an unrecognized kind — callers must refuse
// rather than fall back to a tier, because a silent fallback is exactly the
// "trust the caller" failure law 14 exists to prevent.
func TierForEvidenceKind(k EvidenceKind) (core.EvidenceTier, bool) {
	t, ok := evidenceTier[k]
	return t, ok
}

func evidenceKindNames() []string {
	names := make([]string, 0, len(evidenceTier))
	for k := range evidenceTier {
		names = append(names, string(k))
	}
	sort.Strings(names)
	return names
}

// Evidence is one structural reason to believe a memory.
type Evidence struct {
	Kind EvidenceKind `json:"kind"`
	// Ref names what to look at: a test name, a symbol key, a doc path, a
	// person, a model id. Required — evidence you cannot go and check is
	// not evidence.
	Ref string `json:"ref"`
}

// Key is the evidence's stable identity, "kind:ref".
func (e Evidence) Key() string { return string(e.Kind) + ":" + e.Ref }

// ParseEvidence parses "kind:ref" (e.g. "machine-check:TestFoo").
func ParseEvidence(s string) (Evidence, error) {
	kind, ref, ok := strings.Cut(strings.TrimSpace(s), ":")
	if !ok {
		return Evidence{}, fmt.Errorf("evidence %q is not in kind:ref form (e.g. index:parseConfig)", s)
	}
	ev := Evidence{Kind: EvidenceKind(strings.TrimSpace(kind)), Ref: strings.TrimSpace(ref)}
	if !ValidEvidenceKind(ev.Kind) {
		return Evidence{}, fmt.Errorf("unknown evidence kind %q (want %s)", kind, strings.Join(evidenceKindNames(), " | "))
	}
	if ev.Ref == "" {
		return Evidence{}, fmt.Errorf("evidence %q has an empty ref", s)
	}
	return ev, nil
}

// ActorKind is who (or what) authored or confirmed a memory. It is the
// structural input to re-confirmation tiering: a model confirming a memory
// can never leave it as trustworthy as a human confirming the same memory,
// and the caller never gets to state that (§3.2).
type ActorKind string

const (
	// ActorMachine: a deterministic check re-ran and still passes.
	ActorMachine ActorKind = "machine"
	// ActorHuman: a person looked and said it is still true.
	ActorHuman ActorKind = "human"
	// ActorModel: a model read the new content and said it is still true.
	ActorModel ActorKind = "model"
)

// actorEvidence maps an actor to the evidence its confirmation constitutes.
// This is the whole of the "model-confirmed lands lower than human-confirmed"
// rule — there is no second place to change it.
var actorEvidence = map[ActorKind]EvidenceKind{
	ActorMachine: EvidenceMachineCheck,
	ActorHuman:   EvidenceHuman,
	ActorModel:   EvidenceModel,
}

// ValidActorKind reports whether k is a recognized actor kind.
func ValidActorKind(k ActorKind) bool {
	_, ok := actorEvidence[k]
	return ok
}

// EvidenceForActor returns the evidence kind an actor's confirmation
// constitutes. ok is false for an unrecognized actor kind.
func EvidenceForActor(k ActorKind) (EvidenceKind, bool) {
	e, ok := actorEvidence[k]
	return e, ok
}

func actorKindNames() []string {
	names := make([]string, 0, len(actorEvidence))
	for k := range actorEvidence {
		names = append(names, string(k))
	}
	sort.Strings(names)
	return names
}

// Actor is a named author or confirmer.
type Actor struct {
	Kind ActorKind `json:"kind"`
	Name string    `json:"name,omitempty"`
}

// String renders the actor as "kind:name", or bare "kind" when unnamed.
func (a Actor) String() string {
	if a.Name == "" {
		return string(a.Kind)
	}
	return string(a.Kind) + ":" + a.Name
}

// ParseActor parses "kind" or "kind:name" (e.g. "human:alex", "model").
func ParseActor(s string) (Actor, error) {
	s = strings.TrimSpace(s)
	kind, name, _ := strings.Cut(s, ":")
	a := Actor{Kind: ActorKind(strings.TrimSpace(kind)), Name: strings.TrimSpace(name)}
	if !ValidActorKind(a.Kind) {
		return Actor{}, fmt.Errorf("unknown actor kind %q (want %s)", kind, strings.Join(actorKindNames(), " | "))
	}
	return a, nil
}

// ConfirmKind names what happened in one review resolution.
type ConfirmKind string

const (
	// ConfirmStillTrue: re-confirmed against current content; back to fresh.
	ConfirmStillTrue ConfirmKind = "still-true"
	// ConfirmNoLongerTrue: retired. Retained with a RetiredAt, never
	// surfaces again, still retrievable by id.
	ConfirmNoLongerTrue ConfirmKind = "no-longer-true"
	// ConfirmRewrite: the claim text was edited and re-confirmed against
	// current content.
	ConfirmRewrite ConfirmKind = "rewrite"
	// ConfirmWritten: the memory's original write. Recorded so the history
	// is complete rather than starting at the first review.
	ConfirmWritten ConfirmKind = "written"
)

// Confirmation is one append-only entry in a memory's re-confirmation log.
// It records WHO confirmed and against WHICH commit, which is what makes the
// tier consequence automatic rather than a rule someone has to remember.
type Confirmation struct {
	At     time.Time   `json:"at"`
	Kind   ConfirmKind `json:"kind"`
	By     Actor       `json:"by"`
	Commit string      `json:"commit,omitempty"`
	Note   string      `json:"note,omitempty"`
}

// Memory is one anchored, evidenced, tier-derived claim about a codebase.
type Memory struct {
	// ID is content-addressed over (service, claim, sorted anchor keys) —
	// re-adding the same memory is idempotent rather than duplicating.
	ID      string `json:"id"`
	Service string `json:"service"`
	// Claim is one sentence, present tense, falsifiable.
	Claim string `json:"claim"`
	// Anchors is non-empty and sorted by Key. A memory with no anchor never
	// reaches this struct — WriteMemory refuses it.
	Anchors []Anchor `json:"anchors"`
	// Evidence is non-empty and sorted by Key.
	Evidence []Evidence `json:"evidence"`

	// Tier is DERIVED from Evidence (strongest kind wins) and then weakened
	// by the last confirmation's authority. It is never the caller's claim.
	Tier core.EvidenceTier `json:"tier"`
	// Confidence is the 0–5 confidence after scoring.ApplyCeiling and the
	// tier ceiling have both been applied at the write path.
	Confidence int `json:"confidence"`
	// ClaimedTier / ClaimedConfidence record what the caller ASSERTED, for
	// audit. They have no effect on Tier or Confidence beyond being the
	// upper bound a cap is measured against.
	ClaimedTier       core.EvidenceTier `json:"claimed_tier,omitempty"`
	ClaimedConfidence int               `json:"claimed_confidence,omitempty"`
	// Capped is true when the derivation lowered the caller's claim.
	Capped bool `json:"capped"`
	// CapReason states, in one sentence, what was capped and why. Empty
	// exactly when Capped is false.
	CapReason string `json:"cap_reason,omitempty"`

	// AnchorHashes records each anchor's content hash AS OF the last
	// confirmation, keyed by Anchor.Key(). Read-time drift is a comparison
	// against this map — there is no sweep and nothing to schedule.
	AnchorHashes map[string]string `json:"anchor_hashes"`

	CreatedAt         time.Time `json:"created_at"`
	LastConfirmedAt   time.Time `json:"last_confirmed_at"`
	ConfirmedAtCommit string    `json:"confirmed_at_commit,omitempty"`
	// ConfirmedBy is the actor of the most recent confirmation (the author,
	// until the first review resolves it).
	ConfirmedBy Actor `json:"confirmed_by"`
	Author      Actor `json:"author"`

	// RetiredAt is non-zero once a review resolved the memory
	// "no-longer-true". A retired memory never surfaces again and is still
	// retrievable by id — "we removed the retry logic in July" is worth
	// keeping (§3).
	RetiredAt     time.Time `json:"retired_at,omitempty"`
	RetiredReason string    `json:"retired_reason,omitempty"`

	// History is the append-only confirmation log, oldest first.
	History []Confirmation `json:"history,omitempty"`

	// Warnings are non-fatal review notes recorded at write time (e.g. a
	// loose path anchor). Sorted.
	Warnings []string `json:"warnings,omitempty"`
}

// Retired reports whether the memory has been retired.
func (m Memory) Retired() bool { return !m.RetiredAt.IsZero() }

// Specificity is the sharpest anchor's specificity — the ordering term used
// after tier when surfacing several memories about the same file.
func (m Memory) Specificity() int {
	best := len(anchorSpecificity)
	for _, a := range m.Anchors {
		if s := a.Specificity(); s < best {
			best = s
		}
	}
	return best
}

// clone returns a deep copy so callers cannot mutate a stored memory in place.
func (m Memory) clone() Memory {
	out := m
	out.Anchors = append([]Anchor(nil), m.Anchors...)
	out.Evidence = append([]Evidence(nil), m.Evidence...)
	out.History = append([]Confirmation(nil), m.History...)
	out.Warnings = append([]string(nil), m.Warnings...)
	if m.AnchorHashes != nil {
		out.AnchorHashes = make(map[string]string, len(m.AnchorHashes))
		for k, v := range m.AnchorHashes {
			out.AnchorHashes[k] = v
		}
	}
	return out
}

// sortAnchors sorts anchors by Key and removes exact duplicates.
func sortAnchors(in []Anchor) []Anchor {
	out := append([]Anchor(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return dedupeBy(out, func(a Anchor) string { return a.Key() })
}

// sortEvidence sorts evidence by Key and removes exact duplicates.
func sortEvidence(in []Evidence) []Evidence {
	out := append([]Evidence(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return dedupeBy(out, func(e Evidence) string { return e.Key() })
}

func dedupeBy[T any](in []T, key func(T) string) []T {
	if len(in) == 0 {
		return nil
	}
	out := make([]T, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, v := range in {
		k := key(v)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, v)
	}
	return out
}
