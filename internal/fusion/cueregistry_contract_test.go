package fusion

import (
	"context"
	"strings"
	"testing"

	"cuelang.org/go/mod/module"
)

// The typed CUE pass must never name a registry host: a nil load.Config.Registry
// (even with CUE_REGISTRY=inline:{}) builds the default resolver, which names
// the public default host.
func TestCUETypedLoadConfigNamesNoRegistryHost(t *testing.T) {
	cfg := cueTypedLoadConfig(t.TempDir())
	if cfg.Registry == nil {
		t.Fatal("typed CUE load has no explicit registry: cue/load would build the default one and name its public host")
	}
	for _, e := range cfg.Env {
		if strings.HasPrefix(e, "CUE_REGISTRY=") {
			t.Fatalf("typed CUE load still routes through CUE_REGISTRY: %q", e)
		}
	}
	reg := cfg.Registry
	ctx := context.Background()
	v := module.MustNewVersion("example.org/dep@v0", "v0.0.1")
	if _, err := reg.Requirements(ctx, v); err == nil {
		t.Fatal("Requirements did not refuse")
	}
	if _, err := reg.Fetch(ctx, v); err == nil {
		t.Fatal("Fetch did not refuse")
	}
	if _, err := reg.ModuleVersions(ctx, "example.org/dep@v0"); err == nil {
		t.Fatal("ModuleVersions did not refuse")
	}
}
