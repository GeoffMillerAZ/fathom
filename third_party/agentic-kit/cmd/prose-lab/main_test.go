package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanaryPreflightNeverUsesCredentialsOrCreatesJournal(t *testing.T) {
	base := t.TempDir()
	s := canaryStudy{Version: "prose-lab.canary/1", ID: "synthetic-canary", DatasetKind: "synthetic", BudgetMicroUSD: 200, Lanes: map[string]lane{"decision": testLane(100)}, ExpectedCaseIDs: []string{"case-a"}}
	study := filepath.Join(base, "study.json")
	request := filepath.Join(base, "request.json")
	journalPath := filepath.Join(base, "journal")
	if err := os.WriteFile(study, jsonBytes(s), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(request, []byte(`{"State":"synthetic","Questions":[{"ID":"check","Kind":"predicate","Instructions":"The state is synthetic."}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENROUTER_API_KEY", "")
	for _, tc := range []struct {
		command, extra string
		code           int
	}{
		{"prepare", "", 0},
		{"capture", "", 2},
	} {
		var out, errOut bytes.Buffer
		args := []string{tc.command, "--study", study, "--lane", "decision", "--case", "case-a", "--request", request}
		if got := runCanary(context.Background(), args, &out, &errOut); got != tc.code {
			t.Fatalf("%s: code %d stdout=%s stderr=%s", tc.command, got, out.String(), errOut.String())
		}
		if strings.Contains(out.String(), "synthetic\"") {
			t.Fatal("request content printed")
		}
	}
	if _, err := os.Stat(journalPath); !os.IsNotExist(err) {
		t.Fatal("preflight created journal")
	}
}

func TestNativeRequestRejectsDuplicateUnknownAndMalformedUnicode(t *testing.T) {
	for _, b := range []string{`{"State":"a","State":"b"}`, `{"unexpected":1}`, `{"State":"\ud800"}`} {
		var v struct{ State string }
		if decodeNativeRequest([]byte(b), &v) == nil {
			t.Fatalf("accepted %s", b)
		}
	}
}

func TestInputRefusesSymlinkAndDirectory(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "input.json")
	link := filepath.Join(base, "link.json")
	if err := os.WriteFile(target, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{link, base} {
		if _, err := readLabBytes(path, 100); err == nil {
			t.Fatal("non-regular or symlink input admitted")
		}
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("fixture output unavailable") }

func TestCanaryHelpAndOutputFailure(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"prepare", "--help"}} {
		var out, errOut bytes.Buffer
		if got := runCanary(context.Background(), args, &out, &errOut); got != 0 || out.Len() == 0 {
			t.Fatal("help failed")
		}
	}
	if got := runCanary(context.Background(), []string{"--help"}, brokenWriter{}, io.Discard); got != 2 {
		t.Fatal("output error hidden")
	}
}

func TestCaptureOutputFailureRetainsSuccessfulPrivateOutcome(t *testing.T) {
	base := t.TempDir()
	l := lane{Provider: "anthropic", Model: "model", PricingSource: "fixture", MaxInputTokens: 8192, MaxOutputTokens: 128, InputRate: 1_000_000, OutputRate: 1_000_000, ReserveInputRate: 1_000_000, ReserveOutputRate: 1_000_000}
	s := canaryStudy{Version: "prose-lab.canary/1", ID: "fixture", DatasetKind: "synthetic", BudgetMicroUSD: 20000, Lanes: map[string]lane{"generation": l}, ExpectedCaseIDs: []string{"capture"}}
	study := filepath.Join(base, "study.json")
	request := filepath.Join(base, "request.json")
	journalPath := filepath.Join(base, "journal")
	if err := os.WriteFile(study, jsonBytes(s), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(request, []byte(`{"Model":"model","MaxTokens":128,"Prompt":"synthetic"}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANTHROPIC_API_KEY", "fixture-credential")
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	calls := 0
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"model":"model","stop_reason":"end_turn","content":[{"type":"text","text":"synthetic"}],"usage":{"input_tokens":15,"output_tokens":1}}`))}, nil
	})
	code := runCanary(context.Background(), []string{"capture", "--study", study, "--lane", "generation", "--case", "capture", "--request", request, "--journal", journalPath, "--live"}, brokenWriter{}, io.Discard)
	if code != 2 || calls != 1 {
		t.Fatalf("code=%d calls=%d", code, calls)
	}
	b, err := readLabBytes(filepath.Join(journalPath, "outcome-000001.json"), 4<<20)
	if err != nil {
		t.Fatal(err)
	}
	var o outcome
	if decodeLabJSON(b, &o) != nil || o.Status != "completed" || o.ActualMicroUSD == nil || *o.ActualMicroUSD != 16 {
		t.Fatal("successful attempt lost after output failure")
	}
}

func TestStudyRequiresExplicitNestedPricingFields(t *testing.T) {
	s := canaryStudy{Version: "prose-lab.canary/1", ID: "fixture", DatasetKind: "synthetic", BudgetMicroUSD: 20000, Lanes: map[string]lane{"decision": testLane(8192)}, ExpectedCaseIDs: []string{"capture"}}
	for _, field := range []string{"tier_threshold", "high_input_rate", "cache_write_rate", "max_output_tokens"} {
		t.Run(field, func(t *testing.T) {
			var tree map[string]any
			if err := json.Unmarshal(jsonBytes(s), &tree); err != nil {
				t.Fatal(err)
			}
			delete(tree["lanes"].(map[string]any)["decision"].(map[string]any), field)
			p := filepath.Join(t.TempDir(), "study.json")
			if err := os.WriteFile(p, jsonBytes(tree), 0600); err != nil {
				t.Fatal(err)
			}
			var got canaryStudy
			if _, err := readStudyJSON(p, &got); err == nil {
				t.Fatal("missing nested pricing field accepted")
			}
		})
	}
}
