package semantic

// rerankdeadline_test.go — KS-10 (docs/plans/decision-port.md section J,
// ADR-073 decision 7): on a path whose context carries the caller's ceiling
// (the edit hook's X-Fathom-Deadline, which serve-http's rpcContext turns
// into the request context's deadline), the obligations re-rank asks for
// the ceiling less the time its joins took, never rerank_timeout_ms past
// it, and a ceiling the joins spent refuses the ask before it is made.

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// stepClock is an injected clock that a slow fake join moves forward.
type stepClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *stepClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *stepClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// slowJoinStore is a store whose serve-time claims reads take join in all
// on the injected clock: the slow fake join before the re-rank.
type slowJoinStore struct {
	store.Store
	clock *stepClock
	join  time.Duration
	once  *sync.Once
}

func (s slowJoinStore) ClaimsForServe(ctx context.Context, serviceID string, layers []string, statuses []core.ClaimStatus, scopePrefixes []string) ([]core.SemanticClaim, error) {
	s.once.Do(func() { s.clock.advance(s.join) })
	return s.Store.ClaimsForServe(ctx, serviceID, layers, statuses, scopePrefixes)
}

// deadlineProbe answers every question and records the deadline each ask's
// context carried.
type deadlineProbe struct {
	mu        sync.Mutex
	deadlines []time.Time
}

func (p *deadlineProbe) Name() string { return "probe" }

func (p *deadlineProbe) Decide(ctx context.Context, req decision.Request) (decision.Response, error) {
	dl, _ := ctx.Deadline()
	p.mu.Lock()
	p.deadlines = append(p.deadlines, dl)
	p.mu.Unlock()
	answers := map[string]float64{}
	for _, q := range req.Questions {
		answers[q.ID] = 0.9
	}
	return mockNoulResponse("probe", answers), nil
}

func (p *deadlineProbe) asked() []time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]time.Time(nil), p.deadlines...)
}

func TestObligationsRerankDeadlineIsTheCeilingMinusItsJoins(t *testing.T) {
	const (
		ceiling     = 2 * time.Second         // the edit hook's client timeout
		siteTimeout = 1500 * time.Millisecond // [decide] rerank_timeout_ms
	)
	mem := store.NewMemoryStore()
	seedServed(t, mem, "1-exact", "1-exact: requires review before any change.", []string{rerankTestFile}, core.ClaimFresh)
	seedServed(t, mem, "2-exact", "2-exact: requires review before any change too.", []string{rerankTestFile}, core.ClaimFresh)
	none, err := ObligationsFor(context.Background(), mem, "svc", rerankTestFile, 0)
	if err != nil || len(none.Rules) != 2 {
		t.Fatalf("none path: %d rules, %v", len(none.Rules), err)
	}

	type served struct {
		obl           Obligations
		probe         *deadlineProbe
		before, after time.Time
	}
	serve := func(t *testing.T, join time.Duration) served {
		t.Helper()
		start := time.Now()
		clock := &stepClock{now: start}
		ctx, cancel := context.WithDeadline(context.Background(), start.Add(ceiling))
		defer cancel()
		probe := &deadlineProbe{}
		before := time.Now()
		obl, err := ObligationsForOpts(ctx, slowJoinStore{Store: mem, clock: clock, join: join, once: &sync.Once{}}, "svc", rerankTestFile, 0,
			ObligationsOptions{Decider: probe, RankWithinTier: true, RerankTimeout: siteTimeout, Now: clock.Now})
		after := time.Now()
		if err != nil {
			t.Fatalf("ObligationsForOpts: %v", err)
		}
		return served{obl: obl, probe: probe, before: before, after: after}
	}
	// within asserts every ask ran under a deadline budget past the moment
	// the re-rank started (between before and after).
	within := func(t *testing.T, s served, budget time.Duration) {
		t.Helper()
		asks := s.probe.asked()
		if len(asks) == 0 {
			t.Fatalf("the re-rank asked nothing: %+v", s.obl)
		}
		for _, dl := range asks {
			if dl.Before(s.before.Add(budget)) || dl.After(s.after.Add(budget)) {
				t.Errorf("an ask ran under deadline %v, want the re-rank's start plus %v (between %v and %v)",
					dl, budget, s.before.Add(budget), s.after.Add(budget))
			}
		}
		if s.obl.RankedBy != "probe" {
			t.Errorf("RankedBy = %q, want probe", s.obl.RankedBy)
		}
	}

	t.Run("fast joins leave the site's own budget", func(t *testing.T) {
		within(t, serve(t, 0), siteTimeout)
	})

	t.Run("a slow join leaves less time", func(t *testing.T) {
		// 2000 ms ceiling, 800 ms of joins: 1200 ms left, under the 1500 ms
		// rerank_timeout_ms that would otherwise run from the re-rank's start.
		within(t, serve(t, 800*time.Millisecond), ceiling-800*time.Millisecond)
	})

	t.Run("joins past the ceiling refuse the ask", func(t *testing.T) {
		s := serve(t, 2500*time.Millisecond)
		if n := len(s.probe.asked()); n != 0 {
			t.Fatalf("the joins spent the ceiling and the re-rank still asked %d times", n)
		}
		if s.obl.RankedBy != "none" {
			t.Errorf("RankedBy = %q, want none (the deterministic path)", s.obl.RankedBy)
		}
		if len(s.obl.Rules) != len(none.Rules) {
			t.Fatalf("served %d rules, want the none path's %d", len(s.obl.Rules), len(none.Rules))
		}
		for i := range none.Rules {
			if s.obl.Rules[i].ClaimID != none.Rules[i].ClaimID {
				t.Errorf("rule %d = %s, want the none path's %s", i, s.obl.Rules[i].ClaimID, none.Rules[i].ClaimID)
			}
		}
		if len(s.obl.DecisionErrors) != 1 {
			t.Fatalf("DecisionErrors = %+v, want the one refusal", s.obl.DecisionErrors)
		}
		var ke *decide.KilledError
		refusal := s.obl.DecisionErrors[0].Err
		if !errors.As(refusal, &ke) || ke.Gate.Source != decide.KillSourceDeadline || !errors.Is(refusal, decision.ErrNoProvider) {
			t.Fatalf("refusal = %v, want a *KilledError (kill_source deadline) that is ErrNoProvider", refusal)
		}
		if !strings.Contains(s.obl.RankNote, "kill_source deadline") || !strings.Contains(s.obl.RankNote, "2500ms") {
			t.Errorf("RankNote = %q, want the deadline refusal naming the joins' 2500ms", s.obl.RankNote)
		}
	})

	t.Run("no ceiling keeps the site's own budget", func(t *testing.T) {
		probe := &deadlineProbe{}
		before := time.Now()
		obl, err := ObligationsForOpts(context.Background(), mem, "svc", rerankTestFile, 0,
			ObligationsOptions{Decider: probe, RankWithinTier: true, RerankTimeout: siteTimeout})
		if err != nil {
			t.Fatal(err)
		}
		within(t, served{obl: obl, probe: probe, before: before, after: time.Now()}, siteTimeout)
	})
}
