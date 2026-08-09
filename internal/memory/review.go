package memory

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// review.go — the review queue and the three resolutions (§3.2).
//
// The queue must be cheap to work or it will not be worked, so a resolution
// is one call with three possible answers: still true, no longer true,
// rewrite. Each records WHO resolved it and against WHICH commit, and the
// tier consequence of "who" is automatic — nobody has to remember the rule.

// Resolution is a review verdict on a suspect (or orphaned) memory.
type Resolution string

const (
	// ResolveStillTrue re-confirms the memory against current content: back
	// to fresh, LastConfirmedAt updated, anchors re-baselined.
	ResolveStillTrue Resolution = "still-true"
	// ResolveNoLongerTrue retires the memory. Retained with a RetiredAt,
	// never surfaces again, still retrievable by id.
	ResolveNoLongerTrue Resolution = "no-longer-true"
	// ResolveRewrite edits the claim text and re-confirms against current
	// content. The rewritten memory keeps its id: it is the same memory
	// about the same anchors, said better.
	ResolveRewrite Resolution = "rewrite"
)

// ValidResolution reports whether r is a recognized resolution.
func ValidResolution(r Resolution) bool {
	switch r {
	case ResolveStillTrue, ResolveNoLongerTrue, ResolveRewrite:
		return true
	}
	return false
}

// ReviewItem is one entry in the review queue: the memory, its evaluation,
// and the reason it is queued.
type ReviewItem struct {
	Memory     Memory     `json:"memory"`
	Evaluation Evaluation `json:"evaluation"`
	// Reason is a one-line statement of why this is in the queue.
	Reason string `json:"reason"`
}

// ReviewQueue returns the memories that need a human (or a model) to decide:
// every suspect and orphaned memory for the service, in the deterministic
// surfacing order. Fresh and retired memories are not queued.
func ReviewQueue(ctx context.Context, ms Store, idx Index, cache *DriftCache, now func() time.Time, service string) ([]ReviewItem, error) {
	all, err := ms.List(ctx, service)
	if err != nil {
		return nil, err
	}
	evs, err := EvaluateAll(ctx, idx, cache, now, all)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]Memory, len(all))
	for _, m := range all {
		byID[m.ID] = m
	}
	out := make([]ReviewItem, 0, len(evs))
	for _, e := range evs {
		switch e.State {
		case StateSuspect, StateOrphaned:
		default:
			continue
		}
		reason := "an anchor changed since this was confirmed"
		if e.State == StateOrphaned {
			reason = "an anchor no longer exists; this memory has stopped surfacing"
		}
		out = append(out, ReviewItem{Memory: byID[e.ID], Evaluation: e, Reason: reason})
	}
	return out, nil
}

