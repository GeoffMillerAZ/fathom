package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// claimRepo builds a git repo whose HEAD holds oldMD at docs/plans/p.md
// and whose working tree holds newMD — the claim moment, frozen.
func claimRepo(t *testing.T, oldMD, newMD string) (dir, planPath string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir = t.TempDir()
	plans := filepath.Join(dir, "docs", "plans")
	if err := os.MkdirAll(plans, 0o755); err != nil {
		t.Fatal(err)
	}
	planPath = filepath.Join(plans, "p.md")
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
	return dir, planPath
}

// fakeClaimDaemon serves list_services + verify_plan with T-1 at the given
// status — "fail" (symbol absent) or "pass" — and counts verify calls.
func fakeClaimDaemon(t *testing.T, service, t1Status string, verifyCalls *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var inner string
		switch req.Params.Name {
		case "list_services":
			inner = fmt.Sprintf(`{"services":[{"service":%q,"symbol_count":10}]}`, service)
		case "verify_plan":
			*verifyCalls++
			criteria := `[{"name":"symbol_present","status":"pass","detail":"found"}]`
			label := "verified"
			if t1Status == "fail" {
				criteria = `[{"name":"symbol_present","status":"fail","detail":"\"Ghost\" not found in the index"}]`
				label = "claimed-but-absent"
			}
			inner = fmt.Sprintf(`{"service":%q,"summary":{"total":2},"tasks":[
			  {"id":"T-1","title":"One","status":%q,"label":%q,"criteria":%s},
			  {"id":"T-2","title":"Two","status":"unknown","label":"no-contract","criteria":[]}]}`,
				service, t1Status, label, criteria)
		default:
			http.Error(w, "unknown tool", 400)
			return
		}
		env := map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": inner}}}}
		_ = json.NewEncoder(w).Encode(env)
	}))
}

func isolateClaimTelemetry(t *testing.T) (ledgerPath, beatsPath string) {
	t.Helper()
	ledgerPath = filepath.Join(t.TempDir(), "verify-ledger.jsonl")
	beatsPath = filepath.Join(t.TempDir(), "hooks.jsonl")
	t.Setenv("FATHOM_VERIFY_LEDGER", ledgerPath)
	t.Setenv("FATHOM_HOOKS_LOG", beatsPath)
	t.Setenv("FATHOM_NARRATE_STATE", filepath.Join(t.TempDir(), "narrate.json"))
	return ledgerPath, beatsPath
}

func claimBeats(t *testing.T, path string) []heartbeat.Beat {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	beats, err := heartbeat.Load(f, true)
	if err != nil {
		t.Fatalf("beat log: %v", err)
	}
	return beats
}

