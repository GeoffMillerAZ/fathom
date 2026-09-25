package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCmdBrief_HonestSections pins OB-7 and BR-1: the brief always carries
// its four questions plus the plain-language opener and the work order,
// absences are NAMED under "Not measured", and with real ledger rows the
// counts carry denominators. Nothing is sent anywhere: --print writes only
// to stdout, the default writes only a local file.
func TestCmdBrief_HonestSections(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FATHOM_ARM_STATE", filepath.Join(dir, "arm.json"))
	t.Setenv("FATHOM_NARRATE_STATE", filepath.Join(dir, "narrate.json"))
	t.Setenv("FATHOM_SESSION_LEDGER", filepath.Join(dir, "ledger.jsonl"))
	resetObsCache()

	t.Run("empty state names every absence", func(t *testing.T) {
		var out, errb bytes.Buffer
		if code := cmdBrief([]string{"--url", "http://127.0.0.1:1", "--print"}, &out, &errb); code != 0 {
			t.Fatalf("exit %d, stderr: %s", code, errb.String())
		}
		md := out.String()
		for _, want := range []string{
			"## The short version",
			"## 1 — How much is fathom being used?",
			"## 2 — What did fathom prevent?",
			"## 3 — Did it save tokens?",
			"## 4 — Where does the agent still flounder?",
			"## 5 — Is the knowledge compounding?",
			"## Could fathom help more if it were configured differently?",
			"## Not measured",
			"law 12", "law 14",
		} {
			if !strings.Contains(md, want) {
				t.Errorf("brief missing %q", want)
			}
		}
		if !strings.Contains(md, "unknown") && !strings.Contains(md, "not readable") {
			t.Error("empty-state brief must name its absent instruments")
		}
		// BR-1: the work order is never an empty gesture — with no OFF-arm
		// sessions recorded there is always at least the counterfactual
		// pilot to offer, and it must arrive with its command.
		if !strings.Contains(md, "fathom arm off --sessions") {
			t.Error("the work order must name the pilot command when no OFF arm exists")
		}
	})

	t.Run("ledger rows carry denominators", func(t *testing.T) {
		lines := `{"schema_version":2,"session_id":"a","arm":"on","at":"2026-08-01T10:00:00Z","tokens_total":100,"tokens_note":"","verify_catches":2}` + "\n"
		if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), []byte(lines), 0o644); err != nil {
			t.Fatal(err)
		}
		var out, errb bytes.Buffer
		if code := cmdBrief([]string{"--url", "http://127.0.0.1:1", "--print"}, &out, &errb); code != 0 {
			t.Fatalf("exit %d", code)
		}
		md := out.String()
		if !strings.Contains(md, "caught before they were believed: 2") || !strings.Contains(md, "1 of 1") {
			t.Errorf("brief must carry the count WITH its denominator; got:\n%s", md)
		}
	})

	t.Run("default mode writes a local file and says so", func(t *testing.T) {
		outDir := filepath.Join(dir, "briefs")
		var out, errb bytes.Buffer
		if code := cmdBrief([]string{"--url", "http://127.0.0.1:1", "--out", outDir}, &out, &errb); code != 0 {
			t.Fatalf("exit %d", code)
		}
		if !strings.Contains(out.String(), "sharing is a human verb") {
			t.Error("brief must state that nothing was sent")
		}
		matches, _ := filepath.Glob(filepath.Join(outDir, "brief-*.md"))
		if len(matches) != 1 {
			t.Fatalf("want exactly one brief file, got %v", matches)
		}
	})
}

// The "Not measured" section accounts for every value the brief shows that
// is not a plain measurement: a partial open-defect count is a lower bound,
// so the section may not claim "every instrument above reported a real
// value" beside it (the survey's contradiction, still live after F0).
func TestBriefMD_NotMeasuredNamesPartialObservations(t *testing.T) {
	one, three, twelve := 1, 3, 12
	obs := 4
	m := observatoryModel{
		Services: []obsServiceRow{
			{fleetServiceRow: fleetServiceRow{Service: "reports", OpenObservations: &obs}},
			{fleetServiceRow: fleetServiceRow{Service: "silent"}},
		},
		Impact: obsImpact{VerifyCatchesTotal: &one, DiscoveryTotal: &three, FirstEditMedian: &twelve},
	}
	md := renderBriefMD(m)
	notMeasured := md[strings.Index(md, "## Not measured"):]
	if !strings.Contains(md, "≥ 4") {
		t.Fatalf("section 4 must keep the lower bound:\n%s", md)
	}
	if strings.Contains(notMeasured, "every instrument above reported a real value") {
		t.Errorf("a partial count above is not a real value:\n%s", notMeasured)
	}
	if !strings.Contains(notMeasured, "1 of 2 repos reporting") {
		t.Errorf("the Not measured section must name the partial open-defect count:\n%s", notMeasured)
	}
	// With every repo reporting, the claim is true again.
	m.Services[1].OpenObservations = &obs
	if nm := renderBriefMD(m); !strings.Contains(nm[strings.Index(nm, "## Not measured"):], "every instrument above reported a real value") {
		t.Errorf("complete coverage is a real value")
	}
}
