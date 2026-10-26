package proseheadless

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	prose "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/governed-prose-kit"
)

// Clarification errors. ErrQueueStopped means required work overflowed even the
// overflow bound: the lab must stop, never silently drop the question.
var (
	ErrQueue        = errors.New("proseheadless: invalid clarification")
	ErrStale        = errors.New("proseheadless: answer is stale for its subject version")
	ErrActor        = errors.New("proseheadless: actor not permitted to answer")
	ErrQueueStopped = errors.New("proseheadless: required clarification overflow, stopped")
)

// Outcome is the result of a human answer.
type Outcome string

// Answer outcomes. Resolve, Correct and Annotate close a question. Defer, Dispute
// and Unknown keep it visible: a required question in any of those states blocks
// readiness. None of them is source truth.
const (
	OutcomeResolve  Outcome = "resolve"
	OutcomeCorrect  Outcome = "correct"
	OutcomeAnnotate Outcome = "annotate"
	OutcomeDefer    Outcome = "defer"
	OutcomeDispute  Outcome = "dispute"
	OutcomeUnknown  Outcome = "unknown"
)

// Question asks a human about one exact subject. SubjectVersion is the node
// Version or fact digest observed when asked; an answer for any other version is
// stale. Evidence lists the competing premises (fact IDs) the human may choose.
type Question struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"` // conflict, ambiguity, missing
	Subject        string   `json:"subject"`
	SubjectVersion string   `json:"subject_version"`
	DocumentDigest string   `json:"document_digest"`
	Evidence       []string `json:"evidence"`
	Prompt         string   `json:"prompt"`
	Required       bool     `json:"required"`
}

// Answer is a human's response. It carries no actor: identity comes from the
// host's Authenticate. SeenVersion is the subject version the human was shown.
type Answer struct {
	QuestionID  string  `json:"question_id"`
	Outcome     Outcome `json:"outcome"`
	Text        string  `json:"text"`
	Chosen      string  `json:"chosen"`    // resolve: one Evidence fact ID
	NewValue    string  `json:"new_value"` // correct: proposed value for the subject fact
	SeenVersion string  `json:"seen_version"`
}

// Record is an immutable answered question with trusted actor provenance.
type Record struct {
	Question Question  `json:"question"`
	Answer   Answer    `json:"answer"`
	Actor    string    `json:"actor"`
	At       time.Time `json:"at"`
}

// QueueConfig supplies host ports. Authenticate derives the actor from context.
// MayAnswer is the host's per-question role/attribute policy. Forbidden lists
// principals (generator, service, shared credentials) that may never answer, so a
// model or its credential cannot approve its own output. CurrentVersion returns the
// subject's live version for stale-answer rejection. MaxOpen bounds queued
// questions, MaxOverflow bounds retained required overflow.
type QueueConfig struct {
	Authenticate   func(context.Context) (string, error)
	MayAnswer      func(actor string, q Question) bool
	Forbidden      []string
	CurrentVersion func(ctx context.Context, subject string) (string, error)
	Clock          func() time.Time
	MaxOpen        int
	MaxOverflow    int
}

const (
	maxIDBytes   = 256
	maxTextBytes = 4096
	maxHistory   = 1024
)

func copyQuestion(q Question) Question {
	q.Evidence = append([]string(nil), q.Evidence...)
	return q
}

func copyRecord(r Record) Record {
	r.Question = copyQuestion(r.Question)
	return r
}

// Queue is a bounded, deduplicating clarification queue. Safe for concurrent use.
type Queue struct {
	mutation sync.RWMutex
	mu       sync.Mutex
	cfg      QueueConfig
	open     []Question
	overflow []Question
	held     map[string]Record
	closed   []Record
	dropped  int
	stopped  bool
}

