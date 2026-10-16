package external

// importer.go — the governed import (FL-5, FL-6, FL-7, FL-8, FL-9): turns a
// validated Projection into content-addressed claims, applies the
// findings-loop lifecycle mapping, enforces per-claim ordering/idempotency
// via the external_findings cursor row, and reports a Receipt. Never
// touches a native (review-verb/v1) claim; never writes anything on
// dryRun; never writes anything at all if ParseProjection itself failed
// (that happens before Import is ever called).

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// Importer holds everything Import needs beyond the projection itself —
// every field injected (no package-level state, no time.Now(), per repo
// convention: internal/AGENTS.md's determinism contract).
type Importer struct {
	Store store.Store
	// Now supplies ImportedAt / event timestamps — an injected clock.
	Now func() time.Time
	// Catalog is the loaded producer catalog (semantic.LoadProducers /
	// semantic.DefaultProducers).
	Catalog semantic.ProducerCatalog
	// Subject is the provisioned "org/repo/component" mapping
	// (.fathom/config.toml [findings] subject). "" refuses every import
	// (FL-9).
	Subject string
	// ContentAt returns a repo-relative path's bytes as committed at a
	// revision — used to hash a finding's cited lines into its cites link's
	// TargetHash (WAE-1). Import calls it with the event's own
	// SourceRevision and never with an empty one, so a cite is hashed at
	// the revision the producer observed, never at the working tree; the
	// product wiring is GitContentAt. An error means no cites link for that
	// event, named in Receipt.CitesSkew: the finding still imports (never
	// dropped). A nil func writes no cites link and names nothing.
	ContentAt func(revision, path string) ([]byte, error)
	// Fingerprint returns the store's CURRENT content fingerprint for a
	// repo-relative path (the same table semantic.DriftedFindings reads) —
	// recorded as the claim's scope-fp drift anchor. A nil func, or one
	// that returns ok=false, means no scope-fp link is written and the
	// finding is counted in Receipt.NoFingerprint — it still imports.
	Fingerprint func(path string) (string, bool)
}

// Rejection is one event this Import call refused, naming its index in
// Projection.Events, its findingId, and why.
type Rejection struct {
	Index     int    `json:"index"`
	FindingID string `json:"finding_id"`
	Reason    string `json:"reason"`
}

// Cursor summarizes, per producer, the highest SourceVersion this Import
// call actually applied (delivered) — not the persisted per-claim cursor
// row (which store.ExternalFinding holds one of per claim_id), but a
// batch-level report of how far each producer's stream advanced in this
// call.
type Cursor struct {
	Producer            string `json:"producer"`
	LatestSourceVersion uint64 `json:"latest_source_version"`
}

// Receipt is Import's report: every count vision 30 §3.3 item 7 and plan
// FL-10 name, plus NoFingerprint (§3.3 item 6: a finding imports even when
// no store fingerprint is found for its anchor path — counted here rather
// than silently dropped).
type Receipt struct {
	SchemaVersion int         `json:"schema_version"`
	Delivered     int         `json:"delivered"`
	Duplicate     int         `json:"duplicate"`
	Stale         int         `json:"stale"`
	Unclassified  int         `json:"unclassified"`
	Rejected      []Rejection `json:"rejected"`
	Truncated     bool        `json:"truncated"`
	Cursors       []Cursor    `json:"cursors"`
	DryRun        bool        `json:"dry_run"`
	// NoFingerprint counts an anchored finding imported with no scope-fp
	// link because the store had no current fingerprint for its anchor
	// path — the finding is still imported (never dropped), just without
	// that one drift-tracking link.
	NoFingerprint int `json:"no_fingerprint"`
	// CitesSkew names each anchored finding imported with no cites link
	// because its cited lines could not be read at its own SourceRevision
	// (WAE-1): the revision is empty, unknown or unreadable, or the path
	// or the lines are absent there. The importer never falls back to the
	// working tree, whose lines may have moved since the producer looked.
	// Additive: omitted when nothing skewed.
	CitesSkew []CitesSkew `json:"cites_skew,omitempty"`
}

// CitesSkew is one event whose cited lines were not hashed: its index in
// Projection.Events, its findingId, the anchor path, the revision it
// carried and a stable reason (citesSkewReason).
type CitesSkew struct {
	Index     int    `json:"index"`
	FindingID string `json:"finding_id"`
	Path      string `json:"path"`
	Revision  string `json:"revision"`
	Reason    string `json:"reason"`
}

