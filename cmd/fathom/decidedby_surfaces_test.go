package main

// decidedby_surfaces_test.go — the disclosure rows (ADR-073 decision 6,
// DMP-3): POST /route and certify suggest carry decided_by only when a
// configured port was asked (JG-2, JG-3), and under every kill source no
// surface that can carry a decision holds a judged token (JG-5). The
// provider is a fake transport that answers every question it is sent, so
// any ask a kill failed to stop would serve a judged answer; no socket, no
// key, $0.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/readout"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// jgAnswering is a fake TypeSafe transport that answers every question it
// is sent: a predicate at noul 0.95, a choice with its first option (by
// name) at 0.95; with fail set it answers 429 instead.
type jgAnswering struct {
	mu   sync.Mutex
	n    int
	fail bool
}

func (a *jgAnswering) RoundTrip(r *http.Request) (*http.Response, error) {
	a.mu.Lock()
	a.n++
	fail := a.fail
	a.mu.Unlock()
	reply := func(status int, body string) (*http.Response, error) {
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	}
	if fail {
		return reply(http.StatusTooManyRequests, `{"error":"try later"}`)
	}
	var req struct {
		Questions map[string]struct {
			Type     string          `json:"type"`
			Criteria json.RawMessage `json:"criteria"`
		} `json:"questions"`
	}
	if r.Body != nil {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &req)
	}
	answers := map[string]any{}
	for id, q := range req.Questions {
		if q.Type != "choice" {
			answers[id] = map[string]any{"type": "noul", "noul": 0.95}
			continue
		}
		var opts map[string]any
		_ = json.Unmarshal(q.Criteria, &opts)
		names := make([]string, 0, len(opts))
		for n := range opts {
			names = append(names, n)
		}
		sort.Strings(names)
		probs := map[string]float64{}
		for i, n := range names {
			if i == 0 {
				probs[n] = 0.95
			} else {
				probs[n] = 0.05 / float64(len(names)-1)
			}
		}
		if len(names) > 0 {
			answers[id] = map[string]any{"type": "choice", "choice": names[0], "probabilities": probs}
		}
	}
	body, _ := json.Marshal(map[string]any{"model": "jev-test", "answers": answers})
	return reply(http.StatusOK, string(body))
}

func (a *jgAnswering) calls() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.n
}

const jgKeyVar = "FATHOM_TEST_JG_KEY"

// jgOpen opens the port from loaded as decide.Open's callers do, over the
// fake transport and a fake key source; disclose collects its lines.
func jgOpen(t *testing.T, loaded config.Loaded, tr http.RoundTripper, disclose io.Writer) *decide.Ports {
	t.Helper()
	t.Setenv(jgKeyVar, "test-key-jg-not-a-real-key") // the adapter reads its key at ask time
	// SR-58: every site may rule, so the badges below are the judged ones
	// (the tests set FATHOM_HOME, whose calibration/ the port reads).
	calibrateSites(t, decide.ReadingsDir(), "typesafe", "jev-test", decide.AllSites...)
	ports, err := decide.Open(loaded.Config.Decide, nil, "", decide.NewRecorder(io.Discard, func() time.Time { return time.Unix(0, 0).UTC() }, "test"),
		decide.WithHTTPClient(&http.Client{Transport: tr}), decide.WithDisclose(disclose), decide.WithUserConfig(loaded.UserPath),
		decide.WithKeyIO(config.KeyIO{
			LookupEnv: func(name string) (string, bool) { return "test-key-jg-not-a-real-key", name == jgKeyVar },
			Stat:      os.Stat,
			RunCmd:    func(context.Context, []string) (string, error) { return "", nil },
		}))
	if err != nil {
		t.Fatalf("decide.Open: %v", err)
	}
	return ports
}

// jgWrite writes body to path, creating its directory.
func jgWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// jgRoute posts one prompt to POST /route and returns the body.
func jgRoute(t *testing.T, ports *decide.Ports) string {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/route", strings.NewReader(`{"prompt":"where is Open defined"}`))
	req.Header.Set(client.CallerHeaderName, "pid=7;ppid=1;origin=prompt-hook;session=jg;test=0")
	routeHandler(ports).ServeHTTP(rr, req)
	return rr.Body.String()
}

