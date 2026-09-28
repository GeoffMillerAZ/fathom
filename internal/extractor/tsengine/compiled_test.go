package tsengine

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/memo"
)

// TestCompiledGrammarsChargedToMemo (LB-F15, design §9, I51): each compiled
// grammar is charged to the memo registry's byte budget; the shed trim
// evicts the least recently used and the refuse drop evicts all, and an
// evicted grammar's compiled code is closed once no parser is being made
// from it — while parsers already instantiated from it keep working.
func TestCompiledGrammarsChargedToMemo(t *testing.T) {
	reg := memo.NewRegistry()
	cc := newCompiledCache(reg)
	ctx := context.Background()
	lock, err := embeddedLock()
	if err != nil {
		t.Fatal(err)
	}
	py, _ := lock.Row("python")
	js, _ := lock.Row("javascript")

	cgPy, err := cc.acquire(ctx, "python")
	if err != nil {
		t.Fatal(err)
	}
	cgPy.release()
	if _, resident := reg.Budget(); resident != py.BlobBytes {
		t.Fatalf("resident %d after compiling python, want its %d module bytes", resident, py.BlobBytes)
	}
	cgJS, err := cc.acquire(ctx, "javascript")
	if err != nil {
		t.Fatal(err)
	}
	cgJS.release()
	if _, resident := reg.Budget(); resident != py.BlobBytes+js.BlobBytes {
		t.Fatalf("resident %d, want %d", resident, py.BlobBytes+js.BlobBytes)
	}

	// A parser made from python before its eviction survives it.
	r, err := ensureRuntime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	held, err := cc.acquire(ctx, "python")
	if err != nil {
		t.Fatal(err)
	}
	mod, err := r.InstantiateModule(ctx, held.cm, wazeroModuleConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer mod.Close(ctx)

	// Shed: trim to one grammar's worth — python (least recently used... it
	// was just re-acquired, so javascript goes first).
	reg.SetBudget(py.BlobBytes+js.BlobBytes, 0)
	reg.Trim(memo.ReasonShed, float64(py.BlobBytes)/float64(py.BlobBytes+js.BlobBytes))
	if !cgJS.isClosed() {
		t.Fatal("the shed trim did not close javascript's compiled code")
	}
	if held.isClosed() {
		t.Fatal("python's compiled code was closed while a parser was being made from it")
	}
	held.release()

	reg.Drop(memo.ReasonRefuse)
	if !held.isClosed() {
		t.Fatal("the refuse drop left python's compiled code open")
	}
	if _, resident := reg.Budget(); resident != 0 {
		t.Fatalf("resident %d after refuse, want 0", resident)
	}
	if fn := mod.ExportedFunction("ts_parser_new"); fn == nil {
		t.Fatal("an instance lost its exports")
	} else if _, err := fn.Call(ctx); err != nil {
		t.Fatalf("an instance made before eviction stopped working: %v", err)
	}

	// The next parser recompiles.
	again, err := cc.acquire(ctx, "python")
	if err != nil {
		t.Fatal(err)
	}
	if again == held || again.isClosed() {
		t.Fatal("a grammar evicted at refuse was not recompiled")
	}
	again.release()
	reg.Drop(memo.ReasonRefuse)
}
