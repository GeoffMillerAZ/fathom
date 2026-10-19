package assetrefs_test

import (
	"context"
	"errors"
	"github.com/geoffmilleraz/fathom/internal/assetrefs"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
	"reflect"
	"testing"
)

func TestUnityAssetReferenceVendorMetadataPublicClientPreservesCodeGraph(t *testing.T) {
	local, query, mutate := assetrefs.NewVendorMetadataFixtureForExternalTest(t)
	ctx := context.Background()
	memory := store.NewMemoryStore()
	c := client.New(nil, memory)
	defer c.Close()
	if err := memory.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: local.Service, Path: local.Root}); err != nil {
		t.Fatal(err)
	}
	observations := []core.Observation{{ServiceID: local.Service, SourceRef: "code.go:1", SourceKind: core.KindGoFunc, EvidenceTier: core.TierPreScan, Symbol: "Independent", Claim: "independent code sentinel", Line: 1, EndLine: 1}}
	typed := []core.Edge{{FromService: local.Service, FromFile: "code.go", FromSymbol: "Independent", ToSymbol: "Callee", Kind: core.EdgeDirect}}
	if err := memory.PutObservations(ctx, observations); err != nil {
		t.Fatal(err)
	}
	if err := memory.PutTypedEdges(ctx, local.Service, "scip:independent", typed); err != nil {
		t.Fatal(err)
	}
	before, err := memory.QueryObservations(ctx, local.Service, "")
	if err != nil {
		t.Fatal(err)
	}
	opts := client.UnityAssetReferenceOptions{Project: local.Project, FamilyID: local.FamilyID, ReceiptPath: local.ReceiptPath, ActivePath: local.ActivePath, ExpectedEditorVersion: local.ExpectedEditorVersion, ExpectedEditorSHA256: local.ExpectedEditorSHA256, ExpectedBuildTarget: local.ExpectedBuildTarget}
	initial, err := c.UnityAssetReferences(ctx, local.Service, opts, query)
	if err != nil {
		t.Fatal(err)
	}
	if len(initial.References) != 1 {
		t.Fatalf("fixture did not begin with actual public positive links: %#v", initial)
	}
	mutate()
	stale, queryErr := c.UnityAssetReferences(ctx, local.Service, opts, query)
	after, err := memory.QueryObservations(ctx, local.Service, "")
	if err != nil {
		t.Fatal(err)
	}
	afterTyped, err := memory.TypedEdges(ctx, local.Service)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(typed, afterTyped) {
		t.Fatal("asset metadata mutation/read disturbed independent code or typed producer")
	}
	if !errors.Is(queryErr, client.ErrAssetReferencesRefused) || !reflect.DeepEqual(stale, client.AssetReferenceResult{}) {
		t.Fatalf("metadata-only identity drift retained public current links: %#v %v", stale, queryErr)
	}
}
