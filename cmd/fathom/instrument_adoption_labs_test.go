package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/labs"
)

// labsFixtureRoot is a project root with the two committed cards copied
// under calibration/labs.d; the shipped inventory is exactly their surfaces.
func labsFixtureRoot(t *testing.T) (string, []string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "calibration", "labs.d")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"lab-find-symbol", "lab-search"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "calibration", "labs.d", id+".yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, id+".yaml"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, []string{"mcp:find_symbol", "mcp:search"}
}

func writeLabsFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAdoptionLabsSection(t *testing.T) {
	root, shipped := labsFixtureRoot(t)
	writeLabsFile(t, root, "docs/bench/labs-ledger.jsonl", strings.Join([]string{
		`{"run_id":"r1","task_id":"t1","arm":"replay","lab":"lab-search","evaluation":{"criterion_id":"c","verdict":"fail","reason":"x"}}`,
		`{"run_id":"r1","task_id":"t2","arm":"replay","lab":"lab-find-symbol","evaluation":{"criterion_id":"c","verdict":"pass","reason":"x"}}`,
		`{"run_id":"r2","task_id":"t3","arm":"plugin","lab":"lab-search","evaluation":{"criterion_id":"c","verdict":"indeterminate","reason":"x"}}`,
	}, "\n")+"\n")
	writeLabsFile(t, root, "calibration/labs.d/lab-search/cases/a.json", `{"split":"train"}`)
	writeLabsFile(t, root, "calibration/labs.d/lab-search/cases/b.json", `{"split":"train"}`)
	writeLabsFile(t, root, "calibration/labs.d/lab-search/cases/c.json", `{"split":"confirm"}`)
	// The legacy log says O3 is open; its per-entry file wins and says fixed.
	writeLabsFile(t, root, ".fathom/observations.md", "# log\n\n### O1 — one\n- **Status:** open\n- **Status:** Fixed in abc\n\n### O3 — three\n- **Status:** open\n")
	writeLabsFile(t, root, ".fathom/observations/O3.md", "### O3 — three\n- **Status:** fixed; see test\n- **Lab:** lab-search/literal-log-line\n")
	writeLabsFile(t, root, ".fathom/observations/O12.md", "### O12 — twelve\n- **Status:** open\n")

	got := adoptionLabsSection(root, shipped, nil)
	if len(got.Cards) != 2 || got.Cards[0] != (AdoptionLabCard{ID: "lab-find-symbol", Surfaces: 1}) || got.Cards[1].ID != "lab-search" {
		t.Errorf("cards = %+v", got.Cards)
	}
	if got.CardError != "" || got.ClosureGap != "" || got.LedgerError != "" || got.CaseError != "" || got.ObservationError != "" {
		t.Errorf("unexpected errors: %+v", got)
	}
	wantNewest := []labs.LabResultRow{
		{RunID: "r1", TaskID: "t2", Arm: "replay", Lab: "lab-find-symbol", Verdict: "pass"},
		{RunID: "r2", TaskID: "t3", Arm: "plugin", Lab: "lab-search", Verdict: "indeterminate"},
	}
	if !reflect.DeepEqual(got.NewestResults, wantNewest) {
		t.Errorf("newest = %+v", got.NewestResults)
	}
	if want := []labs.LabCaseCount{{Lab: "lab-search", Train: 2, Confirm: 1}}; !reflect.DeepEqual(got.Cases, want) {
		t.Errorf("cases = %+v", got.Cases)
	}
	if got.Fixed.Count != 2 || !reflect.DeepEqual(got.Fixed.IDs, []string{"O1", "O3"}) || got.Fixed.FixProof != "not measured (LAB-24 unbuilt)" {
		t.Errorf("fixed = %+v", got.Fixed)
	}
	if got.Unlinked.Count != 2 || !reflect.DeepEqual(got.Unlinked.IDs, []string{"O1", "O12"}) || !reflect.DeepEqual(got.Unlinked.ByStatus, map[string]int{"fixed": 1, "open": 1}) {
		t.Errorf("unlinked = %+v", got.Unlinked)
	}

	var text bytes.Buffer
	renderAdoptionLabs(&text, &got)
	for _, want := range []string{
		"labs (calibration/labs.d):",
		"cards: 2",
		"lab-find-symbol  surfaces=1",
		"closure: every shipped surface has a card or a no-lab entry",
		"lab-find-symbol  run=r1 task=t2 arm=replay verdict=pass",
		"lab-search  run=r2 task=t3 arm=plugin verdict=indeterminate",
		"lab-search  train=2 select=0 confirm=1",
		"fixed observations: 2 (fix proof: not measured (LAB-24 unbuilt))",
		"O1, O3",
		"observations without a Lab link: 2 (fixed=1, open=1)",
		"O1, O12",
	} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("text lacks %q:\n%s", want, text.String())
		}
	}

	b, err := json.Marshal(AdoptionReport{Labs: &got})
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Labs struct {
			Fixed struct {
				Count    int      `json:"count"`
				IDs      []string `json:"ids"`
				FixProof string   `json:"fix_proof"`
			} `json:"fixed_observations"`
			Newest []map[string]string `json:"newest_results"`
		} `json:"labs"`
	}
	if err := json.Unmarshal(b, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.Labs.Fixed.Count != 2 || wire.Labs.Fixed.FixProof != "not measured (LAB-24 unbuilt)" || len(wire.Labs.Newest) != 2 || wire.Labs.Newest[1]["verdict"] != "indeterminate" {
		t.Errorf("json = %s", b)
	}
}

