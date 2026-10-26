//go:build command_classifier

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func classifierFixtures(t *testing.T, endpoint, choice string) (string, scope, policy, request) {
	t.Helper()
	dir := t.TempDir()
	data := []byte("safe first line\nsecond line outside view\n")
	if e := os.WriteFile(filepath.Join(dir, "sample.txt"), data, 0600); e != nil {
		t.Fatal(e)
	}
	s := scope{Version: "command-guard.scope/1", RequiredLayerIDs: []string{"base"}, RequiredRuleIDs: []string{"secret"}, RequiredClassifierIDs: []string{"semantic-concern"}, PermittedClassifierSHA256: []string{digest(data)}}
	p := policy{Version: "command-guard.policy/1", Layers: []layer{{ID: "base", AllowedOperations: []string{"read-range"}, Limits: limits{MaxFileBytes: 10000, MaxTotalBytes: 10000, MaxOutputBytes: 100000, MaxResults: 100}, DeniedPathPrefixes: []string{}}}, Rules: []rule{{ID: "secret", Pattern: "NEVERMATCH"}}}
	c := &classifierPolicy{Profile: "loopback-concern/1", Endpoint: endpoint, Model: "requested", ExpectedResponseModel: "response", Criterion: "Judge the selected text.", TimeoutMillis: 3000, MaxInputBytes: 16384, MaxResponseBytes: 16384, MaxTokens: 256}
	c.CriterionSHA256 = digest([]byte(c.Criterion))
	p.Classifier = c
	raw, e := json.Marshal(p)
	if e != nil {
		t.Fatal(e)
	}
	p.sourceSHA256 = digest(raw)
	s.ClassifierPolicySHA256 = p.sourceSHA256
	q := request{Version: "command-guard.request/1", Kind: "read-range", Files: []fileSpec{{Path: "sample.txt", SHA256: digest(data)}}, StartLine: 1, EndLine: 1}
	_ = choice
	return dir, s, p, q
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func reply(choice string) string {
	inner := fmt.Sprintf(`{"answers":{"sensitivity":{"choice":%q,"probabilities":{"concern":%d,"no-concern":%d,"unknown":%d}}}}`, choice, boolInt(choice == "concern"), boolInt(choice == "no-concern"), boolInt(choice == "unknown"))
	return fmt.Sprintf(`{"model":"response","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":%q}}]}`, inner)
}
func questionIDFromWire(raw []byte) string {
	var envelope struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return ""
	}
	for _, message := range envelope.Messages {
		var prompt struct {
			Questions []struct {
				ID string `json:"id"`
			} `json:"questions"`
		}
		if json.Unmarshal([]byte(message.Content), &prompt) == nil && len(prompt.Questions) == 1 {
			return prompt.Questions[0].ID
		}
	}
	return ""
}
func bindClassifierBody(raw []byte, body string) string {
	id := questionIDFromWire(raw)
	if id == "" {
		return body
	}
	return strings.ReplaceAll(body, `\"sensitivity\"`, `\"`+id+`\"`)
}
func writeBoundReply(w http.ResponseWriter, r *http.Request, body string) {
	raw, _ := io.ReadAll(r.Body)
	fmt.Fprint(w, bindClassifierBody(raw, body))
}
func TestClassifierActualHTTPOutcomes(t *testing.T) {
	for _, tc := range []struct{ choice, status, code string }{{"no-concern", "no-findings", "ok"}, {"concern", "blocked", "classifier_concern"}, {"unknown", "incomplete", "classifier_incomplete"}} {
		t.Run(tc.choice, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != "POST" || r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "" || r.Header.Get("Accept-Encoding") != "" {
					t.Errorf("wire metadata: %s %s %v", r.Method, r.URL.Path, r.Header)
				}
				var body map[string]any
				raw, e := io.ReadAll(r.Body)
				if e != nil {
					t.Error(e)
				}
				if json.Unmarshal(raw, &body) != nil {
					t.Error("request JSON")
				}
				b, _ := json.Marshal(body)
				if !bytes.Contains(b, []byte("second line outside view")) || bytes.Contains(b, []byte("sample.txt")) {
					t.Error("state missing text or leaked path")
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, bindClassifierBody(raw, reply(tc.choice)))
			}))
			defer srv.Close()
			dir, s, p, q := classifierFixtures(t, srv.URL+"/v1", tc.choice)
			r := run(dir, s, p, q, "json")
			if r.Status != tc.status || r.Code != tc.code || calls.Load() != 1 {
				t.Fatalf("status=%s code=%s calls=%d receipt=%+v", r.Status, r.Code, calls.Load(), r.ExternalClassifier)
			}
			x := r.ExternalClassifier
			if x == nil || x.AttemptedRequests != 1 || x.CompletedDecisions != 1 || x.SelectedFiles != 1 || x.RequestBytes == 0 || x.ResponseBytes == 0 {
				t.Fatalf("receipt=%+v", x)
			}
			if tc.choice != "no-concern" && len(r.Rows) != 0 {
				t.Fatal("held source")
			}
			if tc.choice == "no-concern" && (len(r.Rows) != 1 || r.Rows[0].Text != "safe first line") {
				t.Fatal("missing verified row")
			}
		})
	}
}
func TestClassifierControlsBeforeSourcesAndHTTP(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); writeBoundReply(w, r, reply("no-concern")) }))
	defer srv.Close()
	dir, s, p, q := classifierFixtures(t, srv.URL+"/v1", "")
	cases := []struct {
		name   string
		mutate func(*scope, *policy, *request)
	}{
		{"missing-site", func(s *scope, p *policy, q *request) { s.RequiredClassifierIDs = nil }},
		{"wrong-policy-pin", func(s *scope, p *policy, q *request) { s.ClassifierPolicySHA256 = digest([]byte("other")) }},
		{"unpermitted", func(s *scope, p *policy, q *request) { s.PermittedClassifierSHA256 = []string{digest([]byte("other"))} }},
		{"bad-port", func(s *scope, p *policy, q *request) { p.Classifier.Endpoint = "http://127.0.0.1:0001/v1" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ss := s
			pp := p
			copyC := *p.Classifier
			pp.Classifier = &copyC
			qq := q
			tc.mutate(&ss, &pp, &qq)
			r := run(filepath.Join(dir, "missing"), ss, pp, qq, "json")
			if r.Code != "invalid" || r.FilesRead != 0 || r.ExternalClassifier != nil || calls.Load() != 0 {
				t.Fatalf("result=%+v calls=%d", r, calls.Load())
			}
		})
	}
}
func TestClassifierChangedInputNeverDispatches(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer srv.Close()
	dir, s, p, q := classifierFixtures(t, srv.URL+"/v1", "")
	if err := os.WriteFile(filepath.Join(dir, "sample.txt"), []byte("changed synthetic text\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r := run(dir, s, p, q, "json")
	if r.Status != "incomplete" || r.Code != "invalid" || len(r.Rows) != 0 || calls.Load() != 0 || r.ExternalClassifier == nil || r.ExternalClassifier.AttemptedRequests != 0 {
		t.Fatalf("%+v calls=%d", r, calls.Load())
	}
}
func TestClassifierLateRegexHold(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer srv.Close()
	dir, s, p, q := classifierFixtures(t, srv.URL+"/v1", "")
	p.Rules[0].Pattern = "safe"
	r := run(dir, s, p, q, "json")
	if r.Status != "blocked" || r.Code != "rule" || calls.Load() != 0 || r.ExternalClassifier == nil || r.ExternalClassifier.AttemptedRequests != 0 {
		t.Fatalf("%+v", r)
	}
}
func TestClassifierMalformedWireWithholds(t *testing.T) {
	for _, tc := range []struct{ name, body, code string }{
		{"duplicate", `{"model":"response","model":"response","choices":[]}`, "response_invalid"},
		{"wrong-model", strings.Replace(reply("no-concern"), `"model":"response"`, `"model":"other"`, 1), "model_mismatch"},
		{"two-choices", `{"model":"response","choices":[{},{}]}`, "unsupported_response"},
		{"refusal", `{"model":"response","choices":[{"finish_reason":"stop","message":{"role":"assistant","refusal":"no","content":"x"}}]}`, "refused"},
		{"tool-call", strings.Replace(reply("no-concern"), `"content":`, `"tool_calls":[{}],"content":`, 1), "unsupported_response"},
		{"trailing", reply("no-concern") + ` true`, "response_invalid"},
		{"bad-prob", strings.Replace(reply("no-concern"), `\"no-concern\":1`, `\"no-concern\":0.8`, 1), "response_invalid"},
		{"null-prob", strings.Replace(reply("no-concern"), `\"concern\":0`, `\"concern\":null`, 1), "response_invalid"},
		{"choice-mismatch", strings.Replace(strings.Replace(reply("no-concern"), `\"concern\":0`, `\"concern\":0.7`, 1), `\"no-concern\":1`, `\"no-concern\":0.3`, 1), "response_invalid"},
		{"tie", strings.Replace(strings.Replace(reply("no-concern"), `\"concern\":0`, `\"concern\":0.5`, 1), `\"no-concern\":1`, `\"no-concern\":0.5`, 1), "response_invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeBoundReply(w, r, tc.body) }))
			defer srv.Close()
			dir, s, p, q := classifierFixtures(t, srv.URL+"/v1", "")
			r := run(dir, s, p, q, "json")
			if r.Code != "classifier_incomplete" || r.ExternalClassifier == nil || r.ExternalClassifier.Code != tc.code || len(r.Rows) != 0 {
				t.Fatalf("%+v", r)
			}
			if tc.name == "refusal" {
				assertClassifierHeldCUE(t, r)
			}
		})
	}
}

func assertClassifierHeldCUE(t *testing.T, r report) {
	t.Helper()
	cueClassifierVet(t, "#OutputReport", r, true)
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"forged-row", func(m map[string]any) { m["rows"] = []any{map[string]any{"file": 0, "line": 1, "text": "forged"}} }},
		{"returned", func(m map[string]any) { m["returned"] = 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(r)
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatal(err)
			}
			tc.mutate(m)
			cueClassifierVet(t, "#OutputReport", m, false)
		})
	}
}
func TestClassifierEmptyToolCallsAreNoCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeBoundReply(w, r, strings.Replace(reply("no-concern"), `"content":`, `"tool_calls":[],"content":`, 1))
	}))
	defer srv.Close()
	dir, s, p, q := classifierFixtures(t, srv.URL+"/v1", "")
	r := run(dir, s, p, q, "json")
	if r.Status != "no-findings" || r.ExternalClassifier == nil || r.ExternalClassifier.CompletedDecisions != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestClassifierCanceledBeforeDispatch(t *testing.T) {
	c := classifierPolicy{TimeoutMillis: 1000, MaxInputBytes: 16384}
	r := newClassifierReceipt(1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := classifySelectedContext(ctx, c, [][]byte{[]byte("synthetic")}, r)
	if got != "incomplete" || r.Code != "deadline" || r.AttemptedRequests != 0 {
		t.Fatalf("%s %+v", got, r)
	}
}
func TestClassifierReplyLimitAndRedirect(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		code    string
	}{
		{"reply-limit", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, strings.Repeat("x", 200)) }, "response_limit"},
		{"redirect", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", "http://example.invalid/")
			w.WriteHeader(302)
		}, "http_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()
			dir, s, p, q := classifierFixtures(t, srv.URL+"/v1", "")
			p.Classifier.MaxResponseBytes = 100
			r := run(dir, s, p, q, "json")
			if r.Code != "classifier_incomplete" || r.ExternalClassifier.Code != tc.code || r.ExternalClassifier.AttemptedRequests != 1 || r.ExternalClassifier.CompletedDecisions != 0 || len(r.Rows) != 0 {
				t.Fatalf("%+v", r)
			}
		})
	}
}

