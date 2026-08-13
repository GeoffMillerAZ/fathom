package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// write.go — the refusals and the ceiling. This is where anchored memory
// either earns its keep or becomes a wiki with worse search (§6).

// RefusalReason is the closed vocabulary of write refusals. Every refusal
// names one of these and carries a sentence a human can act on.
type RefusalReason string

const (
	// RefusalNoAnchor: the memory is not about anything fathom can watch.
	// THE refusal — see §2.1.
	RefusalNoAnchor RefusalReason = "no-anchor"
	// RefusalNoEvidence: nothing makes the claim credible.
	RefusalNoEvidence RefusalReason = "no-evidence"
	// RefusalNoClaim: the claim text is empty.
	RefusalNoClaim RefusalReason = "no-claim"
	// RefusalNoService: no service to anchor within.
	RefusalNoService RefusalReason = "no-service"
	// RefusalBadAnchor: an anchor has an unknown kind or an empty ref.
	RefusalBadAnchor RefusalReason = "bad-anchor"
	// RefusalBadEvidence: evidence has an unknown kind or an empty ref.
	RefusalBadEvidence RefusalReason = "bad-evidence"
	// RefusalAnchorUnresolved: the anchor does not resolve in the index, so
	// the memory could never be evaluated for drift — it would be an
	// unanchored note wearing an anchor's clothes.
	RefusalAnchorUnresolved RefusalReason = "anchor-unresolved"
	// RefusalBadActor: the author's actor kind is unknown.
	RefusalBadActor RefusalReason = "bad-actor"
)

// UnanchoredRemedy is the sentence a caller sees when it tries to store a
// preference. It names where the thing actually belongs, because a refusal
// without a destination just gets worked around.
const UnanchoredRemedy = "this is a preference, not a memory about this codebase; put it in CLAUDE.md"

// RefusedError is a write refusal. It is an error, never a low-tier write:
// storing an unanchored claim quietly at tier assumed is the failure mode
// this whole design exists to prevent.
type RefusedError struct {
	Reason RefusalReason
	Detail string
}

func (e *RefusedError) Error() string {
	return "memory refused (" + string(e.Reason) + "): " + e.Detail
}

// Refused reports whether err is a write refusal, and which one.
func Refused(err error) (*RefusedError, bool) {
	var re *RefusedError
	if errors.As(err, &re) {
		return re, true
	}
	return nil, false
}

func refuse(reason RefusalReason, format string, args ...any) error {
	return &RefusedError{Reason: reason, Detail: fmt.Sprintf(format, args...)}
}

// Draft is a proposed memory as the CALLER states it. Everything in it is
// input; nothing in it is authority. In particular ClaimedTier and
// ClaimedConfidence are recorded for audit and then bounded by what the
// evidence structurally supports.
type Draft struct {
	Service  string
	Claim    string
	Anchors  []Anchor
	Evidence []Evidence
	Author   Actor

	// ClaimedTier is what the caller SAYS the tier is. Leave empty to make
	// no claim; state it and a lie will be recorded as a cap, not honoured.
	ClaimedTier core.EvidenceTier
	// ClaimedConfidence is what the caller SAYS the confidence is (0 = no
	// claim, derive it).
	ClaimedConfidence int

	// Commit is the corpus watermark the memory is being written against.
	// Optional; recorded so re-confirmation can say "confirmed against
	// which commit" from the first write onward.
	Commit string
}

// MemoryID is the content-addressed identity of a memory: sha256 over
// service, claim and the sorted anchor keys. Two writes of the same claim
// about the same anchors are the same memory, so re-adding is idempotent
// rather than duplicating.
func MemoryID(service, claim string, anchors []Anchor) string {
	keys := make([]string, 0, len(anchors))
	for _, a := range anchors {
		keys = append(keys, a.Key())
	}
	sort.Strings(keys)
	h := sha256.Sum256([]byte(service + "\x00" + claim + "\x00" + strings.Join(keys, "\x00")))
	return hex.EncodeToString(h[:])[:16]
}

