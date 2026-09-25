package console_test

import (
	"bufio"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// TestEventBus_ReplayBoundAndKeepalive pins OC-13: events carry IDs and
// names-and-IDs only; a reconnect replays exactly the retained events after
// Last-Event-ID (at most the last 64); the stream refuses a 17th subscriber;
// a keepalive comment arrives on the interval; and a slow subscriber is cut
// loose rather than stalling the publisher.
func TestEventBus_ReplayBoundAndKeepalive(t *testing.T) {
	fixed := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	var bus *console.EventBus = console.NewEventBus(console.BusOptions{Keepalive: 30 * time.Millisecond, Now: func() time.Time { return fixed }})

	// IDs are assigned in order; content is refused.
	e1, err := bus.Publish(console.Event{Kind: console.EventIngest, Service: "fathom", Ref: "a683d602"})
	if err != nil || e1.ID != 1 || !e1.At.Equal(fixed) {
		t.Fatalf("first publish = %+v, %v", e1, err)
	}
	if _, err := bus.Publish(console.Event{Kind: console.EventVerify, Service: "fathom", Name: "plan has <b>content</b>"}); err == nil {
		t.Error("an event field with prose/markup must be refused")
	}
	if _, err := bus.Publish(console.Event{Kind: "gossip", Service: "fathom"}); err == nil {
		t.Error("an unknown kind must be refused")
	}
	if bus.LastID() != 1 {
		t.Fatalf("refused events must not consume IDs: last = %d", bus.LastID())
	}
	for i := 0; i < 99; i++ {
		kind := []console.EventKind{console.EventIngest, console.EventCompile, console.EventVerify, console.EventSession, console.EventDaemon}[i%5]
		if _, err := bus.Publish(console.Event{Kind: kind, Service: "svc-" + string(rune('a'+i%3)), Name: "n", Ref: "r"}); err != nil {
			t.Fatal(err)
		}
	}
	if bus.LastID() != 100 {
		t.Fatalf("last id = %d, want 100", bus.LastID())
	}

	// Replay: the ring keeps the last 64; after ID 90 means 91..100.
	sub, replay, err := bus.Subscribe(90)
	if err != nil {
		t.Fatal(err)
	}
	if len(replay) != 10 || replay[0].ID != 91 || replay[9].ID != 100 {
		t.Fatalf("replay after 90 = %d events (%v..), want 91..100", len(replay), replay)
	}
	sub.Close()
	_, all, _ := bus.Subscribe(0)
	if len(all) != console.DefaultReplay || all[0].ID != 37 {
		t.Fatalf("full replay = %d events starting at %d, want the last %d (37..100)", len(all), all[0].ID, console.DefaultReplay)
	}

	// Verify events name the plans panel source.
	var verifySources []string
	for _, e := range all {
		if e.Kind == console.EventVerify {
			verifySources = e.Sources()
			break
		}
	}
	if len(verifySources) == 0 || !strings.HasPrefix(verifySources[0], "plans:") {
		t.Errorf("a verify event must name the plans panel source, got %v", verifySources)
	}

	// The subscriber bound: the bus above already holds one open
	// subscription (the full replay); fill to 16, then the 17th is refused.
	var subs []*console.Subscription
	for bus.Subscribers() < console.DefaultMaxSubs {
		s, _, err := bus.Subscribe(bus.LastID())
		if err != nil {
			t.Fatalf("subscriber %d refused early: %v", bus.Subscribers()+1, err)
		}
		subs = append(subs, s)
	}
	if _, _, err := bus.Subscribe(0); !errors.Is(err, console.ErrTooManySubscribers) {
		t.Fatalf("17th subscriber: err = %v, want ErrTooManySubscribers", err)
	}
	// Over HTTP the 17th gets a 503 with Retry-After, not a hung stream.
	rec := httptest.NewRecorder()
	bus.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/events", nil))
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("17th HTTP subscriber = %d (Retry-After %q), want 503 with Retry-After", rec.Code, rec.Header().Get("Retry-After"))
	}
	for _, s := range subs {
		s.Close()
	}

	// A slow subscriber (never reads) is cut loose once its buffer fills;
	// the publisher never blocks.
	slow, _, err := bus.Subscribe(bus.LastID())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		for i := 0; i < 200; i++ {
			_, _ = bus.Publish(console.Event{Kind: console.EventSession, Service: "fathom", Ref: "s"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("publisher blocked on a slow subscriber")
	}
	drained := 0
	for range slow.C() {
		drained++
	}
	if drained == 0 || drained >= 200 {
		t.Errorf("slow subscriber drained %d events; want its buffer, then a closed channel", drained)
	}

	// Over HTTP: hello, the replay after Last-Event-ID, then a keepalive.
	fresh := console.NewEventBus(console.BusOptions{Keepalive: 30 * time.Millisecond, Epoch: "e1"})
	for _, svc := range []string{"a", "b", "c"} {
		if _, err := fresh.Publish(console.Event{Kind: console.EventIngest, Service: svc, Ref: "c1"}); err != nil {
			t.Fatal(err)
		}
	}
	ts := httptest.NewServer(fresh)
	defer ts.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL, nil)
	req.Header.Set("Last-Event-ID", "e1-1")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("content-type = %q", ct)
	}
	sc := bufio.NewScanner(resp.Body)
	var lines []string
	sawKeepalive := false
	for sc.Scan() {
		line := sc.Text()
		lines = append(lines, line)
		if line == ": keepalive" {
			sawKeepalive = true
			break
		}
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "event: hello") {
		t.Errorf("stream must open with hello:\n%s", joined)
	}
	if strings.Contains(joined, "id: e1-1\n") || !strings.Contains(joined, "id: e1-2\nevent: ingest") || !strings.Contains(joined, "id: e1-3\nevent: ingest") {
		t.Errorf("replay after Last-Event-ID e1-1 must carry ids e1-2 and e1-3 only:\n%s", joined)
	}
	if !strings.Contains(joined, `"epoch":"e1"`) {
		t.Errorf("hello must name the stream's epoch:\n%s", joined)
	}
	if !strings.Contains(joined, `"sources":["index:b"]`) {
		t.Errorf("event data must carry its panel sources:\n%s", joined)
	}
	if !sawKeepalive {
		t.Errorf("no keepalive within the stream:\n%s", joined)
	}
}

