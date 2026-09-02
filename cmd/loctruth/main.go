// Command loctruth is the committed go/types tool that produces the
// find_callers ground truth for fathom's public localization corpus
// (docs/plans/bench-public.md PB-16, ADR-031 §2.3). It is not fathom: it
// never opens a fathom store and never asks a daemon. A stranger with the
// Go toolchain and a checkout at base_commit gets the same JSON.
//
// Usage:
//
//	loctruth -root <module checkout at base_commit> [-out callers.json] [-func file.go:Name,...]
//
// Output is a publicloc.CallersFile: every direct static call in the
// module's non-test packages whose callee is declared in the module,
// sorted. -func restricts the edges to the named callees
// (repo-relative declaring file, then Name or Type.Method) — the same key
// the corpus's derivation records.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	os.Exit(runLocTruth(os.Args[1:], os.Stdout, os.Stderr))
}

func runLocTruth(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("loctruth", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", "", "module checkout at the pinned base_commit (required)")
	out := fs.String("out", "", "write the CallersFile JSON here (default: stdout)")
	only := fs.String("func", "", "comma-separated file.go:Name filters; only their callers are emitted")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *root == "" {
		fmt.Fprintln(stderr, "loctruth: -root is required")
		return 2
	}

	truth, err := CallersTruth(*root)
	if err != nil {
		fmt.Fprintf(stderr, "loctruth: %v\n", err)
		return 1
	}
	if *only != "" {
		want := map[string]bool{}
		for _, f := range strings.Split(*only, ",") {
			want[strings.TrimSpace(f)] = true
		}
		kept := truth.Edges[:0]
		for _, e := range truth.Edges {
			if want[e.CalleeFile+":"+e.Callee] {
				kept = append(kept, e)
			}
		}
		truth.Edges = kept
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(truth); err != nil {
		fmt.Fprintf(stderr, "loctruth: %v\n", err)
		return 1
	}
	if *out == "" {
		_, err = stdout.Write(buf.Bytes())
	} else {
		err = os.WriteFile(*out, buf.Bytes(), 0o644)
	}
	if err != nil {
		fmt.Fprintf(stderr, "loctruth: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "loctruth: %s — %d packages (%d with type errors), %d edges\n",
		truth.Module, truth.Packages, truth.PackagesWithErrors, len(truth.Edges))
	return 0
}
