package fusion

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func unityCorpus() map[string]string {
	return map[string]string{
		"Assets/Core/Core.asmdef":               `{"name":"Core"}`,
		"Assets/Core/Core.asmdef.meta":          "fileFormatVersion: 2\nguid: 0123456789abcdef0123456789abcdef\n",
		"Assets/Game/Game.asmdef":               `{"name":"Game","rootNamespace":"Example.Game","references":["GUID:0123456789abcdef0123456789abcdef","GUID:ffffffffffffffffffffffffffffffff"],"includePlatforms":[],"excludePlatforms":["Android"],"allowUnsafeCode":true,"overrideReferences":true,"precompiledReferences":["Example.dll"],"autoReferenced":false,"defineConstraints":["FEATURE","!DISABLED"],"versionDefines":[{"name":"com.example.feature","expression":"1.0","define":"FEATURE"}],"noEngineReferences":true}`,
		"Assets/Game/Main.cs":                   "public class Main {}\n",
		"Assets/Game/Deep/Nested.cs":            "public class Nested {}\n",
		"Assets/Game/Editor/Game.Editor.asmdef": `{"name":"Game.Editor","includePlatforms":["Editor"],"references":["Game"]}`,
		"Assets/Game/Editor/Inspector.cs":       "public class Inspector {}\n",
		"Assets/Shared/Shared.asmref":           `{"reference":"Game"}`,
		"Assets/Shared/Shared.cs":               "public class Shared {}\n",
		"Assets/Scripts/Loose.cs":               "public class Loose {}\n",
		"Assets/Scripts/Editor/Tool.cs":         "public class Tool {}\n",
		"Assets/Plugins/Lib.cs":                 "public class Lib {}\n",
		"Assets/Plugins/Editor/LibTool.cs":      "public class LibTool {}\n",
		"Packages/com.example.pkg/Runtime/X.cs": "public class X {}\n",
	}
}

func ingestUnityCorpus(t *testing.T, corpus map[string]string) (*Pipeline, *store.MemoryStore, string) {
	t.Helper()
	dir := t.TempDir()
	writeCorpus(t, dir, corpus)
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "unity-fixture", Path: dir}); err != nil {
		t.Fatal(err)
	}
	return p, st, dir
}