// WriteMemory validates, tiers and stores one memory.
//
// The order is deliberate: refuse first, derive second, cap third, store
// last. A caller can never reach the store with an unanchored claim, and can
// never reach it with a tier its evidence does not support.
//
//   - No anchor → *RefusedError{RefusalNoAnchor} carrying UnanchoredRemedy.
//     The memory is NOT stored at a low tier; it is not stored at all.
//   - Every anchor is probed against the index, and an anchor that does not
//     resolve is refused: an anchor fathom cannot watch cannot expire.
//   - The tier is DERIVED from the evidence kinds (DeriveTier) and the
//     confidence is bounded by capConfidence, which routes through
//     agentic-kit's scoring.ApplyCeiling. The caller's claim is retained in
//     ClaimedTier/ClaimedConfidence and, when it exceeded the derivation,
//     Capped is set and CapReason states what happened.
//
// now must be non-nil: determinism is a contract, and this package never
// calls time.Now() on a caller's behalf.
func WriteMemory(ctx context.Context, ms Store, idx Index, now func() time.Time, d Draft) (Memory, error) {
	if now == nil {
		return Memory{}, errors.New("memory: WriteMemory requires an injected clock (now func() time.Time)")
	}
	if ms == nil {
		return Memory{}, errors.New("memory: WriteMemory requires a store")
	}
	if idx == nil {
		return Memory{}, errors.New("memory: WriteMemory requires an index to resolve anchors against")
	}

	service := strings.TrimSpace(d.Service)
	claim := strings.TrimSpace(d.Claim)
	if service == "" {
		return Memory{}, refuse(RefusalNoService, "a memory must name the service it is about")
	}
	if claim == "" {
		return Memory{}, refuse(RefusalNoClaim, "a memory must state one falsifiable sentence")
	}

	// The refusal that is the feature.
	if len(d.Anchors) == 0 {
		return Memory{}, refuse(RefusalNoAnchor, "%s", UnanchoredRemedy)
	}

	anchors := sortAnchors(d.Anchors)
	for _, a := range anchors {
		if !ValidAnchorKind(a.Kind) {
			return Memory{}, refuse(RefusalBadAnchor, "unknown anchor kind %q (want %s)", a.Kind, strings.Join(anchorKindNames(), " | "))
		}
		if strings.TrimSpace(a.Ref) == "" {
			return Memory{}, refuse(RefusalBadAnchor, "anchor of kind %q has an empty ref", a.Kind)
		}
	}

	if len(d.Evidence) == 0 {
		return Memory{}, refuse(RefusalNoEvidence, "a memory must carry at least one piece of evidence (want %s)", strings.Join(evidenceKindNames(), " | "))
	}
	evidence := sortEvidence(d.Evidence)
	for _, e := range evidence {
		if !ValidEvidenceKind(e.Kind) {
			return Memory{}, refuse(RefusalBadEvidence, "unknown evidence kind %q (want %s)", e.Kind, strings.Join(evidenceKindNames(), " | "))
		}
		if strings.TrimSpace(e.Ref) == "" {
			return Memory{}, refuse(RefusalBadEvidence, "evidence of kind %q has an empty ref — evidence you cannot go and check is not evidence", e.Kind)
		}
	}

	author := d.Author
	if author.Kind == "" {
		author.Kind = ActorHuman
	}
	if !ValidActorKind(author.Kind) {
		return Memory{}, refuse(RefusalBadActor, "unknown author kind %q (want %s)", author.Kind, strings.Join(actorKindNames(), " | "))
	}

	// Resolve every anchor NOW, both to refuse unresolvable ones and to
	// record the baseline hashes read-time drift compares against.
	hashes := make(map[string]string, len(anchors))
	var warnings []string
	for _, a := range anchors {
		p, err := idx.Probe(ctx, service, a)
		if err != nil {
			return Memory{}, fmt.Errorf("probe anchor %s: %w", a.Key(), err)
		}
		if len(p.Ambiguous) > 0 {
			// W1 (review): binding a bare name that matches several
			// packages would record a baseline for whichever candidate an
			// unordered store fold happened to surface — two stores with
			// identical content resolved the same anchor differently, and
			// a per-file re-ingest could flip the winner and fabricate
			// drift naming a file the memory was never about. Refuse, and
			// name the qualified keys the author actually meant one of.
			return Memory{}, refuse(RefusalAnchorUnresolved,
				"anchor %s is ambiguous — %s", a.Key(), p.Covers)
		}
		if !p.Exists {
			return Memory{}, refuse(RefusalAnchorUnresolved,
				"anchor %s does not resolve in the index (%s) — an anchor fathom cannot watch can never expire",
				a.Key(), p.Covers)
		}
		hashes[a.Key()] = p.Hash
		if a.Kind == AnchorPath {
			warnings = append(warnings, LooseAnchorWarning)
		}
	}
	sort.Strings(warnings)
	warnings = dedupeBy(warnings, func(s string) string { return s })

	derivedTier, ok := DeriveTier(evidence)
	if !ok {
		return Memory{}, refuse(RefusalBadEvidence, "evidence does not support any tier")
	}

	claimedConfidence := d.ClaimedConfidence
	stated := claimedConfidence > 0 || d.ClaimedTier != ""
	if claimedConfidence <= 0 {
		claimedConfidence = defaultConfidence(derivedTier, evidence)
	}
	confidence := capConfidence(claimedConfidence, derivedTier)

	capped, reason := capNote(stated, d.ClaimedTier, derivedTier, d.ClaimedConfidence, confidence, evidence)

	at := now().UTC()
	m := Memory{
		ID:                MemoryID(service, claim, anchors),
		Service:           service,
		Claim:             claim,
		Anchors:           anchors,
		Evidence:          evidence,
		Tier:              derivedTier,
		Confidence:        confidence,
		ClaimedTier:       d.ClaimedTier,
		ClaimedConfidence: d.ClaimedConfidence,
		Capped:            capped,
		CapReason:         reason,
		AnchorHashes:      hashes,
		CreatedAt:         at,
		LastConfirmedAt:   at,
		ConfirmedAtCommit: d.Commit,
		ConfirmedBy:       author,
		Author:            author,
		Warnings:          warnings,
		History: []Confirmation{{
			At:     at,
			Kind:   ConfirmWritten,
			By:     author,
			Commit: d.Commit,
		}},
	}

	if err := ms.Put(ctx, m); err != nil {
		return Memory{}, err
	}
	return m, nil
}

