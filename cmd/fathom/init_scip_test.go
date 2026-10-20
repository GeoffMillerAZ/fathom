package main

// init_scip_test.go — SR-65 (surface reckoning F-74, owner decision): `fathom
// init` runs scip-typescript / scip-python when that toolchain is already on
// PATH and the repo is that language, then names the producers whose index
// it left in the ingest tool's `indexers` parameter, so the daemon imports
// the typed edges. It never installs anything, never runs a producer that is
// not installed or not applicable, and a producer that fails costs the
// adopter a printed warning, never the wiring.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/geoffmilleraz/fathom/internal/statedir"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// scipRun is one recorded producer invocation.
type scipRun struct {
	dir, bin string
	args     []string
	deadline bool
}

// fakeSCIPDeps installs producers on a fake PATH and records every run. A
// run writes the file named by --output, as a real producer does, unless
// fail says the producer exits non-zero.
func fakeSCIPDeps(installed map[string]bool, fail map[string]bool) (scipInitDeps, *[]scipRun) {
	var runs []scipRun
	return scipInitDeps{
		lookPath: func(name string) (string, error) {
			if installed[name] {
				return "/fake/bin/" + name, nil
			}
			return "", errors.New("not found")
		},
		run: func(ctx context.Context, dir, bin string, args []string) error {
			_, hasDeadline := ctx.Deadline()
			runs = append(runs, scipRun{dir: dir, bin: filepath.Base(bin), args: args, deadline: hasDeadline})
			out := ""
			for i, a := range args {
				if a == "--output" && i+1 < len(args) {
					out = args[i+1]
				}
			}
			if fail[filepath.Base(bin)] {
				if out != "" {
					_ = os.WriteFile(out, []byte("partial"), 0o644) // a producer that died mid-write
				}
				return errors.New("exit status 1: tsc: cannot find module")
			}
			if out != "" {
				return os.WriteFile(out, []byte("SCIP"), 0o644)
			}
			return nil
		},
		timeout: time.Minute,
	}, &runs
}