// TestClaimMomentVerify_RefusesAbsentAnchor pins PC-22's product: a newly
// checked row whose anchor the index cannot find is refused at the edit,
// the refusal names the row and the criterion, the ledger row carries
// origin: claim-moment, and the refused beat is written.
func TestClaimMomentVerify_RefusesAbsentAnchor(t *testing.T) {
	ledgerPath, beatsPath := isolateClaimTelemetry(t)
	oldMD := "- [ ] One {id: T-1, symbol: Ghost}\n- [x] Two {id: T-2}\n"
	newMD := "- [x] One {id: T-1, symbol: Ghost}\n- [x] Two {id: T-2}\n"
	dir, planPath := claimRepo(t, oldMD, newMD)
	svc := strings.ToLower(filepath.Base(dir))
	calls := 0
	srv := fakeClaimDaemon(t, svc, "fail", &calls)
	defer srv.Close()
	hc := &http.Client{Timeout: 2 * time.Second}

	got := claimMomentVerifyWith(hc, srv.URL, dir, planPath, "sess-1")
	for _, want := range []string{"done-means-proven (claim moment)", "docs/plans/p.md [T-1] One", "not found in the index", "fathom verify --service " + svc} {
		if !strings.Contains(got, want) {
			t.Errorf("refusal missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "T-2") {
		t.Errorf("T-2 was claimed at HEAD (and verifies Unknown) — it must not be refused:\n%s", got)
	}
	if calls != 1 {
		t.Errorf("verify_plan calls = %d, want exactly 1 (one plan, one verify)", calls)
	}
	_ = ledgerPath
	rows := readVerifyLedger()
	if len(rows) != 1 || rows[0].Origin != verifyOriginClaimMoment || rows[0].Plan != "docs/plans/p.md" || rows[0].Failing != 1 || rows[0].Service != svc {
		t.Errorf("ledger rows = %+v, want one origin=claim-moment row for docs/plans/p.md with failing=1", rows)
	}
	kinds := map[string]heartbeat.Beat{}
	for _, b := range claimBeats(t, beatsPath) {
		if b.Channel == claimMomentChannel {
			kinds[b.Kind] = b
		}
	}
	if _, ok := kinds["seen"]; !ok {
		t.Errorf("no seen beat: %v", kinds)
	}
	if b, ok := kinds["refused"]; !ok || b.Meta["refused"] != "1" || b.Meta["plan"] != "docs/plans/p.md" || b.Meta["session"] != "sess-1" {
		t.Errorf("refused beat = %+v", b)
	}
	// Deterministic: the same edit yields the same refusal.
	if again := claimMomentVerifyWith(hc, srv.URL, dir, planPath, "sess-1"); again != got {
		t.Errorf("refusal is not deterministic:\n%s\n---\n%s", got, again)
	}
}

// TestClaimMomentVerify_CleanClaimIsSilent: a newly checked row the index
// proves is silence — nothing injected — but the verify still runs and
// the ledger still records it (origin: claim-moment, failing 0).
func TestClaimMomentVerify_CleanClaimIsSilent(t *testing.T) {
	_, beatsPath := isolateClaimTelemetry(t)
	oldMD := "- [ ] One {id: T-1, symbol: Real}\n"
	newMD := "- [x] One {id: T-1, symbol: Real}\n"
	dir, planPath := claimRepo(t, oldMD, newMD)
	svc := strings.ToLower(filepath.Base(dir))
	calls := 0
	srv := fakeClaimDaemon(t, svc, "pass", &calls)
	defer srv.Close()
	hc := &http.Client{Timeout: 2 * time.Second}

	if got := claimMomentVerifyWith(hc, srv.URL, dir, planPath, "sess-2"); got != "" {
		t.Errorf("clean claim must be silent, got:\n%s", got)
	}
	if calls != 1 {
		t.Errorf("verify_plan calls = %d, want 1", calls)
	}
	rows := readVerifyLedger()
	if len(rows) != 1 || rows[0].Origin != verifyOriginClaimMoment || rows[0].Failing != 0 || rows[0].Verified != 1 {
		t.Errorf("ledger rows = %+v, want one clean origin=claim-moment row", rows)
	}
	clean := false
	for _, b := range claimBeats(t, beatsPath) {
		if b.Channel == claimMomentChannel && b.Kind == "clean" {
			clean = true
		}
	}
	if !clean {
		t.Error("no clean beat after a proven claim")
	}
	// withClaimMoment: silence leaves the obligations payload untouched.
	if text, ok := withClaimMoment(srv.URL, dir, planPath, "sess-2", "rules", true); text != "rules" || !ok {
		t.Errorf("withClaimMoment on a clean claim = %q, %v; want the payload unchanged", text, ok)
	}
	if text, ok := withClaimMoment(srv.URL, dir, planPath, "sess-2", "", false); text != "" || ok {
		t.Errorf("withClaimMoment on a clean claim with no payload = %q, %v; want silence", text, ok)
	}
}

// TestClaimMomentVerify_NoNewClaimCostsNothing: a plan edit that checks
// nothing new, and a non-plan edit, make ZERO daemon calls.
func TestClaimMomentVerify_NoNewClaimCostsNothing(t *testing.T) {
	isolateClaimTelemetry(t)
	md := "- [x] One {id: T-1, symbol: Ghost}\n"
	dir, planPath := claimRepo(t, md, md+"- [ ] Two {id: T-2}\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("daemon called for an edit that claimed nothing: %s", r.URL)
	}))
	defer srv.Close()
	hc := &http.Client{Timeout: 2 * time.Second}
	if got := claimMomentVerifyWith(hc, srv.URL, dir, planPath, ""); got != "" {
		t.Errorf("got %q", got)
	}
	// Not a depth-1 plan: README, an archived plan, a source file.
	for _, rel := range []string{"docs/plans/README.md", "docs/plans/archive/old.md", "main.go"} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte("- [x] X {id: X-1, symbol: Nope}\n"), 0o644)
		if got := claimMomentVerifyWith(hc, srv.URL, dir, p, ""); got != "" {
			t.Errorf("%s: got %q, want silence", rel, got)
		}
		if _, ok := claimMomentPlanRel(dir, p); ok {
			t.Errorf("%s classified as a depth-1 plan", rel)
		}
	}
	if rel, ok := claimMomentPlanRel(dir, planPath); !ok || rel != "docs/plans/p.md" {
		t.Errorf("claimMomentPlanRel = %q, %v", rel, ok)
	}
}

