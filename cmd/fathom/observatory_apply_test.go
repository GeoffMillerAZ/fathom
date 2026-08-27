package main

// observatory_apply_test.go — SH-7 / SH-8, docs/plans/ambient-fathom.md.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func stubRegistry(services ...registeredService) wiringRegistry {
	return wiringRegistry{list: func() ([]registeredService, error) { return services, nil }}
}

// TestMutationEndpointRefusals pins SH-8 (law 13): the gate refuses a
// non-loopback Host, a cross-site Origin, a loopback Origin on ANOTHER
// port (a different local server is a different origin), a service id the
// registry does not know, and any path that is not a registered service
// root — before the handler behind it runs — and lets a same-origin
// loopback request for a registered service through with the registry's
// root, never the caller's. The loopback spellings are one host: a page
// opened as localhost may post to 127.0.0.1 and vice versa.
func TestMutationEndpointRefusals(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	reg := stubRegistry(
		registeredService{Service: "demo", Root: root},
		registeredService{Service: "remote"}, // a corpus with no local path
	)
	reached := 0
	var seen mutationTarget
	gate := requireLocalMutation(reg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached++
		seen, _ = mutationTargetFrom(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	do := func(method, target, host string, headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, nil)
		req.Host = host
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rr := httptest.NewRecorder()
		gate.ServeHTTP(rr, req)
		return rr
	}

	refusals := []struct {
		name    string
		method  string
		target  string
		host    string
		headers map[string]string
		want    int
	}{
		{"non-loopback Host", http.MethodPost, "/observatory/wiring/apply?service=demo", "fathom.example.net:7575", nil, http.StatusForbidden},
		{"LAN address Host", http.MethodPost, "/observatory/wiring/apply?service=demo", "192.168.1.20:7575", nil, http.StatusForbidden},
		{"cross-site Origin", http.MethodPost, "/observatory/wiring/apply?service=demo", "127.0.0.1:7575", map[string]string{"Origin": "http://evil.example"}, http.StatusForbidden},
		{"loopback Origin on another port", http.MethodPost, "/observatory/wiring/apply?service=demo", "127.0.0.1:7575", map[string]string{"Origin": "http://127.0.0.1:9999"}, http.StatusForbidden},
		{"localhost Origin on another port", http.MethodPost, "/observatory/wiring/apply?service=demo", "127.0.0.1:7575", map[string]string{"Origin": "http://localhost:9999"}, http.StatusForbidden},
		{"loopback Origin with no port against a ported Host", http.MethodPost, "/observatory/wiring/apply?service=demo", "127.0.0.1:7575", map[string]string{"Origin": "http://127.0.0.1"}, http.StatusForbidden},
		{"loopback Origin with the wrong scheme", http.MethodPost, "/observatory/wiring/apply?service=demo", "127.0.0.1:7575", map[string]string{"Origin": "https://127.0.0.1:7575"}, http.StatusForbidden},
		{"cross-site Sec-Fetch-Site", http.MethodPost, "/observatory/wiring/apply?service=demo", "127.0.0.1:7575", map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		{"same-site (other port) Sec-Fetch-Site", http.MethodPost, "/observatory/wiring/apply?service=demo", "127.0.0.1:7575", map[string]string{"Sec-Fetch-Site": "same-site"}, http.StatusForbidden},
		{"unknown service id", http.MethodPost, "/observatory/wiring/apply?service=ghost", "127.0.0.1:7575", nil, http.StatusBadRequest},
		{"no service id", http.MethodPost, "/observatory/wiring/apply", "127.0.0.1:7575", nil, http.StatusBadRequest},
		{"path that is not the registered root", http.MethodPost, "/observatory/wiring/apply?service=demo&path=" + url.QueryEscape(t.TempDir()), "127.0.0.1:7575", nil, http.StatusBadRequest},
		{"service with no local root", http.MethodPost, "/observatory/wiring/apply?service=remote", "127.0.0.1:7575", nil, http.StatusBadRequest},
		{"GET is not a mutation", http.MethodGet, "/observatory/wiring/apply?service=demo", "127.0.0.1:7575", nil, http.StatusMethodNotAllowed},
	}
	for _, tc := range refusals {
		t.Run(tc.name, func(t *testing.T) {
			before := reached
			rr := do(tc.method, tc.target, tc.host, tc.headers)
			if rr.Code != tc.want {
				t.Errorf("status = %d, want %d (body: %s)", rr.Code, tc.want, rr.Body.String())
			}
			if reached != before {
				t.Errorf("the handler behind the gate RAN — a refusal must stop before any mutation")
			}
			if strings.TrimSpace(rr.Body.String()) == "" {
				t.Error("a refusal with no reason is a refusal a human cannot act on")
			}
		})
	}

	t.Run("happy path: loopback, same-origin, registered service", func(t *testing.T) {
		for _, hdr := range []map[string]string{
			nil,
			{"Origin": "http://127.0.0.1:7575", "Sec-Fetch-Site": "same-origin"},
			{"Origin": "http://localhost:7575"},
			{"Origin": "http://[::1]:7575"},
		} {
			before := reached
			rr := do(http.MethodPost, "/observatory/wiring/apply?service=demo", "localhost:7575", hdr)
			if rr.Code != http.StatusNoContent {
				t.Fatalf("headers %v: status = %d, want 204 (body: %s)", hdr, rr.Code, rr.Body.String())
			}
			if reached != before+1 {
				t.Fatal("the handler behind the gate did not run")
			}
			if seen.Service != "demo" || seen.Root != root {
				t.Errorf("target = %+v, want the registry's root %s for demo", seen, root)
			}
		}
		// The registered path itself is accepted when spelled back.
		rr := do(http.MethodPost, "/observatory/wiring/apply?service=demo&path="+url.QueryEscape(root), "127.0.0.1:7575", nil)
		if rr.Code != http.StatusNoContent {
			t.Errorf("registered path refused: %d %s", rr.Code, rr.Body.String())
		}
	})
}

// TestWiringApplyRequiresFreshPlan pins SH-7: preview returns the typed
// diff and a plan digest; apply requires that digest and refuses (409)
// when the repository changed in between — including a change OUTSIDE
// fathom's region; a fresh preview → apply writes exactly the previewed
// plan under a receipt; and undo — which requires the receipt_digest the
// apply result carried, exactly as apply requires the plan_digest —
// restores the prior bytes.
func TestWiringApplyRequiresFreshPlan(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	prior := "# demo\n\nhouse prose that must survive\n"
	claudePath := filepath.Join(root, "CLAUDE.md")
	if err := os.WriteFile(claudePath, []byte(prior), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := stubRegistry(registeredService{Service: "demo", Root: root})
	clock := func() time.Time { return time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC) }
	const base = "http://127.0.0.1:1" // the page model degrades to named notes; JSON paths never touch it
	preview := requireLocalMutation(reg, wiringPreviewHandler(base, clock))
	apply := requireLocalMutation(reg, wiringApplyHandler(base, clock))
	undo := requireLocalMutation(reg, wiringUndoHandler(base, clock))

	post := func(h http.Handler, path string, form url.Values) (int, map[string]any) {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
		req.Host = "127.0.0.1:7575"
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		var m map[string]any
		if strings.Contains(rr.Header().Get("Content-Type"), "application/json") {
			if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
				t.Fatalf("%s: decode: %v (%s)", path, err, rr.Body.String())
			}
		} else {
			m = map[string]any{"text": rr.Body.String()}
		}
		return rr.Code, m
	}

	// 1. preview: a typed diff and a digest.
	code, pv := post(preview, "/observatory/wiring/preview", url.Values{"service": {"demo"}})
	if code != http.StatusOK {
		t.Fatalf("preview status = %d: %v", code, pv)
	}
	digest, _ := pv["plan_digest"].(string)
	if !strings.HasPrefix(digest, "sha256:") {
		t.Fatalf("preview carries no plan digest: %v", pv)
	}
	plan, _ := pv["plan"].(map[string]any)
	items, _ := plan["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("preview has no items: %v", pv)
	}
	if got, _ := post(preview, "/observatory/wiring/preview", url.Values{"service": {"demo"}}); got != http.StatusOK {
		t.Fatalf("second preview status = %d", got)
	}

	// 2. apply without a digest is refused; nothing is applied unseen.
	if code, body := post(apply, "/observatory/wiring/apply", url.Values{"service": {"demo"}}); code != http.StatusBadRequest {
		t.Fatalf("apply without plan_digest: status = %d, want 400 (%v)", code, body)
	}

	// 3. the repository changes OUTSIDE fathom's region → stale digest → 409.
	if err := os.WriteFile(claudePath, []byte(prior+"\nan unrelated line added after the preview\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, body := post(apply, "/observatory/wiring/apply", url.Values{"service": {"demo"}, "plan_digest": {digest}})
	if code != http.StatusConflict {
		t.Fatalf("apply with a stale digest: status = %d, want 409 (%v)", code, body)
	}
	if got, _ := os.ReadFile(claudePath); strings.Contains(string(got), fathomStanzaBeginPrefix) {
		t.Fatal("a refused apply WROTE the stanza")
	}
	if _, err := os.Stat(filepath.Join(root, ".fathom", "receipts.jsonl")); err == nil {
		t.Fatal("a refused apply wrote a receipt — nothing should have been written")
	}

	// 4. fresh preview → apply writes exactly that plan, receipted.
	code, pv = post(preview, "/observatory/wiring/preview", url.Values{"service": {"demo"}})
	if code != http.StatusOK {
		t.Fatalf("re-preview status = %d", code)
	}
	fresh, _ := pv["plan_digest"].(string)
	if fresh == digest {
		t.Fatal("the plan digest did not change when a target file changed outside the region")
	}
	code, body = post(apply, "/observatory/wiring/apply", url.Values{"service": {"demo"}, "plan_digest": {fresh}})
	if code != http.StatusOK {
		t.Fatalf("apply with a fresh digest: status = %d, want 200 (%v)", code, body)
	}
	result, _ := body["result"].(map[string]any)
	if result["receipt"] == nil {
		t.Fatalf("apply wrote no receipt: %v", body)
	}
	applied, _ := result["applied"].([]any)
	if len(applied) == 0 {
		t.Fatalf("apply reports nothing applied: %v", body)
	}
	got, _ := os.ReadFile(claudePath)
	if !strings.HasPrefix(string(got), prior) || !strings.Contains(string(got), fathomStanzaBody("demo", false)) {
		t.Errorf("CLAUDE.md after apply is not prior bytes + fathom's region:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "settings.json")); err != nil {
		t.Errorf("hooks were previewed but not written: %v", err)
	}

	// 5. the apply itself moved the repository: the fresh digest is now stale.
	if code, _ := post(apply, "/observatory/wiring/apply", url.Values{"service": {"demo"}, "plan_digest": {fresh}}); code != http.StatusConflict {
		t.Errorf("re-apply with the consumed digest: status = %d, want 409", code)
	}

	// 6. undo needs the receipt_digest the apply result carried: none is
	// 400, a wrong one is 409, and in both cases nothing is restored —
	// `receipt=last` alone was the one mutation a stranger could drive blind.
	receiptDigestToken, _ := body["receipt_digest"].(string)
	if !strings.HasPrefix(receiptDigestToken, "sha256:") {
		t.Fatalf("apply result carries no receipt_digest: %v", body)
	}
	afterApply, _ := os.ReadFile(claudePath)
	if code, ub := post(undo, "/observatory/wiring/undo", url.Values{"service": {"demo"}, "receipt": {"last"}}); code != http.StatusBadRequest {
		t.Fatalf("undo without receipt_digest: status = %d, want 400 (%v)", code, ub)
	}
	if code, ub := post(undo, "/observatory/wiring/undo", url.Values{"service": {"demo"}, "receipt": {"last"}, "receipt_digest": {"sha256:0000"}}); code != http.StatusConflict {
		t.Fatalf("undo with a wrong receipt_digest: status = %d, want 409 (%v)", code, ub)
	}
	if now, _ := os.ReadFile(claudePath); string(now) != string(afterApply) {
		t.Fatal("a refused undo RESTORED the file")
	}

	// 7. undo restores the prior bytes exactly (the receipt is the undo).
	code, body = post(undo, "/observatory/wiring/undo", url.Values{"service": {"demo"}, "receipt": {"last"}, "receipt_digest": {receiptDigestToken}})
	if code != http.StatusOK {
		t.Fatalf("undo status = %d: %v", code, body)
	}
	if rd, _ := body["receipt_digest"].(string); !strings.HasPrefix(rd, "sha256:") || rd == receiptDigestToken {
		t.Errorf("the undo result carries no token of its own receipt (got %q)", rd)
	}
	got, _ = os.ReadFile(claudePath)
	if string(got) != prior+"\nan unrelated line added after the preview\n" {
		t.Errorf("undo did not restore the pre-apply bytes:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Errorf("undo left the settings.json the apply created (err=%v)", err)
	}
}

// TestWiringPagesRenderHTML is the no-browser smoke check for the HTML
// half: the page lists the Preview button, the preview renders the typed
// diff and the consent card carrying the plan digest, and the fleet model
// degrading (no daemon) never breaks the page.
func TestWiringPagesRenderHTML(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	hermeticObsEnv(t)
	root := t.TempDir()
	reg := stubRegistry(registeredService{Service: "demo", Root: root}, registeredService{Service: "remote"})
	clock := func() time.Time { return time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC) }
	h := wiringRoutes(reg, "http://127.0.0.1:1", clock)

	req := httptest.NewRequest(http.MethodGet, "/observatory/wiring/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "Preview changes") || !strings.Contains(rr.Body.String(), "nothing to wire") {
		t.Fatalf("wiring page: %d\n%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/observatory/wiring/preview", strings.NewReader("service=demo"))
	req.Host = "127.0.0.1:7575"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	body := rr.Body.String()
	if rr.Code != http.StatusOK {
		t.Fatalf("preview page: %d\n%s", rr.Code, body)
	}
	for _, want := range []string{"Apply exactly this", `name="plan_digest" value="sha256:`, "Proposed changes", "CLAUDE.md"} {
		if !strings.Contains(body, want) {
			t.Errorf("preview page missing %q", want)
		}
	}
	if strings.Contains(body, "bg-indigo") || strings.Contains(body, "style=\"color") {
		t.Error("palette colour or inline colour in the rendering — role utilities only")
	}
}
