package testlayer

import "testing"

func TestClassify_NonTestFileIsEmpty(t *testing.T) {
	if got := Classify("internal/widget/widget.go", nil); got != "" {
		t.Errorf("Classify(non-test) = %q, want \"\"", got)
	}
}

func TestClassify_PlainUnitTest(t *testing.T) {
	if got := Classify("internal/widget/widget_test.go", nil); got != "unit" {
		t.Errorf("Classify(plain _test.go) = %q, want unit", got)
	}
}

func TestClassify_BuildTagIntegration(t *testing.T) {
	if got := Classify("internal/widget/widget_test.go", []string{"integration"}); got != "integration" {
		t.Errorf("Classify(build tag integration) = %q, want integration", got)
	}
}

func TestClassify_BuildTagE2E(t *testing.T) {
	if got := Classify("internal/widget/widget_test.go", []string{"e2e"}); got != "e2e" {
		t.Errorf("Classify(build tag e2e) = %q, want e2e", got)
	}
}

func TestClassify_DeclaredDir(t *testing.T) {
	if got := Classify("internal/widget/integration/widget_test.go", nil); got != "integration" {
		t.Errorf("Classify(integration/ dir) = %q, want integration", got)
	}
	if got := Classify("e2e/smoke_test.go", nil); got != "e2e" {
		t.Errorf("Classify(e2e/ dir) = %q, want e2e", got)
	}
}

func TestClassify_DirSegmentMustMatchWhole(t *testing.T) {
	// "e2eish" must never match the "e2e" dir rule.
	if got := Classify("internal/e2eish/widget_test.go", nil); got != "unit" {
		t.Errorf("Classify(e2eish dir) = %q, want unit (no partial dir match)", got)
	}
}

func TestClassifyWith_CustomLayerGlob(t *testing.T) {
	spec := LayerSet{Layers: []LayerSpec{{Name: "contract", Globs: []string{"contract/*_test.go"}}}}
	if got := ClassifyWith(spec, "contract/api_test.go", nil); got != "contract" {
		t.Errorf("ClassifyWith(custom glob) = %q, want contract", got)
	}
	// A file outside the glob still falls to the unconditional "unit" base
	// case — ClassifyWith never merges in DefaultLayers' integration/e2e
	// rules alongside a caller-supplied spec.
	if got := ClassifyWith(spec, "internal/widget/integration/widget_test.go", nil); got != "unit" {
		t.Errorf("ClassifyWith(custom spec, non-matching path) = %q, want unit", got)
	}
}

// TestClassifyWith_DoubleStarGlobsMatch is the MINOR fix (finding 4):
// Classify/ClassifyWith matched globs with path.Match, which has no "**"
// — so the two "**"-shaped examples this package ships in
// schema/testlayer/testdata/valid.cue ("**/e2e/**", "contract/**") never
// matched anything. internal/pathglob.Match falls back to path.Match when
// a glob has no "**", so a plain glob's behavior is unchanged.
func TestClassifyWith_DoubleStarGlobsMatch(t *testing.T) {
	spec := LayerSet{Layers: []LayerSpec{
		{Name: "e2e", Globs: []string{"**/e2e/**"}},
		{Name: "contract", Globs: []string{"contract/**"}},
	}}
	if got := ClassifyWith(spec, "e2e/smoke_test.go", nil); got != "e2e" {
		t.Errorf("ClassifyWith(%q glob) = %q, want e2e", "**/e2e/**", got)
	}
	if got := ClassifyWith(spec, "contract/api_test.go", nil); got != "contract" {
		t.Errorf("ClassifyWith(%q glob) = %q, want contract", "contract/**", got)
	}
	if got := ClassifyWith(spec, "contract/nested/api_test.go", nil); got != "contract" {
		t.Errorf("ClassifyWith(%q glob) = %q, want contract (nested)", "contract/**", got)
	}
}

func TestClassify_PrecedenceBuildTagBeforeDir(t *testing.T) {
	// A file under an "e2e" dir but carrying the "integration" tag: build
	// tags are checked first per declared-layer order (DefaultLayers lists
	// integration before e2e), so the tag wins.
	if got := Classify("e2e/widget_test.go", []string{"integration"}); got != "integration" {
		t.Errorf("Classify(tag over dir) = %q, want integration", got)
	}
}
