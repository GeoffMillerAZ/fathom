package certify

import (
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestCertifyGateSweepsMovedAndGoneClaims(t *testing.T) {
	for _, tc := range []struct {
		name             string
		renameMarker     bool
		changedWatermark bool
		sweep            bool
		wantStep         Step
	}{
		{name: "changed files", sweep: true, wantStep: StepSweep},
		{name: "changed declarations", renameMarker: true, sweep: true, wantStep: StepCompile},
		{name: "changed watermark without file reads", changedWatermark: true, wantStep: StepCompile},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, base := seedCompileFixture(t)
			decls := []store.CertifyDeclaration{gateDeclaration("TestLive"), gateDeclaration("TestBroken")}
			if err := base.PutCertifyDeclarations(ctx, compileSvc, decls); err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
			if _, err := CompileCertifications(ctx, base, compileSvc, fixtureCommit, now, "", noopGitRunner); err != nil {
				t.Fatal(err)
			}
			if err := base.PurgeSource(ctx, compileSvc, "pkg/x_test.go"); err != nil {
				t.Fatal(err)
			}
			if err := base.PutObservations(ctx, []core.Observation{
				{ServiceID: compileSvc, Package: "pkg", Symbol: "TestLive", SourceRef: "pkg/moved_test.go", SourceKind: core.KindGoFunc},
				{ServiceID: compileSvc, Package: "pkg", Symbol: "TestBrokenRenamed", SourceRef: "pkg/x_test.go", SourceKind: core.KindGoFunc},
			}); err != nil {
				t.Fatal(err)
			}
			if tc.renameMarker {
				decls[1] = gateDeclaration("TestBrokenRenamed")
				if err := base.PutCertifyDeclarations(ctx, compileSvc, decls); err != nil {
					t.Fatal(err)
				}
			}
			watermark := fixtureCommit
			if tc.changedWatermark {
				watermark = "new-watermark"
			}
			st := &gateCountingStore{Store: base}
			out, err := AfterIngest(ctx, st, compileSvc, watermark, "", now, tc.sweep)
			if err != nil || out.Step != tc.wantStep {
				t.Fatalf("step=%v err=%v, want %v", out.Step, err, tc.wantStep)
			}
			claims, err := base.ClaimsForService(ctx, compileSvc, Layer)
			if err != nil {
				t.Fatal(err)
			}
			byID := make(map[string]core.SemanticClaim, len(claims))
			for _, c := range claims {
				byID[c.ClaimID] = c
			}
			for _, check := range []struct {
				name string
				want core.ClaimStatus
			}{{"TestLive", core.ClaimStale}, {"TestBroken", core.ClaimContradicted}} {
				id := core.ClaimID(Layer, Kind, "invariant:I1", symKey(check.name))
				if got := byID[id].Status; got != check.want {
					t.Errorf("%s status=%s, want %s", check.name, got, check.want)
				}
			}
			if tc.renameMarker {
				id := core.ClaimID(Layer, Kind, "invariant:I1", symKey("TestBrokenRenamed"))
				if _, ok := byID[id]; !ok {
					t.Error("new declaration was not compiled")
				}
			}
			wantReads := 1
			if tc.wantStep == StepCompile {
				wantReads++
			}
			if st.typedReads != wantReads {
				t.Errorf("typed reads=%d, want %d: a compile must still run the sweep", st.typedReads, wantReads)
			}
		})
	}
}