// Import applies every event in p against service, in order. producer ""
// accepts any catalogued producer; a non-empty producer requires every
// event's producer.name to match it exactly (else rejected
// "producer-mismatch"). dryRun computes the identical receipt without
// writing anything — every store write in this function is gated on
// !dryRun; the reads (cursor lookups) happen either way, since they are
// what the receipt's counts are computed from.
func (im Importer) Import(ctx context.Context, service, producer string, p Projection, dryRun bool) (Receipt, error) {
	// Rejected/Cursors start non-nil so a clean receipt marshals as JSON
	// "[]", never "null" — a caller diffing fixture JSON should never need
	// to special-case an empty batch.
	receipt := Receipt{SchemaVersion: 1, DryRun: dryRun, Rejected: []Rejection{}, Cursors: []Cursor{}}

	// FL-9: subject binding is checked for EVERY event before any store
	// read — a batch-level precondition, not a per-event soft rejection.
	for _, ev := range p.Events {
		if err := BindSubject(im.Subject, ev); err != nil {
			return Receipt{}, err
		}
	}

	cursorMax := map[string]uint64{}

	for i, ev := range p.Events {
		findingID := ev.FindingID

		if producer != "" && ev.Producer.Name != producer {
			receipt.Rejected = append(receipt.Rejected, Rejection{Index: i, FindingID: findingID, Reason: "producer-mismatch"})
			continue
		}

		prod, ok := im.Catalog.Lookup(ev.Producer.Name)
		if !ok {
			receipt.Rejected = append(receipt.Rejected, Rejection{Index: i, FindingID: findingID, Reason: "unknown-producer"})
			continue
		}
		if err := im.Catalog.Admit(prod); err != nil {
			receipt.Rejected = append(receipt.Rejected, Rejection{Index: i, FindingID: findingID, Reason: "circular: fathom-origin producer"})
			continue
		}
		if missing := missingRequiredField(ev, prod.Requires); missing != "" {
			receipt.Rejected = append(receipt.Rejected, Rejection{Index: i, FindingID: findingID, Reason: fmt.Sprintf("missing required field %q", missing)})
			continue
		}

		tier := semantic.CapTier(core.EvidenceTier(ev.Rule.Tier), prod)
		claim := ClaimFromEvent(service, ev, tier)
		// SU-22 (ADR-041 decision 7): an imported claim is asserted by its
		// producer's own identity, producer@version — the import cursor
		// identity this whole ordering/idempotency scheme already keys on.
		claim.AssertedBy = core.AssertedBy("import", ev.Producer.Name+"@"+ev.Producer.Version)

		existing, found, err := im.Store.ExternalFinding(ctx, service, claim.ClaimID)
		if err != nil {
			return Receipt{}, fmt.Errorf("findings import: read cursor for claim %s: %w", claim.ClaimID, err)
		}
		if found {
			if ev.SourceVersion < existing.SourceVersion {
				receipt.Stale++
				continue
			}
			if ev.SourceVersion == existing.SourceVersion {
				if ev.PayloadDigest == existing.PayloadDigest {
					receipt.Duplicate++
					continue
				}
				receipt.Rejected = append(receipt.Rejected, Rejection{Index: i, FindingID: findingID, Reason: "conflicting-replay"})
				continue
			}
		}

		var contentAt func(revision, path string) ([]byte, error)
		if im.ContentAt != nil {
			contentAt = im.citedContent
		}
		links, noFP, skew := AnchorLinks(claim.ClaimID, ev, contentAt, im.Fingerprint)
		if noFP {
			receipt.NoFingerprint++
		}
		if skew != nil {
			receipt.CitesSkew = append(receipt.CitesSkew, CitesSkew{
				Index: i, FindingID: findingID, Path: ev.Anchor.Path,
				Revision: ev.SourceRevision, Reason: citesSkewReason(skew),
			})
		}
		if ev.Rule.Class == "" {
			receipt.Unclassified++
		}

		if !dryRun {
			if err := im.ApplyEvent(ctx, service, ev, claim, links); err != nil {
				return Receipt{}, fmt.Errorf("findings import: apply event[%d] (findingId=%s): %w", i, findingID, err)
			}
			ef := core.ExternalFinding{
				ServiceID: service, ClaimID: claim.ClaimID,
				Producer: ev.Producer.Name, ProducerVersion: ev.Producer.Version,
				SourceID: ev.SourceID, SourceVersion: ev.SourceVersion,
				PayloadDigest: ev.PayloadDigest, SourceRevision: ev.SourceRevision,
				RuleID: ev.Rule.ID, RuleVersion: ev.Rule.Version, Remedy: ev.Rule.Remedy,
				ObservedAt: ev.ObservedAt, ImportedAt: im.Now().UTC(),
			}
			if ev.Anchor != nil {
				ef.AnchorPath = ev.Anchor.Path
				ef.AnchorStart = ev.Anchor.Start
				ef.AnchorEnd = ev.Anchor.End
				ef.AnchorSymbol = ev.Anchor.Symbol
				ef.AnchorDigest = ev.Anchor.Digest
			} else {
				ef.Unanchored = true
			}
			if err := im.Store.PutExternalFinding(ctx, ef); err != nil {
				return Receipt{}, fmt.Errorf("findings import: write cursor for claim %s: %w", claim.ClaimID, err)
			}
		}

		receipt.Delivered++
		if ev.SourceVersion > cursorMax[ev.Producer.Name] {
			cursorMax[ev.Producer.Name] = ev.SourceVersion
		}
	}

	var producers []string
	for p := range cursorMax {
		producers = append(producers, p)
	}
	sort.Strings(producers)
	for _, p := range producers {
		receipt.Cursors = append(receipt.Cursors, Cursor{Producer: p, LatestSourceVersion: cursorMax[p]})
	}

	return receipt, nil
}

