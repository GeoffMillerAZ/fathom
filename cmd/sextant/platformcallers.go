package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/geoffmilleraz/sextant/pkg/platform"
)

// cmdPlatformCallers implements the "platform-callers" subcommand.
//
// It accepts --repos (comma-separated id=path pairs) and --symbol, builds the
// platform graph, then prints every cross-repo caller of the named symbol to
// stdout in a human-readable table.
//
// Example:
//
//	sextant platform-callers \
//	    --repos payment-service=/src/payment,order-service=/src/order \
//	    --symbol ProcessPayment
func cmdPlatformCallers(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("platform-callers", stderr)
	reposFlag := fs.String("repos", "", `comma-separated id=path pairs, e.g. svc-a=/src/a,svc-b=/src/b (required)`)
	symbolFlag := fs.String("symbol", "", "bare exported symbol name to look up (required)")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *reposFlag == "" {
		fmt.Fprintln(stderr, "error: --repos is required")
		fs.Usage()
		return 1
	}
	if *symbolFlag == "" {
		fmt.Fprintln(stderr, "error: --symbol is required")
		fs.Usage()
		return 1
	}

	repos, err := parseReposFlag(*reposFlag)
	if err != nil {
		fmt.Fprintf(stderr, "error: --repos: %v\n", err)
		return 1
	}
	if len(repos) == 0 {
		fmt.Fprintln(stderr, "error: --repos must contain at least one id=path pair")
		return 1
	}

	g, buildErr := platform.Build(repos)
	if buildErr != nil {
		fmt.Fprintf(stderr, "error: building platform graph: %v\n", buildErr)
		return 1
	}

	callers := platform.PlatformCallers(g, *symbolFlag)
	if len(callers) == 0 {
		fmt.Fprintf(stdout, "no cross-repo callers of %q found\n", *symbolFlag)
		return 0
	}

	fmt.Fprintf(stdout, "cross-repo callers of %q:\n\n", *symbolFlag)
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