func cueClassifierVet(t *testing.T, definition string, value any, valid bool) {
	t.Helper()
	cue, e := exec.LookPath("cue")
	if e != nil {
		t.Skip("cue unavailable")
	}
	dir := t.TempDir()
	schema, e := os.ReadFile("schema.cue")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "schema.cue"), schema, 0600); e != nil {
		t.Fatal(e)
	}
	b, e := json.Marshal(value)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "value.json"), b, 0600); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(cue, "vet", "-c", "-d", definition, "schema.cue", "value.json")
	cmd.Dir = dir
	out, e := cmd.CombinedOutput()
	if (e == nil) != valid {
		t.Fatalf("cue %s valid=%t err=%v output=%s value=%s", definition, valid, e, out, b)
	}
}
func TestClassifierCUEControlsAndReport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeBoundReply(w, r, reply("no-concern")) }))
	defer srv.Close()
	dir, s, p, q := classifierFixtures(t, srv.URL+"/v1", "")
	cueClassifierVet(t, "#Scope", s, true)
	cueClassifierVet(t, "#Policy", p, true)
	r := run(dir, s, p, q, "json")
	cueClassifierVet(t, "#OutputReport", r, true)
	for _, choice := range []string{"concern", "unknown"} {
		peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeBoundReply(w, r, reply(choice)) }))
		root, selectedScope, selectedPolicy, selectedRequest := classifierFixtures(t, peer.URL+"/v1", choice)
		assertClassifierHeldCUE(t, run(root, selectedScope, selectedPolicy, selectedRequest, "json"))
		peer.Close()
	}
	b := encode(r, "compact")
	var compact map[string]any
	if json.Unmarshal(b, &compact) != nil {
		t.Fatal("compact")
	}
	cueClassifierVet(t, "#OutputReport", compact, true)
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
		valid  bool
	}{
		{"missing-pins", func(m map[string]any) { delete(m, "permittedClassifierSHA256") }, false},
		{"missing-site", func(m map[string]any) { delete(m, "requiredClassifierIds") }, false},
		{"missing-policy-pin", func(m map[string]any) { delete(m, "classifierPolicySHA256") }, false},
		{"extra-pins-without-site", func(m map[string]any) { delete(m, "requiredClassifierIds") }, false},
		{"bad-policy-pin", func(m map[string]any) { m["classifierPolicySHA256"] = "x" }, false},
		{"duplicate-pins", func(m map[string]any) {
			h := m["permittedClassifierSHA256"].([]any)[0]
			m["permittedClassifierSHA256"] = []any{h, h}
		}, false},
		{"empty-disabled", func(m map[string]any) {
			delete(m, "requiredClassifierIds")
			m["permittedClassifierSHA256"] = []string{}
			m["classifierPolicySHA256"] = ""
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(s)
			var m map[string]any
			json.Unmarshal(b, &m)
			tc.mutate(m)
			cueClassifierVet(t, "#Scope", m, tc.valid)
		})
	}
	for _, tc := range []struct {
		name, endpoint, criterion string
		valid                     bool
	}{
		{"port-over", "http://127.0.0.1:65536/v1", "review", false},
		{"port-99999", "http://127.0.0.1:99999/v1", "review", false},
		{"leading-zero", "http://127.0.0.1:0001/v1", "review", false},
		{"criterion-whitespace", srv.URL + "/v1", "  \t ", false},
		{"criterion-byte-limit", srv.URL + "/v1", strings.Repeat("a", 4096), true},
		{"criterion-byte-over", srv.URL + "/v1", strings.Repeat("a", 4097), false},
		{"criterion-utf8-byte-over", srv.URL + "/v1", strings.Repeat("é", 2049), false},
		{"criterion-format-char", srv.URL + "/v1", "a\u200bb", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(p)
			var m map[string]any
			json.Unmarshal(b, &m)
			c := m["classifier"].(map[string]any)
			c["endpoint"] = tc.endpoint
			c["criterion"] = tc.criterion
			cueClassifierVet(t, "#Policy", m, tc.valid)
		})
	}
	broken := r
	broken.ExternalClassifier = &classifierReceipt{Profile: "loopback-concern/1", Status: "incomplete", Code: "response_invalid", ExpectedFiles: 1}
	cueClassifierVet(t, "#OutputReport", broken, false)
}

