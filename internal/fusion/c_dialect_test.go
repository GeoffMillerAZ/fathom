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

// cDialectCorpus is a small C repository: a source file, a C header (no C++
// signal) with a macro and a union only the C port emits, and a C++ header the
// router sends to the C++ extractor.
func cDialectCorpus() map[string]string {
	return map[string]string{
		"src/main.c":   "#include \"util.h\"\n\nint main(void) {\n  return helper(LIMIT);\n}\n",
		"src/util.h":   "#ifndef UTIL_H\n#define UTIL_H\n#define LIMIT 4\nunion value { int i; float f; };\nstatic inline int helper(int x) { return x + 1; }\n#endif\n",
		"src/widget.h": "namespace ui {\nclass Widget {\n public:\n  int size() const { return 1; }\n};\n}\n",
	}
}

// TestCDialectIsWhatFusionDispatches (D1, LB-F119): lang-parity reads the C
// port at least as good as the CGO cextract.go on libuv and curl (MERGE), so
// fusion.New serves .c from the tags dialect and the CT-3 header router sends
// a C header to it, a C++ header still to the C++ extractor.
func TestCDialectIsWhatFusionDispatches(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	corpus := cDialectCorpus()
	e, _, ok, unclassified := p.resolveExtractor("src/main.c", []byte(corpus["src/main.c"]), buildSiblingIndex(nil))
	if !ok || unclassified {
		t.Fatalf("src/main.c: ok=%v unclassified=%v", ok, unclassified)
	}
	if _, isTS := e.(*extractor.TSExtractor); !isTS {
		t.Errorf("src/main.c dispatches to %T, want the C tags dialect", e)
	}
	h, _, ok, _ := p.resolveExtractor("src/util.h", []byte(corpus["src/util.h"]), buildSiblingIndex(nil))
	if !ok {
		t.Fatal("src/util.h: no extractor")
	}
	if _, isRouter := h.(*extractor.CHeaderExtractor); !isRouter {
		t.Fatalf("src/util.h dispatches to %T, want the CT-3 header router", h)
	}
	kinds := func(rel string) []string {
		nodes, err := h.Extract("svc", rel, []byte(corpus[rel]))
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, n := range nodes {
			out = append(out, string(n.Kind)+":"+n.Name)
		}
		sort.Strings(out)
		return out
	}
	have := map[string]bool{}
	for _, k := range kinds("src/util.h") {
		have[k] = true
	}
	for _, want := range []string{"CMacro:LIMIT", "CUnion:value", "CFunc:helper"} {
		if !have[want] {
			t.Errorf("src/util.h through the router: %s missing from %v (the C port's kinds)", want, kinds("src/util.h"))
		}
	}
	have = map[string]bool{}
	for _, k := range kinds("src/widget.h") {
		have[k] = true
	}
	if !have["CppClass:Widget"] {
		t.Errorf("src/widget.h through the router: %v, want the C++ extractor's CppClass Widget", kinds("src/widget.h"))
	}
}

// TestIngest_CDialect (the product-reach proof, LB-F119): an ingest stores the
// C port's symbols for a .c file and a C header — a macro and a union the CGO
// walker never emitted — with main's calls.
func TestIngest_CDialect(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, cDialectCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "cx", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "cx", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, o := range obs {
		got[o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol] = true
	}
	for _, want := range []string{"src/util.h|CMacro|LIMIT", "src/util.h|CUnion|value", "src/util.h|CFunc|helper", "src/main.c|CFunc|main", "src/widget.h|CppClass|Widget"} {
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

// TestIngest_ExtractorEpoch_CDialectDispatchForcesReExtraction pins epoch 43
// (LB-F119): a .c file and a C header fingerprinted by the CGO walker hold no
// macro and no union, so the bump "42"→"43" must re-read them through the
// port. The seeded fingerprints are salted with "42", the true prior epoch, so
// the test fails if the bump reverts (the XT-5 rule in epoch_test.go).
func TestIngest_ExtractorEpoch_CDialectDispatchForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	corpus := cDialectCorpus()
	writeCorpus(t, dir, corpus)
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "epoch43-svc", Path: dir}
	ctx := context.Background()
	for _, name := range []string{"src/main.c", "src/util.h"} {
		old := sha256.Sum256(append([]byte(corpus[name]), []byte("\x00fathom-extractor-epoch:42")...))
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
		t.Fatalf("%d file(s) skipped (a pre-epoch-43 fingerprint was honored): ExtractorEpoch was not bumped for the C dispatch switch", rep.Skipped)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	saw := false
	for _, o := range obs {
		saw = saw || (o.SourceRef == "src/util.h" && o.SourceKind == "CMacro" && o.Symbol == "LIMIT")
	}
	if !saw {
		t.Error("re-extraction did not surface the port's CMacro LIMIT in src/util.h")
	}
}
