package main

// module_sources.go — `fathom module sources`: resolve a manifest's corpus
// selectors through the connector discovery path and print the inventory
// (docs/vision/13-expert-suite.md §4 step 2, "the inventory panel shows
// what's actually in corpus... before any LLM spend").

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/modspec"
)

// sourceInventory is the resolved corpus inventory `sources` and `map` both
// compute (map reuses this for its zero-match-selector gap).
type sourceInventory struct {
	Root            string           `json:"root"`
	TotalFiles      int              `json:"total_files"`
	TotalMatched    int              `json:"total_matched"`
	TotalBytes      int64            `json:"total_bytes"`
	ByClass         map[string]int   `json:"by_class"`
	ByLanguage      map[string]int   `json:"by_language"`
	ZeroMatch       []string         `json:"zero_match_selectors"`
	MatchedBySelect map[string]int   `json:"matched_by_selector"`
	Files           []inventoryEntry `json:"-"` // omitted from JSON: potentially large; sources --json prints the summary only
}

type inventoryEntry struct {
	Path     string
	Class    string
	Language string
	Bytes    int64
}

// resolveSourceInventory discovers every file under root via the LocalConnector
// (respecting .fathomignore, same as ingest), matches it against the
// manifest's include/exclude selectors, and buckets the matched set by class
// (core.SourceKind) and language. Zero-match selectors are flagged loudly —
// never silently dropped.
func resolveSourceInventory(ctx context.Context, root string, m modspec.Manifest) (sourceInventory, error) {
	conn := connector.NewLocalConnector()
	refs, err := conn.Discover(ctx, core.ServiceRef{ID: m.Name, Path: root})
	if err != nil {
		return sourceInventory{}, fmt.Errorf("discover %s: %w", root, err)
	}

	inc := filter.New(m.Corpus.Include)
	var excl *filter.Matcher
	if len(m.Corpus.Exclude) > 0 {
		excl = filter.New(m.Corpus.Exclude)
	}

	inv := sourceInventory{
		Root:            root,
		TotalFiles:      len(refs),
		ByClass:         map[string]int{},
		ByLanguage:      map[string]int{},
		MatchedBySelect: map[string]int{},
	}
	for _, sel := range m.Corpus.Include {
		inv.MatchedBySelect[sel] = 0
	}

	for _, ref := range refs {
		if !inc.Match(ref.Path) {
			continue
		}
		if excl != nil && excl.Match(ref.Path) {
			continue
		}
		inv.TotalMatched++
		inv.ByClass[string(ref.Kind)]++
		lang := connector.LanguageForPath(ref.Path)
		if lang == "" {
			lang = "(none)"
		}
		inv.ByLanguage[lang]++
		for _, sel := range m.Corpus.Include {
			if filter.New([]string{sel}).Match(ref.Path) {
				inv.MatchedBySelect[sel]++
			}
		}
		var size int64
		if fi, serr := os.Stat(filepath.Join(root, filepath.FromSlash(ref.Path))); serr == nil {
			size = fi.Size()
		}
		inv.TotalBytes += size
		inv.Files = append(inv.Files, inventoryEntry{Path: ref.Path, Class: string(ref.Kind), Language: lang, Bytes: size})
	}

	for sel, n := range inv.MatchedBySelect {
		if n == 0 {
			inv.ZeroMatch = append(inv.ZeroMatch, sel)
		}
	}
	sort.Strings(inv.ZeroMatch)
	sort.Slice(inv.Files, func(i, j int) bool { return inv.Files[i].Path < inv.Files[j].Path })

	return inv, nil
}

func cmdModuleSources(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("module sources", stderr)
	manifestPath := fs.String("manifest", "", "path to module.cue (required)")
	root := fs.String("root", "", "corpus root (default: module.cue's own directory)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of human text")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *manifestPath == "" {
		fmt.Fprintln(stderr, "error: --manifest is required")
		fs.Usage()
		return 1
	}
	m, err := modspec.LoadManifest(*manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	corpusRoot := moduleServiceRoot(*manifestPath, *root)
	inv, err := resolveSourceInventory(context.Background(), corpusRoot, m)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if *jsonOut {
		jsonPrint(stdout, inv)
		if len(inv.ZeroMatch) > 0 {
			return 1
		}
		return 0
	}

	fmt.Fprintf(stdout, "root:          %s\n", inv.Root)
	fmt.Fprintf(stdout, "total files:   %d (discovered)\n", inv.TotalFiles)
	fmt.Fprintf(stdout, "matched files: %d\n", inv.TotalMatched)
	fmt.Fprintf(stdout, "total bytes:   %d\n", inv.TotalBytes)
	// Scoped config disclosure (docs/design/scoped-config.md §4): resolveSourceInventory's
	// own Discover call already applied the union; this is a cheap,
	// display-only re-read of the same counts LoadIgnoreScoped exposes.
	if _, counts, ierr := connector.LoadIgnoreScoped(corpusRoot); ierr == nil && (counts.Project > 0 || counts.User > 0) {
		fmt.Fprintf(stdout, "excluded:      %d pattern(s) (%d project, %d user-scope ~/.fathom/ignore)\n", counts.Project+counts.User, counts.Project, counts.User)
	}
	fmt.Fprintln(stdout, "\nby class:")
	printSortedCounts(stdout, inv.ByClass)
	fmt.Fprintln(stdout, "\nby language:")
	printSortedCounts(stdout, inv.ByLanguage)
	fmt.Fprintln(stdout, "\nby selector:")
	sels := make([]string, 0, len(inv.MatchedBySelect))
	for s := range inv.MatchedBySelect {
		sels = append(sels, s)
	}
	sort.Strings(sels)
	for _, s := range sels {
		fmt.Fprintf(stdout, "  %-40s %d\n", s, inv.MatchedBySelect[s])
	}
	if len(inv.ZeroMatch) > 0 {
		fmt.Fprintln(stdout, "\nZERO-MATCH SELECTORS (flagged loudly — these globs matched nothing):")
		for _, s := range inv.ZeroMatch {
			fmt.Fprintf(stdout, "  ! %s\n", s)
		}
		return 1
	}
	return 0
}

func printSortedCounts(w io.Writer, m map[string]int) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(w, "  %-20s %d\n", k, m[k])
	}
}