func TestClassifierPolicySnapshotPin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected request") }))
	defer srv.Close()
	dir, s, p, q := classifierFixtures(t, srv.URL+"/v1", "")
	raw, e := json.Marshal(p)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "policy.json")
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	var captured policy
	if e = strictFile(path, &captured); e != nil {
		t.Fatal(e)
	}
	if captured.sourceSHA256 != digest(raw) || captured.sourceSHA256 != s.ClassifierPolicySHA256 {
		t.Fatal("snapshot hash")
	}
	changed := append(append([]byte{}, raw...), '\n')
	if e = os.WriteFile(path, changed, 0600); e != nil {
		t.Fatal(e)
	}
	var later policy
	if e = strictFile(path, &later); e != nil {
		t.Fatal(e)
	}
	if later.sourceSHA256 == captured.sourceSHA256 {
		t.Fatal("whitespace drift")
	}
	r := run(filepath.Join(dir, "missing"), s, later, q, "json")
	if r.Code != "invalid" || r.FilesRead != 0 || r.ExternalClassifier != nil {
		t.Fatalf("%+v", r)
	}
}

func TestClassifierTinyOutputCapKeepsReceipt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeBoundReply(w, r, reply("no-concern")) }))
	defer srv.Close()
	dir, s, p, q := classifierFixtures(t, srv.URL+"/v1", "")
	p.Layers[0].Limits.MaxOutputBytes = 20
	r := run(dir, s, p, q, "json")
	if r.Code != "output_limit" || r.ExternalClassifier == nil || r.ExternalClassifier.CompletedDecisions != 1 || len(r.Rows) != 0 {
		t.Fatalf("%+v", r)
	}
	var out bytes.Buffer
	if emit(&out, r, "json", 2) != 2 || out.Len() != 0 {
		t.Fatalf("output=%q", out.String())
	}
}

