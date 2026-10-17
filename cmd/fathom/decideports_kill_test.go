package main

// decideports_kill_test.go — KS-2 and KS-7 (docs/plans/decision-port.md
// section I, ADR-073 decision 5, I66): the user's kill closes the port a
// serve-http daemon opened once at startup, between two asks and without a
// restart, and the doctor row under a kill or a refused consent names the
// closing source and calls nothing. The providers here are in-process
// fakes: a loopback httptest server or a counting transport, never a real
// key, host or provider.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// killHome isolates the kill file and the kill variable for one test and
// returns the per-user fathom directory the kill file lives in.
func killHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("FATHOM_HOME", home)
	t.Setenv(decide.KillEnv, "")
	if err := os.Unsetenv(decide.KillEnv); err != nil {
		t.Fatal(err)
	}
	return home
}

func ksPredicate() decision.Request {
	return decision.Request{State: "diff: a.go", Questions: []decision.Question{{ID: "q1", Kind: decision.KindPredicate, Instructions: "The rule governs the edited file."}}}
}

// writeUserConfig writes body as the user config and moves its mtime
// forward by step, so a reload is never hidden by a coarse clock.
func writeKSUserConfig(t *testing.T, path, body string, step time.Duration) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(step)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}

func TestDaemonOpenedPortClosesBetweenAsksWithoutRestart(t *testing.T) {
	home := killHome(t)
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"jev-latest","answers":{"q1":{"type":"noul","noul":0.7}}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("FATHOM_TEST_KS2_KEY", "fake-key-not-a-real-one")
	userPath := filepath.Join(home, "config.toml")
	userBody := "[decide]\nprovider = \"typesafe\"\nbase_url = \"" + srv.URL + "\"\napi_key_env = \"FATHOM_TEST_KS2_KEY\"\n"
	writeKSUserConfig(t, userPath, userBody, 0)
	loaded, err := config.Load(userPath, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// As wirePorts does: open once at startup, then gate the composed port.
	var stderr bytes.Buffer
	ports := openDecisionPorts(loaded, "", "", "serve-http", &stderr)
	route := ports.Gated(closingDecider)
	client := ports.Gated(func(d decision.Decider) decision.Decider { return gateDecider(d, "client") })

	ask := func(d decision.Decider) error {
		_, err := d.Decide(context.Background(), ksPredicate())
		return err
	}
	if err := ask(route); err != nil || hits.Load() != 1 {
		t.Fatalf("first ask: err %v hits %d, want an answer over one request", err, hits.Load())
	}

	killFile := filepath.Join(home, decide.KillFileName)
	if err := os.WriteFile(killFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for i, d := range []decision.Decider{route, client, route} {
		err := ask(d)
		var ke *decide.KilledError
		if !errors.As(err, &ke) || ke.Gate.Source != decide.KillSourceFile || !errors.Is(err, decision.ErrNoProvider) {
			t.Fatalf("ask %d under the kill file: err %v, want a *KilledError (kill_source file) that is ErrNoProvider", i, err)
		}
	}
	if hits.Load() != 1 {
		t.Errorf("the killed port sent %d more requests", hits.Load()-1)
	}
	if n := strings.Count(stderr.String(), "killed"); n != 1 {
		t.Errorf("stderr has %d killed lines, want one:\n%s", n, stderr.String())
	}

	if err := os.Remove(killFile); err != nil {
		t.Fatal(err)
	}
	if err := ask(client); err != nil || hits.Load() != 2 {
		t.Fatalf("after the kill file went: err %v hits %d, want the port open again with no restart", err, hits.Load())
	}

	writeKSUserConfig(t, userPath, userBody+"enabled = false\n", time.Second)
	err = ask(route)
	var ke *decide.KilledError
	if !errors.As(err, &ke) || ke.Gate.Source != decide.KillSourceEnabled || hits.Load() != 2 {
		t.Fatalf("after enabled = false: err %v hits %d, want closed by enabled with no request", err, hits.Load())
	}

	writeKSUserConfig(t, userPath, userBody, 2*time.Second)
	if err := ask(route); err != nil || hits.Load() != 3 {
		t.Fatalf("after enabled came back: err %v hits %d, want the port open again", err, hits.Load())
	}
}

// doctorKeyCounter counts every key touch the doctor row's seams see.
type doctorKeyCounter struct {
	mu sync.Mutex
	n  int
}

func (c *doctorKeyCounter) bump() {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
}

func (c *doctorKeyCounter) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func (c *doctorKeyCounter) io() config.KeyIO {
	return config.KeyIO{
		LookupEnv: func(string) (string, bool) { c.bump(); return "fake-key-not-a-real-one", true },
		Stat:      func(p string) (os.FileInfo, error) { c.bump(); return os.Stat(p) },
		RunCmd:    func(context.Context, []string) (string, error) { c.bump(); return "fake-key-not-a-real-one", nil },
	}
}

type doctorDialCounter struct{ n atomic.Int64 }

func (d *doctorDialCounter) RoundTrip(r *http.Request) (*http.Response, error) {
	d.n.Add(1)
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{"model":"jev-latest","answers":{"ping":{"type":"noul","noul":0.5}}}`)), Request: r}, nil
}

func TestDoctorDecisionProviderUnderKillDialsNothing(t *testing.T) {
	home := killHome(t)
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	projectPath := filepath.Join(dir, "repo", ".fathom", "config.toml")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		t.Fatal(err)
	}
	load := func(t *testing.T, user, project string) config.Loaded {
		t.Helper()
		_ = os.Remove(userPath)
		_ = os.Remove(projectPath)
		if user != "" {
			if err := os.WriteFile(userPath, []byte(user), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if project != "" {
			if err := os.WriteFile(projectPath, []byte(project), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		loaded, err := config.Load(userPath, projectPath)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		return loaded
	}
	userTypeSafe := "[decide]\nprovider = \"typesafe\"\napi_key_cmd = [\"fake-credential-helper\"]\n"
	builds := 0
	build := func(config.Profile) (llmclient.LLMClient, error) { builds++; return llmclient.NewMockClient(), nil }

	verdict := func(t *testing.T, loaded config.Loaded) (doctorCheck, int, int64) {
		t.Helper()
		keys, dials := &doctorKeyCounter{}, &doctorDialCounter{}
		got := decisionProviderVerdict(loaded, build, "", decide.WithKeyIO(keys.io()), decide.WithHTTPClient(&http.Client{Transport: dials}))
		return got, keys.count(), dials.n.Load()
	}

	t.Run("the control, unkilled, reads the key and pings", func(t *testing.T) {
		got, keys, dials := verdict(t, load(t, userTypeSafe, ""))
		if keys == 0 || dials == 0 {
			t.Fatalf("got %+v keys %d dials %d, want the key touched and a ping sent (the seams are on the path)", got, keys, dials)
		}
	})

	for _, tc := range []struct {
		name, user, project, source, layer string
		setup                              func(t *testing.T)
	}{
		{"FATHOM_DECIDE=off", userTypeSafe, "", decide.KillSourceEnv, "environment", func(t *testing.T) { t.Setenv(decide.KillEnv, "off") }},
		{"the kill file", userTypeSafe, "", decide.KillSourceFile, "file", func(t *testing.T) {
			f := filepath.Join(home, decide.KillFileName)
			if err := os.WriteFile(f, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Remove(f) })
		}},
		{"enabled = false", userTypeSafe + "enabled = false\n", "", decide.KillSourceEnabled, "user", func(*testing.T) {}},
		{"a provider only the repository names", "", "[decide]\nprovider = \"typesafe\"\n", decide.KillSourceConsent, "project", func(*testing.T) {}},
		{"an llm provider only the repository names", "", "[decide]\nprovider = \"llm\"\n", decide.KillSourceConsent, "project", func(*testing.T) {}},
	} {
		t.Run(tc.name+" names the source and calls nothing", func(t *testing.T) {
			tc.setup(t)
			builds = 0
			got, keys, dials := verdict(t, load(t, tc.user, tc.project))
			if got.Status != doctorSkipped {
				t.Errorf("Status = %v, want skipped (an honestly closed port): %+v", got.Status, got)
			}
			for _, want := range []string{"kill_source " + tc.source, tc.layer} {
				if !strings.Contains(got.Reason, want) {
					t.Errorf("Reason = %q, want it to name %q", got.Reason, want)
				}
			}
			if keys != 0 || dials != 0 || builds != 0 {
				t.Errorf("a closed port's doctor row touched keys %d times, dialed %d, built %d lanes", keys, dials, builds)
			}
		})
	}

	t.Run("FATHOM_DECIDE=off says a running daemon stays open", func(t *testing.T) {
		t.Setenv(decide.KillEnv, "off")
		got, _, _ := verdict(t, load(t, userTypeSafe, ""))
		if !strings.Contains(got.Reason, "daemon") || !strings.Contains(got.Reason, decide.KillFileName) {
			t.Errorf("Reason = %q, want it to say a serve-http daemon started without it stays open and how to close it", got.Reason)
		}
	})

	t.Run("another FATHOM_DECIDE value is ignored and named", func(t *testing.T) {
		t.Setenv(decide.KillEnv, "0")
		got, _, _ := verdict(t, load(t, "", ""))
		if !strings.Contains(got.Reason, `FATHOM_DECIDE="0" ignored`) {
			t.Errorf("Reason = %q, want the ignored value named", got.Reason)
		}
	})

	t.Run("with no [decide] block the row is unchanged (DP-35)", func(t *testing.T) {
		loaded, err := config.Load("", "")
		if err != nil {
			t.Fatal(err)
		}
		got := decisionProviderVerdict(loaded, nil, "")
		if got.Status != doctorSkipped || got.Reason != "off ([decide] provider = none)" || got.Remedy != "" {
			t.Errorf("got %+v, want the DP-35 row", got)
		}
	})
}
