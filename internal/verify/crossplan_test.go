package verify

// crossplan_test.go — PC-10: needs: may cross plans; lint checks the shape,
// resolveCrossPlanNeed reads the sibling's authored row state.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCrossPlanNeed(t *testing.T) {
	cases := map[string]bool{
		"custodian.md#SD-2": true,
		"SD-2":              false, // bare id: same-plan
		"custodian.md#":     false,
		"#SD-2":             false,
		"custodian#SD-2":    false, // not a .md
		"a/b.md#SD-2":       false, // no directories
	}
	for ref, ok := range cases {
		if _, got := ParseCrossPlanNeed(ref); got != ok {
			t.Errorf("ParseCrossPlanNeed(%q) ok = %v, want %v", ref, got, ok)
		}
	}
}

func TestLintNeeds_CrossPlanTargetIsNotUnknown(t *testing.T) {
	md := "- [-] Panel rows {id: EP-44, disposition: deferred, reason: unbuilt, trigger: later, needs: custodian.md#SD-2}\n" +
		"- [ ] Bad {id: X-1, needs: custodian#SD-2}\n"
	findings := LintPlan(ParseMarkdownPlan("svc", []byte(md)))
	joined := strings.Join(findings, "\n")
	if strings.Contains(joined, "EP-44") {
		t.Errorf("a well-formed cross-plan target must not lint as unknown:\n%s", joined)
	}
	if !strings.Contains(joined, "X-1: needs malformed cross-plan target") {
		t.Errorf("a malformed cross-plan target must lint:\n%s", joined)
	}
}

func TestResolveCrossPlanNeed(t *testing.T) {
	dir := t.TempDir()
	sib := "# custodian\n- [ ] Panel {id: SD-2, symbol: renderSDD, package: cmd/fathom}\n- [x] Done {id: SD-1, symbol: appendVerifyLedger}\n"
	if err := os.WriteFile(filepath.Join(dir, "custodian.md"), []byte(sib), 0o644); err != nil {
		t.Fatal(err)
	}
	row, err := resolveCrossPlanNeed(dir, "custodian.md#SD-2")
	if err != nil || row.ID != "SD-2" || row.Claimed {
		t.Fatalf("resolve = %+v, %v; want the open SD-2 row", row, err)
	}
	if row, err := ResolveCrossPlanNeed(dir, "custodian.md#SD-1"); err != nil || !row.Claimed {
		t.Errorf("SD-1 must resolve claimed: %+v, %v", row, err)
	}
	if _, err := resolveCrossPlanNeed(dir, "custodian.md#SD-9"); err == nil || !strings.Contains(err.Error(), "no row") {
		t.Errorf("missing row must be named: %v", err)
	}
	if _, err := resolveCrossPlanNeed(dir, "nope.md#SD-2"); err == nil || !strings.Contains(err.Error(), "sibling plan nope.md") {
		t.Errorf("missing plan must be named: %v", err)
	}
	if _, err := resolveCrossPlanNeed(dir, "SD-2"); err == nil {
		t.Error("a bare id is not a cross-plan target")
	}
}