func TestClassifierGitSelectedBeforeProcesses(t *testing.T) {
	_, s, p, _ := classifierFixtures(t, "http://127.0.0.1:1/v1", "")
	calls := 0
	r := runGitWithScanner(t.TempDir(), s, p, gitRequest{Kind: "git-status"}, "json", func(context.Context, string, string, []string, []string, int) scannerRunResult {
		calls++
		return scannerRunResult{}
	}, func(context.Context, string, string, []byte, int, ...string) scannerRunResult {
		calls++
		return scannerRunResult{}
	})
	if r.Code != "unsupported" || r.Status != "incomplete" || calls != 0 {
		t.Fatalf("%+v calls=%d", r, calls)
	}
}

func TestClassifierQuestionIDRejectsStaleReply(t *testing.T) {
	for _, kind := range []string{"state", "criterion", "static"} {
		t.Run(kind, func(t *testing.T) {
			var prior atomic.Value
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				id := questionIDFromWire(raw)
				n := calls.Add(1)
				if n == 1 {
					prior.Store(id)
					fmt.Fprint(w, bindClassifierBody(raw, reply("no-concern")))
					return
				}
				old := prior.Load().(string)
				if kind == "static" {
					old = "sensitivity"
				}
				fmt.Fprint(w, strings.ReplaceAll(reply("no-concern"), `\"sensitivity\"`, `\"`+old+`\"`))
			}))
			defer srv.Close()
			dir, s, p, q := classifierFixtures(t, srv.URL+"/v1", "")
			first := run(dir, s, p, q, "json")
			if first.Status != "no-findings" {
				t.Fatalf("baseline %+v", first)
			}
			switch kind {
			case "state":
				b := []byte("new synthetic source\n")
				if e := os.WriteFile(filepath.Join(dir, "sample.txt"), b, 0600); e != nil {
					t.Fatal(e)
				}
				q.Files[0].SHA256 = digest(b)
				s.PermittedClassifierSHA256 = []string{digest(b)}
			case "criterion":
				p.Classifier.Criterion = "Different protected criterion."
				p.Classifier.CriterionSHA256 = digest([]byte(p.Classifier.Criterion))
				raw, _ := json.Marshal(p)
				p.sourceSHA256 = digest(raw)
				s.ClassifierPolicySHA256 = p.sourceSHA256
			}
			second := run(dir, s, p, q, "json")
			if second.Code != "classifier_incomplete" || second.ExternalClassifier == nil || second.ExternalClassifier.Code != "response_invalid" || second.ExternalClassifier.AttemptedRequests != 1 || second.ExternalClassifier.CompletedDecisions != 0 || len(second.Rows) != 0 || calls.Load() != 2 {
				t.Fatalf("replay %+v calls=%d", second, calls.Load())
			}
		})
	}
}

