package client

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestAssetReferencePreparationRequiresRecordedRootAndPreservesCodeEdges(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	client := &Client{st: st}
	options := UnityAssetReferenceOptions{Project: "game/Fixture", FamilyID: "cart", ReceiptPath: ".artifacts/receipt.json", ActivePath: ".artifacts/active.json", ExpectedEditorVersion: "fixture", ExpectedEditorSHA256: "1111111111111111111111111111111111111111111111111111111111111111", ExpectedBuildTarget: "StandaloneOSX"}
	if _, err := client.PrepareUnityAssetReferences(ctx, "missing", options); !errors.Is(err, ErrAssetReferencesRefused) {
		t.Fatalf("unregistered root accepted: %v", err)
	}
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "fixture", Path: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	edges := []core.Edge{{FromSymbol: "fixture::Caller", ToSymbol: "fixture::Target", Kind: core.EdgeDirect}}
	if err := st.PutTypedEdges(ctx, "fixture", "scip:independent", edges); err != nil {
		t.Fatal(err)
	}
	if _, err := client.PrepareUnityAssetReferences(ctx, "fixture", options); !errors.Is(err, ErrAssetReferencesRefused) {
		t.Fatalf("missing active proof accepted: %v", err)
	}
	if _, err := client.UnityAssetReferences(ctx, "fixture", options, AssetReferenceQuery{Object: AssetObjectID{GUID: "11111111111111111111111111111111", FileID: "1"}, Direction: "inbound"}); !errors.Is(err, ErrAssetReferencesRefused) {
		t.Fatalf("query invented proof: %v", err)
	}
	after, err := st.TypedEdges(ctx, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, edges) {
		t.Fatalf("asset refusal disturbed code edges: %#v", after)
	}
}
