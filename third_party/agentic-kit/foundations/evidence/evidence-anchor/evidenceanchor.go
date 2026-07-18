// Package evidenceanchor is the span-aware anchoring layer of the platform's
// "Witness". It proves that a *specific* evidence span — with its exact Grade,
// Author, Locator, record hashes, and timestamps — existed and was unaltered at
// a known instant T, not merely that some opaque hash existed.
//
// It composes two foundation primitives without reimplementing either:
//
//   - evidence-span supplies the Span type (the atomic evidence unit) and its
//     ValidateAsEvidence provenance gate.
//   - provenance/anchor supplies the domain-separated Merkle tree, per-leaf
//     InclusionProof, and the Anchor proof-of-existence record bound to an
//     external timestamp authority.
//
// The bridge between them is CanonicalDigest: a deterministic, injective
// content hash over the load-bearing fields of a span. That digest — not the
// raw span and not an arbitrary upstream hash — is what becomes a Merkle leaf.
// Because the digest covers Grade/Author/Locator/RecordHash/timestamps, any
// alteration of those fields produces a different digest, so a SpanProof can
// only verify against the span it was built from. That is the property that
// makes this "span-aware": the proof is bound to the claim, its grade, and its
// authorship, not to an anonymous blob.
//
// Determinism: no time.Now is called in the output path. AnchorSpans takes an
// injected clock (a func() time.Time), exactly as provenance/anchor's
// AnchorBuilder does, so the same spans + same clock always yield byte-identical
// digests, roots, and proofs.
//
// Dependencies: stdlib only (crypto/sha256, encoding/binary, encoding/hex) plus
// the two sibling kit packages. No external modules.
//
// Concurrency: all exported functions (CanonicalDigest, AnchorSpans, Verify)
// are pure functions with no shared mutable state and are safe for concurrent
// use. They carry no locks and allocate per-call; there is no package-level
// state.
package evidenceanchor

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"time"

	evidencegrade "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-grade"
	evidencespan "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-span"
	"github.com/geoffmilleraz/agentic-kit/foundations/provenance/anchor"
)

// digestDomainTag is a version/scheme tag mixed into every CanonicalDigest so
// that a digest produced by this package can never be confused with a raw
// SHA-256 of some other byte string, and so the canonical form can be revised
// in the future (bump the tag) without silently colliding with v1 digests.
const digestDomainTag = "evidence-anchor/canonical-digest/v1"

// SpanProof is the per-span witness returned by AnchorSpans. It binds one
// evidence span to the batch's Merkle root at AnchoredAt and carries everything
// needed to verify that binding offline.
//
// A SpanProof verifies (via Verify) only for the exact span whose canonical
// form produced SpanDigest: alter the span's Value, Grade, Author, Locator,
// RecordHash, or timestamps and the recomputed digest no longer matches.
type SpanProof struct {
	// SpanID echoes the proven span's ID for correlation. It is NOT part of the
	// trust boundary (Verify recomputes the digest from the supplied span); it
	// is a convenience for matching a proof back to its span.
	SpanID string

	// SpanDigest is the lowercase hex SHA-256 CanonicalDigest of the span. It is
	// the Merkle leaf's pre-image (the leaf hash is the anchor package's
	// domain-separated SHA256(0x00 || rawDigestBytes)).
	SpanDigest string

	// InclusionProof is the Merkle path from this span's leaf to Root, as
	// produced by provenance/anchor.
	InclusionProof anchor.InclusionProof

	// Root is the lowercase hex SHA-256 Merkle root of the whole batch.
	Root string

	// AnchoredAt is the injected wall-clock instant T at which the batch was
	// anchored (the Anchor.CreatedAt). It is the "and was unaltered at time T"
	// half of the proof; trust in T comes from the Anchor's external authority
	// receipt, not from this field alone.
	AnchoredAt time.Time
}

