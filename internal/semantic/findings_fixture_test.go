package semantic

// findings_fixture_test.go — a network-free structural sanity check on the
// findings layer's eval fixtures (docs/design/findings-as-evidence.md §6
// item 5): LoadEvalLabels parses, the corpus splits into the expected
// section count, and every labeled rule's cite_substr actually appears
// within its own doc's full content — the exact "cite_substr spans a
// line-wrap boundary" class of fixture bug adr-rules/eval/RESULTS.md's own
// history section records catching the hard way. Unlike
// TestFindingsEval_Precision (eval_test.go), this test ALWAYS runs — it
// costs no tokens and needs no model, so a broken fixture fails fast in
// every `go test ./...`, not only when someone remembers to spend the
// eval's real tokens.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindingsEvalFixture_LabelsLoadAndCiteSubstrsResolve(t *testing.T) {
	dir := packEvalDir(t, LayerFindings)

	labels, err := LoadEvalLabels(dir)
	if err != nil {
		t.Fatalf("LoadEvalLabels: %v", err)
	}
	if len(labels.Docs) == 0 {
		t.Fatal("labels.Docs is empty")
	}
	if len(labels.Probes) == 0 {
		t.Fatal("labels.Probes is empty")
	}
	const wantRules = 6 // one per class this fixture exercises
	if len(labels.Rules) != wantRules {
		t.Fatalf("labels.Rules = %d, want %d (docs/design/findings-as-evidence.md §6 item 5's own fixture shape)", len(labels.Rules), wantRules)
	}

	docContent := map[string]string{}
	for _, name := range labels.Docs {
		data, rerr := os.ReadFile(filepath.Join(dir, name))
		if rerr != nil {
			t.Fatalf("read fixture doc %s: %v", name, rerr)
		}
		docContent[name] = string(data)
	}

	for _, r := range labels.Rules {
		found := false
		for _, content := range docContent {
			if strings.Contains(content, r.CiteSubstr) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("rule %q's cite_substr %q does not appear verbatim in any fixture doc — likely a line-wrap boundary bug (see adr-rules/eval/RESULTS.md's own history note)", r.Key, r.CiteSubstr)
		}
		if len(r.Governs) == 0 {
			t.Errorf("rule %q governs no probe", r.Key)
		}
		for _, g := range r.Governs {
			if !containsStrFixture(labels.Probes, g) {
				t.Errorf("rule %q governs %q, which is not in labels.Probes", r.Key, g)
			}
		}
	}

	// Every section's heading (WA-101..WA-108) must actually split out —
	// SplitMarkdownSections is the same engine the real compiler uses, so
	// this also proves the fixture doc is well-formed Markdown for it.
	doc := docContent[labels.Docs[0]]
	sections := SplitMarkdownSections(labels.Docs[0], []byte(doc))
	const wantSections = 8 // 6 real findings + 2 traps
	nonEmpty := 0
	for _, sec := range sections {
		if strings.TrimSpace(sec.Text) != "" {
			nonEmpty++
		}
	}
	if nonEmpty != wantSections {
		t.Errorf("split into %d non-empty section(s), want %d (6 labeled findings + 2 traps)", nonEmpty, wantSections)
	}
}

func containsStrFixture(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
