package mcp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/repofacts"
)

// rfm1Queries are RF-M1's pre-registered flag-drift queries
// (docs/bench/readings/repository-facts/roles.md).
var rfm1Queries = []string{"func ", "class ", "import ", "TODO", "test", "error", "return", "config", "assert", "New"}

// TestRFM1RoleReading is RF-M1's harness, not a gate: skipped unless
// FATHOM_RFM1_SUBJECT names a subject checkout. It writes, under
// FATHOM_RFM1_OUT, files.tsv (every indexed file: its full role and basis as
// RF-4 would have served it, rfm1Role below, and its convention test role)
// and flags.tsv (every hit of the pre-registered queries: its search flags
// before RF-2, frozen below, and after). roles-score.py beside the reading
// joins them with the oracle.
func TestRFM1RoleReading(t *testing.T) {
	root := os.Getenv("FATHOM_RFM1_SUBJECT")
	out := os.Getenv("FATHOM_RFM1_OUT")
	if root == "" || out == "" {
		t.Skip("RF-M1 harness: set FATHOM_RFM1_SUBJECT and FATHOM_RFM1_OUT")
	}
	ctx := context.Background()
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	refs, err := connector.NewLocalConnector().Discover(ctx, core.ServiceRef{ID: "rfm1", Path: root})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	var rels []string
	for _, r := range refs {
		p := r.Path
		if filepath.IsAbs(p) {
			if rp, err := filepath.Rel(root, p); err == nil {
				p = rp
			}
		}
		rels = append(rels, filepath.ToSlash(p))
	}
	sort.Strings(rels)
	files := rfm1Writer(t, filepath.Join(out, "files.tsv"))
	fmt.Fprintln(files.w, "path\trole\tbasis\tconvention_test")
	for _, rel := range rels {
		fact := rfm1Role(root, rel)
		conv := 0
		if repofacts.IsTest(rel) {
			conv = 1
		}
		fmt.Fprintf(files.w, "%s\t%s\t%s\t%d\n", rel, fact.Role, fact.Basis, conv)
	}
	files.close(t)

	ignore, pats, _, err := connector.LoadIgnoreScopedForWithPatterns(root, "rfm1")
	if err != nil {
		t.Fatalf("exclusions: %v", err)
	}
	flags := rfm1Writer(t, filepath.Join(out, "flags.tsv"))
	fmt.Fprintln(flags.w, "query\tserved\tpath\tline\tbefore\tafter")
	for _, q := range rfm1Queries {
		hits, _, capped, err := searchRun(ctx, root, searchOpts{pattern: q, caseSensitive: true, limit: 200000, ignore: ignore, ignorePatterns: pats})
		if err != nil {
			t.Fatalf("search %q: %v", q, err)
		}
		if capped != "" {
			t.Logf("query %q: engine bound %s (hits are a lower bound)", q, capped)
		}
		sort.Slice(hits, func(i, j int) bool {
			if hits[i].File != hits[j].File {
				return hits[i].File < hits[j].File
			}
			return hits[i].Line < hits[j].Line
		})
		for i, h := range hits {
			head := rfm1Head(root, h.File)
			served := 0
			if i < searchDefaultLimit {
				served = 1
			}
			fmt.Fprintf(flags.w, "%s\t%d\t%s\t%d\t%s\t%s\n", q, served, h.File, h.Line,
				strings.Join(preRF2SearchFlags(h.File, head), ","), strings.Join(searchComputeFlags(root, h.File), ","))
		}
	}
	flags.close(t)
}

// rfm1DialectRules is every embedded dialect manifest's match block, the
// dialect basis' input (fusion's dialect gate builds the same).
var rfm1DialectRules = sync.OnceValue(func() []connector.DialectRule {
	ds, err := extractor.LoadDialects()
	if err != nil {
		return nil
	}
	var rules []connector.DialectRule
	for _, d := range ds {
		r := connector.DialectRule{ID: d.ID, Basenames: d.Match.Basenames, Globs: d.Match.Globs, Shebangs: d.Match.Shebangs}
		r.Extensions = append(r.Extensions, d.Match.Extensions...)
		for _, v := range d.Variants {
			r.Extensions = append(r.Extensions, v.Extensions...)
		}
		if sn := d.Match.Sniff; sn != nil {
			r.Sniff = &connector.DialectSniffRule{Extensions: sn.Extensions, Pattern: sn.Pattern, Priority: sn.Priority}
		}
		rules = append(rules, r)
	}
	return rules
})

// rfm1Head reads up to repofacts.HeaderBytes of root/rel.
func rfm1Head(root, rel string) []byte {
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return nil
	}
	defer f.Close()
	buf := make([]byte, repofacts.HeaderBytes)
	n, _ := io.ReadFull(f, buf)
	return buf[:n]
}

// rfm1Role is the full role RF-4 would have served for rel (its commit
// 75ccae3b, refused by this reading's CUT): the dialect by the manifests'
// match rules over path and head (else connector's language tag), the
// .gitattributes chain from root, and the head, through repofacts.Role.
func rfm1Role(root, rel string) repofacts.Fact {
	head := rfm1Head(root, rel)
	dialect := connector.ClassifyDialect(rfm1DialectRules(), rel, head).Dialect
	if dialect == "" {
		if lang := connector.LanguageForPath(rel); lang != core.LangUnclassified {
			dialect = lang
		}
	}
	return repofacts.Role(rel, dialect, repofacts.ReadAttributes(os.DirFS(root), rel), head)
}

type rfm1File struct {
	f *os.File
	w *bufio.Writer
}

func rfm1Writer(t *testing.T, p string) rfm1File {
	t.Helper()
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	return rfm1File{f: f, w: bufio.NewWriter(f)}
}

func (r rfm1File) close(t *testing.T) {
	t.Helper()
	if err := r.w.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := r.f.Close(); err != nil {
		t.Fatal(err)
	}
}

// preRF2SearchFlags is search's flag rule as it stood before RF-2
// (search_tool.go at 50783574: isTestPath, the generated rule, and
// searchVendorSegments), frozen here as RF-M1's "before" side. It is the
// reading's baseline, never a product rule.
func preRF2SearchFlags(rel string, head []byte) []string {
	var flags []string
	base := rel
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		base = rel[i+1:]
	}
	isTest := strings.HasSuffix(base, "_test.go") || strings.HasSuffix(base, "_test.py") ||
		strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py") || strings.HasSuffix(base, "_spec.rb") ||
		strings.Contains(base, ".test.") || strings.Contains(base, ".spec.")
	if !isTest {
		for _, seg := range strings.Split(rel, "/") {
			if seg == "test" || seg == "tests" || seg == "testdata" {
				isTest = true
			}
		}
	}
	if isTest {
		flags = append(flags, "test")
	}
	generated := hasPathSegment(rel, "generated")
	if !generated && len(head) > 0 && connector.IsGenerated(rel, head) {
		generated = true
	}
	if generated {
		flags = append(flags, "generated")
	}
	for _, seg := range []string{"vendor", "node_modules", "third_party"} {
		if hasPathSegment(rel, seg) {
			flags = append(flags, "vendored")
			break
		}
	}
	return flags
}