// BindSubject (FL-9) refuses when no subject is provisioned, or when ev's
// subject does not equal the provisioned one. Called for every event before
// any store read — a mismatch anywhere in the batch refuses the WHOLE
// import, nothing written.
func BindSubject(provisioned string, ev Event) error {
	if provisioned == "" {
		return fmt.Errorf("findings import: no [findings] subject provisioned")
	}
	got := ev.Subject.String()
	if got != provisioned {
		return fmt.Errorf("findings import: event subject %q does not match provisioned subject %q", got, provisioned)
	}
	return nil
}

// ClaimFromEvent (FL-5) derives the content-addressed SemanticClaim for ev,
// deterministically: identical events reproduce identical ClaimIDs across
// runs. tier is the ALREADY-CAPPED tier (semantic.CapTier(event tier,
// producer) — Import computes it; ClaimFromEvent just carries it through)
// so this function needs no catalog access of its own.
func ClaimFromEvent(serviceID string, ev Event, tier core.EvidenceTier) core.SemanticClaim {
	subject := externalSubject(ev.Producer.Name, ev.Subject, ev.SourceID)
	claimID := core.ClaimID("findings", "finding", subject, ev.FindingID)

	var scope []string
	if ev.Anchor != nil {
		scope = []string{ev.Anchor.Path}
	}

	return core.SemanticClaim{
		ClaimID:             claimID,
		ServiceID:           serviceID,
		Layer:               "findings",
		LayerVersion:        "v1",
		Kind:                "finding",
		Subject:             subject,
		Rule:                ev.Rule.Text,
		Scope:               scope,
		Tier:                tier,
		TierProfile:         "engineering",
		Status:              core.ClaimFresh,
		PromptVersion:       "external-import/v1",
		ReproKey:            ev.FindingID,
		InputDigest:         ev.PayloadDigest,
		CompiledAtWatermark: ev.SourceRevision,
		CompiledAt:          ev.ObservedAt,
		Class:               core.FindingClass(ev.Rule.Class),
	}
}

// externalSubject builds the "ext:<producer>/<org>/<repo>/<component>/<sourceId>"
// claim Subject (FL-5).
func externalSubject(producer string, subj Subject, sourceID string) string {
	return "ext:" + producer + "/" + subj.Organization + "/" + subj.Repository + "/" + subj.Component + "/" + sourceID
}

