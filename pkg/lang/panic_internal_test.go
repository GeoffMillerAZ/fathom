package lang

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/langroute"
)

// panicExtractor is a built-in-slot extractor that panics, as Rego's did on an
// unclosed bracket.
type panicExtractor struct{}

func (panicExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	panic("index out of range [3] with length 2")
}

// A panic in a built-in extractor is that file's failure and not the caller's:
// Extract returns an Incomplete reading with one line naming it, and the next
// file in the batch reads. (Design rule 3; a stack overflow cannot be caught, so
// the recursive parsers refuse their deep inputs first.)
func TestAPanickingExtractorIsOneFilesFailure(t *testing.T) {
	r, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	router, err := langroute.NewRouter(langroute.WithCUE(panicExtractor{}))
	if err != nil {
		t.Fatal(err)
	}
	r.router.Close()
	r.router = router

	got, err := r.Extract(context.Background(), File{Path: "x.cue", Src: []byte("a: 1\n")})
	if err != nil {
		t.Fatalf("a panicking extractor ended the batch with an error: %v", err)
	}
	if !got.Incomplete || len(got.Notes) == 0 || !strings.Contains(got.Notes[0], "failed on this input") || strings.Contains(got.Notes[0], "\n") {
		t.Fatalf("incomplete=%v notes=%q", got.Incomplete, got.Notes)
	}
	if got.Dialect == "" {
		t.Errorf("a read file has a dialect: %+v", got)
	}
	// The batch goes on.
	next, err := r.Extract(context.Background(), File{Path: "a.go", Src: []byte("package a\n\nfunc F() {}\n")})
	if err != nil || next.Incomplete || len(next.Symbols) == 0 {
		t.Fatalf("the next file: err=%v incomplete=%v symbols=%d", err, next.Incomplete, len(next.Symbols))
	}
}
