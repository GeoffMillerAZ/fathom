package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/harness"
	heartbeat "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/heartbeat"
)

func TestDoctorChannelsStatesAndCosts(t *testing.T) {
	now := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	d, _ := harness.Lookup(harness.ClaudeCode)
	for _, tc := range []struct {
		name      string
		on, wired bool
		beats     []heartbeat.Beat
		want      string
	}{
		{name: "off", want: "off by choice"},
		{name: "missing-wiring", on: true, want: "on but not ready"},
		{name: "unused", on: true, wired: true, want: "on but unused"},
		{name: "ready", on: true, wired: true, want: "on and ready", beats: []heartbeat.Beat{{At: now.Add(-time.Hour), Channel: "payload-cost", Kind: "read-outline-hint", Meta: map[string]string{"tokens_estimate": "70"}}}},
		{name: "not-ready", on: true, wired: true, want: "on but not ready", beats: []heartbeat.Beat{{At: now, Channel: "read-outline-hint", Kind: "skipped:not-ready:no-source"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Config{Hooks: config.Hooks{ReadOutlineHint: tc.on}}
			rows := doctorChannelRows(d, cfg, map[harness.Capability]bool{harness.CapReadOutlineHint: tc.wired}, tc.beats, now, true, "")
			found := false
			for _, r := range rows {
				if r.Channel == string(harness.CapReadOutlineHint) {
					found = true
					if r.State != tc.want {
						t.Fatalf("state=%+v", r)
					}
					if tc.name == "ready" && (r.Injections == nil || *r.Injections != 1 || r.TokensPerDayEstimate == nil || *r.TokensPerDayEstimate != 10) {
						t.Fatalf("cost=%+v", r)
					}
					if tc.name == "not-ready" && !strings.Contains(r.LastSkipReason, "no-source") {
						t.Fatalf("skip=%+v", r)
					}
				}
			}
			if !found {
				t.Fatal("injecting channel omitted")
			}
			for _, b := range d.Hooks {
				if b.Vehicle == harness.VehicleHookInject {
					found = false
					for _, r := range rows {
						found = found || r.Channel == string(b.Capability)
					}
					if !found {
						t.Errorf("missing %s", b.Capability)
					}
				}
			}
		})
	}
}

func TestDoctorChannelsMissingLogIsNotZero(t *testing.T) {
	d, _ := harness.Lookup(harness.ClaudeCode)
	rows := doctorChannelRows(d, config.Config{}, nil, nil, time.Unix(100, 0), false, "hook log unreadable")
	b, _ := json.Marshal(rows)
	if strings.Contains(string(b), `"injections":0`) {
		t.Fatalf("absent log reported zero: %s", b)
	}
}

func TestDoctorSlowCheckHasNamedTimeout(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	rows := runDoctorStepsContext(ctx, []doctorStep{
		{name: "slow_fixture", probe: func(ctx context.Context) doctorCheck {
			<-ctx.Done()
			return doctorCheck{Name: "slow_fixture", Status: doctorSkipped, probeErr: ctx.Err()}
		}},
		{name: "local_fixture", probe: func(context.Context) doctorCheck {
			if !fileExists(root) {
				t.Error("generated repo absent")
			}
			return doctorCheck{Name: "local_fixture", Status: doctorPass}
		}},
	})
	if len(rows) != 2 || rows[0].Name != "slow_fixture" || rows[0].Status != doctorTimeout || rows[1].Status != doctorPass {
		t.Fatalf("unfinished checks hidden: %+v", rows)
	}
}

func TestDoctorChannelsRenderInTextAndJSON(t *testing.T) {
	now := time.Unix(100, 0)
	d, _ := harness.Lookup(harness.ClaudeCode)
	rows := doctorChannelRows(d, config.Config{}, nil, nil, now, false, "hook log unavailable")
	var out strings.Builder
	printDoctorChannels(&out, rows)
	model := doctorJSONPayload("claude", "fixture", t.TempDir(), nil, rows)
	data, _ := json.Marshal(model)
	for _, row := range rows {
		if !strings.Contains(out.String(), row.Channel) || !strings.Contains(string(data), row.Channel) {
			t.Fatalf("channel omitted from a surface: %s", row.Channel)
		}
	}
	if !strings.Contains(out.String(), "~TOKENS/DAY") {
		t.Fatal("units omitted")
	}
}
