package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

type fakeDecider struct{ response decision.Response }

func (f fakeDecider) Name() string { return "fake" }
func (f fakeDecider) Decide(_ context.Context, _ decision.Request) (decision.Response, error) {
	return f.response, nil
}

func baseAssess() (Scope, Policy, Operation) {
	return Scope{Version: "command-lab.scope/1", RequiredLayerIDs: []string{"org", "project"}}, Policy{Version: "command-lab.policy/1", Layers: []Layer{{ID: "org", Scope: "organization", AllowedOperations: []string{"read-range", "git-status"}, MaxBytes: 4096, MaxResults: 20}, {ID: "project", Scope: "project", AllowedOperations: []string{"read-range"}, MaxBytes: 2048, MaxResults: 10}}, Classifier: &Classifier{Provider: "offline", ResolvedModel: "captured-v1"}}, Operation{Version: "command-lab.request/1", ID: "case", Kind: "read-range", Path: "docs/a.md", MaxBytes: 1024, MaxResults: 1}
}
func TestAssessIntersectionAndOrder(t *testing.T) {
	s, p, r := baseAssess()
	a, e := assess(s, p, r, nil, decision.None{})
	if e != nil || a.Status != "advisory" || a.EffectiveMaxBytes != 2048 || !reflect.DeepEqual(a.EffectiveAllowed, []string{"read-range"}) {
		t.Fatalf("unexpected result: %+v %v", a, e)
	}
	p.Layers[0], p.Layers[1] = p.Layers[1], p.Layers[0]
	b, e := assess(s, p, r, nil, decision.None{})
	if e != nil || !reflect.DeepEqual(a, b) {
		t.Fatalf("order changed result: %+v %+v %v", a, b, e)
	}
	r.MaxBytes = 4096
	c, _ := assess(s, p, r, nil, decision.None{})
	if c.Status != "blocked" || !contains(c.Codes, "bytes_exceeded") {
		t.Fatal(c)
	}
	p.Layers[0].AllowedOperations = append(p.Layers[0].AllowedOperations, "search-literal")
	d, _ := assess(s, p, Operation{Version: r.Version, ID: r.ID, Kind: "search-literal", MaxBytes: 1, MaxResults: 1}, nil, decision.None{})
	if !contains(d.Codes, "operation_disallowed") {
		t.Fatal(d)
	}
}
func TestAssessPrivacyAndBlockers(t *testing.T) {
	s, p, r := baseAssess()
	secret := "SENTINEL_PRIVATE_TOKEN_987"
	r.Path = "../" + secret
	r.Command = "--token=" + secret
	out, e := assess(s, p, r, nil, decision.None{})
	if e != nil || out.Status != "blocked" || !contains(out.Codes, "path_traversal") || !contains(out.Codes, "credential_signal") {
		t.Fatalf("%+v %v", out, e)
	}
	b, _ := json.Marshal(out)
	if strings.Contains(string(b), secret) {
		t.Fatal("secret in output")
	}
	r.Kind = "shell"
	r.Path = ""
	r.Command = "echo safe"
	out, _ = assess(s, p, r, nil, decision.None{})
	if !contains(out.Codes, "unsupported_effect") {
		t.Fatal(out)
	}
}
func TestCaptureValidation(t *testing.T) {
	s, p, r := baseAssess()
	out, _ := assess(s, p, r, nil, decision.None{})
	a := decision.Answer{ID: "command-concern", Kind: decision.KindChoice, Choice: "no-concern", Probabilities: map[string]float64{"concern": 0, "no-concern": 1, "unknown": 0}, Confidence: 1}
	c := Capture{Version: "command-lab.capture/1", RequestKey: out.RequestKey, Response: &PortResponse{Provider: "offline", Model: "captured-v1", Answers: map[string]decision.Answer{"command-concern": a}, Invalid: map[string]string{}}}
	got, e := assess(s, p, r, &c, nil)
	if e != nil || got.ClassifierStatus != "answered" || got.Status != "advisory" {
		t.Fatalf("%+v %v", got, e)
	}
	r.ID = "changed"
	got, e = assess(s, p, r, &c, nil)
	if e == nil || got.Status != "incomplete" {
		t.Fatalf("stale capture: %+v %v", got, e)
	}
	r.ID = "case"
	c.Response.Answers["foreign"] = a
	if _, e = assess(s, p, r, &c, nil); e == nil {
		t.Fatal("foreign answer accepted")
	}
	delete(c.Response.Answers, "foreign")
	c.Response.Invalid = map[string]string{"command-concern": "SENTINEL_BAD_REASON"}
	if _, e = assess(s, p, r, &c, nil); e == nil {
		t.Fatal("overlap accepted")
	}
	c.Response.Invalid = map[string]string{}
	a.Confidence = .5
	c.Response.Answers["command-concern"] = a
	if _, e = assess(s, p, r, &c, nil); e == nil {
		t.Fatal("contradictory confidence accepted")
	}
}

