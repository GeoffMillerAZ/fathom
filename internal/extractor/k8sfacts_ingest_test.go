package extractor_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestIngest_K8sContainerFactsReachStore (LB-F451): ingest dispatches a
// Kubernetes manifest to K8sExtractor, and its facts are stored as
// observations of the manifest's own file, at their lines; no stored row
// carries the fixture's env values, arg values or exec command.
func TestIngest_K8sContainerFactsReachStore(t *testing.T) {
	src, err := os.ReadFile(k8sFactsFixture)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "deploy"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "deploy", "workloads.yaml"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	st := store.NewMemoryStore()
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "facts", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "facts", "")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, o := range obs {
		row := fmt.Sprintf("%s %s %s %s %v", o.SourceRef, o.Symbol, o.Claim, o.Package, o.Calls)
		if strings.Contains(row, k8sSentinel) {
			t.Errorf("a stored row carries an env value, arg value or exec command: %s", row)
		}
		if o.SourceKind != core.KindK8sFact {
			continue
		}
		if o.SourceRef != "deploy/workloads.yaml" || o.ToolKind != "k8s" {
			t.Errorf("fact stored off its manifest: %+v", o)
		}
		got = append(got, fmt.Sprintf("%d %s %s", o.Line, o.Symbol, o.Claim))
	}
	sort.SliceStable(got, func(i, j int) bool {
		var a, b int
		fmt.Sscan(got[i], &a)
		fmt.Sscan(got[j], &b)
		if a != b {
			return a < b
		}
		return got[i] < got[j]
	})
	if !reflect.DeepEqual(got, wantK8sFacts) {
		t.Errorf("stored facts\n got: %q\nwant: %q", got, wantK8sFacts)
	}
}
