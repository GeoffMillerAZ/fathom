package lang_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// rowExtractor reads a file of "kind name" lines into one symbol per line,
// a reference per "uses" line and a fact per "set" line.
type rowExtractor struct{ calls int }

func (e *rowExtractor) Extract(_ context.Context, f lang.File) (lang.Extracted, error) {
	var out lang.Extracted
	for i, line := range strings.Split(string(f.Src), "\n") {
		w := strings.Fields(line)
		switch {
		case len(w) == 2 && w[0] == "uses":
			out.References = append(out.References, lang.Reference{From: "app.Main", To: w[1], Edge: "uses", Line: i + 1})
		case len(w) == 3 && w[0] == "set":
			out.Facts = append(out.Facts, lang.Fact{Entity: "Main", Key: w[1], Value: w[2], Line: i + 1})
		case len(w) == 2 && w[0] == "def":
			out.Symbols = append(out.Symbols, lang.Symbol{Name: w[1], Package: "app", Kind: "RowDef", Line: i + 1, EndLine: i + 1, Tier: "verified"})
		case len(w) == 2 && w[0] == "bad":
			out.Unresolved = append(out.Unresolved, lang.Unresolved{From: "app.Main", Edge: "uses", Raw: w[1], Reason: "dynamic", Line: i + 1})
		}
	}
	return out, nil
}

func rowMatch(id string) lang.Match {
	return lang.Match{ID: id, Kind: "tooling", Extensions: []string{".row"}, Kinds: map[string]string{"RowDef": "function"}}
}

func TestWithExtractorReadsAFile(t *testing.T) {
	r := newReader(t, lang.WithExtractor(rowMatch("row-spec"), &rowExtractor{}))
	res := extract(t, r, "a/b.row", "def Main\nuses Other\nset mode fast\nbad $x\n")
	if res.Dialect != "row-spec" || res.Skipped != "" || res.Level != lang.LevelUnmeasured {
		t.Fatalf("dialect %q skipped %q level %q", res.Dialect, res.Skipped, res.Level)
	}
	if len(res.Symbols) != 1 || res.Symbols[0].Key != "app.Main" || res.Symbols[0].Canonical != "function" || res.Symbols[0].Kind != "RowDef" {
		t.Fatalf("symbols: %+v", res.Symbols)
	}
	if len(res.References) != 1 || res.References[0].To != "Other" || len(res.Facts) != 1 || res.Facts[0].Value != "fast" || len(res.Unresolved) != 1 || res.Unresolved[0].Reason != "dynamic" {
		t.Fatalf("result: %+v", res)
	}
	var info lang.DialectInfo
	for _, d := range r.Dialects() {
		if d.ID == "row-spec" {
			info = d
		}
	}
	if info.Engine != "native" || info.Kind != "tooling" || info.Level != lang.LevelUnmeasured || len(info.Matches) != 1 || info.Matches[0] != ".row" {
		t.Fatalf("DialectInfo: %+v", info)
	}
	// A file it does not claim never reaches it.
	if got := extract(t, r, "a/b.txt", "def Main\n"); got.Dialect == "row-spec" {
		t.Fatalf("a .txt file reached the extractor: %+v", got)
	}
}

func TestWithExtractorSharesRoutingWithPacks(t *testing.T) {
	// A Go extractor and a pack dialect claim one glob: the lower id wins, as it does between two pack dialects.
	m := lang.Match{ID: "aaa-rows", Globs: []string{"**/*.widget.json"}, Kinds: map[string]string{"RowDef": "function"}}
	r := newReader(t, lang.WithDialects(os.DirFS(samplePack)), lang.WithExtractor(m, &rowExtractor{}))
	if res := extract(t, r, "ui/clock.widget.json", `{"name":"Clock"}`); res.Dialect != "aaa-rows" {
		t.Fatalf("read by %q, want aaa-rows (id order)", res.Dialect)
	}
	m.ID = "zzz-rows"
	r = newReader(t, lang.WithDialects(os.DirFS(samplePack)), lang.WithExtractor(m, &rowExtractor{}))
	if res := extract(t, r, "ui/clock.widget.json", `{"name":"Clock"}`); res.Dialect != "widget-spec" {
		t.Fatalf("read by %q, want widget-spec (id order)", res.Dialect)
	}
}

