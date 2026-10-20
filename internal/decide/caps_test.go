package decide_test

// caps_test.go — KS-10 (docs/plans/decision-port.md section J, ADR-073
// decision 7): [decide] max_asks_per_site_per_session is the minimum across
// layers, counts asks per session and site through the opened port, and
// past it each session and site hears one `capped` line while the site
// answers on its deterministic path; an ask whose ceiling is spent is not
// made. Every provider here is a counting fake transport: no key, no socket.

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

func TestAskCapIsTheMinimumAcrossLayersAndDisclosedOnce(t *testing.T) {
	if fs, ok := config.ScopeOf("decide.max_asks_per_site_per_session"); !ok || fs.Class != config.ScopeBehaviour {
		t.Errorf("ScopeOf(decide.max_asks_per_site_per_session) = %+v, %v; want behaviour", fs, ok)
	}

	t.Run("the minimum across layers: a repository lowers the user's and never raises it", func(t *testing.T) {
		key := func(n string) string { return "[decide]\nmax_asks_per_site_per_session = " + n + "\n" }
		for _, tc := range []struct {
			name, user, project string
			want                int
			layer               config.Source
		}{
			{"no layer sets it", "", "", 200, ""},
			{"the user's alone", key("50"), "", 50, config.SourceUser},
			{"the user may raise the default", key("500"), "", 500, config.SourceUser},
			{"a repository lowers the user's", key("50"), key("20"), 20, config.SourceProject},
			{"a repository cannot raise the user's", key("50"), key("100"), 50, config.SourceUser},
			{"a repository cannot raise the default", "", key("500"), 200, ""},
			{"a repository lowers the default", "", key("30"), 30, config.SourceProject},
		} {
			d := loadDecide(t, tc.user, tc.project)
			if got := d.MaxAsks(); got != tc.want || d.MaxAsksSource != tc.layer {
				t.Errorf("%s: MaxAsks() = %d from %q, want %d from %q", tc.name, got, d.MaxAsksSource, tc.want, tc.layer)
			}
		}
	})

	unsetProviderKeys(t)
	t.Setenv("HOME", t.TempDir())
	isolateKill(t)
	t.Setenv("FATHOM_TEST_KS10_KEY", testKey)
	fakeEnv := map[string]string{"FATHOM_TEST_KS10_KEY": testKey}
	cfg := loadDecide(t, "[decide]\nprovider = \"typesafe\"\napi_key_env = \"FATHOM_TEST_KS10_KEY\"\nmax_asks_per_site_per_session = 5\n",
		"[decide]\nmax_asks_per_site_per_session = 3\n")
	if cfg.MaxAsks() != 3 {
		t.Fatalf("MaxAsks() = %d, want 3 (the repository's lower value)", cfg.MaxAsks())
	}
	dials := &dialCounter{reply: ksNoulReply}
	var disclose bytes.Buffer
	var log bytes.Buffer
	rec := decide.NewRecorder(&log, func() time.Time { return time.Unix(0, 0).UTC() }, "test")
	ports, err := decide.Open(cfg, nil, "", rec, decide.WithKeyIO(newKeyCounter(fakeEnv).io()), decide.WithHTTPClient(dials.client()), decide.WithDisclose(&disclose), decide.WithUserConfig(""),
		decide.CalibratedFor(t, "typesafe", "jev-latest", decide.AllSites...))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	identity := func(d decision.Decider) decision.Decider { return d }
	route := ports.Gated(identity) // /route's port, as wirePorts builds it
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
	lines := func() int { return strings.Count(disclose.String(), "decision port capped") }

	for range 3 {
		for _, a := range []struct {
			d   decision.Decider
			ctx context.Context
		}{{route, at("s1", decide.SiteRoute)}, {rerank, at("s1", "")}, {route, at("s2", decide.SiteRoute)}} {
			if err := ask(a.d, a.ctx); err != nil {
				t.Fatalf("an ask within the cap: %v", err)
			}
		}
	}
	if dials.count() != 9 || lines() != 0 {
		t.Fatalf("nine asks within the cap: %d dials, %d capped lines; want 9 and 0", dials.count(), lines())
	}

	mustCap := func(d decision.Decider, ctx context.Context) error {
		t.Helper()
		err := ask(d, ctx)
		var ke *decide.KilledError
		if !errors.As(err, &ke) || ke.Gate.Source != decide.KillSourceCapped || ke.Gate.Layer != "project" || !errors.Is(err, decision.ErrNoProvider) {
			t.Fatalf("an ask past the cap: err = %v, want a *KilledError (kill_source capped, project) that is ErrNoProvider", err)
		}
		return err
	}
	capErr := mustCap(route, at("s1", decide.SiteRoute))
	mustCap(route, at("s1", decide.SiteRoute))
	if n := lines(); n != 1 {
		t.Fatalf("two asks past the cap from one session and site wrote %d capped lines, want 1:\n%s", n, disclose.String())
	}
	mustCap(rerank, at("s1", ""))
	mustCap(route, at("s2", decide.SiteRoute))
	mustCap(rerank, at("s1", ""))
	if n := lines(); n != 3 {
		t.Fatalf("three capped (session, site) scopes wrote %d capped lines, want 3:\n%s", n, disclose.String())
	}
	for _, want := range []string{
		"warn: decision port capped (kill_source capped, project) for site route, session s1: [decide] max_asks_per_site_per_session = 3 reached",
		"site obligations_rerank, session s1", "site route, session s2", "the site answers on its deterministic path",
	} {
		if !strings.Contains(disclose.String(), want) {
			t.Errorf("the capped lines do not hold %q:\n%s", want, disclose.String())
		}
	}
	if dials.count() != 9 {
		t.Errorf("asks past the cap dialed: %d dials, want 9", dials.count())
	}
	if err := ask(rerank, at("s2", "")); err != nil {
		t.Errorf("another session's ask from a capped site: %v, want it answered (the cap is per session and site)", err)
	}

	rec.RecordError(decide.SiteRoute, ports.Decider.Name(), "", 1, 0, capErr)
	if !strings.Contains(log.String(), `"kill_source":"capped"`) {
		t.Errorf("a capped ask's row carries no kill_source capped:\n%s", log.String())
	}

	t.Run("an ask whose ceiling its joins spent is not made and not counted", func(t *testing.T) {
		before := dials.count()
		ctx, cancel := context.WithDeadline(at("s3", decide.SiteRoute), time.Now().Add(-time.Millisecond))
		defer cancel()
		var ke *decide.KilledError
		if err := ask(route, ctx); !errors.As(err, &ke) || ke.Gate.Source != decide.KillSourceDeadline {
			t.Fatalf("an ask past its ceiling: err = %v, want a *KilledError (kill_source deadline)", err)
		}
		for range 3 {
			if err := ask(route, at("s3", decide.SiteRoute)); err != nil {
				t.Fatalf("the spent ask counted against the cap: %v", err)
			}
		}
		if dials.count() != before+3 {
			t.Errorf("dials = %d, want %d (the spent ask dialed nothing)", dials.count(), before+3)
		}
	})
}
