package decide_test

// killswitch_test.go — KS-1, KS-4, KS-5 and KS-6 (docs/plans/decision-port.md
// section I, ADR-073 decision 5, I66): the user's kill and consent close the
// decision port before any key read, exec or dial. Every key read, exec and
// dial goes through an injected seam (decide.WithKeyIO, WithHTTPClient, the
// llm lane) whose fake counts calls: a closed port leaves every count at
// zero, and an open one (the control) moves them, so the seams are on the
// path. No test here holds a real key, opens a socket or names a provider's
// host.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

const (
	ksNoulReply      = `{"model":"jev-latest","answers":{"q1":{"type":"noul","noul":0.7}}}`
	ksPredicateReply = `{"model":"gpt-6-luna","answers":[{"type":"predicate","name":"q1","probability":0.8}]}`
)

// keyCounter is the fake config.KeyIO: every variable lookup, key-file stat
// and api_key_cmd exec is counted; env is what the fake environment holds.
type keyCounter struct {
	mu      sync.Mutex
	env     map[string]string
	lookups map[string]int
	stats   int
	execs   int
}

func newKeyCounter(env map[string]string) *keyCounter {
	return &keyCounter{env: env, lookups: map[string]int{}}
}

func (k *keyCounter) io() config.KeyIO {
	return config.KeyIO{
		LookupEnv: func(name string) (string, bool) {
			k.mu.Lock()
			defer k.mu.Unlock()
			k.lookups[name]++
			v, ok := k.env[name]
			return v, ok
		},
		Stat: func(path string) (os.FileInfo, error) {
			k.mu.Lock()
			k.stats++
			k.mu.Unlock()
			return os.Stat(path)
		},
		RunCmd: func(context.Context, []string) (string, error) {
			k.mu.Lock()
			defer k.mu.Unlock()
			k.execs++
			return testKey, nil
		},
	}
}

func (k *keyCounter) total() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	n := k.stats + k.execs
	for _, c := range k.lookups {
		n += c
	}
	return n
}

func (k *keyCounter) lookupsOf(name string) int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.lookups[name]
}

// dialCounter is the fake HTTP transport: it counts every request and
// answers reply without opening a socket.
type dialCounter struct {
	mu    sync.Mutex
	n     int
	reply string
}

func (d *dialCounter) RoundTrip(r *http.Request) (*http.Response, error) {
	d.mu.Lock()
	d.n++
	d.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(d.reply)),
		Request:    r,
	}, nil
}

func (d *dialCounter) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.n
}

func (d *dialCounter) client() *http.Client { return &http.Client{Transport: d} }

