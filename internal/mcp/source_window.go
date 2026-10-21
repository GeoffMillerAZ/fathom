package mcp

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

type sourceWindowInput struct {
	Op       string          `json:"op"`
	Token    string          `json:"token"`
	ReportID string          `json:"report_id"`
	Expected json.RawMessage `json:"expected_exclusions"`
}

func parseSourceWindowInput(raw json.RawMessage) (sourceWindowInput, error) {
	if len(raw) == 0 {
		return sourceWindowInput{}, nil
	}
	var p sourceWindowInput
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return p, fmt.Errorf("current source window: invalid input: %w", err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return p, fmt.Errorf("current source window: trailing input")
	}
	return p, nil
}

// The optional source-window capability is carried by verify_plan, so the
// existing tree-class admission, tool surface and response budget apply.
type sourceWindowBackend interface {
	BeginSourceWindow(context.Context, string) (*client.SourceWindow, error)
	AdvanceSourceWindow(context.Context, *client.SourceWindow, int64, time.Duration) error
}

type serverSourceWindow struct {
	mu              sync.Mutex
	service         string
	token           string
	opened          time.Time
	closedAt        time.Time
	state           *client.SourceWindow
	failed          bool
	closing         bool
	active          int
	reviewed        bool
	projectionBytes int
	reportBytes     int
	reports         map[string]json.RawMessage
}

const sourceWindowTTL = 30 * time.Minute
const sourceWindowMax = 4
const sourceWindowCacheBytes = 32 << 20

func (s *Server) sourceWindowNow() time.Time {
	if s.windowClock != nil {
		return s.windowClock()
	}
	return time.Now()
}

func (s *Server) sourceWindowGet(service, token string) (*serverSourceWindow, error) {
	s.windowMu.Lock()
	defer s.windowMu.Unlock()
	s.sourceWindowPruneLocked()
	w := s.windows[service]
	if w == nil || w.token != token {
		return nil, fmt.Errorf("current source window: missing, expired or wrong service")
	}
	return w, nil
}

func (s *Server) sourceWindowPruneLocked() {
	for key, w := range s.windows {
		if s.sourceWindowNow().Sub(w.opened) > sourceWindowTTL {
			delete(s.windows, key)
		}
	}
	for token, w := range s.closedWindows {
		if s.sourceWindowNow().Sub(w.closedAt) > 5*time.Minute {
			delete(s.closedWindows, token)
		}
	}
}

func (s *Server) sourceWindowCurrent(w *serverSourceWindow) bool {
	s.windowMu.Lock()
	defer s.windowMu.Unlock()
	s.sourceWindowPruneLocked()
	return s.windows[w.service] == w && s.sourceWindowNow().Sub(w.opened) <= sourceWindowTTL
}

func (s *Server) sourceWindowDelete(w *serverSourceWindow) {
	s.windowMu.Lock()
	defer s.windowMu.Unlock()
	if s.windows[w.service] == w {
		delete(s.windows, w.service)
	}
}

func (s *Server) sourceWindowUsedLocked() int {
	n := 0
	for _, w := range s.windows {
		n += w.projectionBytes + w.reportBytes
	}
	for _, w := range s.closedWindows {
		n += w.projectionBytes + w.reportBytes
	}
	return n
}

func reviewedExclusions(p store.SourceRefProjection, raw json.RawMessage) (bool, error) {
	if len(raw) == 0 {
		return p.Complete, nil
	}
	if len(raw) > 1<<20 {
		return false, fmt.Errorf("current source window: expected exclusion manifest exceeds 1 MiB")
	}
	var expected []struct {
		Path   string `json:"path"`
		Reason string `json:"reason"`
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&expected); err != nil || expected == nil {
		return false, fmt.Errorf("current source window: expected exclusion manifest invalid")
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return false, fmt.Errorf("current source window: expected exclusion manifest invalid")
	}
	i := 0
	for _, row := range p.Rows {
		if row.Excluded != "" {
			if i >= len(expected) || expected[i].Path != row.Path || expected[i].Reason != row.Excluded {
				return false, fmt.Errorf("current source window: reviewed exclusions differ")
			}
			i++
		}
	}
	if i != len(expected) {
		return false, fmt.Errorf("current source window: reviewed exclusions differ")
	}
	return true, nil
}

