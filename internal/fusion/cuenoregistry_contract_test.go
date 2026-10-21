package fusion

// Native CUE API controls, separate from the producer wiring regressions.
// These never construct the default registry or operator module cache.
import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/mod/modconfig"
	"cuelang.org/go/mod/module"
)

// Characterization of the exact unsafe assumption. NewResolver resolves only
// locations; unlike NewRegistry it neither opens the module cache nor fetches.
func TestCUENativeLegacyInlineEmptyRegistryStillNamesDefaultHost(t *testing.T) {
	resolver, err := modconfig.NewResolver(&modconfig.Config{Env: []string{"CUE_REGISTRY=inline:{}"}})
	if err != nil {
		t.Fatal(err)
	}
	hosts := resolver.AllHosts()
	if len(hosts) != 1 || hosts[0].Name != modconfig.DefaultRegistry {
		t.Fatalf("pinned loader's inline-empty fallback changed: hosts=%+v; re-audit the source before treating it as offline", hosts)
	}
}

// This minimal test double implements the public load.Config.Registry contract.
// It does not delegate to modconfig.NewRegistry, modcache, HTTP or credentials.
// The producer has a separate zero-state refusal implementation; recording names
// here is confined to a temp module and verifies the pinned native API contract.
type cueRefusingRegistryFixture struct {
	mu    sync.Mutex
	calls []string
}

func (r *cueRefusingRegistryFixture) refuse(method, target string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, method+" "+target)
	return fmt.Errorf("%s (%s %s)", cueRegistryRefusalReason, method, target)
}

func (r *cueRefusingRegistryFixture) Requirements(_ context.Context, target module.Version) ([]module.Version, error) {
	return nil, r.refuse("Requirements", target.String())
}

func (r *cueRefusingRegistryFixture) Fetch(_ context.Context, target module.Version) (module.SourceLoc, error) {
	return module.SourceLoc{}, r.refuse("Fetch", target.String())
}

func (r *cueRefusingRegistryFixture) ModuleVersions(_ context.Context, target string) ([]string, error) {
	return nil, r.refuse("ModuleVersions", target)
}

func (r *cueRefusingRegistryFixture) attempted() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

var _ modconfig.Registry = (*cueRefusingRegistryFixture)(nil)

func TestCUENativeExplicitDenyRegistryPreservesLocalImportsAndBuiltinEvaluation(t *testing.T) {
	root := cueRegistryFixture(t, map[string]string{
		"schema/schema.cue": "package schema\n#Name: \"local\"\n",
		"consumer/main.cue": "package consumer\nimport \"example.org/native/schema\"\nimport \"strings\"\nout: strings.ToUpper(schema.#Name)\n",
	})
	registry := &cueRefusingRegistryFixture{}
	instances := load.Instances([]string{"./consumer"}, &load.Config{
		Dir: root, ModuleRoot: root, Registry: registry,
		// An explicit registry must win over environment registry config.
		Env: []string{"CUE_REGISTRY=file:must-not-open-ambient-registry-config"},
	})
	if len(instances) != 1 || instances[0].Err != nil {
		t.Fatalf("ordinary local package refused: %+v", instances)
	}
	value := cuecontext.New().BuildInstance(instances[0])
	if err := value.Err(); err != nil {
		t.Fatalf("native local/builtin build failed: %v", err)
	}
	got, err := value.LookupPath(cue.ParsePath("out")).String()
	if err != nil || got != "LOCAL" {
		t.Fatalf("native local import + builtin evaluated to %q, %v; want LOCAL", got, err)
	}
	if calls := registry.attempted(); len(calls) != 0 {
		t.Fatalf("local module/builtin consulted registry: %v", calls)
	}
}

func TestCUENativeExplicitDenyRegistryRefusesRemoteDependencyByName(t *testing.T) {
	root := cueRegistryFixture(t, map[string]string{
		"consumer/main.cue": "package consumer\nimport \"example.org/remote/pkg\"\nout: pkg.#Name\n",
	})
	moduleSource := "module: \"example.org/native@v0\"\nlanguage: version: \"v0.16.1\"\ndeps: \"example.org/remote@v0\": v: \"v0.0.1\"\n"
	// cueRegistryFixture deliberately owns only fixture files; overwrite its module
	// declaration here to pin a remote dependency, without touching any cache.
	if err := os.WriteFile(filepath.Join(root, "cue.mod", "module.cue"), []byte(moduleSource), 0644); err != nil {
		t.Fatal(err)
	}
	registry := &cueRefusingRegistryFixture{}
	instances := load.Instances([]string{"./consumer"}, &load.Config{
		Dir: root, ModuleRoot: root, Registry: registry,
	})
	var failures []string
	for _, instance := range instances {
		if instance.Err != nil {
			failures = append(failures, instance.Err.Error())
		}
		if err := cuecontext.New().BuildInstance(instance).Err(); err != nil {
			failures = append(failures, err.Error())
		}
	}
	joined := strings.Join(failures, "\n")
	if !strings.Contains(joined, cueRegistryRefusalReason) || !strings.Contains(joined, "example.org/remote") {
		t.Fatalf("remote dependency refusal lost its cause or target: %q; registry=%v", joined, registry.attempted())
	}
	if calls := registry.attempted(); len(calls) == 0 {
		t.Fatal("remote fixture did not exercise the explicit registry boundary")
	}
}