// TestClaimMomentVerify_DaemonDownIsSilent pins I9 for the new path: a
// down daemon (and a slow one) is silence and exit-0 behavior, with an
// error beat naming the stage, and no ledger row (nothing was verified).
func TestClaimMomentVerify_DaemonDownIsSilent(t *testing.T) {
	_, beatsPath := isolateClaimTelemetry(t)
	oldMD := "- [ ] One {id: T-1, symbol: Ghost}\n"
	newMD := "- [x] One {id: T-1, symbol: Ghost}\n"
	dir, planPath := claimRepo(t, oldMD, newMD)
	hc := &http.Client{Timeout: 500 * time.Millisecond}
	if got := claimMomentVerifyWith(hc, "http://127.0.0.1:1", dir, planPath, "sess-3"); got != "" {
		t.Errorf("daemon down must be silent, got %q", got)
	}
	if rows := readVerifyLedger(); len(rows) != 0 {
		t.Errorf("ledger rows = %+v, want none (nothing verified)", rows)
	}
	errBeat := false
	for _, b := range claimBeats(t, beatsPath) {
		if b.Channel == claimMomentChannel && b.Kind == "error" {
			errBeat = true
		}
	}
	if !errBeat {
		t.Error("no error beat for a down daemon")
	}

	// Slow verify: list_services answers, verify_plan stalls past the client timeout.
	svc := strings.ToLower(filepath.Base(dir))
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Params.Name == "verify_plan" {
			time.Sleep(900 * time.Millisecond)
		}
		inner := fmt.Sprintf(`{"services":[{"service":%q,"symbol_count":10}]}`, svc)
		env := map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": inner}}}}
		_ = json.NewEncoder(w).Encode(env)
	}))
	defer slow.Close()
	if got := claimMomentVerifyWith(hc, slow.URL, dir, planPath, "sess-3"); got != "" {
		t.Errorf("slow verify must be silent at the claim moment (the Stop hook and the push gate still stand), got %q", got)
	}
}

// TestClaimRefusals_SortedAndUnknownNeverFires pins the shared formatter
// both gates use.
func TestClaimRefusals_SortedAndUnknownNeverFires(t *testing.T) {
	t.Setenv("FATHOM_NARRATE_STATE", filepath.Join(t.TempDir(), "narrate.json"))
	report := verify.Report{Tasks: []verify.TaskVerdict{
		{TaskID: "T-1", Title: "One", Status: verify.Fail, Label: "claimed-but-absent", Criteria: []verify.Criterion{{Name: "symbol_present", Status: verify.Fail, Detail: "absent"}}},
		{TaskID: "T-2", Title: "Two", Status: verify.Unknown, Label: "no-contract"},
		{TaskID: "T-3", Title: "Three", Status: verify.Fail, Label: "claimed-but-absent"},
		{TaskID: "T-4", Title: "Four", Status: verify.Fail, Label: "not newly claimed"},
	}}
	got := claimRefusals(t.TempDir(), "docs/plans/p.md", []string{"T-3", "T-1", "T-2"}, report)
	want := []string{
		"docs/plans/p.md [T-1] One — symbol_present: absent",
		"docs/plans/p.md [T-3] Three — claimed-but-absent",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("claimRefusals = %q, want %q (sorted; T-2 unknown never refuses)", got, want)
	}
}