// More than twenty ids: the text shows the first twenty and the rest's count;
// JSON carries every id.
func TestAdoptionLabsSectionTruncatesIDs(t *testing.T) {
	root := t.TempDir()
	var log strings.Builder
	for i := 1; i <= 23; i++ {
		fmt.Fprintf(&log, "### O%d — x\n- **Status:** fixed\n\n", i)
	}
	writeLabsFile(t, root, ".fathom/observations.md", log.String())
	got := adoptionLabsSection(root, nil, nil)
	if got.Fixed.Count != 23 || len(got.Fixed.IDs) != 23 || got.Fixed.IDs[22] != "O23" {
		t.Fatalf("fixed = %+v", got.Fixed)
	}
	var text bytes.Buffer
	renderAdoptionLabs(&text, &got)
	if !strings.Contains(text.String(), "O19, O20 +3 more") || strings.Contains(text.String(), "O21") {
		t.Errorf("text:\n%s", text.String())
	}
}

func TestAdoptionLabsSectionEmptyRoot(t *testing.T) {
	isolateVerifyLedger(t)
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := cmdInstrumentAdoption([]string{"--adoption", "--json", "--root", root, "--call-log", filepath.Join(root, "absent.jsonl"), "--url", "http://127.0.0.1:1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var wire struct {
		Labs map[string]json.RawMessage `json:"labs"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &wire); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"cards": "[]", "card_error": `""`, "closure_gap": `""`, "newest_results": "[]", "ledger_error": `""`, "cases": "[]", "case_error": `""`,
		"ledger_present": "false", "cases_present": "false",
		"fixed_observations":    `{"count":0,"ids":[],"fix_proof":"not measured (LAB-24 unbuilt)"}`,
		"unlinked_observations": `{"count":0,"ids":[],"by_status":{}}`,
	}
	for k, v := range want {
		var compact bytes.Buffer
		_ = json.Compact(&compact, wire.Labs[k])
		if got := compact.String(); got != v {
			t.Errorf("labs.%s = %s, want %s", k, got, v)
		}
	}

	stdout.Reset()
	if code := cmdInstrumentAdoption([]string{"--adoption", "--root", root, "--call-log", filepath.Join(root, "absent.jsonl"), "--url", "http://127.0.0.1:1"}, &stdout, &stderr); code != 0 {
		t.Fatalf("text exit %d", code)
	}
	for _, want := range []string{"labs (calibration/labs.d):", "cards: 0 (no calibration/labs.d)",
		"not measured: docs/bench/labs-ledger.jsonl absent", "not measured: calibration/labs.d/<lab>/cases absent", "fixed observations: 0", "observations without a Lab link: 0"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("text lacks %q:\n%s", want, stdout.String())
		}
	}
}

func TestAdoptionLabsSectionNamesClosureGap(t *testing.T) {
	root, shipped := labsFixtureRoot(t)
	shipped = append(shipped, "mcp:future_probe")
	want := labs.CheckLabClosure(filepath.Join(root, "calibration", "labs.d"), shipped)
	if want == nil {
		t.Fatal("fixture has no closure gap")
	}
	got := adoptionLabsSection(root, shipped, nil)
	if got.ClosureGap != want.Error() || len(got.Cards) != 2 {
		t.Fatalf("closure gap = %q, want %q verbatim", got.ClosureGap, want.Error())
	}
	var text bytes.Buffer
	renderAdoptionLabs(&text, &got)
	if !strings.Contains(text.String(), "closure GAP: "+want.Error()) {
		t.Errorf("text does not print the gap:\n%s", text.String())
	}

	// A card that does not load is printed, never an empty card list.
	writeLabsFile(t, root, "calibration/labs.d/lab-broken.yaml", "id: lab-broken\n")
	got = adoptionLabsSection(root, shipped, nil)
	if got.CardError == "" || len(got.Cards) != 0 {
		t.Fatalf("card error = %q, cards = %v", got.CardError, got.Cards)
	}
	text.Reset()
	renderAdoptionLabs(&text, &got)
	if !strings.Contains(text.String(), "CARDS UNREADABLE: "+got.CardError) {
		t.Errorf("text does not print the card error:\n%s", text.String())
	}
}

// The console's Operate page renders without the labs section: it never
// loads cards or walks the observations per render.
func TestAdoptionLabsSectionSkippedOnConsole(t *testing.T) {
	f := newConsoleFixture(t)
	resetOperateCache()
	env := f.pageJSON(t, "/operate")
	raw, ok := env.Sections[operateAdoptVerb]
	if !ok {
		t.Fatalf("operate sections lack %q", operateAdoptVerb)
	}
	var adoption map[string]json.RawMessage
	if err := json.Unmarshal(raw, &adoption); err != nil {
		t.Fatal(err)
	}
	if got, ok := adoption["labs"]; !ok || string(got) != "null" {
		t.Errorf("console adoption labs = %s (present %v), want null", got, ok)
	}
}

// The committed tree renders every seed card with no closure gap.
func TestAdoptionLabsSectionCommittedRoot(t *testing.T) {
	shipped, err := shippedLabSurfaces()
	got := adoptionLabsSection(filepath.Join("..", ".."), shipped, err)
	if len(got.Cards) != 16 || got.CardError != "" || got.ClosureGap != "" {
		t.Fatalf("committed root: %d cards, card error %q, closure gap %q", len(got.Cards), got.CardError, got.ClosureGap)
	}
	if got.LedgerError != "" || got.CaseError != "" {
		t.Errorf("committed root: ledger %q, cases %q", got.LedgerError, got.CaseError)
	}
}

// An empty-but-present ledger and cases directory read as measured zero, and
// the JSON says present.
func TestAdoptionLabsSectionPresentButEmpty(t *testing.T) {
	root, shipped := labsFixtureRoot(t)
	writeLabsFile(t, root, "docs/bench/labs-ledger.jsonl", "")
	if err := os.MkdirAll(filepath.Join(root, "calibration", "labs.d", "lab-search", "cases"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := adoptionLabsSection(root, shipped, nil)
	if !got.LedgerPresent || !got.CasesPresent {
		t.Errorf("present flags = %v %v, want true true", got.LedgerPresent, got.CasesPresent)
	}
	var text bytes.Buffer
	renderAdoptionLabs(&text, &got)
	for _, want := range []string{"newest ledger row per lab (docs/bench/labs-ledger.jsonl): 0", "cases by split: 0 lab(s) with cases"} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("text lacks %q:\n%s", want, text.String())
		}
	}
	if strings.Contains(text.String(), "not measured: docs/bench") || strings.Contains(text.String(), "cases absent") {
		t.Errorf("present inputs read as not measured:\n%s", text.String())
	}
}

// A per-entry file wins on a duplicate id even with no Status line: the id
// reads "unknown", never the legacy log's status.
func TestObservationStatusesPerEntryWithoutStatusWins(t *testing.T) {
	root := t.TempDir()
	writeLabsFile(t, root, ".fathom/observations.md", "### O5 — five\n- **Status:** fixed in abc\n\n### O6 — six\n- **Status:** open\n")
	writeLabsFile(t, root, ".fathom/observations/O5.md", "### O5 — five\nno status recorded\n")
	got, err := observationStatuses(root)
	if err != nil {
		t.Fatal(err)
	}
	if got[5] != "unknown" || got[6] != "open" {
		t.Errorf("statuses = %v, want O5 unknown and O6 open", got)
	}
	sec := adoptionLabsSection(root, nil, nil)
	if sec.Fixed.Count != 0 {
		t.Errorf("fixed = %+v, want none", sec.Fixed)
	}
}

// An unreadable entry file is named by observationStatuses, as the merged
// reader names it (I38).
func TestObservationStatusesNamesUnreadableEntry(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".fathom", "observations"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A dangling symlink is a listed file that cannot be read.
	if err := os.Symlink(filepath.Join(root, "nowhere"), filepath.Join(root, ".fathom", "observations", "O9.md")); err != nil {
		t.Skip("symlink unavailable")
	}
	if _, _, err := readObservations(root); err == nil || !strings.Contains(err.Error(), "O9.md") {
		t.Fatalf("readObservations err = %v", err)
	}
	if _, err := observationStatuses(root); err == nil || !strings.Contains(err.Error(), "O9.md") {
		t.Errorf("observationStatuses err = %v, want it to name O9.md", err)
	}
}
