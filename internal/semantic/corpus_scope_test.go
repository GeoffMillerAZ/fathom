package semantic_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestObligationsCorpusNeverWidensToIntent (EP-13): the obligations layer
// compiles BINDING documents into governed claims with an LLM. The intent
// corpus is served deterministically instead (ADR-026) — compiling rules
// that are already rules pays for no precision, and design essays produced
// noisy aspirational rules the first time the fleet compiled.
//
// This test exists because "we won't add it" is a promise a future edit can
// break silently; the corpus list is one line, and one line is exactly what
// drifts.
func TestObligationsCorpusNeverWidensToIntent(t *testing.T) {
	root := repoRootForCorpus(t)
	body, err := os.ReadFile(filepath.Join(root, "internal", "semantic", "layers", "obligations", "layer.cue"))
	if err != nil {
		t.Skipf("obligations layer.cue unreadable: %v", err)
	}
	src := string(body)

	var include string
	for _, ln := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "corpus: include:") {
			include = ln
			break
		}
	}
	if include == "" {
		t.Fatal("no `corpus: include:` line in the obligations layer — the pack shape changed")
	}
	for _, forbidden := range []string{"docs/intent", "docs/vision", "docs/design"} {
		if strings.Contains(include, forbidden) {
			t.Errorf("the obligations corpus must not include %q — it is served deterministically (ADR-026) "+
				"or is aspirational prose; the corpus line is: %s", forbidden, strings.TrimSpace(include))
		}
	}
	if !strings.Contains(src, "docs/intent/**") {
		t.Error("layer.cue should SAY why docs/intent is excluded, so the next author does not add it back")
	}
}

func repoRootForCorpus(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Skip("repo root not found")
	return ""
}
