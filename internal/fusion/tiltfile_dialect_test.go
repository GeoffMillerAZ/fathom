package fusion_test

// tiltfile_dialect_test.go — LB-Q-tiltfile (design §13): a Tiltfile is classified by its basename and read by the
// tags engine on the Starlark grammar: a def is a function, and a Tilt API call that declares an entity names it by
// a literal string (local_resource, docker_build, k8s_yaml, config.define_*). Another file named Tiltfile.md or a
// Tiltfile's neighbours are not read as Tiltfiles.

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

func TestIngest_TiltfileDialectIsDispatched(t *testing.T) {
	dir := t.TempDir()
	for rel, src := range map[string]string{
		"Tiltfile": "config.define_string(\"env\")\n\ndef build(name):\n    docker_build(\"example/\" + name, \".\")\n\n" +
			"local_resource(\"lint\", cmd = \"make lint\")\ndocker_build(\"example/web\", \"./web\")\nk8s_yaml(\"deploy.yaml\")\n",
		"services/api/Tiltfile": "k8s_resource(\"api\", port_forwards = 8080)\n",
		"Tiltfile.md":           "local_resource(\"ignored\")\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"Tiltfile", "a/b/Tiltfile"} {
		if got := connector.LanguageForPath(p); got != "tiltfile" {
			t.Errorf("LanguageForPath(%q) = %q, want tiltfile", p, got)
		}
	}
	if got := connector.LanguageForPath("Tiltfile.md"); got == "tiltfile" {
		t.Errorf("LanguageForPath(Tiltfile.md) = tiltfile, want another language")
	}
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "tilt-svc", Path: dir}
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
		"Tiltfile|TiltFunction|build",
		"Tiltfile|TiltImage|example/web",
		"Tiltfile|TiltManifest|deploy.yaml",
		"Tiltfile|TiltResource|lint",
		"Tiltfile|TiltSetting|env",
		"services/api/Tiltfile|TiltResource|api",
	}
	var have []string
	for _, g := range got {
		if !strings.Contains(g, "TiltModule") {
			have = append(have, g)
		}
	}
	slices.Sort(want)
	if strings.Join(have, "\n") != strings.Join(want, "\n") {
		t.Errorf("observations:\n  %s\nwant:\n  %s", strings.Join(have, "\n  "), strings.Join(want, "\n  "))
	}
}
