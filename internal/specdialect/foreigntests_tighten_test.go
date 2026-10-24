package specdialect

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

type foreignCertificateEvidence struct {
	reaches bool
	live    bool
}

func (foreignCertificateEvidence) FindSymbol(name string) []core.SymbolNode {
	return []core.SymbolNode{{Name: name, FilePath: "check.go", Tier: core.TierVerified}}
}
func (e foreignCertificateEvidence) TestReaches(name string) []string {
	if name == "Other" || e.reaches {
		return []string{"TestAlpha"}
	}
	return nil
}
func (foreignCertificateEvidence) Reachable(_, _ string) bool           { return false }
func (foreignCertificateEvidence) Contradictions() []core.Contradiction { return nil }
func (foreignCertificateEvidence) SpecImplementors(string) []string     { return nil }
func (e foreignCertificateEvidence) CertificationsByTest(names []string) map[string][]verify.Certification {
	out := map[string][]verify.Certification{}
	for _, name := range names {
		if name == "TestAlpha" && e.live {
			out[name] = []verify.Certification{{Test: name, Strength: "asserts", State: "live", Scope: []string{"check.go"}}}
		}
	}
	return out
}

func TestForeignTestRefsOnlyTightenRows(t *testing.T) {
	for _, tc := range []struct {
		name, annotation, label string
		reaches, live           bool
		status                  verify.CriterionStatus
	}{
		{"same file certificate cannot replace reachability", "", "present-but-untested", false, true, verify.Fail},
		{"derived certificate cannot replace explicit certificate", ", certified_by: TestExisting", "certificate-absent", true, true, verify.Fail},
		{"reachability and live certificate pass", "", "verified", true, true, verify.Pass},
		{"reachability alone fails", "", "certificate-absent", true, false, verify.Fail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte("- [x] 1. Run `TestAlpha` {symbol: Check" + tc.annotation + "}\n")
			p, _, err := ParseContent(Kiro(), filepath.Join(t.TempDir(), ".kiro/specs/example/tasks.md"), body)
			if err != nil {
				t.Fatal(err)
			}
			for _, plan := range []verify.Plan{p, verify.ParseMarkdownPlan("", Synthesize(p))} {
				v := verify.VerifyPlan(plan, foreignCertificateEvidence{reaches: tc.reaches, live: tc.live}).Tasks[0]
				if v.Status != tc.status || v.Label != tc.label {
					t.Errorf("status/label = %s/%s, want %s/%s; criteria: %+v", v.Status, v.Label, tc.status, tc.label, v.Criteria)
				}
			}
		})
	}
}

func TestForeignTestRefsPreserveExplicitOrder(t *testing.T) {
	for _, d := range []SpecDialect{Kiro(), SpecKit(), TaskMaster()} {
		t.Run(d.ID(), func(t *testing.T) {
			body := []byte("- [x] 1. Run `TestAlpha` {symbol: Check, certified_by: \"TestZulu,TestExisting\"}\n")
			path := ".kiro/specs/example/tasks.md"
			if d.ID() == "speckit" {
				path = "specs/001-example/tasks.md"
				body = []byte(strings.ReplaceAll(string(body), "1.", "T001"))
			} else if d.ID() == "taskmaster" {
				path = ".taskmaster/tasks/tasks.json"
				body = []byte(`{"tasks":[{"id":1,"title":"Check {symbol: Check, certified_by: \"TestZulu,TestExisting\"}","testStrategy":"Run ` + "`TestAlpha`" + `","status":"done"}]}`)
			}
			p, _, err := ParseContent(d, filepath.Join(t.TempDir(), path), body)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Tasks) != 1 {
				t.Fatalf("tasks = %d, want 1", len(p.Tasks))
			}
			if got := p.Tasks[0].Expect.CertifiedBy; !reflect.DeepEqual(got, []string{"TestZulu", "TestExisting"}) {
				t.Fatalf("explicit certificate order changed: %v", got)
			}
			if wire := string(Synthesize(p)); strings.Contains(wire, "derived_tests:") {
				t.Fatalf("explicit certificate row gained derived names: %s", wire)
			}
		})
	}
}

func TestForeignTestRefsRunAlternativesAndAnchors(t *testing.T) {
	for _, tc := range []struct {
		text string
		want []string
	}{
		{"go test -run 'TestA|TestB'", []string{"TestA", "TestB"}},
		{"go test -run=^TestX$", []string{"TestX"}},
		{"go test -run '^TestX$'", []string{"TestX"}},
		{"go test -run '^TestA$/case|^TestB$'.", []string{"TestA", "TestB"}},
	} {
		t.Run(tc.text, func(t *testing.T) {
			if got := mapForeignTestRefs("", tc.text, false); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("test names = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestForeignTestRefsKiroStopsAtHeading(t *testing.T) {
	for _, heading := range []string{"# Notes", "## Notes", "  ### Notes"} {
		t.Run(heading, func(t *testing.T) {
			body := []byte("- [x] 1. Check {symbol: Check}\n  Tests: `TestAlpha`\n\n" + heading + "\nRun `TestEverything`\n")
			p, _, err := ParseContent(Kiro(), filepath.Join(t.TempDir(), ".kiro/specs/example/tasks.md"), body)
			if err != nil {
				t.Fatal(err)
			}
			wire := string(Synthesize(p))
			if !strings.Contains(wire, "derived_tests: TestAlpha") || strings.Contains(wire, "TestEverything") {
				t.Fatalf("heading prose became task evidence: %s", wire)
			}
		})
	}
}
