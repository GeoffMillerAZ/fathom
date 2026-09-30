package main

// testmain_test.go — a package-wide safety net for user-scope config
// (docs/design/scoped-config.md) and session-instrument (production-
// tokenomics.md) code paths that default to $HOME (connector.UserIgnorePath,
// guard.go's loadUserGuardSet, sessionledger.go's sessionLedgerPath) OR to
// os.TempDir() (identcache.go's identCacheDir, editset.go's editSetDir,
// guardstamp.go's guardStampDir) — exactly the repo-wide "tests must not
// write to the real $HOME or the real machine temp dir" rule.
//
// Rather than require every test in this large, pre-existing package to
// individually override each of these, TestMain sets one hermetic HOME and
// one hermetic cache root for the whole binary before any test runs. A
// test that needs its OWN fresh override still calls t.Setenv/direct
// package-var reassignment exactly as before — both patterns save/restore
// relative to whatever TestMain already set (this hermetic baseline, never
// the real one), so per-test overrides remain fully correct and this
// change is purely additive safety.
//
// Critic finding F6 (BLOCKING, phase-P fix round): identCacheDir/
// editSetDir/hookcache.IdentDir all defaulted to os.TempDir() — safe only
// when TMPDIR is externally redirected (this repo's Taskfile does that for
// `task test`/`task validate`, but a bare `go test` does not), so a stray
// stamp could land in the REAL machine temp dir on any invocation outside
// that wrapper. Redirecting them here closes that gap unconditionally.
// RT-19 (docs/design/responsible-tenant.md §10.4): the HOME, state dir,
// call log and daemon URL now come from testisolate.Main, which refuses to
// run (exit 2) when it cannot isolate — before, a failed MkdirTemp silently
// kept the real HOME, and these tests appended rows to the operator's call
// log through the live daemon (INC-23). The package's own temp-dir caches
// are redirected under the isolated root by the setup callback.
import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/hookcache"
	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

func TestMain(m *testing.M) {
	// A re-exec'd helper (unmanaged_test.go) plays its role in the
	// environment its parent test already isolated.
	if role := os.Getenv(testHelperEnv); role != "" {
		os.Exit(runTestHelper(role))
	}
	testisolate.Main(m, func(root string) error {
		cacheRoot := filepath.Join(root, "cache")
		if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
			return err
		}
		identCacheDir = filepath.Join(cacheRoot, "ident-cache")
		hookcache.IdentDir = identCacheDir
		hookcache.HealthDir = filepath.Join(cacheRoot, "hook-health")
		hookcache.ToplevelDir = filepath.Join(cacheRoot, "toplevel-cache")
		hookcache.FileChangedDir = filepath.Join(cacheRoot, "filechanged")
		editSetDir = filepath.Join(cacheRoot, "edit-set")
		guardStampDir = filepath.Join(cacheRoot, "guard-stamps")
		// PE-23/PE-24: a root under the OS temp dir is a scratch clone, and a
		// ledger reader ignores it even when the registry lists it. The
		// isolated root itself sits in the OS temp dir, so the fixture fleet's
		// registered roots under HOME would all read as scratch. The OS temp
		// dir becomes a dedicated subdirectory of the isolated root: still
		// hermetic, and the fixtures' roots are not in it.
		tmp := filepath.Join(root, "tmp")
		if err := os.MkdirAll(tmp, 0o755); err != nil {
			return err
		}
		if err := os.Setenv("TMPDIR", tmp); err != nil {
			return err
		}
		// ADR-059: doctor's Claude Code load path never reads this
		// machine's managed settings or spawns the real claude binary in a
		// test; a test that needs a reading sets its own.
		claudeLoadEnv.managedSettings = func() []string { return nil }
		claudeLoadEnv.version = func(context.Context) (string, error) {
			return "", errors.New("not probed in tests")
		}
		return nil
	})
}
