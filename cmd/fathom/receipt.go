// receipt.go — the `fathom receipt` verb family (VB-20, docs/vision/
// 29-verbs-not-lookups.md §3 wave E; ADR-035 decision 4): today just
// `verify`, which re-derives a saved agent-surface answer's claim from a
// bare git checkout with go/types and git alone — no fathom daemon, no
// store, so a stranger reproduces (or refutes) it independently.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/receiptverify"
)

// cmdReceipt dispatches to the receipt verb named by args[0].
func cmdReceipt(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: receipt requires a verb: verify")
		return 1
	}
	switch args[0] {
	case "verify":
		return cmdReceiptVerify(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown receipt verb %q (want verify)\n", args[0])
		return 1
	}
}

// cmdReceiptVerify implements `fathom receipt verify [--repo <path>]
// <receipt.json>`. Exit 0 means the receipt's claim reproduced against a
// bare checkout of its own index_commit; exit 1 means it did not
// (stdout names the difference); exit 2 means the check could not run at
// all (no git repository, missing commit, no go.mod, unreadable/invalid
// receipt) — an operational refusal, never a false pass or fail.
func cmdReceiptVerify(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("receipt verify", stderr)
	repo := fs.String("repo", ".", "path to a git checkout containing the receipt's index_commit (default: current directory)")
	runTests := fs.Bool("run-tests", false, "certification kind only: actually run the named test (go test -run '^<Test>$' <pkg>) instead of reporting an unverified run")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(stderr, "usage: fathom receipt verify [--repo <path>] [--run-tests] <receipt.json>")
		return 2
	}

	data, err := os.ReadFile(rest[0]) //nolint:gosec — rest[0] is an operator-supplied CLI argument, the tool's whole purpose
	if err != nil {
		fmt.Fprintf(stderr, "receipt verify: %v\n", err)
		return 2
	}
	var r core.Receipt
	if err := json.Unmarshal(data, &r); err != nil {
		fmt.Fprintf(stderr, "receipt verify: invalid receipt JSON: %v\n", err)
		return 2
	}

	if r.Kind == core.ReceiptKindCertification {
		return cmdReceiptVerifyCertification(*repo, r, *runTests, stdout, stderr)
	}

	outcome, err := receiptverify.Verify(context.Background(), *repo, r)
	if err != nil {
		fmt.Fprintf(stderr, "receipt verify: cannot run: %v\n", err)
		return 2
	}
	if outcome.OK {
		fmt.Fprintf(stdout, "receipt verify: OK — %s reproduced at %s\n", r.Symbol, r.IndexCommit)
		return 0
	}
	fmt.Fprintf(stdout, "receipt verify: DIFFERENCE — %s\n", outcome.Diff)
	return 1
}

// receiptRunner runs `fathom receipt` (registered below; EW-45).
type receiptRunner struct{}

func (receiptRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdReceipt(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "receipt", Group: "govern", Order: 70, Run: receiptRunner{},
		Summary: "A stranger's re-derivation (VB-20): verify [--repo DIR] <receipt.json> checks a saved answer against a bare git checkout with go/types and git alone — no daemon, no store"})
}