func TestCapturedNativeModelReferences(t *testing.T) {
	for _, model := range []string{"local/classifier:revision", "registry/model@sha256:abcdef", "deployment.model-v1:0"} {
		t.Run(model, func(t *testing.T) {
			s, p, r := baseAssess()
			p.Classifier.ResolvedModel = model
			prepared, err := assess(s, p, r, nil, decision.None{})
			if err != nil {
				t.Fatal("native model reference rejected", err)
			}
			a := decision.Answer{ID: "command-concern", Kind: decision.KindChoice, Choice: "unknown", Probabilities: map[string]float64{"concern": 0, "no-concern": 0, "unknown": 1}, Confidence: 1}
			c := Capture{Version: "command-lab.capture/1", RequestKey: prepared.RequestKey, Response: &PortResponse{Provider: p.Classifier.Provider, Model: model, Answers: map[string]decision.Answer{a.ID: a}, Invalid: map[string]string{}}}
			got, err := assess(s, p, r, &c, nil)
			if err != nil || got.Status != "advisory" || !got.NoActionAuthority || got.Choice != "unknown" {
				t.Fatalf("capture: %+v %v", got, err)
			}
			if _, err := exec.LookPath("cue"); err == nil {
				for def, value := range map[string]any{"#Policy": p, "#Capture": c} {
					body, err := json.Marshal(value)
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(t.TempDir(), "value.json")
					if err := os.WriteFile(path, body, 0600); err != nil {
						t.Fatal(err)
					}
					if out, err := exec.Command("cue", "vet", "schema.cue", path, "-d", def).CombinedOutput(); err != nil {
						t.Fatalf("CUE %s: %v %s", def, err, out)
					}
				}
			}
			c.Response.Model += "-different"
			if _, err := assess(s, p, r, &c, nil); err == nil {
				t.Fatal("changed native model identity accepted")
			}
		})
	}
	for _, model := range []string{"", "model\nforged", "model name", strings.Repeat("m", 257)} {
		s, p, r := baseAssess()
		p.Classifier.ResolvedModel = model
		if _, err := assess(s, p, r, nil, decision.None{}); err == nil {
			t.Fatal("malformed model reference accepted")
		}
	}
}