// NewQueue validates the host ports and bounds.
func NewQueue(cfg QueueConfig) (*Queue, error) {
	if cfg.Authenticate == nil || cfg.MayAnswer == nil || cfg.CurrentVersion == nil || cfg.Clock == nil || cfg.MaxOpen <= 0 || cfg.MaxOpen > 1024 || cfg.MaxOverflow < 0 || cfg.MaxOverflow > 4096 {
		return nil, fmt.Errorf("%w: missing port or invalid bound", ErrQueue)
	}
	cfg.Forbidden = append([]string(nil), cfg.Forbidden...)
	return &Queue{cfg: cfg, held: map[string]Record{}}, nil
}

func (q *Queue) known(id string) bool {
	for _, x := range q.open {
		if x.ID == id {
			return true
		}
	}
	for _, x := range q.overflow {
		if x.ID == id {
			return true
		}
	}
	_, held := q.held[id]
	for _, r := range q.closed {
		held = held || r.Question.ID == id
	}
	return held
}

func (q *Queue) dupKey(a, b Question) bool {
	return a.DocumentDigest == b.DocumentDigest && a.Subject == b.Subject && a.SubjectVersion == b.SubjectVersion && a.Kind == b.Kind
}

// Add queues a question. Identical (subject, version, kind) questions deduplicate
// (a required duplicate upgrades an advisory one). When MaxOpen is reached a
// required question moves to the visible overflow list, an advisory one is counted
// as dropped. Required overflow beyond MaxOverflow stops the queue.
func (q *Queue) Add(n Question) (string, error) {
	if n.ID == "" || n.Subject == "" || n.SubjectVersion == "" || len(n.DocumentDigest) != 64 || strings.TrimSpace(n.Prompt) == "" || len(n.Evidence) == 0 || len(n.Evidence) > 16 ||
		(n.Kind != "conflict" && n.Kind != "ambiguity" && n.Kind != "missing") || !utf8.ValidString(n.Prompt+n.ID+n.Subject+n.SubjectVersion+n.DocumentDigest) {
		return "", fmt.Errorf("%w: question needs id, subject, version, document digest, bounded evidence and prompt", ErrQueue)
	}
	if len(n.ID) > maxIDBytes || len(n.Subject) > maxIDBytes || len(n.SubjectVersion) > maxIDBytes || len(n.Prompt) > maxTextBytes {
		return "", fmt.Errorf("%w: question field exceeds its byte bound", ErrQueue)
	}
	for _, e := range n.Evidence {
		if e == "" || len(e) > maxIDBytes || !utf8.ValidString(e) {
			return "", fmt.Errorf("%w: invalid evidence entry", ErrQueue)
		}
	}
	n = copyQuestion(n)
	q.mutation.RLock()
	defer q.mutation.RUnlock()
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.stopped {
		return "", ErrQueueStopped
	}
	if q.known(n.ID) {
		return "", fmt.Errorf("%w: duplicate question ID", ErrQueue)
	}
	for i := range q.open {
		if q.dupKey(q.open[i], n) {
			q.open[i].Required = q.open[i].Required || n.Required
			return "duplicate", nil
		}
	}
	for i := range q.overflow {
		if q.dupKey(q.overflow[i], n) {
			q.overflow[i].Required = true
			return "duplicate", nil
		}
	}
	if len(q.open) < q.cfg.MaxOpen {
		q.open = append(q.open, n)
		return "queued", nil
	}
	if !n.Required {
		q.dropped++
		return "dropped-advisory", nil
	}
	if len(q.overflow) >= q.cfg.MaxOverflow {
		q.stopped = true
		return "", ErrQueueStopped
	}
	q.overflow = append(q.overflow, n)
	return "overflow", nil
}