// AnchorLinks (FL-8) builds the claim links for ev: "scopes" whenever
// anchored, "scope-fp" when the store has a current fingerprint for the
// anchor path (noFP reports when it does not — the finding still imports,
// just without that link), and "cites" only when anchor.start/end are both
// given AND contentAt returns the path's bytes at ev.SourceRevision with
// the span in range (a bare-path or hashless cites link would read as
// anchor-dead to semantic.Sweep — see internal/analysis/review/claims.go's
// citeHash, mirrored here). skew is why a wanted cites link was not
// written: contentAt's error, or ErrLinesAbsent when the span is out of
// range in the bytes it returned — never a fabricated hash. contentAt
// decides what a revision means: the importer's reads the commit through
// git (GitContentAt), `fathom claim add`'s reads the working tree its
// author sees (its event carries no revision). Returns (nil, false, nil)
// for an unanchored event — no links, and "no fingerprint" is meaningless
// when there is no path to fingerprint.
func AnchorLinks(claimID string, ev Event, contentAt func(revision, path string) ([]byte, error), fingerprint func(string) (string, bool)) (links []core.ClaimLink, noFingerprint bool, skew error) {
	if ev.Anchor == nil {
		return nil, false, nil
	}
	path := ev.Anchor.Path

	links = append(links, core.ClaimLink{ClaimID: claimID, Rel: core.RelScopes, TargetRef: path})

	fp, fpOK := "", false
	if fingerprint != nil {
		fp, fpOK = fingerprint(path)
	}
	if fpOK {
		links = append(links, core.ClaimLink{ClaimID: claimID, Rel: core.RelScopeFP, TargetRef: path, TargetHash: fp})
	} else {
		noFingerprint = true
	}

	if ev.Anchor.Start > 0 && ev.Anchor.End > 0 && contentAt != nil {
		data, err := contentAt(ev.SourceRevision, path)
		if err != nil {
			return links, noFingerprint, err
		}
		hash, ok := core.HashLines(data, ev.Anchor.Start, ev.Anchor.End)
		if !ok {
			return links, noFingerprint, ErrLinesAbsent
		}
		text, _ := core.LinesText(data, ev.Anchor.Start, ev.Anchor.End)
		links = append(links, core.ClaimLink{
			ClaimID: claimID, Rel: core.RelCites,
			TargetRef:  core.CiteRef(path, ev.Anchor.Start, ev.Anchor.End),
			TargetHash: hash, AnchorText: text,
		})
	}

	return links, noFingerprint, nil
}

// ErrNoSourceRevision is the importer's refusal to hash cited lines for an
// event that carries no SourceRevision: there is no revision to read them
// at, and the working tree is not one.
var ErrNoSourceRevision = errors.New("event carries no sourceRevision")

// ErrLinesAbsent is AnchorLinks' skew when the cited span is out of range
// in the file as read at the revision.
var ErrLinesAbsent = errors.New("cited lines absent at revision")

// citedContent is im.ContentAt held to the importer's rule: a cited line is
// hashed at the event's own revision or not at all.
func (im Importer) citedContent(revision, path string) ([]byte, error) {
	if revision == "" {
		return nil, ErrNoSourceRevision
	}
	return im.ContentAt(revision, path)
}

// GitContentAt is the importer's ContentAt over the corpus at root: the
// path's blob at the revision, read through git without touching the
// working tree, the index, the network or a prompt (gitro.BlobAt). A root
// that is not a git work tree, an unknown revision, or a path absent at it
// is an error, never a disk read.
func GitContentAt(ctx context.Context, root string) func(revision, path string) ([]byte, error) {
	return func(revision, path string) ([]byte, error) {
		if root == "" {
			return nil, gitro.ErrNotRepository
		}
		data, _, err := gitro.BlobAt(ctx, root, revision, path, 0)
		return data, err
	}
}

// citesSkewReason is CitesSkew.Reason: a stable kebab-case name for why a
// cites link was not written.
func citesSkewReason(err error) string {
	switch {
	case errors.Is(err, ErrNoSourceRevision), errors.Is(err, gitro.ErrNoRevision):
		return "no-source-revision"
	case errors.Is(err, gitro.ErrBadRevision):
		return "bad-revision"
	case errors.Is(err, gitro.ErrNotRepository):
		return "not-a-repository"
	case errors.Is(err, gitro.ErrUnknownRevision):
		return "unknown-revision"
	case errors.Is(err, gitro.ErrBadPath):
		return "path-outside-corpus"
	case errors.Is(err, gitro.ErrPathAbsent):
		return "path-absent-at-revision"
	case errors.Is(err, gitro.ErrNotBlob):
		return "not-a-file-at-revision"
	case errors.Is(err, gitro.ErrBlobTooLarge):
		return "over-read-cap"
	case errors.Is(err, ErrLinesAbsent):
		return "lines-absent-at-revision"
	}
	return "unreadable-at-revision"
}