func TestClassifierSeam(t *testing.T) {
	s, p, r := baseAssess()
	baseline, _ := assess(s, p, r, nil, decision.None{})
	if baseline.ClassifierStatus != "unknown" {
		t.Fatal(baseline)
	}
	a := decision.Answer{ID: "command-concern", Kind: decision.KindChoice, Choice: "concern", Probabilities: map[string]float64{"concern": 1, "no-concern": 0, "unknown": 0}, Confidence: 1}
	response := decision.Response{Provider: "offline", Model: "captured-v1", Answers: map[string]decision.Answer{"command-concern": a}}
	got, e := assess(s, p, r, nil, fakeDecider{response})
	if e != nil || got.Choice != "concern" || got.Status != "advisory" {
		t.Fatalf("fake: %+v %v", got, e)
	}
	req := projectedRequest(s, p, r, nil)
	replay, e := decision.NewReplay(decision.Transcript{Version: decision.TranscriptVersion, Decider: "offline", Entries: []decision.TranscriptEntry{{Key: decision.RequestKey(req), Request: req, Response: response}}}, decision.ReplayOptions{})
	if e != nil {
		t.Fatal(e)
	}
	got, e = assess(s, p, r, nil, replay)
	if e != nil || got.Choice != "concern" {
		t.Fatalf("replay: %+v %v", got, e)
	}
	r.Kind = "shell"
	r.Command = "echo test"
	got, e = assess(s, p, r, nil, fakeDecider{response})
	if e != nil || got.Status != "blocked" || got.Choice != "" {
		t.Fatalf("blocker: %+v %v", got, e)
	}
}
func TestStrictJSON(t *testing.T) {
	for _, payload := range []string{`{"version":"command-lab.scope/1","version":"duplicate","requiredLayerIds":["x"]}`, `{"version":"command-lab.scope/1","requiredLayerIds":null}`, `{"Version":"command-lab.scope/1","requiredLayerIds":["x"]}`, `{"version":"command-lab.scope/1","requiredLayerIds":["x"]} true`, strings.Repeat("[", 25) + `0` + strings.Repeat("]", 25)} {
		p := filepath.Join(t.TempDir(), "input.json")
		if e := os.WriteFile(p, []byte(payload), 0600); e != nil {
			t.Fatal(e)
		}
		var s Scope
		if e := readStrict(p, &s); e == nil {
			t.Fatalf("accepted %q", payload)
		}
	}
}

func TestUnicodeWireBeforeDecode(t *testing.T) {
	for _, tc := range []struct {
		name, wire string
		valid      bool
	}{
		{"high-surrogate", `\ud800`, false},
		{"low-surrogate", `\udc00`, false},
		{"wrong-pair", `\ud800\u0041`, false},
		{"paired", `\ud83d\ude00`, true},
		{"escaped-backslash", `\\ud800`, true},
		{"replacement-literal", "\ufffd", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "value.json")
			control := `{"version":"command-lab.request/1","id":"one","kind":"read-range","path":"` + tc.wire + `","maxBytes":1,"maxResults":1}`
			if err := os.WriteFile(path, []byte(control), 0600); err != nil {
				t.Fatal(err)
			}
			var op Operation
			if err := readStrict(path, &op); (err == nil) != tc.valid {
				t.Fatalf("control valid=%v error=%v", tc.valid, err)
			}
			var report = AuditReport{Proposals: zeroCounts(families), Candidates: zeroCounts(families), GitOperations: zeroCounts(gitOps), Failures: zeroCounts([]string{"malformed", "unsupported"})}
			var source = SourceReport{Status: "complete"}
			auditLine("normalized", []byte(`{"tool":"shell_command","command":"echo `+tc.wire+`"}`), &report, &source)
			if (report.Failures["malformed"] == 0) != tc.valid {
				t.Fatalf("audit valid=%v report=%+v", tc.valid, report)
			}
		})
	}
}
func TestAuditCoverageBoundsPrivacy(t *testing.T) {
	dir := t.TempDir()
	secret := "SENTINEL_EXEC_NAME_123"
	p := filepath.Join(dir, secret+".jsonl")
	data := `{"tool":"shell_command","command":"git status"}` + "\n" + `{"tool":"functions.exec","command":"const r=tools.exec_command({cmd: \"rg x\\ncat y\"})"}` + "\n"
	if e := os.WriteFile(p, []byte(data), 0600); e != nil {
		t.Fatal(e)
	}
	m := SourceManifest{Version: "command-lab.manifest/1", ExpectedSourceIDs: []string{secret}, Sources: []Source{{ID: secret, Format: "normalized", Path: p}}}
	got := audit(m)
	if got.Status != "incomplete" || got.Failures["unsupported"] != 1 || got.Proposals["git"] != 1 || got.Candidates["search"] != 1 || got.Candidates["read"] != 1 {
		t.Fatal(got)
	}
	b, _ := json.Marshal(got)
	if bytes.Contains(b, []byte(secret)) {
		t.Fatal("source metadata leaked")
	}
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0600)
	_, _ = f.WriteString(`{"tool":"functions.exec","command":"dynamic()"}` + "\n")
	_ = f.Close()
	got = audit(m)
	if got.Status != "incomplete" || got.Failures["unsupported"] != 2 || got.Records != 3 {
		t.Fatal(got)
	}
	_ = os.WriteFile(p, []byte(strings.Repeat("a", maxLine+2)), 0600)
	got = audit(m)
	if got.Status != "incomplete" || got.Failures["line_oversize"] != 1 || got.Bytes != maxLine+1 {
		t.Fatal(got)
	}
	m.Sources = append(m.Sources, m.Sources[0])
	got = audit(m)
	if got.Failures["manifest"] == 0 {
		t.Fatal("duplicate source accepted")
	}
}