const jgObligationsFile = "pkg/jg/target.go"

// jgObligations serves the obligations tool for two claims through a real
// client wired to d, as serve-http wires the daemon, and returns the
// envelope's text.
func jgObligations(t *testing.T, d decision.Decider, cfg config.Decide) string {
	t.Helper()
	st := store.NewMemoryStore()
	for _, id := range []string{"jg-1", "jg-2"} {
		claim := core.SemanticClaim{
			ClaimID: id, ServiceID: "svc", Layer: "obligations", LayerVersion: "v1", Kind: "obligation",
			Subject: "docs/g.md#" + id, Rule: id + ": requires review before any change.", Scope: []string{jgObligationsFile},
			Tier: core.TierDocClaimed, TierProfile: "engineering", Status: core.ClaimFresh, CompiledAt: time.Unix(1, 0).UTC(),
		}
		links := []core.ClaimLink{
			{ClaimID: id, Rel: core.RelCites, TargetRef: "docs/g.md#L1-L2", TargetHash: "h"},
			{ClaimID: id, Rel: core.RelScopes, TargetRef: jgObligationsFile},
		}
		if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, links); err != nil {
			t.Fatal(err)
		}
	}
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	c.WithDecision(d, cfg, nil)
	line, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "obligations", "arguments": map[string]any{"service": "svc", "file": jgObligationsFile}}})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	_ = mcp.NewServer(c).Serve(context.Background(), bytes.NewReader(append(line, '\n')), &out)
	var resp struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil || len(resp.Result.Content) == 0 {
		t.Fatalf("obligations answer %q: %v", out.String(), err)
	}
	return resp.Result.Content[0].Text
}

// jgCertify runs certify suggest over the ingested fixture store.
func jgCertify(t *testing.T, ports *decide.Ports, service, storePath string) (stdout, stderr, file string) {
	t.Helper()
	outPath := filepath.Join(t.TempDir(), "suggestions.jsonl")
	var out, errb bytes.Buffer
	if code := runCertifySuggest([]string{"--service", service, "--store", storePath, "--out", outPath}, ports,
		func() time.Time { return time.Unix(0, 0).UTC() }, &out, &errb); code != 0 {
		t.Fatalf("certify suggest exit %d: %s", code, errb.String())
	}
	data, _ := os.ReadFile(outPath)
	return out.String(), errb.String(), string(data)
}

// jgIngestCertifyFixture ingests the certify fixture once.
func jgIngestCertifyFixture(t *testing.T) (service, storePath string) {
	t.Helper()
	root := t.TempDir()
	jgWrite(t, filepath.Join(root, "pkg_test.go"), certifySuggestFixtureSrc)
	service, storePath = "jg-svc", filepath.Join(t.TempDir(), "s.sqlite")
	var o, e bytes.Buffer
	if code := run([]string{"ingest", "--service", service, "--path", root, "--store", storePath}, &o, &e); code != 0 {
		t.Fatalf("ingest exit %d: %s", code, e.String())
	}
	return service, storePath
}

