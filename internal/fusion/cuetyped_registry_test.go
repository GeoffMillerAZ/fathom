package fusion

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/mod/module"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func cueRegistryFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if files == nil {
		files = make(map[string]string)
	}
	files["cue.mod/module.cue"] = "module: \"example.org/native@v0\"\nlanguage: version: \"v0.16.1\"\n"
	for name, source := range files {
		name = filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// This preflight inspects the actual producer configuration before native load.
// It is safe on the old source: nil Registry fails here without constructing the
// default network/cache client, even when the fixture declares a remote module.
func requireCUELocalRegistry(t *testing.T, root string, cfg *load.Config) {
	t.Helper()
	if cfg.Registry == nil {
		t.Fatal("typed CUE producer left Registry nil; inline:{} does not disable the default registry")
	}
	if cfg.Dir != root || cfg.ModuleRoot != root {
		t.Fatalf("typed CUE producer root is not pinned: Dir=%q ModuleRoot=%q want %q", cfg.Dir, cfg.ModuleRoot, root)
	}
	if cfg.Env == nil || len(cfg.Env) != 0 {
		t.Fatalf("typed CUE producer retained ambient/registry environment: %q", cfg.Env)
	}
	if cfg.SkipImports {
		t.Fatal("registry refusal must preserve native local imports and builtins")
	}
}

func TestEnrichCUEEdgesExplicitRegistryPolicy(t *testing.T) {
	root := cueRegistryFixture(t, nil)
	called := false
	report, err := enrichCUEEdgesWithLoader(context.Background(), store.NewMemoryStore(), "cue-policy", root, nil,
		func(args []string, cfg *load.Config) []*build.Instance {
			called = true
			requireCUELocalRegistry(t, root, cfg)
			if len(args) != 1 || args[0] != "./..." {
				t.Fatalf("native package selection changed: %q", args)
			}
			return nil // configuration counterexample only; never invokes the old loader
		})
	if err != nil || !called || report.PackagesTotal != 0 {
		t.Fatalf("configuration probe did not reach producer loader: called=%v report=%+v err=%v", called, report, err)
	}
}

func TestEnrichCUEEdgesNativeLocalRegistryBoundary(t *testing.T) {
	root := cueRegistryFixture(t, map[string]string{
		"schema/schema.cue": "package schema\n#Name: \"local\"\n",
		"consumer/main.cue": "package consumer\nimport \"example.org/native/schema\"\nimport \"strings\"\n#Copy: schema.#Name\nout: strings.ToUpper(schema.#Name)\n",
	})
	cache := filepath.Join(t.TempDir(), "must-not-create-cache")
	t.Setenv("CUE_CACHE_DIR", cache)
	t.Setenv("CUE_REGISTRY", "file:must-not-open-ambient-registry-config")
	st := store.NewMemoryStore()
	loaded := false
	report, err := enrichCUEEdgesWithLoader(context.Background(), st, "cue-local", root, nil,
		func(args []string, cfg *load.Config) []*build.Instance {
			requireCUELocalRegistry(t, root, cfg)
			instances := load.Instances(args, cfg)
			for _, instance := range instances {
				if instance.PkgName != "consumer" {
					continue
				}
				loaded = true
				if instance.Err != nil {
					t.Fatalf("native consumer loading failed: %v", instance.Err)
				}
				value := cuecontext.New().BuildInstance(instance)
				got, valueErr := value.LookupPath(cue.ParsePath("out")).String()
				if valueErr != nil || got != "LOCAL" {
					t.Fatalf("local import + builtin = %q, %v; want LOCAL", got, valueErr)
				}
			}
			return instances
		})
	if err != nil || !loaded || report.PackagesFailed != 0 || report.PackagesLoaded != 2 {
		t.Fatalf("native local producer failed: loaded=%v report=%+v err=%v", loaded, report, err)
	}
	edges, err := st.TypedEdges(context.Background(), "cue-local")
	if err != nil {
		t.Fatal(err)
	}
	wantFrom := core.SymbolKey("cue-local", "consumer/consumer", "#Copy")
	wantTo := core.SymbolKey("cue-local", "schema/schema", "#Name")
	found := false
	for _, edge := range edges {
		if edge.Kind == core.EdgeDirect && edge.FromSymbol == wantFrom && edge.ToSymbol == wantTo {
			found = true
		}
	}
	if !found {
		t.Fatalf("native local definition edge missing: %s -> %s; edges=%+v", wantFrom, wantTo, edges)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatalf("producer created configured module cache: stat=%v", err)
	}
}

func TestEnrichCUEEdgesNativeRemoteRegistryRefused(t *testing.T) {
	root := cueRegistryFixture(t, map[string]string{
		"consumer/main.cue": "package consumer\nimport \"example.org/remote/pkg\"\n#Copy: pkg.#Name\n",
	})
	moduleSource := "module: \"example.org/native@v0\"\nlanguage: version: \"v0.16.1\"\ndeps: \"example.org/remote@v0\": v: \"v0.0.1\"\n"
	if err := os.WriteFile(filepath.Join(root, "cue.mod", "module.cue"), []byte(moduleSource), 0644); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(t.TempDir(), "must-not-create-cache")
	t.Setenv("CUE_CACHE_DIR", cache)
	t.Setenv("CUE_REGISTRY", "file:must-not-open-ambient-registry-config")
	st := store.NewMemoryStore()
	var failures []string
	report, err := enrichCUEEdgesWithLoader(context.Background(), st, "cue-remote", root, nil,
		func(args []string, cfg *load.Config) []*build.Instance {
			requireCUELocalRegistry(t, root, cfg)
			instances := load.Instances(args, cfg)
			for _, instance := range instances {
				if instance.Err != nil {
					failures = append(failures, instance.Err.Error())
				}
			}
			return instances
		})
	if err != nil || report.PackagesFailed == 0 || report.PackagesLoaded != 0 || report.EdgesDirect != 0 {
		t.Fatalf("remote package was not a counted native load refusal: report=%+v err=%v", report, err)
	}
	joined := strings.Join(failures, "\n")
	if !strings.Contains(joined, cueRegistryRefusalReason) || !strings.Contains(joined, "example.org/remote") {
		t.Fatalf("native remote refusal lost named cause/target: %q", joined)
	}
	edges, err := st.TypedEdges(context.Background(), "cue-remote")
	if err != nil || len(edges) != 0 {
		t.Fatalf("remote dependency emitted typed edges: edges=%+v err=%v", edges, err)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatalf("remote refusal created configured module cache: stat=%v", err)
	}
}

func TestCUELocalOnlyRegistryRefusesEveryOperation(t *testing.T) {
	registry := cueLocalOnlyRegistry{}
	version := module.MustNewVersion("example.org/remote@v0", "v0.0.1")
	ctx := context.Background()
	requirements, requirementsErr := registry.Requirements(ctx, version)
	location, fetchErr := registry.Fetch(ctx, version)
	versions, versionsErr := registry.ModuleVersions(ctx, "example.org/remote@v0")
	for _, test := range []struct {
		method string
		err    error
	}{
		{"Requirements", requirementsErr}, {"Fetch", fetchErr}, {"ModuleVersions", versionsErr},
	} {
		t.Run(test.method, func(t *testing.T) {
			if test.err == nil || !strings.Contains(test.err.Error(), cueRegistryRefusalReason) || !strings.Contains(test.err.Error(), test.method) || !strings.Contains(test.err.Error(), "example.org/remote@v0") {
				t.Fatalf("registry operation did not name refusal and target: %v", test.err)
			}
		})
	}
	if len(requirements) != 0 || len(versions) != 0 || location.FS != nil {
		t.Fatalf("refused registry operation returned module data: requirements=%v versions=%v location=%+v", requirements, versions, location)
	}
}
