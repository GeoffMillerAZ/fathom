package labs

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeLabObservation(t *testing.T, root, name, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestParseObservationLabLinks(t *testing.T) {
	root := t.TempDir()
	writeLabObservation(t, root, ".fathom/observations.md", "# Legacy\n### O2 — old\n- **Status:** fixed\n- **Lab:** lab-search/near-miss\n### O3 — unlinked\n- **Status:** open\n")
	writeLabObservation(t, root, ".fathom/observations/O12.md", "### O12 — fresh\n- **Lab:** lab-find-symbol/find-definition\n- **Status:** open\n")
	writeLabObservation(t, root, ".fathom/observations/O4.md", "### O4 — unlinked\n- **Status:** open\n")
	writeLabObservation(t, root, ".fathom/observations/README.md", "- **Lab:** deliberately not an entry\n")
	got, err := ParseObservationLabLinks(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []ObservationLabLink{
		{Observation: "O2", Lab: "lab-search", Case: "near-miss", File: ".fathom/observations.md", Line: 4},
		{Observation: "O3", File: ".fathom/observations.md", Line: 5},
		{Observation: "O12", Lab: "lab-find-symbol", Case: "find-definition", File: ".fathom/observations/O12.md", Line: 2},
		{Observation: "O4", File: ".fathom/observations/O4.md", Line: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	empty, err := ParseObservationLabLinks(t.TempDir())
	if err != nil || len(empty) != 0 {
		t.Fatalf("absent observations: %+v, %v", empty, err)
	}
}

func TestParseObservationLabLinksMalformed(t *testing.T) {
	for _, bad := range []string{"", "lab-search", "lab-search/", "/case", "lab-search/case/extra", "lab-search/two words", "../case", "lab-search/case\n- **Lab:** lab-search/second"} {
		t.Run(strings.ReplaceAll(bad, "/", "_"), func(t *testing.T) {
			root := t.TempDir()
			writeLabObservation(t, root, ".fathom/observations/O1.md", "### O1 — bad\n- **Lab:** "+bad+"\n")
			_, err := ParseObservationLabLinks(root)
			if err == nil || !strings.Contains(err.Error(), ".fathom/observations/O1.md:") {
				t.Fatalf("missing file/line diagnostic: %v", err)
			}
		})
	}
	root := t.TempDir()
	writeLabObservation(t, root, ".fathom/observations.md", "### O1 — malformed\n- **Lab:** broken\n### O2 — malformed\n- **Lab:** broken\n")
	_, err := ParseObservationLabLinks(root)
	if err == nil || !strings.Contains(err.Error(), ".fathom/observations.md:2:") || !strings.Contains(err.Error(), ".fathom/observations.md:4:") {
		t.Fatalf("legacy diagnostics: %v", err)
	}
}

func TestParseObservationLabLinksFencesAndOrphans(t *testing.T) {
	root := t.TempDir()
	writeLabObservation(t, root, ".fathom/observations/O1.md", "### O1 — linked\n```md\n- **Lab:** example/bogus\n### O8 — quoted\n```\n~~~\n- **Lab:** another/bogus\n~~~\n- **Lab:** lab-search/real\n")
	got, err := ParseObservationLabLinks(root)
	if err != nil || len(got) != 1 || got[0].Case != "real" || got[0].Line != 9 {
		t.Fatalf("fenced example: %+v, %v", got, err)
	}
	writeLabObservation(t, root, ".fathom/observations.md", "- **Lab:** lab-search/orphan\n")
	_, err = ParseObservationLabLinks(root)
	if err == nil || !strings.Contains(err.Error(), ".fathom/observations.md:1:") {
		t.Fatalf("orphan link: %v", err)
	}
}

func TestParseObservationLabLinksReadFailure(t *testing.T) {
	root := t.TempDir()
	// A directory at the legacy file is a deterministic read failure, even as root.
	if err := os.MkdirAll(filepath.Join(root, ".fathom", "observations.md"), 0755); err != nil {
		t.Fatal(err)
	}
	_, err := ParseObservationLabLinks(root)
	if err == nil || !strings.Contains(err.Error(), ".fathom/observations.md") {
		t.Fatalf("read failure hidden: %v", err)
	}
}
