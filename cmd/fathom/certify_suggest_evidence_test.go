package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/decide"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// certifyEvidenceFixture is one test file per evidence shape (JF-9).
var certifyEvidenceFixture = map[string]string{
	"smoke_test.go": `package pkg

import "testing"

func TestSmokeOnly(t *testing.T) {
	Run()
}
`,
	"asserts_test.go": `package pkg

import "testing"

func TestAssertsDirectly(t *testing.T) {
	if got := F(); got != 1 {
		t.Fatalf("F() = %d, want 1", got)
	}
}

func TestAssertsThroughAHelper(t *testing.T) {
	checkF(t, F())
}
`,
	"golden_test.go": `package pkg

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestReadsAGolden(t *testing.T) {
	want, _ := os.ReadFile(filepath.Join("testdata", "render.golden"))
	if !bytes.Equal(Render(), want) {
		t.Error("render differs from the golden")
	}
}
`,
	"property_test.go": `package pkg

import (
	"testing"
	"testing/quick"
)

func TestHoldsForEveryInput(t *testing.T) {
	if err := quick.Check(func(n int) bool { return Abs(n) >= 0 }, nil); err != nil {
		t.Error(err)
	}
}
`,
}

// TestCertifySuggestNeverOffersALevelTheEvidenceRulesOut pins JF-9
// (docs/design/judgment-foundations.md §7): certify suggest computes, per
// test, the strength levels its evidence permits (an assertion call or the
// test's t handed to a helper permits asserts, a golden file read permits
// golden, a property library import permits property, a mutation report
// permits mutation-killed; smoke and conformance stay, no evidence rules
// them out) and offers the model only those. Mock decider only: no model.
func TestCertifySuggestNeverOffersALevelTheEvidenceRulesOut(t *testing.T) {
	root := t.TempDir()
	for name, src := range certifyEvidenceFixture {
		if err := os.WriteFile(filepath.Join(root, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	report := filepath.Join(t.TempDir(), "mutation.json")
	if err := os.WriteFile(report, []byte(`{"mutants":[{"file":"pkg.go","line":3,"killed":true}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	want := map[string][]string{
		"TestSmokeOnly":             {"smoke", "conformance"},
		"TestAssertsDirectly":       {"smoke", "asserts", "conformance"},
		"TestAssertsThroughAHelper": {"smoke", "asserts", "conformance"},
		"TestReadsAGolden":          {"smoke", "asserts", "golden", "conformance"},
		"TestHoldsForEveryInput":    {"smoke", "asserts", "property", "conformance"},
	}
	offered := func(req decision.Request) map[string][]string {
		out := map[string][]string{}
		for _, q := range req.Questions {
			name, ok := strings.CutSuffix(q.ID, ":strength")
			if !ok {
				continue
			}
			for _, o := range q.Options {
				out[name] = append(out[name], o.Name)
			}
		}
		return out
	}

	t.Run("the request offers exactly what the evidence permits", func(t *testing.T) {
		for _, mutation := range []bool{false, true} {
			var items []certifySuggestItem
			for name, src := range certifyEvidenceFixture {
				for test := range want {
					if n, ok := certifyFixtureNode(name, src, test); ok {
						items = append(items, certifySuggestItemFor(root, n, mutation))
					}
				}
			}
			got := offered(buildCertifySuggestRequest(items))
			for test, levels := range want {
				exp := slices.Clone(levels)
				if mutation {
					exp = append(exp, "mutation-killed")
				}
				if !sameSet(got[test], exp) {
					t.Errorf("mutation report %v: %s offered %v, want %v", mutation, test, got[test], exp)
				}
			}
		}
	})

	t.Run("fathom certify suggest offers the model only those levels", func(t *testing.T) {
		service := "evidence-svc"
		storePath := filepath.Join(t.TempDir(), "s.sqlite")
		var ingestOut, ingestErr bytes.Buffer
		if code := run([]string{"ingest", "--service", service, "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
			t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
		}
		now := func() time.Time { return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) }
		for _, mutation := range []bool{false, true} {
			mock := decision.NewMockDecider("mock", decision.MockEntry{Response: decision.Response{Provider: "mock", Model: "mock-1",
				Answers: map[string]decision.Answer{
					// A level the evidence rules out is never written, even
					// if a provider answers it anyway.
					"TestSmokeOnly:strength": {ID: "TestSmokeOnly:strength", Kind: decision.KindChoice, Choice: "mutation-killed", Confidence: 0.99},
					"TestSmokeOnly:asserts":  {ID: "TestSmokeOnly:asserts", Kind: decision.KindPredicate, Noul: 0.99, Confidence: 0.99},
				}}})
			ports := &decide.Ports{Decider: mock, Cfg: config.Decide{Sites: []string{decide.SiteCertifySuggest}}, Recorder: decide.NewRecorder(&bytes.Buffer{}, now, "test")}
			args := []string{"--service", service, "--store", storePath, "--out", filepath.Join(t.TempDir(), "s.jsonl")}
			t.Setenv(mutationReportEnvVar, "")
			if mutation {
				args = append(args, "--mutation", report)
			}
			var out, errBuf bytes.Buffer
			if code := runCertifySuggest(args, ports, now, &out, &errBuf); code != 0 {
				t.Fatalf("certify suggest exit = %d, stderr: %s", code, errBuf.String())
			}
			calls := mock.Calls()
			if len(calls) != 1 {
				t.Fatalf("Decide called %d times, want 1", len(calls))
			}
			got := offered(calls[0])
			for test, levels := range want {
				exp := slices.Clone(levels)
				if mutation {
					exp = append(exp, "mutation-killed")
				}
				if !sameSet(got[test], exp) {
					t.Errorf("mutation report %v: %s offered %v, want %v", mutation, test, got[test], exp)
				}
			}
			if !mutation && strings.Contains(out.String(), "suggested: TestSmokeOnly") {
				t.Errorf("a strength the evidence rules out was suggested:\n%s", out.String())
			}
		}
	})
}

// certifyFixtureNode is the symbol an ingest would give test in src: its
// func line through its closing brace.
func certifyFixtureNode(file, src, test string) (core.SymbolNode, bool) {
	start := strings.Index(src, "func "+test+"(")
	if start < 0 {
		return core.SymbolNode{}, false
	}
	end := start + strings.Index(src[start:], "\n}\n") + 1
	return core.SymbolNode{Name: test, Kind: core.KindGoFunc, FilePath: file,
		Line: strings.Count(src[:start], "\n") + 1, EndLine: strings.Count(src[:end], "\n") + 1}, true
}

func sameSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}
