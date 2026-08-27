package mcp

import (
	"context"
	"testing"
)

// TestSetProfileDirect exercises the internal profile machinery that still
// backs effectiveSurface, now that the set_profile tool has been removed
// entirely (docs/plans/surfaces-week1.md SW-2 — surfaces supersede profiles;
// amends the ADR-010/014 stdio clause). No production caller reaches
// setProfile any more — there is no CLI flag or tool wired to it — so this
// is the one direct test the machinery keeps, exercised from inside the
// package since no external caller can invoke it. It folds together what
// four now-deleted tool-invocation tests guarded:
//   - unknown profile is rejected (ex TestSetProfileUnknown)
//   - a named profile filters stdio tools/list, including "companion"
//     (ex TestSetProfileCompanionOnStdio)
//   - a group profile's stdio view matches the identically-named HTTP
//     surface's view — the two code paths share toolOnSurface/visibleToolsFor
//     (ex TestGroupSurfaceMatchesProfileView)
//   - tools/call stays advisory on stdio regardless of the active profile: a
//     plain stdio context carries no request scope, so it is never enforced
//     (ex TestStdioProfileDoesNotGateToolsCall)
func TestSetProfileDirect(t *testing.T) {
	srv := NewServer(nil)

	// Unknown profile is rejected.
	if err := srv.setProfile("bogus"); err == nil || err.Code != ErrCodeParams {
		t.Fatalf("setProfile(bogus) = %v, want ErrCodeParams", err)
	}

	// A known profile filters stdio tools/list to the companion allowlist.
	if err := srv.setProfile(SurfaceCompanion); err != nil {
		t.Fatalf("setProfile(companion): %v", err)
	}
	if got := srv.activeProfile(); got != SurfaceCompanion {
		t.Fatalf("activeProfile() = %q, want %q", got, SurfaceCompanion)
	}
	stdioCtx := context.Background()
	companionDescs := srv.visibleToolsCtx(stdioCtx)
	if len(companionDescs) != len(companionTools) {
		t.Fatalf("stdio companion profile lists %d tools, want %d", len(companionDescs), len(companionTools))
	}
	for _, d := range companionDescs {
		if !companionTools[d.Name] {
			t.Errorf("stdio companion profile listed off-allowlist tool %q", d.Name)
		}
	}

	// A group profile's stdio view matches the identically-named HTTP
	// surface's view — both resolve through toolOnSurface/visibleToolsFor.
	if err := srv.setProfile(SurfaceAgent); err != nil {
		t.Fatalf("setProfile(agent): %v", err)
	}
	viaProfile := srv.visibleToolsCtx(stdioCtx)
	httpCtx := WithRequestScope(context.Background(), SurfaceAgent, true)
	viaSurface := srv.visibleToolsCtx(httpCtx)
	if len(viaProfile) != len(viaSurface) {
		t.Fatalf("agent profile (%d tools) != agent surface (%d tools)", len(viaProfile), len(viaSurface))
	}
	surfaceNames := make(map[string]bool, len(viaSurface))
	for _, d := range viaSurface {
		surfaceNames[d.Name] = true
	}
	for _, d := range viaProfile {
		if !surfaceNames[d.Name] {
			t.Errorf("agent profile listed %q, not in agent surface view", d.Name)
		}
	}

	// tools/call stays advisory on stdio: a plain background context carries
	// no request scope (WithRequestScope is only ever attached by the HTTP
	// layer), so scopeFrom(ctx).enforced() is always false and an off-profile
	// tool (find_callers is GroupInvestigation, not on the companion
	// allowlist) must still dispatch regardless of the active profile.
	if err := srv.setProfile(SurfaceCompanion); err != nil {
		t.Fatalf("setProfile(companion): %v", err)
	}
	if sc := scopeFrom(stdioCtx); sc.enforced() {
		t.Fatal("a plain stdio context must never be enforced")
	}
	te, ok := srv.findTool("find_callers")
	if !ok {
		t.Fatal("find_callers not registered")
	}
	if scopeFrom(stdioCtx).enforced() && !srv.toolOnSurface(SurfaceCompanion, te.desc) {
		t.Fatal("find_callers would be gated on stdio, which must never happen")
	}
}
