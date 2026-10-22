package conserve

import (
	"context"
	"testing"
)

type countingPlane struct {
	name  string
	calls *int
}

func (p countingPlane) Name() string { return p.name }
func (p countingPlane) Check(context.Context, Input) ([]Finding, []string) {
	*p.calls++
	return nil, nil
}

func TestRunCanceledDoesNotInvokeLaterPlanes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	first := cancelPlane{cancel: cancel}
	rep := Run(ctx, Input{}, []Plane{first, countingPlane{name: "later", calls: &calls}})
	if calls != 0 || len(rep.Planes) != 2 || rep.Planes[0].Verdict != Unproven || rep.Planes[1].Verdict != Unproven || len(rep.Planes[1].Missing) == 0 {
		t.Fatalf("later plane ran or looked conserved: calls=%d report=%+v", calls, rep.Planes)
	}
}

type cancelPlane struct{ cancel context.CancelFunc }

func (cancelPlane) Name() string                                         { return "first" }
func (p cancelPlane) Check(context.Context, Input) ([]Finding, []string) { p.cancel(); return nil, nil }