// CanonicalDigest returns the deterministic, injective content hash of span as
// a lowercase hex SHA-256 string.
//
// The encoding is length-prefixed: every variable-length field is written as an
// 8-byte big-endian length followed by its raw bytes, and every fixed-shape
// field (timestamps as Unix-nanoseconds, the float64 Confidence as its IEEE-754
// bit pattern, the Grade tier) is written in a fixed-width form. Length-prefixing
// makes the encoding injective — two distinct spans can never serialise to the
// same byte sequence by, e.g., shifting a delimiter into a field value — which
// is the property a content hash needs to be a sound identity.
//
// Fields covered (the load-bearing evidence claim + its provenance):
//
//	domain tag, ID, EntityID, Field, Value, Confidence,
//	SourceID, OccurredAt, ObservedAt, RecordID, RecordHash,
//	Locator{Kind,Value,Start,End},
//	Author (Kind + reproduction key), Grade{Tier,Confidence}.
//
// Timestamps are encoded as UTC Unix-nanoseconds so the digest is invariant to
// the monotonic-clock reading and to the *location* a time.Time carries while
// remaining sensitive to the actual instant. CanonicalDigest does not call
// time.Now and is therefore safe in the deterministic output path.
//
// CanonicalDigest does not validate the span; call span.ValidateAsEvidence
// upstream (AnchorSpans does) when provenance enforcement is required. A
// zero-value or partial span still produces a well-defined, stable digest.
func CanonicalDigest(span evidencespan.Span) string {
	h := sha256.New()

	writeField(h, []byte(digestDomainTag))
	writeField(h, []byte(span.ID))
	writeField(h, []byte(span.EntityID))
	writeField(h, []byte(span.Field))
	writeField(h, []byte(span.Value))
	writeUint64(h, math.Float64bits(span.Confidence))
	writeField(h, []byte(span.SourceID))
	writeTime(h, span.OccurredAt)
	writeTime(h, span.ObservedAt)
	writeField(h, []byte(span.RecordID))
	writeField(h, []byte(span.RecordHash))

	// Locator.
	writeField(h, []byte(span.Locator.Kind))
	writeField(h, []byte(span.Locator.Value))
	writeUint64(h, uint64(int64(span.Locator.Start)))
	writeUint64(h, uint64(int64(span.Locator.End)))

	// Author: kind + a stable reproduction key. ReproKey is itself an injective,
	// length-prefixed encoding of the author's identity/repro fields (see
	// aiprovenance.ReproKey), so it captures who/what produced the span without
	// us re-walking the Author struct here.
	writeField(h, []byte(span.Author.Kind))
	writeField(h, []byte(span.Author.ReproKey()))

	// Grade: present-flag + tier + confidence. The present-flag distinguishes a
	// nil Grade (legacy bare-Confidence span) from a Grade explicitly set to the
	// zero tier, so the two never collide.
	writeGrade(h, span.Grade)

	return hex.EncodeToString(h.Sum(nil))
}

// AnchorSpans builds a Merkle tree over the canonical digests of spans and
// returns one SpanProof per span (in input order), the underlying Anchor, and
// any error.
//
// Each span is first gated through ValidateAsEvidence: a span with no provenance
// is not anchorable (platform invariant: "no provenance → not storable as
// evidence"), and anchoring an unprovenanced span would mint a worthless witness.
// The raw 32-byte digest bytes (not the hex string) are then used as Merkle
// leaves via provenance/anchor's AnchorBuilder, so this package never
// reimplements the tree, the leaf/node hashing, or the inclusion-proof logic.
//
// clock is the injected timestamp source (must be non-nil); it sets
// Anchor.CreatedAt and every SpanProof.AnchoredAt. authority and receipt are
// passed straight through to anchor.MakeAnchor (authority identifies the
// external timestamp authority, e.g. "rfc3161:freetsa.org"; receipt is its
// opaque proof, or nil for a still-pending anchor).
//
// Returns an error if spans is empty, clock is nil, authority is empty, any
// span fails ValidateAsEvidence, or the underlying tree build fails. On error
// the returned proofs slice and Anchor are zero.
func AnchorSpans(spans []evidencespan.Span, clock func() time.Time, authority string, receipt []byte) ([]SpanProof, anchor.Anchor, error) {
	if len(spans) == 0 {
		return nil, anchor.Anchor{}, fmt.Errorf("evidenceanchor: AnchorSpans: at least one span is required")
	}
	if clock == nil {
		return nil, anchor.Anchor{}, fmt.Errorf("evidenceanchor: AnchorSpans: clock must not be nil")
	}
	if authority == "" {
		return nil, anchor.Anchor{}, fmt.Errorf("evidenceanchor: AnchorSpans: authority must not be empty")
	}

	digests := make([]string, len(spans))
	b := anchor.NewAnchorBuilderWithClock(clock)
	for i, span := range spans {
		if err := span.ValidateAsEvidence(); err != nil {
			return nil, anchor.Anchor{}, fmt.Errorf("evidenceanchor: AnchorSpans: span %d (%q) is not anchorable: %w", i, span.ID, err)
		}
		digestHex := CanonicalDigest(span)
		digests[i] = digestHex
		raw, err := hex.DecodeString(digestHex)
		if err != nil {
			// CanonicalDigest always emits valid lowercase hex; this is defensive.
			return nil, anchor.Anchor{}, fmt.Errorf("evidenceanchor: AnchorSpans: span %d (%q): decoding digest: %w", i, span.ID, err)
		}
		b.AddLeaf(raw)
	}

	rootRaw, proofs, err := b.Build()
	if err != nil {
		return nil, anchor.Anchor{}, fmt.Errorf("evidenceanchor: AnchorSpans: building anchor tree: %w", err)
	}

	a, err := b.MakeAnchor(authority, receipt)
	if err != nil {
		return nil, anchor.Anchor{}, fmt.Errorf("evidenceanchor: AnchorSpans: making anchor: %w", err)
	}

	rootHex := hex.EncodeToString(rootRaw)
	out := make([]SpanProof, len(spans))
	for i := range spans {
		out[i] = SpanProof{
			SpanID:         spans[i].ID,
			SpanDigest:     digests[i],
			InclusionProof: proofs[i],
			Root:           rootHex,
			AnchoredAt:     a.CreatedAt,
		}
	}
	return out, a, nil
}

