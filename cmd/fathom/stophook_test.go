package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNewlyClaimedTasks(t *testing.T) {
	oldMD := []byte(`# P
- [ ] One {id: T-1, symbol: A}
- [x] Two {id: T-2, symbol: B}
`)
	newMD := []byte(`# P
- [x] One {id: T-1, symbol: A}
- [x] Two {id: T-2, symbol: B}
- [x] Three {id: T-3, symbol: C}
- [~] Four {id: T-4}
`)
	got := newlyClaimedTasks(oldMD, newMD, "svc")
	want := []string{"T-1", "T-3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("newly claimed = %v, want %v", got, want)
	}
	// No old version → everything claimed is new.
	if got := newlyClaimedTasks(nil, newMD, "svc"); !reflect.DeepEqual(got, []string{"T-1", "T-2", "T-3"}) {
		t.Errorf("nil old: %v", got)
	}
	// Determinism.
	if again := newlyClaimedTasks(oldMD, newMD, "svc"); !reflect.DeepEqual(got, again) {
		t.Error("newlyClaimedTasks is not deterministic")
	}
}

// fakeStopDaemon serves list_services + verify_plan shaped like the real MCP
// endpoint: the claimed task T-1 fails symbol_present.
func fakeStopDaemon(t *testing.T, service string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var inner string
		switch req.Params.Name {
		case "list_services":
			inner = fmt.Sprintf(`{"services":[{"service":%q,"symbol_count":10}]}`, service)
		case "verify_plan":
			inner = `{"service":"` + service + `","summary":{"total":2,"pass":0,"fail":1,"unknown":1},
			  "tasks":[
			    {"id":"T-1","title":"One","status":"fail","label":"claimed-but-absent",
			     "criteria":[{"name":"symbol_present","status":"fail","detail":"\"Ghost\" not found in the index"}]},
			    {"id":"T-2","title":"Two","status":"unknown","label":"no-contract","criteria":[]}
			  ]}`
		default:
			http.Error(w, "unknown tool", 400)
			return
		}
		env := map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": inner}}}}
		_ = json.NewEncoder(w).Encode(env)
	}))
}

func TestRunStopHook_AdvisoryAndBlock(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	plans := filepath.Join(dir, "docs", "plans")
	if err := os.MkdirAll(plans, 0o755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(plans, "p.md")

	// HEAD version: T-1 open. Working tree: T-1 claimed (the new over-claim).
	oldMD := "- [ ] One {id: T-1, symbol: Ghost}\n- [x] Two {id: T-2}\n"
	newMD := "- [x] One {id: T-1, symbol: Ghost}\n- [x] Two {id: T-2}\n"
	if err := os.WriteFile(planPath, []byte(oldMD), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "."},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "init"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(planPath, []byte(newMD), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := fakeStopDaemon(t, svc)
	defer srv.Close()
	hc := &http.Client{Timeout: 2 * time.Second}

	// Advisory mode: additionalContext, exit 0.
	var out bytes.Buffer
	if code := runStopHook(dir, hc, srv.URL, false, &out); code != 0 {
		t.Fatalf("advisory exit = %d, want 0", code)
	}
	var adv struct {
		HookSpecificOutput struct {
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out.Bytes(), &adv); err != nil {
		t.Fatalf("advisory output not JSON: %v (%s)", err, out.String())
	}
	if !strings.Contains(adv.HookSpecificOutput.AdditionalContext, "T-1") ||
		!strings.Contains(adv.HookSpecificOutput.AdditionalContext, "not found in the index") {
		t.Errorf("advisory message missing the failing claim: %s", adv.HookSpecificOutput.AdditionalContext)
	}
	// T-2 was already claimed at HEAD and must NOT be flagged even though it
	// verifies Unknown.
	if strings.Contains(adv.HookSpecificOutput.AdditionalContext, "T-2") {
		t.Error("previously-claimed / Unknown task must not fire the hook")
	}

	// Block mode: decision:block.
	out.Reset()
	if code := runStopHook(dir, hc, srv.URL, true, &out); code != 0 {
		t.Fatalf("block exit = %d, want 0", code)
	}
	var blk struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal(out.Bytes(), &blk); err != nil || blk.Decision != "block" {
		t.Fatalf("block output = %s (err %v)", out.String(), err)
	}

	// Silent paths: daemon down → exit 0, no output.
	out.Reset()
	if code := runStopHook(dir, hc, "http://127.0.0.1:1", false, &out); code != 0 || out.Len() != 0 {
		t.Errorf("daemon-down must be silent exit 0; got %d %q", code, out.String())
	}
	// No plans dir → silent.
	out.Reset()
	if code := runStopHook(t.TempDir(), hc, srv.URL, false, &out); code != 0 || out.Len() != 0 {
		t.Errorf("no-plans must be silent exit 0; got %d %q", code, out.String())
	}
}
