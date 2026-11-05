package fusion

// routing_parity_test.go — the routing-parity harness of the language library
// cuts (ADR-092, docs/design/lang-library.md §4, plan LL-M2). Skipped unless
// FATHOM_PARITY_OUT names a file. It runs every dialect fixture and the
// fauxtown corpus through both dispatches (ExtractForBench's resolveExtractor
// and ReingestFile's own order) and writes one sorted line per file and
// dispatch, "<dispatch>:<path> <sha256>", then DispatchTable() as JSON beside
// it ("<out>.dispatch.json"). A byte-identical file before and after a cut
// proves routing did not change. Nothing here records a Go package path, so a
// type that moves between packages does not move a line.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// parityLabel names the extractor ReingestFile picked without a package path:
// the tags dialect id for a TSExtractor, else the Go type's bare name.
func parityLabel(e extractor.Extractor) string {
	if t, ok := e.(*extractor.TSExtractor); ok {
		return "tags:" + t.Name()
	}
	t := reflect.TypeOf(e)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return "go:" + t.Name()
}

func parityDigest(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		b = []byte("marshal-error:" + err.Error())
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// parityRoots lists the corpus roots as (label, absolute dir): every dialect
// testdata directory and the fauxtown corpus, each its own repository root.
func parityRoots(t *testing.T) [][2]string {
	t.Helper()
	mod := filepath.Join("..", "..")
	var roots [][2]string
	dialects := filepath.Join(mod, "internal", "extractor", "dialects")
	entries, err := os.ReadDir(dialects)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		td := filepath.Join(dialects, e.Name(), "testdata")
		if st, err := os.Stat(td); err == nil && st.IsDir() {
			roots = append(roots, [2]string{"internal/extractor/dialects/" + e.Name() + "/testdata", td})
		}
	}
	roots = append(roots, [2]string{"internal/connector/testdata/fauxtown", filepath.Join(mod, "internal", "connector", "testdata", "fauxtown")})
	for i := range roots {
		abs, err := filepath.Abs(roots[i][1])
		if err != nil {
			t.Fatal(err)
		}
		roots[i][1] = abs
	}
	return roots
}

func TestRoutingParity(t *testing.T) {
	out := os.Getenv("FATHOM_PARITY_OUT")
	if out == "" {
		t.Skip("FATHOM_PARITY_OUT not set")
	}
	p := New(nil, nil, Config{})
	defer p.Close()

	var lines []string
	for _, root := range parityRoots(t) {
		label, dir := root[0], root[1]
		var rels []string
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !d.Type().IsRegular() {
				return nil
			}
			rel, rerr := filepath.Rel(dir, path)
			if rerr == nil {
				rels = append(rels, filepath.ToSlash(rel))
			}
			return nil
		})
		sort.Strings(rels)
		refs := make([]core.SourceRef, 0, len(rels))
		for _, rel := range rels {
			refs = append(refs, core.SourceRef{Service: label, Path: rel})
		}
		for _, rel := range rels {
			content, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
			if err != nil {
				t.Fatal(err)
			}
			full := label + "/" + rel

			ex, berr := p.ExtractForBench(label, refs, rel, content)
			errText := ""
			if berr != nil {
				errText = berr.Error()
			}
			lines = append(lines, "bench:"+full+" "+parityDigest([]any{ex.Nodes, ex.ToolKind, ex.Skipped, ex.TypeRefs, errText}))

			lines = append(lines, "reingest:"+full+" "+parityDigest(routingReingestRecord(p, dir, rel, content)))
		}
	}
	sort.Strings(lines)
	if err := os.WriteFile(out, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	table, err := json.MarshalIndent(DispatchTable(), "", " ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out+".dispatch.json", append(table, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// routingReingestRecord mirrors ReingestFile's gates and then its dispatch seam
// (reingestDispatch), recording the extractor it picks and what that
// extractor reads, with no store.
func routingReingestRecord(p *Pipeline, repoPath, rel string, content []byte) []any {
	if len(content) > MaxExtractSize {
		return []any{"oversize"}
	}
	if binaryExcluded(rel, content) {
		return []any{"binary"}
	}
	if gateDialectFile(rel, content).excluded != "" {
		return []any{"dialect-gate"}
	}
	e, ok := p.reingestDispatch(repoPath, rel, content)
	if !ok {
		return []any{"no-extractor"}
	}
	nodes, err := extractWithFacts(e, "parity", rel, content)
	errText := ""
	if err != nil {
		errText = err.Error()
	}
	return []any{parityLabel(e), nodes, errText}
}