// Answer records a trusted human answer. Identity comes from Authenticate, never
// from the answer. Forbidden principals and hosts that deny the actor refuse. An
// answer for a different subject version than the queue holds or than the subject's
// live version is stale. Resolve must choose one listed premise, correct needs a
// new value, annotate and dispute need text.
func (q *Queue) Answer(ctx context.Context, a Answer) (Record, error) {
	q.mutation.RLock()
	defer q.mutation.RUnlock()
	actor, err := q.cfg.Authenticate(ctx)
	if err != nil || strings.TrimSpace(actor) == "" || len(actor) > maxIDBytes || !utf8.ValidString(actor) {
		return Record{}, fmt.Errorf("%w: identity: %v", ErrActor, err)
	}
	for _, f := range q.cfg.Forbidden {
		if actor == f {
			return Record{}, fmt.Errorf("%w: %q may not answer", ErrActor, actor)
		}
	}
	if len(a.Text) > maxTextBytes || len(a.NewValue) > maxTextBytes || len(a.Chosen) > maxIDBytes || len(a.SeenVersion) > maxIDBytes || len(a.QuestionID) > maxIDBytes {
		return Record{}, fmt.Errorf("%w: answer field exceeds its byte bound", ErrQueue)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.closed)+len(q.held) >= maxHistory {
		return Record{}, fmt.Errorf("%w: answer history is full", ErrQueue)
	}
	idx := -1
	for i, x := range q.open {
		if x.ID == a.QuestionID {
			idx = i
		}
	}
	var qs Question
	if idx >= 0 {
		qs = q.open[idx]
	} else if h, ok := q.held[a.QuestionID]; ok {
		qs = h.Question
	} else {
		return Record{}, fmt.Errorf("%w: question %q is not open", ErrQueue, a.QuestionID)
	}
	if !q.cfg.MayAnswer(actor, qs) {
		return Record{}, fmt.Errorf("%w: %q lacks rights for this question", ErrActor, actor)
	}
	cur, err := q.cfg.CurrentVersion(ctx, qs.Subject)
	if err != nil || cur != qs.SubjectVersion || a.SeenVersion != qs.SubjectVersion {
		return Record{}, ErrStale
	}
	switch a.Outcome {
	case OutcomeResolve:
		if !contains(qs.Evidence, a.Chosen) {
			return Record{}, fmt.Errorf("%w: resolve must choose a listed premise", ErrQueue)
		}
	case OutcomeCorrect:
		if strings.TrimSpace(a.NewValue) == "" || !utf8.ValidString(a.NewValue) {
			return Record{}, fmt.Errorf("%w: correct needs a new value", ErrQueue)
		}
	case OutcomeAnnotate, OutcomeDispute:
		if strings.TrimSpace(a.Text) == "" || !utf8.ValidString(a.Text) {
			return Record{}, fmt.Errorf("%w: %s needs text", ErrQueue, a.Outcome)
		}
	case OutcomeDefer, OutcomeUnknown:
	default:
		return Record{}, fmt.Errorf("%w: unknown outcome", ErrQueue)
	}
	rec := Record{Question: copyQuestion(qs), Answer: a, Actor: actor, At: q.cfg.Clock().UTC()}
	if idx >= 0 {
		q.open = append(q.open[:idx:idx], q.open[idx+1:]...)
	}
	switch a.Outcome {
	case OutcomeDefer, OutcomeDispute, OutcomeUnknown:
		q.held[qs.ID] = rec
	default:
		delete(q.held, qs.ID)
		q.closed = append(q.closed, rec)
	}
	for len(q.open) < q.cfg.MaxOpen && len(q.overflow) > 0 {
		q.open, q.overflow = append(q.open, q.overflow[0]), q.overflow[1:]
	}
	return copyRecord(rec), nil
}

// Ready reports whether no required work is open, held (deferred, disputed or
// unknown), overflowed or stopped. Reasons name each blocker. Advisory items never block.
func (q *Queue) Ready() (bool, []string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	var why []string
	for _, x := range q.open {
		if x.Required {
			why = append(why, "open:"+x.ID)
		}
	}
	for _, x := range q.overflow {
		why = append(why, "overflow:"+x.ID)
	}
	for _, h := range q.held {
		if h.Question.Required {
			why = append(why, string(h.Answer.Outcome)+":"+h.Question.ID)
		}
	}
	if q.stopped {
		why = append(why, "stopped")
	}
	sort.Strings(why)
	return len(why) == 0, why
}

