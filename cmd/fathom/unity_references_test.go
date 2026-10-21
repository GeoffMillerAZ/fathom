package main

// Protocol admission is tested in internal/assetrefs and through a separate
// genuine producer/public-client probe. These CLI fixtures exercise dispatch,
// flag identity, read-only mounting, refusal output and presentation; they do
// not pretend a synthetic receipt means Unity ran.
import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func unityReferencesArgs(path string) []string {
	return []string{"unity-references", "--store", path, "--service", "fixture", "--project", "game/Fixture", "--family", "cart", "--receipt", ".artifacts/fixture/receipt.json", "--active", ".artifacts/fixture/active.json", "--editor-version", "fixture-editor", "--editor-sha256", strings.Repeat("1", 64), "--target", "StandaloneOSX", "--guid", strings.Repeat("2", 32), "--file-id", "9007199254740993", "--json"}
}

func TestUnityReferencesDispatchRequiresExplicitScopeAndExactFlags(t *testing.T) {
	base := unityReferencesArgs(filepath.Join(t.TempDir(), "missing.sqlite"))
	for _, name := range []string{"service", "project", "family", "receipt", "active", "editor-version", "editor-sha256", "target", "guid", "file-id"} {
		t.Run("missing-"+name, func(t *testing.T) {
			args := append([]string(nil), base...)
			for index, value := range args {
				if value == "--"+name {
					args = append(args[:index], args[index+2:]...)
					break
				}
			}
			var out, stderr bytes.Buffer
			if code := run(args, &out, &stderr); code == 0 || out.Len() != 0 || !strings.Contains(stderr.String(), "requires --"+name) {
				t.Fatalf("missing flag silently accepted/printed positive output: code%d out%q err%q", code, out.String(), stderr.String())
			}
		})
	}
	for name, extra := range map[string][]string{"no-arbitrary-root": {"--root", "/untrusted"}, "invalid-bool": {"--json=maybe"}, "invalid-limit": {"--limit", "NaN"}, "overbound-limit": {"--limit", "101"}, "negative-limit": {"--limit", "-1"}, "not-transitive": {"--direction", "transitive"}, "no-positionals": {"unexpected"}} {
		t.Run(name, func(t *testing.T) {
			var out, stderr bytes.Buffer
			args := append(append([]string(nil), base...), extra...)
			if code := run(args, &out, &stderr); code == 0 || out.Len() != 0 || stderr.Len() == 0 {
				t.Fatalf("bad flags accepted: code%d out%q err%q", code, out.String(), stderr.String())
			}
		})
	}
}

func TestUnityReferencesMissingStoreDoesNotCreateAnything(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.sqlite")
	var out, stderr bytes.Buffer
	if code := run(unityReferencesArgs(path), &out, &stderr); code == 0 || out.Len() != 0 || !strings.Contains(stderr.String(), "read-only store") {
		t.Fatalf("missing store did not refuse read-only: code%d out%q err%q", code, out.String(), stderr.String())
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("query created store/sidecars: %#v", files)
	}
}

func TestUnityReferencesRegisteredRootRefusalPreservesStoreAndCode(t *testing.T) {
	ctx := context.Background()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "registered.sqlite")
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	observations := []core.Observation{{ServiceID: "fixture", SourceRef: "code.go:1", SourceKind: core.KindGoFunc, EvidenceTier: core.TierPreScan, Symbol: "Existing", Claim: "independent code", Line: 1, EndLine: 1}}
	edges := []core.Edge{{FromService: "fixture", FromFile: "code.go", FromSymbol: "fixture.Existing", ToSymbol: "fixture.Callee", Kind: core.EdgeDirect}}
	if err := st.PutObservations(ctx, observations); err != nil {
		t.Fatal(err)
	}
	if err := st.PutTypedEdges(ctx, "fixture", "scip:independent", edges); err != nil {
		t.Fatal(err)
	}
	beforeObs, err := st.QueryObservations(ctx, "fixture", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for name, extra := range map[string][]string{"json-false": {"--json=false"}, "zero-default-limit": {"--limit", "0"}, "negative-signed64-id": {"--file-id", "-5831914722253275453"}, "unregistered": {"--service", "unregistered"}, "noncanonical-id": {"--file-id", "9e15"}} {
		t.Run(name, func(t *testing.T) {
			var out, stderr bytes.Buffer
			args := append(unityReferencesArgs(path), extra...)
			if code := run(args, &out, &stderr); code == 0 || out.Len() != 0 || !strings.Contains(stderr.String(), "Unity asset references refused") {
				t.Fatalf("refusal printed positive output or never reached public client: code%d out%q err%q", code, out.String(), stderr.String())
			}
			if name == "unregistered" && !strings.Contains(stderr.String(), "registered service root required") {
				t.Fatalf("root authority bypass: %s", stderr.String())
			}
			if name == "noncanonical-id" && !strings.Contains(stderr.String(), "invalid asset query scope") {
				t.Fatalf("rounded/coerced fileID: %s", stderr.String())
			}
		})
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("read-only CLI changed main store bytes")
	}
	mounted, err := store.MountReadOnly(path, store.LevelL0)
	if err != nil {
		t.Fatal(err)
	}
	defer mounted.Close()
	if err := mounted.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "forbidden", Path: root}); err == nil {
		t.Fatal("read-only mount accepted a write")
	}
	afterObs, err := mounted.QueryObservations(ctx, "fixture", "")
	if err != nil {
		t.Fatal(err)
	}
	afterEdges, err := mounted.TypedEdges(ctx, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(beforeObs, afterObs) || !reflect.DeepEqual(edges, afterEdges) {
		t.Fatal("asset CLI disturbed code observations/typed edges")
	}
}