// Resolve applies a review verdict to one memory and stores the result.
//
// Re-confirmation tiering, the part callers do NOT get to state: the
// confirming actor's authority is itself evidence, and the memory's tier
// afterwards is the WEAKER of what its original evidence supports and what
// the confirmer's authority supports. A model saying "still true" about
// changed code is an inference (assumed); a human saying it is an expert
// statement (sme-stated); a machine check re-running is verified. So the same
// memory re-confirmed by a model lands strictly below the same memory
// re-confirmed by a human, automatically (§3.2).
//
// newClaim is required for ResolveRewrite and ignored otherwise. commit is
// the corpus watermark the confirmation is made against; note is free text.
// now must be non-nil.
func Resolve(ctx context.Context, ms Store, idx Index, now func() time.Time, id string, r Resolution, by Actor, commit, newClaim, note string) (Memory, error) {
	if now == nil {
		return Memory{}, errors.New("memory: Resolve requires an injected clock (now func() time.Time)")
	}
	if !ValidResolution(r) {
		return Memory{}, fmt.Errorf("unknown resolution %q (want %s | %s | %s)", r, ResolveStillTrue, ResolveNoLongerTrue, ResolveRewrite)
	}
	if !ValidActorKind(by.Kind) {
		return Memory{}, fmt.Errorf("unknown actor kind %q (want %s)", by.Kind, strings.Join(actorKindNames(), " | "))
	}
	m, found, err := ms.Get(ctx, id)
	if err != nil {
		return Memory{}, err
	}
	if !found {
		return Memory{}, fmt.Errorf("no memory with id %q", id)
	}
	if m.Retired() {
		return Memory{}, fmt.Errorf("memory %s was retired at %s and cannot be re-resolved", id, m.RetiredAt.UTC().Format(time.RFC3339))
	}

	at := now().UTC()

	if r == ResolveNoLongerTrue {
		m.RetiredAt = at
		m.RetiredReason = note
		if m.RetiredReason == "" {
			m.RetiredReason = "resolved no-longer-true in review"
		}
		m.History = append(m.History, Confirmation{At: at, Kind: ConfirmNoLongerTrue, By: by, Commit: commit, Note: note})
		if err := ms.Put(ctx, m); err != nil {
			return Memory{}, err
		}
		return m, nil
	}

	kind := ConfirmStillTrue
	if r == ResolveRewrite {
		newClaim = strings.TrimSpace(newClaim)
		if newClaim == "" {
			return Memory{}, refuse(RefusalNoClaim, "a rewrite must supply the new claim text")
		}
		m.Claim = newClaim
		kind = ConfirmRewrite
	}

	// Re-baseline every anchor against current content: that is what makes
	// the memory fresh again rather than permanently suspect.
	if idx == nil {
		return Memory{}, errors.New("memory: Resolve requires an index to re-baseline anchors against")
	}
	hashes := make(map[string]string, len(m.Anchors))
	for _, a := range m.Anchors {
		p, perr := idx.Probe(ctx, m.Service, a)
		if perr != nil {
			return Memory{}, fmt.Errorf("probe anchor %s: %w", a.Key(), perr)
		}
		if !p.Exists {
			return Memory{}, fmt.Errorf("anchor %s no longer exists (%s) — an orphaned memory cannot be re-confirmed, only retired or rewritten onto a live anchor", a.Key(), p.Covers)
		}
		hashes[a.Key()] = p.Hash
	}
	m.AnchorHashes = hashes

	// The confirmation is evidence. Record it, then re-derive.
	confirmKind, ok := EvidenceForActor(by.Kind)
	if !ok {
		return Memory{}, fmt.Errorf("unknown actor kind %q", by.Kind)
	}
	confirmerTier, ok := TierForEvidenceKind(confirmKind)
	if !ok {
		return Memory{}, fmt.Errorf("no tier for evidence kind %q", confirmKind)
	}
	baseTier, ok := DeriveTier(m.Evidence)
	if !ok {
		return Memory{}, fmt.Errorf("memory %s carries no tierable evidence", id)
	}

	effective := weaker(baseTier, confirmerTier)
	m.Tier = effective
	m.Confidence = capConfidence(defaultConfidence(baseTier, m.Evidence), effective)
	if effective != baseTier {
		m.Capped = true
		m.CapReason = fmt.Sprintf("re-confirmed by %s against %s: a %s confirmation supports at most %q, so the memory's %q evidence is held at %q until a stronger confirmation",
			by.String(), commitPhrase(commit), by.Kind, confirmerTier, baseTier, effective)
		// The full watermark stays on ConfirmedAtCommit and in History;
		// the sentence carries the readable form.
	} else {
		m.Capped = false
		m.CapReason = ""
	}

	m.LastConfirmedAt = at
	m.ConfirmedAtCommit = commit
	m.ConfirmedBy = by
	m.History = append(m.History, Confirmation{At: at, Kind: kind, By: by, Commit: commit, Note: note})

	if err := ms.Put(ctx, m); err != nil {
		return Memory{}, err
	}
	return m, nil
}

func commitPhrase(commit string) string {
	if commit == "" {
		return "an unnamed commit"
	}
	return ShortWatermark(commit)
}

// TierOrder is a stable, human-facing rendering of the ladder, strongest
// first. Surfaces (the CLI's help text, and later the MCP tool's schema)
// render the vocabulary from here so it is never restated by hand.
func TierOrder() []string {
	out := make([]string, 0, len(tierLadder))
	for _, t := range tierLadder {
		out = append(out, string(t))
	}
	return out
}

// EvidenceHelp lists the evidence vocabulary with the tier each kind
// supports, sorted by tier strength then kind. The pairing is the point: a
// caller choosing an evidence kind is choosing a ceiling, and the help text
// should say so rather than presenting the two as independent knobs.
func EvidenceHelp() []string {
	type row struct {
		kind EvidenceKind
		tier core.EvidenceTier
	}
	rows := make([]row, 0, len(evidenceTier))
	for k, t := range evidenceTier {
		rows = append(rows, row{k, t})
	}
	sort.Slice(rows, func(i, j int) bool {
		if a, b := ladderIndex(rows[i].tier), ladderIndex(rows[j].tier); a != b {
			return a < b
		}
		return rows[i].kind < rows[j].kind
	})
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, fmt.Sprintf("%s → %s", r.kind, r.tier))
	}
	return out
}
