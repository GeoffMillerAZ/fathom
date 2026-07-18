package truthbridge_test

import (
	"testing"

	vectorstore "github.com/geoffmilleraz/agentic-kit/foundations/embedding/vector-store"
	evidencegrade "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-grade"
	evidencespan "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-span"
	truthbridge "github.com/geoffmilleraz/agentic-kit/foundations/retrieval/truth-bridge"
)

// fakeHash returns a deterministic 64-char lowercase hex string for use in tests.
func fakeHash(seed byte) string {
	buf := make([]byte, 32)
	for i := range buf {
		buf[i] = seed
	}
	const hexChars = "0123456789abcdef"
	out := make([]byte, 64)
	for i, b := range buf {
		out[i*2] = hexChars[b>>4]
		out[i*2+1] = hexChars[b&0xf]
	}
	return string(out)
}

// span returns a minimal evidencespan.Span for use in tests.
func span(sourceID, locKind, locValue string) evidencespan.Span {
	return evidencespan.Span{
		ID:       "span-001",
		EntityID: "entity-001",
		Field:    "owner",
		Value:    "alice",
		SourceID: sourceID,
		Locator: evidencespan.Locator{
			Kind:  locKind,
			Value: locValue,
		},
	}
}

func TestBind_PopulatesLink(t *testing.T) {
	doc := vectorstore.Entry{ID: "chunk-001"}
	sp := span("src-001", "line", "42")
	grade := evidencegrade.Grade{Tier: evidencegrade.TierDocumentary, Confidence: 0.9}
	hash := fakeHash(0xab)
	anchorRef := fakeHash(0xcd)

	lc := truthbridge.Bind(doc, sp, grade, hash, anchorRef, "some text content")

	if lc.Link.ChunkID != "chunk-001" {
		t.Errorf("ChunkID = %q; want %q", lc.Link.ChunkID, "chunk-001")
	}
	if lc.Link.GradeTier != evidencegrade.TierDocumentary {
		t.Errorf("GradeTier = %q; want %q", lc.Link.GradeTier, evidencegrade.TierDocumentary)
	}
	if lc.Link.SourceID != "src-001" {
		t.Errorf("SourceID = %q; want %q", lc.Link.SourceID, "src-001")
	}
	if lc.Link.SpanLocatorKind != "line" {
		t.Errorf("SpanLocatorKind = %q; want %q", lc.Link.SpanLocatorKind, "line")
	}
	if lc.Link.SpanLocatorValue != "42" {
		t.Errorf("SpanLocatorValue = %q; want %q", lc.Link.SpanLocatorValue, "42")
	}
	if lc.Link.ContentHash != hash {
		t.Errorf("ContentHash = %q; want %q", lc.Link.ContentHash, hash)
	}
	if lc.Link.AnchorProofRef != anchorRef {
		t.Errorf("AnchorProofRef = %q; want %q", lc.Link.AnchorProofRef, anchorRef)
	}
	if lc.Text != "some text content" {
		t.Errorf("Text = %q; want %q", lc.Text, "some text content")
	}
}

func TestBind_EmptyLocator(t *testing.T) {
	doc := vectorstore.Entry{ID: "chunk-002"}
	sp := span("src-002", "", "")
	grade := evidencegrade.Grade{Tier: evidencegrade.TierInferred, Confidence: 0.3}

	lc := truthbridge.Bind(doc, sp, grade, fakeHash(0x00), "", "text")

	if lc.Link.SpanLocatorKind != "" {
		t.Errorf("expected empty locator kind, got %q", lc.Link.SpanLocatorKind)
	}
	if lc.Link.AnchorProofRef != "" {
		t.Errorf("expected empty anchor ref, got %q", lc.Link.AnchorProofRef)
	}
}

func TestGraded_KnownLinks(t *testing.T) {
	results := []vectorstore.Scored{
		{Entry: vectorstore.Entry{ID: "c1"}, Score: 0.95},
		{Entry: vectorstore.Entry{ID: "c2"}, Score: 0.80},
	}
	links := []truthbridge.ChunkLink{
		{ChunkID: "c1", GradeTier: evidencegrade.TierDirect, SourceID: "src-a", ContentHash: fakeHash(0x01)},
		{ChunkID: "c2", GradeTier: evidencegrade.TierCircumstantial, SourceID: "src-b", ContentHash: fakeHash(0x02)},
	}

	got := truthbridge.Graded(results, links)

	if len(got) != 2 {
		t.Fatalf("len(got) = %d; want 2", len(got))
	}

	if got[0].ChunkID != "c1" {
		t.Errorf("[0].ChunkID = %q; want c1", got[0].ChunkID)
	}
	if got[0].Grade != truthbridge.GradeDirect {
		t.Errorf("[0].Grade = %q; want %q", got[0].Grade, truthbridge.GradeDirect)
	}
	if !got[0].Graded {
		t.Errorf("[0].Graded = false; want true")
	}
	if got[0].SourceID != "src-a" {
		t.Errorf("[0].SourceID = %q; want src-a", got[0].SourceID)
	}

	if got[1].Grade != truthbridge.GradeCircumstantial {
		t.Errorf("[1].Grade = %q; want %q", got[1].Grade, truthbridge.GradeCircumstantial)
	}
	if !got[1].Graded {
		t.Errorf("[1].Graded = false; want true")
	}
}

