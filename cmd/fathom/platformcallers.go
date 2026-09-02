package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/platform"
)

// cmdPlatformCallers implements the "platform-callers" subcommand: it prints
// every cross-repo caller of a named symbol across a fleet of services.
//
// Two data sources are supported:
//
//   - Store (default, language-agnostic): reads the symbols and call edges
//     already ingested into the SQLite store, so callers are found for EVERY
//     language whose extractor populated them (Python, TS, Java, Rust, …), not
//     just Go. Use --store [--services a,b,c]; with no --services, all services
//     in the store are considered.
//
//   - Repos (legacy, Go-only): walks Go source on disk. Use --repos id=path,…
//     This re-parses Go and ignores every other language; prefer the store path.
//
// Examples:
//
//	fathom platform-callers --symbol ProcessPayment
//	fathom platform-callers --store workspace/db/fleet.sqlite --symbol Greet
//	fathom platform-callers --repos svc-a=/src/a,svc-b=/src/b --symbol Greet
func cmdPlatformCallers(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("platform-callers", stderr)
	symbolFlag := fs.String("symbol", "", "bare symbol name to find cross-repo callers of (required)")
	storePath := fs.String("store", defaultStorePath(), "store path (.sqlite/.db; language-agnostic source)")
	servicesFlag := fs.String("services", "", "comma-separated service IDs to include (default: all services in the store)")
	reposFlag := fs.String("repos", "", "legacy Go-only mode: comma-separated id=path pairs; overrides --store when set")

	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *symbolFlag == "" {
		fmt.Fprintln(stderr, "error: --symbol is required")
		fs.Usage()
		return 1
	}

	var (
		g      *platform.Graph
		source string
	)

	if *reposFlag != "" {
		// Legacy Go-only filesystem path.
		repos, err := parseReposFlag(*reposFlag)
		if err != nil {
			fmt.Fprintf(stderr, "error: --repos: %v\n", err)
			return 1
		}
		if len(repos) == 0 {
			fmt.Fprintln(stderr, "error: --repos must contain at least one id=path pair")
			return 1
		}
		g, err = platform.Build(repos)
		if err != nil {
			fmt.Fprintf(stderr, "error: building platform graph from repos: %v\n", err)
			return 1
		}
		source = fmt.Sprintf("%d repo(s) on disk (Go-only)", len(repos))
	} else {
		// Default: language-agnostic store path.
		st, err := store.Open(*storePath)
		if err != nil {
			fmt.Fprintf(stderr, "error: opening store %q: %v\n", *storePath, err)
			return 1
		}
		defer st.Close()

		var services []string
		if *servicesFlag != "" {
			for _, s := range strings.Split(*servicesFlag, ",") {
				if s = strings.TrimSpace(s); s != "" {
					services = append(services, s)
				}
			}
		}

		g, err = platform.BuildFromStore(context.Background(), st, services)
		if err != nil {
			fmt.Fprintf(stderr, "error: building platform graph from store: %v\n", err)
			return 1
		}
		source = fmt.Sprintf("store %s (all languages)", *storePath)
	}

	callers := platform.PlatformCallers(g, *symbolFlag)
	if len(callers) == 0 {
		fmt.Fprintf(stdout, "no cross-repo callers of %q found (%s)\n", *symbolFlag, source)
		return 0
	}

	fmt.Fprintf(stdout, "cross-repo callers of %q (%s):\n\n", *symbolFlag, source)
	fmt.Fprintf(stdout, "%-24s %-40s %-24s %s\n", "FROM-SERVICE", "FROM-FILE", "TO-SERVICE", "TO-FILE")
	fmt.Fprintf(stdout, "%s\n", strings.Repeat("-", 110))
	for _, e := range callers {
		fmt.Fprintf(stdout, "%-24s %-40s %-24s %s\n",
			e.FromService, e.FromFile, e.ToService, e.ToFile)
	}
	fmt.Fprintf(stdout, "\ntotal: %d caller(s)\n", len(callers))
	return 0
}

// parseReposFlag parses a comma-separated list of "id=path" pairs into a map.
// It returns an error if any pair is malformed.
func parseReposFlag(s string) (map[string]string, error) {
	repos := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		idx := strings.IndexByte(pair, '=')
		if idx <= 0 {
			return nil, fmt.Errorf("malformed pair %q: expected id=path", pair)
		}
		id := strings.TrimSpace(pair[:idx])
		path := strings.TrimSpace(pair[idx+1:])
		if id == "" || path == "" {
			return nil, fmt.Errorf("malformed pair %q: id and path must be non-empty", pair)
		}
		repos[id] = path
	}
	return repos, nil
}
