package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/geoffmilleraz/fathom/pkg/client"
)

// mtimeState maps absolute file path → last observed modification time.
// The zero value (nil map) is valid and treated as "all files are new".
type mtimeState map[string]time.Time

// watchConfig holds the parameters for a watch session.
type watchConfig struct {
	service   string
	path      string
	storePath string
	mode      string
	interval  time.Duration
}

// scanMtimes walks dir and returns an mtimeState with the modification time of
// every regular file.
func scanMtimes(dir string) (mtimeState, error) {
	state := make(mtimeState)
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		state[p] = info.ModTime()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return state, nil
}

// hasChanged compares a freshly scanned mtime map against the previous state.
// Returns true if any file is new or has a later mtime than recorded.
func hasChanged(prev, curr mtimeState) bool {
	for path, modTime := range curr {
		if prev == nil {
			return true
		}
		if prevTime, ok := prev[path]; !ok || modTime.After(prevTime) {
			return true
		}
	}
	return false
}

// watchOnce performs a single poll cycle: scan mtimes, compare to prev, and if
// anything changed run an ingest.  It returns (true, nil) when an ingest was
// triggered, (false, nil) when nothing changed, or (false, err) on error.
// It is exported (lower-case in same package) so tests can call it directly.
func watchOnce(cfg watchConfig, prev mtimeState, stdout, stderr io.Writer) (bool, error) {
	curr, err := scanMtimes(cfg.path)
	if err != nil {
		return false, fmt.Errorf("scanMtimes %q: %w", cfg.path, err)
	}

	if !hasChanged(prev, curr) {
		return false, nil
	}

	m, ok := parseModeFlag(cfg.mode)
	if !ok {
		return false, fmt.Errorf("unknown mode %q", cfg.mode)
	}

	c, err := client.NewConstantClient(m, cfg.storePath)
	if err != nil {
		return false, fmt.Errorf("opening client: %w", err)
	}
	defer c.Close()

	svcPath := cfg.path
	if m == client.ModeFaux {
		svcPath = cfg.service
	}

	report, err := c.Ingest(context.Background(), client.ServiceRef{
		ID:   cfg.service,
		Path: svcPath,
	})
	if err != nil {
		return false, fmt.Errorf("ingest: %w", err)
	}

	fmt.Fprintf(stdout, "[watch] %s: re-ingested (discovered=%d extracted=%d symbols=%d)\n",
		cfg.service, report.Discovered, report.Extracted, report.Symbols)
	return true, nil
}

// cmdWatch is the dispatch entry point for the "watch" subcommand.
func cmdWatch(args []string, stdout, stderr io.Writer) int {
	fs2 := newFlagSet("watch", stderr)
	service := fs2.String("service", "", "logical service ID (required)")
	path := fs2.String("path", "", "path to service source root (required)")
	storePath := fs2.String("store", "workspace/db/fathom.sqlite", "store path (.duckdb or .sqlite/.db)")
	interval := fs2.Duration("interval", 5*time.Second, "polling interval (e.g. 5s, 1m)")
	mode := fs2.String("mode", "local", "connector mode: local or faux")

	if err := fs2.Parse(args); err != nil {
		return 1
	}

	if *service == "" {
		fmt.Fprintln(stderr, "error: --service is required")
		fs2.Usage()
		return 1
	}
	if *path == "" {
		fmt.Fprintln(stderr, "error: --path is required")
		fs2.Usage()
		return 1
	}

	cfg := watchConfig{
		service:   *service,
		path:      *path,
		storePath: *storePath,
		mode:      *mode,
		interval:  *interval,
	}

	// Bootstrap the initial mtime state before the first poll.
	state, err := scanMtimes(*path)
	if err != nil {
		fmt.Fprintf(stderr, "error: initial scan of %q: %v\n", *path, err)
		return 1
	}

	fmt.Fprintf(stdout, "[watch] watching %s at %s (interval=%s)\n",
		*service, *path, *interval)

	for {
		triggered, err := watchOnce(cfg, state, stdout, stderr)
		if err != nil {
			fmt.Fprintf(stderr, "error: watch cycle: %v\n", err)
		}

		// After a successful poll (change or not) refresh the state.
		if triggered || err == nil {
			newState, scanErr := scanMtimes(*path)
			if scanErr == nil {
				state = newState
			}
		}

		// Use os.Stderr for the sleep so stdout stays clean for machine parsing.
		_ = os.Stderr
		time.Sleep(cfg.interval)
	}
}