func TestAuditMixedJSAndTypedNormalized(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "events.jsonl")
	data := `{"tool":"functions.exec","command":"const a=tools.exec_command({cmd: \"git status\"}); const b=tools.exec_command({cmd: dynamic});"}` + "\n" + `{"tool":"functions.exec","command":"const run=tools.exec_command; run(dynamic); tools.exec_command({cmd: \"git status\"});"}` + "\n" + `{"tool":"shell_command","command":123}` + "\n"
	if e := os.WriteFile(p, []byte(data), 0600); e != nil {
		t.Fatal(e)
	}
	m := SourceManifest{Version: "command-lab.manifest/1", ExpectedSourceIDs: []string{"one"}, Sources: []Source{{ID: "one", Format: "normalized", Path: p}}}
	r := audit(m)
	if r.Status != "incomplete" || r.Failures["unsupported"] != 2 || r.Failures["malformed"] != 1 || r.Candidates["git"] != 2 || r.Proposals["other"] != 0 {
		t.Fatal(r)
	}
}

func TestJSWrappersAlwaysIncomplete(t *testing.T) {
	cases := []string{
		`tools.exec_command({cmd: "git status"})`,
		`const run=tools.exec_command; run(dynamic); tools.exec_command({cmd: "git status"})`,
		`tools["exec_command"]({cmd: dynamic})`,
		`const decoy = 'cmd: "git status"'; run(dynamic)`,
	}
	var input strings.Builder
	for _, code := range cases {
		b, _ := json.Marshal(map[string]string{"tool": "functions.exec", "command": code})
		input.Write(b)
		input.WriteByte('\n')
	}
	p := filepath.Join(t.TempDir(), "js.jsonl")
	if e := os.WriteFile(p, []byte(input.String()), 0600); e != nil {
		t.Fatal(e)
	}
	m := SourceManifest{Version: "command-lab.manifest/1", ExpectedSourceIDs: []string{"one"}, Sources: []Source{{ID: "one", Format: "normalized", Path: p}}}
	r := audit(m)
	if r.Status != "incomplete" || r.Failures["unsupported"] != len(cases) || r.Records != len(cases) || r.Proposals["git"] != 0 {
		t.Fatal(r)
	}
}

func TestNativeCommandFieldMustBeString(t *testing.T) {
	cases := []struct{ format, line string }{
		{"codex", `{"type":"function_call","name":"functions.exec_command","arguments":"{\"cmd\":42}"}`},
		{"codex", `{"type":"function_call","name":"functions.exec_command","arguments":"{}"}`},
		{"claude", `{"type":"tool_use","name":"Bash","input":{"command":42}}`},
		{"claude", `{"type":"tool_use","name":"Bash","input":{}}`},
	}
	for _, tc := range cases {
		t.Run(tc.format+tc.line, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "events.jsonl")
			if e := os.WriteFile(p, []byte(tc.line+"\n"), 0600); e != nil {
				t.Fatal(e)
			}
			m := SourceManifest{Version: "command-lab.manifest/1", ExpectedSourceIDs: []string{"one"}, Sources: []Source{{ID: "one", Format: tc.format, Path: p}}}
			r := audit(m)
			if r.Status != "incomplete" || r.Proposals["other"] != 0 || r.Proposals["shell"] != 0 {
				t.Fatal(r)
			}
		})
	}
}

