package ask

import (
	"strings"
	"testing"
)

// TestComputeEnvIsOffline (ADR-083): ask's module loads run the go command
// offline whatever environment they start from: GOPROXY=off, GOSUMDB=off,
// GONOPROXY=",", GOTELEMETRY=off, no inherited -mod=mod, GOTOOLCHAIN=local
// last, and
// GOWORK=off for the default environment. A caller's own environment gets
// the same overrides on top.
func TestComputeEnvIsOffline(t *testing.T) {
	t.Setenv("FATHOM_HOME", t.TempDir()) // no user go_network opt-out
	t.Setenv("GOPROXY", "https://proxy.example.test")
	t.Setenv("GOFLAGS", "-mod=mod -tags=x")
	lastOf := func(env []string, key string) string {
		v := ""
		for _, kv := range env {
			if k, val, ok := strings.Cut(kv, "="); ok && k == key {
				v = val
			}
		}
		return v
	}
	for name, env := range map[string][]string{
		"default": computeEnv(nil),
		"caller":  computeEnv([]string{"GOPROXY=https://proxy.example.test", "GOFLAGS=-mod=mod"}),
	} {
		for key, want := range map[string]string{"GOPROXY": "off", "GOSUMDB": "off", "GONOPROXY": ",", "GOTELEMETRY": "off", "GOTOOLCHAIN": "local"} {
			if got := lastOf(env, key); got != want {
				t.Errorf("%s env: %s = %q, want %q", name, key, got, want)
			}
		}
		if f := lastOf(env, "GOFLAGS"); strings.Contains(f, "-mod=mod") {
			t.Errorf("%s env: GOFLAGS = %q keeps -mod=mod", name, f)
		}
		if env[len(env)-1] != "GOTOOLCHAIN=local" {
			t.Errorf("%s env ends %q, want GOTOOLCHAIN=local last", name, env[len(env)-1])
		}
	}
	if got := lastOf(computeEnv(nil), "GOWORK"); got != "off" {
		t.Errorf("default env: GOWORK = %q, want off", got)
	}
	if got := lastOf(computeEnv(nil), "GOFLAGS"); got != "-tags=x" {
		t.Errorf("default env: GOFLAGS = %q, want -tags=x kept", got)
	}
}
