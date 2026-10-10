package fusion

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func composerCorpus() map[string]string {
	return map[string]string{
		"composer.json":               "{\n  \"name\": \"acme/app\",\n  \"require\": {\"php\": \"^8.3\", \"acme/core\": \"^1\"},\n  \"scripts\": {\"test\": \"phpunit\", \"ci\": [\"@test\"]}\n}\n",
		"packages/core/composer.json": "{\n  \"name\": \"acme/core\",\n  \"description\": \"the core\"\n}\n",
		"data/not-composer.json":      "{\"name\": \"x\", \"scripts\": {\"a\": \"b\"}}\n",
	}
}

// TestIngest_ComposerDialect (LB-Q-composer): composer.json is read by
// basename through the composer dialect (an entity per package and script),
// another JSON file stays the generic JSON reading, a require names the
// package of another manifest of the repository, and a script's @name is an
// edge to that script.
func TestIngest_ComposerDialect(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, composerCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "cmp", Path: dir}); err != nil {
		t.Fatal(err)
	}
	got := crSourceKinds(t, st, "cmp")
	for _, rel := range []string{"composer.json", "packages/core/composer.json"} {
		if !crHasKind(got[rel], "ComposerPackage") {
			t.Errorf("%s read as %q, want ComposerPackage", rel, got[rel])
		}
	}
	if !crHasKind(got["composer.json"], "ComposerScript") {
		t.Errorf("composer.json scripts are no entities: %q", got["composer.json"])
	}
	if crHasKind(got["data/not-composer.json"], "ComposerPackage") || crHasKind(got["data/not-composer.json"], "ComposerScript") {
		t.Errorf("another JSON file read as Composer: %q", got["data/not-composer.json"])
	}
	edges := crEdges(t, st, "cmp")
	found := false
	for _, e := range edges {
		if strings.HasPrefix(e, "composer.json -> ") && strings.HasSuffix(e, "packages/core/composer.acme/core") {
			found = true
		}
	}
	if !found {
		t.Errorf("no require edge from composer.json to the acme/core manifest; edges: %q", edges)
	}
}

func TestReingestFile_ComposerParity(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, composerCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "cmp", Path: dir}); err != nil {
		t.Fatal(err)
	}
	before := crSourceKinds(t, st, "cmp")
	for rel := range composerCorpus() {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ReingestFile(ctx, st, "cmp", dir, rel); err != nil {
			t.Fatal(err)
		}
	}
	after := crSourceKinds(t, st, "cmp")
	if !reflect.DeepEqual(before, after) {
		t.Errorf("ReingestFile changed what a file reads as\nbefore: %v\n after: %v", before, after)
	}
}
