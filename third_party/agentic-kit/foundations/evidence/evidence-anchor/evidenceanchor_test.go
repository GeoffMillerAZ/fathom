package evidenceanchor_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	ea "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-anchor"
	evidencegrade "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-grade"
	evidencespan "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-span"
	aiprovenance "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/anchor"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

var epoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// evidenceSpan returns a fully-provenanced span that passes ValidateAsEvidence,
// parameterised by id/value so tests can build distinct anchorable spans.
func evidenceSpan(id, value string) evidencespan.Span {
	g := evidencegrade.Grade{Tier: evidencegrade.TierDocumentary, Confidence: 0.91}
	return evidencespan.Span{
		ID:         id,
		EntityID:   "payments-service",
		Field:      "error-rate",
		Value:      value,
		Confidence: 0.91,
		SourceID:   "src-metrics-feed",
		OccurredAt: time.Date(2024, 6, 1, 14, 2, 0, 0, time.UTC),
		ObservedAt: time.Date(2024, 6, 1, 14, 5, 0, 0, time.UTC),
		RecordID:   "rec-metrics-feed-20240601",
		RecordHash: "a3f1b2c4d5e6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2",
		Locator: evidencespan.Locator{
			Kind:  "line",
			Value: "metrics.log:4821",
			Start: 4821,
			End:   4821,
		},
		Author: aiprovenance.NewAI(
			"claude-sonnet-4-6",
			"20250514",
			"sha256:promptabc123",
			"sha256:paramsxyz789",
			"sha256:inputdigest000",
			time.Date(2024, 6, 1, 14, 5, 1, 0, time.UTC),
		),
		Grade: &g,
	}
}

// ---------------------------------------------------------------------------
// CanonicalDigest — determinism & format
// ---------------------------------------------------------------------------

func TestCanonicalDigest_DeterministicAndHexFormat(t *testing.T) {
	s := evidenceSpan("span-1", "elevated-2x")

	d1 := ea.CanonicalDigest(s)
	d2 := ea.CanonicalDigest(s)
	if d1 != d2 {
		t.Fatalf("CanonicalDigest not deterministic: %q vs %q", d1, d2)
	}
	if len(d1) != 64 {
		t.Errorf("digest length = %d, want 64 (hex sha256)", len(d1))
	}
	for i, c := range d1 {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("digest[%d]=%q is not lowercase hex", i, c)
		}
	}
}

// TestCanonicalDigest_FieldSensitivity is the core "span-aware" guarantee:
// mutating any load-bearing field changes the digest. A platform that only
// hashed an opaque blob would miss these.
func TestCanonicalDigest_FieldSensitivity(t *testing.T) {
	base := evidenceSpan("span-1", "elevated-2x")
	baseDigest := ea.CanonicalDigest(base)

	otherGrade := evidencegrade.Grade{Tier: evidencegrade.TierDirect, Confidence: 0.91}
	otherAuthor := aiprovenance.NewHuman("analyst@example.com")

	mutators := map[string]func(s *evidencespan.Span){
		"ID":         func(s *evidencespan.Span) { s.ID = "span-2" },
		"EntityID":   func(s *evidencespan.Span) { s.EntityID = "other-entity" },
		"Field":      func(s *evidencespan.Span) { s.Field = "latency" },
		"Value":      func(s *evidencespan.Span) { s.Value = "nominal" },
		"Confidence": func(s *evidencespan.Span) { s.Confidence = 0.5 },
		"SourceID":   func(s *evidencespan.Span) { s.SourceID = "src-other" },
		"OccurredAt": func(s *evidencespan.Span) { s.OccurredAt = s.OccurredAt.Add(time.Nanosecond) },
		"ObservedAt": func(s *evidencespan.Span) { s.ObservedAt = s.ObservedAt.Add(time.Nanosecond) },
		"RecordID":   func(s *evidencespan.Span) { s.RecordID = "rec-other" },
		"RecordHash": func(s *evidencespan.Span) {
			s.RecordHash = "b3f1b2c4d5e6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"
		},
		"LocatorKind":   func(s *evidencespan.Span) { s.Locator.Kind = "page" },
		"LocatorValue":  func(s *evidencespan.Span) { s.Locator.Value = "other.log:1" },
		"LocatorStart":  func(s *evidencespan.Span) { s.Locator.Start = 1 },
		"LocatorEnd":    func(s *evidencespan.Span) { s.Locator.End = 9999 },
		"AuthorChanged": func(s *evidencespan.Span) { s.Author = otherAuthor },
		"GradeTier":     func(s *evidencespan.Span) { s.Grade = &otherGrade },
	}

	for name, mut := range mutators {
		t.Run(name, func(t *testing.T) {
			s := evidenceSpan("span-1", "elevated-2x")
			mut(&s)
			got := ea.CanonicalDigest(s)
			if got == baseDigest {
				t.Errorf("mutating %s did not change the digest (still %s) — field is not covered", name, got)
			}
		})
	}
}