// isolateKill points the kill file at a fresh FATHOM_HOME and clears the
// kill variable, whatever the machine running the test exports.
func isolateKill(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("FATHOM_HOME", home)
	t.Setenv(decide.KillEnv, "")
	if err := os.Unsetenv(decide.KillEnv); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestKillSwitchNoKeyReadNoExecNoDial(t *testing.T) {
	unsetProviderKeys(t)
	isolateKill(t)
	keyFile := writeKeyFile(t, 0o600)
	t.Setenv("FATHOM_TEST_KS1_KEY", testKey)
	t.Setenv("OPENROUTER_API_KEY", testKey)
	fakeEnv := map[string]string{"FATHOM_TEST_KS1_KEY": testKey, "OPENROUTER_API_KEY": testKey}

	for _, tc := range []struct {
		name  string
		cfg   config.Decide
		reply string
	}{
		{"typesafe with api_key_env", config.Decide{Provider: "typesafe", APIKeyEnv: "FATHOM_TEST_KS1_KEY"}, ksNoulReply},
		{"typesafe over openrouter with the well-known variable", config.Decide{Provider: "typesafe", Transport: "openrouter"}, ksNoulReply},
		{"openai with api_key_file", config.Decide{Provider: "openai", APIKeyFile: keyFile}, ksPredicateReply},
		{"openai with api_key_cmd", config.Decide{Provider: "openai", APIKeyCmd: []string{"fake-credential-helper"}}, ksPredicateReply},
	} {
		open := func(t *testing.T) (*decide.Ports, *keyCounter, *dialCounter, *bytes.Buffer) {
			t.Helper()
			keys, dials := newKeyCounter(fakeEnv), &dialCounter{reply: tc.reply}
			var disclose bytes.Buffer
			ports, err := decide.Open(tc.cfg, nil, "", nil, decide.WithKeyIO(keys.io()), decide.WithHTTPClient(dials.client()), decide.WithDisclose(&disclose))
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			return ports, keys, dials, &disclose
		}

		t.Run(tc.name+": FATHOM_DECIDE=off reads no key, execs nothing, dials nothing", func(t *testing.T) {
			t.Setenv(decide.KillEnv, "off")
			ports, keys, dials, disclose := open(t)
			if _, ok := ports.Decider.(decision.None); !ok || ports.Adapter != nil {
				t.Fatalf("Decider = %T Adapter = %T, want decision.None and no adapter", ports.Decider, ports.Adapter)
			}
			if !ports.Gate.Closed || ports.Gate.Source != decide.KillSourceEnv {
				t.Errorf("Gate = %+v, want closed with kill_source env", ports.Gate)
			}
			if _, err := ports.Decider.Decide(context.Background(), predicateRequest("q1")); !errors.Is(err, decision.ErrNoProvider) {
				t.Errorf("Decide err = %v, want decision.ErrNoProvider (the site's deterministic path)", err)
			}
			if n := keys.total(); n != 0 {
				t.Errorf("a killed port touched a key source %d times (lookups %v)", n, keys.lookups)
			}
			if n := dials.count(); n != 0 {
				t.Errorf("a killed port dialed %d times", n)
			}
			if disclose.Len() != 0 {
				t.Errorf("Open disclosed %q; the startup line is the caller's", disclose.String())
			}
		})

		t.Run(tc.name+": the control, unkilled, reads the key and dials", func(t *testing.T) {
			ports, keys, dials, _ := open(t)
			if ports.Gate.Closed {
				t.Fatalf("Gate = %+v, want open", ports.Gate)
			}
			if _, err := ports.Decider.Decide(context.Background(), predicateRequest("q1")); err != nil {
				t.Fatalf("Decide: %v", err)
			}
			if keys.total() == 0 || dials.count() == 0 {
				t.Errorf("the seams are off the path: key touches %d, dials %d", keys.total(), dials.count())
			}
		})
	}

	t.Run("provider llm: the lane is never built under the kill", func(t *testing.T) {
		t.Setenv(decide.KillEnv, "off")
		lanes := 0
		lane := func(string) (llmclient.LLMClient, error) { lanes++; return llmclient.NewMockClient(), nil }
		ports, err := decide.Open(config.Decide{Provider: "llm"}, lane, "m", nil)
		if err != nil {
			t.Fatal(err)
		}
		if lanes != 0 || ports.Gate.Source != decide.KillSourceEnv {
			t.Errorf("lane calls %d, Gate %+v; want 0 and kill_source env", lanes, ports.Gate)
		}
		t.Setenv(decide.KillEnv, "")
		if _, err := decide.Open(config.Decide{Provider: "llm"}, lane, "m", nil); err != nil || lanes != 1 {
			t.Errorf("control: lane calls %d err %v, want 1 and nil", lanes, err)
		}
	})

	t.Run("only the exact value off kills; another is ignored and named", func(t *testing.T) {
		for _, v := range []string{"0", "OFF", "false", " off"} {
			t.Setenv(decide.KillEnv, v)
			dials := &dialCounter{reply: ksNoulReply}
			ports, err := decide.Open(config.Decide{Provider: "typesafe", APIKeyEnv: "FATHOM_TEST_KS1_KEY"}, nil, "", nil, decide.WithHTTPClient(dials.client()))
			if err != nil {
				t.Fatal(err)
			}
			if ports.Gate.Closed || ports.Gate.IgnoredEnv != v {
				t.Errorf("FATHOM_DECIDE=%q: Gate = %+v, want open with the value named as ignored", v, ports.Gate)
			}
		}
	})

	t.Run("the unconfigured port keeps its shape and says nothing (I42)", func(t *testing.T) {
		for _, kill := range []string{"", "off"} {
			t.Setenv(decide.KillEnv, kill)
			var disclose bytes.Buffer
			ports, err := decide.Open(config.DefaultDecide, nil, "", nil, decide.WithDisclose(&disclose))
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := ports.Decider.(decision.None); !ok || ports.Adapter != nil || disclose.Len() != 0 {
				t.Errorf("FATHOM_DECIDE=%q: Decider %T Adapter %T disclose %q, want None, no adapter, silence", kill, ports.Decider, ports.Adapter, disclose.String())
			}
		}
	})
}

// writeLayers writes the user and project config files (either body may be
// "") under a fresh directory and returns their paths.
func writeLayers(t *testing.T, user, project string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	userPath := filepath.Join(dir, "home", ".fathom", "config.toml")
	projectPath := filepath.Join(dir, "repo", ".fathom", "config.toml")
	for path, body := range map[string]string{userPath: user, projectPath: project} {
		if body == "" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return userPath, projectPath
}

func loadDecide(t *testing.T, user, project string) config.Decide {
	t.Helper()
	u, p := writeLayers(t, user, project)
	loaded, err := config.Load(u, p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return loaded.Config.Decide
}

func TestRepoProviderWithoutUserConsentNeverReadsAmbientKey(t *testing.T) {
	unsetProviderKeys(t)
	isolateKill(t)
	t.Setenv("OPENROUTER_API_KEY", testKey)
	fakeEnv := map[string]string{"OPENROUTER_API_KEY": testKey, "FATHOM_TEST_KS4_KEY": testKey}
	t.Setenv("FATHOM_TEST_KS4_KEY", testKey)
	repo := "[decide]\nprovider = \"typesafe\"\ntransport = \"openrouter\"\nsites = [\"compile_prefilter\"]\n"

	open := func(t *testing.T, cfg config.Decide) (*decide.Ports, error, *keyCounter, *dialCounter) {
		t.Helper()
		keys, dials := newKeyCounter(fakeEnv), &dialCounter{reply: ksNoulReply}
		ports, err := decide.Open(cfg, nil, "", nil, decide.WithKeyIO(keys.io()), decide.WithHTTPClient(dials.client()))
		return ports, err, keys, dials
	}

	t.Run("a provider only the repository names stays closed and reads nothing", func(t *testing.T) {
		ports, err, keys, dials := open(t, loadDecide(t, "", repo))
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if _, ok := ports.Decider.(decision.None); !ok {
			t.Fatalf("Decider = %T, want decision.None", ports.Decider)
		}
		if !ports.Gate.Closed || ports.Gate.Source != decide.KillSourceConsent || ports.Gate.Layer != "project" {
			t.Errorf("Gate = %+v, want closed by consent at the project layer", ports.Gate)
		}
		if !strings.Contains(ports.Gate.Reason, "consent: provider named by the repository, not allowed by the user") {
			t.Errorf("Gate.Reason = %q, want the consent refusal named", ports.Gate.Reason)
		}
		if keys.total() != 0 || dials.count() != 0 {
			t.Errorf("a refused port touched keys %d times (%v) and dialed %d", keys.total(), keys.lookups, dials.count())
		}
	})

	t.Run("the repository switching the user's transport is the repository's choice alone", func(t *testing.T) {
		cfg := loadDecide(t, "[decide]\nprovider = \"typesafe\"\napi_key_env = \"FATHOM_TEST_KS4_KEY\"\n", "[decide]\ntransport = \"openrouter\"\n")
		ports, err, keys, dials := open(t, cfg)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if ports.Gate.Source != decide.KillSourceConsent || keys.total() != 0 || dials.count() != 0 {
			t.Errorf("Gate = %+v keys %d dials %d, want refused by consent with nothing touched", ports.Gate, keys.total(), dials.count())
		}
	})

	t.Run("allow_repo_provider opens it, but the well-known variable stays unread", func(t *testing.T) {
		cfg := loadDecide(t, "[decide]\nallow_repo_provider = true\n", repo)
		_, err, keys, dials := open(t, cfg)
		if err == nil || !strings.Contains(err.Error(), "OPENROUTER_API_KEY") {
			t.Fatalf("Open err = %v, want the ambient variable refused by name", err)
		}
		if n := keys.lookupsOf("OPENROUTER_API_KEY"); n != 0 {
			t.Errorf("OPENROUTER_API_KEY read %d times; a transport's well-known variable counts only when the user layer named the provider", n)
		}
		if dials.count() != 0 {
			t.Errorf("dialed %d times", dials.count())
		}
	})

	t.Run("a key the user named for no provider still never follows the repository's (DP-39)", func(t *testing.T) {
		cfg := loadDecide(t, "[decide]\nallow_repo_provider = true\napi_key_env = \"FATHOM_TEST_KS4_KEY\"\n", repo)
		_, err, keys, dials := open(t, cfg)
		if err == nil || !strings.Contains(err.Error(), "named for") {
			t.Fatalf("Open err = %v, want the key-transport refusal", err)
		}
		if keys.total() != 0 || dials.count() != 0 {
			t.Errorf("keys %d dials %d, want nothing touched", keys.total(), dials.count())
		}
	})

	t.Run("allow_repo_provider opens a repository's llm provider, consent_layer project", func(t *testing.T) {
		lanes := 0
		lane := func(string) (llmclient.LLMClient, error) { lanes++; return llmclient.NewMockClient(), nil }
		refused, err := decide.Open(loadDecide(t, "", "[decide]\nprovider = \"llm\"\n"), lane, "m", nil)
		if err != nil || refused.Gate.Source != decide.KillSourceConsent || lanes != 0 {
			t.Fatalf("without consent: err %v Gate %+v lanes %d, want refused by consent with no lane built", err, refused.Gate, lanes)
		}
		ports, err := decide.Open(loadDecide(t, "[decide]\nallow_repo_provider = true\n", "[decide]\nprovider = \"llm\"\n"), lane, "m", nil)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if ports.Gate.Closed || ports.Gate.ConsentLayer != "project" || lanes != 1 {
			t.Errorf("Gate = %+v lanes %d, want open with consent_layer project", ports.Gate, lanes)
		}
	})

	// O1133: allow_repo_provider opens every provider the consent rows
	// name, not only llm. A remote provider's key is user-only and stays
	// with the transport it was named for (DP-39), so the user binds it by
	// naming the transport beside it; the repository's provider speaking
	// that transport then opens with consent_layer project, and no
	// transport's well-known variable is read. Without allow_repo_provider
	// the same layers stay closed and touch nothing (the KS-4 guard).
	for _, tc := range []struct{ name, user, project, reply string }{
		{"typesafe over openrouter", "transport = \"openrouter\"\napi_key_env = \"FATHOM_TEST_KS4_KEY\"\n", repo, ksNoulReply},
		{"typesafe", "transport = \"typesafe\"\napi_key_env = \"FATHOM_TEST_KS4_KEY\"\n", "[decide]\nprovider = \"typesafe\"\n", ksNoulReply},
		{"openai", "transport = \"openai\"\napi_key_env = \"FATHOM_TEST_KS4_KEY\"\n", "[decide]\nprovider = \"openai\"\n", ksPredicateReply},
	} {
		t.Run("allow_repo_provider opens a repository's "+tc.name+" provider with the key the user named for its transport", func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			openWith := func(cfg config.Decide, rec *decide.Recorder) (*decide.Ports, error, *keyCounter, *dialCounter) {
				keys, dials := newKeyCounter(fakeEnv), &dialCounter{reply: tc.reply}
				ports, err := decide.Open(cfg, nil, "", rec, decide.WithKeyIO(keys.io()), decide.WithHTTPClient(dials.client()), decide.WithDisclose(io.Discard))
				return ports, err, keys, dials
			}
			refused, err, keys, dials := openWith(loadDecide(t, "[decide]\n"+tc.user, tc.project), nil)
			if err != nil || refused.Gate.Source != decide.KillSourceConsent || keys.total() != 0 || dials.count() != 0 {
				t.Fatalf("without allow_repo_provider: err %v Gate %+v keys %d dials %d, want refused by consent with nothing touched", err, refused.Gate, keys.total(), dials.count())
			}

			var log bytes.Buffer
			rec := decide.NewRecorder(&log, func() time.Time { return time.Unix(0, 0).UTC() }, "test")
			ports, err, keys, dials := openWith(loadDecide(t, "[decide]\nallow_repo_provider = true\n"+tc.user, tc.project), rec)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			if ports.Gate.Closed || ports.Gate.ConsentLayer != "project" {
				t.Fatalf("Gate = %+v, want open with consent_layer project", ports.Gate)
			}
			resp, err := ports.Decider.Decide(context.Background(), predicateRequest("q1"))
			if err != nil {
				t.Fatalf("Decide: %v", err)
			}
			rec.Record(decide.SiteCompilePrefilter, resp, func(decision.Answer) bool { return true })
			var row map[string]any
			if err := json.Unmarshal(bytes.TrimSpace(log.Bytes()), &row); err != nil || row["consent_layer"] != "project" {
				t.Errorf("decisions.jsonl row = %s (%v), want consent_layer project", log.String(), err)
			}
			if dials.count() != 1 || keys.lookupsOf("FATHOM_TEST_KS4_KEY") == 0 {
				t.Errorf("dials %d, user key lookups %d, want one request carrying the key the user named", dials.count(), keys.lookupsOf("FATHOM_TEST_KS4_KEY"))
			}
			for _, v := range []string{"OPENROUTER_API_KEY", "TYPESAFE_API_KEY", "OPENAI_API_KEY"} {
				if n := keys.lookupsOf(v); n != 0 {
					t.Errorf("%s read %d times; a repository's provider never reads a well-known variable", v, n)
				}
			}
		})
	}

	t.Run("a key the user named for one transport never follows the repository's move to another", func(t *testing.T) {
		cfg := loadDecide(t, "[decide]\nallow_repo_provider = true\ntransport = \"typesafe\"\napi_key_env = \"FATHOM_TEST_KS4_KEY\"\n", repo)
		_, err, keys, dials := open(t, cfg)
		if err == nil || !strings.Contains(err.Error(), "named for") {
			t.Fatalf("Open err = %v, want the key-transport refusal", err)
		}
		if keys.total() != 0 || dials.count() != 0 {
			t.Errorf("keys %d dials %d, want nothing touched", keys.total(), dials.count())
		}
	})

	t.Run("the user naming the provider is consent, and the well-known variable counts", func(t *testing.T) {
		cfg := loadDecide(t, "[decide]\nprovider = \"typesafe\"\ntransport = \"openrouter\"\n", repo)
		ports, err, keys, _ := open(t, cfg)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if ports.Gate.Closed || ports.Gate.ConsentLayer != "user" {
			t.Errorf("Gate = %+v, want open with consent_layer user", ports.Gate)
		}
		if keys.lookupsOf("OPENROUTER_API_KEY") == 0 {
			t.Error("the user's provider did not read its transport's well-known variable")
		}
	})
}

func TestSessionSitesOnlyFromUserScopeAndDenyWins(t *testing.T) {
	isolateKill(t)
	if !decide.IsSessionSite(decide.SiteReadoutMode) || !decide.IsSessionSite(decide.SiteCompactAdvice) || decide.IsSessionSite(decide.SiteAsk) {
		t.Fatalf("IsSessionSite: readout_mode and compact_advice are the session sites, ask is not")
	}

	t.Run("a session site named in any layer's sites is refused", func(t *testing.T) {
		cfg := loadDecide(t,
			"[decide]\nprovider = \"typesafe\"\nsites = [\"readout_mode\", \"compact_advice\", \"compile_prefilter\"]\n",
			"[decide]\nsites = [\"readout_mode\", \"compact_advice\", \"compile_prefilter\", \"ask\"]\n")
		for _, s := range []string{decide.SiteReadoutMode, decide.SiteCompactAdvice} {
			if decide.SiteAllowed(cfg, s) {
				t.Errorf("SiteAllowed(%s) = true from [decide] sites; only session_sites opts a session site in", s)
			}
		}
		if !decide.SiteAllowed(cfg, decide.SiteAsk) || !decide.SiteAllowed(cfg, decide.SiteCompilePrefilter) {
			t.Error("the ordinary and opt-in sites the repository named lost their JF-4 rule")
		}
	})

	t.Run("the user's session_sites opts a session site in; the repository cannot", func(t *testing.T) {
		user := "[decide]\nprovider = \"typesafe\"\nsession_sites = [\"readout_mode\"]\n"
		cfg := loadDecide(t, user, "[decide]\nsession_sites = [\"compact_advice\"]\n")
		if !decide.SiteAllowed(cfg, decide.SiteReadoutMode) {
			t.Error("SiteAllowed(readout_mode) = false with the user's session_sites naming it")
		}
		if decide.SiteAllowed(cfg, decide.SiteCompactAdvice) {
			t.Error("SiteAllowed(compact_advice) = true from a repository's session_sites")
		}
	})

	t.Run("deny_sites refuses whatever any list says", func(t *testing.T) {
		cfg := loadDecide(t,
			"[decide]\nprovider = \"typesafe\"\nsession_sites = [\"readout_mode\"]\ndeny_sites = [\"readout_mode\", \"ask\", \"compile_prefilter\"]\n",
			"[decide]\nsites = [\"ask\", \"compile_prefilter\", \"obligations_rerank\"]\n")
		for _, s := range []string{decide.SiteReadoutMode, decide.SiteAsk, decide.SiteCompilePrefilter} {
			if decide.SiteAllowed(cfg, s) {
				t.Errorf("SiteAllowed(%s) = true, want deny_sites to refuse it", s)
			}
			if g := decide.Check(cfg, s); g.Source != decide.KillSourceDeny || g.Layer != "user" {
				t.Errorf("Check(%s) = %+v, want closed by deny_sites at the user layer", s, g)
			}
		}
		if !decide.SiteAllowed(cfg, decide.SiteObligationsRerank) {
			t.Error("a site deny_sites does not name was refused")
		}
	})
}

func TestPortGatePrecedenceNamesTheFirstClosingSource(t *testing.T) {
	unsetProviderKeys(t)
	home := isolateKill(t)
	killFile := filepath.Join(home, decide.KillFileName)

	// A repository-only provider and a user layer that also disables and
	// denies: every port-level source closes at once.
	cfg := loadDecide(t, "[decide]\nenabled = false\ndeny_sites = [\"compile_prefilter\"]\n",
		"[decide]\nprovider = \"typesafe\"\nsites = [\"compile_prefilter\"]\n")
	if err := os.WriteFile(killFile, []byte("anything"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Stat-ed, never read: a kill file nobody may read still kills.
	if err := os.Chmod(killFile, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Setenv(decide.KillEnv, "off")

	steps := []struct {
		source, layer string
		peel          func()
	}{
		{decide.KillSourceEnv, "environment", func() { t.Setenv(decide.KillEnv, "") }},
		{decide.KillSourceFile, "file", func() { _ = os.Chmod(killFile, 0o600); _ = os.Remove(killFile) }},
		{decide.KillSourceEnabled, "user", func() { cfg.Disabled = false }},
		{decide.KillSourceConsent, "project", func() { cfg.AllowRepoProvider = true }},
		{decide.KillSourceDeny, "user", func() { cfg.DenySites = nil }},
	}
	for _, st := range steps {
		g := decide.Check(cfg, decide.SiteCompilePrefilter)
		if !g.Closed || g.Source != st.source || g.Layer != st.layer || g.Reason == "" {
			t.Fatalf("Check = %+v, want closed by %s at %s, named", g, st.source, st.layer)
		}
		st.peel()
	}
	if g := decide.Check(cfg, decide.SiteCompilePrefilter); g.Closed {
		t.Fatalf("every source peeled, Check = %+v, want open", g)
	}
	// KS-12: the site's miss kill comes after deny_sites and before sites.
	killedRec := decide.NewRecorder(nil, time.Now, "test")
	killedRec.Seed(strings.NewReader(logRows(decide.SiteCompilePrefilter, 89, 11)))
	if g := decide.CheckSite(cfg, decide.SiteCompilePrefilter, killedRec); !g.Closed || g.Source != decide.KillSourceMisses || g.Layer != "log" || g.Reason == "" {
		t.Errorf("CheckSite(killed site) = %+v, want closed by misses at log, named", g)
	}
	denied := cfg
	denied.DenySites = []string{decide.SiteCompilePrefilter}
	if g := decide.CheckSite(denied, decide.SiteCompilePrefilter, killedRec); g.Source != decide.KillSourceDeny {
		t.Errorf("CheckSite(denied and killed) = %+v, want deny_sites first", g)
	}
	unlisted := cfg
	unlisted.Sites = nil
	if g := decide.CheckSite(unlisted, decide.SiteCompilePrefilter, killedRec); g.Source != decide.KillSourceMisses {
		t.Errorf("CheckSite(killed and not in sites) = %+v, want the miss kill before sites", g)
	}
	if g := decide.Check(cfg, decide.SiteAsk); !g.Closed || g.Source != decide.KillSourceSites {
		t.Errorf("Check(ask) = %+v, want closed by sites (an opt-in site the repository did not name)", g)
	}
	if g := decide.Check(cfg, decide.SiteReadoutMode); !g.Closed || g.Source != decide.KillSourceSessionSites {
		t.Errorf("Check(readout_mode) = %+v, want closed by session_sites", g)
	}

	t.Run("decisions.jsonl rows carry consent_layer and kill_source", func(t *testing.T) {
		userCfg := loadDecide(t, "[decide]\nprovider = \"typesafe\"\napi_key_env = \"FATHOM_TEST_KS6_KEY\"\n", "")
		t.Setenv("FATHOM_TEST_KS6_KEY", testKey)
		var log bytes.Buffer
		rec := decide.NewRecorder(&log, func() time.Time { return time.Unix(0, 0).UTC() }, "test")
		dials := &dialCounter{reply: ksNoulReply}
		var disclose bytes.Buffer
		ports, err := decide.Open(userCfg, nil, "", rec, decide.WithHTTPClient(dials.client()), decide.WithDisclose(&disclose))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := ports.Decider.Decide(context.Background(), predicateRequest("q1"))
		if err != nil {
			t.Fatal(err)
		}
		rec.Record(decide.SiteCompilePrefilter, resp, func(decision.Answer) bool { return true })
		if err := os.WriteFile(killFile, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		_, err = ports.Decider.Decide(context.Background(), predicateRequest("q1"))
		var ke *decide.KilledError
		if !errors.As(err, &ke) || !errors.Is(err, decision.ErrNoProvider) {
			t.Fatalf("Decide under the kill file err = %v, want a *KilledError that is ErrNoProvider", err)
		}
		rec.RecordError(decide.SiteCompilePrefilter, ports.Decider.Name(), "", 1, 0, err)
		lines := strings.Split(strings.TrimSpace(log.String()), "\n")
		if len(lines) != 2 {
			t.Fatalf("decisions.jsonl = %q, want two rows", log.String())
		}
		var answered, killed map[string]any
		_ = json.Unmarshal([]byte(lines[0]), &answered)
		_ = json.Unmarshal([]byte(lines[1]), &killed)
		if answered["consent_layer"] != "user" || answered["kill_source"] != nil {
			t.Errorf("answered row = %v, want consent_layer user and no kill_source", answered)
		}
		if killed["kill_source"] != decide.KillSourceFile {
			t.Errorf("killed row = %v, want kill_source file", killed)
		}
		if dials.count() != 1 {
			t.Errorf("dials = %d, want 1 (the killed ask dials nothing)", dials.count())
		}
	})
}

// TestPortKilledAtOpenReopensOnTheNextAskAfterTheKillLifts is O1132: a port
// a live kill (the kill file, the user's enabled = false) closed at Open,
// as a serve-http daemon started while killed opens it, re-checks the kill
// before each ask like any open port and opens on the first ask after the
// kill lifts, with no restart and no background job. While killed it reads
// no key, execs nothing, builds no lane and dials nothing; the kill coming
// back closes it again. The environment and a refused consent are settled
// at Open and still never reopen.
func TestPortKilledAtOpenReopensOnTheNextAskAfterTheKillLifts(t *testing.T) {
	unsetProviderKeys(t)
	t.Setenv("HOME", t.TempDir())
	ask := func(d decision.Decider) error {
		_, err := d.Decide(context.Background(), predicateRequest("q1"))
		return err
	}
	killed := func(t *testing.T, err error, source string) {
		t.Helper()
		var ke *decide.KilledError
		if !errors.As(err, &ke) || ke.Gate.Source != source || !errors.Is(err, decision.ErrNoProvider) {
			t.Fatalf("Decide err = %v, want a *KilledError (kill_source %s) that is ErrNoProvider", err, source)
		}
	}
	fakeEnv := map[string]string{"FATHOM_TEST_KS2_KEY": testKey}
	t.Setenv("FATHOM_TEST_KS2_KEY", testKey)

	for _, tc := range []struct {
		name  string
		cfg   config.Decide
		reply string
	}{
		{"typesafe with api_key_env", config.Decide{Provider: "typesafe", APIKeyEnv: "FATHOM_TEST_KS2_KEY"}, ksNoulReply},
		{"openai with api_key_cmd", config.Decide{Provider: "openai", APIKeyCmd: []string{"fake-credential-helper"}}, ksPredicateReply},
	} {
		t.Run(tc.name+": the kill file", func(t *testing.T) {
			home := isolateKill(t)
			killFile := filepath.Join(home, decide.KillFileName)
			if err := os.WriteFile(killFile, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			keys, dials := newKeyCounter(fakeEnv), &dialCounter{reply: tc.reply}
			var disclose bytes.Buffer
			ports, err := decide.Open(tc.cfg, nil, "", nil, decide.WithKeyIO(keys.io()), decide.WithHTTPClient(dials.client()), decide.WithDisclose(&disclose), decide.WithUserConfig(""))
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			if !ports.Gate.Closed || ports.Gate.Source != decide.KillSourceFile {
				t.Fatalf("Gate = %+v, want closed by the kill file at Open", ports.Gate)
			}
			gates := 0
			gated := ports.Gated(func(d decision.Decider) decision.Decider { gates++; return d })
			for _, d := range []decision.Decider{ports.Decider, gated, ports.Decider} {
				killed(t, ask(d), decide.KillSourceFile)
			}
			if keys.total() != 0 || dials.count() != 0 || gates != 0 {
				t.Fatalf("while killed: key touches %d (%v), dials %d, gates applied %d, want none", keys.total(), keys.lookups, dials.count(), gates)
			}

			if err := os.Remove(killFile); err != nil {
				t.Fatal(err)
			}
			if err := ask(gated); err != nil {
				t.Fatalf("the first ask after the kill lifted: %v, want an answer with no restart", err)
			}
			if err := ask(ports.Decider); err != nil {
				t.Fatalf("the second ask: %v", err)
			}
			if dials.count() != 2 || keys.total() == 0 || gates != 1 {
				t.Errorf("after the kill lifted: dials %d, key touches %d, gates applied %d, want 2 requests, the key read, the gate applied once", dials.count(), keys.total(), gates)
			}
			// SR-58: the reopened port is behind the calibration gate too,
			// so a site with no reading is refused and nothing is dialed.
			for _, d := range []decision.Decider{gated, ports.Decider} {
				var nce *decide.NotCalibratedError
				if _, err := d.Decide(decide.WithSite(context.Background(), decide.SiteRoute), predicateRequest("q1")); !errors.As(err, &nce) {
					t.Errorf("a reopened port's uncalibrated site ask: err %v, want a *NotCalibratedError", err)
				}
			}
			if dials.count() != 2 {
				t.Errorf("a reopened port's uncalibrated site dialed %d more times, want 0", dials.count()-2)
			}

			if err := os.WriteFile(killFile, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			killed(t, ask(gated), decide.KillSourceFile)
			if dials.count() != 2 {
				t.Errorf("the kill came back and the port dialed %d more times", dials.count()-2)
			}
		})
	}

	t.Run("the user's enabled = false", func(t *testing.T) {
		home := isolateKill(t)
		userPath := filepath.Join(home, "config.toml")
		body := "[decide]\nprovider = \"typesafe\"\napi_key_env = \"FATHOM_TEST_KS2_KEY\"\n"
		if err := os.WriteFile(userPath, []byte(body+"enabled = false\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		loaded, err := config.Load(userPath, "")
		if err != nil {
			t.Fatal(err)
		}
		keys, dials := newKeyCounter(fakeEnv), &dialCounter{reply: ksNoulReply}
		ports, err := decide.Open(loaded.Config.Decide, nil, "", nil, decide.WithKeyIO(keys.io()), decide.WithHTTPClient(dials.client()), decide.WithDisclose(io.Discard), decide.WithUserConfig(userPath))
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		killed(t, ask(ports.Decider), decide.KillSourceEnabled)
		if keys.total() != 0 || dials.count() != 0 {
			t.Fatalf("while disabled: key touches %d, dials %d, want none", keys.total(), dials.count())
		}
		if err := os.WriteFile(userPath, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		when := time.Now().Add(time.Second)
		if err := os.Chtimes(userPath, when, when); err != nil {
			t.Fatal(err)
		}
		if err := ask(ports.Decider); err != nil || dials.count() != 1 {
			t.Fatalf("after enabled came back: err %v dials %d, want the port open with no restart", err, dials.count())
		}
	})

	t.Run("provider llm: no lane until the kill lifts, then the lane and its model once", func(t *testing.T) {
		home := isolateKill(t)
		killFile := filepath.Join(home, decide.KillFileName)
		if err := os.WriteFile(killFile, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		lanes, models := 0, 0
		lane := func(string) (llmclient.LLMClient, error) { lanes++; return llmclient.NewMockClient(), nil }
		model := func() string { models++; return "m" }
		ports, err := decide.Open(config.Decide{Provider: "llm"}, lane, "", nil, decide.WithLaneModel(model), decide.WithDisclose(io.Discard), decide.WithUserConfig(""))
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		killed(t, ask(ports.Decider), decide.KillSourceFile)
		if lanes != 0 || models != 0 {
			t.Fatalf("while killed: lanes %d models %d, want none", lanes, models)
		}
		if err := os.Remove(killFile); err != nil {
			t.Fatal(err)
		}
		_ = ask(ports.Decider)
		_ = ask(ports.Decider)
		if lanes != 1 || models != 1 {
			t.Errorf("after the kill lifted: lanes %d models %d, want each built once", lanes, models)
		}
	})

	t.Run("a port that fails to open after the kill lifts says so once and stays on the deterministic path", func(t *testing.T) {
		home := isolateKill(t)
		killFile := filepath.Join(home, decide.KillFileName)
		if err := os.WriteFile(killFile, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		var disclose bytes.Buffer
		keys := newKeyCounter(map[string]string{})
		ports, err := decide.Open(config.Decide{Provider: "typesafe", APIKeyEnv: "FATHOM_TEST_KS2_UNSET"}, nil, "", nil, decide.WithKeyIO(keys.io()), decide.WithDisclose(&disclose), decide.WithUserConfig(""))
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if err := os.Remove(killFile); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if err := ask(ports.Decider); !errors.Is(err, decision.ErrNoProvider) {
				t.Fatalf("Decide err = %v, want ErrNoProvider", err)
			}
		}
		if n := strings.Count(disclose.String(), "decision port disabled"); n != 1 {
			t.Errorf("disclosed %d disabled lines, want one:\n%s", n, disclose.String())
		}
		if n := keys.lookupsOf("FATHOM_TEST_KS2_UNSET"); n != 1 {
			t.Errorf("the key source was probed %d times, want once", n)
		}
	})

	t.Run("the environment and a refused consent never reopen", func(t *testing.T) {
		home := isolateKill(t)
		killFile := filepath.Join(home, decide.KillFileName)
		for _, st := range []struct {
			name string
			cfg  config.Decide
			env  string
		}{
			{"FATHOM_DECIDE=off", config.Decide{Provider: "typesafe", APIKeyEnv: "FATHOM_TEST_KS2_KEY"}, "off"},
			{"a repository-only provider", loadDecide(t, "", "[decide]\nprovider = \"typesafe\"\n"), ""},
		} {
			t.Setenv(decide.KillEnv, st.env)
			if err := os.WriteFile(killFile, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			keys, dials := newKeyCounter(fakeEnv), &dialCounter{reply: ksNoulReply}
			ports, err := decide.Open(st.cfg, nil, "", nil, decide.WithKeyIO(keys.io()), decide.WithHTTPClient(dials.client()), decide.WithDisclose(io.Discard), decide.WithUserConfig(""))
			if err != nil {
				t.Fatalf("%s: Open: %v", st.name, err)
			}
			if err := os.Remove(killFile); err != nil {
				t.Fatal(err)
			}
			if _, ok := ports.Decider.(decision.None); !ok {
				t.Errorf("%s: Decider = %T, want decision.None", st.name, ports.Decider)
			}
			if err := ask(ports.Decider); !errors.Is(err, decision.ErrNoProvider) || keys.total() != 0 || dials.count() != 0 {
				t.Errorf("%s: err %v keys %d dials %d, want closed with nothing touched", st.name, err, keys.total(), dials.count())
			}
		}
	})
}

// TestKilledLineOncePerSessionAndSite is KS-14 (O1131, ADR-073 decision 6:
// a miss is disclosed once per session, site and class): under the kill
// file, two sessions times two sites asking twice each through the gated
// ports serve-http builds (/route's, and the client's with its site) write
// four `killed` lines, each naming its site and session; a third ask writes
// none; and the disclosure clears when the port reopens. It holds for a
// port open before the kill (killable) and one the kill held closed at
// Open (reopening).
func TestKilledLineOncePerSessionAndSite(t *testing.T) {
	unsetProviderKeys(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_TEST_KS14_KEY", testKey)
	fakeEnv := map[string]string{"FATHOM_TEST_KS14_KEY": testKey}
	cfg := config.Decide{Provider: "typesafe", APIKeyEnv: "FATHOM_TEST_KS14_KEY"}
	identity := func(d decision.Decider) decision.Decider { return d }

	for _, tc := range []struct {
		name         string
		killedAtOpen bool
	}{{"open, then killed", false}, {"killed at Open", true}} {
		t.Run(tc.name, func(t *testing.T) {
			home := isolateKill(t)
			killFile := filepath.Join(home, decide.KillFileName)
			kill := func() {
				t.Helper()
				if err := os.WriteFile(killFile, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.killedAtOpen {
				kill()
			}
			dials := &dialCounter{reply: ksNoulReply}
			var disclose bytes.Buffer
			ports, err := decide.Open(cfg, nil, "", nil, decide.WithKeyIO(newKeyCounter(fakeEnv).io()), decide.WithHTTPClient(dials.client()), decide.WithDisclose(&disclose), decide.WithUserConfig(""),
				decide.CalibratedFor(t, "typesafe", "jev-latest", decide.AllSites...))
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			// As wirePorts does: /route asks one gated port and sets its
			// site on the context; the client asks another with its site.
			route := ports.Gated(identity)
			rerank := decide.AtSite(ports.Gated(identity), decide.SiteObligationsRerank)
			at := func(session, site string) context.Context {
				ctx := decide.WithSession(context.Background(), session)
				if site != "" {
					ctx = decide.WithSite(ctx, site)
				}
				return ctx
			}
			ask := func(d decision.Decider, ctx context.Context) error {
				_, err := d.Decide(ctx, predicateRequest("q1"))
				return err
			}
			mustKill := func(d decision.Decider, ctx context.Context) {
				t.Helper()
				var ke *decide.KilledError
				if err := ask(d, ctx); !errors.As(err, &ke) || ke.Gate.Source != decide.KillSourceFile {
					t.Fatalf("Decide err = %v, want a *KilledError (kill_source file)", err)
				}
			}
			lines := func() int { return strings.Count(disclose.String(), "decision port killed") }
			if !tc.killedAtOpen {
				if err := ask(route, at("s1", decide.SiteRoute)); err != nil {
					t.Fatalf("before the kill: %v", err)
				}
				kill()
			}

			for _, session := range []string{"s1", "s2"} {
				for range 2 {
					mustKill(route, at(session, decide.SiteRoute))
					mustKill(rerank, at(session, ""))
				}
			}
			if n := lines(); n != 4 {
				t.Fatalf("two sessions x two sites asking twice each wrote %d killed lines, want 4:\n%s", n, disclose.String())
			}
			for _, want := range []string{"site route, session s1", "site route, session s2", "site obligations_rerank, session s1", "site obligations_rerank, session s2"} {
				if !strings.Contains(disclose.String(), want) {
					t.Errorf("no killed line names %q:\n%s", want, disclose.String())
				}
			}

			mustKill(route, at("s2", decide.SiteRoute))
			mustKill(rerank, at("s1", ""))
			if n := lines(); n != 4 {
				t.Fatalf("a third ask wrote %d more killed lines, want none", n-4)
			}

			if err := os.Remove(killFile); err != nil {
				t.Fatal(err)
			}
			if err := ask(route, at("s1", decide.SiteRoute)); err != nil {
				t.Fatalf("after the kill lifted: %v, want the port open again", err)
			}
			kill()
			mustKill(route, at("s1", decide.SiteRoute))
			if n := lines(); n != 5 {
				t.Errorf("the port reopened and was killed again: %d killed lines, want 5 (the disclosure clears on reopen)", n)
			}
		})
	}
}
