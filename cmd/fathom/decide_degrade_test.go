package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// TestDecideDegradesToOneDisclosedLine pins JF-6
// (docs/design/judgment-foundations.md §4, the degrade rule): with provider
// typesafe configured and the site named in [decide] sites, a missing key, a
// timeout, a 429, a 529, the governor at its refuse rung and safe mode each
// write exactly one disclosed line and leave the asking site on its
// deterministic path, for every site in decide.AllSites. The provider is an
// httptest server (no real provider, no key, $0). The port is opened the way
// every entry point opens it (openDecisionPorts, decide.Open) and gated the
// way serve-http gates it (closingDecider), so refuse and safe mode are the
// daemon's own closure, reached inside the kit's fallback to decision.None.
func TestDecideDegradesToOneDisclosedLine(t *testing.T) {
	cases := []struct {
		name   string
		noKey  bool
		slow   bool
		status int
		setup  func(t *testing.T)
		want   string // in the one line
		called bool   // the provider is reached
	}{
		{name: "missing key", noKey: true, want: "decision port disabled"},
		{name: "timeout", slow: true, want: "decision port degraded (timeout)", called: true},
		{name: "429", status: http.StatusTooManyRequests, want: "decision port degraded (rate limited)", called: true},
		{name: "529", status: 529, want: "decision port degraded (overloaded)", called: true},
		{name: "refuse mode", want: "decision port degraded (closed)", setup: func(t *testing.T) {
			tenancy.PublishLevel(tenancy.LevelRefuse)
			t.Cleanup(func() { tenancy.PublishLevel(tenancy.LevelNormal) })
		}},
		{name: "safe mode", want: "decision port degraded (closed)", setup: func(t *testing.T) {
			tenancy.SetSafeMode(&tenancy.SafeModeInfo{Since: time.Now(), Reason: "3 counted exits in 30m0s"})
		}},
	}
	for _, site := range decide.AllSites {
		for _, tc := range cases {
			t.Run(site+"/"+tc.name, func(t *testing.T) {
				resetLifecycleModes(t)
				var calls atomic.Int32
				release := make(chan struct{})
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					if tc.slow {
						select {
						case <-r.Context().Done():
						case <-release:
						}
						return
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(`{"error":"try later"}`))
				}))
				defer srv.Close()
				defer close(release) // runs first: a slow handler never holds Close

				t.Setenv("TYPESAFE_API_KEY", "")
				t.Setenv("JF6_TEST_KEY", "k")
				if tc.noKey {
					t.Setenv("JF6_TEST_KEY", "")
				}
				project := filepath.Join(t.TempDir(), ".fathom", "config.toml")
				if err := os.MkdirAll(filepath.Dir(project), 0o755); err != nil {
					t.Fatal(err)
				}
				toml := "[decide]\nprovider = \"typesafe\"\nendpoint = \"" + srv.URL + "\"\nsites = [\"" + site + "\"]\n"
				if err := os.WriteFile(project, []byte(toml), 0o644); err != nil {
					t.Fatal(err)
				}
				// SEC-1: the key source is the user's, never the repository's.
				user := filepath.Join(t.TempDir(), "user.toml")
				if err := os.WriteFile(user, []byte("[decide]\nprovider = \"typesafe\"\napi_key_env = \"JF6_TEST_KEY\"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				loaded, err := config.Load(user, project)
				if err != nil {
					t.Fatalf("config.Load: %v", err)
				}
				if tc.setup != nil {
					tc.setup(t)
				}

				var stderr bytes.Buffer
				ports := openDecisionPorts(loaded, "", "", "test", &stderr)
				if !decide.SiteAllowed(ports.Cfg, site) {
					t.Fatalf("SiteAllowed(%q) = false with the site named in the repository's [decide] sites", site)
				}
				d := ports.Gated(closingDecider)

				ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
				defer cancel()
				req := decision.Request{State: "state", Questions: []decision.Question{{
					ID: "q", Kind: decision.KindPredicate, Instructions: "The state is a test fixture.",
				}}}
				resp, err := d.Decide(ctx, req)

				var lines []string
				for _, l := range strings.Split(strings.TrimSpace(stderr.String()), "\n") {
					if strings.HasPrefix(l, "warn:") {
						lines = append(lines, l)
					}
				}
				if len(lines) != 1 {
					t.Fatalf("disclosed %d lines, want exactly one:\n%s", len(lines), stderr.String())
				}
				if !strings.Contains(lines[0], tc.want) {
					t.Errorf("the line %q does not say %q", lines[0], tc.want)
				}
				// The deterministic path: nothing answered, and an error every
				// site reads as "no provider" (the same signal provider none
				// gives, decision.ErrNoProvider).
				if err == nil || !errors.Is(err, decision.ErrNoProvider) {
					t.Errorf("Decide error = %v, want one that is decision.ErrNoProvider (the deterministic path)", err)
				}
				if len(resp.Answers) != 0 {
					t.Errorf("Answers = %v, want none", resp.Answers)
				}
				if got := calls.Load() > 0; got != tc.called {
					t.Errorf("provider reached = %v, want %v", got, tc.called)
				}

				// compile_prefilter's deterministic path, end to end: every
				// section kept and the compile not aborted (before JF-6 a 429
				// aborted it).
				if site == decide.SiteCompilePrefilter {
					secs := []semantic.Section{{Path: "AGENTS.md", Subject: "AGENTS.md#a", Text: "Always run the gate."}}
					keep, skipped, _, decErr, perr := semantic.PrefilterSections(ctx, d, 0.5, secs)
					if perr != nil || decErr != nil || len(keep) != 1 || len(skipped) != 0 {
						t.Errorf("PrefilterSections = keep %d skipped %d decErr %v err %v, want every section kept and no error", len(keep), len(skipped), decErr, perr)
					}
				}
			})
		}
	}
}