func TestAuditDuplicateKeysIncomplete(t *testing.T) {
	cases := []string{
		`{"tool":"Read","tool":"Bash","command":""}`,
		`{"type":"tool_use","name":"Bash","input":{"command":"git status","command":"curl example"}}`,
		`{"type":"function_call","name":"functions.exec_command","arguments":"{\"cmd\":\"git status\",\"cmd\":\"curl example\"}"}`,
	}
	for i, line := range cases {
		format := "normalized"
		if i == 1 {
			format = "claude"
		} else if i == 2 {
			format = "codex"
		}
		p := filepath.Join(t.TempDir(), "events.jsonl")
		if e := os.WriteFile(p, []byte(line+"\n"), 0600); e != nil {
			t.Fatal(e)
		}
		m := SourceManifest{Version: "command-lab.manifest/1", ExpectedSourceIDs: []string{"one"}, Sources: []Source{{ID: "one", Format: format, Path: p}}}
		r := audit(m)
		if r.Status != "incomplete" || r.Failures["malformed"] != 1 || r.Proposals["other"] != 0 {
			t.Fatal(r)
		}
	}
}

func TestAuditAllowsNullMetadata(t *testing.T) {
	p := filepath.Join(t.TempDir(), "events.jsonl")
	if e := os.WriteFile(p, []byte(`{"tool":"Read","command":"","metadata":null}`+"\n"), 0600); e != nil {
		t.Fatal(e)
	}
	m := SourceManifest{Version: "command-lab.manifest/1", ExpectedSourceIDs: []string{"one"}, Sources: []Source{{ID: "one", Format: "normalized", Path: p}}}
	r := audit(m)
	if r.Status != "complete" || r.Proposals["read"] != 1 {
		t.Fatal(r)
	}
}

func TestNonregularInputsRejected(t *testing.T) {
	dir := t.TempDir()
	var s Scope
	if e := readStrict(dir, &s); e == nil {
		t.Fatal("directory accepted as control input")
	}
	m := SourceManifest{Version: "command-lab.manifest/1", ExpectedSourceIDs: []string{"one"}, Sources: []Source{{ID: "one", Format: "normalized", Path: dir}}}
	r := audit(m)
	if r.Status != "incomplete" || r.Failures["source_unreadable"] != 1 {
		t.Fatal(r)
	}
}

type trackedBytes struct {
	data                        []byte
	position, calls, maxRequest int
}

func (t *trackedBytes) Read(p []byte) (int, error) {
	t.calls++
	if len(p) > t.maxRequest {
		t.maxRequest = len(p)
	}
	if t.position >= len(t.data) {
		return 0, io.EOF
	}
	n := copy(p, t.data[t.position:])
	t.position += n
	return n, nil
}

func TestLineReaderPhysicalLimit(t *testing.T) {
	prefix := []byte(`{"tool":"Read","command":""}` + "\n")
	data := append(append([]byte(nil), prefix...), bytes.Repeat([]byte{'x'}, maxLine+100000)...)
	tracked := &trackedBytes{data: data}
	counted := &countingReader{inner: tracked}
	reader := &lineReader{inner: counted}
	first, e := reader.readLine()
	if e != nil || !bytes.Equal(first, prefix) {
		t.Fatalf("first line: %d %v", len(first), e)
	}
	_, e = reader.readLine()
	if e != errLineOverflow || counted.bytes != int64(len(prefix)+maxLine+1) || tracked.calls >= maxLine/1000 || tracked.maxRequest > 32<<10 {
		t.Fatalf("overflow: %v bytes=%d calls=%d max=%d", e, counted.bytes, tracked.calls, tracked.maxRequest)
	}
}

