package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
)

// `fathom init --demo` (RH-12, docs/design/reach.md §5.1): a newcomer sees
// fathom do provable work before pointing it at a repository of their own.
// The demo copies the fauxtown fixture fleet (compiled into the binary, no
// download) into a throwaway directory, runs this binary's own init, ingest
// and verify in it under a temporary HOME, and prints the time to that
// first proven answer — the stranger check's RH-9 step
// (scripts/stranger-check.sh), from a prebuilt binary.
//
// Every step is a child process whose environment is rebuilt, never
// inherited for the paths that matter: HOME, TMPDIR (a child's os.TempDir
// caches), the XDG directories and every FATHOM_ variable point inside the
// throwaway directory, the daemon URL
// points at a port nothing listens on, and model API keys are dropped. So
// the real HOME, ~/.fathom and a running daemon are out of the children's
// reach by construction, not by care.

// demoService is the service id the demo ingests and verifies.
const demoService = "fauxtown-checkout"

// demoNoDaemon is where the demo's children look for a daemon: a port
// nothing listens on, so no step can reach the operator's.
const demoNoDaemon = "http://127.0.0.1:1"

// demoPlan is the two-row fixture plan the first proven answer verifies:
// one claimed row the fixture ingest proves, one left open on purpose (an
// open row never gates). The stranger check's plan, against the demo copy.
const demoPlan = `# fathom init --demo: a two-row fixture plan, one claimed, one open.
- [x] FetchItem is defined in the freshly ingested fixture {id: DEMO-1, oracle: lexical, path: checkout-service/checkout.go, pattern: "^func FetchItem"}
- [ ] HoldItem, left open on purpose {id: DEMO-2, symbol: HoldItem}
`

// initDemoDeps is what the demo reads from its process: the binary its
// steps run, the fleet it copies, where its throwaway directory goes, and
// the clock its one timing line reads.
type initDemoDeps struct {
	exe     string   // the fathom binary each step runs
	extra   []string // environment added to every step after the rebuild
	tmpRoot string   // parent of the throwaway directory ("" = the system temp dir)
	fleet   func() (fs.FS, error)
	now     func() time.Time
}

func defaultInitDemoDeps() initDemoDeps {
	exe, err := os.Executable()
	if err == nil {
		if resolved, rerr := filepath.EvalSymlinks(exe); rerr == nil {
			exe = resolved
		}
	}
	return initDemoDeps{exe: exe, fleet: connector.Fauxtown, now: time.Now}
}