func scipRepo(t *testing.T, files ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(root, f), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// TestInitRunsSCIPWhenInstalled is SR-65's anchor.
func TestInitRunsSCIPWhenInstalled(t *testing.T) {
	root := scipRepo(t, "tsconfig.json", "pyproject.toml")
	deps, runs := fakeSCIPDeps(map[string]bool{"scip-typescript": true, "scip-python": true}, nil)
	var out bytes.Buffer

	got := initRunSCIP(context.Background(), nil, root, "widgets", &out, deps)

	if strings.Join(got, ",") != "scip-typescript,scip-python" {
		t.Fatalf("produced = %v, want both producers, in order", got)
	}
	if len(*runs) != 2 {
		t.Fatalf("runs = %+v, want one per producer", *runs)
	}
	ts := (*runs)[0]
	wantTS := filepath.Join(statedir.Resolve(root), "scip", "scip-typescript.scip")
	if ts.bin != "scip-typescript" || ts.dir != root || strings.Join(ts.args, " ") != "index --output "+wantTS {
		t.Errorf("scip-typescript ran as %s %v in %s, want `index --output %s` in the repo root", ts.bin, ts.args, ts.dir, wantTS)
	}
	py := (*runs)[1]
	wantPy := filepath.Join(statedir.Resolve(root), "scip", "scip-python.scip")
	if py.bin != "scip-python" || strings.Join(py.args, " ") != "index . --project-name widgets --output "+wantPy {
		t.Errorf("scip-python ran as %s %v, want `index . --project-name widgets --output %s`", py.bin, py.args, wantPy)
	}
	for _, r := range *runs {
		if !r.deadline {
			t.Errorf("%s ran with no deadline; a producer type-checks the whole repo and must be bounded", r.bin)
		}
	}
	// The index directory ignores itself: a binary index is never committed.
	if b, err := os.ReadFile(filepath.Join(statedir.Resolve(root), "scip", ".gitignore")); err != nil || !strings.Contains(string(b), "*") {
		t.Errorf("scip/.gitignore = %q, %v; want it to ignore everything in the directory", b, err)
	}
	if !strings.Contains(out.String(), "ok scip-typescript") || !strings.Contains(out.String(), "ok scip-python") {
		t.Errorf("init output does not say what ran:\n%s", out.String())
	}
}

func TestInitSCIPNeverRunsWhatIsNotInstalled(t *testing.T) {
	root := scipRepo(t, "tsconfig.json", "requirements.txt")
	deps, runs := fakeSCIPDeps(map[string]bool{"scip-python": true}, nil)
	var out bytes.Buffer

	got := initRunSCIP(context.Background(), nil, root, "widgets", &out, deps)

	if strings.Join(got, ",") != "scip-python" {
		t.Fatalf("produced = %v, want only the installed producer", got)
	}
	for _, r := range *runs {
		if r.bin == "scip-typescript" {
			t.Fatal("ran scip-typescript, which is not on PATH")
		}
	}
	// The TypeScript repo is told what would make its index, never made to
	// install it: init prints the name and moves on.
	if !strings.Contains(out.String(), "scip-typescript is not on PATH") || !strings.Contains(out.String(), "not installed by init") {
		t.Errorf("a TypeScript repo without the producer must be told, once, that init does not install it:\n%s", out.String())
	}
}

func TestInitSCIPSkipsLanguagesTheRepoIsNot(t *testing.T) {
	root := scipRepo(t, "go.mod") // a Go repo: no SCIP producer applies
	deps, runs := fakeSCIPDeps(map[string]bool{"scip-typescript": true, "scip-python": true}, nil)
	var out bytes.Buffer

	got := initRunSCIP(context.Background(), nil, root, "widgets", &out, deps)
	if len(got) != 0 || len(*runs) != 0 {
		t.Errorf("produced %v runs %+v in a repo with no TypeScript or Python markers", got, *runs)
	}
	if strings.Contains(out.String(), "scip") {
		t.Errorf("a repo those producers do not apply to hears nothing about them:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".fathom")); err == nil {
		t.Error("init created .fathom/ for producers that never ran")
	}
}

func TestInitSCIPInfersATsconfigForAPackageJSONRepo(t *testing.T) {
	root := scipRepo(t, "package.json")
	deps, runs := fakeSCIPDeps(map[string]bool{"scip-typescript": true}, nil)
	initRunSCIP(context.Background(), nil, root, "widgets", io.Discard, deps)
	if len(*runs) != 1 || !strings.Contains(strings.Join((*runs)[0].args, " "), "--infer-tsconfig") {
		t.Errorf("runs = %+v, want scip-typescript with --infer-tsconfig for a package.json repo with no tsconfig.json", *runs)
	}
}

func TestInitSCIPFailureIsAWarningNotAnAbortAndLeavesNoPartialIndex(t *testing.T) {
	root := scipRepo(t, "tsconfig.json", "pyproject.toml")
	deps, _ := fakeSCIPDeps(map[string]bool{"scip-typescript": true, "scip-python": true}, map[string]bool{"scip-typescript": true})
	var out bytes.Buffer

	got := initRunSCIP(context.Background(), nil, root, "widgets", &out, deps)

	if strings.Join(got, ",") != "scip-python" {
		t.Fatalf("produced = %v, want the producer that succeeded and not the one that failed", got)
	}
	if !strings.Contains(out.String(), "warning: scip-typescript failed") || !strings.Contains(out.String(), "cannot find module") {
		t.Errorf("the failure is not named:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(statedir.Resolve(root), "scip", "scip-typescript.scip")); err == nil {
		t.Error("a failed producer's half-written index was left where the daemon would import it")
	}
}

func TestInitSCIPCancelledIsSilentWork(t *testing.T) {
	root := scipRepo(t, "tsconfig.json")
	deps, _ := fakeSCIPDeps(map[string]bool{"scip-typescript": true}, map[string]bool{"scip-typescript": true})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// A cancelled context stops the run (the fake reports it as a failure
	// like exec.CommandContext would); initRunSCIP must return, not hang.
	if got := initRunSCIP(ctx, nil, root, "widgets", io.Discard, deps); len(got) != 0 {
		t.Errorf("produced %v under a cancelled context", got)
	}
}

// scipStubDaemon answers init's ingest calls, recording each one's
// arguments, with the given scip block in the result.
func scipStubDaemon(t *testing.T, scipBlock string) (*httptest.Server, *[]map[string]any) {
	t.Helper()
	var calls []map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Params.Name != "ingest" {
			fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{}"}]}}`)
			return
		}
		calls = append(calls, req.Params.Arguments)
		text, _ := json.Marshal(`{"service":"svc","ok":true,"scip":` + scipBlock + `}`)
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":%s}]}}`, text)
	}))
	t.Cleanup(ts.Close)
	return ts, &calls
}

// TestInitSCIPStepImportsTheProducedIndexes: init names the producers whose
// index it left in the ingest tool's `indexers` parameter and reports what
// the daemon imported, per indexer — edges, the unmapped definitions (O115)
// and any index it declined.
func TestInitSCIPStepImportsTheProducedIndexes(t *testing.T) {
	ts, calls := scipStubDaemon(t, `[{"indexer":"scip-typescript","path":".fathom/scip/scip-typescript.scip","producer":"scip-typescript 0.3.16","edges":12,"direct":9,"unresolved":3,"unmapped":2},{"indexer":"scip-python","path":".fathom/scip/scip-python.scip","error":"the index is 90 MiB, over the 64 MiB import budget — not imported"}]`)
	root := scipRepo(t, "tsconfig.json", "pyproject.toml")
	deps, _ := fakeSCIPDeps(map[string]bool{"scip-typescript": true, "scip-python": true}, nil)
	var out bytes.Buffer

	initSCIPStep(context.Background(), nil, ts.URL, root, "svc", true, &out, deps)

	if len(*calls) != 1 {
		t.Fatalf("ingest calls = %v, want exactly one, carrying the indexers", *calls)
	}
	call := (*calls)[0]
	names, _ := call["indexers"].([]any)
	if call["service"] != "svc" || len(names) != 2 || names[0] != "scip-typescript" || names[1] != "scip-python" {
		t.Errorf("ingest arguments = %v, want service svc and indexers [scip-typescript scip-python]", call)
	}
	for _, want := range []string{
		"ok scip-typescript: 12 typed edge(s) (9 direct / 3 unresolved) via scip-typescript 0.3.16",
		"2 definition(s) could not be mapped",
		"warning: scip-python index not imported: the index is 90 MiB",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("init output missing %q:\n%s", want, out.String())
		}
	}
}

