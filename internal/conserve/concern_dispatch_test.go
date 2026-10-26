package conserve

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func concernManifest(name string, replicas int) string {
	return fmt.Sprintf("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: %s\nspec:\n  replicas: %d\n", name, replicas)
}

func concernIngest(t *testing.T, repo *inputTestRepo, st store.Store) {
	t.Helper()
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{DisableTypedEdges: true, Rehash: true})
	defer p.Close()
	if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "svc", Path: repo.root}); err != nil {
		t.Fatal(err)
	}
}

func concernInput(t *testing.T, repo *inputTestRepo, base string, st store.Store) Input {
	t.Helper()
	in, err := NewInput(context.Background(), repo.root, base, st, "svc", nil)
	if err != nil {
		t.Fatal(err)
	}
	return in
}

func TestConcernDeltasIngestDispatch(t *testing.T) {
	for _, tc := range []struct{ name, path, reason string }{
		{"helm", "chart/templates/api.yaml", "helm-template-yaml"},
		{"historical-chart", "chart/templates/api.yaml", "helm-template-yaml"},
		{"base-ignored", "deploy/api.yaml", "ignored"},
		{"json", "deploy/api.json", "Kubernetes"},
		{"testdata", "testdata/api.yaml", "always-skipped"},
		{"vendor", "vendor/api.yaml", "always-skipped"},
		{"ignored", "ignored/api.yaml", "ignored"},
		{"oversize", "deploy/large.yaml", "oversize"},
		{"head-tool-kind", "deploy/api.yaml", "stored"},
		{"text", "notes.txt", "Kubernetes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newInputTestRepo(t)
			r.write("chart/Chart.yaml", "apiVersion: v2\nname: app\nversion: 1.0.0\n")
			r.write(".fathomignore", "ignored/\n")
			content := func(n int) string {
				if tc.name == "json" {
					return fmt.Sprintf(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"api"},"spec":{"replicas":%d}}`, n)
				}
				s := concernManifest("api", n)
				if tc.name == "oversize" {
					s += "#" + strings.Repeat(" ", connector.MaxFetchSize) + "\n"
				}
				return s
			}
			if tc.name == "base-ignored" {
				r.write(".fathomignore", "deploy/\n")
			}
			r.write(tc.path, content(3))
			r.commitAll("base")
			base := r.headSHA()
			if tc.name == "historical-chart" {
				r.write(".fathomignore", "chart/Chart.yaml\n")
			}
			if tc.name == "base-ignored" {
				r.write(".fathomignore", "")
			}
			r.write(tc.path, content(1))
			r.commitAll("head")
			st := store.NewMemoryStore()
			concernIngest(t, r, st)
			if tc.name == "head-tool-kind" {
				obs, err := st.QueryObservations(context.Background(), "svc", "")
				if err != nil {
					t.Fatal(err)
				}
				for i := range obs {
					obs[i].ToolKind = "helm-template-yaml"
				}
				if err := st.PurgeSource(context.Background(), "svc", tc.path); err != nil {
					t.Fatal(err)
				}
				if err := st.PutObservations(context.Background(), obs); err != nil {
					t.Fatal(err)
				}
			}
			rep, err := ConcernDeltas(context.Background(), concernInput(t, r, base, st))
			if err != nil {
				t.Fatal(err)
			}
			if len(rep.Deltas) != 0 {
				t.Errorf("non-Kubernetes deltas: %+v", rep.Deltas)
			}
			found := false
			for _, s := range rep.Skipped {
				if s.Path == tc.path && strings.Contains(s.Reason, tc.reason) {
					found = true
				}
			}
			if !found {
				t.Errorf("missing skip reason %q for %s: %+v", tc.reason, tc.path, rep.Skipped)
			}
		})
	}
}
