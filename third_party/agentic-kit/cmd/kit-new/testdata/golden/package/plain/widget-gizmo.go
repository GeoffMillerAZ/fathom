package widgetgizmo

import (
	"errors"
	"time"
)

// WidgetGizmo is the package's primary value: one validated Config plus the
// injected clock. Construct it with New.
type WidgetGizmo struct {
	cfg Config
	now func() time.Time
}

// New returns a WidgetGizmo for cfg. now is the injected clock and must not be
// nil: this package never falls back to time.Now.
func New(cfg Config, now func() time.Time) (*WidgetGizmo, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if now == nil {
		return nil, errors.New("widgetgizmo: clock must not be nil (inject it — no wall-time default)")
	}
	return &WidgetGizmo{cfg: cfg, now: now}, nil
}

// Config returns the validated configuration this instance was built with.
func (p *WidgetGizmo) Config() Config { return p.cfg }

// Stamp returns the current time from the injected clock, in UTC. It is the
// placeholder behavior: replace it with what the package actually does.
func (p *WidgetGizmo) Stamp() time.Time { return p.now().UTC() }
