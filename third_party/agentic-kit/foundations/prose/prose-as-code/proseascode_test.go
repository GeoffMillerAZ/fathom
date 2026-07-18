package proseascode_test

import (
	"testing"

	pac "github.com/geoffmilleraz/agentic-kit/foundations/prose/prose-as-code"
)

// validSegment returns a well-formed ProseSegment.
func validSegment() pac.ProseSegment {
	return pac.ProseSegment{
		ID:           "seg-001",
		Text:         "The payments service processes 10 000 requests per second.",
		EvidenceRefs: []string{"span-perf-001"},
	}
}

// validBlock returns a well-formed ProseBlock.
func validBlock() pac.ProseBlock {
	return pac.ProseBlock{
		Segments: []pac.ProseSegment{validSegment()},
	}
}

// TestProseSegment_ValidateHappyPath ensures a well-formed segment passes.
func TestProseSegment_ValidateHappyPath(t *testing.T) {
	if err := validSegment().Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// TestProseSegment_NoEvidenceRefs ensures empty EvidenceRefs is allowed.
func TestProseSegment_NoEvidenceRefs(t *testing.T) {
	s := validSegment()
	s.EvidenceRefs = nil
	if err := s.Validate(); err != nil {
		t.Fatalf("nil EvidenceRefs should be allowed: %v", err)
	}
}

// TestProseSegment_EmptyEvidenceRef ensures empty strings inside EvidenceRefs
// are rejected.
func TestProseSegment_EmptyEvidenceRef(t *testing.T) {
	s := validSegment()
	s.EvidenceRefs = []string{"span-001", ""}
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for empty EvidenceRefs element, got nil")
	}
}

// TestProseSegment_RequiredFields ensures ID and Text are required.
func TestProseSegment_RequiredFields(t *testing.T) {
	cases := []struct {
		name  string
		mutFn func(*pac.ProseSegment)
	}{
		{"empty-ID", func(s *pac.ProseSegment) { s.ID = "" }},
		{"empty-Text", func(s *pac.ProseSegment) { s.Text = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := validSegment()
			tc.mutFn(&s)
			if err := s.Validate(); err == nil {
				t.Errorf("%s: expected validation error, got nil", tc.name)
			}
		})
	}
}

// TestProseBlock_ValidateHappyPath ensures a well-formed block passes.
func TestProseBlock_ValidateHappyPath(t *testing.T) {
	if err := validBlock().Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// TestProseBlock_EmptySegments ensures a block with no segments is rejected.
func TestProseBlock_EmptySegments(t *testing.T) {
	b := pac.ProseBlock{Segments: nil}
	if err := b.Validate(); err == nil {
		t.Fatal("expected error for empty Segments, got nil")
	}
}

// TestProseBlock_DuplicateIDs ensures duplicate segment IDs are rejected.
func TestProseBlock_DuplicateIDs(t *testing.T) {
	seg := validSegment()
	b := pac.ProseBlock{Segments: []pac.ProseSegment{seg, seg}}
	if err := b.Validate(); err == nil {
		t.Fatal("expected error for duplicate segment IDs, got nil")
	}
}

// TestProseBlock_PropagatesSegmentError ensures segment validation errors
// bubble up through block validation.
func TestProseBlock_PropagatesSegmentError(t *testing.T) {
	seg := validSegment()
	seg.Text = ""
	b := pac.ProseBlock{Segments: []pac.ProseSegment{seg}}
	if err := b.Validate(); err == nil {
		t.Fatal("expected error from invalid segment, got nil")
	}
}

// TestProseBlock_MultipleSegments ensures a block with distinct valid segments
// passes.
func TestProseBlock_MultipleSegments(t *testing.T) {
	s1 := pac.ProseSegment{ID: "s1", Text: "First sentence.", EvidenceRefs: []string{"e1"}}
	s2 := pac.ProseSegment{ID: "s2", Text: "Second sentence.", EvidenceRefs: nil}
	b := pac.ProseBlock{Segments: []pac.ProseSegment{s1, s2}}
	if err := b.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