// TestRouteAndCertifySuggestDecidedByOnlyWhenConfigured is JG-2 for POST
// /route and certify suggest (the obligations envelope is
// TestObligationsWire_DecidedByOnlyWhenConfigured): a judged answer names
// judged(<site> · <provider>/<model> · conf <x> · calibration uncalibrated
// 0/100); a miss names `computed · not consulted (<class>)` on the answer
// itself; provider none keeps I42's bytes (TestI42UnconfiguredDecideOutput
// IsByteIdentical's golden, re-read here).
func TestRouteAndCertifySuggestDecidedByOnlyWhenConfigured(t *testing.T) {
	home := t.TempDir()
	t.Setenv("FATHOM_HOME", home)
	t.Setenv(decide.KillEnv, "")
	user := fathomhome.Path("config.toml")
	jgWrite(t, user, "[decide]\nprovider = \"typesafe\"\napi_key_env = \""+jgKeyVar+"\"\nsites = [\"route\", \"certify_suggest\"]\n")
	loaded, err := config.Load(user, "")
	if err != nil {
		t.Fatal(err)
	}
	service, storePath := jgIngestCertifyFixture(t)

	tr := &jgAnswering{}
	var disclose bytes.Buffer
	ports := jgOpen(t, loaded, tr, &disclose)

	var route routeResponse
	if err := json.Unmarshal([]byte(jgRoute(t, ports)), &route); err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^judged\(route · typesafe/jev-test · conf 0\.\d\d · calibration uncalibrated 0/100\)$`).MatchString(route.DecidedBy) {
		t.Errorf("route decided_by = %q, want the judged badge", route.DecidedBy)
	}
	stdout, _, file := jgCertify(t, ports, service, storePath)
	if !regexp.MustCompile(`\) decided_by judged\(certify_suggest · typesafe/jev-test · conf 0\.\d\d · calibration uncalibrated 0/100\)\n`).MatchString(stdout) {
		t.Errorf("certify stdout carries no judged badge:\n%s", stdout)
	}
	if !strings.Contains(file, `"decided_by":"judged(certify_suggest`) {
		t.Errorf("certify suggestions carry no decided_by:\n%s", file)
	}

	// A miss: every answer still names its class, the line once.
	tr.mu.Lock()
	tr.fail = true
	tr.mu.Unlock()
	for i := 0; i < 2; i++ {
		if err := json.Unmarshal([]byte(jgRoute(t, ports)), &route); err != nil {
			t.Fatal(err)
		}
		if route.DecidedBy != "computed · not consulted (rate limited)" || route.Provider != "none" {
			t.Errorf("ask %d: route under a miss = %+v, want the none shape and decided_by computed · not consulted (rate limited)", i, route)
		}
	}
	if n := strings.Count(disclose.String(), "decision port degraded (rate limited) for site route, session jg"); n != 1 {
		t.Errorf("two missed route asks from one session wrote %d degrade lines, want 1:\n%s", n, disclose.String())
	}
	stdout, _, _ = jgCertify(t, ports, service, storePath)
	if !strings.Contains(stdout, "decided_by computed · not consulted (rate limited)") {
		t.Errorf("certify summary under a miss names no class:\n%s", stdout)
	}

	// Provider none: the I42 golden's route body, no badge anywhere.
	none, err := config.Load("", "")
	if err != nil {
		t.Fatal(err)
	}
	nonePorts := openDecisionPorts(none, "", "", "test", io.Discard)
	if got, want := jgRoute(t, nonePorts), jgGoldenSection(t, "== route body\n"); got != want {
		t.Errorf("provider none route body = %q, want the I42 golden's %q", got, want)
	}
	noneOut, _, _ := jgCertify(t, nonePorts, service, storePath)
	if want := jgGoldenSection(t, "== certify suggest stdout\n"); noneOut != want {
		t.Errorf("provider none certify stdout = %q, want the I42 golden's %q", noneOut, want)
	}
}

// jgGoldenSection is one section of the I42 golden (TestI42Unconfigured
// DecideOutputIsByteIdentical's): the bytes after header up to the next
// "== " header.
func jgGoldenSection(t *testing.T, header string) string {
	t.Helper()
	raw, err := os.ReadFile(i42UnconfiguredGolden)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	i := strings.Index(s, header)
	if i < 0 {
		t.Fatalf("the I42 golden has no %q section", header)
	}
	s = s[i+len(header):]
	if j := strings.Index(s, "\n== "); j >= 0 {
		s = s[:j+1]
	}
	return s
}

