package testevidence

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
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// This is a synthetic producer/SCIP fixture. The syntax observations, including
// the display truncation and extraction fingerprint, come from actual fusion
// ingest. The source has two declared variants, matching the native XML cohort.
func unityLongAttributeFixture(t *testing.T, suffix string) (UnityEvidence, []core.SymbolNode) {
	t.Helper()
	f := newUnityFixture(t)
	sourcePath := f.r.Project + "/Assets/BowlTests.cs"
	attributes := "[TestCase(1)]\n[TestCase(2)]\n" + `[Category("` + strings.Repeat("category", 45) + `")]` + "\n" + suffix
	// Use normal authored lines. A single line plus the qualified policy would
	// exceed the connector's existing minified-file average-line boundary.
	source := []byte("namespace Workshop {\npublic class BowlTests {\n" + attributes + "\npublic void Fits(int n) {}\n}\n}\n")
	if err := os.WriteFile(filepath.Join(f.root, sourcePath), source, 0600); err != nil {
		t.Fatal(err)
	}
	for i := range f.r.Source.Inputs {
		if f.r.Source.Inputs[i].Path == sourcePath {
			f.r.Source.Inputs[i].SHA256 = unityDigest(source)
		}
	}
	addUnityTypedFixture(t, f)
	rewriteUnityTypedDefinition(t, f, func(o *scippb.Occurrence) {
		before := string(source[:strings.Index(string(source), "Fits(")])
		line := int32(strings.Count(before, "\n"))
		column := int32(len(before) - strings.LastIndex(before, "\n") - 1)
		o.Range = []int32{line, column, column + 4}
	})
	e, err := f.load()
	if err != nil {
		t.Fatal(err)
	}
	st := store.NewMemoryStore()
	pipeline := fusion.New(connector.NewLocalConnector(), st, fusion.Config{DisableTypedEdges: true})
	t.Cleanup(pipeline.Close)
	if _, err := pipeline.Ingest(context.Background(), core.ServiceRef{ID: "svc", Path: f.root}); err != nil {
		t.Fatal(err)
	}
	observations, err := st.QueryObservations(context.Background(), "svc", "")
	if err != nil {
		t.Fatal(err)
	}
	current := UnityCurrentSymbols(e, observations)
	if len(current) != 1 || current[0].Name != "BowlTests.Fits" || !strings.HasSuffix(current[0].Signature, "…") {
		t.Fatalf("fixture must exercise the real clipped method display: current=%+v observations=%+v", current, observations)
	}
	if current[0].IsTest() {
		t.Fatal("a clipped attribute list must not be accepted as a complete test policy")
	}
	return e, current
}

func TestUnityNUnitLongAttributesJoinFromBoundCurrentSource(t *testing.T) {
	e, current := unityLongAttributeFixture(t, "")
	joined, binding, err := UnityJoinCases(e, current)
	if err != nil {
		t.Fatal(err)
	}
	key := current[0].Key()
	if len(joined) != 2 || joined[0].TestSymbolKey != key || joined[1].TestSymbolKey != key || binding.Cohorts[key] != "pass" {
		t.Fatalf("a complete current two-variant method lost its typed native identity: joined=%+v binding=%+v", joined, binding)
	}
}

func TestUnityNUnitLongAttributesCannotHideRetryPolicy(t *testing.T) {
	for _, suffix := range []string{"[Retry(2)]", "[NUnit.Framework.Repeat(2)]"} {
		t.Run(suffix, func(t *testing.T) {
			e, current := unityLongAttributeFixture(t, suffix)
			if strings.Contains(current[0].Signature, "Retry") || strings.Contains(current[0].Signature, "Repeat") {
				t.Fatal("retry policy must be beyond the clipped display boundary")
			}
			joined, binding, err := UnityJoinCases(e, current)
			if err != nil {
				t.Fatal(err)
			}
			if len(binding.CaseKeys) != 0 || len(binding.Cohorts) != 0 {
				t.Fatalf("hidden unsupported retry policy was joined: %+v", binding)
			}
			for _, c := range joined {
				if c.TestSymbolKey != "" {
					t.Fatal("retry case acquired a native method identity")
				}
			}
		})
	}
}

func TestUnityNUnitLongAttributesKeepExactDeclarationAndSourceGuards(t *testing.T) {
	for _, which := range []string{"wrong end line", "duplicate declaration", "filtered family", "source changed"} {
		t.Run(which, func(t *testing.T) {
			e, current := unityLongAttributeFixture(t, "")
			switch which {
			case "wrong end line":
				current[0].EndLine++
			case "duplicate declaration":
				current = append(current, current[0])
			case "filtered family":
				e.Receipt.Filter = "Workshop.BowlTests.Fits(1)"
			case "source changed":
				path := filepath.Join(e.Receipt.Root, current[0].FilePath)
				if err := os.WriteFile(path, []byte("// changed after attestation\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			joined, binding, err := UnityJoinCases(e, current)
			if which == "source changed" {
				if err == nil {
					t.Fatal("changed bound source was not refused")
				}
				return
			}
			if err != nil || len(binding.CaseKeys) != 0 || len(binding.Cohorts) != 0 {
				t.Fatalf("unexpected guard result: %+v %v", binding, err)
			}
			for _, c := range joined {
				if c.TestSymbolKey != "" {
					t.Fatal("non-exact declaration acquired a runtime identity")
				}
			}
		})
	}
}