// TestCanonicalDigest_NilVsZeroGradeDistinct verifies the present-flag: a nil
// Grade must not collide with a Grade explicitly set to the zero value.
func TestCanonicalDigest_NilVsZeroGradeDistinct(t *testing.T) {
	withNil := evidenceSpan("span-1", "v")
	withNil.Grade = nil

	withZero := evidenceSpan("span-1", "v")
	zero := evidencegrade.Grade{}
	withZero.Grade = &zero

	if ea.CanonicalDigest(withNil) == ea.CanonicalDigest(withZero) {
		t.Error("nil Grade and zero-value Grade produced the same digest; present-flag missing")
	}
}

// TestCanonicalDigest_Injective_FieldBoundary is an escaping/safety test: two
// spans that differ only by where a string boundary falls must not collide.
// Without length-prefixing, "ab"+"c" and "a"+"bc" could hash identically.
func TestCanonicalDigest_Injective_FieldBoundary(t *testing.T) {
	a := evidenceSpan("span-1", "v")
	a.EntityID = "ab"
	a.Field = "c-field" // Field must be non-empty per Validate; keep it real

	b := evidenceSpan("span-1", "v")
	b.EntityID = "a"
	b.Field = "bc-field"

	// Different field contents must produce different digests.
	if ea.CanonicalDigest(a) == ea.CanonicalDigest(b) {
		t.Error("boundary-shifted fields collided; encoding is not injective")
	}

	// Tighter check: a value containing the length-prefix bytes must not let an
	// adversary forge a colliding span. Build two spans whose concatenated
	// (EntityID+Field) bytes are equal but split differently.
	x := evidenceSpan("span-1", "v")
	x.EntityID = "alpha"
	x.Field = "beta"

	y := evidenceSpan("span-1", "v")
	y.EntityID = "alphabe"
	y.Field = "ta"

	if ea.CanonicalDigest(x) == ea.CanonicalDigest(y) {
		t.Error("EntityID/Field re-split collided; length-prefixing failed")
	}
}

// TestCanonicalDigest_TimeLocationInvariant ensures the same instant in a
// different time.Location yields the same digest (UTC normalisation).
func TestCanonicalDigest_TimeLocationInvariant(t *testing.T) {
	s1 := evidenceSpan("span-1", "v")

	loc := time.FixedZone("UTC+5", 5*3600)
	s2 := evidenceSpan("span-1", "v")
	s2.OccurredAt = s1.OccurredAt.In(loc)
	s2.ObservedAt = s1.ObservedAt.In(loc)

	if ea.CanonicalDigest(s1) != ea.CanonicalDigest(s2) {
		t.Error("same instant in different Location produced different digests; UTC normalisation missing")
	}
}

// ---------------------------------------------------------------------------
// AnchorSpans — happy path, determinism, leaf binding
// ---------------------------------------------------------------------------

