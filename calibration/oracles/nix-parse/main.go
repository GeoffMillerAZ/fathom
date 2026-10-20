// nix-parse -- the Nix dialect's oracle (LB-Q-nix, tier C): the reference parser, `nix-instantiate --parse`,
// run as a separate process per file. See README.md.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

type xrefOut struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	ToPath string `json:"to_path"`
}

type output struct {
	Tool        string      `json:"tool"`
	Version     string      `json:"version"`
	Tier        string      `json:"tier"`
	Definitions []defOut    `json:"definitions"`
	References  []pairOut   `json:"references"`
	CrossRefs   []xrefOut   `json:"cross_references"`
	Imports     []importOut `json:"imports"`
	Disclose    []string    `json:"disclose,omitempty"`
}

func main() {
	subject := flag.String("subject", "", "subject checkout")
	nix := flag.String("nix-instantiate", "nix-instantiate", "the reference parser")
	filesOut := flag.String("files-out", "", "write the files the parser accepted, one per line")
	jobs := flag.Int("j", 4, "parallel parses")
	flake := flag.Bool("flake", false, "also run `nix flake show --json --offline` on the subject root when it holds a flake.nix, and disclose the outputs it lists")
	nixBin := flag.String("nix", "nix", "the nix command, for -flake")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "usage: nix-parse -subject <checkout> [-nix-instantiate <bin>] [-files-out <list>] > oracle.json")
		os.Exit(2)
	}
	root, err := filepath.Abs(*subject)
	if err != nil {
		fatal(err)
	}
	var files []string
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return fs.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".nix") && d.Type().IsRegular() {
			rel, _ := filepath.Rel(root, p)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		fatal(err)
	}
	sort.Strings(files)

	ver, _ := exec.Command(*nix, "--version").Output()
	out := output{Tool: "nix-instantiate --parse + source locator", Version: strings.TrimSpace(string(ver)), Tier: "C"}

	type job struct {
		rel string
		res *fileResult
		why string
	}
	results := make([]job, len(files))
	var wg sync.WaitGroup
	sem := make(chan struct{}, max(1, *jobs))
	for i, rel := range files {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = job{rel: rel}
			src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				results[i].why = "unreadable"
				return
			}
			printed, why := parseFile(*nix, filepath.Join(root, filepath.FromSlash(rel)))
			if why != "" {
				results[i].why = why
				return
			}
			res, err := extract(root, rel, src, printed)
			if err != nil {
				results[i].why = "ast_unreadable: " + err.Error()
				results[i].res = res
				return
			}
			results[i].res = res
		}()
	}
	wg.Wait()

	counts := map[string]int{}
	exports := map[string]map[string]bool{}
	var accepted []string
	var unread, notes []string
	for _, j := range results {
		counts["files"]++
		if j.why != "" {
			if strings.HasPrefix(j.why, "ast_unreadable") {
				counts["ast_unreadable"]++
				unread = append(unread, j.rel+": "+j.why)
			} else {
				counts["rejected_by_parser"]++
			}
			continue
		}
		counts["accepted"]++
		accepted = append(accepted, j.rel)
		notes = append(notes, j.res.notes...)
		for k, v := range j.res.count {
			counts[k] += v
		}
		sortResult(j.res)
		out.Definitions = append(out.Definitions, j.res.defs...)
		out.References = append(out.References, j.res.calls...)
		exports[j.rel] = j.res.exports
		out.Imports = append(out.Imports, j.res.imports...)
	}
	if out.Definitions == nil {
		out.Definitions = []defOut{}
	}
	if out.References == nil {
		out.References = []pairOut{}
	}
	seenX := map[xrefOut]bool{}
	for _, j := range results {
		if j.res == nil || j.why != "" {
			continue
		}
		for _, c := range j.res.cands {
			if c.to != j.rel && exports[c.to][c.attr] {
				x := xrefOut{j.rel, c.from, c.to}
				if !seenX[x] {
					seenX[x] = true
					out.CrossRefs = append(out.CrossRefs, x)
				}
			}
		}
	}
	sort.Slice(out.CrossRefs, func(a, b int) bool {
		x, y := out.CrossRefs[a], out.CrossRefs[b]
		return x.Path+"\x00"+x.From+"\x00"+x.ToPath < y.Path+"\x00"+y.From+"\x00"+y.ToPath
	})
	if out.CrossRefs == nil {
		out.CrossRefs = []xrefOut{}
	}
	counts["cross_references"] = len(out.CrossRefs)
	if out.Imports == nil {
		out.Imports = []importOut{}
	}
	var keys []string
	for k := range counts {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, counts[k]))
	}
	out.Disclose = []string{"nix-parse: " + strings.Join(parts, " ")}
	for _, u := range unread[:min(len(unread), 5)] {
		out.Disclose = append(out.Disclose, "nix-parse unread: "+u)
	}
	for _, u := range notes[:min(len(notes), 8)] {
		out.Disclose = append(out.Disclose, "nix-parse "+u)
	}
	if *flake {
		out.Disclose = append(out.Disclose, flakeShow(*nixBin, root, accepted))
	}
	if *filesOut != "" {
		if err := os.WriteFile(*filesOut, []byte(strings.Join(accepted, "\n")+"\n"), 0o644); err != nil {
			fatal(err)
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(out); err != nil {
		fatal(err)
	}
}

// parseFile runs the reference parser on one file: its printed expression, or why it refused.
func parseFile(bin, file string) (string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--parse", file)
	cmd.Env = append(os.Environ(), "NIX_CONFIG=substituters =\nconnect-timeout = 1")
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	if err := cmd.Run(); err != nil {
		first, _, _ := strings.Cut(strings.TrimSpace(se.String()), "\n")
		return "", "rejected: " + first
	}
	return so.String(), ""
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "nix-parse:", err)
	os.Exit(1)
}

// flakeShow runs `nix flake show --json --offline` on the subject root (tier A for a flake's outputs, where it
// can run without fetching: an input already in the store at the lock's hash needs no fetch) and reports the
// top-level outputs it lists, and whether the root flake.nix reads as the reader expects (a root `outputs`
// definition). It writes no lock file. It is disclosed, never scored: output names are keys below depth one.
func flakeShow(bin, root string, accepted []string) string {
	if !slices.Contains(accepted, "flake.nix") {
		return "nix flake show: no flake.nix at the subject root"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "flake", "show", "--json", "--offline", "--no-write-lock-file", root)
	cmd.Env = append(os.Environ(), "NIX_CONFIG=substituters =\nconnect-timeout = 1\nexperimental-features = nix-command flakes")
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	if err := cmd.Run(); err != nil {
		lines := strings.Split(strings.TrimSpace(se.String()), "\n")
		return "nix flake show: could not run offline (" + strings.TrimSpace(lines[len(lines)-1]) + "); graded on the parse"
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(so.Bytes(), &top); err != nil {
		return "nix flake show: unreadable output; graded on the parse"
	}
	var names []string
	for k := range top {
		names = append(names, k)
	}
	slices.Sort(names)
	return "nix flake show --json --offline: ran; top-level outputs " + strings.Join(names, ", ")
}
