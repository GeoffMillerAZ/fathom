//go:build decidelive

package decide_test

// live_smoke_test.go — DP-37 (docs/plans/decision-port.md section H): a
// bounded live smoke of the two remote providers through fathom's own
// decide.Open, so the request and reply shapes the kit's adapters expect
// are checked against the real wire. Never in the gate: the build tag
// keeps it out of every `go test ./...`, and FATHOM_DECIDE_LIVE=1 must be
// set as well. Owner-approved 2026-10-09: at most 20 calls per provider.
//
//	FATHOM_DECIDE_LIVE=1 FATHOM_DECIDE_LIVE_OUT=<file.json> \
//	  go test -tags decidelive -run TestLiveProviderSmoke -count=1 -v ./internal/decide
//
// Keys: OpenAI's from the OPENAI_API_KEY variable (api_key_env), Jev over
// OpenRouter's from a chmod 600 file whose absolute path
// FATHOM_DECIDE_LIVE_OPENROUTER_KEY_FILE names (api_key_file; absolute,
// because the test harness gives the process a scratch HOME).
// This test never reads either key: the adapters do. It records bodies,
// statuses and timings only — never a header — and redacts key shapes from
// every recorded body.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// liveCallCap is the owner's bound: at most this many calls per provider.
const liveCallCap = 20

var liveRedactions = []struct {
	re   *regexp.Regexp
	with string
}{
	{regexp.MustCompile(`sk-[A-Za-z0-9_\-]{8,}`), "[REDACTED]"},
	{regexp.MustCompile(`(?i)bearer\s+[^\s"',;}\]]+`), "Bearer [REDACTED]"},
	{regexp.MustCompile(`(?i)("authorization"\s*:\s*")[^"]*`), "${1}[REDACTED]"},
}

func liveRedact(s string) string {
	for _, r := range liveRedactions {
		s = r.re.ReplaceAllString(s, r.with)
	}
	return s
}

// liveCall is one recorded exchange: no header is ever kept.
type liveCall struct {
	Provider   string          `json:"provider"`
	Label      string          `json:"label"`
	URL        string          `json:"url"`
	Status     int             `json:"status"`
	WallMS     int64           `json:"wall_ms"`
	WireReq    json.RawMessage `json:"wire_request,omitempty"`
	WireResp   json.RawMessage `json:"wire_response,omitempty"`
	WireRespTx string          `json:"wire_response_text,omitempty"`
	Mapped     *liveMapped     `json:"mapped,omitempty"`
	Error      string          `json:"error,omitempty"`
}

type liveMapped struct {
	Model     string                     `json:"model"`
	Answers   map[string]decision.Answer `json:"answers"`
	Invalid   map[string]string          `json:"invalid,omitempty"`
	Declined  map[string]string          `json:"declined,omitempty"`
	Usage     decision.Usage             `json:"usage"`
	LatencyMS int64                      `json:"latency_ms"`
}

// recorder wraps the default transport: it counts and caps calls per
// provider, stops calling a provider that refused access, and keeps the
// last exchange's bodies (redacted) for the current label.
type recorder struct {
	base http.RoundTripper

	mu      sync.Mutex
	calls   map[string]int
	refused map[string]string
	last    liveCall
}

func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Hostname()
	r.mu.Lock()
	if why, ok := r.refused[host]; ok {
		r.mu.Unlock()
		return nil, fmt.Errorf("not called: %s refused access earlier (%s)", host, why)
	}
	if r.calls[host] >= liveCallCap {
		r.mu.Unlock()
		return nil, fmt.Errorf("not called: the %d-call cap for %s is spent", liveCallCap, host)
	}
	r.calls[host]++
	r.mu.Unlock()

	var reqBody []byte
	if req.Body != nil {
		reqBody, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(reqBody))
	}
	start := time.Now()
	resp, err := r.base.RoundTrip(req)
	wall := time.Since(start)
	call := liveCall{URL: req.URL.Scheme + "://" + req.URL.Host + req.URL.Path, WallMS: wall.Milliseconds()}
	if json.Valid(reqBody) {
		call.WireReq = json.RawMessage(liveRedact(string(reqBody)))
	}
	if err != nil {
		call.Error = liveRedact(err.Error())
		r.mu.Lock()
		r.last = call
		r.mu.Unlock()
		return nil, err
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(respBody))
	call.Status = resp.StatusCode
	red := liveRedact(string(respBody))
	if json.Valid([]byte(red)) {
		call.WireResp = json.RawMessage(red)
	} else {
		call.WireRespTx = truncate(red, 2000)
	}
	r.mu.Lock()
	r.last = call
	// Refused access, a missing endpoint or an empty account: record it
	// and stop calling that provider (a 429 is a refusal only when the
	// body says the quota or credit is gone, never a plain rate limit).
	switch {
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusPaymentRequired,
		resp.StatusCode == http.StatusForbidden, resp.StatusCode == http.StatusNotFound:
		r.refused[host] = fmt.Sprintf("status %d", resp.StatusCode)
	case resp.StatusCode == http.StatusTooManyRequests && (strings.Contains(red, "insufficient_quota") || strings.Contains(red, "credit")):
		r.refused[host] = "status 429: quota or credit exhausted"
	}
	r.mu.Unlock()
	return resp, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// The fathom-shaped prompts: an obligation-relevance predicate, a route