func (s *Server) sourceWindowOperation(ctx context.Context, backend QueryInterface, raw json.RawMessage) (any, *rpcError) {
	capable, ok := backend.(sourceWindowBackend)
	if !ok {
		return nil, &rpcError{Code: ErrCodeNotFound, Message: "current source window: unsupported backend"}
	}
	var outer struct {
		Service string          `json:"service"`
		Window  json.RawMessage `json:"source_window"`
	}
	if json.Unmarshal(raw, &outer) != nil || outer.Service == "" {
		return nil, &rpcError{Code: ErrCodeParams, Message: "current source window: service required"}
	}
	p, err := parseSourceWindowInput(outer.Window)
	if err != nil || p.Op == "" {
		return nil, &rpcError{Code: ErrCodeParams, Message: "current source window: operation required or invalid"}
	}
	service := outer.Service
	if p.Op == "result" {
		s.windowMu.Lock()
		s.sourceWindowPruneLocked()
		w := s.closedWindows[p.Token]
		s.windowMu.Unlock()
		if w == nil || w.service != service {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "current source window: closed token missing or expired"}
		}
		w.mu.Lock()
		defer w.mu.Unlock()
		s.windowMu.Lock()
		s.sourceWindowPruneLocked()
		valid := s.closedWindows[p.Token] == w
		s.windowMu.Unlock()
		if !valid {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "current source window: closed token expired"}
		}
		result := w.reports[p.ReportID]
		if len(result) == 0 {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "current source window: report missing"}
		}
		return result, nil
	}
	if p.Op == "begin" {
		state, err := capable.BeginSourceWindow(ctx, service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		reviewed, err := reviewedExclusions(state.Projection, p.Expected)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: err.Error()}
		}
		// The sealed projection is compact, but bound the sum of all live
		// session metadata before accepting another service.
		b, _ := json.Marshal(state.Projection)
		s.windowMu.Lock()
		defer s.windowMu.Unlock()
		s.sourceWindowPruneLocked()
		used := len(b) + s.sourceWindowUsedLocked()
		if s.windows[service] != nil || len(s.windows) >= sourceWindowMax || used > sourceWindowCacheBytes {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "current source window: service busy or cache cap"}
		}
		var secret [24]byte
		if _, err = rand.Read(secret[:]); err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "current source window: token unavailable"}
		}
		token := hex.EncodeToString(secret[:])
		if s.windows == nil {
			s.windows = make(map[string]*serverSourceWindow)
		}
		s.windows[service] = &serverSourceWindow{service: service, token: token, opened: s.sourceWindowNow(), state: state, reviewed: reviewed, projectionBytes: len(b), reports: make(map[string]json.RawMessage)}
		return map[string]any{"source_window_token": token, "source_window_state": "hashing_before", "watermark": state.Projection.Watermark, "total": len(state.Projection.Rows), "exclusion_review": map[bool]string{true: "matched", false: "unsupplied"}[reviewed]}, nil
	}
	w, err := s.sourceWindowGet(service, p.Token)
	if err != nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
	}
	if !w.mu.TryLock() {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "current source window: concurrent continuation"}
	}
	defer w.mu.Unlock()
	if !s.sourceWindowCurrent(w) {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "current source window: expired or replaced"}
	}
	if p.Op == "abort" {
		w.failed = true
		s.sourceWindowDelete(w)
		return map[string]any{"source_window_state": "aborted"}, nil
	}
	if p.Op != "advance" && p.Op != "close" {
		return nil, &rpcError{Code: ErrCodeParams, Message: "current source window: invalid operation"}
	}
	if p.Op == "advance" && w.state.Pass != 0 || p.Op == "close" && w.state.Pass != 1 {
		return nil, &rpcError{Code: ErrCodeParams, Message: "current source window: wrong phase"}
	}
	if p.Op == "close" && w.active != 0 {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "current source window: verification calls still active"}
	}
	if p.Op == "close" {
		w.closing = true
	}
	if err := capable.AdvanceSourceWindow(ctx, w.state, 0, 0); err != nil {
		w.failed = true
		s.sourceWindowDelete(w)
		return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error(), Data: map[string]any{"bytes": w.state.Bytes, "files": w.state.Files, "pass": w.state.Pass, "index": w.state.Index, "offset": w.state.Offset}}
	}
	status := "hashing_before"
	if w.state.Pass == 1 {
		status = "provisional"
	}
	if w.state.Ready {
		status = "closed"
		s.windowMu.Lock()
		if s.windows[service] != w || s.sourceWindowNow().Sub(w.opened) > sourceWindowTTL {
			s.windowMu.Unlock()
			w.failed = true
			return nil, &rpcError{Code: ErrCodeInternal, Message: "current source window: expired or replaced at close"}
		}
		delete(s.windows, service)
		w.closedAt = s.sourceWindowNow()
		if s.closedWindows == nil {
			s.closedWindows = make(map[string]*serverSourceWindow)
		}
		s.closedWindows[w.token] = w
		s.windowMu.Unlock()
	}
	answer := map[string]any{"source_window_state": status, "pass": w.state.Pass, "index": w.state.Index, "offset": w.state.Offset, "bytes": w.state.Bytes, "files": w.state.Files, "watermark": w.state.Projection.Watermark}
	if w.state.Ready {
		answer["excluded_hashes"] = w.state.ExcludedHashes()
		answer["observation_scope"] = "bounded current-source window"
		answer["exclusion_review"] = map[bool]string{true: "matched", false: "unsupplied"}[w.reviewed]
	}
	return answer, nil
}

