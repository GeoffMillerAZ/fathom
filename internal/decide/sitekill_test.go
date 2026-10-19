package decide_test

// sitekill_test.go — KS-12 (docs/plans/decision-port.md section J, ADR-073
// decision 8): a site whose last 100 asks in decisions.jsonl hold more than
// 10% misses is off for the process's lifetime, with one disclosed line,
// and never turns back on by itself. The rows are synthesized
// decisions.jsonl lines seeded as openDecisionRecorder seeds them, then
// rows this process writes; every provider is a counting fake transport.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// logRows is a decisions.jsonl body: hits answered rows then misses
// failed-call rows for site, in that order.
func logRows(site string, hits, misses int) string {
	var b strings.Builder
	for i := range hits {
		fmt.Fprintf(&b, `{"schema_version":2,"site":%q,"provider":"typesafe","model":"jev-latest","question_id":"q%d","kind":"predicate","noul":0.7}`+"\n", site, i)
	}
	for range misses {
		fmt.Fprintf(&b, `{"schema_version":2,"site":%q,"provider":"typesafe","model":"","error":"Post \"https://provider.example.test/v1\": context deadline exceeded","questions":1}`+"\n", site)
	}
	return b.String()
}

func TestSiteOffAfterTenPercentMissesOverOneHundredAsks(t *testing.T) {
	unsetProviderKeys(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_TEST_KS12_KEY", testKey)
	fakeEnv := map[string]string{"FATHOM_TEST_KS12_KEY": testKey}
	cfg := config.Decide{Provider: "typesafe", APIKeyEnv: "FATHOM_TEST_KS12_KEY"}
	site := decide.SiteObligationsRerank

	type port struct {
		ask      func() error
		dials    *dialCounter
		disclose *bytes.Buffer
		rec      *decide.Recorder
	}
	open := func(t *testing.T, seed string) port {
		t.Helper()
		isolateKill(t)
		var log bytes.Buffer
		rec := decide.NewRecorder(&log, func() time.Time { return time.Unix(0, 0).UTC() }, "test")
		rec.Seed(strings.NewReader(seed))
		dials := &dialCounter{reply: ksNoulReply}
		var disclose bytes.Buffer
		ports, err := decide.Open(cfg, nil, "", rec, decide.WithKeyIO(newKeyCounter(fakeEnv).io()), decide.WithHTTPClient(dials.client()), decide.WithDisclose(&disclose), decide.WithUserConfig(""))
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		d := decide.AtSite(ports.Decider, site)
		return port{
			ask: func() error {
				_, err := d.Decide(decide.WithSession(context.Background(), "s1"), predicateRequest("q1"))
				return err
			},
			dials: dials, disclose: &disclose, rec: rec,
		}
	}
	mustOff := func(t *testing.T, p port) {
		t.Helper()
		before := p.dials.count()
		err := p.ask()
		var ke *decide.KilledError
		if !errors.As(err, &ke) || ke.Gate.Source != decide.KillSourceMisses || !errors.Is(err, decision.ErrNoProvider) {
			t.Fatalf("an ask from a killed site: err = %v, want a *KilledError (kill_source misses) that is ErrNoProvider", err)
		}
		if p.dials.count() != before {
			t.Fatalf("a killed site dialed")
		}
	}
	mustOn := func(t *testing.T, p port) {
		t.Helper()
		if err := p.ask(); err != nil {
			t.Fatalf("an ask from a site that is on: %v", err)
		}
	}
	lines := func(p port) int { return strings.Count(p.disclose.String(), "kill_source misses") }

	t.Run("10 misses in 100 asks stays on", func(t *testing.T) {
		mustOn(t, open(t, logRows(site, 90, 10)))
	})

	t.Run("fewer than 100 asks never judges", func(t *testing.T) {
		mustOn(t, open(t, logRows(site, 9, 90)))
	})

	t.Run("rows the provider never saw are not asks", func(t *testing.T) {
		var notAsks strings.Builder
		for range 50 {
			fmt.Fprintf(&notAsks, `{"site":%q,"provider":"typesafe","error":"decision port killed (kill_source file, file): the kill file is present","kill_source":"file"}`+"\n", site)
			fmt.Fprintf(&notAsks, `{"site":%q,"provider":"typesafe","error":"decision port closed: safe mode: %s"}`+"\n", site, decision.ErrNoProvider)
		}
		mustOn(t, open(t, logRows(site, 89, 10)+notAsks.String()))
	})

	t.Run("11 misses in 100 asks turns the site off at Open, the line once", func(t *testing.T) {
		p := open(t, logRows("route", 0, 100)+logRows(site, 89, 11))
		mustOff(t, p)
		mustOff(t, p)
		if n := lines(p); n != 1 {
			t.Fatalf("two asks from a killed site wrote %d lines, want 1:\n%s", n, p.disclose.String())
		}
		for _, want := range []string{"warn: decision port killed (kill_source misses, log) for site obligations_rerank, session s1: site obligations_rerank missed 11 of its last 100 asks", "the site answers on its deterministic path"} {
			if !strings.Contains(p.disclose.String(), want) {
				t.Errorf("the line does not hold %q:\n%s", want, p.disclose.String())
			}
		}
		if got := p.rec.KilledSites(); strings.Join(got, ",") != "obligations_rerank,route" {
			t.Errorf("KilledSites = %v, want [obligations_rerank route]", got)
		}
	})

	t.Run("an eleventh miss in process turns it off, and it never turns back on by itself", func(t *testing.T) {
		p := open(t, logRows(site, 90, 10))
		mustOn(t, p)
		p.rec.RecordError(site, "typesafe", "", 1, time.Second, &decide.DegradedError{Provider: "typesafe", Err: context.DeadlineExceeded})
		mustOff(t, p)
		for i := range 100 {
			p.rec.Append([]decision.Record{{Site: site, Provider: "typesafe", Model: "jev-latest", QuestionID: fmt.Sprintf("q%d", i), Kind: decision.KindPredicate, Confidence: 0.9}})
		}
		mustOff(t, p)
		if n := lines(p); n != 1 {
			t.Errorf("the kill wrote %d lines, want 1", n)
		}
	})
}
