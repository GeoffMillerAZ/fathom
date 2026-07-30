package review

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
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
// (review_test.go, external test package) cross-checks kind, tier, AND the
// layer manifest's oracle-tier admission so the vocabularies never drift.
const (
	ClaimsLayer        = "findings"
	ClaimsLayerVersion = "v1"
	ClaimsKind         = "finding"
	// ClaimsPromptVersion is the "versioned prompt identity" every compiled
	// claim carries (core.SemanticClaim.PromptVersion). Review findings are
	// a DETERMINISTIC bridge (FT-7: "tier verified end-to-end"), not an LLM
	// compile — this names the deterministic pass itself, and is ALSO the
	// discriminator StaleReviewClaims uses to reconcile only review-minted
	// claims, never `fathom compile --layer findings` corpus claims.
	ClaimsPromptVersion = "review-verb/v1"
	// ClaimsTierProfile is the tier vocabulary review findings' Tier
	// belongs to (core.SemanticClaim.TierProfile) — the shipped v1
	// vocabulary, matching every other compiled claim in the store.
	ClaimsTierProfile = "engineering"
)

// ClaimsTier is the evidence tier every persisted review finding carries:
// verified — these are deterministic-oracle findings (Model empty; ADR-018's
// oracle classes, admitted for exactly this case by the ADR-016 M1
// amendment). The findings layer's own manifest tier stays "sme-stated" for
// its LLM-compiled corpus; the layer.cue records this bridge's
// verified-tier admission explicitly (pinned by
// TestClaimsVocabularyMatchesSemanticLayer).
const ClaimsTier = core.TierVerified

// persistableDim reports whether a dimension's findings are persisted as
// claims. ONLY the deterministic-oracle dimensions persist:
//
//   - obligations NEVER persists: that dimension RE-SERVES already-compiled
//     claims — reporting them is its job; re-minting them as new wrapper
//     claims is store corruption (each run would wrap what the previous run
//     minted, growing without bound, and the exact-path-scoped wrappers
//     would out-rank real obligations' broader globs in ObligationsFor's
//     MaxObligations serving budget, displacing them from the edit-hook).
//   - impact NEVER persists: info-severity blast-radius enumeration is
//     context, not a finding — persisting it is noise with the same
//     budget-displacement failure mode.
func persistableDim(name string) bool {
	return name == DimArch || name == DimSpecCoverage
}

// PersistInput bundles the store-derived context AttachClaims needs to mint
// SOUND claims — sound meaning: every persisted link satisfies the substrate
// contracts the daemon's post-refresh machinery enforces (semantic.Sweep for
// cites, semantic.DriftedFindings for scope-fp), so a freshly persisted
// review claim survives a sweep with zero flips.
type PersistInput struct {
	// Now supplies CompiledAt (an injected clock — this package never calls
	// time.Now(), per repo convention).
	Now func() time.Time
	// Watermark is the service's corpus watermark at review time; "" is a
	// valid best-effort "as of".
	Watermark string
	// FileContent returns a repo-relative corpus file's CURRENT content —
	// the same view semantic.Sweep's fetch reads — used to hash cited lines
	// (core.HashLines) into each cites link's TargetHash. nil, or a file it
	// can't return, means no cites link is written for findings citing that
	// file: a cites link without a real TargetHash (or with a bare-path
	// TargetRef) reads as anchor-dead to the sweep and would flip the claim
	// contradicted — with a lying "anchor no longer resolves" event — on
	// the next daemon refresh. That is the exact defect this field closes.
	FileContent func(path string) ([]byte, bool)
	// ScopeFingerprint returns the store's CURRENT content fingerprint for
	// a repo-relative file (store.Fingerprint — the same table
	// semantic.DriftedFindings reads), recorded as the claim's scope-fp
	// drift anchor (FT-6). A finding whose scoped file has NO stored
	// fingerprint (or a nil ScopeFingerprint) is NOT persisted at all —
	// mirroring the compiler's own strictness (errDriftAnchor, compile.go):
	// a drift-tracked-layer claim without a scope-fp anchor would falsify
	// DriftedFindings' documented compile invariant.
	ScopeFingerprint func(path string) (string, bool)
}

