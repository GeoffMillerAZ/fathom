package main

// fleetwiring.go — `fathom fleet receipts` and `fathom fleet undo` (SH-6,
// docs/plans/ambient-fathom.md; docs/design/self-healing-fleet.md §5).
//
// Phase U's premise is that fathom should execute the wiring it currently
// only advises. The premise is only defensible if every write is reversible
// by the person whose repository it is, from the same terminal, without
// reading fathom's source. Two verbs:
//
//	fathom fleet receipts               what fathom has written here, newest first
//	fathom fleet undo --last            restore the most recent apply
//	fathom fleet undo <receipt-id>      restore that one
//
// Undo is a RESTORE FROM RECEIPT: the receipt carries the full prior
// content of every file touched, so undo never has to reason about what a
// change meant. It refuses — writing nothing at all — when any file in the
// receipt has changed since it was written, naming the file and the reason.
// That refusal is law 15 pointed the other way: restoring into a world that
// has moved on would destroy whatever moved it.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// resolveRepoRoot turns a --repo flag into an absolute path, defaulting to
// the working directory. An unreadable root is an error, not a silent
// fallback: writing receipts for the wrong repository is exactly the class
// of mistake this whole phase exists to avoid.
func resolveRepoRoot(repo string) (string, error) {
	if repo == "" {
		repo = "."
	}
	abs, err := filepath.Abs(repo)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", abs)
	}
	return abs, nil
}

func cmdFleetReceipts(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("fleet receipts", stderr)
	repo := fs.String("repo", "", "repository root (default: the working directory)")
	limit := fs.Int("limit", 20, "how many receipts to list, newest first")
	asJSON := fs.Bool("json", false, "emit JSON instead of a table")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	root, err := resolveRepoRoot(*repo)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	receipts, err := wiring.LoadReceipts(root)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if *limit > 0 && len(receipts) > *limit {
		receipts = receipts[:*limit]
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(map[string]any{"root": root, "receipts": receipts}); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}

	if len(receipts) == 0 {
		// Law 3: an empty list is a measurement with a reason, not a zero.
		fmt.Fprintf(stdout, "no receipts in %s — fathom has written nothing here\n", root)
		return 0
	}
	fmt.Fprintf(stdout, "%s — %d receipt(s), newest first\n", root, len(receipts))
	for _, r := range receipts {
		fmt.Fprintf(stdout, "  %s\n", r.Summary())
		for _, f := range r.Files {
			state := "restored to"
			if !f.Existed {
				state = "created (undo removes)"
			}
			fmt.Fprintf(stdout, "      %-28s  undo %s %d prior byte(s)\n", f.Path, state, len(f.Before))
		}
	}
	fmt.Fprintln(stdout, "undo the most recent with: fathom fleet undo --last")
	return 0
}

func cmdFleetUndo(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("fleet undo", stderr)
	repo := fs.String("repo", "", "repository root (default: the working directory)")
	last := fs.Bool("last", false, "undo the most recent apply")
	asJSON := fs.Bool("json", false, "emit JSON instead of prose")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	root, err := resolveRepoRoot(*repo)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	id := ""
	if fs.NArg() > 0 {
		id = fs.Arg(0)
	}
	if id == "" && !*last {
		fmt.Fprintln(stderr, "usage: fathom fleet undo [--repo DIR] (--last | <receipt-id>)")
		return 1
	}
	if id != "" && *last {
		fmt.Fprintln(stderr, "error: pass --last or a receipt id, not both")
		return 1
	}

	res, err := wiring.Undo(root, id, wiring.ApplyOptions{Now: time.Now})
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		payload := map[string]any{"root": root, "result": res}
		if err != nil {
			payload["error"] = err.Error()
		}
		if encErr := enc.Encode(payload); encErr != nil {
			fmt.Fprintf(stderr, "error: %v\n", encErr)
			return 1
		}
		if err != nil {
			return 1
		}
		return 0
	}

	if errors.Is(err, wiring.ErrUndoRefused) {
		// The refusal is the product here, so it is printed in full on
		// stdout: which file, what changed, and what fathom did instead
		// (nothing).
		fmt.Fprintf(stdout, "undo REFUSED for receipt %s — nothing was written\n", res.Receipt.ID)
		for _, r := range res.Refused {
			fmt.Fprintf(stdout, "  %s: %s\n", r.Path, r.Reason)
		}
		fmt.Fprintln(stdout, "fathom will not restore over a change it did not make. Resolve the file by hand, or leave it.")
		return 1
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "undid receipt %s (%s)\n", res.Receipt.ID, res.Receipt.Time)
	for _, p := range res.Restored {
		fmt.Fprintf(stdout, "  restored %s\n", p)
	}
	for _, p := range res.Removed {
		fmt.Fprintf(stdout, "  removed  %s (fathom created it)\n", p)
	}
	if res.Receipted != nil {
		fmt.Fprintf(stdout, "the undo is itself receipted as %s — `fathom fleet undo %s` puts it back\n", res.Receipted.ID, res.Receipted.ID)
	}
	return 0
}