// choice over the verb set, and a test-strength score, twice (one case
// each way), plus one batch of the three per set.
type liveSet struct {
	name                          string
	obligation, diff, prompt, tst string
}

var liveSets = []liveSet{
	{
		name:       "set1",
		obligation: `Obligation (docs/AGENTS.md): "symbol: anchors name FUNCTIONS/METHODS only - never struct/type names, and never a method name shared by two receivers in the same package."`,
		diff:       "Diff (docs/plans/decision-port.md):\n+- [ ] doctor names the key source {id: DP-32, symbol: Decide, package: internal/config, reachable_from: TestDoctorDecisionProvider_Remote}",
		prompt:     "User prompt: where is decisionProviderVerdict defined?",
		tst:        "Test (cmd/fathom/doctor_decide_test.go):\nfunc TestDoctorKeyFile(t *testing.T) {\n\tgot := decisionProviderVerdict(loaded, nil, \"\")\n\tif got.Status != doctorFail || !strings.Contains(got.Remedy, \"chmod 600\") {\n\t\tt.Fatalf(\"got %+v\", got)\n\t}\n\tif strings.Contains(got.Reason, key) {\n\t\tt.Errorf(\"leaks the key\")\n\t}\n}",
	},
	{
		name:       "set2",
		obligation: `Obligation (AGENTS.md): "Deploy = task deploy, NEVER a raw cp - cp truncates a running executable, and an unsigned binary loses the daemon's TCC grant."`,
		diff:       "Diff (internal/config/decidekey.go):\n+func DefaultDecideKeyEnv(transport string) string {\n+\tswitch transport {\n+\tcase DecideTransportOpenRouter:\n+\t\treturn \"OPENROUTER_API_KEY\"",
		prompt:     "User prompt: TestDecideDegradesToOneDisclosedLine fails with \"disclosed 2 lines, want exactly one\" after my change to degrade.go - what broke?",
		tst:        "Test (internal/decide/smoke_test.go):\nfunc TestOpenRuns(t *testing.T) {\n\t_, _ = decide.Open(config.Decide{}, nil, \"\", nil)\n}",
	},
}

var routeVerbs = []decision.Option{
	{Name: "localize", Criteria: "a failure, stack trace or error to trace to its cause"},
	{Name: "find_symbol", Criteria: "where a named symbol is defined"},
	{Name: "references", Criteria: "every use of a named symbol"},
	{Name: "find_callers", Criteria: "who calls a named function"},
	{Name: "obligations", Criteria: "which rules govern a file about to be edited"},
	{Name: "staleness_report", Criteria: "whether the index is current"},
	{Name: "search", Criteria: "a literal string, config value or log line"},
	{Name: "none", Criteria: "none of these fits"},
}

var strengthLevels = []string{
	"no assertion: the test only runs code",
	"smoke: checks only that no error occurred",
	"asserts one value or outcome",
	"asserts values and a failure or safety path",
}

func liveQuestions() (pred, choice, score decision.Question) {
	pred = decision.Question{ID: "obligation_relevant", Kind: decision.KindPredicate,
		Instructions:  "The obligation governs the change shown in the diff.",
		TrueCriteria:  "the diff edits what the obligation constrains",
		FalseCriteria: "the obligation is about something the diff does not touch"}
	choice = decision.Question{ID: "route_verb", Kind: decision.KindChoice,
		Instructions: "Which code-intelligence verb answers the user prompt?", Options: routeVerbs}
	score = decision.Question{ID: "test_strength", Kind: decision.KindScore,
		Instructions: "How strongly does the test check the behavior it names?", Levels: strengthLevels}
	return pred, choice, score
}

