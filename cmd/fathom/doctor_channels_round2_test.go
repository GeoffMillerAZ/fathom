package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/harness"
	heartbeat "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/heartbeat"
)

func channelRow(t *testing.T, rows []doctorChannel, cap harness.Capability) doctorChannel {
	t.Helper()
	for _, row := range rows {
		if row.Channel == string(cap) {
			return row
		}
	}
	t.Fatalf("missing channel %s", cap)
	return doctorChannel{}
}

func TestDoctorChannelsRetainedSpan(t *testing.T) {
	now := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	root := doctorHookFixture(t, "fathom prompt-hook")
	t.Setenv("FATHOM_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", path)
	old, _ := json.Marshal(heartbeat.Beat{At: now.Add(-48 * time.Hour), Channel: "payload-cost", Kind: "post-edit", Meta: map[string]string{"tokens_estimate": "140"}})
	if err := os.WriteFile(path+".1", append(old, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	// More than the previous 1 MiB tail: recent rows carry no post-edit cost.
	noise, _ := json.Marshal(heartbeat.Beat{At: now.Add(-time.Hour), Channel: "discovery", Meta: map[string]string{"padding": strings.Repeat("x", 1024)}})
	if err := os.WriteFile(path, []byte(strings.Repeat(string(noise)+"\n", 1100)), 0600); err != nil {
		t.Fatal(err)
	}
	rows := doctorChannels(root, "claude", now)
	row := channelRow(t, rows, harness.CapObligationsPostEdit)
	if row.Injections == nil || *row.Injections != 1 || row.TokensPerDayEstimate == nil || *row.TokensPerDayEstimate != 70 {
		t.Fatalf("rotated cost or covered denominator lost: %+v", row)
	}
	row = channelRow(t, rows, harness.CapPrime)
	if row.State != "unknown" || row.Injections != nil || !strings.Contains(row.Coverage, now.Add(-48*time.Hour).Format(time.RFC3339)) {
		t.Fatalf("incomplete history claimed unused: %+v", row)
	}
}

func TestDoctorChannelBindingsAndSkipPhases(t *testing.T) {
	d, _ := harness.Lookup(harness.ClaudeCode)
	now := time.Unix(1000000, 0)
	rows := doctorChannelRows(d, config.Config{Readouts: config.Readouts{Mode: config.ReadoutsRich}}, map[harness.Capability]bool{harness.CapObligationsPostEdit: true, harness.CapGuardPreEdit: true, harness.CapReadout: true}, []heartbeat.Beat{
		{At: now, Channel: "edit-hook", Kind: "answered", Meta: map[string]string{"phase": "post-bash", "reason": "bash-only"}},
		{At: now.Add(-time.Second), Channel: "edit-hook", Kind: "answered", Meta: map[string]string{"phase": "post", "reason": "edit-only"}},
	}, now, true, "")
	for _, row := range rows {
		if row.Channel == string(harness.CapClaimGate) {
			t.Fatalf("unbound capability misdiagnosed: %+v", row)
		}
	}
	if row := channelRow(t, rows, harness.CapObligationsPostEdit); row.LastSkipReason != "edit-only" {
		t.Fatalf("wrong phase: %+v", row)
	}
	if row := channelRow(t, rows, harness.CapGuardPreEdit); row.LastSkipReason != "" {
		t.Fatalf("wrong phase: %+v", row)
	}
	if row := channelRow(t, rows, harness.CapReadout); row.State == "off by choice" {
		t.Fatalf("rich readout omitted: %+v", row)
	}
}

func TestDoctorChannelsOffArm(t *testing.T) {
	root := doctorHookFixture(t, "fathom prompt-hook")
	t.Setenv("FATHOM_HOME", t.TempDir())
	t.Setenv("FATHOM_CHANNELS", "")
	t.Setenv("FATHOM_ARM_STATE", filepath.Join(t.TempDir(), "arm.json"))
	if err := writeArmState(armState{Arm: "off"}); err != nil {
		t.Fatal(err)
	}
	for _, row := range doctorChannels(root, "claude", time.Unix(1000000, 0)) {
		if row.State != "off by choice" {
			t.Errorf("OFF arm ignored: %+v", row)
		}
	}
}
