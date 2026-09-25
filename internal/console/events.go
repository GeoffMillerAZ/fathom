package console

// events.go — live freshness (design §4, OC-13). An in-process bus in
// serve-http publishes ingest, compile, verify, session and daemon events;
// GET /events streams them as server-sent events with an ID, a keepalive
// every 25 s, a replay of the last 64 events on reconnect through
// Last-Event-ID, and a bound of 16 subscribers. Events carry names and IDs
// only, never content: a page learns THAT a source changed and offers
// "updated — refresh this panel"; it never receives what changed.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// EventKind is one of the five published kinds.
type EventKind string

const (
	EventIngest  EventKind = "ingest"  // service, ref = indexed commit
	EventCompile EventKind = "compile" // service, name = layer, ref = run id
	EventVerify  EventKind = "verify"  // service, name = plan, ref = pass|fail
	EventSession EventKind = "session" // service = repo, ref = session id
	EventDaemon  EventKind = "daemon"  // name = start|restart|adopt, ref = binary commit
)

// Bus defaults (design §4).
const (
	DefaultReplay       = 64
	DefaultMaxSubs      = 16
	DefaultKeepalive    = 25 * time.Second
	subscriberBuffer    = 64
	maxEventFieldLength = 200
)

// Event is one bus event. Every field is a name or an ID.
type Event struct {
	ID      uint64    `json:"id"`
	Kind    EventKind `json:"kind"`
	Service string    `json:"service,omitempty"`
	Name    string    `json:"name,omitempty"`
	Ref     string    `json:"ref,omitempty"`
	At      time.Time `json:"at"`
}

// Sources are the panel source keys an event names — a panel carrying
// data-console-source="plans:fathom" is told it is stale by a verify event
// for fathom. Deterministic order.
func (e Event) Sources() []string {
	svc := e.Service
	switch e.Kind {
	case EventIngest:
		return []string{"index:" + svc}
	case EventCompile:
		out := []string{"layers:" + svc}
		if e.Name != "" {
			out = append(out, "layer:"+svc+"/"+e.Name)
		}
		return out
	case EventVerify:
		return []string{"plans:" + svc, "verify:" + svc}
	case EventSession:
		return []string{"sessions", "sessions:" + svc}
	case EventDaemon:
		return []string{"daemon"}
	}
	return nil
}

// nameOrID is the only shape an event field may take: no spaces, no markup,
// no prose — a name or an identifier.
var nameOrID = regexp.MustCompile(`^[A-Za-z0-9._:/@+#-]*$`)

// ErrTooManySubscribers is returned when the subscriber bound is reached.
var ErrTooManySubscribers = errors.New("console: event stream subscriber limit reached")

func validateEvent(e Event) error {
	switch e.Kind {
	case EventIngest, EventCompile, EventVerify, EventSession, EventDaemon:
	default:
		return fmt.Errorf("console: unknown event kind %q", e.Kind)
	}
	for field, v := range map[string]string{"service": e.Service, "name": e.Name, "ref": e.Ref} {
		if len(v) > maxEventFieldLength || !nameOrID.MatchString(v) {
			return fmt.Errorf("console: event %s %q is not a name or an ID — events never carry content", field, v)
		}
	}
	return nil
}

// BusOptions sizes a bus. Zero values take the design defaults.
type BusOptions struct {
	Replay    int
	MaxSubs   int
	Keepalive time.Duration
	Now       func() time.Time
}

// EventBus is the in-process publish/subscribe hub behind GET /events.
type EventBus struct {
	mu        sync.Mutex
	next      uint64
	ring      []Event
	replay    int
	maxSubs   int
	keepalive time.Duration
	now       func() time.Time
	subs      map[*Subscription]struct{}
	dropped   uint64
}

