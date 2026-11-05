package lang

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// Dialect Engine says what reads the dialect (types.go): for every dialect the
// router dispatches by an extension or a basename to one kind of extractor, the
// listed Engine is that kind, whatever the manifest declares. CMake and proto
// declare "tags" and are read by Go readers; a test over the table keeps the next
// one from reading as a grammar when it is not.
func TestDialectEngineIsWhatReadsIt(t *testing.T) {
	r, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ds, err := extractor.LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	engine := map[string]string{}
	for _, d := range r.Dialects() {
		engine[d.ID] = d.Engine
	}
	exts := r.router.Exts()
	checked := 0
	for _, d := range ds {
		want := ""
		keys := append([]string(nil), d.Match.Basenames...)
		for _, e := range d.Match.Extensions {
			keys = append(keys, strings.ToLower(e))
		}
		mixed := false
		for _, k := range keys {
			e, ok := exts[k]
			if !ok {
				continue
			}
			class := "native"
			switch e.(type) {
			case *extractor.TSExtractor:
				class = "tags"
			case *extractor.TagsDocExtractor:
				class = "tags+doc"
			case *extractor.DocExtractor:
				class = "doc"
			}
			if want != "" && want != class {
				mixed = true
			}
			want = class
		}
		got, listed := engine[d.ID]
		if want == "" || mixed || !listed {
			continue
		}
		checked++
		if got != want {
			t.Errorf("dialect %s is listed with engine %q but the router dispatches it to a %s reader (manifest: %q)", d.ID, got, want, d.Engine)
		}
	}
	if checked < 80 {
		t.Errorf("only %d dialects were checked; the dispatch table moved", checked)
	}
	for id, want := range map[string]string{"cmake": "native", "protobuf": "native", "sql": "tags", "python": "tags", "terraform": "native"} {
		if engine[id] != want {
			t.Errorf("engine of %s = %q, want %q", id, engine[id], want)
		}
	}
}
