package client

// claim_add.go — SU-17 (docs/vision/31-substrate.md §3 D8 item 4, ADR-041
// decision 8, plan `docs/plans/substrate.md` §D): "a single-utterance
// 'remember this' that becomes a scoped standing fact without waiting for
// recurrence." AddClaim mints ONE content-addressed, human-authored claim
// directly — no projection JSON, no batch, no subject provisioning (that
// machinery is internal/findings/external's governed BATCH import, FL-9;
// this is a single deliberate CLI write, mirroring ClaimFeedback's own
// store-direct discipline). It reuses the importer's own claim-construction
// helpers (external.ClaimFromEvent, external.AnchorLinks) so a human-
// authored claim is identical in shape to an imported one — same tier
// capping via the producer catalog, same scope-fp/cites link conventions —
// rather than a second, drifting code path.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/findings/external"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// humanReviewProducer is the catalogued producer id every `fathom claim add`
// claim is attributed to (schema/semantic/producers.cue: kind "human",
// origin "external", tier_cap "sme-stated" — the human cap SU-17 requires).
const humanReviewProducer = "human.review"

// HumanAuthoredPromptVersion is the claim's own reconciliation domain
// (ADR-040 decision 3's pattern, extended): distinct from "review-verb/v1"
// (native review) and "external-import/v1" (the governed batch import) so
// neither of those ever supersedes or is superseded by a `claim add` claim.
const HumanAuthoredPromptVersion = "human-authored/v1"

// AddClaimRequest bundles `fathom claim add`'s inputs.
type AddClaimRequest struct {
	Service string
	// Scope is one or more code-path globs this claim governs (ADR-016
	// §4.4's own convention) — required, non-empty.
	Scope []string
	// Rule is the claim text served to agents — required, non-empty.
	Rule string
	// Remedy is optional remediation guidance (carried onto the
	// external_findings row's Remedy field, mirroring an imported finding's
	// rule.remedy).
	Remedy string
	// CiteRef, when non-empty, is "path#Lstart-Lend" (core.CiteRef's own
	// shape) — the claim's citation anchor. Its path need not be one of
	// Scope's globs (a rule scoped broadly can still cite one file as its
	// evidence).
	CiteRef string
	// SessionID (SU-22, ADR-041 decision 7) is the harness hook session id
	// that asked for this claim, when the caller can resolve one
	// (cmd/fathom/claim_add.go's currentSessionID()) — recorded as
	// core.SemanticClaim.AssertedBy ("claim-add:<session>"). "" when no
	// session is resolvable (a script driving the client directly); the
	// claim is still minted, just with an empty AssertedBy.
	SessionID string
}

// AddClaimResult is what AddClaim minted.
type AddClaimResult struct {
	ClaimID string             `json:"claim_id"`
	Claim   core.SemanticClaim `json:"claim"`
}