func cueClassifierVetRaw(t *testing.T, definition string, raw []byte, valid bool) {
	t.Helper()
	cue, e := exec.LookPath("cue")
	if e != nil {
		t.Skip("cue unavailable")
	}
	dir := t.TempDir()
	schema, e := os.ReadFile("schema.cue")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "schema.cue"), schema, 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "value.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(cue, "vet", "-c", "-d", definition, "schema.cue", "value.json")
	cmd.Dir = dir
	out, e := cmd.CombinedOutput()
	if (e == nil) != valid {
		t.Fatalf("cue %s valid=%t err=%v output=%s", definition, valid, e, out)
	}
}
func TestClassifierCUERequiredFieldsAndEmittedFormats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeBoundReply(w, r, reply("no-concern")) }))
	defer srv.Close()
	dir, s, p, q := classifierFixtures(t, srv.URL+"/v1", "")
	lines := make([]string, 25)
	for i := range lines {
		lines[i] = fmt.Sprintf("synthetic line %02d", i)
	}
	data := []byte(strings.Join(lines, "\n") + "\n")
	if e := os.WriteFile(filepath.Join(dir, "sample.txt"), data, 0600); e != nil {
		t.Fatal(e)
	}
	s.PermittedClassifierSHA256 = []string{digest(data)}
	q.Files[0].SHA256 = digest(data)
	q.EndLine = 25
	p.Layers[0].Limits.MaxResults = 32
	r := run(dir, s, p, q, "compact")
	if r.Status != "no-findings" || r.ExternalClassifier == nil {
		t.Fatalf("%+v", r)
	}
	var compact bytes.Buffer
	if emit(&compact, r, "compact", 0) != 0 {
		t.Fatal("compact emit")
	}
	var compactMap map[string]json.RawMessage
	if json.Unmarshal(compact.Bytes(), &compactMap) != nil {
		t.Fatal("compact decode")
	}
	var version string
	json.Unmarshal(compactMap["version"], &version)
	if version != "command-guard.compact-report/1" || compactMap["externalClassifier"] == nil {
		t.Fatalf("compact version=%s", version)
	}
	cueClassifierVetRaw(t, "#OutputReport", compact.Bytes(), true)
	var textOut bytes.Buffer
	if emit(&textOut, r, "text", 0) != 0 {
		t.Fatal("text emit")
	}
	cueClassifierVetRaw(t, "#OutputReport", textOut.Bytes(), true)
	pb, _ := json.Marshal(p)
	var policyMap map[string]any
	json.Unmarshal(pb, &policyMap)
	for _, field := range []string{"profile", "endpoint", "model", "expectedResponseModel", "criterion", "criterionSHA256", "timeoutMillis", "maxInputBytes", "maxResponseBytes", "maxTokens"} {
		t.Run("policy-missing-"+field, func(t *testing.T) {
			b, _ := json.Marshal(policyMap)
			var m map[string]any
			json.Unmarshal(b, &m)
			delete(m["classifier"].(map[string]any), field)
			cueClassifierVet(t, "#Policy", m, false)
		})
	}
	rb, _ := json.Marshal(r)
	var reportMap map[string]any
	json.Unmarshal(rb, &reportMap)
	for _, field := range []string{"profile", "status", "code", "expectedFiles", "selectedFiles", "selectedBytes", "attemptedRequests", "completedDecisions", "requestBytes", "responseBytes"} {
		t.Run("receipt-missing-"+field, func(t *testing.T) {
			b, _ := json.Marshal(reportMap)
			var m map[string]any
			json.Unmarshal(b, &m)
			delete(m["externalClassifier"].(map[string]any), field)
			cueClassifierVet(t, "#OutputReport", m, false)
		})
	}
}