func TestAnchorSpans_HappyPath(t *testing.T) {
	spans := []evidencespan.Span{
		evidenceSpan("span-1", "v1"),
		evidenceSpan("span-2", "v2"),
		evidenceSpan("span-3", "v3"),
	}

	proofs, a, err := ea.AnchorSpans(spans, fixedClock(epoch), "rfc3161:freetsa.org", []byte("receipt"))
	if err != nil {
		t.Fatalf("AnchorSpans: %v", err)
	}
	if len(proofs) != len(spans) {
		t.Fatalf("got %d proofs, want %d", len(proofs), len(spans))
	}
	if err := a.Validate(); err != nil {
		t.Errorf("returned Anchor failed Validate: %v", err)
	}
	if a.LeafCount != len(spans) {
		t.Errorf("Anchor.LeafCount = %d, want %d", a.LeafCount, len(spans))
	}
	if a.CreatedAt != epoch {
		t.Errorf("Anchor.CreatedAt = %v, want %v", a.CreatedAt, epoch)
	}
	if a.Status != anchor.StatusConfirmed {
		t.Errorf("with receipt, Anchor.Status = %q, want confirmed", a.Status)
	}

	for i, p := range proofs {
		if p.SpanID != spans[i].ID {
			t.Errorf("proof[%d].SpanID = %q, want %q", i, p.SpanID, spans[i].ID)
		}
		if p.Root != a.Root {
			t.Errorf("proof[%d].Root = %q, want anchor root %q", i, p.Root, a.Root)
		}
		if p.AnchoredAt != epoch {
			t.Errorf("proof[%d].AnchoredAt = %v, want %v", i, p.AnchoredAt, epoch)
		}
		if p.SpanDigest != ea.CanonicalDigest(spans[i]) {
			t.Errorf("proof[%d].SpanDigest does not match CanonicalDigest of span", i)
		}
		// Every proof must verify against its span.
		if !ea.Verify(spans[i], p) {
			t.Errorf("proof[%d] does not Verify for its span", i)
		}
	}
}

// TestAnchorSpans_LeafIsRawDigest confirms we anchor the canonical digest bytes
// (not the hex string, not the raw span) by recomputing the single-leaf root
// independently: root of a 1-leaf tree == leafHash(rawDigest).
func TestAnchorSpans_LeafIsRawDigest(t *testing.T) {
	s := evidenceSpan("span-1", "v")
	proofs, a, err := ea.AnchorSpans([]evidencespan.Span{s}, fixedClock(epoch), "opentimestamps", nil)
	if err != nil {
		t.Fatalf("AnchorSpans: %v", err)
	}

	rawDigest, err := hex.DecodeString(ea.CanonicalDigest(s))
	if err != nil {
		t.Fatalf("decode digest: %v", err)
	}
	// Domain-separated leaf hash exactly as provenance/anchor computes it.
	lh := sha256.New()
	lh.Write([]byte{0x00})
	lh.Write(rawDigest)
	wantRoot := hex.EncodeToString(lh.Sum(nil))

	if a.Root != wantRoot {
		t.Errorf("single-leaf root = %q, want leafHash(rawDigest) = %q", a.Root, wantRoot)
	}
	if proofs[0].Root != wantRoot {
		t.Errorf("proof root = %q, want %q", proofs[0].Root, wantRoot)
	}
	// No receipt -> pending.
	if a.Status != anchor.StatusPending {
		t.Errorf("no receipt: Status = %q, want pending", a.Status)
	}
}

func TestAnchorSpans_Determinism(t *testing.T) {
	spans := []evidencespan.Span{
		evidenceSpan("span-1", "v1"),
		evidenceSpan("span-2", "v2"),
		evidenceSpan("span-3", "v3"),
		evidenceSpan("span-4", "v4"),
		evidenceSpan("span-5", "v5"), // odd count exercises promotion
	}

	p1, a1, err1 := ea.AnchorSpans(spans, fixedClock(epoch), "ct-log:oak", []byte("r"))
	p2, a2, err2 := ea.AnchorSpans(spans, fixedClock(epoch), "ct-log:oak", []byte("r"))
	if err1 != nil || err2 != nil {
		t.Fatalf("AnchorSpans errors: %v, %v", err1, err2)
	}
	if a1.Root != a2.Root {
		t.Errorf("same spans+clock produced different roots: %q vs %q", a1.Root, a2.Root)
	}
	if len(p1) != len(p2) {
		t.Fatalf("proof counts differ: %d vs %d", len(p1), len(p2))
	}
	for i := range p1 {
		if p1[i].SpanDigest != p2[i].SpanDigest {
			t.Errorf("proof[%d].SpanDigest differs across runs", i)
		}
		if p1[i].Root != p2[i].Root {
			t.Errorf("proof[%d].Root differs across runs", i)
		}
	}
}

// ---------------------------------------------------------------------------
// AnchorSpans — error paths
// ---------------------------------------------------------------------------

