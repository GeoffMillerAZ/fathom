package receipt

// build_test.go — EVX-7 (docs/plans/portable-receipts.md; vision 43 D4,
// D11, section 6.2): BuildReceipt is the one allowlist constructor for an
// exported receipt. It writes the existing core.Receipt JSON, only the
// fields its kind carries, plus anchored facts; an excerpt is bounded to
// MaxExcerptLines; --no-excerpts keeps digests only; an absence fact
// carries its read set in the kit's CoverageManifest shape, and a missing
// read makes its count a named lower bound (I38).

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	readmanifest "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/read-manifest"
)

// leakShaped are values that must never leave through a field the
// allowlist does not name: a key shape, an absolute path, an address, a
// host name.
var leakShaped = []string{
	"sk-test-FAKESECRETVALUEnotreal0000",
	"/Users/someone/private/billing.go",
	"someone@example.invalid",
	"build-host.example.invalid",
}

func numberedLines(n int) []byte {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return []byte(b.String())
}

func committedAnchor(path string, start, end int, content []byte) CitedAnchor {
	digest, ok := excerptDigest(content, start, end)
	if !ok {
		panic("fixture span outside its content")
	}
	ex, _ := excerptOf(content, start, end)
	return CitedAnchor{Anchor: Anchor{
		Commit: strings.Repeat("a", 40), ObjectFormat: "sha1", Path: path, StartLine: start, EndLine: end,
		BlobID: strings.Repeat("b", 40), ExcerptSHA256: digest, FathomFingerprint: fusion.FingerprintHash(content),
		State: StateCommitted, Clean: true,
	}, Excerpt: ex}
}

