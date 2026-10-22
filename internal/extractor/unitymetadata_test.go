package extractor

import (
	"encoding/json"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestUnityMetadataDialectSharesDerivedClaims(t *testing.T) {
	x, err := NewDocDialectExtractor("unity-asmdef")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ file, content, kind, name string }{
		{"Assets/Core/Core.asmdef", `{"name":"Core","includePlatforms":["Android"],"rootNamespace":"Example","defineConstraints":["FEATURE"]}`, "UnityAssemblyDefinition", "Core"},
		{"Assets/Shared/Shared.asmref", `{"reference":"Core"}`, "UnityAssemblyReference", "Core"},
	} {
		t.Run(tc.file, func(t *testing.T) {
			result, err := x.ExtractDoc("generated", tc.file, []byte(tc.content))
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, n := range result.Nodes {
				if n.Tier != core.TierPreScan {
					t.Errorf("declaration tier = %s", n.Tier)
				}
				var claim map[string]any
				if err := json.Unmarshal([]byte(n.Signature), &claim); err != nil {
					t.Fatal(err)
				}
				if claim["evidence"] != "derived" {
					t.Errorf("not derived: %s", n.Signature)
				}
				if n.Kind != core.SourceKind(tc.kind) {
					continue
				}
				found = true
				if n.Package != pkgFromPath(tc.file) || n.Name != tc.name {
					t.Errorf("identity = %+v", n)
				}
				if claim["refreshNote"] != "Unity metadata refresh needs a full ingest" {
					t.Errorf("missing full-ingest note: %s", n.Signature)
				}
				if _, ok := claim["unresolvedReferences"]; ok {
					t.Errorf("single-file declaration claims cross-file resolution: %s", n.Signature)
				}
				if n.Kind == "UnityAssemblyDefinition" && (claim["scope"] != "player-only" || claim["rootNamespace"] != "Example") {
					t.Errorf("metadata lost: %s", n.Signature)
				}
			}
			if !found {
				t.Fatal("missing metadata declaration")
			}
		})
	}
}

func TestUnityMetadataDialectIgnoresHiddenPaths(t *testing.T) {
	x, err := NewDocDialectExtractor("unity-asmdef")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"Assets/.hidden/A.asmdef", "Packages/pkg/Samples~/A.asmdef", "Assets/cvs/A.asmdef", "Assets/cache.tmp/A.asmdef", "Assets/.Ignored.asmdef"} {
		result, err := x.ExtractDoc("generated", file, []byte(`{"name":"Ignored"}`))
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Nodes) != 0 {
			t.Errorf("%s produced Unity facts: %+v", file, result.Nodes)
		}
	}
}