func TestAnchorSpans_Errors(t *testing.T) {
	good := evidenceSpan("span-1", "v")

	t.Run("empty spans", func(t *testing.T) {
		if _, _, err := ea.AnchorSpans(nil, fixedClock(epoch), "auth", nil); err == nil {
			t.Error("expected error for empty spans, got nil")
		}
	})

	t.Run("nil clock", func(t *testing.T) {
		if _, _, err := ea.AnchorSpans([]evidencespan.Span{good}, nil, "auth", nil); err == nil {
			t.Error("expected error for nil clock, got nil")
		}
	})

	t.Run("empty authority", func(t *testing.T) {
		if _, _, err := ea.AnchorSpans([]evidencespan.Span{good}, fixedClock(epoch), "", nil); err == nil {
			t.Error("expected error for empty authority, got nil")
		}
	})

	t.Run("unprovenanced span rejected", func(t *testing.T) {
		// A bare span passes Validate but fails ValidateAsEvidence (no provenance).
		bare := evidencespan.Span{ID: "x", EntityID: "e", Field: "f", Value: "v", Confidence: 0.5}
		if err := bare.Validate(); err != nil {
			t.Fatalf("precondition: bare span should pass Validate: %v", err)
		}
		if _, _, err := ea.AnchorSpans([]evidencespan.Span{bare}, fixedClock(epoch), "auth", nil); err == nil {
			t.Error("expected AnchorSpans to reject an unprovenanced span, got nil error")
		}
	})

	t.Run("one bad span fails whole batch", func(t *testing.T) {
		bare := evidencespan.Span{ID: "bad", EntityID: "e", Field: "f", Value: "v", Confidence: 0.5}
		batch := []evidencespan.Span{good, bare}
		if _, _, err := ea.AnchorSpans(batch, fixedClock(epoch), "auth", nil); err == nil {
			t.Error("expected error when one span in the batch is unprovenanced")
		}
	})
}

// ---------------------------------------------------------------------------
// Verify — tamper detection
// ---------------------------------------------------------------------------

func TestVerify_RoundTrip(t *testing.T) {
	spans := []evidencespan.Span{
		evidenceSpan("span-1", "v1"),
		evidenceSpan("span-2", "v2"),
	}
	proofs, _, err := ea.AnchorSpans(spans, fixedClock(epoch), "auth", nil)
	if err != nil {
		t.Fatalf("AnchorSpans: %v", err)
	}
	for i := range spans {
		if !ea.Verify(spans[i], proofs[i]) {
			t.Errorf("Verify(span %d) = false, want true", i)
		}
	}
}

func TestVerify_TamperedSpan_ReturnsFalse(t *testing.T) {
	spans := []evidencespan.Span{
		evidenceSpan("span-1", "v1"),
		evidenceSpan("span-2", "v2"),
	}
	proofs, _, err := ea.AnchorSpans(spans, fixedClock(epoch), "auth", nil)
	if err != nil {
		t.Fatalf("AnchorSpans: %v", err)
	}

	// Tamper the value after anchoring — the proof must no longer verify.
	tampered := spans[0]
	tampered.Value = "tampered-value"
	if ea.Verify(tampered, proofs[0]) {
		t.Error("Verify accepted a tampered span (Value changed)")
	}

	// Tamper the grade — span-aware proofs must catch a downgrade/upgrade.
	stronger := evidencegrade.Grade{Tier: evidencegrade.TierDirect, Confidence: 1.0}
	tampered2 := spans[1]
	tampered2.Grade = &stronger
	if ea.Verify(tampered2, proofs[1]) {
		t.Error("Verify accepted a span whose Grade was altered")
	}
}

func TestVerify_WrongSpanForProof_ReturnsFalse(t *testing.T) {
	spans := []evidencespan.Span{
		evidenceSpan("span-1", "v1"),
		evidenceSpan("span-2", "v2"),
	}
	proofs, _, err := ea.AnchorSpans(spans, fixedClock(epoch), "auth", nil)
	if err != nil {
		t.Fatalf("AnchorSpans: %v", err)
	}
	// span-2 against span-1's proof must fail.
	if ea.Verify(spans[1], proofs[0]) {
		t.Error("Verify accepted span-2 against span-1's proof")
	}
}