func unityFact(t *testing.T, st *store.MemoryStore, file, kind string) core.Observation {
	t.Helper()
	obs, err := st.QueryObservations(context.Background(), "unity-fixture", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range obs {
		if o.SourceRef == file && string(o.SourceKind) == kind {
			if o.EvidenceTier != core.TierPreScan {
				t.Fatalf("%s: Unity tier = %s, want pre-scan (derived)", file, o.EvidenceTier)
			}
			return o
		}
	}
	t.Fatalf("missing %s fact for %s", kind, file)
	return core.Observation{}
}

func unityClaim(t *testing.T, o core.Observation) map[string]any {
	t.Helper()
	var fact map[string]any
	if err := json.Unmarshal([]byte(o.Claim), &fact); err != nil {
		t.Fatal(err)
	}
	if fact["evidence"] != "derived" {
		t.Fatalf("missing derived label: %s", o.Claim)
	}
	return fact
}

func assertUnityOwner(t *testing.T, st *store.MemoryStore, file, assembly, scope string) {
	t.Helper()
	o := unityFact(t, st, file, "UnityAssemblyOwnership")
	f := unityClaim(t, o)
	if f["assembly"] != assembly || f["scope"] != scope {
		t.Fatalf("%s: got %s, want %s / %s", file, o.Claim, assembly, scope)
	}
	if assembly == "" {
		if f["status"] != "unknown" {
			t.Fatalf("unknown ownership = %s", o.Claim)
		}
		return
	}
	edges, err := st.TypedEdges(context.Background(), "unity-fixture")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range edges {
		if e.FromFile == file && e.FromSymbol == o.ToSymbolNode().Key() && e.Kind == core.EdgeFileResolved && strings.HasSuffix(e.ToSymbol, "."+assembly) {
			return
		}
	}
	t.Fatalf("missing file -> assembly edge for %s", file)
}

func TestUnityAssemblyOwnershipFollowsNearestAsmdef(t *testing.T) {
	_, st, _ := ingestUnityCorpus(t, unityCorpus())
	assertUnityOwner(t, st, "Assets/Game/Main.cs", "Game", "player-and-editor")
	assertUnityOwner(t, st, "Assets/Game/Deep/Nested.cs", "Game", "player-and-editor")
	assertUnityOwner(t, st, "Assets/Game/Editor/Inspector.cs", "Game.Editor", "editor-only")
}

func TestUnityPredefinedAssemblies(t *testing.T) {
	corpus := unityCorpus()
	for _, folder := range []string{"Standard Assets", "Pro Standard Assets"} {
		corpus["Assets/"+folder+"/Lib.cs"] = "public class Lib {}\n"
		corpus["Assets/"+folder+"/Editor/Tool.cs"] = "public class Tool {}\n"
	}
	corpus["Assets/Nested/Plugins/Other.cs"] = "public class Other {}\n"
	corpus["Assets/EditorTools/Other.cs"] = "public class Other {}\n"
	_, st, _ := ingestUnityCorpus(t, corpus)
	assertUnityOwner(t, st, "Assets/Scripts/Loose.cs", "Assembly-CSharp", "player-and-editor")
	assertUnityOwner(t, st, "Assets/Scripts/Editor/Tool.cs", "Assembly-CSharp-Editor", "editor-only")
	for _, folder := range []string{"Plugins", "Standard Assets", "Pro Standard Assets"} {
		assertUnityOwner(t, st, "Assets/"+folder+"/Lib.cs", "Assembly-CSharp-firstpass", "player-and-editor")
		file := "Assets/" + folder + "/Editor/Tool.cs"
		if folder == "Plugins" {
			file = "Assets/Plugins/Editor/LibTool.cs"
		}
		assertUnityOwner(t, st, file, "Assembly-CSharp-Editor-firstpass", "editor-only")
	}
	assertUnityOwner(t, st, "Assets/Nested/Plugins/Other.cs", "Assembly-CSharp", "player-and-editor")
	assertUnityOwner(t, st, "Assets/EditorTools/Other.cs", "Assembly-CSharp", "player-and-editor")
	assertUnityOwner(t, st, "Packages/com.example.pkg/Runtime/X.cs", "", "unknown")
}

func TestUnityAsmrefRedirectsFolder(t *testing.T) {
	corpus := unityCorpus()
	corpus["Assets/Redirect/Ref.asmref"] = `{"reference":"GUID:0123456789abcdef0123456789abcdef"}`
	corpus["Assets/Redirect/CorePart.cs"] = "public class CorePart {}\n"
	_, st, _ := ingestUnityCorpus(t, corpus)
	assertUnityOwner(t, st, "Assets/Shared/Shared.cs", "Game", "player-and-editor")
	assertUnityOwner(t, st, "Assets/Redirect/CorePart.cs", "Core", "player-and-editor")
}

func TestUnityGuidReferenceNeedsMeta(t *testing.T) {
	p, st, dir := ingestUnityCorpus(t, unityCorpus())
	check := func(want int, resolved bool) {
		t.Helper()
		o := unityFact(t, st, "Assets/Game/Game.asmdef", "UnityAssemblyDefinition")
		if got := unityClaim(t, o)["unresolvedReferences"]; got != float64(want) {
			t.Fatalf("unresolved = %v, want %d", got, want)
		}
		edges, _ := st.TypedEdges(context.Background(), "unity-fixture")
		found := false
		for _, e := range edges {
			if e.FromSymbol == o.ToSymbolNode().Key() && strings.HasSuffix(e.ToSymbol, ".Core") {
				found = true
			}
		}
		if found != resolved {
			t.Fatalf("Game -> Core = %v, want %v", found, resolved)
		}
	}
	check(1, true)
	if err := os.Remove(filepath.Join(dir, "Assets/Core/Core.asmdef.meta")); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "unity-fixture", Path: dir}); err != nil {
		t.Fatal(err)
	}
	check(2, false)
	// An existing but unreadable sidecar must not be guessed from the name.
	if err := os.Mkdir(filepath.Join(dir, "Assets/Core/Core.asmdef.meta"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "unity-fixture", Path: dir}); err != nil {
		t.Fatal(err)
	}
	check(2, false)
}

