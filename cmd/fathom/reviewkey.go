package main

// reviewkey.go — `fathom review-key` (FL-17/FL-18/FL-19, docs/plans/
// findings-loop.md; vision 30 §3.2, ADR-040 decision 6): computes one
// review key over every input a review verdict depends on, through the
// SAME code path pkg/client.ReviewKey exposes to any other Go embedder
// (FL-19) — this file only parses flags, resolves the changed-path
// manifest (from --since or --manifest), and prints.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/reviewkey"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func cmdReviewKey(args []string, stdout, stderr io.Writer) int {
	return runReviewKey(args, defaultGitRunner, stdout, stderr)
}

func runReviewKey(args []string, gitRunner func(args ...string) ([]byte, error), stdout, stderr io.Writer) int {
	fs := newFlagSet("review-key", stderr)
	storePath := fs.String("store", defaultStorePath(), "store path (.sqlite/.db)")
	service := fs.String("service", "", "service ID (required)")
	since := fs.String("since", "", "git ref to diff against (mutually exclusive with --manifest; one of the two is required)")
	manifestPath := fs.String("manifest", "", "explicit changed-path manifest FILE (JSON [{\"path\":\"…\",\"sha256\":\"…\"}]) — its declared digests are trusted as-is, mutually exclusive with --since")
	priorPath := fs.String("prior", "", "a prior review-key JSON output to diff against — sets changed_since_prior")
	jsonOut := fs.Bool("json", false, "emit the Key as JSON instead of a human-readable summary")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *service == "" {
		fmt.Fprintln(stderr, "error: review-key requires --service")
		fs.Usage()
		return 1
	}
	if (*since == "") == (*manifestPath == "") {
		fmt.Fprintln(stderr, "error: review-key requires exactly one of --since or --manifest")
		fs.Usage()
		return 1
	}

	st, err := store.Open(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", *storePath, err)
		return 1
	}
	defer st.Close()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	ctx := context.Background()

	corpusRoot := ""
	if path, _, found, merr := c.ServiceMeta(ctx, *service); merr == nil && found {
		corpusRoot = path
	}

	var manifest []reviewkey.ManifestEntry
	if *manifestPath != "" {
		entries, merr := loadReviewManifest(*manifestPath)
		if merr != nil {
			fmt.Fprintf(stderr, "error: %v\n", merr)
			return 1
		}
		for _, e := range entries {
			manifest = append(manifest, reviewkey.ManifestEntry{Path: e.Path, SHA256: e.SHA256})
		}
	} else {
		out, gerr := gitRunner("diff", "--name-only", *since)
		if gerr != nil {
			fmt.Fprintf(stderr, "error: git diff --name-only %s: %v\n", *since, gerr)
			return 1
		}
		for _, line := range strings.Split(string(out), "\n") {
			path := strings.TrimSpace(line)
			if path == "" {
				continue
			}
			full := path
			if corpusRoot != "" {
				full = filepath.Join(corpusRoot, path)
			}
			data, rerr := os.ReadFile(full)
			if rerr != nil {
				manifest = append(manifest, reviewkey.ManifestEntry{Path: path, Unreadable: true})
				continue
			}
			sum := sha256.Sum256(data)
			manifest = append(manifest, reviewkey.ManifestEntry{Path: path, SHA256: hex.EncodeToString(sum[:])})
		}
	}

	var prior *reviewkey.Key
	if *priorPath != "" {
		data, perr := os.ReadFile(*priorPath)
		if perr != nil {
			fmt.Fprintf(stderr, "error: read --prior %q: %v\n", *priorPath, perr)
			return 1
		}
		var p reviewkey.Key
		if err := json.Unmarshal(data, &p); err != nil {
			fmt.Fprintf(stderr, "error: parse --prior %q: %v\n", *priorPath, err)
			return 1
		}
		prior = &p
	}

	key, err := c.ReviewKey(ctx, reviewkey.Request{Service: *service, Manifest: manifest, Prior: prior})
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(key); err != nil {
			fmt.Fprintf(stderr, "error: encode JSON: %v\n", err)
			return 1
		}
		return 0
	}

	printReviewKeySummary(stdout, key)
	return 0
}

// printReviewKeySummary renders key as a short, human-readable report:
// the overall key, each component's digest, and every unknown reason —
// never silently clean when Complete is false.
func printReviewKeySummary(w io.Writer, key reviewkey.Key) {
	fmt.Fprintf(w, "review-key: %s (complete=%v)\n", key.Key, key.Complete)
	names := []string{
		reviewkey.ComponentManifest, reviewkey.ComponentClosure, reviewkey.ComponentRules,
		reviewkey.ComponentPlans, reviewkey.ComponentTooling, reviewkey.ComponentIndex,
	}
	for _, name := range names {
		fmt.Fprintf(w, "  %-10s %s\n", name, key.Components[name])
	}
	if len(key.Unknown) > 0 {
		fmt.Fprintln(w, "unknown:")
		for _, u := range key.Unknown {
			fmt.Fprintf(w, "  - %s\n", u)
		}
	}
	if len(key.ChangedSincePrior) > 0 {
		fmt.Fprintf(w, "changed since prior: %s\n", strings.Join(key.ChangedSincePrior, ", "))
	}
}

// reviewKeyRunner runs `fathom review-key` (registered below; EW-45).
type reviewKeyRunner struct{}

func (reviewKeyRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdReviewKey(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "review-key", Group: "review", Order: 30, Run: reviewKeyRunner{},
		Summary: "One sha256 review key over manifest + typed dependency closure + rules + plans + tooling + index identity (--since REF | --manifest FILE) [--prior FILE]; complete:false + unknown[] name any input it could not verify"})
}
