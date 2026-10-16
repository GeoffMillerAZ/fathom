package governedinvestigation_test

import (
	"testing"

	scenario "github.com/geoffmilleraz/fathom/third_party/agentic-kit/scenarios/governed-investigation"
)

// ── The destructive remediation goes through the effect gate ─────────────────

func TestRun_RemediationIsGatedAndApproved(t *testing.T) {
	r := scenario.Run()
	rem := r.Remediation

	if rem.ToolName != "rollback_deployment" {
		t.Errorf("Remediation.ToolName = %q, want rollback_deployment", rem.ToolName)
	}
	if rem.Effect != "destructive" {
		t.Errorf("Remediation.Effect = %q, want destructive (the tool's declared DestructiveHint)", rem.Effect)
	}
	if !rem.Approved || rem.ApprovedBy != "on-call reviewer (seeded)" {
		t.Errorf("Remediation approved=%t by %q, want the seeded approval", rem.Approved, rem.ApprovedBy)
	}
	if rem.GateApproved != 1 || rem.GateDenied != 0 {
		t.Errorf("gate events approved=%d denied=%d, want exactly one approval and no denials (the read-only read_deploy_log needs no token)", rem.GateApproved, rem.GateDenied)
	}
	if rem.Stop != "completed" {
		t.Errorf("Remediation.Stop = %q, want completed", rem.Stop)
	}
	if rem.Summary == "" {
		t.Error("Remediation.Summary is empty, want the model's final text")
	}
}

func TestRun_RemediationIsJournaled(t *testing.T) {
	r := scenario.Run()
	steps := make(map[string]bool, len(r.Journal))
	for _, e := range r.Journal {
		steps[e.Step] = true
	}
	for _, want := range []string{"remediation-proposed", "run-started", "gate-approved", "run-ended", "remediation-complete"} {
		if !steps[want] {
			t.Errorf("journal missing step %q; steps = %v", want, steps)
		}
	}
	if steps["gate-denied"] {
		t.Error("journal has a gate-denied step; the seeded approver grants the rollback")
	}
}

func TestRun_RemediationIsDeterministic(t *testing.T) {
	r1, r2 := scenario.Run(), scenario.Run()
	if r1.Remediation != r2.Remediation {
		t.Errorf("Remediation non-deterministic:\n  run1: %+v\n  run2: %+v", r1.Remediation, r2.Remediation)
	}
}