// initDemo runs the demo and returns init's exit code: 0 on a first proven
// answer, 1 when a step fails (its output is printed).
func initDemo(ctx context.Context, d initDemoDeps, stdout, stderr io.Writer) int {
	start := d.now()
	if d.exe == "" {
		fmt.Fprintln(stderr, "error: --demo: cannot resolve this fathom binary to run its steps")
		return 1
	}
	dir, err := os.MkdirTemp(d.tmpRoot, "fathom-demo-")
	if err != nil {
		fmt.Fprintf(stderr, "error: --demo: making the throwaway directory: %v\n", err)
		return 1
	}
	defer os.RemoveAll(dir)
	home := filepath.Join(dir, "home")
	fleetDir := filepath.Join(dir, "fauxtown")
	for _, d := range []string{home, filepath.Join(dir, "tmp")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			fmt.Fprintf(stderr, "error: --demo: %v\n", err)
			return 1
		}
	}
	fleet, err := d.fleet()
	if err != nil {
		fmt.Fprintf(stderr, "error: --demo: the bundled fixture fleet: %v\n", err)
		return 1
	}
	services, err := copyDemoFleet(fleet, fleetDir)
	if err != nil {
		fmt.Fprintf(stderr, "error: --demo: copying the fixture fleet: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "fathom init --demo — a throwaway copy of the fauxtown fixture fleet (%d services) at %s, under a temporary HOME; your HOME, ~/.fathom and any running daemon are not touched\n", services, fleetDir)

	env := demoEnv(os.Environ(), dir, home, d.extra)
	step := func(name string, args ...string) (string, int) {
		cmd := exec.CommandContext(ctx, d.exe, args...)
		cmd.Dir = fleetDir
		cmd.Env = env
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		err := cmd.Run()
		var exitErr *exec.ExitError
		switch {
		case err == nil:
			return out.String(), 0
		case errors.As(err, &exitErr):
			return out.String(), exitErr.ExitCode()
		default:
			return fmt.Sprintf("%s: %v\n%s", name, err, out.String()), -1
		}
	}
	fail := func(what, out string) int {
		fmt.Fprintf(stderr, "error: --demo: %s failed:\n%s", what, out)
		return 1
	}

	// 1. Wire the copy with init itself. The demo runs no daemon, so init's
	// own exit 2 (wired, index unconfirmed) is the expected outcome here.
	out, code := step("init", "init", "--path", fleetDir, "--url", demoNoDaemon, "--no-scip")
	if code != 0 && code != 2 {
		return fail("fathom init on the copy", out)
	}
	fmt.Fprintf(stdout, "ok wired the copy with fathom init: %d files written (hooks, the AGENTS.md stanza, the .fathom scaffolds)\n", countCreated(out))

	// 2. Ingest one service into a scratch store.
	store := filepath.Join(dir, "demo.sqlite")
	if out, code := step("ingest", "ingest", "-service", demoService, "-path", "checkout-service", "-store", store); code != 0 {
		return fail("fathom ingest of checkout-service", out)
	}
	fmt.Fprintln(stdout, "ok ingested checkout-service into a scratch store")

	// 3. Verify the fixture plan against what was just ingested.
	plan := filepath.Join(fleetDir, "demo-plan.md")
	if err := os.WriteFile(plan, []byte(demoPlan), 0o644); err != nil {
		fmt.Fprintf(stderr, "error: --demo: writing the fixture plan: %v\n", err)
		return 1
	}
	if out, code := step("verify", "verify", "-service", demoService, "-plan", plan, "-store", store, "-ledger", filepath.Join(dir, "verify-ledger.jsonl")); code != 0 {
		return fail("fathom verify of the fixture plan", out)
	}
	fmt.Fprintln(stdout, "ok verified a two-row fixture plan: the claimed row is proven by the ingest, the open row never gates")
	fmt.Fprintf(stdout, "first proven answer in %s (copy, wire, ingest, verify); the throwaway copy is removed — run fathom init --dry-run in a repository of yours to see what init would write there\n", d.now().Sub(start).Round(10*time.Millisecond))
	return 0
}

// copyDemoFleet copies fleet into dst and returns how many services (top
// directories) it holds.
func copyDemoFleet(fleet fs.FS, dst string) (int, error) {
	services := 0
	err := fs.WalkDir(fleet, ".", func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dst, filepath.FromSlash(p))
		if e.IsDir() {
			if p != "." && !strings.Contains(p, "/") {
				services++
			}
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(fleet, p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	return services, err
}

// demoEnv rebuilds the environment for the demo's children: HOME, the temp
// directory variables, every XDG_ and FATHOM_ variable, the Claude/Codex
// config homes and model API keys are dropped from base, then HOME, TMPDIR,
// the XDG directories, fathom's home and state directories point inside
// dir and the daemon URL at
// demoNoDaemon; extra goes last.
func demoEnv(base []string, dir, home string, extra []string) []string {
	env := make([]string, 0, len(base)+len(extra)+10)
	for _, kv := range base {
		name, _, _ := strings.Cut(kv, "=")
		switch {
		case name == "HOME", name == "CLAUDE_CONFIG_DIR", name == "CODEX_HOME",
			name == "TMPDIR", name == "TMP", name == "TEMP",
			strings.HasPrefix(name, "XDG_"), strings.HasPrefix(name, "FATHOM_"),
			strings.HasSuffix(name, "_API_KEY"):
			continue
		}
		env = append(env, kv)
	}
	env = append(env,
		"HOME="+home,
		"TMPDIR="+filepath.Join(dir, "tmp"),
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
		"XDG_CACHE_HOME="+filepath.Join(home, ".cache"),
		"XDG_DATA_HOME="+filepath.Join(home, ".local", "share"),
		"XDG_STATE_HOME="+filepath.Join(home, ".local", "state"),
		"FATHOM_HOME="+filepath.Join(home, ".fathom"),
		"FATHOM_STATE_DIR="+filepath.Join(dir, "state"),
		"FATHOM_HTTP="+demoNoDaemon,
	)
	return append(env, extra...)
}

// countCreated counts init's "created X" narration lines.
func countCreated(out string) int {
	n := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "created ") {
			n++
		}
	}
	return n
}