// TestGraded_UnknownDefault verifies the honest-default rule: chunks with no
// link are NEVER silently graded — they get GradeUnknown and Graded=false.
func TestGraded_UnknownDefault(t *testing.T) {
	results := []vectorstore.Scored{
		{Entry: vectorstore.Entry{ID: "unlinked"}, Score: 0.5},
	}

	got := truthbridge.Graded(results, nil)

	if len(got) != 1 {
		t.Fatalf("len(got) = %d; want 1", len(got))
	}
	if got[0].Grade != truthbridge.GradeUnknown {
		t.Errorf("Grade = %q; want %q (honest default)", got[0].Grade, truthbridge.GradeUnknown)
	}
	if got[0].Graded {
		t.Errorf("Graded = true for unlinked chunk; want false")
	}
	if got[0].SourceID != "" {
		t.Errorf("SourceID = %q; want empty for unlinked chunk", got[0].SourceID)
	}
}

// TestGraded_MixedKnownUnknown verifies order preservation and per-element correctness.
func TestGraded_MixedKnownUnknown(t *testing.T) {
	results := []vectorstore.Scored{
		{Entry: vectorstore.Entry{ID: "known"}, Score: 0.9},
		{Entry: vectorstore.Entry{ID: "orphan"}, Score: 0.6},
		{Entry: vectorstore.Entry{ID: "also-known"}, Score: 0.4},
	}
	links := []truthbridge.ChunkLink{
		{ChunkID: "known", GradeTier: evidencegrade.TierTestimonial, SourceID: "src-t", ContentHash: fakeHash(0x10)},
		{ChunkID: "also-known", GradeTier: evidencegrade.TierInferred, SourceID: "src-i", ContentHash: fakeHash(0x11)},
	}

	got := truthbridge.Graded(results, links)

	if len(got) != 3 {
		t.Fatalf("len(got) = %d; want 3", len(got))
	}
	if got[0].Grade != truthbridge.GradeTestimonial || !got[0].Graded {
		t.Errorf("[0] want testimonial/graded, got %q/%v", got[0].Grade, got[0].Graded)
	}
	if got[1].Grade != truthbridge.GradeUnknown || got[1].Graded {
		t.Errorf("[1] want unknown/ungraded, got %q/%v", got[1].Grade, got[1].Graded)
	}
	if got[2].Grade != truthbridge.GradeInferred || !got[2].Graded {
		t.Errorf("[2] want inferred/graded, got %q/%v", got[2].Grade, got[2].Graded)
	}
}

// TestGraded_EmptyResults verifies zero-len input produces zero-len output.
func TestGraded_EmptyResults(t *testing.T) {
	got := truthbridge.Graded(nil, nil)
	if len(got) != 0 {
		t.Errorf("expected empty output, got %d", len(got))
	}
}

// TestGraded_InvariantGradedConsistent verifies that Graded == (Grade != GradeUnknown)
// for every output element, regardless of input.
func TestGraded_InvariantGradedConsistent(t *testing.T) {
	results := []vectorstore.Scored{
		{Entry: vectorstore.Entry{ID: "a"}, Score: 0.9},
		{Entry: vectorstore.Entry{ID: "b"}, Score: 0.5},
		{Entry: vectorstore.Entry{ID: "c"}, Score: 0.1},
	}
	links := []truthbridge.ChunkLink{
		{ChunkID: "a", GradeTier: evidencegrade.TierDocumentary, ContentHash: fakeHash(0x20)},
	}

	got := truthbridge.Graded(results, links)
	for i, r := range got {
		wantGraded := r.Grade != truthbridge.GradeUnknown
		if r.Graded != wantGraded {
			t.Errorf("[%d] Graded(%v) != (Grade != GradeUnknown) for grade=%q", i, r.Graded, r.Grade)
		}
	}
}

// TestGraded_ScorePreserved verifies the score from the vectorstore is echoed unchanged.
func TestGraded_ScorePreserved(t *testing.T) {
	results := []vectorstore.Scored{
		{Entry: vectorstore.Entry{ID: "x"}, Score: 0.123456789},
	}
	got := truthbridge.Graded(results, nil)
	if got[0].Score != 0.123456789 {
		t.Errorf("Score = %v; want 0.123456789", got[0].Score)
	}
}

// TestGraded_AnchorProofRefPreserved verifies anchor proof ref is propagated.
func TestGraded_AnchorProofRefPreserved(t *testing.T) {
	proofRef := fakeHash(0x55)
	results := []vectorstore.Scored{
		{Entry: vectorstore.Entry{ID: "anchored"}, Score: 0.77},
	}
	links := []truthbridge.ChunkLink{
		{
			ChunkID:        "anchored",
			GradeTier:      evidencegrade.TierDirect,
			ContentHash:    fakeHash(0x33),
			AnchorProofRef: proofRef,
		},
	}

	got := truthbridge.Graded(results, links)
	if got[0].AnchorProofRef != proofRef {
		t.Errorf("AnchorProofRef = %q; want %q", got[0].AnchorProofRef, proofRef)
	}
}