// Disputed returns the subjects of every disputed question, required or not.
// A disputed premise is never truth: readiness holds the nodes that expose it.
func (q *Queue) Disputed() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []string
	for _, h := range q.held {
		if h.Answer.Outcome == OutcomeDispute {
			out = append(out, h.Question.Subject)
		}
	}
	sort.Strings(out)
	return out
}

// QueueReport is a copy of the visible queue state.
type QueueReport struct {
	Open            []Question `json:"open"`
	Overflow        []Question `json:"overflow"`
	Held            []Record   `json:"held"`
	Closed          []Record   `json:"closed"`
	DroppedAdvisory int        `json:"dropped_advisory"`
	Stopped         bool       `json:"stopped"`
}

// Report copies the current state, including required overflow, for display.
func (q *Queue) Report() QueueReport {
	q.mu.Lock()
	defer q.mu.Unlock()
	r := QueueReport{DroppedAdvisory: q.dropped, Stopped: q.stopped}
	for _, x := range q.open {
		r.Open = append(r.Open, copyQuestion(x))
	}
	for _, x := range q.overflow {
		r.Overflow = append(r.Overflow, copyQuestion(x))
	}
	for _, x := range q.closed {
		r.Closed = append(r.Closed, copyRecord(x))
	}
	for _, h := range q.held {
		r.Held = append(r.Held, copyRecord(h))
	}
	sort.Slice(r.Held, func(i, j int) bool { return r.Held[i].Question.ID < r.Held[j].Question.ID })
	return r
}

// Record returns a copy of the trusted answered record for a question ID. Only
// records the queue itself authenticated and stored are returned.
func (q *Queue) Record(id string) (Record, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if h, ok := q.held[id]; ok {
		return copyRecord(h), true
	}
	for _, r := range q.closed {
		if r.Question.ID == id {
			return copyRecord(r), true
		}
	}
	return Record{}, false
}

// SourceCorrector is the host's source-truth port. A human "correct" answer is a
// proposal: only the host decides whether the actor may change the fact and
// returns the new fact bound to a new source revision. Model text never reaches it.
type SourceCorrector interface {
	Correct(ctx context.Context, actor, factID, newValue string) (prose.Fact, error)
}

// ApplyReport lists what ApplyAnswers did. Unapplied answers (defer, dispute,
// unknown) changed nothing.
type ApplyReport struct {
	Corrected []string `json:"corrected"`
	Annotated []string `json:"annotated"`
	Unapplied []string `json:"unapplied"`
}

