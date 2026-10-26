//go:build !command_classifier

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultClassifierControlsHoldBeforeSourceOrScanner(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*scope, *policy)
	}{
		{"required-site", func(s *scope, _ *policy) { s.RequiredClassifierIDs = []string{"semantic-concern"} }},
		{"permitted-pin", func(s *scope, _ *policy) { s.PermittedClassifierSHA256 = []string{strings.Repeat("a", 64)} }},
		{"policy-pin", func(s *scope, _ *policy) { s.ClassifierPolicySHA256 = strings.Repeat("b", 64) }},
		{"policy-object", func(_ *scope, p *policy) { p.Classifier = &classifierPolicy{Profile: "loopback-concern/1"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, s, p, q := fixture(t)
			scanner := testScannerPolicy(t)
			s.RequiredScannerIDs = []string{"gitleaks"}
			p.Scanner = &scanner
			tc.mutate(&s, &p)
			calls := 0
			r := runWithScanner(root, s, p, q, "json", func(context.Context, string, string, []byte, int, ...string) scannerRunResult {
				calls++
				return scannerRunResult{started: true}
			})
			if r.Status != "incomplete" || r.Code != "unsupported" || r.FilesRead != 0 || r.BytesRead != 0 || r.BytesScanned != 0 || len(r.Rows) != 0 || r.ExternalScanner != nil || r.ExternalClassifier != nil || calls != 0 {
				t.Fatalf("result=%+v scannerCalls=%d", r, calls)
			}
		})
	}
}
func TestDefaultClassifierHelpAndAbsentBehavior(t *testing.T) {
	var out bytes.Buffer
	if execute([]string{"help"}, &out) != 0 || !bytes.Contains(out.Bytes(), []byte(classifierCapabilityLine)) {
		t.Fatalf("help=%q", out.String())
	}
	root, s, p, q := fixture(t)
	r := run(root, s, p, q, "json")
	if r.Status != "no-findings" || r.Code != "ok" || r.ExternalClassifier != nil || len(r.Rows) != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestDefaultSelectedClassifierEmittedFormatsCUE(t *testing.T) {
	cue, e := exec.LookPath("cue")
	if e != nil {
		t.Skip("cue unavailable")
	}
	root, s, p, q := fixture(t)
	s.RequiredClassifierIDs = []string{"semantic-concern"}
	dir := t.TempDir()
	write := func(name string, v any) string {
		t.Helper()
		b, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(dir, name)
		if e = os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
		return path
	}
	sp, pp, qp := write("scope.json", s), write("policy.json", p), write("request.json", q)
	schema, e := os.ReadFile("schema.cue")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "schema.cue"), schema, 0600); e != nil {
		t.Fatal(e)
	}
	for _, format := range []string{"json", "text", "compact"} {
		t.Run(format, func(t *testing.T) {
			var out bytes.Buffer
			code := execute([]string{"inspect", "--root", root, "--scope", sp, "--policy", pp, "--request", qp, "--format", format}, &out)
			var report map[string]json.RawMessage
			if code != 2 || json.Unmarshal(out.Bytes(), &report) != nil || string(report["status"]) != `"incomplete"` || string(report["code"]) != `"unsupported"` || string(report["filesRead"]) != "0" || string(report["bytesRead"]) != "0" || report["externalClassifier"] != nil || report["externalScanner"] != nil || report["rows"] != nil {
				t.Fatalf("code=%d report=%s", code, out.String())
			}
			path := filepath.Join(dir, "report-"+format+".json")
			if e := os.WriteFile(path, out.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			cmd := exec.Command(cue, "vet", "-c", "-d", "#OutputReport", "schema.cue", filepath.Base(path))
			cmd.Dir = dir
			if evidence, e := cmd.CombinedOutput(); e != nil {
				t.Fatalf("cue %s: %v %s", format, e, evidence)
			}
			reject := func(name string, mutate func(map[string]json.RawMessage)) {
				t.Run(name, func(t *testing.T) {
					var candidate map[string]json.RawMessage
					if err := json.Unmarshal(out.Bytes(), &candidate); err != nil {
						t.Fatal(err)
					}
					mutate(candidate)
					wire, err := json.Marshal(candidate)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, wire, 0600); err != nil {
						t.Fatal(err)
					}
					probe := exec.Command(cue, "vet", "-c", "-d", "#OutputReport", "schema.cue", filepath.Base(path))
					probe.Dir = dir
					if _, err := probe.CombinedOutput(); err == nil {
						t.Fatal("forged unsupported report vetted")
					}
				})
			}
			reject("read-count", func(m map[string]json.RawMessage) { m["filesRead"] = json.RawMessage("1") })
			reject("bytes-read", func(m map[string]json.RawMessage) { m["bytesRead"] = json.RawMessage("1") })
			reject("scanner", func(m map[string]json.RawMessage) {
				m["externalScanner"] = json.RawMessage(`{"id":"gitleaks","profile":"gitleaks-stdin-8.30.1","status":"not-invoked","code":"not_invoked","expectedFiles":1,"attemptedFiles":0,"invokedFiles":0,"completedFiles":0,"bytesSubmitted":0,"bytesObserved":0,"findings":0}`)
			})
			reject("rows", func(m map[string]json.RawMessage) {
				m["rows"] = json.RawMessage(`[{"file":0,"line":1,"text":"forged"}]`)
			})
			reject("missing-zero", func(m map[string]json.RawMessage) { delete(m, "filesRead") })
			// Unsupported cannot carry a classifier receipt, even if its other fields are valid.
			report["externalClassifier"] = json.RawMessage(`{"profile":"loopback-concern/1","status":"not-invoked","code":"not_invoked","expectedFiles":1,"selectedFiles":0,"selectedBytes":0,"attemptedRequests":0,"completedDecisions":0,"requestBytes":0,"responseBytes":0}`)
			invalid, _ := json.Marshal(report)
			if e := os.WriteFile(path, invalid, 0600); e != nil {
				t.Fatal(e)
			}
			cmd = exec.Command(cue, "vet", "-c", "-d", "#OutputReport", "schema.cue", filepath.Base(path))
			cmd.Dir = dir
			if _, e := cmd.CombinedOutput(); e == nil {
				t.Fatal("unsupported receipt vetted")
			}
		})
	}
}
