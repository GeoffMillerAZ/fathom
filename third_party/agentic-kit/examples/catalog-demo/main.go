// Command catalog-demo is a worked example for foundations/knowledge/catalog:
// it scans a small directory tree into a Catalog, loads a previously
// "published" catalog from a fixture, and gates the two with Verify —
// design deliverable #8 ("a worked example building a small catalog from
// a directory scan and gating it"). It is a demo, not a library binary:
// per the design's non-goals, this package owns no verb name a consumer
// is meant to reuse.
//
// Run from this directory: go run .
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/knowledge/catalog"
)

// stableKeys maps a scanned file's path (relative to the scanned root) to
// a durable business key that survives a rename. In a real system this
// might come from document frontmatter, a content hash, or a registry;
// here it is a small hardcoded table so the demo can show Verify
// detecting a rename as Moved rather than a Missing+Extra pair.
var stableKeys = map[string]string{
	"docs/guide.md":         "doc:guide",
	"docs/api-reference.md": "doc:reference",
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "catalog-demo:", err)
		os.Exit(1)
	}
}

func run() error {
	built, err := scan("testdata/tree")
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	publishedData, err := os.ReadFile("testdata/published.json")
	if err != nil {
		return fmt.Errorf("read published.json: %w", err)
	}
	published, err := catalog.LoadAuthoredJSON(publishedData, "")
	if err != nil {
		return fmt.Errorf("load published.json: %w", err)
	}

	fmt.Println(catalog.RenderMarkdown(built, catalog.MarkdownOptions{
		Title:        "# Demo catalog (live scan)",
		RegenCommand: "go run ./examples/catalog-demo",
	}))

	drift, err := catalog.Verify(built, published)
	if err != nil {
		// This is also the gate: Verify refuses rather than guess when a
		// StableKey is ambiguous (design addition — see
		// catalog.DuplicateStableKeyError), so a consumer treats that
		// refusal as a hard failure exactly like a non-clean Drift.
		return fmt.Errorf("verify: %w", err)
	}
	fmt.Printf("Verify: checked %d built node(s) against %d published node(s); clean=%v\n",
		drift.CheckedBuilt, drift.CheckedPublished, drift.Clean())
	for _, e := range drift.Missing {
		fmt.Printf("  MISSING  %-28s %s\n", e.ID, e.Title)
	}
	for _, e := range drift.Extra {
		fmt.Printf("  EXTRA    %-28s %s\n", e.ID, e.Title)
	}
	for _, e := range drift.Changed {
		fmt.Printf("  CHANGED  %-28s %s\n", e.ID, e.Title)
	}
	for _, m := range drift.Moved {
		fmt.Printf("  MOVED    %-28s %s -> %s\n", m.StableKey, m.From, m.To)
	}

	// This is the gate itself: a consumer wires Verify exactly this way
	// — a non-clean Drift fails the build (design §4).
	if !drift.Clean() {
		return fmt.Errorf("catalog drifted from testdata/published.json (see above)")
	}
	return nil
}

// slugPattern matches everything ValidateSlug rejects, so slugify can
// replace it with a single hyphen.
var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

// slugify turns an arbitrary file/directory name into a valid catalog
// slug (lowercase kebab-case): "guide.md" -> "guide-md".
func slugify(name string) string {
	s := slugPattern.ReplaceAllString(strings.ToLower(name), "-")
	return strings.Trim(s, "-")
}

// scan builds a Catalog from a directory tree: one group Node per
// directory, one leaf Node per file, Title = base name, ParentID/ID
// derived from the walked path (slugified), StableKey looked up from
// stableKeys when known.
func scan(root string) (catalog.Catalog, error) {
	pathToID := map[string]string{".": ""} // raw rel path -> computed node ID; "." is the scan root itself
	var nodes []catalog.Node

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		parentRel := filepath.ToSlash(filepath.Dir(rel))
		parentID, ok := pathToID[parentRel]
		if !ok {
			return fmt.Errorf("parent of %s not visited yet (WalkDir order assumption violated)", rel)
		}

		n := catalog.Node{
			ParentID:   parentID,
			Slug:       slugify(d.Name()),
			Title:      d.Name(),
			Provenance: catalog.Provenance{Source: filepath.ToSlash(path)},
		}
		if key, ok := stableKeys[rel]; ok {
			n.StableKey = key
		}
		nodes = append(nodes, n)
		pathToID[rel] = n.ID()
		return nil
	})
	if err != nil {
		return catalog.Catalog{}, err
	}
	return catalog.Build(catalog.Layer{Name: "scan", Nodes: nodes})
}
