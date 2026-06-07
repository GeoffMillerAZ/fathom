package adr_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/adr"
)

// ─── Parse — ID extraction ────────────────────────────────────────────────────

// ID extracted from filename "ADR-001-use-sqlite.md" when no heading carries one.
func TestParse_IDFromFilename(t *testing.T) {
	content := []byte("# Use SQLite\n\nStatus: accepted\n")
	got, err := adr.Parse("docs/adr/ADR-001-use-sqlite.md", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "ADR-001" {
		t.Errorf("ID = %q, want %q", got.ID, "ADR-001")
	}
}

// ID extracted from a heading "# ADR-002: Adopt gRPC".
func TestParse_IDFromHeading(t *testing.T) {
	content := []byte("# ADR-002: Adopt gRPC\n\nStatus: proposed\n")
	got, err := adr.Parse("docs/adr/0002.md", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "ADR-002" {
		t.Errorf("ID = %q, want %q", got.ID, "ADR-002")
	}
}

// Filename wins when BOTH filename and heading contain an ADR-NNN marker.
func TestParse_FilenameWinsOverHeading(t *testing.T) {
	content := []byte("# ADR-099: Something\n\nStatus: accepted\n")
	got, err := adr.Parse("docs/adr/ADR-001-title.md", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "ADR-001" {
		t.Errorf("ID = %q, want ADR-001", got.ID)
	}
}

// When neither filename nor heading carry ADR-NNN, Parse still succeeds (ID
// left empty — will be flagged by Conformance).
func TestParse_NoID(t *testing.T) {
	content := []byte("# Some doc\n\nStatus: accepted\n")
	got, err := adr.Parse("docs/misc.md", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "" {
		t.Errorf("expected empty ID, got %q", got.ID)
	}
}

// ─── Parse — Title extraction ─────────────────────────────────────────────────

// Title is taken from the first "# " heading, stripping any leading ADR-NNN prefix.
func TestParse_TitleFromHeading(t *testing.T) {
	content := []byte("# Use SQLite for the evidence store\n\nStatus: accepted\n")
	got, err := adr.Parse("ADR-001-use-sqlite.md", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Title != "Use SQLite for the evidence store" {
		t.Errorf("Title = %q", got.Title)
	}
}

// "# ADR-002: Adopt gRPC" → Title = "Adopt gRPC".
func TestParse_TitleStripsADRPrefix(t *testing.T) {
	content := []byte("# ADR-002: Adopt gRPC\n\nStatus: proposed\n")
	got, err := adr.Parse("0002.md", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Title != "Adopt gRPC" {
		t.Errorf("Title = %q, want %q", got.Title, "Adopt gRPC")
	}
}

// No heading → Title is empty.
func TestParse_NoTitle(t *testing.T) {
	content := []byte("Status: accepted\n\nBody text.\n")
	got, err := adr.Parse("ADR-003.md", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Title != "" {
		t.Errorf("expected empty Title, got %q", got.Title)
	}
}

// ─── Parse — Status extraction ────────────────────────────────────────────────

func TestParse_StatusAccepted(t *testing.T) {
	content := []byte("# ADR-001: Use SQLite\n\nStatus: accepted\n")
	got, err := adr.Parse("ADR-001.md", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != adr.StatusAccepted {
		t.Errorf("Status = %q, want accepted", got.Status)
	}
}

func TestParse_StatusProposed(t *testing.T) {
	content := []byte("# ADR-002: gRPC\n\nStatus: proposed\n")
	got, err := adr.Parse("ADR-002.md", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != adr.StatusProposed {
		t.Errorf("Status = %q, want proposed", got.Status)
	}
}

func TestParse_StatusSuperseded(t *testing.T) {
	content := []byte("# ADR-003: Old Approach\n\nStatus: superseded\n")
	got, err := adr.Parse("ADR-003.md", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != adr.StatusSuperseded {
		t.Errorf("Status = %q, want superseded", got.Status)
	}
}

func TestParse_StatusDeprecated(t *testing.T) {
	content := []byte("# ADR-004: Deprecated Feature\n\nStatus: deprecated\n")
	got, err := adr.Parse("ADR-004.md", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != adr.StatusDeprecated {
		t.Errorf("Status = %q, want deprecated", got.Status)
	}
}

func TestParse_StatusRejected(t *testing.T) {
	content := []byte("# ADR-005: Rejected Idea\n\nStatus: rejected\n")
	got, err := adr.Parse("ADR-005.md", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != adr.StatusRejected {
		t.Errorf("Status = %q, want rejected", got.Status)
	}
}

// Status may appear as a "## Status" section heading followed by the value.
func TestParse_StatusSectionHeading(t *testing.T) {
	content := []byte("# ADR-006: Section-style status\n\n## Status\n\naccepted\n\n## Context\n\nSome context.\n")
	got, err := adr.Parse("ADR-006.md", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != adr.StatusAccepted {
		t.Errorf("Status = %q, want accepted", got.Status)
	}
}

// Case-insensitive status value.
func TestParse_StatusCaseInsensitive(t *testing.T) {
	content := []byte("# ADR-007: Mixed case\n\nStatus: Accepted\n")
	got, err := adr.Parse("ADR-007.md", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != adr.StatusAccepted {
		t.Errorf("Status = %q, want accepted", got.Status)
	}
}

// Missing status → StatusUnknown (not an error; Conformance will flag it).
func TestParse_StatusMissing(t *testing.T) {
	content := []byte("# ADR-008: No status\n\nSome body.\n")
	got, err := adr.Parse("ADR-008.md", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != adr.StatusUnknown {
		t.Errorf("Status = %q, want unknown", got.Status)
	}
}

// Path is stored on the ADR.
func TestParse_PathPreserved(t *testing.T) {
	content := []byte("# ADR-009: Path test\n\nStatus: accepted\n")
	got, err := adr.Parse("internal/adr/ADR-009.md", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Path != "internal/adr/ADR-009.md" {
		t.Errorf("Path = %q", got.Path)
	}
}

// ─── Collect ──────────────────────────────────────────────────────────────────

func TestCollect_Sorted(t *testing.T) {
	files := map[string][]byte{
		"docs/ADR-003-thing.md":  []byte("# ADR-003: Thing\n\nStatus: accepted\n"),
		"docs/ADR-001-first.md":  []byte("# ADR-001: First\n\nStatus: accepted\n"),
		"docs/ADR-002-second.md": []byte("# ADR-002: Second\n\nStatus: proposed\n"),
	}
	got, err := adr.Collect(files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	ids := []string{got[0].ID, got[1].ID, got[2].ID}
	want := []string{"ADR-001", "ADR-002", "ADR-003"}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("got[%d].ID = %q, want %q", i, ids[i], want[i])
		}
	}
}

// Collect is deterministic: repeated calls with the same input return the same order.
func TestCollect_Deterministic(t *testing.T) {
	files := map[string][]byte{
		"docs/ADR-002.md": []byte("# ADR-002: B\n\nStatus: accepted\n"),
		"docs/ADR-001.md": []byte("# ADR-001: A\n\nStatus: accepted\n"),
		"docs/ADR-003.md": []byte("# ADR-003: C\n\nStatus: superseded\n"),
	}
	a, err := adr.Collect(files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := adr.Collect(files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(a) != len(b) {
		t.Fatalf("lengths differ: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			t.Errorf("index %d: %q vs %q", i, a[i].ID, b[i].ID)
		}
	}
}

// Collect with empty input returns empty slice (not nil error).
func TestCollect_Empty(t *testing.T) {
	got, err := adr.Collect(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

// ─── ActiveDecisions ──────────────────────────────────────────────────────────

func TestActiveDecisions_OnlyAccepted(t *testing.T) {
	adrs := []adr.ADR{
		{ID: "ADR-001", Status: adr.StatusAccepted},
		{ID: "ADR-002", Status: adr.StatusProposed},
		{ID: "ADR-003", Status: adr.StatusSuperseded},
		{ID: "ADR-004", Status: adr.StatusDeprecated},
		{ID: "ADR-005", Status: adr.StatusRejected},
		{ID: "ADR-006", Status: adr.StatusAccepted},
	}
	got := adr.ActiveDecisions(adrs)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2; got %v", len(got), got)
	}
	if got[0].ID != "ADR-001" || got[1].ID != "ADR-006" {
		t.Errorf("ActiveDecisions = %v", got)
	}
}

func TestActiveDecisions_Empty(t *testing.T) {
	got := adr.ActiveDecisions(nil)
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestActiveDecisions_NoneAccepted(t *testing.T) {
	adrs := []adr.ADR{
		{ID: "ADR-001", Status: adr.StatusProposed},
		{ID: "ADR-002", Status: adr.StatusRejected},
	}
	got := adr.ActiveDecisions(adrs)
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

// ─── Conformance ──────────────────────────────────────────────────────────────

func TestConformance_MissingTitle(t *testing.T) {
	adrs := []adr.ADR{
		{ID: "ADR-001", Title: "", Status: adr.StatusAccepted, Path: "ADR-001.md"},
	}
	issues := adr.Conformance(adrs)
	if !containsIssue(issues, "ADR-001", "title") {
		t.Errorf("expected missing-title issue for ADR-001; got %v", issues)
	}
}

func TestConformance_MissingStatus(t *testing.T) {
	adrs := []adr.ADR{
		{ID: "ADR-002", Title: "Some Title", Status: adr.StatusUnknown, Path: "ADR-002.md"},
	}
	issues := adr.Conformance(adrs)
	if !containsIssue(issues, "ADR-002", "status") {
		t.Errorf("expected missing-status issue for ADR-002; got %v", issues)
	}
}

func TestConformance_MissingID(t *testing.T) {
	adrs := []adr.ADR{
		{ID: "", Title: "Some Title", Status: adr.StatusAccepted, Path: "misc.md"},
	}
	issues := adr.Conformance(adrs)
	if !containsIssue(issues, "", "id") {
		t.Errorf("expected missing-id issue; got %v", issues)
	}
}

func TestConformance_MultipleIssues(t *testing.T) {
	adrs := []adr.ADR{
		{ID: "", Title: "", Status: adr.StatusUnknown, Path: "bad.md"},
	}
	issues := adr.Conformance(adrs)
	if len(issues) < 3 {
		t.Errorf("expected at least 3 issues (id, title, status), got %d: %v", len(issues), issues)
	}
}

func TestConformance_ValidADR(t *testing.T) {
	adrs := []adr.ADR{
		{ID: "ADR-001", Title: "Use SQLite", Status: adr.StatusAccepted, Path: "ADR-001.md"},
	}
	issues := adr.Conformance(adrs)
	if len(issues) != 0 {
		t.Errorf("expected no issues for valid ADR, got %v", issues)
	}
}

func TestConformance_Deterministic(t *testing.T) {
	adrs := []adr.ADR{
		{ID: "", Title: "", Status: adr.StatusUnknown, Path: "bad.md"},
		{ID: "ADR-001", Title: "Good", Status: adr.StatusAccepted, Path: "ADR-001.md"},
		{ID: "ADR-002", Title: "", Status: adr.StatusUnknown, Path: "ADR-002.md"},
	}
	a := adr.Conformance(adrs)
	b := adr.Conformance(adrs)
	if len(a) != len(b) {
		t.Fatalf("lengths differ: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].ADRID != b[i].ADRID || a[i].Field != b[i].Field {
			t.Errorf("index %d differs: %+v vs %+v", i, a[i], b[i])
		}
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// containsIssue reports whether issues contains an Issue for the given ADR ID
// with a Field containing fieldSubstr (case-insensitive).
func containsIssue(issues []adr.Issue, adrID, fieldSubstr string) bool {
	for _, iss := range issues {
		if iss.ADRID == adrID && containsSubstr(toLower(iss.Field), toLower(fieldSubstr)) {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}

func containsSubstr(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
