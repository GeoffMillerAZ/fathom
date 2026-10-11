package receipt

// sarif_test.go — AnchorRun (EVX-2, EVX-3, EVX-5): a SARIF run's provenance
// and each finding's excerpt fingerprint, from the anchor core.

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

func planLines(n int, at int, cited string) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		if i == at {
			b.WriteString(cited + "\n")
			continue
		}
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.String()
}

func TestAnchorRun_ProvenanceAndFingerprints(t *testing.T) {
	r := newFixtureRepo(t)
	r.write("docs/plan.md", planLines(20, 10, "- [x] the claim {id: X-1}"))
	r.write("deploy/secret.yaml", "apiVersion: v1\nkind: Secret\n")
	head := r.commitAll("a")

	results := []verify.SARIFResult{
		{RuleID: "test_reaches", File: "docs/plan.md", Line: 10, Message: "m"},
		{RuleID: "test_reaches", File: filepath.Join(r.dir, "docs", "plan.md"), Line: 10, Message: "absolute"},
		{RuleID: "not-computed", File: "docs/plan.md", Message: "no line"},
		{RuleID: "x", File: "/elsewhere/a.md", Line: 1, Message: "outside"},
		{RuleID: "x", File: "deploy/secret.yaml", Line: 1, Message: "secret"},
		{RuleID: "x", File: "pkg/dir", Message: "a package directory"},
	}
	nodes := []core.SymbolNode{{Name: "Secret/db", Kind: core.KindK8sWorkload, FilePath: "deploy/secret.yaml", Signature: "v1"}}
	prov, out := AnchorRun(context.Background(), Run{Dir: filepath.Join(r.dir, "docs"), Nodes: nodes, RepositoryURI: "https://example.invalid/r.git"}, results)

	if prov.Commit != head || !prov.Clean || prov.ExtractorEpoch != fusion.ExtractorEpoch || prov.Producer.Name != "fathom" || prov.Producer.Version == "" {
		t.Fatalf("provenance = %+v, want commit %s, clean, epoch %s, producer fathom", prov, head, fusion.ExtractorEpoch)
	}
	if prov.RepositoryURI != "https://example.invalid/r.git" {
		t.Errorf("RepositoryURI = %q, want the configured one passed through", prov.RepositoryURI)
	}
	want := sha256Hex([]byte("- [x] the claim {id: X-1}\n"))
	if out[0].Fingerprint != want {
		t.Errorf("fingerprint = %q, want the cited line's excerpt sha256 %s", out[0].Fingerprint, want)
	}
	if out[1].File != "docs/plan.md" || out[1].Fingerprint != want {
		t.Errorf("an absolute path inside the work tree: %+v, want docs/plan.md and its fingerprint", out[1])
	}
	if out[2].Fingerprint != "" {
		t.Errorf("a result with no line carries a fingerprint %q: a whole-file digest moves with every edit", out[2].Fingerprint)
	}
	if out[3].File != "/elsewhere/a.md" || out[3].Fingerprint != "" {
		t.Errorf("a path outside the work tree: %+v, want it unchanged and unanchored", out[3])
	}
	if out[4].Fingerprint != "" {
		t.Errorf("a secret-bearing file's result carries a digest %q (ADR-065)", out[4].Fingerprint)
	}
	if results[0].Fingerprint != "" {
		t.Error("AnchorRun wrote into the caller's slice")
	}

	// An edited work tree: the run is not clean, and the edited file's
	// result loses its fingerprint.
	r.write("docs/plan.md", planLines(20, 10, "- [x] the claim, edited {id: X-1}"))
	prov, out = AnchorRun(context.Background(), Run{Dir: r.dir}, results[:3])
	if prov.Clean || out[0].Fingerprint != "" {
		t.Errorf("edited work tree: clean %v, fingerprint %q; want not clean, none", prov.Clean, out[0].Fingerprint)
	}

	// The index's stored fingerprint, when it is the commit's, is clean.
	stored := fusion.FingerprintHash([]byte(planLines(20, 10, "- [x] the claim {id: X-1}")))
	prov, out = AnchorRun(context.Background(), Run{Dir: r.dir, Stored: func(p string) (string, bool) { return stored, p == "docs/plan.md" }}, results[:1])
	if !prov.Clean || out[0].Fingerprint != want {
		t.Errorf("stored fingerprint at the commit: clean %v, fingerprint %q", prov.Clean, out[0].Fingerprint)
	}
}

func TestAnchorRun_NotARepository(t *testing.T) {
	results := []verify.SARIFResult{{RuleID: "x", File: "a.md", Line: 1}}
	prov, out := AnchorRun(context.Background(), Run{Dir: t.TempDir()}, results)
	if prov.Commit != "" || prov.Clean || out[0].Fingerprint != "" || prov.ExtractorEpoch != fusion.ExtractorEpoch {
		t.Fatalf("outside a repository: %+v %+v, want no commit, not clean, no fingerprint, the epoch still named", prov, out)
	}
}

// TestAnchorRun_FingerprintSurvivesLineShift is EVX-5 end to end: the same
// finding, rendered as SARIF at two commits 40 lines apart, carries the same
// partialFingerprints["fathomExcerpt/v1"] while its startLine moves.
func TestAnchorRun_FingerprintSurvivesLineShift(t *testing.T) {
	r := newFixtureRepo(t)
	cited := "- [x] a claimed row {id: X-9}"
	r.write("plan.md", planLines(12, 5, cited))
	r.commitAll("a")
	render := func(line int) (string, int) {
		prov, out := AnchorRun(context.Background(), Run{Dir: r.dir}, []verify.SARIFResult{{RuleID: "test_reaches", File: "plan.md", Line: line, Message: "m"}})
		var doc struct {
			Runs []struct {
				Results []struct {
					PartialFingerprints map[string]string `json:"partialFingerprints"`
					Locations           []struct {
						PhysicalLocation struct {
							Region struct {
								StartLine int `json:"startLine"`
							} `json:"region"`
						} `json:"physicalLocation"`
					} `json:"locations"`
				} `json:"results"`
			} `json:"runs"`
		}
		if err := json.Unmarshal(verify.RenderSARIFRun("fathom verify", prov, out), &doc); err != nil {
			t.Fatal(err)
		}
		res := doc.Runs[0].Results[0]
		return res.PartialFingerprints[verify.ExcerptFingerprintKey], res.Locations[0].PhysicalLocation.Region.StartLine
	}
	fa, la := render(5)
	r.write("plan.md", planLines(52, 45, cited))
	r.commitAll("b: 40 lines above the finding")
	fb, lb := render(45)
	if fa == "" || fa != fb || la == lb {
		t.Fatalf("fingerprint %q at line %d, %q at line %d: want equal fingerprints at different lines", fa, la, fb, lb)
	}
}