func TestClassifierScannerPrecedence(t *testing.T) {
	for _, kind := range []string{"finding", "incomplete"} {
		t.Run(kind, func(t *testing.T) {
			var httpCalls atomic.Int32
			peer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { httpCalls.Add(1) }))
			defer peer.Close()
			root, s, p, q := classifierFixtures(t, peer.URL+"/v1", "")
			scanner := testScannerPolicy(t)
			p.Scanner = &scanner
			s.RequiredScannerIDs = []string{"gitleaks"}
			policyBytes, e := json.Marshal(p)
			if e != nil {
				t.Fatal(e)
			}
			p.sourceSHA256 = digest(policyBytes)
			s.ClassifierPolicySHA256 = p.sourceSHA256
			scannerCalls := 0
			runner := func(_ context.Context, _, _ string, input []byte, _ int, args ...string) scannerRunResult {
				if len(args) == 1 && args[0] == "version" {
					return scannerRunResult{out: []byte("8.30.1\n"), started: true}
				}
				scannerCalls++
				if kind == "incomplete" {
					return scannerRunResult{started: true, code: "execution_incomplete"}
				}
				return scannerRunResult{out: []byte(`[{"RuleID":"generic-api-key","Secret":"[REDACTED]","Match":"[REDACTED]","File":"stdin","StartLine":1,"Tags":[]}]`), errout: scannerTestLog(len(input), 1), exit: 3, started: true}
			}
			r := runWithScanner(root, s, p, q, "json", runner)
			wantStatus, wantCode := "blocked", "scanner_finding"
			if kind == "incomplete" {
				wantStatus, wantCode = "incomplete", "scanner_incomplete"
			}
			if r.Status != wantStatus || r.Code != wantCode || r.FilesRead != 1 || r.BytesScanned == 0 || len(r.Rows) != 0 || httpCalls.Load() != 0 || scannerCalls != 1 || r.ExternalScanner == nil || r.ExternalScanner.AttemptedFiles != 1 || r.ExternalScanner.InvokedFiles != 1 || r.ExternalClassifier == nil || r.ExternalClassifier.Status != "not-invoked" || r.ExternalClassifier.AttemptedRequests != 0 {
				t.Fatalf("result=%+v scannerCalls=%d httpCalls=%d", r, scannerCalls, httpCalls.Load())
			}
			if kind == "finding" && (r.ExternalScanner.Findings != 1 || r.ExternalScanner.CompletedFiles != 1) {
				t.Fatalf("scanner=%+v", r.ExternalScanner)
			}
			if kind == "incomplete" && (r.ExternalScanner.Code != "execution_incomplete" || r.ExternalScanner.CompletedFiles != 0) {
				t.Fatalf("scanner=%+v", r.ExternalScanner)
			}
		})
	}
}

