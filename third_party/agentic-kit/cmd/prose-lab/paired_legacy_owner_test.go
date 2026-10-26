package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

// These native/preparation hashes were published before the paired code existed.
// The oracle is the retained real study, not a newly collected request.
func TestOwnerPairedLegacyWireCompatibility(t *testing.T) {
	root := filepath.Join("..", "..", "docs", "evidence", "prose-headless", "2026-10-25-support-live")
	var pins []struct {
		ID       string `json:"case_id"`
		Prepared string `json:"prepared_digest"`
		Native   string `json:"native_request_digest"`
	}
	if err := json.Unmarshal(mustReadDocumentTest(t, filepath.Join(root, "preparations.json")), &pins); err != nil {
		t.Fatal(err)
	}
	if len(pins) != 67 {
		t.Fatal("published legacy denominator changed")
	}
	seen := map[string]bool{}
	for _, p := range pins {
		if seen[p.ID] {
			t.Fatal("duplicate published case")
		}
		seen[p.ID] = true
		args := []string{"review-prepare", "--study", filepath.Join(root, "study.json"), "--corpus", "testdata/document-corpus-v2", "--goal-scope", "testdata/document-scopes/v2-length-goals.json", "--goal-scope-digest", frozenV2GoalScope, "--review-scope", "testdata/document-scopes/v2-support-review.json", "--review-scope-digest", ownerReviewPin, "--review-lane", "review", "--case", p.ID}
		var out, diag bytes.Buffer
		if code := runLab(context.Background(), args, &out, &diag); code != 0 {
			t.Fatalf("legacy %s: code%d %s", p.ID, code, diag.String())
		}
		var got struct {
			Prepared string `json:"prepared_digest"`
			Native   string `json:"native_request_digest"`
		}
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Prepared != p.Prepared || got.Native != p.Native {
			t.Fatalf("old request or preparation changed for %s", p.ID)
		}
	}
}
