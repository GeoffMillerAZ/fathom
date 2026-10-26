package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/labs"
)

func fileLabObservation(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := cmdObserve([]string{"--root", root, "--symptom", "lookup missed", "--tool", "find_symbol", "--lab", "lab-find-symbol/find-definition"}, &stdout, &stderr); code != 0 {
		t.Fatalf("observe: %d: %s", code, stderr.String())
	}
	content, ok, err := readObservations(root)
	if err != nil || !ok || !strings.Contains(content, "- **Lab:** lab-find-symbol/find-definition\n") {
		t.Fatalf("linked entry missing: %q, %v", content, err)
	}
	return root, content
}

func TestObserveLabWritesPointer(t *testing.T) {
	root, content := fileLabObservation(t)
	if strings.Contains(strings.ToLower(content), "**proof:") {
		t.Fatal("a link must never write a proof bullet")
	}
	links, err := labs.ParseObservationLabLinks(root)
	if err != nil || len(links) != 1 || links[0].Lab != "lab-find-symbol" || links[0].Case != "find-definition" {
		t.Fatalf("filed pointer: %+v, %v", links, err)
	}
	var stdout, stderr bytes.Buffer
	if code := cmdObserve([]string{"--root", root, "--symptom", "unlinked"}, &stdout, &stderr); code != 0 {
		t.Fatalf("observe unlinked: %d: %s", code, stderr.String())
	}
	b, err := os.ReadFile(filepath.Join(root, ".fathom", "observations", "O2.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "**Lab:") {
		t.Fatal("absent --lab wrote a link")
	}
}

func TestObserveLabRejectsMalformedBeforeWriting(t *testing.T) {
	for _, bad := range []string{"", "lab-search", "lab-search/", "lab-search/case/extra", "lab-search/case\n- **Proof:** yes", "lab-search/two words"} {
		t.Run(strings.ReplaceAll(bad, "/", "_"), func(t *testing.T) {
			root := t.TempDir()
			var stdout, stderr bytes.Buffer
			code := cmdObserve([]string{"--root", root, "--symptom", "bad pointer", "--lab", bad}, &stdout, &stderr)
			if code != 1 || !strings.Contains(stderr.String(), "--lab") {
				t.Fatalf("bad --lab: code %d: %s", code, stderr.String())
			}
			if _, err := os.Stat(filepath.Join(root, ".fathom")); !os.IsNotExist(err) {
				t.Fatalf("invalid flag wrote state: %v", err)
			}
		})
	}
}

// Discovery: rg -n 'observations' cmd internal --type go. Entry-reading
// consumers share readObservations, nextObservationID, countOpenObservations,
// and observationsByTool. The historical flywheel reads its own Markdown.
// Each regression starts by filing a real linked entry through the command.
func TestObserveLabReaders(t *testing.T) {
	for _, reader := range []string{"merged-files", "id", "status", "tool", "fleet", "adoption", "research-report", "research-bundle", "flywheel"} {
		t.Run(reader, func(t *testing.T) {
			root, content := fileLabObservation(t)
			switch reader {
			case "merged-files":
				// Also prove the legacy read path tolerates the same additive bullet.
				if err := os.WriteFile(observationsPath(root), []byte(strings.ReplaceAll(content, "O1", "O2")), 0644); err != nil {
					t.Fatal(err)
				}
				merged, ok, err := readObservations(root)
				if err != nil || !ok || strings.Count(merged, "- **Lab:**") != 2 {
					t.Fatalf("merged read: %q, %v", merged, err)
				}
			case "id":
				if n := nextObservationID(content); n != 2 {
					t.Fatalf("next id: %d", n)
				}
			case "status":
				if n := countOpenObservations(content); n != 1 {
					t.Fatalf("open count: %d", n)
				}
			case "tool":
				if got := observationsByTool(content); len(got) != 1 || got["find_symbol"] != 1 {
					t.Fatalf("tools: %+v", got)
				}
			case "fleet":
				n, note := serviceOpenObservations(root)
				if n == nil || *n != 1 || note != "" {
					t.Fatalf("fleet: %v, %s", n, note)
				}
			case "adoption":
				isolateVerifyLedger(t)
				var stdout, stderr bytes.Buffer
				// Explicit local inputs and an invalid daemon URL prevent live reads.
				code := cmdInstrumentAdoption([]string{"--json", "--root", root,
					"--call-log", filepath.Join(root, "missing-calls.jsonl"), "--url", "://offline"}, &stdout, &stderr)
				if code != 0 {
					t.Fatalf("adoption: %d: %s", code, stderr.String())
				}
				var r AdoptionReport
				if err := json.Unmarshal(stdout.Bytes(), &r); err != nil {
					t.Fatal(err)
				}
				if r.OpenObservationCount == nil || *r.OpenObservationCount != 1 {
					t.Fatalf("adoption did not read linked entry: %+v", r.OpenObservationCount)
				}
			case "research-report":
				now := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
				r := buildResearchReport(researchReportInputs{since: now.Add(-time.Hour), until: now, now: now, obsContent: content, obsRead: true})
				if r.TotalObservations == nil || *r.TotalObservations != 1 || r.OpenObservations == nil || *r.OpenObservations != 1 || r.ObservationsByTool["find_symbol"] != 1 {
					t.Fatalf("research observations: %+v", r)
				}
			case "research-bundle":
				now := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
				r := aggregateBundleInput("", now.Add(-time.Hour), now, nil, false, nil, false, content, true, nil, "fixture-salt")
				if r.TotalObservations == nil || *r.TotalObservations != 1 || r.OpenObservations == nil || *r.OpenObservations != 1 {
					t.Fatalf("bundle observations: %+v", r)
				}
			case "flywheel":
				t.Chdir(root)
				if err := os.MkdirAll("docs", 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile("docs/dogfood-observations.md", []byte("# Log\n"+content), 0644); err != nil {
					t.Fatal(err)
				}
				r := checkLoopObservations()
				if r.Status != loopClosed || !strings.Contains(r.Evidence, "1 observation(s)") {
					t.Fatalf("flywheel: %+v", r)
				}
			}
		})
	}
}

func TestObserveLabHelpersSingleLink(t *testing.T) {
	for _, helper := range []any{mintObservationEntry, formatObservationEntry} {
		typ := reflect.TypeOf(helper)
		if typ.IsVariadic() || typ.In(typ.NumIn()-1).Kind() != reflect.String {
			t.Errorf("observation helper must take one lab string: %s", typ)
		}
	}
}