func (s *Server) sourceWindowPending(ctx context.Context, w *serverSourceWindow, out map[string]any) (map[string]any, error) {
	var idBytes [18]byte
	if _, err := rand.Read(idBytes[:]); err != nil {
		return nil, fmt.Errorf("current source window: report ID unavailable")
	}
	id := hex.EncodeToString(idBytes[:])
	out["source_window_state"] = "closed"
	out["current_worktree_freshness"] = "observed_window"
	out["source_window_report_id"] = id
	if w.reviewed {
		out["exclusion_review"] = "matched"
	} else {
		out["exclusion_review"] = "unsupplied"
	}
	budget := defaultCharBudget
	if n := declaredBudgetFrom(ctx); n > 0 {
		budget = n
	}
	if budget > 256 {
		budget -= 256
	}
	_, raw, err := fitSourceWindowReport(out, budget)
	if err != nil {
		return nil, err
	}
	var finalMap map[string]any
	if err = json.Unmarshal(raw, &finalMap); err != nil {
		return nil, err
	}
	items, ok := finalMap["tasks"].([]any)
	if !ok {
		return nil, fmt.Errorf("current source window: final report could not fit task envelope")
	}
	pendingTasks := make([]map[string]any, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("current source window: invalid task envelope")
		}
		pendingTasks = append(pendingTasks, map[string]any{"id": row["id"], "title": row["title"], "status": "unknown", "label": "source-window-pending", "criteria": []any{}})
	}
	pending := map[string]any{"service": out["service"], "source_scope": "current_worktree", "source_window_state": "pending", "source_window_report_id": id, "current_worktree_freshness": "unknown_until_close", "summary": map[string]any{"total": len(pendingTasks), "pass": 0, "fail": 0, "unknown": len(pendingTasks)}, "tasks": pendingTasks}
	if v, ok := finalMap["elided"]; ok {
		pending["elided"] = v
	}
	if v, ok := finalMap["partial"]; ok {
		pending["partial"] = v
	}
	if n, e := jsonSize(pending, budget); e != nil || n > budget {
		return nil, fmt.Errorf("current source window: pending receipt exceeds response budget")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.failed || w.closing || !s.sourceWindowCurrent(w) {
		return nil, fmt.Errorf("current source window: no longer active")
	}
	s.windowMu.Lock()
	if s.sourceWindowUsedLocked()+len(raw) > sourceWindowCacheBytes {
		s.windowMu.Unlock()
		return nil, fmt.Errorf("current source window: report cache cap")
	}
	w.reports[id] = raw
	w.reportBytes += len(raw)
	s.windowMu.Unlock()
	return pending, nil
}

func (s *Server) sourceWindowProjection(service, token string) (store.SourceRefProjection, func(), error) {
	w, err := s.sourceWindowGet(service, token)
	if err != nil {
		return store.SourceRefProjection{}, nil, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if !s.sourceWindowCurrent(w) || w.failed || w.closing || w.state.Pass != 1 || w.state.Ready {
		return store.SourceRefProjection{}, nil, fmt.Errorf("current source window: before-hash incomplete")
	}
	w.active++
	release := func() { w.mu.Lock(); w.active--; w.mu.Unlock() }
	return w.state.Projection, release, nil
}