func TestTaggedClassifierHelp(t *testing.T) {
	var out bytes.Buffer
	if !classifierBuildEnabled || execute([]string{"help"}, &out) != 0 || !bytes.Contains(out.Bytes(), []byte(classifierCapabilityLine)) {
		t.Fatalf("help=%q", out.String())
	}
}

func repinClassifierPolicy(t *testing.T, s *scope, p *policy) {
	t.Helper()
	b, e := json.Marshal(p)
	if e != nil {
		t.Fatal(e)
	}
	p.sourceSHA256 = digest(b)
	s.ClassifierPolicySHA256 = p.sourceSHA256
}

func TestClassifierAggregateInputLimitsBeforeHTTP(t *testing.T) {
	for _, kind := range []string{"over-limit", "zero"} {
		t.Run(kind, func(t *testing.T) {
			var calls atomic.Int32
			peer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
			defer peer.Close()
			root, s, p, q := classifierFixtures(t, peer.URL+"/v1", "")
			if kind == "over-limit" {
				p.Classifier.MaxInputBytes = 40
				repinClassifierPolicy(t, &s, &p)
			} else {
				empty := []byte{}
				if e := os.WriteFile(filepath.Join(root, "sample.txt"), empty, 0600); e != nil {
					t.Fatal(e)
				}
				q.Files[0].SHA256 = digest(empty)
				s.PermittedClassifierSHA256 = []string{digest(empty)}
			}
			r := run(root, s, p, q, "json")
			wantBytes := 41
			if kind == "zero" {
				wantBytes = 0
			}
			if r.Status != "incomplete" || r.Code != "classifier_incomplete" || r.ExternalClassifier == nil || r.ExternalClassifier.Code != "input_limit" || r.ExternalClassifier.SelectedFiles != 1 || r.ExternalClassifier.SelectedBytes != wantBytes || r.ExternalClassifier.AttemptedRequests != 0 || r.ExternalClassifier.RequestBytes != 0 || r.ExternalClassifier.ResponseBytes != 0 || len(r.Rows) != 0 || calls.Load() != 0 {
				t.Fatalf("%s result=%+v calls=%d", kind, r, calls.Load())
			}
		})
	}
}
func TestClassifierEncodedRequestLimitBeforeHTTP(t *testing.T) {
	var calls atomic.Int32
	peer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer peer.Close()
	root, s, p, q := classifierFixtures(t, peer.URL+"/v1", "")
	source := []byte(strings.Repeat("<", 16384))
	if e := os.WriteFile(filepath.Join(root, "sample.txt"), source, 0600); e != nil {
		t.Fatal(e)
	}
	q.Files[0].SHA256 = digest(source)
	s.PermittedClassifierSHA256 = []string{digest(source)}
	p.Layers[0].Limits.MaxFileBytes = 16384
	p.Layers[0].Limits.MaxTotalBytes = 16384
	p.Classifier.Criterion = strings.Repeat("<", 4096)
	p.Classifier.CriterionSHA256 = digest([]byte(p.Classifier.Criterion))
	repinClassifierPolicy(t, &s, &p)
	r := run(root, s, p, q, "json")
	if r.Status != "incomplete" || r.Code != "classifier_incomplete" || r.ExternalClassifier == nil || r.ExternalClassifier.Code != "request_limit" || r.ExternalClassifier.SelectedBytes != 16384 || r.ExternalClassifier.RequestBytes != 131073 || r.ExternalClassifier.AttemptedRequests != 0 || r.ExternalClassifier.ResponseBytes != 0 || len(r.Rows) != 0 || calls.Load() != 0 {
		t.Fatalf("result=%+v calls=%d", r, calls.Load())
	}
}
func TestClassifierResponseHeaderLimit(t *testing.T) {
	var calls atomic.Int32
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("X-Oversized", strings.Repeat("x", 9<<10))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, reply("no-concern"))
	}))
	defer peer.Close()
	root, s, p, q := classifierFixtures(t, peer.URL+"/v1", "")
	r := run(root, s, p, q, "json")
	if r.Status != "incomplete" || r.Code != "classifier_incomplete" || r.ExternalClassifier == nil || r.ExternalClassifier.AttemptedRequests != 1 || r.ExternalClassifier.CompletedDecisions != 0 || r.ExternalClassifier.ResponseBytes != 0 || len(r.Rows) != 0 || calls.Load() != 1 {
		t.Fatalf("result=%+v calls=%d", r, calls.Load())
	}
}
func TestClassifierCancelAfterRequestArrival(t *testing.T) {
	arrived := make(chan struct{})
	release := make(chan struct{})
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(arrived)
		<-release
		_, _ = io.WriteString(w, reply("no-concern"))
	}))
	defer peer.Close()
	_, _, p, _ := classifierFixtures(t, peer.URL+"/v1", "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	receipt := newClassifierReceipt(1)
	finished := make(chan string, 1)
	go func() {
		finished <- classifySelectedContext(ctx, *p.Classifier, [][]byte{[]byte("synthetic")}, receipt)
	}()
	select {
	case <-arrived:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("request never arrived")
	}
	cancel()
	close(release)
	select {
	case result := <-finished:
		if result != "incomplete" || receipt.Status != "incomplete" || receipt.Code != "deadline" || receipt.AttemptedRequests != 1 || receipt.CompletedDecisions != 0 || len(receipt.Code) == 0 {
			t.Fatalf("result=%s receipt=%+v", result, receipt)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not finish")
	}
}

func TestClassifierConfiguredDeadlineAfterRequestArrival(t *testing.T) {
	arrived := make(chan struct{})
	release := make(chan struct{})
	peer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { close(arrived); <-release }))
	defer peer.Close()
	defer close(release)
	_, _, p, _ := classifierFixtures(t, peer.URL+"/v1", "")
	p.Classifier.TimeoutMillis = 2000
	receipt := newClassifierReceipt(1)
	finished := make(chan string, 1)
	go func() {
		finished <- classifySelectedContext(context.Background(), *p.Classifier, [][]byte{[]byte("synthetic")}, receipt)
	}()
	select {
	case <-arrived:
	case <-time.After(5 * time.Second):
		t.Fatal("request never arrived")
	}
	select {
	case result := <-finished:
		if result != "incomplete" || receipt.Code != "deadline" || receipt.AttemptedRequests != 1 || receipt.CompletedDecisions != 0 {
			t.Fatalf("result=%s receipt=%+v", result, receipt)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("configured deadline did not finish")
	}
}

