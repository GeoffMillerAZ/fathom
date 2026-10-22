package fusion

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func resolveUnityReview(t *testing.T, corpus map[string]string) unityAssemblyResult {
	t.Helper()
	var files []unityAssemblyFile
	for file, content := range corpus {
		files = append(files, unityAssemblyFile{path: file, content: []byte(content)})
	}
	r, err := ResolveUnityAssemblies("generated", files)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func reviewOwner(t *testing.T, r unityAssemblyResult, file, assembly, scope string) {
	t.Helper()
	for _, n := range r.nodes {
		if n.FilePath == file && n.Kind == "UnityAssemblyOwnership" {
			f := unityClaim(t, core.Observation{Claim: n.Signature})
			if f["assembly"] != assembly || f["scope"] != scope {
				t.Fatalf("%s: %s, want %s / %s", file, n.Signature, assembly, scope)
			}
			return
		}
	}
	t.Fatalf("no ownership for %s", file)
}

func TestUnityReviewPredefinedAnchor(t *testing.T) {
	for _, marker := range []bool{false, true} {
		t.Run(fmt.Sprint(marker), func(t *testing.T) {
			corpus := map[string]string{"Assets/Core/Core.asmdef": `{"name":"Core"}`, "Assets/A.cs": "", "Assets/editor/Tool.cs": ""}
			if marker {
				corpus["ProjectSettings/ProjectVersion.txt"] = "m_EditorVersion: 2021.3.0f1\n"
			}
			r := resolveUnityReview(t, corpus)
			reviewOwner(t, r, "Assets/A.cs", "Assembly-CSharp", "player-and-editor")
			count := 0
			for _, n := range r.nodes {
				if n.Kind == "UnityAssemblyDefinition" && strings.HasPrefix(n.Name, "Assembly-CSharp") {
					count++
					if n.FilePath != "ProjectSettings/ProjectVersion.txt" || n.Line != 1 {
						t.Errorf("predefined assembly anchored to %+v", n)
					}
				}
			}
			want := 0
			if marker {
				want = 2
			}
			if count != want {
				t.Errorf("predefined nodes = %d, want %d", count, want)
			}
			for _, e := range r.edges {
				if !marker && strings.Contains(e.ToSymbol, "Assembly-CSharp") {
					t.Errorf("unanchored predefined edge: %+v", e)
				}
			}
		})
	}
}

func TestUnityReviewFolderRules(t *testing.T) {
	corpus := map[string]string{
		"Assets/Core/Core.asmdef": `{"name":"Core"}`, "Assets/Core/Editor/X.cs": "",
		"Assets/editor/X.cs": "", "Assets/plugins/X.cs": "", "Assets/plugins/editor/X.cs": "",
		"Assets/standard assets/X.cs": "", "Assets/pro standard assets/X.cs": "", "Assets/iphone standard assets/X.cs": "",
	}
	r := resolveUnityReview(t, corpus)
	reviewOwner(t, r, "Assets/Core/Editor/X.cs", "Core", "player-and-editor")
	reviewOwner(t, r, "Assets/editor/X.cs", "Assembly-CSharp-Editor", "editor-only")
	reviewOwner(t, r, "Assets/plugins/editor/X.cs", "Assembly-CSharp-Editor-firstpass", "editor-only")
	for _, dir := range []string{"plugins", "standard assets", "pro standard assets", "iphone standard assets"} {
		reviewOwner(t, r, "Assets/"+dir+"/X.cs", "Assembly-CSharp-firstpass", "player-and-editor")
	}
}

func TestUnityReviewIgnoredPaths(t *testing.T) {
	corpus := map[string]string{"Assets/Core/Core.asmdef": `{"name":"Core"}`, "Assets/Core/.Ignored.asmdef": `{"name":"Ignored"}`, "Assets/Core/X.cs": ""}
	for _, dir := range []string{".hidden", "Samples~", "cvs", "CVS", "cache.tmp"} {
		corpus["Packages/pkg/"+dir+"/Demo/Demo.cs"] = ""
		corpus["Assets/"+dir+"/Bad.asmdef"] = `{"name":"Bad"}`
		corpus["Assets/"+dir+"/X.cs"] = ""
	}
	r := resolveUnityReview(t, corpus)
	reviewOwner(t, r, "Assets/Core/X.cs", "Core", "player-and-editor")
	for _, n := range r.nodes {
		if n.FilePath != "Assets/Core/Core.asmdef" && n.FilePath != "Assets/Core/X.cs" {
			t.Errorf("ignored path produced fact: %+v", n)
		}
	}
}

func TestUnityReviewAmbiguities(t *testing.T) {
	for _, tc := range []struct {
		name          string
		corpus        map[string]string
		file, problem string
	}{
		{"asmdef-and-asmref", map[string]string{"Assets/Core/Core.asmdef": `{"name":"Core"}`, "Assets/Core/Ref.asmref": `{"reference":"Core"}`, "Assets/Core/X.cs": ""}, "Assets/Core/X.cs", "multiple assembly"},
		{"two-asmdefs", map[string]string{"Assets/Core/A.asmdef": `{"name":"A"}`, "Assets/Core/B.asmdef": `{"name":"B"}`, "Assets/Core/X.cs": ""}, "Assets/Core/X.cs", "multiple assembly"},
		{"duplicate-names", map[string]string{"Assets/A/A.asmdef": `{"name":"Duplicate"}`, "Assets/B/B.asmdef": `{"name":"Duplicate"}`, "Assets/A/X.cs": ""}, "Assets/A/X.cs", "duplicate assembly name"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := resolveUnityReview(t, tc.corpus)
			if !strings.Contains(strings.Join(r.problems, "\n"), tc.problem) {
				t.Errorf("problems = %v, want %q", r.problems, tc.problem)
			}
			reviewOwner(t, r, tc.file, "", "unknown")
		})
	}
}

