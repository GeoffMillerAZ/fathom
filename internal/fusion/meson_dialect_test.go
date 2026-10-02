package fusion_test

// meson_dialect_test.go — LB-Q-meson (design §6, §13): subdir('dir') imports dir/meson.build through the
// relative-path strategy (index file meson.build), so a target or dependency object a file names that an
// imported file defines resolves to it; ingest dispatches meson.build, meson.options and meson_options.txt
// through the tags-engine loop (no hand-wired line: the manifest claims them by basename). A name a child
// directory uses from its parent has no import to bind it: Meson's variables are one namespace, an import
// graph is not (the measured ceiling, as for CMake).

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestResolveFileEdges_MesonSubdirs(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "mesonfixture"))
	if err != nil {
		t.Fatal(err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "meson", fixtureFiles(t, dir, "meson.build"))
	edges, report := fusion.ResolveFileEdges(context.Background(), "meson", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("skipped: %s", report.SkipReason)
	}
	want := []string{
		"meson.build|app -> src/meson.build|core",
		"meson.build|app -> src/meson.build|thr",
		"src/meson.build|core -> src/meson.build|thr",
	}
	got := namedEdges(edges, symbols)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("edges:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

func TestIngest_MesonDialectIsDispatched(t *testing.T) {
	dir := t.TempDir()
	for rel, src := range map[string]string{
		"meson.build":       "project('p', 'c')\nsubdir('src')\nexecutable('app', 'm.c')\n",
		"src/meson.build":   "lib = static_library('lib', 'l.c')\n",
		"meson.options":     "option('docs', type : 'boolean', value : false)\n",
		"meson_options.txt": "option('legacy', type : 'boolean', value : false)\n",
		"not-meson.build":   "executable('ignored', 'i.c')\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"meson.build", "a/b/meson.options", "meson_options.txt"} {
		if got := connector.LanguageForPath(p); got != "meson" {
			t.Errorf("LanguageForPath(%q) = %q, want meson", p, got)
		}
	}
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "meson-svc", Path: dir}
	p := fusion.New(connector.NewLocalConnector(), st, defaultCfg())
	t.Cleanup(p.Close)
	if _, err := p.Ingest(context.Background(), svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	obs, err := st.QueryObservations(context.Background(), svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, o := range obs {
		got = append(got, o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol)
	}
	slices.Sort(got)
	want := []string{
		"meson.build|MesonProject|p",
		"meson.build|MesonTarget|app",
		"meson.options|MesonOption|docs",
		"meson_options.txt|MesonOption|legacy",
		"src/meson.build|MesonTarget|lib",
		"src/meson.build|MesonVariable|lib",
	}
	slices.Sort(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("observations:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}
