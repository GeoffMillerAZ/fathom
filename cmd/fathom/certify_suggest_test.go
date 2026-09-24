package main

// certify_suggest_test.go — TC-24's own test:
// TestCmdCertifySuggest_BandsAndNone. Fixture: three unregistered Test*
// symbols (no "// certifies:" marker on any) in one ingested repo, and a
// decision.MockDecider scripted so each test lands in a different band:
//
//   - TestAlpha: both answers proceed (>= 0.9)      -> a written suggestion
//   - TestBeta:  both answers confirm (0.5..0.9)     -> "unsure", not written
//   - TestGamma: both answers below 0.5               -> fully silent
//
// plus provider "none", which must print exactly one line and exit 0
// without ever calling a Decider or touching the store.
import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/decide"
)

const certifySuggestFixtureSrc = `package pkg

func TestAlpha(t *T) {}

func TestBeta(t *T) {}

func TestGamma(t *T) {}

type T struct{}
`

func certifySuggestMockResponse() decision.Response {
	ans := func(id string, kind decision.Kind, choice string, noul, confidence float64) decision.Answer {
		return decision.Answer{ID: id, Kind: kind, Choice: choice, Noul: noul, Confidence: confidence}
	}
	return decision.Response{
		Provider: "mock",
		Model:    "mock-1",
		Answers: map[string]decision.Answer{
			"TestAlpha:strength": ans("TestAlpha:strength", decision.KindChoice, "asserts", 0, 0.95),
			"TestAlpha:asserts":  ans("TestAlpha:asserts", decision.KindNoul, "", 0.96, 0.96),
			"TestBeta:strength":  ans("TestBeta:strength", decision.KindChoice, "smoke", 0, 0.70),
			"TestBeta:asserts":   ans("TestBeta:asserts", decision.KindNoul, "", 0.60, 0.60),
			"TestGamma:strength": ans("TestGamma:strength", decision.KindChoice, "smoke", 0, 0.30),
			"TestGamma:asserts":  ans("TestGamma:asserts", decision.KindNoul, "", 0.10, 0.90),
		},
	}
}

func TestCmdCertifySuggest_BandsAndNone(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC) }

	// ── provider none: one line, exit 0, no store/service needed at all ──
	nonePorts := &decide.Ports{Decider: decision.None{}}
	var noneOut, noneErr bytes.Buffer
	code := runCertifySuggest(nil, nonePorts, now, &noneOut, &noneErr)
	if code != 0 {
		t.Fatalf("provider none: exit = %d, stderr: %s", code, noneErr.String())
	}
	lines := strings.Split(strings.TrimRight(noneOut.String(), "\n"), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "decision provider is required") {
		t.Fatalf("provider none: stdout = %q, want exactly one line naming a required provider", noneOut.String())
	}

	// ── a real provider: bands ──
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pkg_test.go"), []byte(certifySuggestFixtureSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	service := "suggest-svc"
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", service, "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
	}

	mock := decision.NewMockDecider("mock", decision.MockEntry{Response: certifySuggestMockResponse()})
	rec := decide.NewRecorder(&bytes.Buffer{}, now, "test")
	ports := &decide.Ports{Decider: mock, Recorder: rec}

	outPath := filepath.Join(t.TempDir(), "suggestions.jsonl")
	var out, errBuf bytes.Buffer
	code = runCertifySuggest(
		[]string{"--service", service, "--store", storePath, "--out", outPath},
		ports, now, &out, &errBuf,
	)
	if code != 0 {
		t.Fatalf("certify suggest exit = %d, stderr: %s", code, errBuf.String())
	}

	if calls := mock.Calls(); len(calls) != 1 {
		t.Fatalf("Decide called %d time(s), want exactly 1 (all 3 tests fit one chunk)", len(calls))
	}

	stdout := out.String()
	if !strings.Contains(stdout, "suggested: TestAlpha may certify") {
		t.Errorf("stdout missing TestAlpha's suggested line:\n%s", stdout)
	}
	if !strings.Contains(stdout, "unsure: TestBeta") {
		t.Errorf("stdout missing TestBeta's unsure line:\n%s", stdout)
	}
	if strings.Contains(stdout, "TestGamma") {
		t.Errorf("TestGamma is below both bands and must render NOTHING, got:\n%s", stdout)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read suggestions file: %v", err)
	}
	lines = strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("suggestions file has %d line(s), want exactly 1 (only TestAlpha proceeds): %q", len(lines), string(data))
	}
	var s certifySuggestion
	if err := json.Unmarshal([]byte(lines[0]), &s); err != nil {
		t.Fatalf("unmarshal suggestion line %q: %v", lines[0], err)
	}
	if s.Test != "TestAlpha" || s.File != "pkg_test.go" || s.Strength != "asserts" || s.Model != "mock-1" {
		t.Errorf("suggestion = %+v, want {Test:TestAlpha File:pkg_test.go Strength:asserts Model:mock-1 ...}", s)
	}
	if s.AssertsNoul != 0.96 || s.Confidence != 0.95 {
		t.Errorf("suggestion confidence/noul = %v/%v, want 0.95/0.96", s.Confidence, s.AssertsNoul)
	}
	if s.GeneratedAt != "2026-09-23T00:00:00Z" {
		t.Errorf("generated_at = %q, want the injected clock's timestamp", s.GeneratedAt)
	}

	// certify gaps (table path) renders the suggested hint from the file.
	var gapsOut, gapsErr bytes.Buffer
	if code := cmdCertifyGaps([]string{"--service", service, "--store", storePath}, &gapsOut, &gapsErr); code != 0 {
		t.Fatalf("certify gaps exit = %d, stderr: %s", code, gapsErr.String())
	}
	// certify gaps reads the WORKSPACE default path, not --out, so with a
	// custom --out it will not see this run's own file — this only proves
	// the render function's own wiring is exercised elsewhere
	// (renderCertifyGapsTable is unit-testable directly); skip asserting
	// content here to avoid depending on the real repo's workspace/.
	_ = gapsOut.String()
}

func TestRenderCertifyGapsTable_SuggestedLine(t *testing.T) {
	var buf bytes.Buffer
	renderCertifyGapsTable(&buf, certify.GapReport{}, []certifySuggestion{
		{Test: "TestAlpha", Strength: "asserts", Confidence: 0.95, AssertsNoul: 0.96},
	})
	got := buf.String()
	if !strings.Contains(got, "suggested: TestAlpha may certify") {
		t.Errorf("output = %q, want a suggested: line", got)
	}
}