// TestNoJudgedTokenOnAnySurfaceUnderKill is JG-5: under each kill source
// (KS-6's precedence: the environment, the kill file, the user's enabled,
// a refused consent, deny_sites, sites), with a provider that would answer
// every question, no surface that can carry a decision (the obligations
// envelope, POST /route, certify suggest, the readout receipt, the status
// line) holds a judged token or a decided_by, nothing is dialed, and each
// decision-bearing payload is provider none's: byte for byte where the port
// was not asked (the I42 golden's route body and certify line), and the
// same rules and ranked_by where a live kill refused an ask. session_sites
// closes only a session site (readout_mode, compact_advice), which none of
// these surfaces asks; the readout receipt reads no decision at all, so its
// bytes are compared with the unkilled run's.
func TestNoJudgedTokenOnAnySurfaceUnderKill(t *testing.T) {
	service, storePath := jgIngestCertifyFixture(t)
	base := "[decide]\nprovider = \"typesafe\"\napi_key_env = \"" + jgKeyVar + "\"\nrank_within_tier = true\n"
	allSites := "sites = [\"route\", \"obligations_rerank\", \"certify_suggest\"]\n"
	vet := "internal/shop/cart.go:10:1: unreachable code\ninternal/shop/cart.go:20:1: unreachable code\n"
	statusFake := newStatuslineFake(t, http.StatusOK, statuslineAnswerJSON("0", "false"))

	type surfaces struct {
		route, obligations, certifyOut, certifyErr, certifyFile, readout, status, disclose string
		dials                                                                              int
	}
	render := func(t *testing.T, userBody, projectBody string, setup func(t *testing.T, home string)) surfaces {
		t.Helper()
		home := t.TempDir()
		t.Setenv("FATHOM_HOME", home)
		t.Setenv(decide.KillEnv, "")
		repo := t.TempDir()
		user := fathomhome.Path("config.toml")
		jgWrite(t, user, userBody)
		project := ""
		if projectBody != "" {
			project = filepath.Join(repo, ".fathom", "config.toml")
			jgWrite(t, project, projectBody)
		}
		if setup != nil {
			setup(t, home)
		}
		loaded, err := config.Load(user, project)
		if err != nil {
			t.Fatalf("config.Load: %v", err)
		}
		tr := &jgAnswering{}
		var disclose bytes.Buffer
		ports := jgOpen(t, loaded, tr, &disclose)
		var s surfaces
		s.route = jgRoute(t, ports)
		s.obligations = jgObligations(t, ports.Decider, loaded.Config.Decide)
		s.certifyOut, s.certifyErr, s.certifyFile = jgCertify(t, ports, service, storePath)
		rs, err := readout.LoadRules(fathomhome.FathomHome())
		if err != nil {
			t.Fatalf("readout rules: %v", err)
		}
		cmd := shellcmd.ParseCommand("go vet ./...")
		s.readout = readout.Render(rs, readout.Input{Command: &cmd, Stdout: vet}).Stdout
		stdin := `{"workspace":{"project_dir":"` + repo + `"}}`
		s.status = runStatusline(t, stdin, "--url", statusFake.srv.URL) + runStatusline(t, stdin, "--url", statusFake.srv.URL, "--json")
		s.disclose = disclose.String()
		s.dials = tr.calls()
		return s
	}

	// The control: the same harness unkilled serves judged answers on
	// every surface that can carry one, so the kills below are not vacuous.
	open := render(t, base+allSites, "", nil)
	for name, text := range map[string]string{"route": open.route, "obligations": open.obligations, "certify": open.certifyOut} {
		if !strings.Contains(text, "judged(") {
			t.Fatalf("control: %s carries no judged badge, so this harness proves nothing:\n%s", name, text)
		}
	}
	if open.dials == 0 || !strings.Contains(open.status, "J:on(3)") {
		t.Fatalf("control: dials %d, status %q, want asks and J:on(3)", open.dials, open.status)
	}

	// Provider none's payloads, the bytes a closed port must serve.
	noneObligations := jgObligations(t, decision.None{}, config.DefaultDecide)
	noneRoute := jgGoldenSection(t, "== route body\n")
	noneCertify := jgGoldenSection(t, "== certify suggest stdout\n")

	for _, tc := range []struct {
		source       string
		user, proj   string
		setup        func(t *testing.T, home string)
		asked        bool // a live kill refuses an ask the port received
		statusToken  string
		routeIsNone  bool // the route body is the I42 golden's bytes
		certIsNone   bool // certify's stdout is the I42 golden's line
		obligIsBytes bool // the obligations envelope is provider none's bytes
	}{
		{source: decide.KillSourceEnv, user: base + allSites, setup: func(t *testing.T, _ string) { t.Setenv(decide.KillEnv, decide.KillValue) },
			statusToken: "J:killed", routeIsNone: true, certIsNone: true, obligIsBytes: true},
		{source: decide.KillSourceFile, user: base + allSites, setup: func(t *testing.T, home string) { jgWrite(t, filepath.Join(home, decide.KillFileName), "") },
			asked: true, statusToken: "J:killed"},
		{source: decide.KillSourceEnabled, user: base + allSites + "enabled = false\n", asked: true, statusToken: "J:killed"},
		{source: decide.KillSourceConsent, user: "[decide]\napi_key_env = \"" + jgKeyVar + "\"\nrank_within_tier = true\n" + allSites,
			proj: "[decide]\nprovider = \"typesafe\"\n", statusToken: "J:killed", routeIsNone: true, certIsNone: true, obligIsBytes: true},
		{source: decide.KillSourceDeny, user: base + allSites + "deny_sites = [\"route\", \"obligations_rerank\", \"certify_suggest\"]\n",
			statusToken: "J:off", obligIsBytes: true},
		{source: decide.KillSourceSites, user: base + "sites = [\"assess\"]\n", statusToken: "J:on(1)", obligIsBytes: true},
	} {
		t.Run(tc.source, func(t *testing.T) {
			s := render(t, tc.user, tc.proj, tc.setup)
			all := strings.Join([]string{s.route, s.obligations, s.certifyOut, s.certifyErr, s.certifyFile, s.readout, s.status, s.disclose}, "\n")
			if strings.Contains(all, "judged") {
				t.Errorf("a judged token under kill_source %s:\n%s", tc.source, all)
			}
			if strings.Contains(all, "decided_by") {
				t.Errorf("a decided_by under kill_source %s:\n%s", tc.source, all)
			}
			if s.dials != 0 {
				t.Errorf("kill_source %s dialed the provider %d times", tc.source, s.dials)
			}
			var route routeResponse
			if err := json.Unmarshal([]byte(s.route), &route); err != nil {
				t.Fatal(err)
			}
			if route.Shape != "" || route.Verb != "" || route.Confidence != 0 || route.Band != string(decision.BandUnsure) || route.Provider != "none" {
				t.Errorf("route under kill_source %s = %+v, want the none shape", tc.source, route)
			}
			if tc.routeIsNone && s.route != noneRoute {
				t.Errorf("route body under kill_source %s = %q, want the I42 golden's %q", tc.source, s.route, noneRoute)
			}
			if tc.certIsNone && s.certifyOut != noneCertify {
				t.Errorf("certify under kill_source %s = %q, want the I42 golden's %q", tc.source, s.certifyOut, noneCertify)
			}
			if strings.Contains(s.certifyOut, "suggested:") || strings.TrimSpace(s.certifyFile) != "" {
				t.Errorf("certify under kill_source %s suggested something:\n%s\n%s", tc.source, s.certifyOut, s.certifyFile)
			}
			if tc.obligIsBytes && s.obligations != noneObligations {
				t.Errorf("obligations under kill_source %s moved off provider none's bytes:\n--- got\n%s\n--- want\n%s", tc.source, s.obligations, noneObligations)
			}
			var got, want map[string]any
			_ = json.Unmarshal([]byte(s.obligations), &got)
			_ = json.Unmarshal([]byte(noneObligations), &want)
			if got["ranked_by"] != "none" || !jsonEqual(got["rules"], want["rules"]) {
				t.Errorf("obligations under kill_source %s: ranked_by %v, rules %v; want none and provider none's rules %v", tc.source, got["ranked_by"], got["rules"], want["rules"])
			}
			if tc.asked && !strings.Contains(s.disclose, "decision port killed (kill_source "+tc.source) {
				t.Errorf("kill_source %s refused an ask without its killed line:\n%s", tc.source, s.disclose)
			}
			if s.readout != open.readout {
				t.Errorf("readout under kill_source %s = %q, want the unkilled bytes %q", tc.source, s.readout, open.readout)
			}
			if !strings.HasPrefix(s.status, "fathom ● @abcdef1 "+tc.statusToken+"\n") {
				t.Errorf("status line under kill_source %s = %q, want the index state then %s", tc.source, s.status, tc.statusToken)
			}
		})
	}
}

// jsonEqual compares two decoded JSON values.
func jsonEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}
