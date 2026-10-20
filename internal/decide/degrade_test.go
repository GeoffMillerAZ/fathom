package decide_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// statusTransport answers every request with the status the test last set,
// without opening a socket.
type statusTransport struct {
	mu     sync.Mutex
	status int
	n      int
}

func (s *statusTransport) set(status int) {
	s.mu.Lock()
	s.status = status
	s.mu.Unlock()
}

func (s *statusTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	s.n++
	status := s.status
	s.mu.Unlock()
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":"try later"}`)),
		Request:    r,
	}, nil
}

// TestDegradeLineOncePerSessionSiteAndClass is JG-3 (ADR-073 decision 6:
// a miss is disclosed once per session, site and class). Two sessions times
// two sites times two classes (a 429, a 529), each asking twice, write
// eight degrade lines, each naming its scope; a third round writes none.
// Every ask still returns the degrade, so each answer's own receipt names
// `not consulted (<class>)` whether or not its line was written. The scope
// is KS-14's (decide.WithSession, decide.WithSite), the one the killed line
// is keyed by.
func TestDegradeLineOncePerSessionSiteAndClass(t *testing.T) {
	unsetProviderKeys(t)
	isolateKill(t)
	t.Setenv("FATHOM_TEST_JG3_KEY", testKey)
	tr := &statusTransport{status: http.StatusTooManyRequests}
	var disclose bytes.Buffer
	cfg := config.Decide{Provider: "typesafe", APIKeyEnv: "FATHOM_TEST_JG3_KEY"}
	ports, err := decide.Open(cfg, nil, "", nil,
		decide.WithKeyIO(newKeyCounter(map[string]string{"FATHOM_TEST_JG3_KEY": testKey}).io()),
		decide.WithHTTPClient(&http.Client{Transport: tr}), decide.WithDisclose(&disclose), decide.WithUserConfig(""),
		decide.CalibratedFor(t, "typesafe", "jev-latest", decide.AllSites...))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	classes := []struct {
		status int
		class  string
	}{{http.StatusTooManyRequests, "rate limited"}, {529, "overloaded"}}
	sites := []string{decide.SiteRoute, decide.SiteObligationsRerank}
	sessions := []string{"s1", "s2"}
	lines := func() []string {
		var out []string
		for _, l := range strings.Split(strings.TrimSpace(disclose.String()), "\n") {
			if strings.Contains(l, "decision port degraded") {
				out = append(out, l)
			}
		}
		return out
	}
	round := func(asks int) {
		t.Helper()
		for _, c := range classes {
			tr.set(c.status)
			for _, session := range sessions {
				for _, site := range sites {
					ctx := decide.WithSite(decide.WithSession(context.Background(), session), site)
					for i := 0; i < asks; i++ {
						_, err := ports.Decider.Decide(ctx, predicateRequest("q1"))
						var de *decide.DegradedError
						if !errors.As(err, &de) || !errors.Is(err, decision.ErrNoProvider) {
							t.Fatalf("%s/%s/%s: Decide err = %v, want a *DegradedError that is ErrNoProvider", c.class, session, site, err)
						}
						receipt := decide.DecidedBy(decide.Decision{Provider: ports.Decider.Name(), Tier: decide.DecidedComputed, Miss: decide.MissClass(err)})
						if want := "not consulted (" + c.class + ")"; !strings.Contains(receipt, want) {
							t.Errorf("%s/%s/%s: receipt %q does not name %q", c.class, session, site, receipt, want)
						}
					}
				}
			}
		}
	}

	round(2)
	got := lines()
	if len(got) != 8 {
		t.Fatalf("two sessions x two sites x two classes asking twice wrote %d degrade lines, want 8:\n%s", len(got), disclose.String())
	}
	for _, c := range classes {
		for _, session := range sessions {
			for _, site := range sites {
				want := "decision port degraded (" + c.class + ") for site " + site + ", session " + session
				n := 0
				for _, l := range got {
					if strings.Contains(l, want) {
						n++
					}
				}
				if n != 1 {
					t.Errorf("%d lines say %q, want exactly 1:\n%s", n, want, disclose.String())
				}
			}
		}
	}
	round(1)
	if n := len(lines()); n != 8 {
		t.Errorf("a third ask from every scope wrote %d more degrade lines, want none:\n%s", n-8, disclose.String())
	}

	// The process scope (serve-mcp, a CLI verb) hears of each class once.
	tr.set(http.StatusTooManyRequests)
	for i := 0; i < 3; i++ {
		_, _ = ports.Decider.Decide(context.Background(), predicateRequest("q1"))
	}
	if n := len(lines()); n != 9 {
		t.Errorf("three asks from the process scope wrote %d lines, want 1:\n%s", n-8, disclose.String())
	}
}
