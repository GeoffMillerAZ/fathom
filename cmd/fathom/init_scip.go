package main

// init_scip.go — SR-65 (surface reckoning F-74, owner decision): `fathom
// init` runs scip-typescript / scip-python when that toolchain is already on
// PATH and the repository is that language, then asks the daemon to import
// what each produced through the ingest tool's additive `indexers`
// parameter (internal/mcp/scip_import.go).
//
// The decision's three edges are the rules here:
//   - init never installs anything. A producer that is not on PATH is named
//     once, with the fact that init did not install it, and skipped.
//   - only the two producers, only for a repo that has that language's
//     marker file at its root: init does not run a TypeScript type-checker
//     over a Go repository because a binary happens to be installed.
//   - the daemon imports the index under its admission budget; init only
//     reports what came back, per indexer, and a producer or an import that
//     fails is a printed warning — the wiring around it always finishes.
//
// Each producer runs under a deadline through exec.CommandContext in the
// repo root and writes .fathom/scip/<name>.scip, a directory that ignores
// itself (a binary index is never committed).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/statedir"
)

// initSCIPTimeout bounds one producer: it type-checks the whole repository.
const initSCIPTimeout = 10 * time.Minute

// scipProducer is one SCIP indexer init knows how to run.
type scipProducer struct {
	name string
	// markers are root-level files any of which makes the repo this
	// producer's language.
	markers []string
	// args builds the producer's command line for a repo root, the index
	// file to write and the service id.
	args func(root, out, svc string) []string
}

// scipProducers is the closed set, in run order; it matches
// mcp.SCIPIndexers (TestInitSCIPProducersAreTheIngestToolsIndexers).
var scipProducers = []scipProducer{
	{
		name:    "scip-typescript",
		markers: []string{"tsconfig.json", "package.json"},
		args: func(root, out, _ string) []string {
			args := []string{"index"}
			if !fileExists(filepath.Join(root, "tsconfig.json")) {
				// A package.json-only (JavaScript) repo: scip-typescript
				// refuses to run without a tsconfig unless asked to infer one.
				args = append(args, "--infer-tsconfig")
			}
			return append(args, "--output", out)
		},
	},
	{
		name:    "scip-python",
		markers: []string{"pyproject.toml", "setup.py", "setup.cfg", "requirements.txt", "Pipfile"},
		args: func(_, out, svc string) []string {
			return []string{"index", ".", "--project-name", svc, "--output", out}
		},
	},
}

// scipInitDeps are initRunSCIP's seams: where a binary is found, how it
// runs, and how long it may.
type scipInitDeps struct {
	lookPath func(string) (string, error)
	run      func(ctx context.Context, dir, bin string, args []string) error
	timeout  time.Duration
}

func defaultSCIPInitDeps() scipInitDeps {
	return scipInitDeps{lookPath: exec.LookPath, run: runSCIPProducer, timeout: initSCIPTimeout}
}

// runSCIPProducer runs bin in dir under ctx, keeping the tail of its output
// for the error that names why it failed.
func runSCIPProducer(ctx context.Context, dir, bin string, args []string) error {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	var tail tailWriter
	tail.max = 2048
	cmd.Stdout, cmd.Stderr = &tail, &tail
	if err := cmd.Run(); err != nil {
		if t := strings.TrimSpace(tail.String()); t != "" {
			return fmt.Errorf("%w: %s", err, t)
		}
		return err
	}
	return nil
}

// tailWriter keeps the last max bytes written to it.
type tailWriter struct {
	max int
	buf []byte
}

func (w *tailWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	if len(w.buf) > w.max {
		w.buf = w.buf[len(w.buf)-w.max:]
	}
	return len(p), nil
}

func (w *tailWriter) String() string { return string(w.buf) }

func repoHasAny(root string, markers []string) bool {
	for _, m := range markers {
		if fileExists(filepath.Join(root, m)) {
			return true
		}
	}
	return false
}