// TestDecideRefusalDegradesToOneDisclosedLine pins DP-33
// (docs/plans/decision-port.md section H): a provider that refuses a
// question (an OpenAI Decisions "refusal" answer, Response.Declined) is a
// degrade like any other failure: exactly one disclosed line naming the
// refused question, and the asking site on its deterministic path — the
// refusal is never read as an answer, a false or a zero — for every site
// in decide.AllSites. The provider is an httptest server on loopback (no
// real provider, no key, $0), opened the way every entry point opens it.
func TestDecideRefusalDegradesToOneDisclosedLine(t *testing.T) {
	for _, site := range decide.AllSites {
		t.Run(site, func(t *testing.T) {
			resetLifecycleModes(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"model":"gpt-6-luna","answers":[{"type":"predicate","name":"a","probability":0.9},{"type":"refusal","name":"b"}]}`))
			}))
			defer srv.Close()
			t.Setenv("DP31_TEST_KEY", "k")
			project := filepath.Join(t.TempDir(), ".fathom", "config.toml")
			if err := os.MkdirAll(filepath.Dir(project), 0o755); err != nil {
				t.Fatal(err)
			}
			toml := "[decide]\nprovider = \"openai\"\nbase_url = \"" + srv.URL + "\"\nsites = [\"" + site + "\"]\n"
			if err := os.WriteFile(project, []byte(toml), 0o644); err != nil {
				t.Fatal(err)
			}
			// SEC-1: the key source is the user's, never the repository's.
			user := filepath.Join(t.TempDir(), "user.toml")
			if err := os.WriteFile(user, []byte("[decide]\nprovider = \"openai\"\napi_key_env = \"DP31_TEST_KEY\"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			loaded, err := config.Load(user, project)
			if err != nil {
				t.Fatalf("config.Load: %v", err)
			}
			var stderr bytes.Buffer
			ports := openDecisionPorts(loaded, "", "", "test", &stderr)
			if !strings.Contains(stderr.String(), "test: decision port openai (model gpt-6-luna)") {
				t.Fatalf("startup line missing:\n%s", stderr.String())
			}
			d := ports.Gated(closingDecider)
			req := decision.Request{State: "state", Questions: []decision.Question{
				{ID: "a", Kind: decision.KindPredicate, Instructions: "The state is a test fixture."},
				{ID: "b", Kind: decision.KindPredicate, Instructions: "The state is another test fixture."},
			}}
			resp, err := d.Decide(context.Background(), req)
			var lines []string
			for _, l := range strings.Split(strings.TrimSpace(stderr.String()), "\n") {
				if strings.HasPrefix(l, "warn:") {
					lines = append(lines, l)
				}
			}
			if len(lines) != 1 || !strings.Contains(lines[0], "decision port degraded (declined)") || !strings.Contains(lines[0], "b") {
				t.Fatalf("disclosed %q, want exactly one declined line naming b", lines)
			}
			if err == nil || !errors.Is(err, decision.ErrNoProvider) {
				t.Errorf("Decide error = %v, want one that is decision.ErrNoProvider (the deterministic path)", err)
			}
			if len(resp.Answers) != 0 {
				t.Errorf("Answers = %v, want none", resp.Answers)
			}
		})
	}
}
