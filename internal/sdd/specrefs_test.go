package sdd

import (
	"testing"
)

func TestExtractFileRefs_DefaultPattern(t *testing.T) {
	files := map[string]string{
		"auth/login.go":  "// implements [US-042] and also references M18.2\nfunc Login() {}\n// see doc 31 for details\n",
		"billing/pay.go": "// [EP-013] payment processing\nfunc Pay() { /* (M17.4) */ }\n",
		"util/fmt.go":    "// no spec refs here\nfunc Format() {}\n",
	}

	refs, scanned, err := ExtractFileRefs(files, "", "")
	if err != nil {
		t.Fatalf("ExtractFileRefs: %v", err)
	}
	if scanned != 3 {
		t.Errorf("files_scanned = %d, want 3", scanned)
	}
	if len(refs) == 0 {
		t.Fatal("expected at least one ref")
	}

	// Build a lookup: specID → []file:line strings for quick assertions.
	bySpec := make(map[string][]SpecRef)
	for _, r := range refs {
		bySpec[r.SpecID] = append(bySpec[r.SpecID], r)
	}

	// [US-042] must be normalised (brackets stripped).
	if _, ok := bySpec["US-042"]; !ok {
		t.Errorf("US-042 missing; got keys: %v", specKeys(bySpec))
	}
	// M18.2 milestone.
	if _, ok := bySpec["M18.2"]; !ok {
		t.Errorf("M18.2 missing; got keys: %v", specKeys(bySpec))
	}
	// doc 31 — must collapse extra space.
	if _, ok := bySpec["doc 31"]; !ok {
		t.Errorf("doc 31 missing; got keys: %v", specKeys(bySpec))
	}
	// [EP-013].
	if _, ok := bySpec["EP-013"]; !ok {
		t.Errorf("EP-013 missing; got keys: %v", specKeys(bySpec))
	}
	// M17.4 inside parens — regex \b...\b still matches inside (M17.4).
	if _, ok := bySpec["M17.4"]; !ok {
		t.Errorf("M17.4 missing; got keys: %v", specKeys(bySpec))
	}
}

func TestExtractFileRefs_LowercaseM(t *testing.T) {
	files := map[string]string{
		"a.go": "// m18.2 is the milestone\n",
	}
	refs, _, err := ExtractFileRefs(files, "", "")
	if err != nil {
		t.Fatalf("ExtractFileRefs: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d: %v", len(refs), refs)
	}
	if refs[0].SpecID != "M18.2" {
		t.Errorf("SpecID = %q, want M18.2", refs[0].SpecID)
	}
}

func TestExtractFileRefs_DocCollapse(t *testing.T) {
	files := map[string]string{
		"b.go": "// see doc  31 for contract\n",
	}
	refs, _, err := ExtractFileRefs(files, "", "")
	if err != nil {
		t.Fatalf("ExtractFileRefs: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d: %v", len(refs), refs)
	}
	if refs[0].SpecID != "doc 31" {
		t.Errorf("SpecID = %q, want \"doc 31\"", refs[0].SpecID)
	}
}

func TestExtractFileRefs_SpecFilter(t *testing.T) {
	files := map[string]string{
		"a.go": "[US-042] and M18.2 and [EP-001]\n",
	}
	refs, _, err := ExtractFileRefs(files, "", "US-042")
	if err != nil {
		t.Fatalf("ExtractFileRefs: %v", err)
	}
	for _, r := range refs {
		if r.SpecID != "US-042" {
			t.Errorf("unexpected SpecID %q with filter US-042", r.SpecID)
		}
	}
	if len(refs) == 0 {
		t.Error("expected US-042 ref")
	}
}

func TestExtractFileRefs_DeterministicOrder(t *testing.T) {
	files := map[string]string{
		"z.go": "M18.2 and [US-001]\n",
		"a.go": "M18.2 line1\nM18.2 line2\n",
	}
	refs, _, err := ExtractFileRefs(files, "", "")
	if err != nil {
		t.Fatalf("ExtractFileRefs: %v", err)
	}
	// Verify sort: SpecID asc, File asc, Line asc.
	for i := 1; i < len(refs); i++ {
		a, b := refs[i-1], refs[i]
		if a.SpecID > b.SpecID {
			t.Errorf("refs[%d].SpecID %q > refs[%d].SpecID %q", i-1, a.SpecID, i, b.SpecID)
		}
		if a.SpecID == b.SpecID && a.File > b.File {
			t.Errorf("same SpecID: refs[%d].File %q > refs[%d].File %q", i-1, a.File, i, b.File)
		}
		if a.SpecID == b.SpecID && a.File == b.File && a.Line > b.Line {
			t.Errorf("same SpecID+File: refs[%d].Line %d > refs[%d].Line %d", i-1, a.Line, i, b.Line)
		}
	}
}

func TestExtractFileRefs_EmptyFiles(t *testing.T) {
	refs, scanned, err := ExtractFileRefs(map[string]string{}, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scanned != 0 {
		t.Errorf("files_scanned = %d, want 0", scanned)
	}
	if refs == nil {
		t.Error("refs should be non-nil (empty slice)")
	}
	if len(refs) != 0 {
		t.Errorf("len(refs) = %d, want 0", len(refs))
	}
}

func TestExtractFileRefs_InvalidPattern(t *testing.T) {
	_, _, err := ExtractFileRefs(map[string]string{"a.go": "x"}, "[invalid(", "")
	if err == nil {
		t.Error("expected error for invalid pattern")
	}
}

func TestExtractFileRefs_LineNumbers(t *testing.T) {
	files := map[string]string{
		"a.go": "line 1\nline 2 with [US-001]\nline 3\n",
	}
	refs, _, err := ExtractFileRefs(files, "", "")
	if err != nil {
		t.Fatalf("ExtractFileRefs: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].Line != 2 {
		t.Errorf("Line = %d, want 2", refs[0].Line)
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func specKeys(m map[string][]SpecRef) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