func TestUnityReviewProjectRoots(t *testing.T) {
	corpus := map[string]string{
		"game/ProjectSettings/ProjectVersion.txt": "m_EditorVersion: 2021.3.0f1\n",
		"game/Assets/plugins/editor/X.cs":         "", "game/Assets/Core/Core.asmdef": `{"name":"Core"}`, "game/Assets/Core/Editor/X.cs": "",
		"other/Assets/Core/Core.asmdef": `{"name":"Core"}`, "other/Assets/Core/X.cs": "",
	}
	r := resolveUnityReview(t, corpus)
	reviewOwner(t, r, "game/Assets/plugins/editor/X.cs", "Assembly-CSharp-Editor-firstpass", "editor-only")
	reviewOwner(t, r, "game/Assets/Core/Editor/X.cs", "Core", "player-and-editor")
	reviewOwner(t, r, "other/Assets/Core/X.cs", "Core", "player-and-editor")
	if len(r.problems) != 0 {
		t.Errorf("separate roots conflicted: %v", r.problems)
	}
	corpus["ProjectSettings/ProjectVersion.txt"] = ""
	r = resolveUnityReview(t, corpus)
	if !strings.Contains(strings.Join(r.problems, "\n"), "ambiguous Unity project roots") {
		t.Errorf("nested roots not disclosed: %v", r.problems)
	}
}

func TestUnityReviewPlatformScopes(t *testing.T) {
	for _, tc := range []struct{ platforms, scope string }{
		{`"includePlatforms":["Android"]`, "player-only"},
		{`"excludePlatforms":["Editor"]`, "player-only"},
		{`"includePlatforms":["Editor"]`, "editor-only"},
		{`"includePlatforms":["Editor","Android"]`, "player-and-editor"},
	} {
		t.Run(tc.platforms, func(t *testing.T) {
			r := resolveUnityReview(t, map[string]string{"Assets/A/A.asmdef": `{"name":"A",` + tc.platforms + `}`, "Assets/A/X.cs": ""})
			reviewOwner(t, r, "Assets/A/X.cs", "A", tc.scope)
		})
	}
}

func TestUnityReviewCRLFMeta(t *testing.T) {
	r := resolveUnityReview(t, map[string]string{
		"Assets/A/A.asmdef": `{"name":"A"}`, "Assets/A/A.asmdef.meta": "fileFormatVersion: 2\r\nguid: 0123456789abcdef0123456789abcdef\r\n",
		"Assets/B/B.asmref": `{"reference":"GUID:0123456789abcdef0123456789abcdef"}`, "Assets/B/X.cs": "",
	})
	reviewOwner(t, r, "Assets/B/X.cs", "A", "player-and-editor")
	if len(r.problems) != 0 {
		t.Fatal(r.problems)
	}
}