func TestUnityEditorOnlyScope(t *testing.T) {
	corpus := unityCorpus()
	corpus["Assets/Game/Editor/WithoutDefinition/Other.cs"] = "public class Other {}\n"
	corpus["Assets/Mixed/Mixed.asmdef"] = `{"name":"Mixed","includePlatforms":["Editor","Standalone"]}`
	corpus["Assets/Mixed/Mixed.cs"] = "public class Mixed {}\n"
	_, st, _ := ingestUnityCorpus(t, corpus)
	assertUnityOwner(t, st, "Assets/Game/Editor/WithoutDefinition/Other.cs", "Game.Editor", "editor-only")
	assertUnityOwner(t, st, "Assets/Mixed/Mixed.cs", "Mixed", "player-and-editor")
	game := unityClaim(t, unityFact(t, st, "Assets/Game/Game.asmdef", "UnityAssemblyDefinition"))
	for _, key := range []string{"rootNamespace", "references", "includePlatforms", "excludePlatforms", "allowUnsafeCode", "overrideReferences", "precompiledReferences", "autoReferenced", "defineConstraints", "versionDefines", "noEngineReferences"} {
		if _, ok := game[key]; !ok {
			t.Errorf("assembly metadata missing %s", key)
		}
	}
	if game["autoReferenced"] != false || game["allowUnsafeCode"] != true {
		t.Fatalf("flags = %v", game)
	}
	obs, _ := st.QueryObservations(context.Background(), "unity-fixture", "")
	var constraints []string
	edges, _ := st.TypedEdges(context.Background(), "unity-fixture")
	for _, o := range obs {
		if string(o.SourceKind) == "UnityDefineConstraint" {
			constraints = append(constraints, o.Symbol)
			found := false
			for _, e := range edges {
				if e.ToSymbol == o.ToSymbolNode().Key() {
					found = true
				}
			}
			if !found {
				t.Errorf("constraint %s lacks an incoming edge", o.Symbol)
			}
		}
		if strings.HasPrefix(string(o.SourceKind), "Unity") && o.EvidenceTier != core.TierPreScan {
			t.Errorf("verified Unity fact: %+v", o)
		}
	}
	sort.Strings(constraints)
	if !reflect.DeepEqual(constraints, []string{"!DISABLED", "FEATURE"}) {
		t.Fatalf("constraints = %v", constraints)
	}
}

func unitySnapshot(t *testing.T, st *store.MemoryStore) []byte {
	t.Helper()
	obs, err := st.QueryObservations(context.Background(), "unity-fixture", "")
	if err != nil {
		t.Fatal(err)
	}
	for i := range obs {
		obs[i].IngestedAt = time.Time{}
	}
	edges, err := st.TypedEdges(context.Background(), "unity-fixture")
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(obs, func(i, j int) bool {
		a, _ := json.Marshal(obs[i])
		b, _ := json.Marshal(obs[j])
		return bytes.Compare(a, b) < 0
	})
	sort.Slice(edges, func(i, j int) bool {
		a, _ := json.Marshal(edges[i])
		b, _ := json.Marshal(edges[j])
		return bytes.Compare(a, b) < 0
	})
	b, err := json.Marshal(struct {
		Observations []core.Observation
		Edges        []core.Edge
	}{obs, edges})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestUnityAbsentLeavesCSharpOutputsByteIdentical(t *testing.T) {
	corpus := map[string]string{"src/App.cs": "public class App { public void Run() {} }\n", "Assets/Plain.cs": "public class Plain {}\n"}
	p, st, dir := ingestUnityCorpus(t, corpus)
	before := unitySnapshot(t, st)
	obs, _ := st.QueryObservations(context.Background(), "unity-fixture", "")
	for _, o := range obs {
		if strings.HasPrefix(string(o.SourceKind), "Unity") {
			t.Fatalf("non-Unity repository produced %+v", o)
		}
	}
	if !bytes.Contains(before, []byte("CSharpClass")) {
		t.Fatal("negative control has no C# symbols")
	}
	// Compare the served symbols byte for byte with the incumbent C# dialect.
	x, err := extractor.NewTSExtractor("csharp")
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	var want, got []string
	for file, content := range corpus {
		nodes, err := x.Extract("unity-fixture", file, []byte(content))
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range nodes {
			b, err := json.Marshal(n)
			if err != nil {
				t.Fatal(err)
			}
			want = append(want, string(b))
		}
	}
	for _, o := range obs {
		b, err := json.Marshal(o.ToSymbolNode())
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, string(b))
	}
	sort.Strings(want)
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("C# symbols differ from incumbent:\ngot %v\nwant %v", got, want)
	}
	report, err := p.Ingest(context.Background(), core.ServiceRef{ID: "unity-fixture", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("UnityAssembly")) {
		t.Fatalf("non-Unity report gained Unity fields: %s", b)
	}
	if !bytes.Equal(before, unitySnapshot(t, st)) {
		t.Fatal("non-Unity C# output changed")
	}
	refreshed, err := p.ReingestFile(context.Background(), st, "unity-fixture", dir, "src/App.cs")
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.TypedEdgesStale {
		t.Fatal("non-Unity C# refresh changed its stale-edge verdict")
	}
}

