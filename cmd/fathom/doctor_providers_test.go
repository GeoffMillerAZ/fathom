package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// TestDoctorProvidersRow pins SR-32 (surface reckoning F-71): doctor's
// `providers` row is CF-5's own zero/near-zero-token ProbeAll pass,
// condensed to one row, without ever running CF-6's real-spend `smoke`.
func TestDoctorProvidersRow(t *testing.T) {
	t.Run("no profiles configured and env-overlay unusable is skipped, not failed", func(t *testing.T) {
		_, projectDir := withIsolatedConfigHome(t)
		build := func(config.Profile) (llmclient.LLMClient, error) {
			return nil, errors.New("no API key configured")
		}
		got := doctorProvidersRow(projectDir, build)
		if got.Status != doctorSkipped {
			t.Fatalf("Status = %v, want skipped: %+v", got.Status, got)
		}
		if !strings.Contains(got.Reason, "optional") {
			t.Errorf("Reason = %q, want it to say providers/smoke are optional", got.Reason)
		}
	})

	t.Run("configured profile pings healthy is a pass", func(t *testing.T) {
		_, projectDir := withIsolatedConfigHome(t)
		writeProjectConfig(t, projectDir, `
[profiles.work]
provider = "claude-cli"
`)
		build := func(config.Profile) (llmclient.LLMClient, error) {
			return fakePingerClient{}, nil
		}
		got := doctorProvidersRow(projectDir, build)
		if got.Status != doctorPass {
			t.Fatalf("Status = %v, want pass: %+v", got.Status, got)
		}
		if !strings.Contains(got.Reason, "1/1") {
			t.Errorf("Reason = %q, want it to count 1/1 profile(s) ok", got.Reason)
		}
	})

	t.Run("a configured profile that fails to authenticate is a fail naming it", func(t *testing.T) {
		_, projectDir := withIsolatedConfigHome(t)
		writeProjectConfig(t, projectDir, `
[profiles.broken]
provider = "claude-cli"
`)
		build := func(config.Profile) (llmclient.LLMClient, error) {
			return fakePingerClient{pingErr: errors.New("401 unauthorized")}, nil
		}
		got := doctorProvidersRow(projectDir, build)
		if got.Status != doctorFail {
			t.Fatalf("Status = %v, want fail: %+v", got.Status, got)
		}
		if !strings.Contains(got.Reason, "broken") {
			t.Errorf("Reason = %q, want it to name the broken profile", got.Reason)
		}
		if got.Remedy == "" {
			t.Errorf("Remedy is empty, want a one-command remedy on fail")
		}
	})
}

// TestDoctorProvidersRow_HiddenAliasesStayDispatched pins the other half
// of SR-32: `providers` and `smoke` are hidden from `fathom help`'s
// rendered output but still fully dispatched — a hook or skill calling
// either directly keeps working, and CF-6 (smoke) is never invoked from
// doctor's own row.
func TestDoctorProvidersRow_HiddenAliasesStayDispatched(t *testing.T) {
	for _, name := range []string{"providers", "smoke"} {
		found, ok := lookupVerb(name)
		if !ok {
			t.Fatalf("verb %q not registered", name)
		}
		if !found.Hidden {
			t.Errorf("verb %q: Hidden = false, want true (SR-32 hides it from fathom help)", name)
		}
	}
}
