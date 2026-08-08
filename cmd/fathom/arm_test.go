package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestArmStamping pins IO-14 (vision 14 §2): the OFF arm is stamped,
// bounded, announced, expiring — never a silent kill switch (O15).
func TestArmStamping(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "arm.json")
	t.Setenv("FATHOM_ARM_STATE", statePath)

	t.Run("off_requires_a_bound", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if code := cmdArm([]string{"off"}, &stdout, &stderr); code == 0 {
			t.Fatal("an unbounded OFF arm must refuse — indefinite is the env var's job")
		}
		if !strings.Contains(stderr.String(), "bound") {
			t.Errorf("refusal should name the rule, got %q", stderr.String())
		}
	})

	t.Run("off_with_sessions_activates_and_counts_down", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if code := cmdArm([]string{"off", "--sessions", "2", "--note", "pilot"}, &stdout, &stderr); code != 0 {
			t.Fatalf("arm off: %s", stderr.String())
		}
		if !armOffActive() {
			t.Fatal("arm should be active")
		}
		if got := currentArmLabel(time.Now()); got != "off" {
			t.Fatalf("ledger label = %q, want off", got)
		}
		decrementArmSessions()
		decrementArmSessions()
		if armOffActive() {
			t.Fatal("arm should have expired after its session budget")
		}
		if got := currentArmLabel(time.Now()); got != "on" {
			t.Errorf("ledger label after expiry = %q, want on", got)
		}
	})

	t.Run("wall_clock_expiry", func(t *testing.T) {
		st := armState{
			SchemaVersion: armSchemaVersion, FathomVersion: "test", Arm: "off",
			SetAt: time.Now().UTC().Format(time.RFC3339),
			Until: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
		}
		if err := writeArmState(st); err != nil {
			t.Fatal(err)
		}
		if armOffActive() {
			t.Fatal("a past-deadline arm must be inactive")
		}
	})

	t.Run("arm_on_clears", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if code := cmdArm([]string{"off", "--sessions", "5"}, &stdout, &stderr); code != 0 {
			t.Fatal(stderr.String())
		}
		if code := cmdArm([]string{"on"}, &stdout, &stderr); code != 0 {
			t.Fatal(stderr.String())
		}
		if armOffActive() {
			t.Fatal("arm on must deactivate")
		}
		if _, err := os.Stat(statePath); !os.IsNotExist(err) {
			t.Error("arm on should remove the state file")
		}
	})

	t.Run("channels_kill_switch_honors_the_arm", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if code := cmdArm([]string{"off", "--for", "1h"}, &stdout, &stderr); code != 0 {
			t.Fatal(stderr.String())
		}
		if !fathomChannelsDisabled() {
			t.Fatal("an active OFF arm must disable channels exactly like FATHOM_CHANNELS=off")
		}
		cmdArm([]string{"on"}, &stdout, &stderr)
	})
}
