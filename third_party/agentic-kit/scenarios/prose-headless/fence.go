package proseheadless

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	fitness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/fitness-function-registry"
	prose "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/governed-prose-kit"
)

// FenceSynthetic is the only supported fence qualification: an in-memory host
// authority whose state changes only through its own lock. Any other value, or no
// fence, is unsupported and holds. It does not fence independently mutable
// external systems: a source provider, queue or truth store is fenced only for
// mutations routed through MemoryFence.Mutate or its setters.
const FenceSynthetic = "synthetic-in-memory/v1"

// Scope is the exact authority scope a fence must find current.
type Scope struct {
	DocumentDigest string
	SessionDigest  string
	ProfileDigest  string
	EvidenceDigest string
	PolicyEpoch    string
	Tenant         string
	Purpose        string
	Destination    string
	Account        string
	Actor          string
}

// Fence is trusted host authority, never a model attestation. Enter validates every
// condition for the scope and keeps them valid until release is called. The caller
// performs its effect (local apply, prepare, dispatch) while the fence is held and
// must not call Enter or any mutator of the same fence from inside.
type Fence interface {
	Qualification() string
	Enter(ctx context.Context, s Scope, doc prose.DocumentVersion) (release func(), err error)
}

// HoldError lists every unmet condition and satisfies errors.Is(err, ErrHold).
type HoldError struct{ Reasons []string }

func (e *HoldError) Error() string { return ErrHold.Error() + ": " + strings.Join(e.Reasons, "; ") }

// Is matches ErrHold.
func (e *HoldError) Is(t error) bool { return t == ErrHold }

// MemoryFenceConfig supplies the synthetic authority. Engine re-resolves sources.
// Queue is required. ExpectedRequired is the independent required-goal roster.
type MemoryFenceConfig struct {
	Engine           *prose.DocumentEngine
	Queue            *Queue
	Assessment       AssessmentConfig // PolicyEpoch is taken from the scope
	ExpectedRequired []string
}

// MemoryFence is the selected synthetic fence. Lock order: fence, then queue or
// source ports, then HITL store. Safe for concurrent use.
type MemoryFence struct {
	mu    sync.Mutex
	cfg   MemoryFenceConfig
	truth map[string]TruthStatus
	obs   []CriterionObservation
	tasks map[string]bool
	// AfterEnter, if set, runs while the fence is held after validation (test barrier).
	AfterEnter func()
}

// TruthStatus is the host source-authority verdict for one fact. Only TruthConfirmed
// passes; absent entries are unknown and hold.
type TruthStatus string

// Truth statuses.
const (
	TruthConfirmed  TruthStatus = "confirmed"
	TruthDisputed   TruthStatus = "disputed"
	TruthSuperseded TruthStatus = "superseded"
	TruthUnknown    TruthStatus = "unknown"
)

func nilPort(v any) bool {
	if v == nil {
		return true
	}
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
		return r.IsNil()
	}
	return false
}

// NewMemoryFence validates required ports and the independent goal roster.
func NewMemoryFence(cfg MemoryFenceConfig) (*MemoryFence, error) {
	if nilPort(cfg.Engine) || nilPort(cfg.Queue) || cfg.Assessment.Now == nil || len(cfg.ExpectedRequired) == 0 {
		return nil, fmt.Errorf("%w: engine, queue, assessment clock and expected goal roster required", ErrPublish)
	}
	cfg.ExpectedRequired = append([]string(nil), cfg.ExpectedRequired...)
	methods := make(map[string][]fitness.EvidenceMethod, len(cfg.Assessment.Methods))
	for key, values := range cfg.Assessment.Methods {
		methods[key] = append([]fitness.EvidenceMethod(nil), values...)
	}
	cfg.Assessment.Methods = methods
	return &MemoryFence{cfg: cfg, truth: map[string]TruthStatus{}, tasks: map[string]bool{}}, nil
}

// Qualification names the supported scope.
func (f *MemoryFence) Qualification() string { return FenceSynthetic }

// Mutate runs fn under the fence lock so authority changes cannot interleave with a
// held fence. fn must not call Enter or other fence methods.
func (f *MemoryFence) Mutate(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn()
}

// SetTruth records the host verdict for a fact.
func (f *MemoryFence) SetTruth(factID string, s TruthStatus) {
	f.Mutate(func() { f.truth[factID] = s })
}

// SetTask replaces the host-attested task purposes.
func (f *MemoryFence) SetTask(purposes []string) {
	f.Mutate(func() {
		f.tasks = map[string]bool{}
		for _, p := range purposes {
			f.tasks[p] = true
		}
	})
}

