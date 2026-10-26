package main

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

func TestFixtureProfileKeepsPriorRosterHashes(t *testing.T) {
	if got := rosterHash("files-core/1"); got != "12e1c94081dc4a4bf6bda89f5f82079df3aca19d5238a4a6167b138b21106dfc" {
		t.Fatalf("core roster changed: %s", got)
	}
	if got := rosterHash("files-gitleaks/1"); got != "8a08389724e92c31a1c2b104b0eec84d77fabfbff441b8fac4155db73966fce4" {
		t.Fatalf("scanner roster changed: %s", got)
	}
	r := newReport("files-loopback-fixture/1", strings.Repeat("a", 64), "")
	if !validQualification(r) || r.Required != 30 || !r.SyntheticPeerOnly || r.Scanner != nil {
		t.Fatal("fixture receipt shape")
	}
	r.SyntheticPeerOnly = false
	if validQualification(r) {
		t.Fatal("missing synthetic marker accepted")
	}
}
func TestFixturePeerRejectsCredentialsAndCountsRequests(t *testing.T) {
	f := classifierFixture("classifier-no-concern")
	p, err := startFixturePeer(f)
	if err != nil {
		t.Fatal(err)
	}
	defer p.close()
	payload := []byte(`{}`)
	req, _ := http.NewRequest(http.MethodPost, p.endpoint+"/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer forbidden")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatal("credential request accepted")
	}
	p.mu.Lock()
	calls, valid, got := p.calls, p.valid, p.requestBytes
	p.mu.Unlock()
	if calls != 1 || valid || got != len(payload) {
		t.Fatal("peer failed to record rejected request")
	}
}
func TestFixturePeerClosesListener(t *testing.T) {
	p, err := startFixturePeer(classifierFixture("classifier-redirect"))
	if err != nil {
		t.Fatal(err)
	}
	first, _ := url.Parse(p.endpoint)
	second, _ := url.Parse(p.redirect.URL)
	p.close()
	for _, host := range []string{first.Host, second.Host} {
		conn, dialErr := net.DialTimeout("tcp", host, 100*time.Millisecond)
		if dialErr == nil {
			conn.Close()
			t.Fatal("fixture listener survived close")
		}
	}
}
func TestFixtureOracleRejectsAbsentPeerAndWrongCounters(t *testing.T) {
	f := classifierFixture("classifier-no-concern")
	c := classifierReport{Profile: "loopback-concern/1", Status: "complete", Code: "no_concern", ExpectedFiles: 1, SelectedFiles: 1, SelectedBytes: len(f.files[0].body), AttemptedRequests: 1, CompletedDecisions: 1, RequestBytes: 500, ResponseBytes: len(fixtureReply(f.id, fixtureQuestionID(f)))}
	report := candidateReport{Version: "command-guard.report/1", Status: f.wantStatus, Code: f.wantCode, NoActionAuthority: true, FilesRead: 1, BytesRead: f.wantRead, BytesScanned: f.wantScanned, Scope: "selected-files-only", Scanner: "configured-regex-only", ExternalClassifier: &c, Limitation: "not-proof-of-secret-or-pii-absence", TotalMatches: f.wantTotal, Returned: len(f.wantRows), Rows: f.wantRows}
	raw, _ := json.Marshal(report)
	raw = append(raw, '\n')
	p := &fixturePeer{valid: true}
	if checkClassifierCase(raw, f, p) {
		t.Fatal("absent peer accepted")
	}
	p.calls = 1
	p.requestBytes = 499
	if checkClassifierCase(raw, f, p) {
		t.Fatal("counter mismatch accepted")
	}
}
func TestFixtureQuestionIDBindsValidatedContext(t *testing.T) {
	f := classifierFixture("classifier-no-concern")
	baseID := fixtureQuestionID(f)
	const schema = `{"type":"object","properties":{"answers":{"type":"object","properties":{"sensitivity":{"type":"object","properties":{"choice":{"type":"string","enum":["concern","no-concern","unknown"]},"probabilities":{"type":"object","properties":{"concern":{"type":"number"},"no-concern":{"type":"number"},"unknown":{"type":"number"}},"required":["concern","no-concern","unknown"],"additionalProperties":false}},"required":["choice","probabilities"],"additionalProperties":false}},"required":["sensitivity"],"additionalProperties":false}},"required":["answers"],"additionalProperties":false}`
	makeBody := func(id, state string) []byte {
		prompt, _ := json.Marshal(map[string]any{"state": state, "questions": []any{map[string]any{"id": id, "kind": "choice", "instructions": fixtureInstructions, "options": []any{map[string]string{"name": "concern"}, map[string]string{"name": "no-concern"}, map[string]string{"name": "unknown"}}}}})
		var shape map[string]any
		if err := json.Unmarshal([]byte(strings.ReplaceAll(schema, `"sensitivity"`, `"`+id+`"`)), &shape); err != nil {
			t.Fatal(err)
		}
		body, _ := json.Marshal(map[string]any{"model": fixtureModel, "messages": []any{map[string]string{"role": "system", "content": "synthetic system"}, map[string]string{"role": "user", "content": string(prompt)}}, "max_completion_tokens": fixtureMaxTokens, "stream": false, "response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "response", "strict": true, "schema": shape}}})
		return body
	}
	for _, tc := range []struct {
		name, id, state string
		want            bool
	}{
		{"valid", baseID, fixtureState(f), true},
		{"static-id", "sensitivity", fixtureState(f), false},
		{"arbitrary-id", "sensitivity.sha256:" + strings.Repeat("a", 64), fixtureState(f), false},
		{"changed-state", keyedQuestionID(`[{"index":0,"bytes":1,"text":"x"}]`, fixtureInstructions, []decision.Option{{Name: "concern"}, {Name: "no-concern"}, {Name: "unknown"}}), `[{"index":0,"bytes":1,"text":"x"}]`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := makeBody(tc.id, tc.state)
			req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:1/v1/chat/completions", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			got, ok := validatedFixtureQuestionID(req, body, f)
			if ok != tc.want || ok && got != baseID {
				t.Fatal("unexpected request oracle verdict")
			}
		})
	}
}
