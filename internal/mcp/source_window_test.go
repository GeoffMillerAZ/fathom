package mcp

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

type windowBackendFixture struct{ QueryInterface }

func TestSourceWindowInputRejectsUnknownOrMalformedFields(t *testing.T) {
	for _, raw := range []string{`{"op":"begin","unexpected":true}`, `[]`, `{"op":2}`, `{"op":"begin"}{}`} {
		if _, err := parseSourceWindowInput(json.RawMessage(raw)); err == nil {
			t.Fatalf("malformed source window accepted: %s", raw)
		}
	}
	parsed, err := parseSourceWindowInput(json.RawMessage(`{"op":"begin","expected_exclusions":[{"path":"a.go","reason":"oversize"}]}`))
	if err != nil || parsed.Op != "begin" || len(parsed.Expected) == 0 {
		t.Fatalf("valid source window rejected: %+v %v", parsed, err)
	}
}

func (windowBackendFixture) BeginSourceWindow(context.Context, string) (*client.SourceWindow, error) {
	return &client.SourceWindow{Projection: store.SourceRefProjection{ServiceID: "svc", Watermark: "head@1", Rows: []store.SourceRefFact{{Path: "a.go"}}}}, nil
}

func TestSourceWindowPendingCannotPublishPassBeforeClose(t *testing.T) {
	b := windowBackendFixture{}
	s := NewServer(b)
	begin, e := windowOp(t, s, b, "svc", "begin", "")
	if e != nil {
		t.Fatal(e)
	}
	token := begin["source_window_token"].(string)
	if _, e := windowOp(t, s, b, "svc", "advance", token); e != nil {
		t.Fatal(e)
	}
	w, e2 := s.sourceWindowGet("svc", token)
	if e2 != nil {
		t.Fatal(e2)
	}
	pending, err := s.sourceWindowPending(context.Background(), w, map[string]any{"service": "svc", "summary": map[string]any{"total": 1, "pass": 1, "fail": 0, "unknown": 0}, "tasks": []any{map[string]any{"id": "T-1", "title": "row", "status": "pass", "criteria": []any{}}}})
	if err != nil {
		t.Fatal(err)
	}
	if pending["source_window_state"] != "pending" || pending["summary"].(map[string]any)["pass"] != 0 || pending["tasks"].([]map[string]any)[0]["status"] != "unknown" {
		t.Fatalf("positive leaked: %v", pending)
	}
	if _, e := windowOp(t, s, b, "svc", "close", token); e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(map[string]any{"service": "svc", "source_window": map[string]string{"op": "result", "token": token, "report_id": pending["source_window_report_id"].(string)}})
	result, rpcErr := s.sourceWindowOperation(context.Background(), b, raw)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	var final struct {
		SourceWindowState string `json:"source_window_state"`
		Tasks             []struct {
			Status string `json:"status"`
		} `json:"tasks"`
	}
	encoded, _ := json.Marshal(result)
	if err := json.Unmarshal(encoded, &final); err != nil || final.SourceWindowState != "closed" || len(final.Tasks) != 1 || final.Tasks[0].Status != "pass" {
		t.Fatalf("final=%s err=%v", encoded, err)
	}
}

func TestSourceWindowExpiredPointerCannotUseOrDeleteReplacement(t *testing.T) {
	b := windowBackendFixture{}
	s := NewServer(b)
	var stamp atomic.Int64
	stamp.Store(time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC).UnixNano())
	s.windowClock = func() time.Time { return time.Unix(0, stamp.Load()) }
	begin, e := windowOp(t, s, b, "svc", "begin", "")
	if e != nil {
		t.Fatal(e)
	}
	oldToken := begin["source_window_token"].(string)
	old, err := s.sourceWindowGet("svc", oldToken)
	if err != nil {
		t.Fatal(err)
	}
	old.mu.Lock()
	answer := make(chan error, 1)
	go func() {
		_, release, e := s.sourceWindowProjection("svc", oldToken)
		if release != nil {
			release()
		}
		answer <- e
	}()
	stamp.Add(int64(sourceWindowTTL + time.Second))
	newBegin, e := windowOp(t, s, b, "svc", "begin", "")
	if e != nil {
		old.mu.Unlock()
		t.Fatal(e)
	}
	newToken := newBegin["source_window_token"].(string)
	old.mu.Unlock()
	if e := <-answer; e == nil {
		t.Fatal("expired pointer admitted")
	}
	s.sourceWindowDelete(old)
	if _, e := s.sourceWindowGet("svc", newToken); e != nil {
		t.Fatal("old window deleted replacement", e)
	}
}

