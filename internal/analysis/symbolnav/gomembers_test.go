package symbolnav_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/symbolnav"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// TestImplementations_GoInterfaceMembersAreNotImplementers: since K17 (wave
// lf2) goextract names an interface's own methods Iface.Method, so the
// method-set heuristic reads Go interfaces at all; an interface whose methods
// cover another's is never one of its implementations.
func TestImplementations_GoInterfaceMembersAreNotImplementers(t *testing.T) {
	const src = `package w

type Writer interface {
	Write(p []byte) (int, error)
	Close() error
}

type WriteCloser interface {
	Write(p []byte) (int, error)
	Close() error
	Flush() error
}

type File struct{ fd int }

func (f *File) Write(p []byte) (int, error) { return 0, nil }

func (f *File) Close() error { return nil }
`
	nodes, err := extractor.NewGoExtractor().Extract("svc", "w/w.go", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if got := symbolnav.Implementations(nodes, "Writer"); !reflect.DeepEqual(got, []string{"File"}) {
		t.Fatalf("Implementations(Writer) = %v, want [File]", got)
	}
}
