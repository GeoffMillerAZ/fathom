package conserve

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestConcernDeltasAcrossFiles(t *testing.T) {
	for _, mode := range []string{"rename-identical", "split-file", "move-plus-edit"} {
		t.Run(mode, func(t *testing.T) {
			r := newInputTestRepo(t)
			first := concernManifest("api", 3)
			second := concernManifest("worker", 2)
			baseText := first
			if mode == "split-file" {
				baseText += "---\n" + second
			}
			r.write("deploy/old.yaml", baseText)
			r.commitAll("base")
			base := r.headSHA()
			r.run("rm", "deploy/old.yaml")
			if mode == "move-plus-edit" {
				first = concernManifest("api", 1)
			}
			r.write("new/api.yaml", "# moved\n"+first)
			if mode == "split-file" {
				r.write("new/worker.yaml", second)
			}
			r.commitAll("head")
			st := store.NewMemoryStore()
			concernIngest(t, r, st)
			rep, err := ConcernDeltas(context.Background(), concernInput(t, r, base, st))
			if err != nil {
				t.Fatal(err)
			}
			if mode != "move-plus-edit" {
				if len(rep.Deltas) != 0 {
					t.Fatalf("unchanged moved facts: %+v", rep.Deltas)
				}
				return
			}
			if len(rep.Deltas) != 1 {
				t.Fatalf("deltas: %+v", rep.Deltas)
			}
			d := rep.Deltas[0]
			if d.Change != "changed" || d.Key != "replicas" || *d.Before != "3" || *d.After != "1" || d.BaseAnchor != "deploy/old.yaml:6" || d.HeadAnchor != "new/api.yaml:7" {
				t.Fatalf("moved edit: %+v", d)
			}
		})
	}
}

func TestConcernDeltasDuplicateNamesKeepPaths(t *testing.T) {
	for _, side := range []string{"base", "head", "both"} {
		t.Run(side, func(t *testing.T) {
			r := newInputTestRepo(t)
			r.write("deploy/api.yaml", concernManifest("api", 3))
			if side != "head" {
				r.write("patch/api.yaml", concernManifest("api", 7))
			}
			r.commitAll("base")
			base := r.headSHA()
			r.write("deploy/api.yaml", concernManifest("api", 1))
			if side == "base" {
				r.run("rm", "patch/api.yaml")
			} else {
				r.write("patch/api.yaml", concernManifest("api", 8))
			}
			r.commitAll("head")
			st := store.NewMemoryStore()
			concernIngest(t, r, st)
			rep, err := ConcernDeltas(context.Background(), concernInput(t, r, base, st))
			if err != nil {
				t.Fatal(err)
			}
			if len(rep.Deltas) != 2 {
				t.Fatalf("duplicate identities collapsed: %+v", rep.Deltas)
			}
			for _, d := range rep.Deltas {
				if strings.HasPrefix(d.BaseAnchor, "deploy/") || strings.HasPrefix(d.HeadAnchor, "deploy/") {
					if d.Change != "changed" || *d.Before != "3" || *d.After != "1" {
						t.Fatalf("cross-paired patch: %+v", d)
					}
				} else {
					want := "changed"
					if side == "base" {
						want = "removed"
					}
					if side == "head" {
						want = "added"
					}
					if d.Change != want {
						t.Fatalf("patch delta: %+v", d)
					}
				}
			}
		})
	}
}