func TestSourceWindowReviewedExclusionsExact(t *testing.T) {
	p := store.SourceRefProjection{Rows: []store.SourceRefFact{{Path: "a.go"}, {Path: "z.dat", Excluded: "oversize"}}}
	for _, tc := range []struct {
		raw  string
		want bool
	}{
		{`[{"path":"z.dat","reason":"oversize"}]`, true},
		{`[{"path":"z.dat","reason":"unreadable"}]`, false},
		{`[{"path":"other.dat","reason":"oversize"}]`, false},
		{`[]`, false},
	} {
		reviewed, err := reviewedExclusions(p, json.RawMessage(tc.raw))
		if tc.want != (err == nil && reviewed) {
			t.Fatalf("raw=%s reviewed=%v err=%v", tc.raw, reviewed, err)
		}
	}
}
func (windowBackendFixture) AdvanceSourceWindow(_ context.Context, w *client.SourceWindow, _ int64, _ time.Duration) error {
	if w.Pass == 0 {
		w.Pass = 1
	} else {
		w.Ready = true
	}
	return nil
}
func windowOp(t *testing.T, s *Server, backend QueryInterface, service, op, token string) (map[string]any, *rpcError) {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"service": service, "source_window": map[string]string{"op": op, "token": token}})
	v, e := s.sourceWindowOperation(context.Background(), backend, raw)
	if e != nil {
		return nil, e
	}
	return v.(map[string]any), nil
}
func TestSourceWindowLifecycleTokenExpiryAndReplay(t *testing.T) {
	b := windowBackendFixture{}
	s := NewServer(b)
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	s.windowClock = func() time.Time { return now }
	begin, e := windowOp(t, s, b, "svc", "begin", "")
	if e != nil {
		t.Fatal(e)
	}
	token := begin["source_window_token"].(string)
	if _, _, e := s.sourceWindowProjection("svc", token); e == nil {
		t.Fatal("before-hash projection exposed")
	}
	if _, e := windowOp(t, s, b, "svc", "begin", ""); e == nil {
		t.Fatal("concurrent window admitted")
	}
	if _, e := windowOp(t, s, b, "other", "advance", token); e == nil {
		t.Fatal("cross-service token admitted")
	}
	advance, e := windowOp(t, s, b, "svc", "advance", token)
	if e != nil || advance["source_window_state"] != "provisional" {
		t.Fatalf("advance=%v err=%v", advance, e)
	}
	if _, release, e := s.sourceWindowProjection("svc", token); e != nil {
		t.Fatal(e)
	} else {
		release()
	}
	close, e := windowOp(t, s, b, "svc", "close", token)
	if e != nil || close["source_window_state"] != "closed" {
		t.Fatalf("close=%v err=%v", close, e)
	}
	if _, _, e := s.sourceWindowProjection("svc", token); e == nil {
		t.Fatal("closed token replayed")
	}
	begin, e = windowOp(t, s, b, "svc", "begin", "")
	if e != nil {
		t.Fatal(e)
	}
	now = now.Add(sourceWindowTTL + time.Second)
	if _, e := windowOp(t, s, b, "svc", "advance", begin["source_window_token"].(string)); e == nil {
		t.Fatal("expired window accepted")
	}
}
