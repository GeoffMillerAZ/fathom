package decision

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// TranscriptVersion is the Transcript format this package writes and the
// only one NewReplay accepts.
const TranscriptVersion = 1

// ErrNotRecorded is returned (wrapped) by Replay.Decide for a Request its
// transcript holds no entry for.
var ErrNotRecorded = errors.New("decision: request not recorded in the transcript")

// Transcript is a recording of one Decider's Decide calls: every Request
// with the Response and error it got. It is plain data, JSON-serializable,
// so a test or an offline run can keep it as a file. Entries are in a
// stable order, by Key and then Seq, whatever order the calls ran in.
type Transcript struct {
	// Version is the format version, TranscriptVersion.
	Version int `json:"version"`
	// Decider is the recorded Decider's Name.
	Decider string `json:"decider"`
	// Capabilities is the recorded Decider's declaration (CapabilitiesOf)
	// when the recording started.
	Capabilities Capabilities `json:"capabilities"`
	// Entries are the recorded calls, sorted by Key, then Seq.
	Entries []TranscriptEntry `json:"entries"`
}

// TranscriptEntry is one recorded Decide call.
type TranscriptEntry struct {
	// Key is RequestKey(Request).
	Key string `json:"key"`
	// Seq is the call's occurrence among the calls with the same Key,
	// counting from 0 in call order.
	Seq int `json:"seq"`
	// Request is the Request the Decider was asked.
	Request Request `json:"request"`
	// Response is the Response it returned, even alongside an error.
	Response Response `json:"response"`
	// Err is the error it returned, nil when none.
	Err *RecordedError `json:"error,omitempty"`
}

// RecordedError is an error as a Transcript keeps it: its message, and
// whether it was ErrNoProvider, the one error identity a replay preserves
// (errors.Is(err, ErrNoProvider) holds on the replayed error). Any other
// error replays as its message only.
type RecordedError struct {
	// Message is the recorded error's Error().
	Message string `json:"message"`
	// NoProvider is true when the recorded error wrapped ErrNoProvider.
	NoProvider bool `json:"no_provider,omitempty"`
}

// Error returns the recorded message.
func (e *RecordedError) Error() string { return e.Message }

// Is reports whether target is ErrNoProvider and the recorded error was
// one, so errors.Is sees an abstention through a replay.
func (e *RecordedError) Is(target error) bool { return e.NoProvider && target == ErrNoProvider }