func TestVerify_TamperedRoot_ReturnsFalse(t *testing.T) {
	s := evidenceSpan("span-1", "v")
	proofs, _, err := ea.AnchorSpans([]evidencespan.Span{s, evidenceSpan("span-2", "w")}, fixedClock(epoch), "auth", nil)
	if err != nil {
		t.Fatalf("AnchorSpans: %v", err)
	}
	p := proofs[0]
	// Flip the root to a different valid-hex 32-byte value.
	p.Root = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef0"
	if ea.Verify(s, p) {
		t.Error("Verify accepted a proof with a wrong root")
	}
}

func TestVerify_TamperedSibling_ReturnsFalse(t *testing.T) {
	spans := []evidencespan.Span{evidenceSpan("span-1", "v"), evidenceSpan("span-2", "w")}
	proofs, _, err := ea.AnchorSpans(spans, fixedClock(epoch), "auth", nil)
	if err != nil {
		t.Fatalf("AnchorSpans: %v", err)
	}
	p := proofs[0]
	if len(p.InclusionProof.Siblings) == 0 {
		t.Skip("no siblings to tamper for this tree shape")
	}
	sib := make([]byte, len(p.InclusionProof.Siblings[0]))
	copy(sib, p.InclusionProof.Siblings[0])
	sib[0] ^= 0xFF
	p.InclusionProof.Siblings[0] = sib
	if ea.Verify(spans[0], p) {
		t.Error("Verify accepted a proof with a tampered Merkle sibling")
	}
}

// TestVerify_MalformedInputs_NoPanic ensures adversarial/corrupt proofs return
// false rather than panicking.
func TestVerify_MalformedInputs_NoPanic(t *testing.T) {
	s := evidenceSpan("span-1", "v")
	proofs, _, err := ea.AnchorSpans([]evidencespan.Span{s}, fixedClock(epoch), "auth", nil)
	if err != nil {
		t.Fatalf("AnchorSpans: %v", err)
	}
	valid := proofs[0]

	cases := map[string]ea.SpanProof{
		"non-hex digest": {
			SpanID:         valid.SpanID,
			SpanDigest:     "not-hex!!" + valid.SpanDigest[8:],
			InclusionProof: valid.InclusionProof,
			Root:           valid.Root,
			AnchoredAt:     valid.AnchoredAt,
		},
		"wrong digest length": {
			SpanID:         valid.SpanID,
			SpanDigest:     "abcd",
			InclusionProof: valid.InclusionProof,
			Root:           valid.Root,
			AnchoredAt:     valid.AnchoredAt,
		},
		"non-hex root": {
			SpanID:         valid.SpanID,
			SpanDigest:     valid.SpanDigest,
			InclusionProof: valid.InclusionProof,
			Root:           "zzzz",
			AnchoredAt:     valid.AnchoredAt,
		},
		"empty root": {
			SpanID:         valid.SpanID,
			SpanDigest:     valid.SpanDigest,
			InclusionProof: valid.InclusionProof,
			Root:           "",
			AnchoredAt:     valid.AnchoredAt,
		},
		"empty proof": {},
	}

	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			// Must not panic; must return false (the digest won't match anyway,
			// but the point is graceful rejection of malformed input).
			if ea.Verify(s, p) {
				t.Errorf("Verify accepted malformed proof case %q", name)
			}
		})
	}
}

// TestVerify_SpanDigestMustMatchSuppliedSpan guards the binding: even a
// structurally valid inclusion proof must be rejected if proof.SpanDigest does
// not equal CanonicalDigest(span). This stops a swapped-digest attack where an
// attacker presents a real proof for digest A alongside an unrelated span B.
func TestVerify_SpanDigestMustMatchSuppliedSpan(t *testing.T) {
	spanA := evidenceSpan("span-A", "vA")
	spanB := evidenceSpan("span-B", "vB")

	proofsA, _, err := ea.AnchorSpans([]evidencespan.Span{spanA}, fixedClock(epoch), "auth", nil)
	if err != nil {
		t.Fatalf("AnchorSpans: %v", err)
	}
	// proofsA[0] is a genuine proof for spanA's digest+root. Hand it spanB.
	if ea.Verify(spanB, proofsA[0]) {
		t.Error("Verify accepted spanB against spanA's genuine proof (digest binding broken)")
	}
}