func TestWithExtractorRefusedWhole(t *testing.T) {
	with := func(f func(*lang.Match)) lang.Match {
		m := rowMatch("row-spec")
		f(&m)
		return m
	}
	cases := []struct {
		name   string
		m      lang.Match
		reason string
	}{
		{"id is a built-in dialect", with(func(m *lang.Match) { m.ID = "python" }), lang.PackConflict},
		{"id is the native Go dialect", with(func(m *lang.Match) { m.ID = "go" }), lang.PackConflict},
		{"id spelling", with(func(m *lang.Match) { m.ID = "Row_Spec" }), lang.PackManifest},
		{"extension a built-in dialect claims", with(func(m *lang.Match) { m.Extensions = []string{".py"} }), lang.PackConflict},
		{"extension the router reads generically", with(func(m *lang.Match) { m.Extensions = []string{".json"} }), lang.PackRoute},
		{"extension the router owns", with(func(m *lang.Match) { m.Extensions = []string{".go"} }), lang.PackRoute},
		{"basename a built-in dialect claims", with(func(m *lang.Match) { m.Extensions, m.Basenames = nil, []string{"package.json"} }), lang.PackConflict},
		{"claims nothing", with(func(m *lang.Match) { m.Extensions = nil }), lang.PackClaims},
		{"no kinds", with(func(m *lang.Match) { m.Kinds = nil }), lang.PackManifest},
		{"canonical outside the closed list", with(func(m *lang.Match) { m.Kinds = map[string]string{"RowDef": "gizmo"} }), lang.PackManifest},
		{"kind a built-in dialect emits", with(func(m *lang.Match) { m.Kinds = map[string]string{"TerraformResource": "resource"} }), lang.PackConflict},
		{"kind is not language or tooling", with(func(m *lang.Match) { m.Kind = "weird" }), lang.PackManifest},
		{"sniff that does not compile", with(func(m *lang.Match) { m.Sniff = &lang.Sniff{Extensions: []string{".row"}, Pattern: "("} }), lang.PackManifest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := lang.New(lang.WithExtractor(tc.m, &rowExtractor{}))
			if err == nil {
				r.Close()
				t.Fatal("New accepted the extractor")
			}
			if reason, dialect := packReason(t, err); reason != tc.reason || dialect != tc.m.ID {
				t.Fatalf("refused as (%s, %s), want (%s, %s): %v", reason, dialect, tc.reason, tc.m.ID, err)
			}
		})
	}
	if _, err := lang.New(lang.WithExtractor(rowMatch("row-spec"), nil)); err == nil {
		t.Error("a nil extractor was accepted")
	}
	// Two extractors that share an extension: the second is refused.
	if _, err := lang.New(lang.WithExtractor(rowMatch("row-a"), &rowExtractor{}), lang.WithExtractor(rowMatch("row-b"), &rowExtractor{})); err == nil {
		t.Error("two extractors for one extension were accepted")
	} else if reason, dialect := packReason(t, err); reason != lang.PackConflict || dialect != "row-b" {
		t.Errorf("refused as (%s, %s)", reason, dialect)
	}
	// A bad extractor refuses a good pack with it.
	if _, err := lang.New(lang.WithDialects(os.DirFS(samplePack)), lang.WithExtractor(rowMatch("python"), &rowExtractor{})); err == nil {
		t.Error("a bad extractor beside a good pack was accepted")
	}
}

type panicExtractor struct{}

func (panicExtractor) Extract(context.Context, lang.File) (lang.Extracted, error) {
	panic("boom")
}

type errExtractor struct{}

func (errExtractor) Extract(context.Context, lang.File) (lang.Extracted, error) {
	return lang.Extracted{Symbols: []lang.Symbol{{Name: "Half", Kind: "RowDef", Line: 1}}}, errors.New("half read")
}

func TestWithExtractorFailuresStayContained(t *testing.T) {
	r := newReader(t, lang.WithExtractor(rowMatch("row-spec"), panicExtractor{}))
	if _, err := r.Extract(context.Background(), lang.File{Path: "x.row", Src: []byte("def A")}); err == nil || !strings.Contains(err.Error(), "panicked") {
		t.Fatalf("a panic is not an error: %v", err)
	}
	// The Reader stays usable for every other file.
	if res := extract(t, r, "x.tf", tfSrc); res.Dialect != "terraform" {
		t.Fatalf("after a panic: %q", res.Dialect)
	}
	r = newReader(t, lang.WithExtractor(rowMatch("row-spec"), errExtractor{}))
	res := extract(t, r, "x.row", "")
	if len(res.Symbols) != 1 || len(res.Notes) != 1 || !strings.Contains(res.Notes[0], "half read") {
		t.Fatalf("a failing extractor's partial read: %+v", res)
	}
}

func TestWithExtractorConcurrentDeterministic(t *testing.T) {
	r := newReader(t, lang.WithExtractor(rowMatch("row-spec"), &rowExtractor{}))
	src := "def Z\ndef A\nuses B\nset k v\n"
	want := extract(t, r, "p/x.row", src)
	if want.Symbols[0].Name != "Z" || want.Symbols[1].Name != "A" {
		t.Fatalf("symbols are not in line order: %+v", want.Symbols)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := r.Extract(context.Background(), lang.File{Path: "p/x.row", Src: []byte(src)})
			if err != nil || fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("concurrent read: %v %v", err, got)
			}
		}()
	}
	wg.Wait()
}
