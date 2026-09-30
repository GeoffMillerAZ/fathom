package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// cmdLangPin is `indexbench lang-pin` (design §8.2 and §13.1 steps 5-6, LB-F50
// and LB-F51), three modes:
//
//	-catalogue <checkout> -rev <40-hex> [-write]   pre-fill the grammar lock rows
//	    of the queue's unlocked tags items from tree-sitter-language-pack's
//	    catalogue at that revision into docs/plans/language-breadth.grammars.json
//	    (stdout without -write); the override map and the recorded verdicts in
//	    the file are re-read, so a regeneration keeps them.
//	-id <id>                                       print the draft GrammarLockRow
//	    (digests empty for build.sh) on stdout and the flags on stderr; exit 1
//	    with no row when the grammar is refused or unlicensed.
//	-tree <dir|tarball>                            the licence verdict at the
//	    pinned commit as JSON on stdout; exit 1 when refused.
func cmdLangPin(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lang-pin", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "repository root")
	catalogue := fs.String("catalogue", "", "a tree-sitter-language-pack checkout: pre-fill the lock rows from its sources/")
	rev := fs.String("rev", "", "the catalogue checkout's revision (40 hex; a git checkout must be at it)")
	write := fs.Bool("write", false, "with -catalogue: write "+langbench.GrammarPrefillPath+" instead of printing it")
	id := fs.String("id", "", "print the draft lock row for this queue id")
	tree := fs.String("tree", "", "a grammar checkout or tarball at the pinned commit: print its licence verdict")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	modes := 0
	for _, s := range []string{*catalogue, *id, *tree} {
		if s != "" {
			modes++
		}
	}
	if modes != 1 {
		fmt.Fprintln(stderr, "lang-pin: give exactly one of -catalogue <checkout> -rev <sha> [-write], -id <id>, -tree <dir|tarball>")
		return 2
	}
	if *write && *catalogue == "" {
		fmt.Fprintln(stderr, "lang-pin: -write applies to -catalogue")
		return 2
	}
	switch {
	case *catalogue != "":
		return langPinCatalogue(*root, *catalogue, *rev, *write, stdout, stderr)
	case *id != "":
		return langPinDraft(*root, *id, stdout, stderr)
	default:
		return langPinTree(*tree, stdout, stderr)
	}
}

func langPinCatalogue(root, catalogue, rev string, write bool, stdout, stderr io.Writer) int {
	if rev == "" {
		fmt.Fprintln(stderr, "lang-pin: -catalogue needs -rev <40-hex revision>")
		return 2
	}
	if head, ok, err := checkoutHead(catalogue); err != nil {
		fmt.Fprintln(stderr, "lang-pin:", err)
		return 1
	} else if ok && head != rev {
		fmt.Fprintf(stderr, "lang-pin: the catalogue checkout is at %s, not -rev %s\n", head, rev)
		return 1
	}
	q, err := langbench.LoadQueue(filepath.Join(root, "docs", "plans", "language-breadth.queue.json"))
	if err != nil {
		fmt.Fprintln(stderr, "lang-pin:", err)
		return 1
	}
	lb, err := os.ReadFile(filepath.Join(root, "internal", "extractor", "tsengine", "grammars", "grammars.lock.json"))
	if err != nil {
		fmt.Fprintln(stderr, "lang-pin:", err)
		return 1
	}
	lock, err := tsengine.ParseGrammarLock(lb)
	if err != nil {
		fmt.Fprintln(stderr, "lang-pin:", err)
		return 1
	}
	cat, err := langbench.LoadCatalogue(catalogue)
	if err != nil {
		fmt.Fprintln(stderr, "lang-pin:", err)
		return 1
	}
	dataPath := filepath.Join(root, filepath.FromSlash(langbench.GrammarPrefillPath))
	prior, err := langbench.LoadGrammarPrefill(dataPath)
	if err != nil {
		fmt.Fprintln(stderr, "lang-pin:", err)
		return 1
	}
	data, err := langbench.PrefillLockRows(q, lock, cat, prior, rev)
	if err != nil {
		fmt.Fprintln(stderr, "lang-pin:", err)
		return 1
	}
	b, err := langbench.MarshalPrefill(data)
	if err != nil {
		fmt.Fprintln(stderr, "lang-pin:", err)
		return 1
	}
	if !write {
		_, _ = stdout.Write(b)
		return 0
	}
	if err := os.WriteFile(dataPath, b, 0o644); err != nil {
		fmt.Fprintln(stderr, "lang-pin:", err)
		return 1
	}
	blocked := 0
	for _, e := range data.Grammars {
		for _, f := range e.Flags {
			if f == "unlicensed" || f == "not-in-catalogue" || strings.HasPrefix(f, "refused:") || strings.HasPrefix(f, "abi-outside:") {
				blocked++
				break
			}
		}
	}
	fmt.Fprintf(stdout, "wrote %s: %d grammars pre-filled from %s @ %s, %d with a blocking flag\n",
		langbench.GrammarPrefillPath, len(data.Grammars), data.Source, rev[:12], blocked)
	return 0
}

func langPinDraft(root, id string, stdout, stderr io.Writer) int {
	data, err := langbench.LoadGrammarPrefill(filepath.Join(root, filepath.FromSlash(langbench.GrammarPrefillPath)))
	if err != nil {
		fmt.Fprintln(stderr, "lang-pin:", err)
		return 1
	}
	dr, err := data.Draft(id)
	if err != nil {
		fmt.Fprintln(stderr, "lang-pin:", err)
		return 1
	}
	for _, f := range dr.Flags {
		fmt.Fprintln(stderr, "flag:", f)
	}
	if v, ok := data.Verdicts[id]; ok {
		fmt.Fprintf(stderr, "verdict: %s @ %s is %s (%s)\n", v.Repo, v.Commit, verdictWord(v.LicenceVerdict), v.Basis)
	}
	if dr.Refused {
		for _, r := range dr.Reasons {
			fmt.Fprintln(stderr, r)
		}
		return 1
	}
	b, err := json.MarshalIndent(dr.Row, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, "lang-pin:", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s\n", b)
	return 0
}

func verdictWord(v langbench.LicenceVerdict) string {
	if v.Flag() != "" {
		return v.Flag()
	}
	return v.Licence
}

func langPinTree(tree string, stdout, stderr io.Writer) int {
	files, err := langbench.LicenceTreeFilesFromPath(tree)
	if err != nil {
		fmt.Fprintln(stderr, "lang-pin:", err)
		return 1
	}
	v := langbench.GrammarLicenceVerdict(files)
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, "lang-pin:", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s\n", b)
	if v.Refused {
		fmt.Fprintf(stderr, "lang-pin: refused (%s): %s\n", v.Flag(), v.Reason)
		return 1
	}
	return 0
}

// checkoutHead is the HEAD of dir when it is a git checkout (ok false when it
// is not one). The child runs without GIT_* variables: a hook's environment
// must not aim git at another repository.
func checkoutHead(dir string) (head string, ok bool, err error) {
	if _, statErr := os.Stat(filepath.Join(dir, ".git")); statErr != nil {
		return "", false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitHeadTimeout)
	defer cancel()
	c := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD")
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			c.Env = append(c.Env, kv)
		}
	}
	out, err := c.Output()
	if err != nil {
		return "", false, fmt.Errorf("git rev-parse HEAD in %s: %w", dir, err)
	}
	return strings.TrimSpace(string(out)), true, nil
}
