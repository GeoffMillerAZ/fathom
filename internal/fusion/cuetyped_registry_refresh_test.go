package fusion

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/mod/module"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func writeCUERegistryFile(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestEnrichCUEEdgesRegistryDenialRemovesOnlyAffectedPriorRows(t *testing.T) {
	root := cueRegistryFixture(t, map[string]string{
		"schema/schema.cue":       "package schema\n#Name: string\n",
		"consumer/main.cue":       "package consumer\nimport \"example.org/native/schema\"\n#Copy: schema.#Name\n",
		"dependent/dependent.cue": "package dependent\nimport \"example.org/native/consumer\"\n#Copy: consumer.#Copy\n",
		"stable/stable.cue":       "package stable\nimport \"example.org/native/schema\"\n#Copy: schema.#Name\n",
	})
	ctx := context.Background()
	st := store.NewMemoryStore()
	const service = "cue-refresh"
	first, err := EnrichCUEEdges(ctx, st, service, root, nil)
	if err != nil || first.PackagesFailed != 0 || first.EdgesDirect != 3 {
		t.Fatalf("initial local native fixture did not establish three prior edges: report=%+v err=%v", first, err)
	}
	other := core.Edge{FromService: service, FromFile: "other.cs", FromSymbol: "other-source", ToSymbol: "other-target", Kind: core.EdgeDirect}
	if err := st.PutTypedEdges(ctx, service, "scip:other", []core.Edge{other}); err != nil {
		t.Fatal(err)
	}
	legacy := other
	legacy.FromSymbol = "legacy-consumer-source"
	if err := st.PutTypedEdges(ctx, service, "cue:"+filepath.Join(root, "consumer")+":consumer", []core.Edge{legacy}); err != nil {
		t.Fatal(err)
	}
	prior, err := st.ListTypedEdgePackages(ctx, service)
	if err != nil {
		t.Fatal(err)
	}
	writeCUERegistryFile(t, root, "cue.mod/module.cue", "module: \"example.org/native@v0\"\nlanguage: version: \"v0.16.1\"\ndeps: \"example.org/remote@v0\": v: \"v0.0.1\"\n")
	writeCUERegistryFile(t, root, "consumer/main.cue", "package consumer\nimport \"example.org/remote/pkg\"\n#Copy: pkg.#Name\n")
	var loads []string
	started := time.Now()
	second, err := enrichCUEEdgesWithLoader(ctx, st, service, root, nil, func(args []string, cfg *load.Config) []*build.Instance {
		loads = append(loads, args...)
		return load.Instances(args, cfg)
	})
	t.Logf("native denial and selective package inspection: loads=%q duration=%s", loads, time.Since(started))
	if len(loads) > 1+len(prior) { // at most one native retry per prior CUE identity
		t.Fatalf("denial inspection performed unbounded/repeated loads: %q, prior=%q", loads, prior)
	}
	if err != nil || second.PackagesFailed != 1 || second.PackagesLoaded != 0 {
		t.Fatalf("expected native synthetic whole-load refusal: report=%+v err=%v", second, err)
	}
	edges, err := st.TypedEdges(ctx, service)
	if err != nil {
		t.Fatal(err)
	}
	var stable, unrelated bool
	for _, edge := range edges {
		if edge.FromFile == "consumer/main.cue" || edge.FromFile == "dependent/dependent.cue" || edge.FromSymbol == legacy.FromSymbol {
			t.Errorf("registry-denied package retained old typed evidence: %+v", edge)
		}
		stable = stable || edge.FromFile == "stable/stable.cue"
		unrelated = unrelated || edge.FromSymbol == other.FromSymbol
	}
	if !stable || !unrelated {
		t.Fatalf("refusal removed unrelated/local evidence: stable=%v unrelated=%v edges=%+v", stable, unrelated, edges)
	}
}

