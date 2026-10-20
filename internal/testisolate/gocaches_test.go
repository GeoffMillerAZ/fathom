package testisolate

import (
	"errors"
	"os"
	"testing"
)

// TestIsolateAndRun_KeepsGoCachesOutsideTheIsolatedHome: GOCACHE and
// GOMODCACHE default under HOME, so an isolated HOME used to hand every go
// command a test started (the typed pass's go/packages load) an empty build
// and module cache: every dependency recompiled and every module
// downloaded, and the CT-5 performance gate measured that instead of the
// typed pass (CI red from ab4cab36b). An unset cache is pinned to what the
// go command resolves while HOME is still the operator's; an inherited one
// is kept as it was.
func TestIsolateAndRun_KeepsGoCachesOutsideTheIsolatedHome(t *testing.T) {
	saveEnv(t)
	realHome := os.Getenv("HOME")
	t.Setenv("GOCACHE", "")
	t.Setenv("GOMODCACHE", "")
	var askedHome string
	var asked []string
	seen := map[string]string{}
	code := IsolateAndRun(runnerFunc(func() int {
		for _, k := range append([]string{"HOME"}, GoCacheVars...) {
			seen[k] = os.Getenv(k)
		}
		return 0
	}), Options{
		MkdirTemp: func(_, pattern string) (string, error) { return os.MkdirTemp(t.TempDir(), pattern) },
		GoEnv: func(keys []string) (map[string]string, error) {
			askedHome, asked = os.Getenv("HOME"), keys
			return map[string]string{"GOCACHE": "/operator/cache/go-build", "GOMODCACHE": "/operator/go/pkg/mod"}, nil
		},
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if askedHome != realHome {
		t.Errorf("go env was asked under HOME %q, want the operator's %q (before HOME moves)", askedHome, realHome)
	}
	if len(asked) != 2 {
		t.Errorf("go env asked for %v, want both unset caches", asked)
	}
	if seen["HOME"] == realHome {
		t.Errorf("HOME was not isolated")
	}
	if seen["GOCACHE"] != "/operator/cache/go-build" || seen["GOMODCACHE"] != "/operator/go/pkg/mod" {
		t.Errorf("suite saw GOCACHE=%q GOMODCACHE=%q, want the operator's resolved caches", seen["GOCACHE"], seen["GOMODCACHE"])
	}
}

// TestIsolateAndRun_GoCachesInheritedOrUnresolvable: an inherited cache is
// never re-asked or replaced, a relative answer is not pinned, and a go
// command that cannot answer pins nothing and refuses nothing (a suite
// that never runs go does not need the caches).
func TestIsolateAndRun_GoCachesInheritedOrUnresolvable(t *testing.T) {
	saveEnv(t)
	mk := func(_, pattern string) (string, error) { return os.MkdirTemp(t.TempDir(), pattern) }

	t.Setenv("GOCACHE", "/inherited/go-build")
	t.Setenv("GOMODCACHE", "")
	var asked []string
	var seenCache, seenMod string
	code := IsolateAndRun(runnerFunc(func() int {
		seenCache, seenMod = os.Getenv("GOCACHE"), os.Getenv("GOMODCACHE")
		return 0
	}), Options{MkdirTemp: mk, GoEnv: func(keys []string) (map[string]string, error) {
		asked = keys
		return map[string]string{"GOCACHE": "/wrong", "GOMODCACHE": "relative/mod"}, nil
	}})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if len(asked) != 1 || asked[0] != "GOMODCACHE" {
		t.Errorf("go env asked for %v, want only the unset GOMODCACHE", asked)
	}
	if seenCache != "/inherited/go-build" {
		t.Errorf("GOCACHE = %q, want the inherited value kept", seenCache)
	}
	if seenMod != "" {
		t.Errorf("GOMODCACHE = %q, want a relative answer left unpinned", seenMod)
	}

	t.Setenv("GOCACHE", "")
	ran := false
	code = IsolateAndRun(runnerFunc(func() int {
		ran = true
		seenCache = os.Getenv("GOCACHE")
		return 0
	}), Options{MkdirTemp: mk, GoEnv: func([]string) (map[string]string, error) {
		return nil, errors.New("exec: \"go\": executable file not found in $PATH")
	}})
	if code != 0 || !ran {
		t.Fatalf("an unanswerable go env refused the run: code=%d ran=%v", code, ran)
	}
	if seenCache != "" {
		t.Errorf("GOCACHE = %q, want unset when go env cannot answer", seenCache)
	}

	t.Setenv("GOCACHE", "/inherited/go-build")
	t.Setenv("GOMODCACHE", "/inherited/mod")
	called := false
	IsolateAndRun(runnerFunc(func() int { return 0 }), Options{MkdirTemp: mk, GoEnv: func([]string) (map[string]string, error) {
		called = true
		return nil, nil
	}})
	if called {
		t.Error("go env ran although every cache was inherited")
	}
}
