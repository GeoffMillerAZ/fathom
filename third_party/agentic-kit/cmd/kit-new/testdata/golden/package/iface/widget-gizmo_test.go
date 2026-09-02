package widgetgizmo_test

import (
	"errors"
	"testing"
	"time"

	widgetgizmo "example.test/kit/foundations/alpha/widget-gizmo"
)

// fixedClock returns a clock that starts at t0 and advances by step on every
// call — the injected-clock pattern every kit test uses: no sleeps, no wall
// time.
func fixedClock(t0 time.Time, step time.Duration) func() time.Time {
	i := 0
	return func() time.Time {
		t := t0.Add(time.Duration(i) * step)
		i++
		return t
	}
}

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func validConfig() widgetgizmo.Config {
	return widgetgizmo.Config{Name: "example", Limit: 1}
}

// TestConfigValidate mirrors testdata/invalid.cue: every commented-out
// negative fixture there is rejected here in Go.
func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     widgetgizmo.Config
		wantErr bool
	}{
		{name: "valid", cfg: validConfig()},
		{name: "empty name", cfg: widgetgizmo.Config{Name: "", Limit: 1}, wantErr: true},
		{name: "zero limit", cfg: widgetgizmo.Config{Name: "example", Limit: 0}, wantErr: true},
		{name: "negative limit", cfg: widgetgizmo.Config{Name: "example", Limit: -1}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr && !errors.Is(err, widgetgizmo.ErrInvalidConfig) {
				t.Fatalf("Validate() = %v, want ErrInvalidConfig", err)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestNewRejectsNilClockAndInvalidConfig(t *testing.T) {
	if _, err := widgetgizmo.New(validConfig(), nil); err == nil {
		t.Fatal("nil clock must be rejected — no wall-time defaults")
	}
	if _, err := widgetgizmo.New(widgetgizmo.Config{}, fixedClock(t0, 0)); !errors.Is(err, widgetgizmo.ErrInvalidConfig) {
		t.Fatalf("New(zero Config) = %v, want ErrInvalidConfig", err)
	}
}

func TestStampUsesInjectedClock(t *testing.T) {
	p, err := widgetgizmo.New(validConfig(), fixedClock(t0, time.Second))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		want := t0.Add(time.Duration(i) * time.Second)
		if got := p.Stamp(); !got.Equal(want) {
			t.Fatalf("Stamp() #%d = %v, want %v", i, got, want)
		}
	}
	if got := p.Config(); got != validConfig() {
		t.Fatalf("Config() = %+v, want %+v", got, validConfig())
	}
}

func TestMockStamperScriptThenFallback(t *testing.T) {
	m := widgetgizmo.NewMockStamper(t0)
	m.Fallback = t0.Add(time.Hour)
	if got := m.Stamp(); !got.Equal(t0) {
		t.Fatalf("first Stamp() = %v, want %v", got, t0)
	}
	if got := m.Stamp(); !got.Equal(m.Fallback) {
		t.Fatalf("exhausted Stamp() = %v, want fallback %v", got, m.Fallback)
	}
	if m.Calls() != 2 {
		t.Fatalf("Calls() = %d, want 2", m.Calls())
	}
}
