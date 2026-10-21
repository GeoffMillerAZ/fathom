package connector_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// The ingest dialect alone is insufficient: discovery, language reporting and
// capabilities must classify the same Unity assembly-definition files. The
// serialized metadata and assembly-reference format are not claimed by it.
func TestUnityAsmdefClassificationAcrossLocalDiscovery(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"Rules.asmdef":      `{"name":"Fixture.Rules","noEngineReferences":true}`,
		"Rules.asmdef.meta": "fileFormatVersion: 2\n",
		"Other.asmref":      `{"reference":"Fixture.Rules"}`,
		"Rules.cs":          "namespace Fixture { public class Rules {} }",
	}
	for path, source := range files {
		if err := os.WriteFile(filepath.Join(root, path), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	refs, err := connector.NewLocalConnector().Discover(context.Background(), core.ServiceRef{ID: "unity", Path: root})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ref := range refs {
		if ref.Path != "Rules.asmdef" {
			continue
		}
		found = true
		if ref.Kind != core.SourceKind("UnityAssemblyDefinition") {
			t.Errorf("discovery kind = %q, want UnityAssemblyDefinition", ref.Kind)
		}
		if lang := connector.LanguageForPath(ref.Path); lang != "unity-asmdef" {
			t.Errorf("language = %q, want unity-asmdef", lang)
		}
	}
	if !found {
		t.Fatal("local discovery omitted the assembly definition")
	}
	if producer := connector.TypedEdgeProducer("unity-asmdef"); producer != "syntax" {
		t.Errorf("declared facts producer = %q, want syntax (no typed edges)", producer)
	}
	for _, path := range []string{"Other.asmref", "Rules.asmdef.meta"} {
		if lang := connector.LanguageForPath(path); lang != core.LangUnclassified {
			t.Errorf("unsupported %s classified as %q", path, lang)
		}
	}
}
