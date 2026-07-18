// Package heartbeat records channel beats — the liveness substrate for
// forced delivery channels (hooks, gates, instruments). Activity signals
// ("edit-seen") land beside effect signals ("injection-emitted"), so coverage
// has a denominator and a silent channel is detectable instead of invisible.
//
// Motivating failure (fathom, 2026-07): the flagship PostToolUse injection
// channel produced zero events for four days and nothing noticed, because the
// call log recorded effects only — "no sessions ran" and "the hook died" were
// indistinguishable.
//
// Shape contract: schema.cue is authoritative; the Go types mirror it.
//
// Honest scope: beats are SIGNALS and liveness alarms are advisory data.
// CheckLiveness never exits a process, blocks a hook, or gates a push —
// consumers decide what an alarm means.
//
// Concurrency: a *Log serializes its own writes with an internal mutex and is
// safe for concurrent Beat calls; Load and the projections are pure functions
// over value slices and need no synchronization.
package heartbeat

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"
)

// Beat is one channel firing. Field order is the byte-stability contract for
// the JSONL encoding — do not reorder.
type Beat struct {
	// Channel names the instrumented channel ("edit-hook", "session-hook").
	Channel string `json:"channel"`
	// Kind refines the event within the channel ("seen", "emitted", "empty",
	// "error"). Empty means the channel's default event.
	Kind string `json:"kind,omitempty"`
	// At is the beat timestamp, always UTC (stamped by the Log's injected
	// clock — projections never read wall time).
	At time.Time `json:"at"`
	// Meta carries small string labels (service, file class …). Label-sized
	// values only — beats are counters, not payloads.
	Meta map[string]string `json:"meta,omitempty"`
}

// Log appends beats to w as one JSON object per line.
type Log struct {
	mu  sync.Mutex
	w   io.Writer
	now func() time.Time
}

// NewLog returns a Log writing to w, stamping beats with the injected clock.
func NewLog(w io.Writer, now func() time.Time) (*Log, error) {
	if w == nil {
		return nil, errors.New("heartbeat: writer must not be nil")
	}
	if now == nil {
		return nil, errors.New("heartbeat: clock must not be nil (inject it — no wall-time defaults)")
	}
	return &Log{w: w, now: now}, nil
}

// Beat appends one beat. The timestamp is the Log's clock in UTC.
func (l *Log) Beat(channel, kind string, meta map[string]string) error {
	if channel == "" {
		return errors.New("heartbeat: channel must not be empty")
	}
	b := Beat{Channel: channel, Kind: kind, At: l.now().UTC(), Meta: meta}
	line, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("heartbeat: marshal beat: %w", err)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := l.w.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("heartbeat: write beat: %w", err)
	}
	return nil
}

// Load reads beats from r (one JSON object per line, blank lines skipped).
// With acceptPartial, undecodable lines are dropped instead of failing the
// load — a torn tail write must not make the whole history unreadable.
func Load(r io.Reader, acceptPartial bool) ([]Beat, error) {
	var out []Beat
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	line := 0
	for sc.Scan() {
		line++
		raw := sc.Bytes()
		if len(raw) == 0 {
			continue
		}
		var b Beat
		if err := json.Unmarshal(raw, &b); err != nil {
			if acceptPartial {
				continue
			}
			return nil, fmt.Errorf("heartbeat: line %d: %w", line, err)
		}
		out = append(out, b)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("heartbeat: scan: %w", err)
	}
	return out, nil
}

// DayCount is one (day, channel, kind) tally.
type DayCount struct {
	Day     string // "2006-01-02", UTC
	Channel string
	Kind    string
	N       int
}

// Rollup tallies beats per (UTC day, channel, kind), sorted by (Day, Channel,
// Kind). Pure and deterministic.
func Rollup(beats []Beat) []DayCount {
	type key struct{ day, ch, kind string }
	counts := map[key]int{}
	for _, b := range beats {
		counts[key{b.At.UTC().Format("2006-01-02"), b.Channel, b.Kind}]++
	}
	out := make([]DayCount, 0, len(counts))
	for k, n := range counts {
		out = append(out, DayCount{Day: k.day, Channel: k.ch, Kind: k.kind, N: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Day != out[j].Day {
			return out[i].Day < out[j].Day
		}
		if out[i].Channel != out[j].Channel {
			return out[i].Channel < out[j].Channel
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

// Expectation pairs an activity channel with the effect channel that must
// accompany it: "if IfChannel beat within Within of the reference time,
// ThenChannel must have beat in that window too." Channel matching uses
// "channel" or "channel.kind" — an Expectation channel containing a dot
// matches Channel+"."+Kind; otherwise it matches Channel alone.
type Expectation struct {
	IfChannel   string
	ThenChannel string
	Within      time.Duration
}

// Alarm reports one violated Expectation.
type Alarm struct {
	Expectation Expectation
	// LastIf is the newest matching IfChannel beat inside the window.
	LastIf time.Time
	// LastThen is the newest ThenChannel beat inside the window; zero when
	// the channel never beat in the window (the alarm condition).
	LastThen time.Time
	Detail   string
}

// CheckLiveness evaluates exps against beats at reference time ref: an
// Expectation alarms iff some IfChannel beat lies in [ref-Within, ref] and no
// ThenChannel beat does. Pure — ref is caller-supplied, never wall time.
// Alarms come out in exps order.
func CheckLiveness(beats []Beat, exps []Expectation, ref time.Time) []Alarm {
	newest := func(name string, since time.Time) time.Time {
		var last time.Time
		for _, b := range beats {
			if b.At.Before(since) || b.At.After(ref) {
				continue
			}
			id := b.Channel
			if b.Kind != "" {
				id = b.Channel + "." + b.Kind
			}
			if id == name || b.Channel == name {
				if b.At.After(last) {
					last = b.At
				}
			}
		}
		return last
	}
	var alarms []Alarm
	for _, e := range exps {
		if e.IfChannel == "" || e.ThenChannel == "" || e.Within <= 0 {
			continue // malformed expectations are skipped, not alarmed
		}
		since := ref.Add(-e.Within)
		lastIf := newest(e.IfChannel, since)
		if lastIf.IsZero() {
			continue // no activity in the window — nothing owed
		}
		lastThen := newest(e.ThenChannel, since)
		if lastThen.IsZero() {
			alarms = append(alarms, Alarm{
				Expectation: e,
				LastIf:      lastIf,
				Detail: fmt.Sprintf("%s beat at %s but %s is silent in the %s window ending %s",
					e.IfChannel, lastIf.UTC().Format(time.RFC3339), e.ThenChannel,
					e.Within, ref.UTC().Format(time.RFC3339)),
			})
		}
	}
	return alarms
}
