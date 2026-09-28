package extractor

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

// TestDialectGoldens_Dockerfile (LB-Q-dockerfile, design §13.2 steps 10/12):
// holds the dockerfile tags dialect to its hand-written goldens — stage,
// ARG and ENV key definitions, and the naive same-file FROM-references-a-
// prior-stage reference collection (see tags.scm's header note on why
// COPY --from is deliberately not captured). Regenerate with the same
// -update flag TestTagsDialectParity uses; a regenerated golden is reviewed
// as a diff, never accepted blind.
func TestDialectGoldens_Dockerfile(t *testing.T) {
	e, err := NewTSExtractor("dockerfile")
	if err != nil {
		t.Fatalf("NewTSExtractor(dockerfile): %v", err)
	}
	defer e.Close()
	for _, fx := range dialectFixtures(t, "dockerfile") {
		got := extractGolden(t, e, fx)
		gp := goldenPathFor(fx)
		if *updateGoldens {
			if err := os.WriteFile(gp, marshalGolden(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		raw, err := os.ReadFile(gp)
		if err != nil {
			t.Fatalf("read golden: %v (run with -update to create)", err)
		}
		var want goldenFile
		if err := json.Unmarshal(raw, &want); err != nil {
			t.Fatalf("%s: %v", gp, err)
		}
		gb, wb := marshalGolden(got), marshalGolden(want)
		if !bytes.Equal(gb, wb) {
			t.Errorf("%s: golden mismatch\n--- got\n%s\n--- want\n%s", fx, gb, wb)
		}
	}
}

// TestDockerfileExtractor_SurvivesSyntaxError confirms the tags engine keeps
// extracting definitions after content the grammar cannot parse cleanly
// (design §13.2 step 10's required fixture) — the same "never fail the
// whole file" guarantee dockerextract.go's port retires.
func TestDockerfileExtractor_SurvivesSyntaxError(t *testing.T) {
	e, err := NewTSExtractor("dockerfile")
	if err != nil {
		t.Fatalf("NewTSExtractor(dockerfile): %v", err)
	}
	defer e.Close()
	src, err := os.ReadFile("dialects/dockerfile/testdata/syntax-error.dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := e.Extract("svc", "Dockerfile", src)
	if err != nil {
		t.Fatalf("Extract must not error on a recoverable syntax error: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes (a stage before the error, an ARG after it), got %d: %+v", len(nodes), nodes)
	}
}
