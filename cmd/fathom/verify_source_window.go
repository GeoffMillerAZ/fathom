package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type sourceWindowReply struct {
	Token           string `json:"source_window_token"`
	State           string `json:"source_window_state"`
	Pass            int    `json:"pass"`
	Index           int    `json:"index"`
	Offset          int64  `json:"offset"`
	Bytes           int64  `json:"bytes"`
	Files           int    `json:"files"`
	Watermark       string `json:"watermark"`
	ExclusionReview string `json:"exclusion_review"`
}

type sourceWindowExclusion struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

func readSourceWindowExclusions(path string) ([]sourceWindowExclusion, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 1<<20+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 1<<20 {
		return nil, fmt.Errorf("current source window: expected exclusions exceed 1 MiB")
	}
	var rows []sourceWindowExclusion
	if err = json.Unmarshal(raw, &rows); err != nil || rows == nil {
		return nil, fmt.Errorf("current source window: expected exclusions must be a JSON array")
	}
	for i, row := range rows {
		if row.Path == "" || row.Reason == "" || i > 0 && rows[i-1].Path >= row.Path {
			return nil, fmt.Errorf("current source window: expected exclusions must have sorted unique paths and reasons")
		}
	}
	return rows, nil
}

// emitCurrentWindowRefusal is an operational result, never a plan verdict.
// In JSON mode it preserves the reason without exposing any provisional row.
func emitCurrentWindowRefusal(stdout, stderr io.Writer, service string, jsonOut bool, err error) int {
	fmt.Fprintf(stderr, "error: %v\n", err)
	if jsonOut {
		_ = json.NewEncoder(stdout).Encode(map[string]any{
			"service":                    service,
			"source_scope":               "current_worktree",
			"current_worktree_freshness": "unknown",
			"source_window_state":        "refused",
			"error":                      map[string]string{"code": "current_source_window_refused", "reason": err.Error()},
		})
	}
	return 2
}

// Source-window lifecycle travels through verify_plan's existing tree-class
// admission. An old daemon is identified by its published input schema.
func callSourceWindow(hc *http.Client, baseURL, service, op, token string, expected ...[]sourceWindowExclusion) (sourceWindowReply, bool, error) {
	window := map[string]any{"op": op}
	args := map[string]any{"service": service, "source_window": window}
	if len(expected) > 0 && expected[0] != nil {
		window["expected_exclusions"] = expected[0]
	}
	if token != "" {
		window["token"] = token
	}
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "verify_plan", "arguments": args}})
	req, err := originHeader(baseURL, bytes.NewReader(body))
	if err != nil {
		return sourceWindowReply{}, false, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return sourceWindowReply{}, false, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, verifyPlanReadLimit+1))
	if err != nil {
		return sourceWindowReply{}, false, err
	}
	inner, err := unwrapRPCResult(raw)
	if err != nil {
		return sourceWindowReply{}, false, fmt.Errorf("current source window: %w", err)
	}
	var answer sourceWindowReply
	if err = json.Unmarshal(inner, &answer); err != nil {
		return answer, false, err
	}
	if answer.State == "" {
		return answer, false, fmt.Errorf("current source window: daemon omitted lifecycle state")
	}
	return answer, true, nil
}

func sourceWindowSupported(hc *http.Client, baseURL string) (bool, error) {
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	req, err := originHeader(baseURL, bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, verifyPlanReadLimit+1))
	if err != nil {
		return false, err
	}
	var msg struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				InputSchema struct {
					Properties map[string]json.RawMessage `json:"properties"`
				} `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err = json.Unmarshal(raw, &msg); err != nil {
		return false, err
	}
	if msg.Error != nil {
		return false, fmt.Errorf("current source window: tools/list: %s", msg.Error.Message)
	}
	for _, tool := range msg.Result.Tools {
		if tool.Name == "verify_plan" {
			_, ok := tool.InputSchema.Properties["source_window"]
			return ok, nil
		}
	}
	return false, fmt.Errorf("current source window: verify_plan missing from tools/list")
}

func beginCurrentWindow(hc *http.Client, baseURL, service string, expected ...[]sourceWindowExclusion) (string, error) {
	supported, err := sourceWindowSupported(hc, baseURL)
	if err != nil {
		return "", fmt.Errorf("current source window: capability check: %w", err)
	}
	if !supported {
		if len(expected) > 0 && expected[0] != nil {
			return "", fmt.Errorf("current source window: daemon lacks reviewed-exclusion protocol")
		}
		return "", nil
	}
	begin, supported, err := callSourceWindow(hc, baseURL, service, "begin", "", expected...)
	if err != nil {
		return "", err
	}
	if !supported {
		return "", fmt.Errorf("current source window: advertised lifecycle omitted state")
	}
	if begin.Token == "" || begin.State != "hashing_before" {
		return "", fmt.Errorf("current source window: malformed begin")
	}
	completed := false
	defer func() {
		if !completed {
			abortCurrentWindow(hc, baseURL, service, begin.Token)
		}
	}()
	for i := 0; i < 4096; i++ {
		next, _, err := callSourceWindow(hc, baseURL, service, "advance", begin.Token)
		if err != nil {
			return "", err
		}
		if next.State == "provisional" {
			completed = true
			return begin.Token, nil
		}
		if next.State != "hashing_before" {
			return "", fmt.Errorf("current source window: invalid before-hash state %q", next.State)
		}
	}
	return "", fmt.Errorf("current source window: before-hash continuation limit")
}

func abortCurrentWindow(hc *http.Client, baseURL, service, token string) {
	if token != "" {
		_, _, _ = callSourceWindow(hc, baseURL, service, "abort", token)
	}
}

func closeCurrentWindow(hc *http.Client, baseURL, service, token string) error {
	if token == "" {
		return nil
	}
	for i := 0; i < 4096; i++ {
		next, _, err := callSourceWindow(hc, baseURL, service, "close", token)
		if err != nil {
			return err
		}
		if next.State == "closed" {
			return nil
		}
		if next.State != "provisional" {
			return fmt.Errorf("current source window: invalid close state %q", next.State)
		}
	}
	return fmt.Errorf("current source window: close continuation limit")
}

func fetchCurrentWindowResult(hc *http.Client, baseURL, service, token, id string) (httpVerifyResult, error) {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "verify_plan", "arguments": map[string]any{"service": service, "source_window": map[string]any{"op": "result", "token": token, "report_id": id}}}})
	req, err := originHeader(baseURL, bytes.NewReader(body))
	if err != nil {
		return httpVerifyResult{}, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return httpVerifyResult{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, verifyPlanReadLimit+1))
	if err != nil {
		return httpVerifyResult{}, err
	}
	result, err := parseVerifyPlanResponse(raw)
	if err != nil {
		return result, err
	}
	if result.SourceWindowState != "closed" || result.SourceWindowReportID != id {
		return httpVerifyResult{}, fmt.Errorf("current source window: final report identity differs")
	}
	return result, nil
}