func TestInitSCIPStepCallsNothingWhenNothingWasProduced(t *testing.T) {
	ts, calls := scipStubDaemon(t, `[]`)
	root := scipRepo(t, "tsconfig.json")
	deps, runs := fakeSCIPDeps(nil, nil) // nothing installed
	initSCIPStep(context.Background(), nil, ts.URL, root, "svc", true, io.Discard, deps)
	if len(*calls) != 0 || len(*runs) != 0 {
		t.Errorf("calls %v runs %v with no producer installed; init must not ask the daemon to import nothing", *calls, *runs)
	}
}

// TestInitSCIPStepWaitsForAnIndexedService: a producer type-checks the whole
// repository and its index is useless without a daemon that has the syntax
// pass, so init runs nothing until the ingest confirmed, and says why.
func TestInitSCIPStepWaitsForAnIndexedService(t *testing.T) {
	ts, calls := scipStubDaemon(t, `[]`)
	root := scipRepo(t, "tsconfig.json")
	deps, runs := fakeSCIPDeps(map[string]bool{"scip-typescript": true}, nil)
	var out bytes.Buffer
	initSCIPStep(context.Background(), nil, ts.URL, root, "svc", false, &out, deps)
	if len(*runs) != 0 || len(*calls) != 0 {
		t.Errorf("ran producers (%v) or called the daemon (%v) for a service that never read as indexed", *runs, *calls)
	}
	if !strings.Contains(out.String(), "not indexed") {
		t.Errorf("init must say why it did not run the producers:\n%s", out.String())
	}
}

// TestInitSCIPProducersAreTheIngestToolsIndexers keeps init's producer table
// and the ingest tool's closed `indexers` set one list: a producer init runs
// that the daemon would refuse by name is an index nobody imports.
func TestInitSCIPProducersAreTheIngestToolsIndexers(t *testing.T) {
	var ours []string
	for _, p := range scipProducers {
		ours = append(ours, p.name)
	}
	if got, want := strings.Join(ours, ","), strings.Join(mcp.SCIPIndexers(), ","); got != want {
		t.Errorf("init runs %s, the ingest tool accepts %s", got, want)
	}
}

// writeFakeProducer writes an executable shell script standing in for a
// producer binary: it runs as a real child of runSCIPProducer.
func writeFakeProducer(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fake-producer")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestRunSCIPProducerRunsInTheRepoAndNamesAFailure runs a real child: it
// starts in the repo root, and a non-zero exit carries the tail of what the
// producer said, not a bare "exit status 3".
func TestRunSCIPProducerRunsInTheRepoAndNamesAFailure(t *testing.T) {
	dir := t.TempDir()
	ok := writeFakeProducer(t, `pwd > "$1"`)
	target := filepath.Join(t.TempDir(), "cwd.txt")
	if err := runSCIPProducer(context.Background(), dir, ok, []string{target}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(target)
	if want, _ := filepath.EvalSymlinks(dir); strings.TrimSpace(string(got)) != want && strings.TrimSpace(string(got)) != dir {
		t.Errorf("producer ran in %q, want the repo root %q", strings.TrimSpace(string(got)), dir)
	}
	bad := writeFakeProducer(t, `echo "error TS2307: Cannot find module './x'" >&2; exit 3`)
	err := runSCIPProducer(context.Background(), dir, bad, nil)
	if err == nil || !strings.Contains(err.Error(), "TS2307") || !strings.Contains(err.Error(), "exit status 3") {
		t.Errorf("err = %v, want the exit status and the producer's own words", err)
	}
}

// TestRunSCIPProducerStopsAtItsDeadline: a producer that never ends is
// killed when its context does, and init moves on.
func TestRunSCIPProducerStopsAtItsDeadline(t *testing.T) {
	hang := writeFakeProducer(t, `exec sleep 30`)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := runSCIPProducer(ctx, t.TempDir(), hang, nil)
	if err == nil {
		t.Fatal("a producer past its deadline reported success")
	}
	if took := time.Since(start); took > 10*time.Second {
		t.Errorf("the producer outlived its 200ms deadline by %s", took)
	}
}