func TestLiveProviderSmoke(t *testing.T) {
	if os.Getenv("FATHOM_DECIDE_LIVE") != "1" {
		t.Skip("live smoke: set FATHOM_DECIDE_LIVE=1 (owner-approved, at most 20 calls per provider)")
	}
	out := os.Getenv("FATHOM_DECIDE_LIVE_OUT")
	if out == "" {
		t.Fatal("FATHOM_DECIDE_LIVE_OUT must name the JSON file the readings go to")
	}
	keyFile := os.Getenv("FATHOM_DECIDE_LIVE_OPENROUTER_KEY_FILE")
	rec := &recorder{base: http.DefaultTransport, calls: map[string]int{}, refused: map[string]string{}}
	http.DefaultTransport = rec
	t.Cleanup(func() { http.DefaultTransport = rec.base })

	providers := []struct {
		label string
		cfg   config.Decide
	}{
		{"openai", config.Decide{Provider: "openai", APIKeyEnv: "OPENAI_API_KEY"}},
		{"jev-openrouter", config.Decide{Provider: "typesafe", Transport: "openrouter", APIKeyFile: keyFile}}, // an empty path falls back to OPENROUTER_API_KEY
	}
	only := os.Getenv("FATHOM_DECIDE_LIVE_ONLY")

	var calls []liveCall
	for _, p := range providers {
		if only != "" && only != p.label {
			continue
		}
		var disclose bytes.Buffer
		ports, err := decide.Open(p.cfg, nil, "", nil, decide.WithDisclose(&disclose))
		if err != nil {
			calls = append(calls, liveCall{Provider: p.label, Label: "open", Error: liveRedact(err.Error())})
			t.Logf("%s: open: %v", p.label, liveRedact(err.Error()))
			continue
		}
		pred, choice, score := liveQuestions()
		for _, set := range liveSets {
			asks := []struct {
				label string
				req   decision.Request
			}{
				{set.name + "/predicate", decision.Request{State: set.obligation + "\n\n" + set.diff, Questions: []decision.Question{pred}}},
				{set.name + "/choice", decision.Request{State: set.prompt, Questions: []decision.Question{choice}}},
				{set.name + "/score", decision.Request{State: set.tst, Questions: []decision.Question{score}}},
				{set.name + "/batch3", decision.Request{State: set.obligation + "\n\n" + set.diff + "\n\n" + set.prompt + "\n\n" + set.tst, Questions: []decision.Question{pred, choice, score}}},
			}
			for _, a := range asks {
				ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
				resp, err := ports.Adapter.Decide(ctx, a.req)
				cancel()
				rec.mu.Lock()
				call := rec.last
				rec.last = liveCall{}
				rec.mu.Unlock()
				call.Provider, call.Label = p.label, a.label
				if err != nil {
					call.Error = liveRedact(err.Error())
				} else {
					call.Mapped = &liveMapped{Model: resp.Model, Answers: resp.Answers, Invalid: resp.Invalid, Declined: resp.Declined, Usage: resp.Usage, LatencyMS: resp.Latency.Milliseconds()}
				}
				calls = append(calls, call)
				t.Logf("%s %s: status %d wall %dms err=%q answers=%d invalid=%v declined=%v", p.label, a.label, call.Status, call.WallMS, call.Error, len(resp.Answers), resp.Invalid, resp.Declined)
			}
		}
	}

	rec.mu.Lock()
	hosts := make([]string, 0, len(rec.calls))
	for h := range rec.calls {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	counts := map[string]int{}
	for _, h := range hosts {
		counts[h] = rec.calls[h]
	}
	refused := rec.refused
	rec.mu.Unlock()
	doc := map[string]any{"calls_per_host": counts, "refused": refused, "calls": calls}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "Bearer ") && !strings.Contains(string(data), "Bearer [REDACTED]") {
		t.Fatal("refusing to write a reading that carries a bearer token")
	}
	if err := os.WriteFile(out, data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("calls per host: %v; refused: %v; reading written to %s", counts, refused, out)
}