func TestUnityReviewUnreadableMeta(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read mode-000 files")
	}
	dir := t.TempDir()
	corpus := unityCorpus()
	writeCorpus(t, dir, corpus)
	meta := filepath.Join(dir, "Assets/Core/Core.asmdef.meta")
	if err := os.Chmod(meta, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(meta, 0600) })
	if _, err := os.ReadFile(meta); err == nil {
		t.Fatal("mode-000 fixture is readable")
	}
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	defer p.Close()
	refs, err := p.conn.Discover(context.Background(), core.ServiceRef{ID: "unity-fixture", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	rep, err := p.enrichUnityAssemblies(context.Background(), core.ServiceRef{ID: "unity-fixture", Path: dir}, refs, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(rep.problems, "\n"), "permission denied") {
		t.Errorf("missing unreadable .meta problem: %v", rep.problems)
	}
	f := unityClaim(t, unityFact(t, st, "Assets/Game/Game.asmdef", "UnityAssemblyDefinition"))
	if f["unresolvedReferences"] != float64(2) {
		t.Errorf("unreadable GUID resolved: %v", f)
	}
}

func TestUnityReviewRemovingNestedDefinition(t *testing.T) {
	corpus := map[string]string{"Assets/A/A.asmdef": `{"name":"A"}`, "Assets/A/Nested/B.asmdef": `{"name":"B"}`, "Assets/A/Nested/X.cs": "public class X {}"}
	p, st, dir := ingestUnityCorpus(t, corpus)
	assertUnityOwner(t, st, "Assets/A/Nested/X.cs", "B", "player-and-editor")
	if err := os.Remove(filepath.Join(dir, "Assets/A/Nested/B.asmdef")); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "unity-fixture", Path: dir}); err != nil {
		t.Fatal(err)
	}
	assertUnityOwner(t, st, "Assets/A/Nested/X.cs", "A", "player-and-editor")
}

func TestUnityReviewMetadataRefresh(t *testing.T) {
	p, st, dir := ingestUnityCorpus(t, unityCorpus())
	for _, file := range []string{"Assets/Shared/Shared.asmref", "Assets/Game/Game.asmdef"} {
		kind := "UnityAssemblyDefinition"
		if strings.HasSuffix(file, ".asmref") {
			kind = "UnityAssemblyReference"
		}
		before := unityFact(t, st, file, kind)
		content, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, file), append(content, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ReingestFile(context.Background(), st, "unity-fixture", dir, file); err != nil {
			t.Fatal(err)
		}
		after := unityFact(t, st, file, kind)
		if before.ToSymbolNode().Key() != after.ToSymbolNode().Key() {
			t.Errorf("refresh changed identity: %s -> %s", before.ToSymbolNode().Key(), after.ToSymbolNode().Key())
		}
		claim := unityClaim(t, after)
		if claim["refreshNote"] != "Unity metadata refresh needs a full ingest" {
			t.Errorf("missing refresh note: %s", after.Claim)
		}
		if kind == "UnityAssemblyDefinition" && claim["rootNamespace"] != "Example.Game" {
			t.Errorf("refresh lost metadata: %s", after.Claim)
		}
	}
}

type unityReviewStore struct {
	*store.MemoryStore
	reads, purges, puts, edges int
}

func (s *unityReviewStore) SourceObservations(ctx context.Context, service string, paths []string) ([]core.Observation, error) {
	s.reads++
	return s.MemoryStore.SourceObservations(ctx, service, paths)
}
func (s *unityReviewStore) PurgeSource(ctx context.Context, service, file string) error {
	s.purges++
	return s.MemoryStore.PurgeSource(ctx, service, file)
}
func (s *unityReviewStore) PutObservations(ctx context.Context, obs []core.Observation) error {
	s.puts++
	return s.MemoryStore.PutObservations(ctx, obs)
}
func (s *unityReviewStore) PutTypedEdges(ctx context.Context, service, key string, edges []core.Edge) error {
	s.edges++
	return s.MemoryStore.PutTypedEdges(ctx, service, key, edges)
}