// AddClaim mints and persists one human-authored SemanticClaim (SU-17).
// Tier is ALWAYS capped at whatever schema/semantic/producers.cue declares
// for "human.review" (sme-stated as of this writing) via the same
// semantic.CapTier the governed importer uses — there is no request field
// that can raise it; the CLI layer (cmd/fathom/claim_add.go) refuses a
// --tier flag outright rather than accepting and ignoring one.
func (c *Client) AddClaim(ctx context.Context, req AddClaimRequest) (AddClaimResult, error) {
	if req.Service == "" {
		return AddClaimResult{}, fmt.Errorf("client.AddClaim: service is required")
	}
	if len(req.Scope) == 0 {
		return AddClaimResult{}, fmt.Errorf("client.AddClaim: at least one --scope glob is required")
	}
	if req.Rule == "" {
		return AddClaimResult{}, fmt.Errorf("client.AddClaim: --rule is required")
	}

	prod, ok := semantic.DefaultProducers().Lookup(humanReviewProducer)
	if !ok {
		return AddClaimResult{}, fmt.Errorf("client.AddClaim: producer %q is not catalogued", humanReviewProducer)
	}

	scope := append([]string(nil), req.Scope...)
	sort.Strings(scope)

	sourceID := humanSourceID(req.Rule)
	findingID := humanFindingID(req.Rule, scope, req.Remedy)
	payloadDigest := findingID

	var anchor *external.Anchor
	if req.CiteRef != "" {
		path, start, end, ok := core.ParseCiteRef(req.CiteRef)
		if !ok {
			return AddClaimResult{}, fmt.Errorf("client.AddClaim: malformed --cite %q (want path#Lstart-Lend)", req.CiteRef)
		}
		anchor = &external.Anchor{Path: path, Start: start, End: end}
	} else if len(scope) == 1 && !looksLikeGlob(scope[0]) {
		// A single literal-file scope doubles as its own anchor path so the
		// claim still gets a scope-fp drift anchor even with no explicit
		// --cite (mirrors compile.go's drift-tracked-layer convention).
		anchor = &external.Anchor{Path: scope[0]}
	}

	ev := external.Event{
		Kind:          "observation",
		FindingID:     findingID,
		Producer:      external.Producer{Name: humanReviewProducer},
		Subject:       external.Subject{Organization: "human", Repository: req.Service, Component: "review"},
		SourceID:      sourceID,
		SourceVersion: 1,
		PayloadDigest: payloadDigest,
		ObservedAt:    time.Now().UTC(),
		Outcome:       "fail", // a live, unresolved finding — never "pass"/"closure" (ApplyEvent's default branch: fresh, no supersede)
		Anchor:        anchor,
		Rule:          external.Rule{ID: sourceID, Version: "1", Text: req.Rule, Remedy: req.Remedy, Tier: string(core.TierSMEStated)},
	}

	tier := semantic.CapTier(core.EvidenceTier(ev.Rule.Tier), prod)
	claim := external.ClaimFromEvent(req.Service, ev, tier)
	// ClaimFromEvent derives Scope from the anchor alone (a single path, or
	// none) — this request's own --scope glob(s) are the real governing
	// scope and may be broader than (or absent from) the anchor.
	claim.Scope = scope
	claim.PromptVersion = HumanAuthoredPromptVersion
	// SU-22 (ADR-041 decision 7): a human-authored claim is asserted by the
	// hook session that minted it, when one is resolvable — "" (never
	// fabricated) otherwise.
	if req.SessionID != "" {
		claim.AssertedBy = core.AssertedBy("claim-add", req.SessionID)
	}

	root := ""
	if meta, found, _ := c.st.GetServiceMeta(ctx, req.Service); found {
		root = meta.Path
	}
	// A human-authored claim cites the working tree its author sees: its
	// event carries no SourceRevision, so the revision is ignored here
	// (the importer's revision-bound read is GitContentAt, WAE-1).
	fileContent := func(_, p string) ([]byte, error) {
		diskPath := p
		if !filepath.IsAbs(p) && root != "" {
			diskPath = filepath.Join(root, p)
		}
		return os.ReadFile(diskPath)
	}
	fingerprint := func(p string) (string, bool) {
		hash, found, err := c.st.Fingerprint(ctx, req.Service, p)
		if err != nil || !found {
			return "", false
		}
		return hash, true
	}

	var links []core.ClaimLink
	for _, g := range scope {
		links = append(links, core.ClaimLink{ClaimID: claim.ClaimID, Rel: core.RelScopes, TargetRef: g})
	}
	if anchor != nil {
		anchorLinks, _, _ := external.AnchorLinks(claim.ClaimID, ev, fileContent, fingerprint)
		for _, l := range anchorLinks {
			if l.Rel == core.RelScopes {
				continue // already written above, once per --scope glob
			}
			links = append(links, l)
		}
	}

	if err := c.st.PutClaims(ctx, []core.SemanticClaim{claim}, links); err != nil {
		return AddClaimResult{}, fmt.Errorf("client.AddClaim: put claim: %w", err)
	}

	now := time.Now().UTC()
	ef := core.ExternalFinding{
		ServiceID: req.Service, ClaimID: claim.ClaimID,
		Producer: humanReviewProducer, ProducerVersion: "",
		SourceID: sourceID, SourceVersion: 1, PayloadDigest: payloadDigest,
		RuleID: sourceID, RuleVersion: "1", Remedy: req.Remedy,
		ObservedAt: ev.ObservedAt, ImportedAt: now,
	}
	if anchor != nil {
		ef.AnchorPath, ef.AnchorStart, ef.AnchorEnd = anchor.Path, anchor.Start, anchor.End
	} else {
		ef.Unanchored = true
	}
	if err := c.st.PutExternalFinding(ctx, ef); err != nil {
		return AddClaimResult{}, fmt.Errorf("client.AddClaim: put external finding: %w", err)
	}

	return AddClaimResult{ClaimID: claim.ClaimID, Claim: claim}, nil
}

// humanSourceID is SU-17's literal spec: "source id human:<sha256(rule)[:12]>".
func humanSourceID(rule string) string {
	h := sha256.Sum256([]byte(rule))
	return "human:" + hex.EncodeToString(h[:])[:12]
}

// humanFindingID is the claim's content-addressed ReproKey/FindingID: full
// sha256 over rule+scope+remedy, so two `claim add` calls with the SAME rule
// text but DIFFERENT scope or remedy never collide (unlike humanSourceID's
// truncated, rule-only hash, which exists only for the source id label).
func humanFindingID(rule string, scope []string, remedy string) string {
	h := sha256.New()
	h.Write([]byte(rule))
	h.Write([]byte{0})
	for _, s := range scope {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	h.Write([]byte(remedy))
	return hex.EncodeToString(h.Sum(nil))
}

// looksLikeGlob reports whether s contains a glob metacharacter — used only
// to decide whether a single-entry --scope can double as a literal anchor
// path (a real glob has no one file to fingerprint).
func looksLikeGlob(s string) bool {
	for _, r := range s {
		switch r {
		case '*', '?', '[':
			return true
		}
	}
	return false
}