// NewEventBus builds a bus.
func NewEventBus(o BusOptions) *EventBus {
	if o.Replay <= 0 {
		o.Replay = DefaultReplay
	}
	if o.MaxSubs <= 0 {
		o.MaxSubs = DefaultMaxSubs
	}
	if o.Keepalive <= 0 {
		o.Keepalive = DefaultKeepalive
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return &EventBus{replay: o.Replay, maxSubs: o.MaxSubs, keepalive: o.Keepalive, now: o.Now, subs: map[*Subscription]struct{}{}}
}

// Subscription is one live subscriber.
type Subscription struct {
	bus  *EventBus
	ch   chan Event
	once sync.Once
}

// C delivers events published after the subscription began. It is closed
// when the subscription ends — by Close, or because the subscriber fell
// more than a buffer behind (a slow reader never stalls the publisher).
func (s *Subscription) C() <-chan Event { return s.ch }

// Close ends the subscription; safe to call twice.
func (s *Subscription) Close() {
	s.bus.mu.Lock()
	defer s.bus.mu.Unlock()
	s.closeLocked()
}

func (s *Subscription) closeLocked() {
	s.once.Do(func() {
		delete(s.bus.subs, s)
		close(s.ch)
	})
}

// Publish assigns the next ID, stamps the time when unset, appends to the
// replay ring and fans out. An event that is not names-and-IDs only is
// refused and never published.
func (b *EventBus) Publish(e Event) (Event, error) {
	if err := validateEvent(e); err != nil {
		return Event{}, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.next++
	e.ID = b.next
	if e.At.IsZero() {
		e.At = b.now().UTC()
	}
	b.ring = append(b.ring, e)
	if len(b.ring) > b.replay {
		b.ring = append([]Event(nil), b.ring[len(b.ring)-b.replay:]...)
	}
	for s := range b.subs {
		select {
		case s.ch <- e:
		default:
			b.dropped++
			s.closeLocked()
		}
	}
	return e, nil
}

// Subscribe registers a subscriber and returns the replay: every retained
// event with an ID greater than lastID (all retained events when lastID is
// 0). It refuses past the subscriber bound.
func (b *EventBus) Subscribe(lastID uint64) (*Subscription, []Event, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.subs) >= b.maxSubs {
		return nil, nil, ErrTooManySubscribers
	}
	s := &Subscription{bus: b, ch: make(chan Event, subscriberBuffer)}
	b.subs[s] = struct{}{}
	var replay []Event
	for _, e := range b.ring {
		if e.ID > lastID {
			replay = append(replay, e)
		}
	}
	return s, replay, nil
}

// Subscribers is the live subscriber count.
func (b *EventBus) Subscribers() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs)
}

// LastID is the most recently assigned event ID (0 before any publish).
func (b *EventBus) LastID() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.next
}

// sseData is the data: payload of one event — the event plus its panel
// source keys, still names and IDs only.
type sseData struct {
	Event
	Sources []string `json:"sources"`
}

// WriteSSE writes one event in text/event-stream framing.
func WriteSSE(w http.ResponseWriter, e Event) error {
	raw, err := json.Marshal(sseData{Event: e, Sources: e.Sources()})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.ID, e.Kind, raw)
	return err
}

// ServeHTTP streams the bus as SSE: a hello event (no ID, so it never moves
// a client's Last-Event-ID), the replay after Last-Event-ID (header, or the
// last_event_id query parameter for clients that cannot set headers), then
// live events and a ": keepalive" comment every keepalive interval. Past the
// subscriber bound it answers 503 with Retry-After instead of queueing.
func (b *EventBus) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	lastID := uint64(0)
	raw := r.Header.Get("Last-Event-ID")
	if raw == "" {
		raw = r.URL.Query().Get("last_event_id")
	}
	if raw != "" {
		if n, err := strconv.ParseUint(raw, 10, 64); err == nil {
			lastID = n
		}
	}
	sub, replay, err := b.Subscribe(lastID)
	if err != nil {
		w.Header().Set("Retry-After", "30")
		http.Error(w, "the event stream serves at most "+strconv.Itoa(b.maxSubs)+" subscribers; retry later", http.StatusServiceUnavailable)
		return
	}
	defer sub.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flush := func() {
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	fmt.Fprint(w, "event: hello\ndata: {\"server\":\"fathom\"}\n\n")
	for _, e := range replay {
		if WriteSSE(w, e) != nil {
			return
		}
	}
	flush()

	tick := time.NewTicker(b.keepalive)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e, ok := <-sub.C():
			if !ok {
				return // fell behind: the client reconnects with Last-Event-ID
			}
			if WriteSSE(w, e) != nil {
				return
			}
			flush()
		case <-tick.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flush()
		}
	}
}