// TestEventBus_ReplaySurvivesDaemonRestart: event IDs are per process, so a
// browser that reconnects after a daemon restart sends the OLD process's
// Last-Event-ID. The stream's epoch (a boot ID in every SSE id) tells the
// new bus the cursor is foreign, and it replays its whole ring — the
// restart event first — instead of filtering out every event at or below a
// number from another process.
func TestEventBus_ReplaySurvivesDaemonRestart(t *testing.T) {
	old := console.NewEventBus(console.BusOptions{Epoch: "boot1"})
	for i := 0; i < 5; i++ {
		if _, err := old.Publish(console.Event{Kind: console.EventVerify, Service: "fathom", Name: "plan.md", Ref: "pass"}); err != nil {
			t.Fatal(err)
		}
	}
	cursor := old.Cursor()
	if cursor != "boot1-5" {
		t.Fatalf("cursor = %q, want boot1-5", cursor)
	}
	fresh := console.NewEventBus(console.BusOptions{Epoch: "boot2"})
	restart, _ := fresh.Publish(console.Event{Kind: console.EventDaemon, Name: "restart", Ref: "abc"})
	for i := 0; i < 9; i++ {
		_, _ = fresh.Publish(console.Event{Kind: console.EventIngest, Service: "fathom", Ref: "c1"})
	}
	_, replay, err := fresh.SubscribeAfter(cursor)
	if err != nil {
		t.Fatal(err)
	}
	if len(replay) != 10 || replay[0].ID != restart.ID || replay[0].Kind != console.EventDaemon {
		t.Fatalf("replay after a foreign cursor = %d events (first %+v), want all 10, the restart first", len(replay), replay)
	}
	// The same epoch still resumes exactly after its cursor.
	_, replay, _ = fresh.SubscribeAfter("boot2-8")
	if len(replay) != 2 || replay[0].ID != 9 {
		t.Fatalf("replay after boot2-8 = %v, want ids 9 and 10", replay)
	}
	// A bare number beyond this bus's last ID cannot be one of its own.
	small := console.NewEventBus(console.BusOptions{Epoch: "boot3"})
	_, _ = small.Publish(console.Event{Kind: console.EventDaemon, Name: "restart", Ref: "abc"})
	if _, replay, _ := small.Subscribe(old.LastID()); len(replay) != 1 {
		t.Fatalf("a lastID from another process (5 > 1) must replay the ring, got %d", len(replay))
	}
	// Over HTTP: the old process's Last-Event-ID still gets the restart.
	ts := httptest.NewServer(fresh)
	defer ts.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL, nil)
	req.Header.Set("Last-Event-ID", cursor)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	var got []string
	for sc.Scan() {
		got = append(got, sc.Text())
		if strings.HasPrefix(sc.Text(), "id: boot2-10") {
			break
		}
	}
	if joined := strings.Join(got, "\n"); !strings.Contains(joined, "id: boot2-1\nevent: daemon") {
		t.Errorf("a reconnect across a restart must replay the restart event:\n%s", joined)
	}
}

// TestLedgerTail_ReportsOnlyAppendedLines pins the verify/session pollers'
// reader: nothing on the priming poll, whole appended lines after, a
// partial trailing line held until it completes, and a truncation re-read.
func TestLedgerTail_ReportsOnlyAppendedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	if err := os.WriteFile(path, []byte("{\"old\":1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tail := &console.LedgerTail{Path: path}
	if lines, err := tail.Poll(); err != nil || len(lines) != 0 {
		t.Fatalf("priming poll = %q, %v; want nothing", lines, err)
	}
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString("{\"a\":1}\n{\"b\":")
	f.Close()
	lines, _ := tail.Poll()
	if len(lines) != 1 || string(lines[0]) != `{"a":1}` {
		t.Fatalf("after append = %q, want the one whole line", lines)
	}
	f, _ = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString("2}\n")
	f.Close()
	lines, _ = tail.Poll()
	if len(lines) != 1 || string(lines[0]) != `{"b":2}` {
		t.Fatalf("completed line = %q", lines)
	}
	if err := os.WriteFile(path, []byte("{\"new\":1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, _ = tail.Poll()
	if len(lines) != 1 || string(lines[0]) != `{"new":1}` {
		t.Fatalf("after truncation = %q", lines)
	}
	missing := &console.LedgerTail{Path: filepath.Join(t.TempDir(), "none.jsonl")}
	missing.Prime()
	if lines, err := missing.Poll(); err != nil || lines != nil {
		t.Errorf("missing ledger = %q, %v; want nothing and no error", lines, err)
	}
}
