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
	"fmt"
	"sort"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// Importer holds everything Import needs beyond the projection itself —
// every field injected (no package-level state, no time.Now(), per repo
// convention: internal/CLAUDE.md's determinism contract).
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
	// FileContent returns the current corpus bytes for a repo-relative
	// path — used to hash cited lines into a cites link's TargetHash. A
	// nil func, or one that returns ok=false for a given path, means no
	// cites link is written for that path: the finding still imports, with
	// no cites link (never dropped).
	FileContent func(path string) ([]byte, bool)
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
		if prod.Origin == "fathom" {
			receipt.Rejected = append(receipt.Rejected, Rejection{Index: i, FindingID: findingID, Reason: "circular: fathom-origin producer"})
			continue
		}
		if missing := missingRequiredField(ev, prod.Requires); missing != "" {
			receipt.Rejected = append(receipt.Rejected, Rejection{Index: i, FindingID: findingID, Reason: fmt.Sprintf("missing required field %q", missing)})
			continue
		}

		tier := semantic.CapTier(core.EvidenceTier(ev.Rule.Tier), prod)
		claim := ClaimFromEvent(service, ev, tier)

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

		links, noFP := AnchorLinks(claim.ClaimID, ev, im.FileContent, im.Fingerprint)
		if noFP {
			receipt.NoFingerprint++
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
// given AND fileContent resolves the path (a bare-path or hashless cites
// link would read as anchor-dead to semantic.Sweep — see
// internal/analysis/review/claims.go's citeHash, mirrored here). Returns
// (nil, false) for an unanchored event — no links, and "no fingerprint" is
// meaningless when there is no path to fingerprint.
func AnchorLinks(claimID string, ev Event, fileContent func(string) ([]byte, bool), fingerprint func(string) (string, bool)) (links []core.ClaimLink, noFingerprint bool) {
	if ev.Anchor == nil {
		return nil, false
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

	if ev.Anchor.Start > 0 && ev.Anchor.End > 0 && fileContent != nil {
		if data, ok := fileContent(path); ok {
			if hash, ok := core.HashLines(data, ev.Anchor.Start, ev.Anchor.End); ok {
				text, _ := core.LinesText(data, ev.Anchor.Start, ev.Anchor.End)
				links = append(links, core.ClaimLink{
					ClaimID: claimID, Rel: core.RelCites,
					TargetRef:  core.CiteRef(path, ev.Anchor.Start, ev.Anchor.End),
					TargetHash: hash, AnchorText: text,
				})
			}
			// An unreadable or out-of-range line span still imports with no
			// cites link — never a fabricated hash.
		}
		// fileContent returning ok=false covers BOTH "absent" and
		// "unreadable" (I38): this fixed two-return-value signature cannot
		// distinguish them on its own. A caller wiring FileContent over real
		// disk I/O (cmd/fathom/findings_import.go) records the real
		// distinction into its own agentic-kit read-manifest Set at the
		// point of the actual os.ReadFile call, for its own observability;
		// AnchorLinks itself treats every "could not resolve" uniformly as
		// "no cites link, finding still imported" — which is the behavior
		// vision 30 §4 item 7 requires regardless of which of the two it was.
	}

	return links, noFingerprint
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
