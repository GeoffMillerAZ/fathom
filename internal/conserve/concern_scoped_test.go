package conserve

import (
	"context"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

type concernScopedStore struct {
	store.Store
	t     *testing.T
	reads int
}

func (s *concernScopedStore) QueryObservations(context.Context, string, string) ([]core.Observation, error) {
	s.t.Fatal("concern deltas loaded service-wide observations")
	return nil, nil
}

func (s *concernScopedStore) PackageObservations(ctx context.Context, service string, pkgs []string) ([]core.Observation, error) {
	if service != "svc" || !reflect.DeepEqual(pkgs, []string{"api"}) {
		s.t.Fatalf("unscoped packages: %s %v", service, pkgs)
	}
	s.reads++
	return s.Store.PackageObservations(ctx, service, pkgs)
}

func TestConcernDeltasScopedObservations(t *testing.T) {
	r := newInputTestRepo(t)
	r.write("deploy/api.yaml", concernManifest("api", 3))
	r.write("unrelated/worker.yaml", concernManifest("worker", 7))
	r.write("unrelated/api.yaml", concernManifest("api", 8))
	r.commitAll("base")
	base := r.headSHA()
	r.write("deploy/api.yaml", concernManifest("api", 1))
	r.commitAll("head")
	st := store.NewMemoryStore()
	concernIngest(t, r, st)
	scoped := &concernScopedStore{Store: st, t: t}
	rep, err := ConcernDeltas(context.Background(), concernInput(t, r, base, scoped))
	if err != nil {
		t.Fatal(err)
	}
	if scoped.reads != 1 || len(rep.Deltas) != 1 || rep.Deltas[0].Change != "changed" || rep.Deltas[0].BaseAnchor != "deploy/api.yaml:6" || *rep.Deltas[0].Before != "3" || *rep.Deltas[0].After != "1" {
		t.Fatalf("scoped reads=%d deltas=%+v", scoped.reads, rep.Deltas)
	}
}