func marshalExported(t *testing.T, r ExportedReceipt) []byte {
	t.Helper()
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func topLevelKeys(t *testing.T, data []byte) []string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TestBuildReceiptAllowlist: a value set on a field the receipt's kind does
// not carry, a withheld anchor's free-text reason and bytes it holds never
// reach the output; the top-level keys are exactly the kind's allowlist.
func TestBuildReceiptAllowlist(t *testing.T) {
	content := numberedLines(5)
	withheld := committedAnchor("secret.yaml", 1, 2, []byte(leakShaped[0]+"\n"+leakShaped[1]+"\n"))
	withheld.Anchor.State, withheld.Anchor.Reason = StateWithheld, "suppressed ("+leakShaped[2]+"): no excerpt and no digest leave"
	in := ExportInput{
		Receipt: core.Receipt{
			Kind: core.ReceiptKindSymbolLocation, Tool: "find_symbol", Service: "svc", Symbol: "Greet",
			IndexCommit: strings.Repeat("a", 40), File: "a.go", Line: 3,
			// None of these is a symbol_location field.
			Callers: []string{leakShaped[0]}, Files: []string{leakShaped[1]}, Test: leakShaped[2],
			Target: leakShaped[3], Strength: leakShaped[0], Reached: []string{leakShaped[1]}, RunDigest: leakShaped[2],
		},
		Facts: []FactInput{
			{Kind: FactDefinition, Subject: "Greet", Anchors: []CitedAnchor{committedAnchor("a.go", 3, 4, content), withheld}},
		},
	}
	got, err := BuildReceipt(in)
	if err != nil {
		t.Fatal(err)
	}
	data := marshalExported(t, got)
	for _, s := range leakShaped {
		if strings.Contains(string(data), s) {
			t.Errorf("exported receipt carries %q, which no allowlisted field holds:\n%s", s, data)
		}
	}
	want := []string{"facts", "file", "index_commit", "kind", "line", "service", "symbol", "tool"}
	if keys := topLevelKeys(t, data); !reflect.DeepEqual(keys, want) {
		t.Errorf("top-level keys = %v, want exactly the symbol_location allowlist %v", keys, want)
	}
	w := got.Facts[0].Anchors[1]
	if w.Excerpt != "" || w.ExcerptSHA256 != "" || w.BlobID != "" || w.FathomFingerprint != "" {
		t.Errorf("a withheld anchor carries bytes or digests: %+v", w)
	}

	// The existing reader takes the exported bytes as a core.Receipt.
	var back core.Receipt
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.Kind != core.ReceiptKindSymbolLocation || back.File != "a.go" || back.Line != 3 || back.Symbol != "Greet" {
		t.Errorf("core.Receipt read back = %+v", back)
	}

	if _, err := BuildReceipt(ExportInput{Receipt: core.Receipt{Kind: "judgment", Symbol: "X"}}); err == nil {
		t.Error("a receipt kind outside the closed set builds")
	}
	bad := in
	bad.Facts = []FactInput{{Kind: "opinion", Subject: "Greet"}}
	if _, err := BuildReceipt(bad); err == nil {
		t.Error("a fact kind outside the closed set builds")
	}
}

// TestBuildReceiptBoundsExcerpts: an excerpt holds at most MaxExcerptLines
// lines and names its last line when it is cut; the digest still covers the
// whole span.
func TestBuildReceiptBoundsExcerpts(t *testing.T) {
	content := numberedLines(100)
	long := committedAnchor("a.go", 1, 100, content)
	short := committedAnchor("a.go", 10, 12, content)
	got, err := BuildReceipt(ExportInput{
		Receipt: core.Receipt{Kind: core.ReceiptKindSymbolLocation, Symbol: "Greet", IndexCommit: long.Anchor.Commit, File: "a.go", Line: 1},
		Facts:   []FactInput{{Kind: FactDefinition, Subject: "Greet", Anchors: []CitedAnchor{long, short}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	a, b := got.Facts[0].Anchors[0], got.Facts[0].Anchors[1]
	if n := strings.Count(a.Excerpt, "\n"); n != MaxExcerptLines {
		t.Errorf("a 100-line span's excerpt holds %d lines, want %d", n, MaxExcerptLines)
	}
	if !strings.HasPrefix(a.Excerpt, "line 1\n") || !strings.HasSuffix(a.Excerpt, "line 40\n") {
		t.Errorf("excerpt is not the span's first %d lines: %q…", MaxExcerptLines, a.Excerpt[:20])
	}
	if a.ExcerptEndLine != MaxExcerptLines {
		t.Errorf("a cut excerpt's excerpt_end_line = %d, want %d", a.ExcerptEndLine, MaxExcerptLines)
	}
	if a.ExcerptSHA256 != long.Anchor.ExcerptSHA256 {
		t.Error("cutting the excerpt changed the span's digest")
	}
	if b.Excerpt != "line 10\nline 11\nline 12\n" || b.ExcerptEndLine != 0 {
		t.Errorf("a 3-line excerpt = %q (end %d), want the whole span and no end line", b.Excerpt, b.ExcerptEndLine)
	}
}

// TestBuildReceiptNoExcerpts: NoExcerpts (the CLI's --no-excerpts) drops
// every excerpt and keeps the blob id, the excerpt digest and the file's
// fathom fingerprint.
func TestBuildReceiptNoExcerpts(t *testing.T) {
	content := numberedLines(5)
	an := committedAnchor("a.go", 2, 3, content)
	got, err := BuildReceipt(ExportInput{
		Receipt:    core.Receipt{Kind: core.ReceiptKindSymbolLocation, Symbol: "Greet", IndexCommit: an.Anchor.Commit, File: "a.go", Line: 2},
		Facts:      []FactInput{{Kind: FactDefinition, Subject: "Greet", Anchors: []CitedAnchor{an}}},
		NoExcerpts: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	data := string(marshalExported(t, got))
	if strings.Contains(data, `"excerpt"`) || strings.Contains(data, "line 2") {
		t.Errorf("--no-excerpts output carries an excerpt: %s", data)
	}
	for _, want := range []string{`"excerpt_sha256":"` + an.Anchor.ExcerptSHA256 + `"`, `"blob_id":"` + an.Anchor.BlobID + `"`, `"fathom_fingerprint":"` + an.Anchor.FathomFingerprint + `"`} {
		if !strings.Contains(data, want) {
			t.Errorf("--no-excerpts output lacks %s: %s", want, data)
		}
	}
}

// TestBuildReceiptAbsenceLowerBound: an absence fact's read set is the
// kit's CoverageManifest (read files, skips by category, the tree, the
// tool); a missing read is named and makes the count a lower bound, a
// complete read set a bare count.
func TestBuildReceiptAbsenceLowerBound(t *testing.T) {
	tree := strings.Repeat("c", 40)
	set := readmanifest.NewSet()
	set.Record("b.go", readmanifest.Read(10))
	set.Record("a.go", readmanifest.Read(10))
	set.Record("gone.go", readmanifest.Absent())
	set.Record("locked.go", readmanifest.Unreadable(fs.ErrPermission))
	set.Skip("logo.png", "binary")
	set.Skip("vendor/x.go", "ignored")
	partial := set.Manifest()
	base := core.Receipt{Kind: core.ReceiptKindCallers, Symbol: "Lonely", IndexCommit: strings.Repeat("a", 40)}
	absence := func(m readmanifest.Manifest) FactInput {
		return FactInput{Kind: FactAbsence, Subject: "Lonely", Reads: &m, TreeHash: tree, Tool: "fathom/find_callers", Noun: "caller(s)"}
	}

	got, err := BuildReceipt(ExportInput{Receipt: base, Facts: []FactInput{absence(partial)}})
	if err != nil {
		t.Fatal(err)
	}
	f := got.Facts[0]
	if f.ReadSet == nil {
		t.Fatalf("an absence fact carries no read set: %+v", f)
	}
	if !reflect.DeepEqual(f.ReadSet.Files, []string{"a.go", "b.go"}) || f.ReadSet.TreeHash != tree || f.ReadSet.Tool != "fathom/find_callers" {
		t.Errorf("read set = %+v, want files [a.go b.go], the tree and the tool", f.ReadSet)
	}
	if !reflect.DeepEqual(f.ReadSet.Skipped.Binary, []string{"logo.png"}) || !reflect.DeepEqual(f.ReadSet.Skipped.Ignored, []string{"vendor/x.go"}) {
		t.Errorf("skips = %+v, want binary [logo.png] and ignored [vendor/x.go]", f.ReadSet.Skipped)
	}
	if !f.LowerBound || !strings.HasPrefix(f.Count, "≥ 0 caller(s) (") || !strings.Contains(f.Count, "locked.go") {
		t.Errorf("a missing read renders count %q (lower_bound %v), want a named lower bound naming locked.go", f.Count, f.LowerBound)
	}
	// The unreadable file and the binary skip (it could hold a match) are
	// missing; the ignored path is out of scope and the absent one is not
	// there to read.
	want := []MissingRead{{Path: "locked.go", Detail: "permission denied"}, {Path: "logo.png", Detail: "skipped (binary)"}}
	if !reflect.DeepEqual(f.MissingReads, want) {
		t.Errorf("missing reads = %+v, want %+v", f.MissingReads, want)
	}

	// A binary skip alone could hide the symbol: still a lower bound.
	skipOnly := readmanifest.NewSet()
	skipOnly.Record("a.go", readmanifest.Read(1))
	skipOnly.Skip("logo.png", "binary")
	got, err = BuildReceipt(ExportInput{Receipt: base, Facts: []FactInput{absence(skipOnly.Manifest())}})
	if err != nil {
		t.Fatal(err)
	}
	if f := got.Facts[0]; !f.LowerBound || !strings.HasPrefix(f.Count, "≥ 0") || !strings.Contains(f.Count, "logo.png") {
		t.Errorf("an unexplained skip renders count %q (lower_bound %v), want a named lower bound", f.Count, f.LowerBound)
	}

	whole := readmanifest.NewSet()
	whole.Record("a.go", readmanifest.Read(1))
	got, err = BuildReceipt(ExportInput{Receipt: base, Facts: []FactInput{absence(whole.Manifest())}})
	if err != nil {
		t.Fatal(err)
	}
	if f := got.Facts[0]; f.LowerBound || f.Count != "0 caller(s)" || len(f.MissingReads) != 0 {
		t.Errorf("a complete read set renders %q (lower_bound %v, missing %v), want the bare count", f.Count, f.LowerBound, f.MissingReads)
	}

	if _, err := BuildReceipt(ExportInput{Receipt: base, Facts: []FactInput{{Kind: FactAbsence, Subject: "Lonely"}}}); err == nil {
		t.Error("an absence fact with no read set builds")
	}
}

// TestBuildReceiptFromAnchorAt: an anchor AnchorAt gives carries the
// checkout's bytes to BuildReceipt, so the excerpt is the cited lines; two
// spans of one file carry the file's one fathom fingerprint.
func TestBuildReceiptFromAnchorAt(t *testing.T) {
	r := newFixtureRepo(t)
	content := numberedLines(50)
	r.write("pkg/a.go", string(content))
	r.commitAll("a")
	c := mustResolve(t, r.dir, "")
	fp := fusion.FingerprintHash(content)
	var anchors []CitedAnchor
	for _, s := range []Span{{Path: "pkg/a.go", StartLine: 1, EndLine: 50}, {Path: "pkg/a.go", StartLine: 7, EndLine: 8}} {
		a, ex, err := AnchorExcerpt(context.Background(), c, s, Source{Fingerprint: fp})
		if err != nil {
			t.Fatal(err)
		}
		anchors = append(anchors, CitedAnchor{Anchor: a, Excerpt: ex})
	}
	got, err := BuildReceipt(ExportInput{
		Receipt: core.Receipt{Kind: core.ReceiptKindSymbolLocation, Symbol: "A", IndexCommit: c.ID, File: "pkg/a.go", Line: 1},
		Facts:   []FactInput{{Kind: FactDefinition, Subject: "A", Anchors: anchors}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ex := got.Facts[0].Anchors
	if ex[0].Excerpt != string(numberedLines(MaxExcerptLines)) {
		t.Errorf("excerpt of lines 1-50 = %q…, want lines 1-%d", ex[0].Excerpt[:20], MaxExcerptLines)
	}
	if ex[1].Excerpt != "line 7\nline 8\n" {
		t.Errorf("excerpt of lines 7-8 = %q", ex[1].Excerpt)
	}
	if ex[0].FathomFingerprint != fp || ex[1].FathomFingerprint != fp {
		t.Errorf("per-file fathom_fingerprint = %q, %q, want the file's %q", ex[0].FathomFingerprint, ex[1].FathomFingerprint, fp)
	}
}

// excerptDigest and excerptOf keep the fixtures above on the package's own
// formula.
func excerptDigest(content []byte, start, end int) (string, bool) {
	b, ok := excerptOf(content, start, end)
	if !ok {
		return "", false
	}
	return sha256Hex(b), true
}

func excerptOf(content []byte, start, end int) ([]byte, bool) {
	lines := strings.SplitAfter(string(content), "\n")
	if start < 1 || end > len(lines) || end < start {
		return nil, false
	}
	return []byte(strings.Join(lines[start-1:end], "")), true
}