func TestUnityReviewNoOpDoesNotRewriteObservations(t *testing.T) {
	_, st, dir := ingestUnityCorpus(t, unityCorpus())
	spy := &unityReviewStore{MemoryStore: st}
	conn := connector.NewLocalConnector()
	refs, err := conn.Discover(context.Background(), core.ServiceRef{ID: "unity-fixture", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	p := New(conn, spy, Config{})
	defer p.Close()
	before, _ := st.QueryObservations(context.Background(), "unity-fixture", "")
	if _, err := p.enrichUnityAssemblies(context.Background(), core.ServiceRef{ID: "unity-fixture", Path: dir}, refs, time.Unix(123, 0)); err != nil {
		t.Fatal(err)
	}
	after, _ := st.QueryObservations(context.Background(), "unity-fixture", "")
	if spy.purges != 0 || spy.puts != 0 {
		t.Errorf("no-op rewrote facts: purges=%d puts=%d", spy.purges, spy.puts)
	}
	if !reflect.DeepEqual(before, after) {
		t.Error("no-op changed stored observations")
	}
}

func TestUnityReviewNonUnityAllocationBound(t *testing.T) {
	spy := &unityReviewStore{MemoryStore: store.NewMemoryStore()}
	p := New(connector.NewLocalConnector(), spy, Config{})
	defer p.Close()
	refs := make([]core.SourceRef, 2048)
	for i := range refs {
		refs[i].Path = fmt.Sprintf("src/Code%d.cs", i)
	}
	allocs := testing.AllocsPerRun(5, func() {
		if _, err := p.enrichUnityAssembliesChanged(context.Background(), core.ServiceRef{ID: "generated"}, refs, []string{"src/Code0.cs"}, time.Time{}); err != nil {
			t.Fatal(err)
		}
	})
	if allocs > 16 {
		t.Errorf("non-Unity pass allocated %.0f objects, bound 16", allocs)
	}
	if spy.reads+spy.purges+spy.puts+spy.edges != 0 {
		t.Errorf("non-Unity pass touched store: %+v", spy)
	}
}

func TestUnityReviewTypedKeySurvivesGoReconciliation(t *testing.T) {
	if !isNamespacedTypedKey(unityAssemblyEdgeKey) {
		t.Fatal("Go typed pass treats Unity edges as a stale Go package")
	}
}

func TestUnityReviewSubdirectoryIngest(t *testing.T) {
	_, st, _ := ingestUnityCorpus(t, map[string]string{
		"g/ProjectSettings/ProjectVersion.txt": "m_EditorVersion: 2021.3.0f1\n",
		"g/Assets/Core/Core.asmdef":            `{"name":"Core"}`,
		"g/Assets/Core/Editor/X.cs":            "public class X {}",
		"g/Assets/plugins/X.cs":                "public class Plugin {}",
	})
	assertUnityOwner(t, st, "g/Assets/Core/Editor/X.cs", "Core", "player-and-editor")
	assertUnityOwner(t, st, "g/Assets/plugins/X.cs", "Assembly-CSharp-firstpass", "player-and-editor")
	n := unityFact(t, st, "g/ProjectSettings/ProjectVersion.txt", "UnityAssemblyDefinition")
	if n.Symbol != "Assembly-CSharp-firstpass" {
		t.Fatalf("predefined anchor = %+v", n)
	}
}

func TestUnityReviewPassChargedToItself(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{"Assets/Core/Core.asmdef": `{"name":"Core"}`, "Assets/Core/X.cs": "public class X {}"})
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	defer p.Close()
	// An injected constant clock records labels without measuring real time.
	ctx := WithPassClock(context.Background(), PassClock{
		Now: func() time.Time { return time.Unix(123, 0) },
		CPU: func() time.Duration { return 0 },
	})
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "generated", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	for _, pass := range rep.PassTimes {
		if pass.Name == "unity_assembly" && pass.Parent == "" {
			return
		}
	}
	t.Fatalf("Unity pass uncharged: %+v", rep.PassTimes)
}

func TestUnityReviewIgnoredFactsAreRemoved(t *testing.T) {
	p, st, dir := ingestUnityCorpus(t, map[string]string{
		"Assets/Core/Core.asmdef":                `{"name":"Core"}`,
		"Packages/pkg/Samples~/Demo/Demo.asmdef": `{"name":"Old"}`,
		"Packages/pkg/Samples~/Demo/Demo.cs":     "public class Demo {}",
	})
	// Model persisted facts from the previous resolver, which imported Samples~.
	if err := st.PutObservations(context.Background(), []core.Observation{
		{ServiceID: "unity-fixture", SourceRef: "Packages/pkg/Samples~/Demo/Demo.cs", SourceKind: "UnityAssemblyOwnership", Symbol: "unity:ownership", Claim: `{"evidence":"derived","assembly":"Old"}`},
		{ServiceID: "unity-fixture", SourceRef: "Packages/pkg/Samples~/Demo/Demo.asmdef", SourceKind: "UnityAssemblyDefinition", Symbol: "Old", Claim: `{"evidence":"derived"}`},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "unity-fixture", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(context.Background(), "unity-fixture", "")
	if err != nil {
		t.Fatal(err)
	}
	foundCSharp := false
	for _, o := range obs {
		if !strings.Contains(o.SourceRef, "Samples~/") {
			continue
		}
		if strings.HasPrefix(string(o.SourceKind), "Unity") {
			t.Errorf("stale ignored Unity fact survived: %+v", o)
		}
		if o.SourceKind == "CSharpClass" {
			foundCSharp = true
		}
	}
	if !foundCSharp {
		t.Fatal("cleanup lost ordinary C# extraction")
	}
}