func TestCaptureShapeMatchesCUE(t *testing.T) {
	p := filepath.Join(t.TempDir(), "capture.json")
	b, e := os.ReadFile("fixtures/capture.json")
	if e != nil {
		t.Fatal(e)
	}
	b = bytes.ReplaceAll(b, []byte(`,"Score":0,"Noul":0`), nil)
	if e = os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
	var c Capture
	if e = readStrict(p, &c); e != nil {
		t.Fatalf("Go rejected CUE-valid choice answer: %v", e)
	}
	if c.Response == nil || c.Response.Answers["command-concern"].Score != 0 || c.Response.Answers["command-concern"].Noul != 0 {
		t.Fatal("wrong zero values")
	}
}

func TestAuditExactByteCaps(t *testing.T) {
	dir := t.TempDir()
	valid := `{"tool":"Read","command":""}`
	line := valid + strings.Repeat(" ", maxLine-len(valid)-1) + "\n"
	p := filepath.Join(dir, "exact.jsonl")
	if e := os.WriteFile(p, []byte(strings.Repeat(line, 4)), 0600); e != nil {
		t.Fatal(e)
	}
	m := SourceManifest{Version: "command-lab.manifest/1", ExpectedSourceIDs: []string{"s1"}, Sources: []Source{{ID: "s1", Format: "normalized", Path: p}}}
	r := audit(m)
	if r.Status != "complete" || r.Bytes != maxSource || r.Records != 4 {
		t.Fatal(r)
	}
	f, e := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.Write([]byte{'x'})
	if e != nil {
		t.Fatal(e)
	}
	_ = f.Close()
	r = audit(m)
	if r.Status != "incomplete" || r.Bytes != maxSource+1 || r.Failures["source_oversize"] != 1 {
		t.Fatal(r)
	}
	if e := os.WriteFile(p, []byte(line+"x"), 0600); e != nil {
		t.Fatal(e)
	}
	m.ExpectedSourceIDs = nil
	m.Sources = nil
	for i := 0; i < 16; i++ {
		id := fmt.Sprintf("s%d", i)
		m.ExpectedSourceIDs = append(m.ExpectedSourceIDs, id)
		m.Sources = append(m.Sources, Source{ID: id, Format: "normalized", Path: filepath.Join(dir, "exact16.jsonl")})
	}
	if e := os.WriteFile(filepath.Join(dir, "exact16.jsonl"), []byte(strings.Repeat(line, 4)), 0600); e != nil {
		t.Fatal(e)
	}
	m.ExpectedSourceIDs = append(m.ExpectedSourceIDs, "overflow")
	m.Sources = append(m.Sources, Source{ID: "overflow", Format: "normalized", Path: p})
	r = audit(m)
	if r.Status != "incomplete" || r.Bytes != maxTotal+1 || r.Failures["total_oversize"] != 1 {
		t.Fatal(r)
	}
	long := strings.Repeat("x", maxLine+100000)
	if e := os.WriteFile(p, []byte(long), 0600); e != nil {
		t.Fatal(e)
	}
	m.ExpectedSourceIDs = []string{"line"}
	m.Sources = []Source{{ID: "line", Format: "normalized", Path: p}}
	r = audit(m)
	if r.Status != "incomplete" || r.Bytes != maxLine+1 || r.Failures["line_oversize"] != 1 {
		t.Fatal(r)
	}
	prefix := `{"tool":"Read","command":""}` + "\n"
	if e := os.WriteFile(p, []byte(prefix+long), 0600); e != nil {
		t.Fatal(e)
	}
	r = audit(m)
	if r.Status != "incomplete" || r.Bytes != int64(len(prefix)+maxLine+1) || r.Records != 1 || r.Failures["line_oversize"] != 1 {
		t.Fatal(r)
	}
}

func TestEmptyCLIHasSafetyStatement(t *testing.T) {
	var b bytes.Buffer
	if c := run(nil, &b); c != 2 || !bytes.Contains(b.Bytes(), []byte(`"noActionAuthority": true`)) {
		t.Fatalf("%d %s", c, b.String())
	}
}
func TestCLIOutputError(t *testing.T) {
	w := failWriter{}
	if code := run([]string{"nope"}, w); code != 2 {
		t.Fatal(code)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, os.ErrClosed }
