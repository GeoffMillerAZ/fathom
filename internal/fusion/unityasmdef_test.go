package fusion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngestUnityAsmdefDeclarationsAreNotCalls(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"Assets/Rules/Rules.asmdef":       `{"name":"Fixture.Rules","noEngineReferences":true}`,
		"Assets/Adapters/Adapters.asmdef": `{"name":"Fixture.Adapters","references":["Fixture.Rules","Missing.Assembly"]}`,
		"Assets/Rules/Rules.cs":           "namespace Fixture { public class Rules {} }",
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	_, err := p.Ingest(ctx, core.ServiceRef{ID: "unity", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "unity", "")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, o := range obs {
		if !strings.HasPrefix(string(o.SourceKind), "UnityAssembly") {
			continue
		}
		got = append(got, o.SourceRef+":"+o.Symbol+":"+string(o.SourceKind))
		if o.Symbol == "Fixture.Rules" && string(o.SourceKind) == "UnityAssemblyDefinition" && o.Claim != "noEngineReferences=true" {
			t.Fatalf("declared engine flag was lost in store: %+v", o)
		}
		if len(o.Calls) > 0 {
			t.Fatalf("asmdef dependency became calls: %+v", o)
		}
	}
	sort.Strings(got)
	want := []string{"Assets/Adapters/Adapters.asmdef:Fixture.Adapters:UnityAssemblyDefinition", "Assets/Adapters/Adapters.asmdef:Fixture.Rules:UnityAssemblyReference", "Assets/Adapters/Adapters.asmdef:Missing.Assembly:UnityAssemblyReference", "Assets/Rules/Rules.asmdef:Fixture.Rules:UnityAssemblyDefinition"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("declarations=%q, want %q", got, want)
	}
	edges, err := st.TypedEdges(ctx, "unity")
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Fatalf("declarations became typed/file call edges: %+v", edges)
	}
}

func TestIngestUnityAsmdefInvalidatesUnknownFingerprint(t *testing.T) {
	dir := t.TempDir()
	src := `{"name":"Fixture.Rules","noEngineReferences":true}`
	writeCorpus(t, dir, map[string]string{"Assets/Rules.asmdef": src})
	st := store.NewMemoryStore()
	ctx := context.Background()
	old := sha256.Sum256(append([]byte(src), []byte("\x00fathom-extractor-epoch:64")...))
	if err := st.SetFingerprint(ctx, "unity", "Assets/Rules.asmdef", hex.EncodeToString(old[:])); err != nil {
		t.Fatal(err)
	}
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	_, err := p.Ingest(ctx, core.ServiceRef{ID: "unity", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "unity", "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, o := range obs {
		found = found || (o.Symbol == "Fixture.Rules" && string(o.SourceKind) == "UnityAssemblyDefinition")
	}
	if !found {
		t.Fatal("previously unclassified asmdef fingerprint prevented new assembly extraction")
	}
}