func TestUnityResolutionRefusesAmbiguityAndUnreadableMeta(t *testing.T) {
	files := []unityAssemblyFile{
		{path: "Assets/Core/Core.asmdef", content: []byte(`{"name":"Core"}`)},
		{path: "Assets/Core/Core.asmdef.meta", fault: "permission denied"},
		{path: "Assets/Game/Game.asmdef", content: []byte(`{"name":"Game","references":["GUID:0123456789abcdef0123456789abcdef"]}`)},
		{path: "Assets/Game/Redirect/Ref.asmref", content: []byte(`{"reference":"Missing"}`)},
		{path: "Assets/Game/Redirect/A.cs"},
		{path: "Assets/Bad/A.asmdef", content: []byte(`{"name":`)},
		{path: "Assets/Bad/A.cs"},
		{path: "Assets/Duplicate/One.asmdef", content: []byte(`{"name":"One"}`)},
		{path: "Assets/Duplicate/Two.asmdef", content: []byte(`{"name":"Two"}`)},
		{path: "Assets/Duplicate/A.cs"},
	}
	result, err := ResolveUnityAssemblies("generated", files)
	if err != nil {
		t.Fatal(err)
	}
	if result.unresolved != 2 {
		t.Fatalf("unresolved=%d, want 2", result.unresolved)
	}
	if !strings.Contains(strings.Join(result.problems, "\n"), "permission denied") {
		t.Fatalf("unreadable metadata hidden: %v", result.problems)
	}
	owners := 0
	for _, n := range result.nodes {
		if n.Kind == "UnityAssemblyOwnership" {
			owners++
			if unityClaim(t, core.Observation{Claim: n.Signature})["status"] != "unknown" {
				t.Fatalf("invalid folder fell back: %+v", n)
			}
		}
	}
	if owners != 3 {
		t.Fatalf("ownership facts=%d, want 3", owners)
	}
	for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
		files[i], files[j] = files[j], files[i]
	}
	again, err := ResolveUnityAssemblies("generated", files)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result, again) {
		t.Fatal("resolution depends on input order")
	}
}

func TestUnityRefreshAndRemovalPreserveCSharp(t *testing.T) {
	p, st, dir := ingestUnityCorpus(t, unityCorpus())
	before := unitySnapshot(t, st)
	if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "unity-fixture", Path: dir}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, unitySnapshot(t, st)) {
		t.Fatal("repeat ingest changed assembly facts")
	}
	if _, err := p.ReingestFile(context.Background(), st, "unity-fixture", dir, "Assets/Game/Main.cs"); err != nil {
		t.Fatal(err)
	}
	assertUnityOwner(t, st, "Assets/Game/Main.cs", "Game", "player-and-editor")
	for file := range unityCorpus() {
		if strings.HasSuffix(file, ".asmdef") || strings.HasSuffix(file, ".asmref") || strings.HasSuffix(file, ".meta") {
			if err := os.Remove(filepath.Join(dir, filepath.FromSlash(file))); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "unity-fixture", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(context.Background(), "unity-fixture", "")
	if err != nil {
		t.Fatal(err)
	}
	hasCSharp := false
	for _, o := range obs {
		if strings.HasPrefix(string(o.SourceKind), "Unity") {
			t.Fatalf("stale Unity fact after removal: %+v", o)
		}
		if o.SourceKind == "CSharpClass" {
			hasCSharp = true
		}
	}
	if !hasCSharp {
		t.Fatal("Unity removal lost C# symbols")
	}
	edges, err := st.TypedEdges(context.Background(), "unity-fixture")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range edges {
		if strings.Contains(e.FromSymbol, "unity:ownership") {
			t.Fatalf("stale Unity edge: %+v", e)
		}
	}
}
