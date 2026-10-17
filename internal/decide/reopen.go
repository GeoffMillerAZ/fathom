package decide

// reopen.go — O1132 (KS-2, ADR-073 decision 5): a port a live kill (the
// kill file, the user's enabled = false) closed at Open is not settled the
// way FATHOM_DECIDE=off and a refused consent are. Like any open port it
// re-checks the kill before each ask, and the first ask after the kill
// lifts finishes Open once (the lane or the remote adapter, its key
// source, the degrade fallback), so a serve-http daemon started while
// killed reopens with no restart. There is no background job: the ask's
// own re-check is the only trigger, and while killed the port reads no
// key, execs nothing, builds no lane and dials nothing.

import (
	"context"
	"fmt"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/config"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// isLiveKill reports whether source is a kill re-checked before each ask
// (the kill file, the user's enabled), as opposed to one Open settles for
// the process (the environment, consent).
func isLiveKill(source string) bool {
	return source == KillSourceFile || source == KillSourceEnabled
}

// reopener holds what Open needs to finish a port a live kill held closed
// at Open, and the result of finishing it, built once.
type reopener struct {
	cfg          config.Decide
	lane         func(profile string) (llmclient.LLMClient, error)
	model        string
	rec          *Recorder
	o            openOptions
	live         *liveGate
	consentLayer string

	once    sync.Once
	adapter decision.Decider
	degrade *degrader
	err     error
}

// open builds the adapter on the first ask after the kill lifted; a
// failure is disclosed once, as Open's caller discloses one at startup,
// and the port stays on the deterministic path.
func (r *reopener) open() (decision.Decider, *degrader, error) {
	r.once.Do(func() {
		adapter, err := buildAdapter(r.cfg, r.lane, r.model, r.o)
		if err != nil {
			r.err = err
			fmt.Fprintf(r.o.disclose, "warn: decision port disabled: %v\n", err)
			return
		}
		r.adapter, r.degrade = adapter, &degrader{w: r.o.disclose}
		r.rec.setConsentLayer(r.consentLayer)
	})
	return r.adapter, r.degrade, r.err
}

// notOpenedError is what a reopening port's ask returns when finishing
// Open failed: decision.ErrNoProvider, so the site takes its
// deterministic path.
type notOpenedError struct{ err error }

func (e *notOpenedError) Error() string {
	return fmt.Sprintf("decision port did not open after the kill lifted: %v", e.err)
}

func (e *notOpenedError) Unwrap() error { return decision.ErrNoProvider }

// reopening is the Decider a site asks for a port a reopener holds: the
// live gate, then, once the port opened, the provider with gate applied
// (none for Ports.Decider; serve-http's for Ports.Gated) inside the
// degrade fallback, composed once.
type reopening struct {
	r    *reopener
	gate func(decision.Decider) decision.Decider

	mu sync.Mutex
	d  decision.Decider
}

// Name is the configured provider, the composed fallback's own name.
func (v *reopening) Name() string { return v.r.cfg.Provider }

func (v *reopening) Decide(ctx context.Context, req decision.Request) (decision.Response, error) {
	if g := v.r.live.check(); g.Closed {
		return decision.Response{Provider: v.Name()}, &KilledError{Gate: g}
	}
	d, err := v.decider()
	if err != nil {
		return decision.Response{Provider: v.Name()}, err
	}
	return d.Decide(ctx, req)
}

func (v *reopening) decider() (decision.Decider, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.d != nil {
		return v.d, nil
	}
	adapter, deg, err := v.r.open()
	if err != nil {
		return nil, &notOpenedError{err: err}
	}
	if v.gate != nil {
		adapter = v.gate(adapter)
	}
	d, err := compose(v.r.cfg, adapter, deg)
	if err != nil {
		return nil, &notOpenedError{err: fmt.Errorf("composing the degrade fallback: %w", err)}
	}
	v.d = d
	return d, nil
}

// Capabilities declares what the composed provider declares once the port
// opened, and nothing before: asking for them never finishes Open.
func (v *reopening) Capabilities() decision.Capabilities {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.d == nil {
		return decision.Capabilities{}
	}
	return decision.CapabilitiesOf(v.d)
}
