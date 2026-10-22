package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func stopDeliveryRepo(t *testing.T, contract string) (string, string) {
	t.Helper()
	root := cbHookEnv(t)
	t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(t.TempDir(), "verify.jsonl"))
	writeEditedFile(t, root, "docs/plans/p.md", "- [ ] One {id: T-1, "+contract+"}\n")
	writeEditedFile(t, root, "pkg/tracked/code.go", "package tracked\n")
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.name=test", "-c", "user.email=test@example.invalid", "commit", "-qm", "init"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	writeEditedFile(t, root, "docs/plans/p.md", "- [x] One {id: T-1, "+contract+"}\n")
	return root, strings.ToLower(filepath.Base(root))
}

type stopDeliveryTransport struct{ service string }

func (s stopDeliveryTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Body == nil {
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("unknown route")), Header: make(http.Header)}, nil
	}
	var req struct {
		Params struct {
			Name string `json:"name"`
		} `json:"params"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	var result any
	switch req.Params.Name {
	case "list_services":
		result = map[string]any{"services": []any{map[string]any{"service": s.service, "symbol_count": 10}}}
	case "verify_plan":
		result = map[string]any{"service": s.service, "summary": map[string]int{"total": 1, "fail": 1}, "tasks": []any{map[string]any{"id": "T-1", "title": "One", "status": "fail", "label": "claimed-but-absent", "criteria": []any{map[string]string{"name": "symbol_present", "status": "fail", "detail": "Ghost not found in the index"}}}}}
	default:
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("unknown tool")), Header: make(http.Header)}, nil
	}
	inner, _ := json.Marshal(result)
	body, _ := json.Marshal(map[string]any{"result": map[string]any{"content": []any{map[string]string{"type": "text", "text": string(inner)}}}})
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
}

func stopDelivery(t *testing.T, root, service string) string {
	t.Helper()
	var out bytes.Buffer
	hc := newStopHookClient()
	hc.Transport = stopDeliveryTransport{service}
	if code, _ := runStopHook(root, hc, "http://127.0.0.1:1", true, &out, "session-a"); code != 0 {
		t.Fatalf("stop exit %d", code)
	}
	return out.String()
}

func TestStopDeliveryUncommittedIsUnknown(t *testing.T) {
	for _, kind := range []string{"untracked", "modified", "staged", "oracle-file"} {
		t.Run(kind, func(t *testing.T) {
			contract, path, file := "symbol: Ghost, package: pkg/new", "pkg/new", "pkg/new/code.go"
			if kind == "modified" || kind == "staged" {
				contract, path, file = "symbol: Ghost, package: pkg/tracked", "pkg/tracked", "pkg/tracked/code.go"
			}
			if kind == "oracle-file" {
				contract, path, file = "oracle: lexical, path: evidence.txt, pattern: PASS", "evidence.txt", "evidence.txt"
			}
			root, svc := stopDeliveryRepo(t, contract)
			writeEditedFile(t, root, file, "package changed\n")
			if kind == "staged" {
				if out, err := exec.Command("git", "-C", root, "add", file).CombinedOutput(); err != nil {
					t.Fatalf("git add: %v: %s", err, out)
				}
			}
			got := stopDelivery(t, root, svc)
			want := "unknown (not indexed yet: 1 uncommitted files under " + path + ")"
			if !strings.Contains(got, want) || strings.Contains(got, `"decision":"block"`) || strings.Contains(got, "not found in the index") {
				t.Fatalf("unseen code must be unknown and advisory, want %q: %s", want, got)
			}
			rows := readVerifyLedger()
			if len(rows) != 1 || len(rows[0].Rows) != 1 || rows[0].Rows[0].State != "unknown" {
				t.Fatalf("unknown missing from verify ledger: %+v", rows)
			}
			for _, row := range rows {
				if row.Failing != 0 {
					t.Fatalf("unknown recorded as refusal: %+v", row)
				}
			}
			if got := stopDelivery(t, root, svc); got != "" {
				t.Fatalf("unchanged unknown set repeated: %s", got)
			}
		})
	}
}

func TestStopDeliverySetOncePerChange(t *testing.T) {
	root, svc := stopDeliveryRepo(t, "symbol: Ghost, package: pkg/tracked")
	first := stopDelivery(t, root, svc)
	if !strings.Contains(first, "not found in the index") {
		t.Fatalf("committed code failure lost: %s", first)
	}
	if got := stopDelivery(t, root, svc); got != "" {
		t.Fatalf("unchanged failure set repeated: %s", got)
	}
	writeEditedFile(t, root, "pkg/tracked/code.go", "package changed\n")
	if got := stopDelivery(t, root, svc); !strings.Contains(got, "unknown (not indexed yet:") {
		t.Fatalf("changed set not reported: %s", got)
	}
	if got := stopDelivery(t, root, svc); got != "" {
		t.Fatalf("changed set repeated: %s", got)
	}
	// A resolved set clears the previous digest, so a later recurrence reports.
	writeEditedFile(t, root, "docs/plans/p.md", "- [ ] One {id: T-1, symbol: Ghost, package: pkg/tracked}\n")
	if got := stopDelivery(t, root, svc); got != "" {
		t.Fatalf("resolved set emitted: %s", got)
	}
	writeEditedFile(t, root, "docs/plans/p.md", "- [x] One {id: T-1, symbol: Ghost, package: pkg/tracked}\n")
	if got := stopDelivery(t, root, svc); !strings.Contains(got, "unknown (not indexed yet:") {
		t.Fatalf("recurring set not reported: %s", got)
	}
}

func TestStopDeliveryStateErrorReportsInFull(t *testing.T) {
	root, svc := stopDeliveryRepo(t, "symbol: Ghost")
	first := stopDelivery(t, root, svc)
	path := cbDedupePath("session-a")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("invalid state"), 0o600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if got := stopDelivery(t, root, svc); got == "" || got != first {
			t.Fatalf("state error must report in full: %s, want %s", got, first)
		}
	}
}

func TestStopDeliveryNoRefreshAndFailedWrite(t *testing.T) {
	root, svc := stopDeliveryRepo(t, "symbol: Ghost")
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	previous := nowFunc
	nowFunc = func() time.Time { return now }
	t.Cleanup(func() { nowFunc = previous })
	hc := newStopHookClient()
	hc.Transport = stopDeliveryTransport{svc}
	runStopHook(root, hc, "http://127.0.0.1:1", true, cbFailedWriter{}, "session-a")
	if got := stopDelivery(t, root, svc); !strings.Contains(got, "not found in the index") {
		t.Fatalf("failed delivery suppressed report: %s", got)
	}
	now = now.Add(time.Hour)
	for i := 0; i < 12; i++ {
		if got := stopDelivery(t, root, svc); got != "" {
			t.Fatalf("unchanged claim set refreshed: %s", got)
		}
	}
}
