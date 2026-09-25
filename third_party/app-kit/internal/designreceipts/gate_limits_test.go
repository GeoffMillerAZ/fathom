package designreceipts

import (
	"encoding/json"
	"fmt"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func gateLimitCUEAccepts(t *testing.T, value any, definition string) bool {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	ctx := cuecontext.New()
	return ctx.CompileBytes(body).Unify(ctx.CompileBytes(schema).LookupPath(cue.ParsePath(definition))).Validate(cue.Concrete(true)) == nil
}

func gateLimitBundle(count int) testBundle {
	b := newTestBundle()
	gate := b.contract.Gates["visual"]
	gate.Slots = make([]Slot, 0, count)
	result := b.receipt.Results["visual"]
	for _, evidence := range result.Evidence {
		delete(b.content, evidence.ID)
	}
	result.Evidence = make([]Evidence, 0, count)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("visual-evidence-%02d", i)
		a, body := testAsset(id, "visual evidence "+id)
		b.content[id] = body
		gate.Slots = append(gate.Slots, Slot{ID: id, Role: "image"})
		result.Evidence = append(result.Evidence, Evidence{Asset: a, Role: "image"})
	}
	b.contract.Gates["visual"] = gate
	b.receipt.Results["visual"] = result
	return b
}

func TestGateEvidenceLimitMatchesCUEAndRuntime(t *testing.T) {
	limit, err := cuecontext.New().CompileBytes(schema).LookupPath(cue.ParsePath("#MaxGateEvidence")).Int64()
	if err != nil || limit != MaxGateEvidence {
		t.Fatalf("CUE/Go gate limit mismatch: CUE=%d Go=%d err=%v", limit, MaxGateEvidence, err)
	}

	for _, count := range []int{32, 33, 64} {
		t.Run(fmt.Sprintf("accept-%d", count), func(t *testing.T) {
			b := gateLimitBundle(count)
			contractBody, _, _ := sealTestBundle(t, b)
			b.receipt.ContractSHA256 = Digest(contractBody)
			b.receipt.SourceRevision = b.contract.Subject.SourceRevision
			b.receipt.ArtifactSHA256 = b.contract.Subject.ArtifactSHA256
			if !gateLimitCUEAccepts(t, b.contract, "#Contract") || !gateLimitCUEAccepts(t, b.receipt, "#Receipt") {
				t.Fatal("CUE rejected an in-bound gate or result directly")
			}
			report := checkTestBundle(t, b)
			if report.Verification != "not-verified" || len(report.IncompleteGates) != 0 {
				t.Fatalf("authored claims were promoted or lost: %#v", report)
			}
		})
	}

	t.Run("reject-65-contract", func(t *testing.T) {
		b := gateLimitBundle(65)
		if gateLimitCUEAccepts(t, b.contract, "#Contract") {
			t.Fatal("CUE accepted 65 gate slots")
		}
		if _, err := EncodeContract(b.contract); err == nil {
			t.Fatal("65 gate slots accepted")
		}
	})
	t.Run("reject-65-receipt", func(t *testing.T) {
		b := gateLimitBundle(65)
		gate := b.contract.Gates["visual"]
		gate.Slots = gate.Slots[:1]
		b.contract.Gates["visual"] = gate
		contractBody, err := EncodeContract(b.contract)
		if err != nil {
			t.Fatal(err)
		}
		b.receipt.ContractSHA256 = Digest(contractBody)
		b.receipt.SourceRevision = b.contract.Subject.SourceRevision
		b.receipt.ArtifactSHA256 = b.contract.Subject.ArtifactSHA256
		if gateLimitCUEAccepts(t, b.receipt, "#Receipt") {
			t.Fatal("CUE accepted 65 result evidence entries")
		}
		requireReceiptEncodeRejected(t, b)
	})
}

func TestGateEvidenceLimitDoesNotPromoteIncompleteClaims(t *testing.T) {
	b := gateLimitBundle(MaxGateEvidence)
	for _, name := range testGateNames {
		status, reason := "not-run", "No full-gate acceptance is retained."
		if name == "interaction" {
			status, reason = "failed", "A retained interaction observation failed."
		}
		if name == "visual" {
			result := b.receipt.Results[name]
			result.Status, result.Reason = status, reason
			b.receipt.Results[name] = result
			continue
		}
		for _, evidence := range b.receipt.Results[name].Evidence {
			delete(b.content, evidence.ID)
		}
		b.receipt.Results[name] = Result{Status: status, Reason: reason, Evidence: []Evidence{}}
	}
	report := checkTestBundle(t, b)
	if report.Verification != "not-verified" || len(report.IncompleteGates) != 9 || report.ClaimedStatuses["interaction"] != "failed" {
		t.Fatalf("incomplete claims were promoted: %#v", report)
	}
}