func unityReferencesPresentationFixture() client.AssetReferenceResult {
	source := client.AssetObjectID{GUID: strings.Repeat("1", 32), FileID: "-5831914722253275453"}
	target := client.AssetObjectID{GUID: strings.Repeat("2", 32), FileID: "9007199254740993"}
	return client.AssetReferenceResult{Status: "current_partial", Service: "fixture", Project: "game/Fixture", FamilyID: "cart", Object: target, Direction: "inbound", ReceiptSHA256: strings.Repeat("3", 64), SourceSHA256: strings.Repeat("4", 64), ContextSHA256: strings.Repeat("5", 64), EditorVersion: "fixture-editor", BuildTarget: "StandaloneOSX", Limitations: []string{"Partial declared/native intersection; no runtime or grade."}, Total: 1, References: []core.AssetReference{{Kind: "declared-object-reference", Source: source, Target: target, SourcePath: "game/Fixture/Assets/Cart.prefab", TargetPath: "game/Fixture/Assets/Model.fbx", MatchedOccurrences: 2}}, Counts: core.AssetReferenceCounts{Matched: 3, AdmittedMatched: 2, NativeObservedUnbound: 1, NativeOnly: 1}}
}

func TestUnityReferencesRendererKeepsPathsExactIDsAndPartialDenominators(t *testing.T) {
	result := unityReferencesPresentationFixture()
	for _, jsonOut := range []bool{false, true} {
		var out bytes.Buffer
		if err := renderUnityReferences(&out, result, jsonOut); err != nil {
			t.Fatal(err)
		}
		if out.Len() > unityReferencesWireLimit {
			t.Fatal("output exceeded budget")
		}
		for _, part := range []string{"current_partial", "9007199254740993", "-5831914722253275453", "game/Fixture/Assets/Cart.prefab", "game/Fixture/Assets/Model.fbx"} {
			if !strings.Contains(out.String(), part) {
				t.Fatalf("lost readable/exact identity %q: %s", part, out.String())
			}
		}
		if jsonOut {
			var decoded client.AssetReferenceResult
			if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.Counts.NativeObservedUnbound != 1 || decoded.FamilyComplete {
				t.Fatal("JSON inflated cohort coverage")
			}
		} else if !strings.Contains(out.String(), "package-unbound=1") || !strings.Contains(out.String(), "unpaired") {
			t.Fatal("text hid partial/unpaired denominator")
		}
	}
}

func TestUnityReferencesRendererBudgetsEntireJSONWithoutExtraNewline(t *testing.T) {
	result := unityReferencesPresentationFixture()
	result.Limitations = []string{""}
	base, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	result.Limitations[0] = strings.Repeat("x", unityReferencesWireLimit-len(base))
	var exact bytes.Buffer
	if err := renderUnityReferences(&exact, result, true); err != nil {
		t.Fatal(err)
	}
	if exact.Len() != unityReferencesWireLimit || !json.Valid(exact.Bytes()) {
		t.Fatalf("exact cap includes unexpected framing: %d", exact.Len())
	}
	result.Limitations[0] += "x"
	for _, jsonOut := range []bool{false, true} {
		var out bytes.Buffer
		if err := renderUnityReferences(&out, result, jsonOut); err == nil || out.Len() != 0 {
			t.Fatalf("overbudget result leaked positive output: bytes%d err%v", out.Len(), err)
		}
	}
}

func TestUnityReferencesHelpUsesRegisteredPublicVerb(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := run([]string{"help", "unity-references"}, &out, &stderr); code != 0 {
		t.Fatalf("help not registered: code%d out%q err%q", code, out.String(), stderr.String())
	}
	combined := out.String() + stderr.String()
	for _, flag := range []string{"-service", "-receipt", "-active", "-editor-sha256", "-file-id", "-store", "-json"} {
		if !strings.Contains(combined, flag) {
			t.Fatalf("missing help flag %s: %s", flag, combined)
		}
	}
}
