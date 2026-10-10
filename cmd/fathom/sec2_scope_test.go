package main

// sec2_scope_test.go — SEC-2 (ADR-042 amendment 2026-10-09, docs/plans/
// config-experience.md CF-16): a repository's .fathom/config.toml never
// makes fathom run an executable it names, nor write a path outside the
// repository, on any automatic path. Each test plants a fixture repository
// whose config names ./evil.sh, a script that would create a sentinel file,
// drives the real entry point, and asserts the sentinel stays absent and
// the field is named in the ignored list.

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/statedir"
)

// evilRepo is a repository holding ./evil.sh (which creates the returned
// sentinel when run) and the given .fathom/config.toml. FATHOM_HOME is a
// fresh empty directory, so the user layer names nothing.
func evilRepo(t *testing.T, configTOML string) (repo, sentinel string) {
	t.Helper()
	t.Setenv("FATHOM_HOME", t.TempDir())
	repo = t.TempDir()
	sentinel = filepath.Join(t.TempDir(), "sentinel")
	// echo and the redirect are shell builtins: the script needs no PATH.
	script := "#!/bin/sh\necho ran > " + sentinel + "\nexit 1\n"
	if err := os.WriteFile(filepath.Join(repo, "evil.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".fathom", "config.toml"), []byte(configTOML), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo, sentinel
}

func assertSentinelAbsent(t *testing.T, sentinel, path string) {
	t.Helper()
	if _, err := os.Stat(sentinel); err == nil {
		t.Errorf("%s ran the repository's ./evil.sh: %s exists", path, sentinel)
		_ = os.Remove(sentinel) // so the next path is judged on its own
	}
}

func assertIgnored(t *testing.T, loaded config.Loaded, field string) {
	t.Helper()
	for _, ig := range loaded.IgnoredKeySources {
		if ig.Field == field {
			return
		}
	}
	t.Errorf("%s is not named in the ignored list: %+v", field, loaded.IgnoredKeySources)
}

// TestSEC2_RepoBinNeverRunsOnAnyProfilePath drives profiles.*.bin through
// every path that builds a profile client from a repository's config:
// serve-http's startup compile lane (wireLLM), the decision port's llm lane
// that serve-http and serve-mcp open at startup (openDecisionPorts, whose
// obligations re-rank then rides every edit hook), and doctor's providers
// row. The codex-cli lane execs its binary at construction (its Ping), so
// before SEC-2 each path ran ./evil.sh.
func TestSEC2_RepoBinNeverRunsOnAnyProfilePath(t *testing.T) {
	repo, sentinel := evilRepo(t, `default_profile = "evil"

[profiles.evil]
provider = "codex-cli"
bin      = "./evil.sh"

[decide]
provider = "llm"
profile  = "evil"
`)
	for _, v := range []string{"FATHOM_CODEX_BIN", "FATHOM_CLAUDE_BIN", "FATHOM_LLM_PROVIDER", "FATHOM_COMPILE_MODEL", "ANTHROPIC_API_KEY"} {
		t.Setenv(v, "")
	}
	// No codex on PATH: with the repository's bin dropped, the lane fails
	// to resolve "codex" and runs nothing at all.
	t.Setenv("PATH", t.TempDir())
	t.Chdir(repo) // serve-http and serve-mcp read the project layer from their working directory

	loaded, err := config.Load(config.DefaultUserPath(), config.DefaultProjectPath())
	if err != nil {
		t.Fatal(err)
	}
	assertIgnored(t, loaded, "profiles.evil.bin")

	r := &serveRun{flog: dlog.New(io.Discard, dlog.LevelInfo, time.Now), loaded: loaded}
	r.wireLLM()
	assertSentinelAbsent(t, sentinel, "serve-http startup (wireLLM)")

	_ = openDecisionPorts(loaded, loaded.Config.DefaultProfile, "", "serve-mcp", io.Discard)
	assertSentinelAbsent(t, sentinel, "the decision port's llm lane (openDecisionPorts)")

	_ = doctorProvidersRow(repo, buildLLMClientForProfile)
	assertSentinelAbsent(t, sentinel, "doctor's providers row")
}

// TestSEC2_FindingsOutboxNeverWritesOutsideTheRepo drives the session
// hook's outbox sync (syncFindingsOutbox, SessionStart and SubagentStart):
// before SEC-2 a repository's [daemon] store and an absolute or escaping
// [findings] outbox made it exec `fathom findings import --store <a path the
// repository chose>`. The fake runner stands in for that import and creates
// the sentinel where the real one would create the store.
func TestSEC2_FindingsOutboxNeverWritesOutsideTheRepo(t *testing.T) {
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "outbox.json"), []byte(`{"findings":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	evilStore := filepath.Join(outside, "evil.sqlite")
	runner := func(t *testing.T) *[]string {
		var stores []string
		withFindingsImportRunner(t, func(exe, store, service, subject, outbox string) ([]byte, int, error) {
			stores = append(stores, store)
			_ = os.WriteFile(store, []byte("ran"), 0o644)
			return []byte(`{"delivered":0,"duplicate":0,"stale":0,"rejected":[]}`), 0, nil
		})
		return &stores
	}

	for _, tc := range []struct {
		name, outbox, wantErr string
	}{
		{"absolute outbox", filepath.Join(outside, "outbox.json"), "absolute path"},
		{"escaping outbox", "../" + filepath.Base(outside) + "/outbox.json", "escapes the repository"},
		{"in-repo outbox, repository store", "outbox.json", "~/.fathom/config.toml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, _ := evilRepo(t, "[daemon]\nstore = "+tomlQuote(evilStore)+"\n\n[findings]\nsubject = \"org/repo/comp\"\noutbox = "+tomlQuote(tc.outbox)+"\n")
			// The escaping case needs the repository beside the outside dir.
			if strings.HasPrefix(tc.outbox, "../") {
				repo = filepath.Join(filepath.Dir(outside), "repo-"+filepath.Base(outside))
				if err := os.MkdirAll(filepath.Join(repo, ".fathom"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(repo, ".fathom", "config.toml"), []byte("[daemon]\nstore = "+tomlQuote(evilStore)+"\n\n[findings]\nsubject = \"org/repo/comp\"\noutbox = "+tomlQuote(tc.outbox)+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(repo, "outbox.json"), []byte(`{"findings":[]}`), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv(statedir.EnvOverride, filepath.Join(t.TempDir(), "state"))
			srv := indexedDaemon(t, repo, filepath.Base(repo))
			defer srv.Close()
			stores := runner(t)

			got := syncFindingsOutbox(srv.URL, repo)
			if !strings.Contains(got, "sync failed") || !strings.Contains(got, tc.wantErr) {
				t.Errorf("brief line = %q, want a sync-failed line naming %q", got, tc.wantErr)
			}
			if len(*stores) != 0 {
				t.Errorf("findings import ran with --store %v", *stores)
			}
			assertSentinelAbsent(t, evilStore, "the session hook's findings outbox sync")

			loaded, err := config.Load(config.DefaultUserPath(), filepath.Join(repo, ".fathom", "config.toml"))
			if err != nil {
				t.Fatal(err)
			}
			assertIgnored(t, loaded, "daemon.store")
		})
	}

	t.Run("control: the user's store and an in-repo outbox import", func(t *testing.T) {
		repo, _ := evilRepo(t, "[findings]\nsubject = \"org/repo/comp\"\noutbox = \"outbox.json\"\n")
		userStore := filepath.Join(t.TempDir(), "fleet.sqlite")
		if err := os.WriteFile(config.DefaultUserPath(), []byte("[daemon]\nstore = "+tomlQuote(userStore)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, "outbox.json"), []byte(`{"findings":[]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv(statedir.EnvOverride, filepath.Join(t.TempDir(), "state"))
		srv := indexedDaemon(t, repo, filepath.Base(repo))
		defer srv.Close()
		stores := runner(t)
		if got := syncFindingsOutbox(srv.URL, repo); !strings.HasPrefix(got, "findings outbox: imported") {
			t.Fatalf("got %q, want an import", got)
		}
		if len(*stores) != 1 || (*stores)[0] != userStore {
			t.Errorf("--store = %v, want the user's %s", *stores, userStore)
		}
	})
}

// TestSEC2_TestRunHookRunsNothingTheRepositoryNames drives the test-run
// hook gate with a repository that declares ./evil.sh as a test command,
// switches the hook on, and names an evil profile binary and decision
// lane: the hook reads the output of a command the harness already ran
// (TR-X2) and runs none of them. hooks.test_run and tests.commands stay
// repository behaviour (scope.go); profiles.*.bin is still dropped.
func TestSEC2_TestRunHookRunsNothingTheRepositoryNames(t *testing.T) {
	isolateHookRun(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
	t.Setenv("PATH", "/usr/bin:/bin") // git for the hook's diff read; no codex
	repo, sentinel := evilRepo(t, `default_profile = "evil"

[profiles.evil]
provider = "codex-cli"
bin      = "./evil.sh"

[decide]
provider = "llm"
profile  = "evil"

[hooks]
test_run = true

[tests]
commands = ["./evil.sh"]
`)
	gitInitTempRepo(t, repo)
	srv, _ := countingDaemon(t)
	for _, tc := range []struct{ event, payload string }{
		{"test-run", testRunPostPayload(repo, "./evil.sh 2>&1 | tail -20", testRunFailingOutput)},
		{"failed-test-run", testRunFailurePayload(repo, "./evil.sh", testRunFailingOutput)},
	} {
		code, out := runTestRunHook(t, tc.event, repo, srv.URL, tc.payload)
		if code != 0 {
			t.Errorf("%s: exit %d", tc.event, code)
		}
		if !strings.Contains(out, "fathom test run") {
			t.Errorf("%s: output %q, want the hook to read the failing run (the gate opened)", tc.event, out)
		}
	}
	assertSentinelAbsent(t, sentinel, "the test-run hook")
	loaded, err := config.Load(config.DefaultUserPath(), filepath.Join(repo, ".fathom", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	assertIgnored(t, loaded, "profiles.evil.bin")
}

// TestSEC2_IngestRefusesResultsDirOutsideTheRepo is CF-17 at the plain
// ingest's results_dir pickup (ingestConfiguredTestResults, every `fathom
// ingest` and the daemon's re-ingest): an absolute value, a .. escape and a
// symlink out are refused by name, and the envelopes beyond are never read
// into the store.
func TestSEC2_IngestRefusesResultsDirOutsideTheRepo(t *testing.T) {
	t.Setenv("FATHOM_HOME", t.TempDir())
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "run.json"), []byte(testsIngestFixtureEnvelope("2026-09-23T00:00:00Z")), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, value string
		want        error
	}{
		{"absolute", outside, config.ErrRepoPathAbsolute},
		{"dotdot", "../" + filepath.Base(outside), config.ErrRepoPathEscapes},
		{"symlink", "linked", config.ErrRepoPathSymlink},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, err := os.MkdirTemp(filepath.Dir(outside), "repo-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(root) })
			if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(root, ".fathom"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".fathom", "config.toml"), []byte("[tests]\nresults_dir = "+tomlQuote(tc.value)+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Chdir(root)
			storePath := filepath.Join(t.TempDir(), "fathom.sqlite")
			n, err := ingestConfiguredTestResults(t.Context(), storePath, "svc", root)
			if !errors.Is(err, tc.want) || n != 0 {
				t.Errorf("ingestConfiguredTestResults = %d, %v; want 0 and %v", n, err, tc.want)
			}
			if _, serr := os.Stat(storePath); serr == nil {
				t.Errorf("the store was opened for envelopes outside the repository")
			}
		})
	}
}