// Verify reports whether proof is a valid witness for span: it recomputes the
// span's CanonicalDigest, checks it matches proof.SpanDigest, and verifies the
// inclusion proof against proof.Root.
//
// It returns true only if ALL of the following hold:
//
//   - proof.SpanDigest equals CanonicalDigest(span) — so the proof is bound to
//     this exact span, with this exact Grade/Author/Locator/RecordHash/value/
//     timestamps. Tamper with any covered field and this check fails.
//   - proof.Root and proof.SpanDigest are well-formed lowercase hex.
//   - anchor.Verify(rawDigest, proof.InclusionProof, rawRoot) is true — so the
//     span's leaf really was in the tree that produced Root.
//
// Verify never panics on adversarial or corrupt input (malformed hex, a
// mismatched proof, a wrong root); it returns false. It does NOT, by itself,
// establish *when* the span existed — that comes from the Anchor's external
// authority receipt bound to Root. Verify proves "this span is in this root";
// the Anchor proves "this root existed at time T".
func Verify(span evidencespan.Span, proof SpanProof) bool {
	want := CanonicalDigest(span)
	if proof.SpanDigest != want {
		return false
	}

	rawDigest, err := hex.DecodeString(proof.SpanDigest)
	if err != nil {
		return false
	}
	rawRoot, err := hex.DecodeString(proof.Root)
	if err != nil {
		return false
	}
	if len(rawRoot) == 0 {
		return false
	}

	return anchor.Verify(rawDigest, proof.InclusionProof, rawRoot)
}

// ---------------------------------------------------------------------------
// Canonical-encoding helpers (length-prefixed, injective).
// ---------------------------------------------------------------------------

// writeField writes an 8-byte big-endian length prefix followed by data. The
// length prefix is what makes the overall encoding injective: a field value can
// never "leak" into an adjacent field because its boundary is declared, not
// inferred from a delimiter that the value itself might contain.
func writeField(h interface{ Write([]byte) (int, error) }, data []byte) {
	writeUint64(h, uint64(len(data)))
	_, _ = h.Write(data)
}

// writeUint64 writes v as 8 big-endian bytes.
func writeUint64(h interface{ Write([]byte) (int, error) }, v uint64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], v)
	_, _ = h.Write(buf[:])
}

// writeTime writes a time as its UTC Unix-nanosecond count (8 big-endian bytes).
// The zero time is encoded as the int64 minimum sentinel's nanos via UnixNano,
// which is stable; what matters is that two equal instants encode identically
// and distinct instants encode differently, independent of monotonic-clock
// readings or the time.Location attached to the value.
func writeTime(h interface{ Write([]byte) (int, error) }, t time.Time) {
	// A zero time.Time has a wildly negative UnixNano; encode an explicit flag
	// so a genuine zero timestamp is distinguishable from a real instant and is
	// stable regardless of the platform's UnixNano behaviour for the zero value.
	if t.IsZero() {
		writeUint64(h, 0) // present flag: 0 == zero/unset
		return
	}
	writeUint64(h, 1) // present flag: 1 == set
	writeUint64(h, uint64(t.UTC().UnixNano()))
}

// writeGrade writes a present-flag then, when present, the grade's tier and
// confidence. A nil grade (legacy bare-Confidence span) encodes as a single
// 0 flag, which can never collide with any present grade.
func writeGrade(h interface{ Write([]byte) (int, error) }, g *evidencegrade.Grade) {
	if g == nil {
		writeUint64(h, 0) // present flag: 0 == nil grade
		return
	}
	writeUint64(h, 1) // present flag: 1 == grade present
	writeField(h, []byte(g.Tier))
	writeUint64(h, math.Float64bits(g.Confidence))
}
