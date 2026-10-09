package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLocalizeReplayScoresTheThrowSite is TL-7's harness end to end: a
// one-case corpus whose "runtime" is sh replaying a recorded capture, a
// scratch ingest of its app, localize through the MCP handler, and the
// reading's verdict.
func TestLocalizeReplayScoresTheThrowSite(t *testing.T) {
	corpus := t.TempDir()
	dir := filepath.Join(corpus, "python")
	app := filepath.Join(dir, "app")
	if err := os.MkdirAll(filepath.Join(app, "shop"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(p, s string) {
		t.Helper()
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(app, "shop", "pricing.py"), "def helper():\n    return 1\n\n\ndef compute(a, b):\n    return a // b\n")
	write(filepath.Join(app, "shop", "main.py"), "from shop.pricing import compute\n\n\ndef handle():\n    return compute(1, 0)\n")
	trace := "Traceback (most recent call last):\n  File \"/app/shop/main.py\", line 5, in handle\n    return compute(1, 0)\n  File \"/app/shop/pricing.py\", line 6, in compute\n    return a // b\nZeroDivisionError: integer division or modulo by zero\n"
	line, _ := json.Marshal(map[string]any{
		"case": "div0", "lang": "python",
		"truth":    map[string]any{"file": "shop/pricing.py", "line": 6, "func": "compute", "name": "compute"},
		"payloads": map[string]string{"raw": trace},
	})
	write(filepath.Join(dir, "capture.jsonl"), string(line)+"\n")
	write(filepath.Join(dir, "capture.sh"), "cat capture.jsonl\n")
	write(filepath.Join(dir, "incident.json"), `{"service": "incident-python", "app": "app", "runtime": "python", "args": ["capture.sh"]}`)

	out := filepath.Join(t.TempDir(), "reading.json")
	var stdout, stderr bytes.Buffer
	code := cmdLocalizeReplay([]string{"-corpus", corpus, "-work", t.TempDir(), "-langs", "python", "-python", "/bin/sh", "-out", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var r replayReading
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	py := r.Arms[0].Langs["python"]
	if py.Answers != 1 || py.Correct != 1 || r.Verdict != "PASS" {
		t.Errorf("reading = %+v (verdict %s), want the one case correct and PASS; stderr %s", py, r.Verdict, stderr.String())
	}
}

func TestReplayHit(t *testing.T) {
	truth := incidentTruth{File: "src/billing/util.js", Name: "round"}
	for _, c := range []struct {
		file, symbol string
		want         bool
	}{
		{"src/billing/util.js", "round", true},
		{"app/src/billing/util.js", "Util.round", true},
		{"src/shipping/util.js", "round", false},
		{"src/billing/util.js", "roundUp", false},
		{"src/billing/util.js", "App\\Billing\\Util::round", true},
	} {
		if got := replayHit(c.file, c.symbol, truth); got != c.want {
			t.Errorf("replayHit(%q, %q) = %v, want %v", c.file, c.symbol, got, c.want)
		}
	}
}

func TestReplayVerdictNeedsEveryLanguage(t *testing.T) {
	a := replayArm{Answers: 10, Top1: 0.95, Langs: map[string]replayLang{"java": {Top1: 1}, "node": {Top1: 0.85}}}
	if v := replayVerdict(a); v != "BELOW" {
		t.Errorf("verdict = %s, want BELOW (node under the bar)", v)
	}
	a.Langs["node"] = replayLang{Top1: 0.9}
	if v := replayVerdict(a); v != "PASS" {
		t.Errorf("verdict = %s, want PASS", v)
	}
	if !strings.Contains(replayVerdict(replayArm{}), "BELOW") {
		t.Error("an empty arm must read BELOW")
	}
}

// TestReplayCaseRowRoundTrips: a miss is one compact row, read back when a
// baseline arm is embedded.
func TestReplayCaseRowRoundTrips(t *testing.T) {
	in := replayCase{Case: "c", Shape: "raw", TopFile: "a.js", TopSymbol: "f", Rank: 3}
	b, err := json.Marshal(in)
	if err != nil || string(b) != `["c","raw","a.js","f",3]` {
		t.Fatalf("row = %s, %v", b, err)
	}
	var out replayCase
	if err := json.Unmarshal(b, &out); err != nil || out != in {
		t.Errorf("round trip = %+v, %v", out, err)
	}
}

func TestCollapseScalarArrays(t *testing.T) {
	in, _ := json.MarshalIndent(map[string]any{"rows": [][]any{{"c", "raw", "a.js", "f", 3}}, "n": 1}, "", "  ")
	got := string(collapseScalarArrays(in))
	if !strings.Contains(got, `["c","raw","a.js","f",3]`) {
		t.Errorf("rows not collapsed:\n%s", got)
	}
	var v any
	if err := json.Unmarshal([]byte(got), &v); err != nil {
		t.Errorf("collapsed JSON is invalid: %v", err)
	}
}
