package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// cmdServeMCP is the entry point for the "serve-mcp" subcommand.
// It reads from os.Stdin and writes to stdout.
func cmdServeMCP(args []string, stdout, stderr io.Writer) int {
	return runServeMCP(args, os.Stdin, stdout, stderr)
}

// runServeMCP is the testable core of "serve-mcp". stdin is injected so
// tests can feed finite byte streams without blocking.
func runServeMCP(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := newFlagSet("serve-mcp", stderr)
	storePath := fs.String("store", "", "store path, .sqlite/.db (required)")
	mode := fs.String("mode", "local", "connector mode: local or faux")
	autoRefresh := fs.Bool("auto-refresh", true, "re-ingest a service when its git HEAD drifts from the indexed commit, before answering queries")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *storePath == "" {
		fmt.Fprintln(stderr, "error: --store is required")
		fs.Usage()
		return 1
	}

	m, ok := parseModeFlag(*mode)
	if !ok {
		fmt.Fprintf(stderr, "error: unknown --mode %q (want local or faux)\n", *mode)
		return 1
	}

	c, err := client.NewConstantClient(m, *storePath)
	if err != nil {
		if isStoreLockConflict(err) {
			fmt.Fprintf(stderr, "error: another fathom process already holds the store %q.\n", *storePath)
			fmt.Fprintf(stderr, "  %v\n", err)
			fmt.Fprintln(stderr, "  SQLite serialises writers: a second serve-mcp waited past busy_timeout for the")
			fmt.Fprintln(stderr, "  writer to release the store. Reap the other process, or point --store at a different file.")
			return 1
		}
		fmt.Fprintf(stderr, "error: opening client: %v\n", err)
		return 1
	}
	defer c.Close()

	// ADR-042/DP-9: the decision port — obligations' within-tier re-rank
	// reads it through the Client exactly like serve-http (compile_layer's
	// own prefilter benefits too, on the rare stdio caller that also wires
	// an LLM via WithLLM elsewhere). Fail-open (openDecisionPorts never
	// errors outward): a misconfigured or unreachable [decide] provider
	// warns to stderr and falls back to decision.None rather than blocking
	// stdio startup (I14).
	loaded, cfgErr := config.Load(config.DefaultUserPath(), config.DefaultProjectPath())
	if cfgErr != nil {
		fmt.Fprintf(stderr, "warn: decision port disabled: loading config: %v\n", cfgErr)
	} else {
		daemonProfileName := loaded.Config.Daemon.DefaultProfile
		if daemonProfileName == "" {
			daemonProfileName = loaded.Config.DefaultProfile
		}
		ports := openDecisionPorts(loaded, daemonProfileName, resolveCallLogPath(), "serve-mcp", stderr)
		c.WithDecision(ports.Decider, ports.Cfg, ports.Recorder)
	}

	// I51: the MCP tools here share the same memo registry as serve-http's,
	// so it takes the same byte budget — the cache fraction of an
	// unmanaged daemon's tenancy budget ([tenancy.unmanaged], or
	// FATHOM_TENANCY_BUDGET). Without one the registry held every service
	// a session touched until its watermark moved.
	boundServeMCPMemo(loaded, cfgErr, stderr)

	// Announce build identity at startup. A long-running serve-mcp process can
	// outlive several commits; logging the embedded VCS revision makes a stale
	// server self-evident (compare against `git rev-parse HEAD`). See
	// docs/dogfood-observations.md (staleness finding).
	fmt.Fprintf(stderr, "fathom serve-mcp starting: %s\n", BuildRevision())

	flog := dlog.New(stderr, dlog.LevelFromEnv(os.Getenv), time.Now)
	c.WithLogger(flog) // DG-4: the typed-edge pass's own diagnostics gain a timestamp/level too.

	// Default: keep the index fresh transparently. A query for a service whose
	// git HEAD has drifted from the indexed commit triggers a hash-gated
	// re-ingest before the answer — removing the manual staleness→ingest loop.
	var backend mcp.QueryInterface = c
	if *autoRefresh {
		backend = mcp.NewAutoRefresh(c, flog)
		// RT-109: an in_progress row is an interrupted ingest only once
		// its owner is gone (a live CLI ingest is never duplicated).
		mcp.SetIngestOwnerProbe(ingestOwnerProbe(realDaemonOps{}))
	}

	srv := mcp.NewServerWithFleet(backend, newFleetAdapter(c, flog))

	// RT-86: a stdio server runs in its agent's working directory, so its
	// calls carry that as the caller's cwd — references then defaults to
	// the agent's own service instead of refusing.
	serveCtx := context.Background()
	if wd, err := os.Getwd(); err == nil {
		serveCtx = mcp.WithCaller(serveCtx, mcp.CallerInfo{PID: os.Getpid(), PPID: os.Getppid(), Cwd: wd})
	}
	if err := srv.Serve(serveCtx, stdin, stdout); err != nil {
		// EOF / context cancel are expected end-of-stream signals; only
		// surface genuine errors.
		fmt.Fprintf(stderr, "error: mcp serve: %v\n", err)
		return 1
	}
	return 0
}

// isStoreLockConflict reports whether err is SQLite's busy/locked conflict —
// another process held the write lock past busy_timeout (ADR-020; modernc.org/
// sqlite reports this as a SQLITE_BUSY or SQLITE_LOCKED result code, rendered
// into the error text as one of these substrings).
func isStoreLockConflict(err error) bool {
	s := err.Error()
	return strings.Contains(s, "SQLITE_BUSY") || strings.Contains(s, "SQLITE_LOCKED") || strings.Contains(s, "database is locked")
}

// boundServeMCPMemo sets the memo registry's byte budget for a stdio
// server: serve-mcp is never supervised, so it resolves the unmanaged
// budget. An unreadable config still bounds the registry (the unmanaged
// default).
func boundServeMCPMemo(loaded config.Loaded, cfgErr error, stderr io.Writer) {
	if cfgErr != nil {
		loaded = config.Loaded{}
	}
	tcfg, _, err := config.ResolveTenancyConfig(loaded, os.Getenv)
	if err != nil {
		fmt.Fprintf(stderr, "warn: tenancy config: %v — the memo registry takes the unmanaged default budget\n", err)
		tcfg, _, _ = config.ResolveTenancyConfig(config.Loaded{}, os.Getenv)
	}
	b, _ := tenancy.ResolveBudget(tcfg, tenancy.HostFacts{RAM: hostRAM(), NumCPU: runtime.NumCPU()})
	memo.Default.SetBudget(b.Caches, b.CacheShedTrim)
}
