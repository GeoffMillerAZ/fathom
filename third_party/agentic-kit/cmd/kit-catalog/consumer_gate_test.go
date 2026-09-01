package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	internalcatalog "github.com/geoffmilleraz/agentic-kit/cmd/internal/catalog"
	"github.com/geoffmilleraz/agentic-kit/foundations/knowledge/catalog"
)

// TestConsumerGate_LiveScanVerifiesAgainstDocsCatalogMD is catalog-kit's
// first named consumer proven for real (design §10): it builds a
// foundations/knowledge/catalog Catalog from the SAME live source
// cmd/internal/catalog.Collect already scans, builds a second Catalog by
// parsing the COMMITTED docs/catalog.md bullet lines (unchanged format —
// this test reads the existing generated-doc shape, it does not add a
// new one), and runs Verify between them.
//
// This is deliberately redundant with constitution-lint's catalog-fresh
// check (a byte-for-byte string diff): catalog-fresh proves the
// cmd/internal/catalog renderer is fresh; this test proves the NEW
// package's own Verify/Drift machinery agrees with reality at the kit's
// real, current package count — turning the "three hand-agreed counts"
// the design calls out into an actual gate, on an entirely different
// code path than the one that generates docs/catalog.md in the first
// place.
func TestConsumerGate_LiveScanVerifiesAgainstDocsCatalogMD(t *testing.T) {
	entries, err := internalcatalog.Collect("../..")
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	built, err := buildPathTreeCatalog(entriesToPathItems(entries))
	if err != nil {
		t.Fatalf("build live-scan Catalog: %v", err)
	}

	mdData, err := os.ReadFile("../../docs/catalog.md")
	if err != nil {
		t.Fatalf("read docs/catalog.md: %v", err)
	}
	items, err := parseCatalogMarkdown(string(mdData))
	if err != nil {
		t.Fatalf("parse docs/catalog.md: %v", err)
	}
	published, err := buildPathTreeCatalog(items)
	if err != nil {
		t.Fatalf("build published Catalog: %v", err)
	}

	drift, err := catalog.Verify(built, published)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !drift.Clean() {
		t.Errorf("docs/catalog.md disagrees with the live source scan (regenerate with: task docs-index):\n"+
			"checked %d built node(s) against %d published node(s)\nmissing=%v\nextra=%v\nchanged=%v\nmoved=%v",
			drift.CheckedBuilt, drift.CheckedPublished, drift.Missing, drift.Extra, drift.Changed, drift.Moved)
	}
	if drift.CheckedBuilt < 100 {
		t.Errorf("expected >100 nodes from the live scan (sanity check the walk ran), got %d", drift.CheckedBuilt)
	}
}

// pathItem is one flat, path-addressed entry to fold into a hierarchical
// Catalog via buildPathTreeCatalog — the common shape shared by both
// cmd/internal/catalog.Entry (the live scan) and a parsed docs/catalog.md
// bullet line (the published doc), so the exact same tree-building logic
// runs on both sides and any real disagreement between them is a real
// Verify finding, not an artifact of two different builders.
type pathItem struct {
	RelPath  string
	Title    string
	Synopsis string
}

func entriesToPathItems(entries []internalcatalog.Entry) []pathItem {
	items := make([]pathItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, pathItem{RelPath: e.RelPath, Title: e.Name, Synopsis: e.Synopsis})
	}
	return items
}

// catalogLineRe matches one docs/catalog.md package bullet:
// "- `RelPath` (`pkgname`) — synopsis", exactly cmd/internal/catalog.Render's format.
var catalogLineRe = regexp.MustCompile("^- `([^`]+)` \\(`([^`]+)`\\) — (.*)$")

// parseCatalogMarkdown extracts one pathItem per package bullet line from
// docs/catalog.md, ignoring section headers and the leading banner/prose.
// "(no package doc)" (Render's placeholder for an empty synopsis) is
// mapped back to "", matching what Collect would have produced.
func parseCatalogMarkdown(md string) ([]pathItem, error) {
	var items []pathItem
	for _, line := range strings.Split(md, "\n") {
		m := catalogLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		synopsis := m[3]
		if synopsis == "(no package doc)" {
			synopsis = ""
		}
		items = append(items, pathItem{RelPath: m[1], Title: m[2], Synopsis: synopsis})
	}
	if len(items) == 0 {
		return nil, strings_ErrNoBullets
	}
	return items, nil
}

var strings_ErrNoBullets = errNoBullets{}

type errNoBullets struct{}

func (errNoBullets) Error() string { return "no package bullet lines found in docs/catalog.md" }

// buildPathTreeCatalog folds a flat list of path-addressed items into a
// hierarchical Catalog: every path segment gets an intermediate group
// Node (deduplicated, Title = segment name) except the final segment,
// which gets the item's own Title/Synopsis. This is the general
// arbitrary-depth construction this package proves elsewhere
// (TestArbitraryDepthNoSpecialCasing) applied to real fleet data instead
// of a synthetic chain.
func buildPathTreeCatalog(items []pathItem) (catalog.Catalog, error) {
	byID := map[string]catalog.Node{}
	var order []string

	ensureGroup := func(parentID, segment, id string) {
		if _, ok := byID[id]; ok {
			return
		}
		byID[id] = catalog.Node{ParentID: parentID, Slug: segment, Title: segment}
		order = append(order, id)
	}

	for _, item := range items {
		segments := strings.Split(item.RelPath, "/")
		parentID := ""
		for i, seg := range segments {
			id := catalog.JoinID(parentID, seg)
			if i == len(segments)-1 {
				byID[id] = catalog.Node{
					ParentID:   parentID,
					Slug:       seg,
					Title:      item.Title,
					Synopsis:   item.Synopsis,
					Provenance: catalog.Provenance{Source: item.RelPath},
				}
				if _, seen := indexOf(order, id); !seen {
					order = append(order, id)
				}
			} else {
				ensureGroup(parentID, seg, id)
			}
			parentID = id
		}
	}

	nodes := make([]catalog.Node, 0, len(order))
	for _, id := range order {
		nodes = append(nodes, byID[id])
	}
	return catalog.New(nodes)
}

func indexOf(haystack []string, needle string) (int, bool) {
	for i, s := range haystack {
		if s == needle {
			return i, true
		}
	}
	return -1, false
}