// AttachClaims converts v's persistable findings into content-addressed
// SemanticClaims plus their links — the write side of the durable-findings
// guarantee (C2-1). Each persisted Finding's ClaimID is recorded in place as
// a side effect; callers must call this BEFORE ApplyClaimStatus, which
// matches by ClaimID. Findings in non-persistable dimensions (impact,
// obligations — see persistableDim), findings with no File, and findings
// whose scoped file has no stored fingerprint keep ClaimID == "" and are
// never persisted.
//
// ClaimID is content-addressed (core.ClaimID: sha256 of
// layer+kind+subject+reproKey), so an UNCHANGED finding re-emitted by a
// later review reproduces the IDENTICAL ID. That is the whole mechanism the
// durability guarantee rests on: the store's own PutClaims upsert
// (internal/store — "status = CASE WHEN ... = 'refuted' THEN ... ELSE
// excluded.status END") keeps an already-refuted claim refuted no matter
// how many times this function re-derives the same content-addressed row —
// no special-casing needed here or in the caller. It is also what makes a
// persisting re-run idempotent: identical findings upsert identical rows
// (PutClaims replaces each claim's link set wholesale), never new ones.
//
// Links per persisted claim:
//   - scopes: the finding's file (exact path).
//   - scope-fp: the file's current store fingerprint — FT-6's remediation-
//     drift anchor, so DriftedFindings can flag the claim when the scoped
//     code changes after persistence.
//   - cites: ONLY when the finding's citation is a parseable
//     "path#Lstart-Lend" AND the cited lines hash successfully from
//     in.FileContent — with the real line hash as TargetHash. NEVER a
//     bare-path or hashless cites link (semantic.Sweep would flip the claim
//     contradicted). A finding without a hashable citation persists with no
//     cites link at all: the sweep leaves zero-cites claims fresh.
func AttachClaims(v *Verdict, in PersistInput) ([]core.SemanticClaim, []core.ClaimLink) {
	compiledAt := in.Now().UTC()
	var claims []core.SemanticClaim
	var links []core.ClaimLink

	for di := range v.Dimensions {
		d := &v.Dimensions[di]
		if !persistableDim(d.Name) {
			continue
		}
		for fi := range d.Findings {
			f := &d.Findings[fi]
			if f.File == "" {
				continue // no scope anchor: not persistable
			}
			fp := ""
			if in.ScopeFingerprint != nil {
				if h, ok := in.ScopeFingerprint(f.File); ok {
					fp = h
				}
			}
			if fp == "" {
				continue // no drift anchor available: not persistable (see PersistInput.ScopeFingerprint)
			}

			subject := d.Name + "#" + findingSubjectKey(*f)
			reproKey := findingReproKey(*f)
			id := core.ClaimID(ClaimsLayer, ClaimsKind, subject, reproKey)
			f.ClaimID = id

			claims = append(claims, core.SemanticClaim{
				ClaimID:             id,
				ServiceID:           v.Service,
				Layer:               ClaimsLayer,
				LayerVersion:        ClaimsLayerVersion,
				Kind:                ClaimsKind,
				Subject:             subject,
				Rule:                f.Message,
				Scope:               []string{f.File},
				Tier:                ClaimsTier,
				TierProfile:         ClaimsTierProfile,
				Status:              core.ClaimFresh,
				PromptVersion:       ClaimsPromptVersion,
				ReproKey:            reproKey,
				InputDigest:         reproKey,
				CompiledAtWatermark: in.Watermark,
				CompiledAt:          compiledAt,
				// Class (docs/design/findings-as-evidence.md §3.2): the
				// dimension already assigned this deterministically
				// (runArch/runSpecCoverage, engine.go) — carried straight
				// through, never re-derived here.
				Class: f.Class,
			})
			if hash, text, ok := citeHash(f.Citation, in.FileContent); ok {
				links = append(links, core.ClaimLink{ClaimID: id, Rel: core.RelCites, TargetRef: f.Citation, TargetHash: hash, AnchorText: text})
			}
			links = append(links, core.ClaimLink{ClaimID: id, Rel: core.RelScopes, TargetRef: f.File})
			links = append(links, core.ClaimLink{ClaimID: id, Rel: core.RelScopeFP, TargetRef: f.File, TargetHash: fp})
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

// citeHash resolves a finding citation to its current cited-lines hash and
// literal text (T5-1/O13: AnchorText lets the sweep re-anchor a drifted
// finding citation, not just detect the drift). ok is false for a bare-path
// or malformed citation, unavailable content, or an out-of-range line —
// every case where a persisted cites link would lie to the sweep.
func citeHash(citation string, fileContent func(path string) ([]byte, bool)) (hash, text string, ok bool) {
	if citation == "" || fileContent == nil {
		return "", "", false
	}
	path, start, end, ok := core.ParseCiteRef(citation)
	if !ok {
		return "", "", false
	}
	data, found := fileContent(path)
	if !found {
		return "", "", false
	}
	hash, ok = core.HashLines(data, start, end)
	if !ok {
		return "", "", false
	}
	text, ok = core.LinesText(data, start, end)
	if !ok {
		return "", "", false
	}
	return hash, text, true
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

// ApplyClaimStatus overlays previously-persisted REFUTED claim statuses onto
// v's findings — the read side of the durable-findings guarantee (C2-1).
// Call AttachClaims FIRST: this matches existing by ClaimID.
//
// Every finding whose ClaimID matches an existing refuted claim
// (core.ClaimRefuted — the one terminal human verdict) is labeled
// status:"refuted" and excluded from its dimension's Count (the active-issue
// count) — but stays IN Findings, visible for audit: "carries refuted
// status", never silently dropped. ONLY refuted overlays: a superseded
// existing claim whose finding re-appears in this run is a REGRESSION — the
// store's PutClaims upsert resurrects it to fresh (only refuted is
// upsert-durable), so labeling it superseded here would contradict the
// state the store is about to hold.
//
// Findings that carry a serving-status label from their source claim (the
// obligations dimension's stale/contradicted, set by runObligations and
// never persisted here) are left untouched and stay in the active count —
// served-with-label, per internal/semantic/serve.go's discipline.
func ApplyClaimStatus(v *Verdict, existing []core.SemanticClaim) {
	refuted := make(map[string]bool, len(existing))
	for _, c := range existing {
		if c.Status == core.ClaimRefuted {
			refuted[c.ClaimID] = true
		}
	}
	for di := range v.Dimensions {
		d := &v.Dimensions[di]
		active := 0
		for fi := range d.Findings {
			f := &d.Findings[fi]
			if f.ClaimID != "" && refuted[f.ClaimID] {
				f.Status = string(core.ClaimRefuted)
				continue
			}
			active++
		}
		d.Count = active
	}
}

// StaleReviewClaims returns the ClaimIDs of previously-persisted review
// claims this run supersedes — the reconcile that keeps the claims store
// from rotting (C2-1): under the content-addressed scheme an unchanged
// finding re-derives its identical ID, so a review-minted claim this run
// did NOT re-emit means the underlying finding no longer exists — fixed.
//
// A claim is superseded only when ALL of:
//   - it is review-minted (Layer + PromptVersion match this bridge's own —
//     `fathom compile --layer findings` corpus claims are never touched);
//   - it is currently served (fresh/stale/contradicted; refuted and
//     superseded are terminal and untouched — refutation durability);
//   - its subject's dimension RAN this time (Checked true): absent because
//     unchecked is not absent because fixed;
//   - its scope file is in this run's changed-file set F (a run only
//     vouches for the files it reviewed — an arch finding on a file outside
//     F keeps re-emitting while it still violates, and reconciles on a
//     later run whose F covers its file);
//   - its ClaimID is not among this run's emitted findings.
//
// Callers flip each returned ID to core.ClaimSuperseded via the store.
func StaleReviewClaims(v Verdict, existing []core.SemanticClaim) []string {
	emitted := map[string]bool{}
	checkedDim := map[string]bool{}
	for _, d := range v.Dimensions {
		if d.Checked && persistableDim(d.Name) {
			checkedDim[d.Name] = true
		}
		for _, f := range d.Findings {
			if f.ClaimID != "" {
				emitted[f.ClaimID] = true
			}
		}
	}
	changed := toSet(v.ChangedFiles)

	var stale []string
	for _, cl := range existing {
		if cl.Layer != ClaimsLayer || cl.PromptVersion != ClaimsPromptVersion {
			continue
		}
		if !cl.Status.Served() {
			continue
		}
		dim, _, ok := strings.Cut(cl.Subject, "#")
		if !ok || !checkedDim[dim] {
			continue
		}
		inF := false
		for _, s := range cl.Scope {
			if changed[s] {
				inF = true
				break
			}
		}
		if !inF || emitted[cl.ClaimID] {
			continue
		}
		stale = append(stale, cl.ClaimID)
	}
	sort.Strings(stale)
	return stale
}

// ClassifyResolution reports which of the two review-reconcile resolution
// outcomes (docs/design/findings-as-evidence.md §3.4, §6 item 4) explains
// why a StaleReviewClaims-superseded claim's finding is no longer emitted:
//
//   - core.ResolutionCodeChange: every one of the claim's scoped files
//     still exists in the corpus (fileExists reports true) — the
//     underlying code changed IN PLACE and the issue no longer reproduces.
//   - core.ResolutionScopeDisappearance: at least one scoped file no
//     longer exists at all — the finding's subject is gone, not fixed.
//
// A THIRD case, refutation, is deliberately NOT this function's concern —
// it is a distinct human-verdict path (core.ClaimRefuted via
// pkg/client.ClaimFeedback) that never flows through StaleReviewClaims at
// all; see core.ClaimEvent.ResolutionKind's doc comment for the full
// three-case account. fileExists nil is treated as "nothing exists"
// (ResolutionScopeDisappearance) — the conservative, honest default when a
// caller cannot check.
func ClassifyResolution(scope []string, fileExists func(path string) bool) string {
	if fileExists == nil {
		return core.ResolutionScopeDisappearance
	}
	for _, s := range scope {
		if !fileExists(s) {
			return core.ResolutionScopeDisappearance
		}
	}
	return core.ResolutionCodeChange
}

// BuildPersist is the one shared write-path composition both callers (the
// review_diff MCP tool and the `fathom review` CLI) run, so their persist
// semantics can never drift: AttachClaims (mint IDs) → ApplyClaimStatus
// (label refuted findings, fix active counts) → StaleReviewClaims
// (reconcile fixed findings). The caller then persists claims/links via the
// store's PutClaims and flips each supersede ID to core.ClaimSuperseded.
// Pure beyond the injected PersistInput callbacks. A report-only
// (no-persist) caller runs BuildPersist too — for the refuted-overlay
// labels — and simply discards claims/links/supersede without writing.
func BuildPersist(v *Verdict, existing []core.SemanticClaim, in PersistInput) (claims []core.SemanticClaim, links []core.ClaimLink, supersede []string) {
	claims, links = AttachClaims(v, in)
	ApplyClaimStatus(v, existing)
	supersede = StaleReviewClaims(*v, existing)
	return claims, links, supersede
}