// capNote states what the derivation did to the caller's claim. It returns
// (false, "") exactly when the caller claimed nothing beyond what the
// evidence supports.
func capNote(stated bool, claimedTier, derivedTier core.EvidenceTier, claimedConfidence, confidence int, ev []Evidence) (bool, string) {
	if !stated {
		return false, ""
	}
	var notes []string
	if claimedTier != "" && claimedTier != derivedTier && ladderIndex(claimedTier) < ladderIndex(derivedTier) {
		notes = append(notes, fmt.Sprintf("claimed tier %q exceeds what the evidence supports (%s) — stored at %q",
			claimedTier, describeEvidence(ev), derivedTier))
	}
	if claimedConfidence > 0 && confidence < claimedConfidence {
		notes = append(notes, fmt.Sprintf("confidence %d capped to %d by the %q ceiling (scoring.ApplyCeiling)",
			claimedConfidence, confidence, derivedTier))
	}
	if len(notes) == 0 {
		return false, ""
	}
	return true, strings.Join(notes, "; ")
}

// describeEvidence renders the evidence kinds present, sorted, for a cap
// reason a human can argue with.
func describeEvidence(ev []Evidence) string {
	seen := map[string]bool{}
	var kinds []string
	for _, e := range ev {
		if seen[string(e.Kind)] {
			continue
		}
		seen[string(e.Kind)] = true
		kinds = append(kinds, string(e.Kind))
	}
	sort.Strings(kinds)
	if len(kinds) == 0 {
		return "no evidence"
	}
	return strconv.Itoa(len(kinds)) + " kind(s): " + strings.Join(kinds, ", ")
}