// RequestKey returns "sha256:" and the hex SHA-256 of a canonical encoding
// of req: its State and every Question in order, with the ID and every
// template field (see TemplateDigest; the alias "noul" is folded into
// KindPredicate, so it does not change the key).
func RequestKey(req Request) string {
	h := sha256.New()
	writeField(h, "decision.request/1")
	writeField(h, req.State)
	writeField(h, strconv.Itoa(len(req.Questions)))
	for _, q := range req.Questions {
		writeField(h, q.ID)
		writeTemplate(h, q)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// RecorderOptions configures NewRecorder. It is plain data, so a composer
// can build it from configuration.
type RecorderOptions struct {
	// Name is what the Recorder's Name returns. Empty means "recorder".
	Name string
}

// Recorder is a Decider that passes every call to its inner Decider,
// returns the inner Response and error unchanged, and appends the pair to
// a Transcript. It adds no provider call and changes no answer.
//
// Concurrency: Recorder guards its transcript with a mutex and is safe for
// concurrent use when its inner Decider is. Concurrent calls with the same
// Request get Seq numbers in the order they finish.
type Recorder struct {
	name  string
	inner Decider
	caps  Capabilities

	mu      sync.Mutex
	entries []TranscriptEntry
	seqs    map[string]int
}

// NewRecorder returns a Recorder over inner. It fails when inner is nil.
func NewRecorder(inner Decider, opts RecorderOptions) (*Recorder, error) {
	if isNilDecider(inner) {
		return nil, errors.New("decision: NewRecorder: inner Decider is nil")
	}
	name := opts.Name
	if name == "" {
		name = "recorder"
	}
	return &Recorder{name: name, inner: inner, caps: CapabilitiesOf(inner), seqs: map[string]int{}}, nil
}

// Name returns RecorderOptions.Name, or "recorder".
func (r *Recorder) Name() string { return r.name }

// Capabilities declares the inner Decider's Capabilities unchanged. It
// makes no call.
func (r *Recorder) Capabilities() Capabilities { return CapabilitiesOf(r.inner) }

// Decide calls the inner Decider and records the Request, Response and
// error before returning the Response and error unchanged.
func (r *Recorder) Decide(ctx context.Context, req Request) (Response, error) {
	resp, err := r.inner.Decide(ctx, req)
	entry := TranscriptEntry{Key: RequestKey(req), Request: cloneRequest(req), Response: cloneResponse(resp)}
	if err != nil {
		entry.Err = &RecordedError{Message: err.Error(), NoProvider: errors.Is(err, ErrNoProvider)}
	}
	r.mu.Lock()
	entry.Seq = r.seqs[entry.Key]
	r.seqs[entry.Key]++
	r.entries = append(r.entries, entry)
	r.mu.Unlock()
	return resp, err
}

// Transcript returns a deep copy of everything recorded so far, entries
// sorted by Key, then Seq.
func (r *Recorder) Transcript() Transcript {
	r.mu.Lock()
	entries := make([]TranscriptEntry, len(r.entries))
	for i, e := range r.entries {
		entries[i] = cloneEntry(e)
	}
	r.mu.Unlock()
	slices.SortFunc(entries, func(a, b TranscriptEntry) int {
		if c := strings.Compare(a.Key, b.Key); c != 0 {
			return c
		}
		return a.Seq - b.Seq
	})
	caps := r.caps
	caps.Kinds = slices.Clone(caps.Kinds)
	return Transcript{Version: TranscriptVersion, Decider: r.inner.Name(), Capabilities: caps, Entries: entries}
}

// ReplayOptions configures NewReplay. It is plain data, so a composer can
// build it from configuration.
type ReplayOptions struct {
	// Name is what the Replay's Name returns. Empty means "replay".
	Name string
}

// Replay is a Decider that answers from a Transcript and never calls a
// provider: a Request whose RequestKey the transcript holds gets the
// recorded Response and error, the recorded occurrences in Seq order and
// then the last one again; any other Request gets an error wrapping
// ErrNotRecorded. The replayed Response is the recorded one, identical,
// so its Provider and Model name the Decider that was recorded; the
// Replay itself is named by Name and declares no latency and no cost.
//
// Concurrency: Replay guards its per-Request position with a mutex and is
// safe for concurrent use. Concurrent calls with the same Request take the
// recorded occurrences in the order they arrive.
type Replay struct {
	name string
	caps Capabilities

	mu     sync.Mutex
	byKey  map[string][]TranscriptEntry
	cursor map[string]int
}

// NewReplay returns a Replay over t. It fails when t.Version is not
// TranscriptVersion, an entry's Key is not RequestKey of its Request (the
// transcript was edited or written by something else), or two entries
// share a Key and a Seq.
func NewReplay(t Transcript, opts ReplayOptions) (*Replay, error) {
	if t.Version != TranscriptVersion {
		return nil, fmt.Errorf("decision: NewReplay: transcript version %d, want %d", t.Version, TranscriptVersion)
	}
	byKey := map[string][]TranscriptEntry{}
	for i, e := range t.Entries {
		if got := RequestKey(e.Request); got != e.Key {
			return nil, fmt.Errorf("decision: NewReplay: entry %d: key %q is not its request's key %q", i, e.Key, got)
		}
		for _, prev := range byKey[e.Key] {
			if prev.Seq == e.Seq {
				return nil, fmt.Errorf("decision: NewReplay: entry %d: key %q seq %d recorded twice", i, e.Key, e.Seq)
			}
		}
		byKey[e.Key] = append(byKey[e.Key], cloneEntry(e))
	}
	for _, es := range byKey {
		slices.SortFunc(es, func(a, b TranscriptEntry) int { return a.Seq - b.Seq })
	}
	caps := undeclaredCapabilities()
	if t.Capabilities.Declared {
		caps = t.Capabilities
		caps.Kinds = canonicalKinds(caps.Kinds)
		caps.Latency = LatencyNone
		caps.Cost = CostNone
	}
	name := opts.Name
	if name == "" {
		name = "replay"
	}
	return &Replay{name: name, caps: caps, byKey: byKey, cursor: map[string]int{}}, nil
}

// Name returns ReplayOptions.Name, or "replay".
func (p *Replay) Name() string { return p.name }

// Capabilities declares the recorded Decider's Capabilities with Latency
// LatencyNone and Cost CostNone (a replay makes no round trip and bills
// nothing), or undeclared when the recording was. It makes no call.
func (p *Replay) Capabilities() Capabilities {
	c := p.caps
	c.Kinds = slices.Clone(c.Kinds)
	return c
}

// Decide returns a copy of the recorded Response and error for req (see
// Replay), or an error wrapping ErrNotRecorded.
func (p *Replay) Decide(_ context.Context, req Request) (Response, error) {
	key := RequestKey(req)
	p.mu.Lock()
	es := p.byKey[key]
	if len(es) == 0 {
		p.mu.Unlock()
		return Response{}, fmt.Errorf("%w: key %s", ErrNotRecorded, key)
	}
	i := min(p.cursor[key], len(es)-1)
	p.cursor[key]++
	e := es[i]
	p.mu.Unlock()
	resp := cloneResponse(e.Response)
	if e.Err != nil {
		return resp, &RecordedError{Message: e.Err.Message, NoProvider: e.Err.NoProvider}
	}
	return resp, nil
}

// cloneEntry returns a deep copy of e.
func cloneEntry(e TranscriptEntry) TranscriptEntry {
	e.Request = cloneRequest(e.Request)
	e.Response = cloneResponse(e.Response)
	if e.Err != nil {
		errCopy := *e.Err
		e.Err = &errCopy
	}
	return e
}

// cloneRequest returns a deep copy of req, keeping nil slices nil.
func cloneRequest(req Request) Request {
	if req.Questions == nil {
		return req
	}
	qs := make([]Question, len(req.Questions))
	for i, q := range req.Questions {
		q.Options = slices.Clone(q.Options)
		q.Levels = slices.Clone(q.Levels)
		qs[i] = q
	}
	req.Questions = qs
	return req
}

// cloneResponse returns a deep copy of resp, keeping nil maps nil.
func cloneResponse(resp Response) Response {
	if resp.Answers != nil {
		answers := make(map[string]Answer, len(resp.Answers))
		for id, a := range resp.Answers {
			answers[id] = cloneAnswer(a)
		}
		resp.Answers = answers
	}
	resp.Invalid = maps.Clone(resp.Invalid)
	return resp
}
