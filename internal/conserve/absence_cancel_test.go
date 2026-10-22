package conserve

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestAbsenceCancellationStopsParsing(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "a.go", "package a\n// Deprecated: use New.\nfunc Old() {}\n")
	writeGoFile(t, root, "b.go", "package a\nfunc New() {}\n")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	parsed, reads := 0, 0
	plane := deprecatedCallersPlane{parse: func(path string, body []byte) (*ast.File, error) {
		parsed++
		f, err := parser.ParseFile(token.NewFileSet(), path, body, parser.ParseComments)
		cancel()
		return f, err
	}}
	read := func(string) ([]byte, bool, error) { reads++; return nil, false, nil }
	rep := Run(ctx, Input{Root: root, ChangedFiles: []string{"a.go", "b.go"}, BaseFile: read, HeadFile: read}, []Plane{plane})
	if parsed != 1 || reads != 0 || rep.Planes[0].Verdict != Unproven || !strings.Contains(strings.Join(rep.Planes[0].Missing, " "), "canceled") {
		t.Fatalf("parsed=%d reads=%d report=%+v", parsed, reads, rep)
	}
}

func TestFindModulesStopsAtCanceledWalkEntry(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "go.mod", "module example.invalid/fixture\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := findModules(ctx, root); err != context.Canceled {
		t.Fatalf("canceled module walk = %v", err)
	}
}
