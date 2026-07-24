package review

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// ClaimsLayer/ClaimsLayerVersion/ClaimsKind/ClaimsPromptVersion are review's
// findings-bridge identity (C2-1, FT-7's pre-armed trigger — docs/plans/
// review-verb.md, docs/plans/findings-tracer.md). Review findings are
// persisted on the SAME "findings" layer/kind vocabulary
// schema/semantic/layers/findings/layer.cue already defines. Mirrored here
// as literal constants rather than importing internal/semantic: review is a
// leaf internal/analysis package (imported BY internal/mcp and cmd/fathom,
// never the reverse), and internal/semantic must stay free of any
// internal/analysis dependency. TestClaimsVocabularyMatchesSemanticLayer
// (review_test.go, external test package) cross-checks the two never drift
// apart.
const (
	ClaimsLayer        = "findings"
	ClaimsLayerVersion = "v1"
	ClaimsKind         = "finding"
	// ClaimsPromptVersion is the "versioned prompt identity" every compiled
	// claim carries (core.SemanticClaim.PromptVersion). Review findings are
	// a DETERMINISTIC bridge (FT-7: "tier verified end-to-end"), not an LLM
	// compile — this names the deterministic pass itself.
	ClaimsPromptVersion = "review-verb/v1"
	// ClaimsTierProfile is the tier vocabulary review findings' Tier
	// belongs to (core.SemanticClaim.TierProfile) — the shipped v1
	// vocabulary, matching every other compiled claim in the store.
	ClaimsTierProfile = "engineering"
)

// AttachClaims converts v's findings into content-addressed SemanticClaims
// plus their scope (and, when a citation exists, cites) links — the write
// side of the durable-findings guarantee (C2-1). Every Finding's ClaimID is
// recorded in place as a side effect; callers must call this BEFORE
// ApplyClaimStatus, which matches by ClaimID.
//
// ClaimID is content-addressed (core.ClaimID: sha256 of
// layer+kind+subject+reproKey), so an UNCHANGED finding re-emitted by a
// later review reproduces the IDENTICAL ID. That is the whole mechanism the
// durability guarantee rests on: the store's own PutClaims upsert
// (internal/store — "status = CASE WHEN ... = 'refuted' THEN ... ELSE
// excluded.status END") keeps an already-refuted claim refuted no matter
// how many times this function re-derives the same content-addressed row —
// no special-casing needed here or in the caller.
//
// now supplies CompiledAt (an injected clock — this package never calls
// time.Now(), per repo convention). watermark is the service's corpus
// watermark at review time; "" is valid (best-effort "as of").
func AttachClaims(v *Verdict, now func() time.Time, watermark string) ([]core.SemanticClaim, []core.ClaimLink) {
	compiledAt := now().UTC()
	var claims []core.SemanticClaim
	var links []core.ClaimLink

	for di := range v.Dimensions {
		d := &v.Dimensions[di]
		for fi := range d.Findings {
			f := &d.Findings[fi]
			subject := d.Name + "#" + findingSubjectKey(*f)
			reproKey := findingReproKey(*f)
			id := core.ClaimID(ClaimsLayer, ClaimsKind, subject, reproKey)
			f.ClaimID = id

			scope := f.File
			if scope == "" {
				scope = "**"
			}
			claims = append(claims, core.SemanticClaim{
				ClaimID:             id,
				ServiceID:           v.Service,
				Layer:               ClaimsLayer,
				LayerVersion:        ClaimsLayerVersion,
				Kind:                ClaimsKind,
				Subject:             subject,
				Rule:                f.Message,
				Scope:               []string{scope},
				Tier:                core.TierVerified,
				TierProfile:         ClaimsTierProfile,
				Status:              core.ClaimFresh,
				PromptVersion:       ClaimsPromptVersion,
				ReproKey:            reproKey,
				InputDigest:         reproKey,
				CompiledAtWatermark: watermark,
				CompiledAt:          compiledAt,
			})
			if f.Citation != "" {
				links = append(links, core.ClaimLink{ClaimID: id, Rel: core.RelCites, TargetRef: f.Citation})
			}
			links = append(links, core.ClaimLink{ClaimID: id, Rel: core.RelScopes, TargetRef: scope})
		}
	}

	sort.Slice(claims, func(i, j int) bool { return claims[i].ClaimID < claims[j].ClaimID })
	sort.Slice(links, func(i, j int) bool {
		if links[i].ClaimID != links[j].ClaimID {
			return links[i].ClaimID < links[j].ClaimID
		}
		return links[i].Rel < links[j].Rel
	})
	return claims, links
}

// findingSubjectKey picks the most specific stable identifier available for
// a finding's claim Subject.
func findingSubjectKey(f Finding) string {
	if f.Symbol != "" {
		return f.Symbol
	}
	if f.File != "" {
		return f.File
	}
	return f.Kind
}

// findingReproKey hashes a finding's content — the ClaimID changes if and
// only if the finding's substance changes, so a genuinely different finding
// sharing the same subject never inherits a stale refutation.
func findingReproKey(f Finding) string {
	h := sha256.Sum256([]byte(f.Kind + "\x00" + f.File + "\x00" + f.Citation + "\x00" + f.Message))
	return hex.EncodeToString(h[:])
}

// ApplyClaimStatus overlays previously-persisted claim statuses onto v's
// findings — the read side of the durable-findings guarantee (C2-1). Call
// AttachClaims FIRST: this matches existing by ClaimID.
//
// For every finding whose ClaimID matches an existing claim in a status the
// findings layer's serving rules would never re-serve
// (core.ClaimStatus.Served() == false — refuted or superseded), the finding
// is labeled with that Status and excluded from its dimension's Count (the
// active-issue count) — but stays IN Findings, visible for audit: "carries
// refuted status", per the C2-1 test contract, never silently dropped.
func ApplyClaimStatus(v *Verdict, existing []core.SemanticClaim) {
	byID := make(map[string]core.ClaimStatus, len(existing))
	for _, c := range existing {
		byID[c.ClaimID] = c.Status
	}
	for di := range v.Dimensions {
		d := &v.Dimensions[di]
		active := 0
		for fi := range d.Findings {
			f := &d.Findings[fi]
			if st, ok := byID[f.ClaimID]; ok && !st.Served() {
				f.Status = string(st)
				continue
			}
			active++
		}
		d.Count = active
	}
}
