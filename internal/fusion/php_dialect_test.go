package fusion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// phpDialectCorpus is a small PHP repository: a namespaced class (the
// dialect's PhpNamespace symbol and its namespace Package, which the CGO
// phpextract.go never emitted) and a file of plain functions with no
// namespace.
func phpDialectCorpus() map[string]string {
	return map[string]string{
		"src/Http/Client.php": "<?php\nnamespace App\\Http;\n\nclass Client\n{\n    public function send(string $uri): string\n    {\n        return $this->build($uri);\n    }\n\n    private function build(string $uri): string\n    {\n        return trim($uri);\n    }\n}\n",
		"src/helpers.php":     "<?php\n\nfunction app_name(): string\n{\n    return 'demo';\n}\n",
	}
}

// TestPHPDialectIsWhatFusionDispatches (D1, LB-F183): lang-parity reads the
// PHP tags dialect at least as good as the CGO phpextract.go on guzzle and
// monolog (MERGE, docs/bench/readings/lang/parity/php.json), so fusion.New
// serves .php from the dialect, which names a file's namespace.
func TestPHPDialectIsWhatFusionDispatches(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	corpus := phpDialectCorpus()
	rel := "src/Http/Client.php"
	e, _, ok, unclassified := p.resolveExtractor(rel, []byte(corpus[rel]), buildSiblingIndex(nil))
	if !ok || unclassified {
		t.Fatalf("%s: ok=%v unclassified=%v", rel, ok, unclassified)
	}
	if _, isTS := e.(*extractor.TSExtractor); !isTS {
		t.Fatalf("%s dispatches to %T, want the PHP tags dialect", rel, e)
	}
	nodes, err := e.Extract("svc", rel, []byte(corpus[rel]))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, n := range nodes {
		got = append(got, string(n.Kind)+":"+n.Name+"@"+n.Package)
	}
	sort.Strings(got)
	have := map[string]bool{}
	for _, g := range got {
		have[g] = true
	}
	for _, want := range []string{`PhpNamespace:App\Http@App\Http`, `PhpClass:Client@App\Http`, `PhpMethod:Client.send@App\Http`} {
		if !have[want] {
			t.Errorf("%s: %s missing from %q (the PHP dialect's kinds, Package from the declared namespace)", rel, want, got)
		}
	}
}

// TestIngest_PHPDialect (the product-reach proof, LB-F183): an ingest stores
// the dialect's symbols for a .php file, the namespace symbol the CGO walker
// never emitted among them, and a namespace-less file's function.
func TestIngest_PHPDialect(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, phpDialectCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "phpx", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "phpx", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, o := range obs {
		got[o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol] = true
	}
	for _, want := range []string{`src/Http/Client.php|PhpNamespace|App\Http`, "src/Http/Client.php|PhpClass|Client", "src/Http/Client.php|PhpMethod|Client.send", "src/helpers.php|PhpFunction|app_name"} {
		if !got[want] {
			var have []string
			for k := range got {
				have = append(have, k)
			}
			sort.Strings(have)
			t.Errorf("%s not stored; have %q", want, have)
		}
	}
}

// TestIngest_ExtractorEpoch_PHPDialectDispatchForcesReExtraction pins epoch
// 49 (LB-F183; wave ld's one bump, ADR-048 D2: renumbered at its merge to
// origin/main's 48 + 1): a .php file fingerprinted by the CGO walker holds no
// PhpNamespace, so the bump "48"→"49" must re-read it through the dialect.
// The seeded fingerprints are salted with "48", the true prior epoch, so the
// test fails if the bump reverts (the XT-5 rule in epoch_test.go).
func TestIngest_ExtractorEpoch_PHPDialectDispatchForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	corpus := phpDialectCorpus()
	writeCorpus(t, dir, corpus)
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "epoch49-php", Path: dir}
	ctx := context.Background()
	for name := range corpus {
		old := sha256.Sum256(append([]byte(corpus[name]), []byte("\x00fathom-extractor-epoch:48")...))
		if err := st.SetFingerprint(ctx, svc.ID, name, hex.EncodeToString(old[:])); err != nil {
			t.Fatalf("SetFingerprint %s: %v", name, err)
		}
	}
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Skipped != 0 {
		t.Fatalf("%d file(s) skipped (a pre-epoch-49 fingerprint was honored): ExtractorEpoch was not bumped for the PHP dispatch switch", rep.Skipped)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	saw := false
	for _, o := range obs {
		saw = saw || (o.SourceRef == "src/Http/Client.php" && o.SourceKind == "PhpNamespace" && o.Symbol == `App\Http`)
	}
	if !saw {
		t.Error(`re-extraction did not surface the dialect's PhpNamespace App\Http in src/Http/Client.php`)
	}
}