func TestCUERegistryInspectionRefusesUnsafeIdentities(t *testing.T) {
	root := cueRegistryFixture(t, map[string]string{"consumer/main.cue": "package consumer\n#A: string\n"})
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, id, want string
	}{
		{"native", "example.org/native/consumer@v0", "example.org/native/consumer@v0"},
		{"native-qualified", "example.org/native/consumer@v0:consumer", "example.org/native/consumer@v0:consumer"},
		{"in-root-fallback", filepath.Join(root, "consumer") + ":consumer", "./consumer:consumer"},
		{"outside-fallback", outside + ":consumer", ""},
		{"symlink-fallback", filepath.Join(root, "escape") + ":consumer", ""},
		{"file-fallback", filepath.Join(root, "consumer", "main.cue") + ":consumer", ""},
		{"invalid-fallback-name", filepath.Join(root, "consumer") + ":bad/name", ""},
		{"invalid-qualifier", "example.org/native/consumer:bad/name", ""},
		{"traversal", "../outside", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := cueRegistryInspectionArg(root, test.id)
			if test.want == "" {
				if err == nil {
					t.Fatalf("unsafe/invalid identity was admitted: %q -> %q", test.id, got)
				}
			} else if err != nil || got != test.want {
				t.Fatalf("native identity conversion = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

type cueRegistryListFailStore struct{ store.Store }

func (cueRegistryListFailStore) ListTypedEdgePackages(context.Context, string) ([]string, error) {
	return nil, errors.New("fixture list failure")
}

type cueRegistryDeleteFailStore struct{ store.Store }

func (cueRegistryDeleteFailStore) DeleteTypedEdgePackage(context.Context, string, string) error {
	return errors.New("fixture delete failure")
}

type cueRegistryCancelListStore struct {
	store.Store
	cancel context.CancelFunc
}

func (s cueRegistryCancelListStore) ListTypedEdgePackages(ctx context.Context, service string) ([]string, error) {
	s.cancel()
	return s.Store.ListTypedEdgePackages(context.WithoutCancel(ctx), service)
}

func TestCUERegistryIncompleteInspectionLeavesNoCompletedPass(t *testing.T) {
	for _, name := range []string{"list-failed", "cancelled-after-load"} {
		t.Run(name, func(t *testing.T) {
			root := cueRegistryFixture(t, map[string]string{
				"consumer/main.cue": "package consumer\nimport \"example.org/remote/pkg\"\n#Copy: pkg.#Name\n",
			})
			writeCUERegistryFile(t, root, "cue.mod/module.cue", "module: \"example.org/native@v0\"\nlanguage: version: \"v0.16.1\"\ndeps: \"example.org/remote@v0\": v: \"v0.0.1\"\n")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			memory := store.NewMemoryStore()
			if err := memory.PutTypedEdges(ctx, "cue-incomplete", "cue:example.org/native/consumer@v0:consumer", []core.Edge{{FromSymbol: "residual-prior-row"}}); err != nil {
				t.Fatal(err)
			}
			var st store.Store = cueRegistryListFailStore{memory}
			if name == "cancelled-after-load" {
				st = cueRegistryCancelListStore{memory, cancel}
			}
			pipeline := New(connector.NewLocalConnector(), st, Config{})
			_, _ = pipeline.enrichCUEEdges(ctx, core.ServiceRef{ID: "cue-incomplete", Path: root}, []core.SourceRef{{Path: "consumer/main.cue"}}, nil)
			if pipeline.CUEPassTotal() != 1 {
				t.Fatalf("native pass did not execute: %d", pipeline.CUEPassTotal())
			}
			fresh := context.Background()
			if pipeline.passStateOK(fresh, "cue-incomplete", cuePassStateKey) || !cuePassNeeded(nil, pipeline.passStateOK(fresh, "cue-incomplete", cuePassStateKey)) {
				t.Fatal("incomplete inspection acquired a completed/partial pass stamp and would skip retry")
			}
			edges, err := memory.TypedEdges(fresh, "cue-incomplete")
			if err != nil || len(edges) != 1 {
				t.Fatalf("fixture lost its explicitly residual evidence: edges=%+v err=%v", edges, err)
			}
		})
	}
}

func TestCUERegistryInspectionNamesIncompleteWork(t *testing.T) {
	ctx := context.Background()
	root := cueRegistryFixture(t, nil)
	st := store.NewMemoryStore()
	if err := st.PutTypedEdges(ctx, "cue-inspection", "cue:example.org/native/consumer@v0", []core.Edge{{FromSymbol: "prior"}}); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	for _, test := range []struct {
		name string
		ctx  context.Context
		st   store.Store
	}{
		{"cancelled", cancelled, st},
		{"list-failed", ctx, cueRegistryListFailStore{st}},
		{"empty-native-result", ctx, st},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			err := invalidateCUEDeniedPackages(test.ctx, test.st, "cue-inspection", root, func([]string, *load.Config) []*build.Instance {
				calls++
				return nil
			})
			if err == nil || !strings.Contains(err.Error(), "CUE registry denial inspection incomplete") {
				t.Fatalf("incomplete inspection was not named: calls=%d err=%v", calls, err)
			}
			if test.name != "empty-native-result" && calls != 0 {
				t.Fatalf("refused inspection still called native loader %d times", calls)
			}
		})
	}
	t.Run("delete-failed", func(t *testing.T) {
		err := invalidateCUEDeniedPackages(ctx, cueRegistryDeleteFailStore{st}, "cue-inspection", root, func(_ []string, cfg *load.Config) []*build.Instance {
			_, _ = cfg.Registry.Fetch(ctx, module.MustNewVersion("example.org/remote@v0", "v0.0.1"))
			return []*build.Instance{{}}
		})
		if err == nil || !strings.Contains(err.Error(), "CUE registry denial invalidation failed") {
			t.Fatalf("failed invalidation was not named: %v", err)
		}
	})
}

func TestCUENativePolicyEpochRechecksPrePolicyStores(t *testing.T) {
	root := cueRegistryFixture(t, map[string]string{
		"consumer/main.cue": "package consumer\n#Base: string\n#Copy: #Base\n",
	})
	ctx := context.Background()
	st := store.NewMemoryStore()
	pipeline := New(connector.NewLocalConnector(), st, Config{})
	gateIngest(t, pipeline, root)
	if report := gateIngest(t, pipeline, root); pipeline.CUEPassTotal() != 1 || report.CUESkipped == "" {
		t.Fatalf("same-policy unchanged tree was not gated: count=%d report=%+v", pipeline.CUEPassTotal(), report)
	}
	for _, oldState := range []string{passStateOK, passStatePartial, passStateOK + "@epoch-0"} {
		if err := st.SetTypedEdgePackageFingerprint(ctx, "gate", cuePassStateKey, oldState); err != nil {
			t.Fatal(err)
		}
		want := pipeline.CUEPassTotal() + 1
		if report := gateIngest(t, pipeline, root); pipeline.CUEPassTotal() != want || report.CUESkipped != "" {
			t.Fatalf("pre-policy state %q skipped the new registry policy: count=%d want=%d report=%+v", oldState, pipeline.CUEPassTotal(), want, report)
		}
		if report := gateIngest(t, pipeline, root); pipeline.CUEPassTotal() != want || report.CUESkipped == "" {
			t.Fatalf("current-policy result did not gate: count=%d report=%+v", pipeline.CUEPassTotal(), report)
		}
	}
}

func TestEnrichCUEEdgesRegistryDenialClearsPriorRowsOnWholesaleFailure(t *testing.T) {
	root := cueRegistryFixture(t, map[string]string{
		"consumer/main.cue": "package consumer\n#Base: string\n#Copy: #Base\n",
	})
	ctx := context.Background()
	st := store.NewMemoryStore()
	first, err := EnrichCUEEdges(ctx, st, "cue-wholesale", root, nil)
	if err != nil || first.EdgesDirect != 1 || first.PackagesFailed != 0 {
		t.Fatalf("initial local typed evidence missing: report=%+v err=%v", first, err)
	}
	writeCUERegistryFile(t, root, "cue.mod/module.cue", "module: \"example.org/native@v0\"\nlanguage: version: \"v0.16.1\"\ndeps: \"example.org/remote@v0\": v: \"v0.0.1\"\n")
	writeCUERegistryFile(t, root, "consumer/main.cue", "package consumer\nimport \"example.org/remote/pkg\"\n#Copy: pkg.#Name\n")
	second, err := EnrichCUEEdges(ctx, st, "cue-wholesale", root, nil)
	if err != nil || second.PackagesFailed == 0 || second.PackagesLoaded != 0 {
		t.Fatalf("fixture did not reach wholesale registry denial: report=%+v err=%v", second, err)
	}
	edges, err := st.TypedEdges(ctx, "cue-wholesale")
	if err != nil || len(edges) != 0 {
		t.Fatalf("wholesale registry denial retained prior typed evidence: edges=%+v err=%v", edges, err)
	}
}

func TestEnrichCUEEdgesOrdinaryCompileFailureKeepsExistingPolicy(t *testing.T) {
	root := cueRegistryFixture(t, map[string]string{
		"consumer/main.cue": "package consumer\n#Base: string\n#Copy: #Base\n",
	})
	ctx := context.Background()
	st := store.NewMemoryStore()
	first, err := EnrichCUEEdges(ctx, st, "cue-syntax", root, nil)
	if err != nil || first.EdgesDirect != 1 || first.PackagesFailed != 0 {
		t.Fatalf("initial local typed evidence missing: report=%+v err=%v", first, err)
	}
	// A diagnostic-looking string cannot classify an unrelated parser failure.
	writeCUERegistryFile(t, root, "consumer/main.cue", "package consumer\nmessage: \""+cueRegistryRefusalReason+"\"\n#Base: {\n")
	second, err := EnrichCUEEdges(ctx, st, "cue-syntax", root, nil)
	if err != nil || second.PackagesFailed == 0 {
		t.Fatalf("fixture did not reach ordinary parsing failure: report=%+v err=%v", second, err)
	}
	edges, err := st.TypedEdges(ctx, "cue-syntax")
	if err != nil || len(edges) != 1 {
		t.Fatalf("registry policy broadened ordinary compile-failure retention: edges=%+v err=%v", edges, err)
	}
}