// SetObservations replaces the raw host/human criterion observations (deep copied).
func (f *MemoryFence) SetObservations(o []CriterionObservation) {
	f.Mutate(func() {
		f.obs = make([]CriterionObservation, len(o))
		for i := range o {
			f.obs[i] = o[i]
			f.obs[i].ArtifactDigests = append([]string(nil), o[i].ArtifactDigests...)
		}
	})
}

// exposedFacts returns every fact a node was exposed to, cited or not.
func exposedFacts(doc prose.DocumentVersion) []string {
	seen := map[string]bool{}
	for _, n := range doc.Nodes {
		for _, manifest := range []*prose.ReadManifest{n.GenManifest, n.ReviewMan} {
			if manifest != nil {
				for _, fact := range manifest.Facts {
					seen[fact.ID] = true
				}
			}
		}
		for _, c := range n.Claims {
			for _, id := range append(append([]string(nil), c.FactIDs...), c.Contradictions...) {
				seen[id] = true
			}
		}
	}
	facts := make(map[string]prose.Fact, len(doc.Target.Input.Facts))
	for _, fact := range doc.Target.Input.Facts {
		facts[fact.Span.ID] = fact
	}
	visited := map[string]bool{}
	var visit func(string)
	visit = func(id string) {
		if visited[id] {
			return
		}
		visited[id], seen[id] = true, true
		if fact, ok := facts[id]; ok && fact.Derivation != nil {
			for _, input := range fact.Derivation.Inputs {
				visit(input)
			}
		}
	}
	for id := range seen {
		visit(id)
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Enter validates scope, source currency, truth of every exposed fact, clarification,
// task purpose and Assignment Kit goal assessment, and holds the fence until release.
func (f *MemoryFence) Enter(ctx context.Context, s Scope, doc prose.DocumentVersion) (func(), error) {
	f.mu.Lock()
	// Queue writers also take this gate, including direct Add and Answer calls.
	f.cfg.Queue.mutation.Lock()
	var holds []string
	d := doc.Receipt
	switch {
	case ctx.Err() != nil:
		holds = append(holds, "context:"+ctx.Err().Error())
	case s.Actor == "" || s.PolicyEpoch == "" || s.Purpose == "" || s.Destination == "" || s.Account == "" || s.Tenant == "":
		holds = append(holds, "scope:incomplete")
	case prose.VerifyDocument(doc) != nil:
		holds = append(holds, "integrity")
	case s.DocumentDigest != d.DocumentDigest || s.SessionDigest != d.SessionDigest || s.ProfileDigest != d.ProfileDigest || s.EvidenceDigest != d.EvidenceDigest:
		holds = append(holds, "scope:document-binding")
	}
	if len(holds) == 0 {
		if !f.tasks[s.Purpose] {
			holds = append(holds, "task:purpose-not-attested:"+s.Purpose)
		}
		if _, err := f.cfg.Engine.Render(ctx, doc); err != nil {
			holds = append(holds, "source:"+err.Error())
		}
		for _, id := range exposedFacts(doc) {
			if st := f.truth[id]; st != TruthConfirmed {
				if st == "" {
					st = TruthUnknown
				}
				holds = append(holds, "truth:"+id+":"+string(st))
			}
		}
		disputed := map[string]bool{}
		for _, sub := range f.cfg.Queue.Disputed() {
			disputed[sub] = true
		}
		for _, id := range exposedFacts(doc) {
			if disputed[id] {
				holds = append(holds, "disputed:"+id)
			}
		}
		for _, n := range doc.Nodes {
			if disputed[n.ID] {
				holds = append(holds, "disputed:"+n.ID)
			}
		}
		if ok, why := f.cfg.Queue.Ready(); !ok {
			holds = append(holds, "clarification:"+strings.Join(why, ","))
		}
		var live []CriterionObservation
		for _, o := range f.obs {
			if o.PolicyEpoch == s.PolicyEpoch {
				live = append(live, o)
			}
		}
		cfg := f.cfg.Assessment
		cfg.PolicyEpoch = s.PolicyEpoch
		if ga, err := AssessGoals(cfg, doc.Target.Session, d.DocumentDigest, f.cfg.ExpectedRequired, live); err != nil {
			holds = append(holds, "goals:"+err.Error())
		} else if !ga.Complete {
			holds = append(holds, "goals:"+ga.Status)
		}
	}
	if len(holds) > 0 {
		f.cfg.Queue.mutation.Unlock()
		f.mu.Unlock()
		return nil, &HoldError{Reasons: holds}
	}
	if f.AfterEnter != nil {
		f.AfterEnter()
	}
	var once sync.Once
	return func() { once.Do(func() { f.cfg.Queue.mutation.Unlock(); f.mu.Unlock() }) }, nil
}

var _ = errors.New
