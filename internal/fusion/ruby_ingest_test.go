package fusion_test

// ruby_ingest_test.go — LB-Q-ruby: a .rb, .rake, .ru or Rakefile reaches the ruby dialect through
// discovery and dispatch, and what only the upgraded dialect does — a constant, an attr_* property, an
// alias, a constant reference as a call target and a require resolved to a file-tier edge — reaches the
// store.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngest_RubyReachesTheRubyDialect(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"lib/app.rb": `require_relative 'app/util'

module App
  VERSION = '1.0'

  class Service
    attr_reader :name
    alias label name

    def run
      Util.clean(name)
    end
  end
end
`,
		"lib/app/util.rb": `module Util
  def self.clean(s)
    s.strip
  end
end
`,
		"Rakefile":         "def release_tag\n  'v1'\nend\n",
		"tasks/build.rake": "def compile_all\nend\n",
		"config.ru":        "def build_app\nend\n",
	}
	for rel, src := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "rb-svc", Path: dir}
	pl := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 1})
	t.Cleanup(pl.Close)
	ctx := context.Background()
	if _, err := pl.Ingest(ctx, svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.Observation{}
	for _, o := range obs {
		got[o.Symbol] = o
	}
	for symbol, kind := range map[string]core.SourceKind{
		"App::VERSION":       "RubyConstant",
		"App::Service.name":  "RubyProperty",
		"App::Service.label": "RubyAlias",
		"App::Service.run":   core.KindRubyMethod,
		"Util.clean":         core.KindRubyMethod,
		"release_tag":        core.KindRubyMethod, // Rakefile
		"compile_all":        core.KindRubyMethod, // .rake
		"build_app":          core.KindRubyMethod, // .ru
	} {
		o, ok := got[symbol]
		if !ok {
			t.Errorf("no observation for %q (got %v)", symbol, keysOf(got))
			continue
		}
		if o.SourceKind != kind {
			t.Errorf("%s: kind %q, want %q", symbol, o.SourceKind, kind)
		}
	}
	if calls := strings.Join(got["App::Service.run"].Calls, ","); calls != "Util,clean" {
		t.Errorf("App::Service.run calls = %q, want Util,clean (a constant reference is a call target)", calls)
	}
	edges, err := st.TypedEdges(ctx, svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range edges {
		if e.Kind == core.EdgeFileResolved && e.FromFile == "lib/app.rb" && strings.HasSuffix(e.ToSymbol, "Util") {
			found = true
		}
	}
	if !found {
		t.Errorf("no file-resolved edge from lib/app.rb to Util (require_relative 'app/util'); edges %+v", edges)
	}
}
