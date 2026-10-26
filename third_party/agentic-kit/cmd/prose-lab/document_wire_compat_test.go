package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// These fingerprints are obtained from the unchanged 892b9f5 command against
// the same local libraries. They freeze actual v1 preparation and held receipt
// bytes, rather than recalculate an expected value with the new implementation.
const v1PreparedWireHash = "f25ca4ef485fa2e0c8744a7b2e6518ccc4acbf77ac6862e36202b638988433e9"
const v1HeldWireHash = "2d22326b939446c95e4996898b8133bcaa6944e1a649b7a7d7886ef69371ff33"

func TestDocumentV1FrozenWireCompatibility(t *testing.T) {
	c, err := loadDocumentCorpus("testdata/document-corpus")
	if err != nil {
		t.Fatal(err)
	}
	args := documentArgs(documentTestStudy(t, c), "testdata/document-corpus", "freeform-brief-technical-r1")
	var out, errOut bytes.Buffer
	if code := runDocument(context.Background(), args, &out, &errOut); code != 0 {
		t.Fatalf("v1 prepare: %d %s", code, errOut.String())
	}
	if got := hashBytes(out.Bytes()); got != v1PreparedWireHash {
		t.Errorf("v1 prepared byte hash=%s want=%s", got, v1PreparedWireHash)
	}
	var p map[string]any
	if err := json.Unmarshal(out.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	artifacts := filepath.Join(root, "artifacts")
	capture := append([]string{}, args...)
	capture[0] = "document-capture"
	capture = append(capture, "--prepared-digest", p["prepared_digest"].(string), "--live", "--journal", filepath.Join(root, "journal"), "--artifacts", artifacts)
	spy := &documentSpy{} // Empty synthetic response forces a hold, never a provider call.
	now := func() time.Time { return time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC) }
	if code := runDocumentWith(context.Background(), capture, &bytes.Buffer{}, &bytes.Buffer{}, documentDeps{generation: spy, review: spy, now: now}); code != 2 || spy.calls != 1 {
		t.Fatalf("v1 hold changed: code=%d calls=%d", code, spy.calls)
	}
	b, err := os.ReadFile(filepath.Join(artifacts, "freeform-brief-technical-r1.receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := hashBytes(b); got != v1HeldWireHash {
		t.Errorf("v1 held receipt byte hash=%s want=%s", got, v1HeldWireHash)
	}
}
