// receipt.go — the `fathom receipt` verb family (VB-20, docs/vision/
// 29-verbs-not-lookups.md §3 wave E; ADR-035 decision 4): today just
// `verify`, which re-derives a saved agent-surface answer's claim from a
// bare git checkout with go/types and git alone — no fathom daemon, no
// store, so a stranger reproduces (or refutes) it independently.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/receipt"
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
	if err := uniqueKeys(json.NewDecoder(bytes.NewReader(data))); err != nil {
		fmt.Fprintf(stderr, "receipt verify: invalid receipt JSON: %v\n", err)
		return 2
	}
	var exported receipt.ExportedReceipt
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&exported); err != nil {
		fmt.Fprintf(stderr, "receipt verify: invalid receipt JSON: %v\n", err)
		return 2
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		fmt.Fprintln(stderr, "receipt verify: invalid receipt JSON: trailing data")
		return 2
	}

	r := exported.Receipt
	var outcome receiptverify.Outcome
	if r.Kind == core.ReceiptKindCertification {
		outcome, err = receiptverify.VerifyGitAnchors(context.Background(), *repo, exported.Facts)
	} else {
		outcome, err = receiptverify.VerifyExported(context.Background(), *repo, exported)
	}
	for _, rung := range outcome.Rungs {
		fmt.Fprintln(stdout, rung)
	}
	if err != nil {
		fmt.Fprintf(stderr, "receipt verify: cannot run: %v\n", err)
		return 2
	}
	if outcome.OK {
		if r.Kind == core.ReceiptKindCertification {
			return cmdReceiptVerifyCertification(*repo, r, *runTests, stdout, stderr)
		}
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
		Summary: "A stranger's re-derivation: verify [--repo DIR] <receipt.json> checks a saved answer against a bare git checkout with go/types and git alone — no daemon, no store"})
}

// uniqueKeys refuses a key repeated in one object, exactly or by case:
// encoding/json keeps the last of them, another reader may show the first.
func uniqueKeys(dec *json.Decoder) error {
	t, err := dec.Token()
	d, ok := t.(json.Delim)
	if err != nil || !ok {
		return err
	}
	seen := map[string]bool{}
	for err == nil && dec.More() {
		if d == '{' {
			k, kerr := dec.Token()
			fold := strings.ToUpper(strings.ToLower(fmt.Sprint(k)))
			if kerr == nil && seen[fold] {
				return fmt.Errorf("key %q repeats in one object", k)
			}
			seen[fold], err = true, kerr
		}
		if err == nil {
			err = uniqueKeys(dec)
		}
	}
	if err == nil {
		_, err = dec.Token()
	}
	return err
}
