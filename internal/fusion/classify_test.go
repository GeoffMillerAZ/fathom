package fusion

// classify_test.go — white-box (package fusion, not fusion_test) unit tests
// for the image↔Dockerfile cross-ref
// (ADR-021 §2 / TI-1, TI-7). Full-pipeline behavior (dispatch actually wired
// into extractAndStore) is covered by toolident_test.go.

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestCrossRefImageToDockerfile_NilIndex(t *testing.T) {
	var idx *siblingIndex
	if got := crossRefImageToDockerfile("svc", "f.yaml", nil, idx); got != nil {
		t.Errorf("crossRefImageToDockerfile with nil index should return nil, got %v", got)
	}
}

// ─── cross-refs ─────────────────────────────────────────────────────────────

func TestImageBaseName(t *testing.T) {
	cases := map[string]string{
		"myapp":                             "myapp",
		"myapp:1.2.3":                       "myapp",
		"ghcr.io/org/myapp:1.2.3":           "myapp",
		"myapp@sha256:deadbeef":             "myapp",
		"registry.example.com:5000/x/myapp": "myapp",
		"":                                  "",
	}
	for in, want := range cases {
		if got := imageBaseName(in); got != want {
			t.Errorf("imageBaseName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCrossRefImageToDockerfile_MatchAndNoMatch(t *testing.T) {
	idx := buildSiblingIndex([]core.SourceRef{{Path: "myapp/Dockerfile"}, {Path: "other/Dockerfile"}})
	nodes := []core.SymbolNode{
		{Name: "myapp:1.0", Kind: core.KindK8sContainer, Line: 5},
		{Name: "unrelated:1.0", Kind: core.KindK8sContainer, Line: 6},
	}
	refs := crossRefImageToDockerfile("svc", "k8s/deploy.yaml", nodes, idx)
	if len(refs) != 1 {
		t.Fatalf("expected 1 cross-ref, got %d: %+v", len(refs), refs)
	}
	if refs[0].Name != "myapp" || refs[0].Kind != core.KindImageDockerfileRef {
		t.Errorf("unexpected cross-ref: %+v", refs[0])
	}
	if refs[0].Tier != core.TierPreScan {
		t.Errorf("cross-ref Tier = %q, want TierPreScan", refs[0].Tier)
	}
}

func TestCrossRefImageToDockerfile_StemBeatsDirName(t *testing.T) {
	// A *.dockerfile stem match wins over a bare Dockerfile's dir-name
	// match for the same image name.
	idx := buildSiblingIndex([]core.SourceRef{
		{Path: "build/myapp.dockerfile"},
		{Path: "myapp/Dockerfile"},
	})
	nodes := []core.SymbolNode{{Name: "myapp:1.0", Kind: core.KindK8sContainer, Line: 3}}
	refs := crossRefImageToDockerfile("svc", "k8s/deploy.yaml", nodes, idx)
	if len(refs) != 1 {
		t.Fatalf("expected 1 cross-ref, got %d: %+v", len(refs), refs)
	}
	if refs[0].Signature != "dockerfile: build/myapp.dockerfile" {
		t.Errorf("Signature = %q, want the stem-matched *.dockerfile to win", refs[0].Signature)
	}
}

func TestCrossRefImageToDockerfile_ComposeBuildOnlySkipped(t *testing.T) {
	idx := buildSiblingIndex([]core.SourceRef{{Path: "web/Dockerfile"}})
	nodes := []core.SymbolNode{
		{Name: "web", Kind: core.KindComposeService, Signature: "build: ./web"},
	}
	if refs := crossRefImageToDockerfile("svc", "compose.yml", nodes, idx); len(refs) != 0 {
		t.Errorf("a build-only compose service (no image:) should not cross-ref, got %+v", refs)
	}
}