// initRunSCIP runs each installed producer that applies to root and returns
// the names of those that left an index at .fathom/scip/<name>.scip, in run
// order. It prints one line per producer it acts on or wants installed and
// nothing for a language the repo is not.
//
// w is init's write door (init_dryrun.go): a dry run records the index and
// its .gitignore each producer would leave, and runs no producer.
func initRunSCIP(ctx context.Context, w *initWrites, root, svc string, stdout io.Writer, d scipInitDeps) []string {
	var produced []string
	outDir := statedir.LocalPath(root, projectdir.SCIP)
	prepared := false
	for _, p := range scipProducers {
		if !repoHasAny(root, p.markers) {
			continue
		}
		bin, err := d.lookPath(p.name)
		if err != nil {
			fmt.Fprintf(stdout, "note: %s is not on PATH — typed edges for this repo's %s need its index; not installed by init (install it, then re-run `fathom init`)\n", p.name, scipLanguage(p.name))
			continue
		}
		if !prepared {
			if err := w.mkdirAll(outDir); err != nil {
				fmt.Fprintf(stdout, "warning: %s not run: cannot create %s: %v\n", p.name, outDir, err)
				return produced
			}
			// A directory that ignores itself: a binary index is never committed.
			if gi := filepath.Join(outDir, ".gitignore"); !fileExists(gi) {
				_ = w.writeFile(gi, []byte("*\n"), 0o644)
			}
			prepared = true
		}
		out := filepath.Join(outDir, p.name+".scip")
		_ = w.remove(out) // a stale index is never imported in place of a failed run's
		if w.isDry() {
			w.record(out, writeAction(out))
			w.note(fmt.Sprintf("run %s once the service is indexed (it type-checks the whole repository)", p.name))
			produced = append(produced, p.name)
			continue
		}
		fmt.Fprintf(stdout, "running %s (it type-checks the whole repository and can take minutes)\n", p.name)
		runCtx, cancel := context.WithTimeout(ctx, d.timeout)
		runErr := d.run(runCtx, root, bin, p.args(root, out, svc))
		cancel()
		if runErr != nil {
			_ = os.Remove(out) // never leave a half-written index where the daemon would import it
			fmt.Fprintf(stdout, "warning: %s failed: %v — no typed edges from it (init continues)\n", p.name, runErr)
			continue
		}
		if fi, err := os.Stat(out); err != nil || fi.Size() == 0 {
			_ = os.Remove(out)
			fmt.Fprintf(stdout, "warning: %s exited cleanly but wrote no index at %s (init continues)\n", p.name, out)
			continue
		}
		fmt.Fprintf(stdout, "ok %s (index written to %s)\n", p.name, filepath.Join(outDir, p.name+".scip"))
		produced = append(produced, p.name)
	}
	return produced
}

func scipLanguage(name string) string {
	if name == "scip-python" {
		return "Python"
	}
	return "TypeScript/JavaScript"
}

// initSCIPStep is init's SCIP step: run what is installed, then ask the
// daemon to import it. indexed is whether the ingest just confirmed the
// service — an index is imported onto the syntax pass, and a producer's
// minutes are not spent for a daemon that has nothing to import onto.
func initSCIPStep(ctx context.Context, w *initWrites, baseURL, root, svc string, indexed bool, stdout io.Writer, d scipInitDeps) {
	if !indexed {
		for _, p := range scipProducers {
			if !repoHasAny(root, p.markers) {
				continue
			}
			if _, err := d.lookPath(p.name); err == nil {
				fmt.Fprintf(stdout, "note: %s not run: service %q is not indexed yet (see the MANUAL line above), and its index is imported onto the syntax pass — re-run `fathom init` once it is\n", p.name, svc)
			}
		}
		return
	}
	produced := initRunSCIP(ctx, w, root, svc, stdout, d)
	if len(produced) == 0 {
		return
	}
	if w.isDry() {
		w.note(fmt.Sprintf("import the %s index into the daemon (its store, not a file here)", strings.Join(produced, ", ")))
		return
	}
	initImportSCIP(baseURL, svc, root, produced, stdout)
}

// scipImportRow is one row of the ingest tool's `scip` block.
type scipImportRow struct {
	Indexer    string `json:"indexer"`
	Producer   string `json:"producer"`
	Edges      int    `json:"edges"`
	Direct     int    `json:"direct"`
	Unresolved int    `json:"unresolved"`
	Unmapped   int    `json:"unmapped"`
	Error      string `json:"error"`
}

// initImportSCIP asks the daemon to import the produced indexes: a second
// ingest call naming the indexers (the first already indexed the tree; this
// one is hash-gated, so it adds only the import) — sent after the syntax
// pass has been confirmed, so a slow first ingest never strands the import.
// It prints what came back, per indexer.
func initImportSCIP(baseURL, svc, root string, indexers []string, stdout io.Writer) {
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ingest", "arguments": map[string]any{"service": svc, "path": root, "indexers": indexers}},
	})
	if err != nil {
		return
	}
	req, err := originHeader(baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return
	}
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "warning: SCIP import not requested: %v — re-run `fathom init` to retry\n", err)
		return
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		fmt.Fprintf(stdout, "warning: SCIP import result unreadable: %v\n", err)
		return
	}
	inner, rpcErr := unwrapRPCResult(raw)
	if rpcErr != nil {
		fmt.Fprintf(stdout, "warning: SCIP import refused: %v\n", rpcErr)
		return
	}
	var res struct {
		Scip []scipImportRow `json:"scip"`
	}
	if err := json.Unmarshal(inner, &res); err != nil {
		fmt.Fprintf(stdout, "warning: SCIP import result unreadable: %v\n", err)
		return
	}
	if len(res.Scip) == 0 {
		fmt.Fprintf(stdout, "warning: the daemon reported no SCIP import for %s — it may predate the indexers parameter; restart it on this build\n", strings.Join(indexers, ", "))
		return
	}
	for _, r := range res.Scip {
		if r.Error != "" {
			fmt.Fprintf(stdout, "warning: %s index not imported: %s\n", r.Indexer, r.Error)
			continue
		}
		fmt.Fprintf(stdout, "ok %s: %d typed edge(s) (%d direct / %d unresolved) via %s\n", r.Indexer, r.Edges, r.Direct, r.Unresolved, r.Producer)
		if r.Unmapped > 0 {
			fmt.Fprintf(stdout, "warning: %s: %d definition(s) could not be mapped to a fathom symbol and contributed no edge (O115)\n", r.Indexer, r.Unmapped)
		}
	}
}
