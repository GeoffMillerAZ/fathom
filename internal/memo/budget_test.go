package memo

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// synthNodes is a service shaped like a real Go one: n symbols over n/50
// packages, each calling four others (package-qualified and bare).
func synthNodes(n int) []core.SymbolNode {
	nodes := make([]core.SymbolNode, n)
	for i := range nodes {
		pkg := fmt.Sprintf("internal/pkg%03d", i/50)
		nodes[i] = core.SymbolNode{
			Service:   "svc",
			Package:   pkg,
			Name:      fmt.Sprintf("Func%05d", i),
			Kind:      core.KindGoFunc,
			FilePath:  fmt.Sprintf("%s/file%02d.go", pkg, i%7),
			Line:      i%400 + 1,
			Signature: fmt.Sprintf("func Func%05d(ctx context.Context, x int) error", i),
			Calls: []string{
				fmt.Sprintf("pkg%03d.Func%05d", ((i+1)%n)/50, (i+1)%n),
				fmt.Sprintf("Func%05d", (i+7)%n),
				fmt.Sprintf("pkg%03d.Func%05d", ((i+131)%n)/50, (i+131)%n),
				fmt.Sprintf("Func%05d", (i+977)%n),
			},
		}
	}
	return nodes
}

func heapInUse() uint64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// TestByteEstimatesTrackTheHeap (review finding, I51): the registry's byte
// budget is only as good as the builders' estimates. Measured against the
// heap for a 20k-symbol service, the snapshot's and each graph's estimate
// must be at least 80% of what the value really holds (an undercount lets
// the cache fraction admit several times its budget) and at most 2.5×.
func TestByteEstimatesTrackTheHeap(t *testing.T) {
	if testing.Short() {
		t.Skip("heap measurement")
	}
	isolateGraphStore(t)
	ctx := context.Background()
	const n = 20000
	var nodes []core.SymbolNode
	base := heapInUse()
	nodes = synthNodes(n)
	afterNodes := heapInUse()
	sc := Scope{Store: "budget", Service: "svc", Watermark: "w1"}
	snap, err := SnapshotFor(ctx, sc, func(context.Context) ([]core.SymbolNode, error) { return nodes, nil })
	if err != nil {
		t.Fatal(err)
	}
	afterSnap := heapInUse()
	// The snapshot owns the node slice: count it with the index.
	check := func(what string, est int64, real uint64) {
		t.Helper()
		ratio := float64(est) / float64(real)
		t.Logf("%s: estimate %d, heap %d (%.2f×)", what, est, real, ratio)
		if ratio < 0.8 || ratio > 2.5 {
			t.Errorf("%s estimate %d is %.2f× the heap it holds (%d)", what, est, ratio, real)
		}
	}
	check("snapshot", snapshotGroup.stats().Bytes, afterSnap-base)
	_ = afterNodes
	for _, kind := range []GraphKind{GraphSyntax, GraphBoth, GraphResolved} {
		before := heapInUse()
		startBytes := graphGroup.stats().Bytes
		if _, err := GraphFor(ctx, snap, kind, func(context.Context) ([]core.Edge, error) { return nil, nil }); err != nil {
			t.Fatal(err)
		}
		after := heapInUse()
		check(string(kind)+" graph", graphGroup.stats().Bytes-startBytes, after-before)
	}
	runtime.KeepAlive(snap)
}