func TestClassifierWireRejectsSecondAttempt(t *testing.T) {
	base := decision.Request{State: "synthetic state", Questions: []decision.Question{{ID: "sensitivity", Kind: decision.KindChoice, Instructions: "synthetic criterion", Options: []decision.Option{{Name: "concern"}, {Name: "no-concern"}, {Name: "unknown"}}}}}
	questionID := "sensitivity." + decision.RequestKey(base)
	body := strings.ReplaceAll(reply("no-concern"), `\"sensitivity\"`, `\"`+questionID+`\"`)
	var peerCalls, dials atomic.Int32
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { peerCalls.Add(1); _, _ = io.WriteString(w, body) }))
	defer peer.Close()
	endpoint := peer.URL + "/v1"
	receipt := newClassifierReceipt(1)
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, DisableCompression: true, MaxResponseHeaderBytes: 8 << 10, DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		dials.Add(1)
		var dialer net.Dialer
		return dialer.DialContext(ctx, network, addr)
	}}
	defer transport.CloseIdleConnections()
	wire := &classifierWire{policy: classifierPolicy{Endpoint: endpoint, ExpectedResponseModel: "response", MaxResponseBytes: 16384}, questionID: questionID, receipt: receipt, transport: transport}
	newRequest := func() *http.Request {
		t.Helper()
		r, e := http.NewRequest(http.MethodPost, endpoint+"/chat/completions", strings.NewReader("{}"))
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	response, e := wire.RoundTrip(newRequest())
	if e != nil {
		t.Fatalf("first attempt: %v", e)
	}
	response.Body.Close()
	if !wire.completed || receipt.AttemptedRequests != 1 || receipt.RequestBytes != 2 || receipt.ResponseBytes == 0 || peerCalls.Load() != 1 || dials.Load() != 1 {
		t.Fatalf("first witness=%+v calls=%d dials=%d", receipt, peerCalls.Load(), dials.Load())
	}
	extra, e := wire.RoundTrip(newRequest())
	if e == nil || extra != nil || wire.attempts.Load() != 2 || receipt.AttemptedRequests != 1 || peerCalls.Load() != 1 || dials.Load() != 1 {
		t.Fatalf("second response=%v err=%v receipt=%+v attempts=%d calls=%d dials=%d", extra, e, receipt, wire.attempts.Load(), peerCalls.Load(), dials.Load())
	}
}