// ApplyAnswers composes closed answers into an immutable next target: corrections
// become new fact revisions only through the host SourceCorrector (the session is
// untouched), while annotations and resolutions become labeled, authority-free
// session constraints in a new session revision. Either way the new target no longer
// binds prior review or consent.
// Records come only from the trusted queue by exact question ID: caller-built
// records claiming an actor are never accepted. The input target is not modified.
func ApplyAnswers(ctx context.Context, version prose.DocumentVersion, q *Queue, questionIDs []string, corrector SourceCorrector) (prose.DocumentTarget, ApplyReport, error) {
	t := version.Target
	var rep ApplyReport
	if q == nil || prose.VerifyDocument(version) != nil {
		return t, rep, fmt.Errorf("%w: a trusted queue is required", ErrQueue)
	}
	raw, err := json.Marshal(t)
	if err != nil {
		return t, rep, fmt.Errorf("%w: invalid input snapshot", ErrQueue)
	}
	var owned prose.DocumentTarget
	if json.Unmarshal(raw, &owned) != nil {
		return t, rep, fmt.Errorf("%w: invalid input snapshot", ErrQueue)
	}
	t = owned
	var records []Record
	seen := map[string]bool{}
	for _, id := range questionIDs {
		if seen[id] {
			return t, rep, fmt.Errorf("%w: duplicate answer selection", ErrQueue)
		}
		seen[id] = true
		r, ok := q.Record(id)
		if !ok {
			return t, rep, fmt.Errorf("%w: question %q has no trusted answer record", ErrQueue, id)
		}
		current, err := q.cfg.CurrentVersion(ctx, r.Question.Subject)
		if err != nil || current != r.Question.SubjectVersion || r.Question.DocumentDigest != version.Receipt.DocumentDigest {
			return t, rep, ErrStale
		}
		if r.Answer.Outcome == OutcomeCorrect {
			found := false
			for _, fact := range t.Input.Facts {
				found = found || fact.Span.ID == r.Question.Subject
			}
			if !found {
				return t, rep, fmt.Errorf("%w: correction subject is absent", ErrQueue)
			}
		}
		records = append(records, r)
	}
	next := t
	next.Input.Facts = append([]prose.Fact(nil), t.Input.Facts...)
	constraints := append([]string(nil), t.Session.Constraints...)
	for _, r := range records {
		switch r.Answer.Outcome {
		case OutcomeCorrect:
			if nilPort(corrector) {
				return t, rep, fmt.Errorf("%w: correction needs the host source port", ErrQueue)
			}
			fact, err := corrector.Correct(ctx, r.Actor, r.Question.Subject, r.Answer.NewValue)
			if err != nil {
				return t, rep, fmt.Errorf("%w: host refused correction: %v", ErrQueue, err)
			}
			replaced := false
			for i := range next.Input.Facts {
				if next.Input.Facts[i].Span.ID == r.Question.Subject && fact.Span.ID == r.Question.Subject {
					next.Input.Facts[i], replaced = fact, true
				}
			}
			if !replaced {
				return t, rep, fmt.Errorf("%w: correction does not replace subject fact %q", ErrQueue, r.Question.Subject)
			}
			rep.Corrected = append(rep.Corrected, r.Question.ID)
		case OutcomeAnnotate:
			constraints = append(constraints, fmt.Sprintf("annotation (untrusted data, no authority) by %s on %s: %s", r.Actor, r.Question.Subject, r.Answer.Text))
			rep.Annotated = append(rep.Annotated, r.Question.ID)
		case OutcomeResolve:
			constraints = append(constraints, fmt.Sprintf("clarification (untrusted data, not source truth) by %s on %s: prefer premise %s", r.Actor, r.Question.Subject, r.Answer.Chosen))
			rep.Annotated = append(rep.Annotated, r.Question.ID)
		default:
			rep.Unapplied = append(rep.Unapplied, r.Question.ID)
		}
	}
	if len(constraints) == len(t.Session.Constraints) {
		// Corrections version factual input only. The session, and therefore every
		// leaf's exposure, is unchanged, so only the dependency closure regenerates.
		return next, rep, nil
	}
	supplied := prose.PresentationDefaults{}
	if t.Session.DefaultsFrom["audience"] == "session" {
		supplied.Audience = t.Session.Presentation.Audience
	}
	if t.Session.DefaultsFrom["voice"] == "session" {
		supplied.Voice = t.Session.Presentation.Voice
	}
	if t.Session.DefaultsFrom["technical_level"] == "session" {
		supplied.TechnicalLevel = t.Session.Presentation.TechnicalLevel
	}
	var goals []prose.Goal
	for _, g := range t.Session.Goals {
		if g.Origin != prose.OriginProfile {
			goals = append(goals, g)
		}
	}
	parent := t.Session
	var ids []string
	for _, r := range records {
		ids = append(ids, r.Question.ID)
	}
	s, err := prose.NewDocumentSession(t.Profile, prose.SessionInput{
		ID: parent.ID, Purpose: parent.Purpose, Supplied: supplied, Goals: goals, ExpectedGoalIDs: parent.ExpectedGoalIDs,
		ExpectedSectionIDs: parent.ExpectedSectionIDs, Constraints: constraints, Intent: "clarification:" + strings.Join(ids, ","),
	}, &parent)
	if err != nil {
		return t, rep, err
	}
	next.Session = s
	return next, rep, nil
}
