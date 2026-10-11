package semantic

// compile_dryrun_ratio_test.go — EW-74 (G8): the dry-run estimate prices
// output at the service's own observed out/in ratio, else the documented
// default band. No LLM is ever called (the mock has no entries).

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func seedRun(t *testing.T, st store.Store, id, layer string, in, out int, dry bool, age int) {
	t.Helper()
	err := st.RecordCompileRun(context.Background(), core.CompileRun{
		RunID: id, ServiceID: "svc", Layer: layer, DryRun: dry,
		TokensIn: in, TokensOut: out, RanAt: fixedNow().Add(-time.Duration(age) * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDryRun_OutputBandUsesServiceHistoryRatio(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	seedRun(t, st, "r1", LayerObligations, 1000, 6700, false, 1)
	seedRun(t, st, "r2", LayerObligations, 2000, 13400, false, 2)
	c := newCompiler(t, st)
	run, err := c.Compile(ctx, compileInput(true))
	if err != nil {
		t.Fatal(err)
	}
	if run.TokensIn == 0 {
		t.Fatal("fixture estimated no input tokens")
	}
	if run.EstOutRatioLow != 6.7 || run.EstOutRatioHigh != 6.7 {
		t.Fatalf("ratio band = [%v,%v], want 6.7", run.EstOutRatioLow, run.EstOutRatioHigh)
	}
	want := int(math.Round(float64(run.TokensIn) * 6.7))
	if run.EstTokensOutLow != want || run.EstTokensOutHigh != want {
		t.Fatalf("est out = %d-%d, want %d", run.EstTokensOutLow, run.EstTokensOutHigh, want)
	}
	if run.EstOutBasis == "" || run.EstOutBasis == outBasisDefault {
		t.Fatalf("basis = %q, want a history basis", run.EstOutBasis)
	}
}

func TestDryRun_OutputBandNoHistoryUsesDefault(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	// A dry-run row and a refused/empty row are not history.
	seedRun(t, st, "d1", LayerObligations, 1000, 0, true, 1)
	c := newCompiler(t, st)
	run, err := c.Compile(ctx, compileInput(true))
	if err != nil {
		t.Fatal(err)
	}
	if run.EstOutRatioLow != dryRunOutRatioLow || run.EstOutRatioHigh != dryRunOutRatioHigh {
		t.Fatalf("ratio band = [%v,%v], want default [%d,%d]", run.EstOutRatioLow, run.EstOutRatioHigh, dryRunOutRatioLow, dryRunOutRatioHigh)
	}
	if run.EstTokensOutLow != run.TokensIn*dryRunOutRatioLow || run.EstTokensOutHigh != run.TokensIn*dryRunOutRatioHigh {
		t.Fatalf("est out = %d-%d for in %d", run.EstTokensOutLow, run.EstTokensOutHigh, run.TokensIn)
	}
	if run.EstOutBasis != outBasisDefault {
		t.Fatalf("basis = %q, want %q", run.EstOutBasis, outBasisDefault)
	}
}

func TestDryRun_OutputBandPrefersSameLayer(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	seedRun(t, st, "o1", LayerObligations, 1000, 3000, false, 1)
	seedRun(t, st, "x1", "other-layer", 1000, 9000, false, 2)
	c := newCompiler(t, st)
	run, err := c.Compile(ctx, compileInput(true))
	if err != nil {
		t.Fatal(err)
	}
	if run.EstOutRatioLow != 3 || run.EstOutRatioHigh != 3 {
		t.Fatalf("ratio band = [%v,%v], want the obligations layer's own 3", run.EstOutRatioLow, run.EstOutRatioHigh)
	}
}

func TestOutRatioBand_RangeAcrossRuns(t *testing.T) {
	runs := []core.CompileRun{
		{Layer: "l", TokensIn: 100, TokensOut: 300},
		{Layer: "l", TokensIn: 100, TokensOut: 900},
		{Layer: "l", TokensIn: 100, TokensOut: 0},                  // no output: not history
		{Layer: "l", TokensIn: 100, TokensOut: 500, Refused: true}, // refused: not history
	}
	lo, hi, n := outRatioBand(runs, "l")
	if lo != 3 || hi != 9 || n != 2 {
		t.Fatalf("band = %v-%v over %d, want 3-9 over 2", lo, hi, n)
	}
	if _, _, n := outRatioBand(runs, "absent"); n != 0 {
		t.Fatalf("a layer with no runs counted %d", n)
	}
}
