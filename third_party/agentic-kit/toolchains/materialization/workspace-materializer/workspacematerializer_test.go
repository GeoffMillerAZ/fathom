package workspacematerializer_test

import (
	"testing"

	wm "github.com/geoffmilleraz/fathom/third_party/agentic-kit/toolchains/materialization/workspace-materializer"
	wqb "github.com/geoffmilleraz/fathom/third_party/agentic-kit/toolchains/materialization/workspace-query-builder"
)

// buildTestPack returns a small but complete ParquetQueryPack.
func buildTestPack() wqb.ParquetQueryPack {
	query := wqb.SelectEntity("finding").
		WithJoin("evidence", "finding_id").
		WithFilter(wqb.FilterExpr{Entity: "finding", Field: "severity", Op: wqb.FilterOpGt, Value: "1"}).
		WithLimit(100)

	descriptor := wqb.ReadsetDescriptor{
		ReadsetID: "rs-001",
		Datasets: []wqb.DatasetEntry{
			{Alias: "finding", Path: "/data/findings.parquet"},
			{Alias: "evidence", Path: "/data/evidence.parquet"},
		},
	}

	return wqb.ParquetQueryPack{
		PackID:     "gap-analysis",
		Descriptor: descriptor,
		Query:      query,
	}
}

// buildTestData returns in-memory rows that satisfy the pack's join key.
func buildTestData() map[string][]map[string]string {
	return map[string][]map[string]string{
		"finding": {
			{"finding.finding_id": "f1", "finding.severity": "3", "finding.id": "F-001"},
			{"finding.finding_id": "f2", "finding.severity": "2", "finding.id": "F-002"},
		},
		"evidence": {
			{"evidence.finding_id": "f1", "evidence.detail": "log spike"},
			{"evidence.finding_id": "f2", "evidence.detail": "alert"},
		},
	}
}

// TestMaterialize_ReceiptContents verifies that the receipt is populated with
// the expected fields from the pack and data.
func TestMaterialize_ReceiptContents(t *testing.T) {
	pack := buildTestPack()
	data := buildTestData()

	receipt, err := wm.Materialize(pack, data)
	if err != nil {
		t.Fatalf("Materialize returned unexpected error: %v", err)
	}

	if receipt.PackID != pack.PackID {
		t.Errorf("PackID: got %q, want %q", receipt.PackID, pack.PackID)
	}
	if receipt.ReadsetID != pack.Descriptor.ReadsetID {
		t.Errorf("ReadsetID: got %q, want %q", receipt.ReadsetID, pack.Descriptor.ReadsetID)
	}
	if len(receipt.Datasets) != 2 {
		t.Errorf("Datasets length: got %d, want 2", len(receipt.Datasets))
	}
	if receipt.RowCounts["finding"] != 2 {
		t.Errorf("RowCounts[finding]: got %d, want 2", receipt.RowCounts["finding"])
	}
	if receipt.RowCounts["evidence"] != 2 {
		t.Errorf("RowCounts[evidence]: got %d, want 2", receipt.RowCounts["evidence"])
	}
	if len(receipt.Hash) != 16 {
		t.Errorf("Hash length: got %d, want 16 hex chars", len(receipt.Hash))
	}
	if receipt.At.IsZero() {
		t.Error("At timestamp must not be zero")
	}
}

// TestMaterialize_Determinism calls Materialize twice with identical inputs and
// verifies the Hash fields are identical.  This is the core determinism probe.
func TestMaterialize_Determinism(t *testing.T) {
	pack := buildTestPack()
	data := buildTestData()

	r1, err := wm.Materialize(pack, data)
	if err != nil {
		t.Fatalf("first Materialize: %v", err)
	}
	r2, err := wm.Materialize(pack, data)
	if err != nil {
		t.Fatalf("second Materialize: %v", err)
	}

	if r1.Hash != r2.Hash {
		t.Errorf("Hash not deterministic: %q != %q", r1.Hash, r2.Hash)
	}
	if r1.PackID != r2.PackID {
		t.Errorf("PackID not deterministic: %q != %q", r1.PackID, r2.PackID)
	}
	if r1.ReadsetID != r2.ReadsetID {
		t.Errorf("ReadsetID not deterministic: %q != %q", r1.ReadsetID, r2.ReadsetID)
	}
}

// TestMaterialize_DifferentPacksDifferentHash verifies that structurally
// different packs (different SQL) produce different hashes.
func TestMaterialize_DifferentPacksDifferentHash(t *testing.T) {
	pack1 := buildTestPack()

	// Pack 2: no join, no filter — different SQL → different hash.
	pack2 := wqb.ParquetQueryPack{
		PackID: "simple",
		Descriptor: wqb.ReadsetDescriptor{
			ReadsetID: "rs-002",
			Datasets: []wqb.DatasetEntry{
				{Alias: "finding", Path: "/data/findings.parquet"},
			},
		},
		Query: wqb.SelectEntity("finding"),
	}

	data1 := buildTestData()
	data2 := map[string][]map[string]string{
		"finding": data1["finding"],
	}

	r1, err := wm.Materialize(pack1, data1)
	if err != nil {
		t.Fatalf("pack1 Materialize: %v", err)
	}
	r2, err := wm.Materialize(pack2, data2)
	if err != nil {
		t.Fatalf("pack2 Materialize: %v", err)
	}

	if r1.Hash == r2.Hash {
		t.Errorf("expected different hashes for different packs, but both are %q", r1.Hash)
	}
}

// TestMaterialize_HashIs16HexChars verifies the hash format contract.
func TestMaterialize_HashIs16HexChars(t *testing.T) {
	pack := buildTestPack()
	receipt, err := wm.Materialize(pack, buildTestData())
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	for i, c := range receipt.Hash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Hash[%d] = %q is not a lowercase hex digit", i, c)
		}
	}
}

// TestMaterialize_InvalidPackReturnsError verifies graceful error propagation.
func TestMaterialize_InvalidPackReturnsError(t *testing.T) {
	// A pack with an empty query has no entities — EmitSQL will fail.
	pack := wqb.ParquetQueryPack{
		PackID: "bad",
		Descriptor: wqb.ReadsetDescriptor{
			ReadsetID: "rs-bad",
		},
		Query: wqb.WorkspaceQuery{}, // no entities
	}
	_, err := wm.Materialize(pack, nil)
	if err == nil {
		t.Fatal("expected an error for an invalid pack, got nil")
	}
}