// ApplyEvent (FL-6) writes the store side effects for one already-ordering-
// cleared event: the SemanticClaim upsert plus whatever the lifecycle
// mapping calls for. Only called when !dryRun (Import gates the call), and
// never writes the external_findings cursor row itself — Import does that
// once, uniformly, after every kind, so ordering/idempotency covers
// annotation events too.
//
// observation/unknown is deliberately treated like observation/fail (fresh,
// no supersede): the contract's lifecycle table names only fail and pass
// explicitly, and "unknown" is not proof of resolution — never inventing a
// closure the source did not state.
func (im Importer) ApplyEvent(ctx context.Context, service string, ev Event, claim core.SemanticClaim, links []core.ClaimLink) error {
	now := im.Now().UTC()

	switch {
	case ev.Kind == "annotation":
		return im.Store.AppendClaimEvent(ctx, core.ClaimEvent{
			ClaimID: claim.ClaimID, Verdict: "annotated", Author: ev.Producer.Name, Note: ev.Annotation, At: now,
		})

	case ev.Kind == "revocation":
		if err := im.Store.PutClaims(ctx, []core.SemanticClaim{claim}, links); err != nil {
			return err
		}
		if err := im.Store.SetClaimStatus(ctx, service, claim.ClaimID, core.ClaimSuperseded); err != nil {
			return err
		}
		// SU-19 (ADR-041 decision 7): close the claim's bi-temporal
		// validity window at the same instant its status flips — the
		// PutClaims upsert just above reopened it (a fresh, non-refuted
		// row's valid_to always takes the incoming — zero/open — value).
		if err := im.Store.SetClaimValidTo(ctx, service, claim.ClaimID, now); err != nil {
			return err
		}
		return im.Store.AppendClaimEvent(ctx, core.ClaimEvent{
			ClaimID: claim.ClaimID, Verdict: "superseded", ResolutionKind: core.ResolutionRevoked,
			Author: ev.Producer.Name,
			Note:   fmt.Sprintf("revoked by %s v%s at %s", ev.Producer.Name, ev.Producer.Version, ev.SourceRevision),
			At:     now,
		})

	case ev.Kind == "closure" || (ev.Kind == "observation" && ev.Outcome == "pass"):
		if err := im.Store.PutClaims(ctx, []core.SemanticClaim{claim}, links); err != nil {
			return err
		}
		if err := im.Store.SetClaimStatus(ctx, service, claim.ClaimID, core.ClaimSuperseded); err != nil {
			return err
		}
		// SU-19 (ADR-041 decision 7): see the revocation branch's own
		// comment above — same close, same reason.
		if err := im.Store.SetClaimValidTo(ctx, service, claim.ClaimID, now); err != nil {
			return err
		}
		return im.Store.AppendClaimEvent(ctx, core.ClaimEvent{
			ClaimID: claim.ClaimID, Verdict: "superseded", ResolutionKind: core.ResolutionExternalClosure,
			Author: ev.Producer.Name,
			Note:   fmt.Sprintf("closure by %s v%s at %s", ev.Producer.Name, ev.Producer.Version, ev.SourceRevision),
			At:     now,
		})

	default: // observation/fail, observation/unknown
		return im.Store.PutClaims(ctx, []core.SemanticClaim{claim}, links)
	}
}

// missingRequiredField returns the first name in requires whose
// corresponding event field is unset, or "" when every requirement is met.
// A requirement name this importer does not recognize fails closed (treated
// as unmet) rather than silently passing — a producer catalog requirement
// this code cannot check must never read as satisfied.
func missingRequiredField(ev Event, requires []string) string {
	for _, r := range requires {
		switch r {
		case "rule.closure":
			if ev.Rule.Closure == "" {
				return r
			}
		case "rule.remedy":
			if ev.Rule.Remedy == "" {
				return r
			}
		case "rule.text":
			if ev.Rule.Text == "" {
				return r
			}
		case "rule.id":
			if ev.Rule.ID == "" {
				return r
			}
		case "anchor":
			if ev.Anchor == nil {
				return r
			}
		case "sourceRevision":
			if ev.SourceRevision == "" {
				return r
			}
		default:
			return r
		}
	}
	return ""
}

// IndependentProducers reports whether the catalogued producers named a and
// b are independent — corroboration must count only across independent
// producers, never the same one twice (vision 30 §4 / ADR-040 decision 4,
// SU-25 / ADR-041 decision 8). Delegates to im.Catalog.Independent, which
// itself delegates to agentic-kit's evidenceproducer.Catalog.Independent.
// An unresolved producer name can't be proven distinct from anything, so
// this fails closed rather than defaulting to "independent" — mirrors
// missingRequiredField's convention for an unrecognized requirement.
func (im Importer) IndependentProducers(a, b string) bool {
	pa, aok := im.Catalog.Lookup(a)
	pb, bok := im.Catalog.Lookup(b)
	if !aok || !bok {
		return false
	}
	return im.Catalog.Independent(pa, pb)
}
