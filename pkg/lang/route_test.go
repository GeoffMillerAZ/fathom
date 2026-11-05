package lang_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// A pack or Go extractor claims files after every specific built-in claim and
// before an extension's generic reader (design section 6). These tests hold
// that order at each place the router makes a claim.

// contentClaimed are JSON documents the router gives to a dialect by what they
// hold: no name or extension says so. A pack glob that matches them must lose.
var contentClaimed = map[string]string{
	"cfn.json":         `{"AWSTemplateFormatVersion":"2010-09-09","Resources":{"B":{"Type":"AWS::S3::Bucket","Properties":{}}}}`,
	"api.json":         `{"openapi":"3.0.0","info":{"title":"t","version":"1"},"paths":{"/a":{"get":{"operationId":"getA","responses":{}}}}}`,
	"dash.json":        `{"title":"D","uid":"abc","panels":[{"id":1,"title":"P","type":"graph","targets":[{"expr":"up"}]}],"schemaVersion":30}`,
	"arm.json":         `{"$schema":"https://schema.management.azure.com/schemas/2019-04-01/deploymentTemplate.json#","contentVersion":"1.0.0.0","resources":[{"type":"Microsoft.Storage/storageAccounts","apiVersion":"2019-06-01","name":"s"}]}`,
	"user.schema.json": `{"$schema":"http://json-schema.org/draft-07/schema#","type":"object","properties":{"a":{"type":"string"}}}`,
	"package.json":     `{"name":"web","version":"1.0.0"}`,
}

func TestPackLayerComesAfterContentClaims(t *testing.T) {
	plain := newReader(t)
	m := lang.Match{ID: "aaa-rows", Globs: []string{"**/*.json"}, Kinds: map[string]string{"RowDef": "function"}}
	r := newReader(t, lang.WithExtractor(m, &rowExtractor{}))
	for path, src := range contentClaimed {
		want, got := extract(t, plain, path, src), extract(t, r, path, src)
		if want.Dialect == "" || want.Dialect == "aaa-rows" {
			t.Fatalf("setup: the built-ins read %s as %q, so this case proves nothing", path, want.Dialect)
		}
		if got.Dialect != want.Dialect {
			t.Errorf("%s read by %q, want the built-in %q: the pack layer ran ahead of a content claim", path, got.Dialect, want.Dialect)
		}
	}
	// The glob works on a JSON file no built-in settles by content.
	if got := extract(t, r, "data/other.json", `{"name":"Clock"}`); got.Dialect != "aaa-rows" {
		t.Errorf("data/other.json read by %q, want aaa-rows", got.Dialect)
	}
}

func TestPackClaimsAYAMLFileNoBuiltInSettles(t *testing.T) {
	plain := newReader(t)
	if got := extract(t, plain, "x/a.rows.yaml", "a: 1\n"); got.Skipped != lang.SkipUnclassified {
		t.Fatalf("setup: the built-ins read x/a.rows.yaml as %q skipped %q", got.Dialect, got.Skipped)
	}
	m := lang.Match{ID: "aaa-rows", Globs: []string{"**/*.rows.yaml"}, Kinds: map[string]string{"RowDef": "function"}}
	r := newReader(t, lang.WithExtractor(m, &rowExtractor{}))
	if got := extract(t, r, "x/a.rows.yaml", "def Main\n"); got.Dialect != "aaa-rows" || got.Skipped != "" || len(got.Symbols) != 1 {
		t.Fatalf("YAML file the pack claims: dialect %q skipped %q symbols %d", got.Dialect, got.Skipped, len(got.Symbols))
	}
	// A YAML file a built-in classifies stays the built-in's.
	k8s := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: c\n"
	m.Globs = []string{"**/*.yaml"}
	r = newReader(t, lang.WithExtractor(m, &rowExtractor{}))
	if got, want := extract(t, r, "k.yaml", k8s), extract(t, plain, "k.yaml", k8s); got.Dialect != want.Dialect {
		t.Fatalf("k.yaml read by %q, want %q", got.Dialect, want.Dialect)
	}
}

// blockingExtractor holds every read until release is closed.
type blockingExtractor struct {
	entered chan struct{}
	release chan struct{}
	done    atomic.Int32
}

func (b *blockingExtractor) Extract(context.Context, lang.File) (lang.Extracted, error) {
	b.entered <- struct{}{}
	<-b.release
	b.done.Add(1)
	return lang.Extracted{}, nil
}

func TestCloseWaitsForExtractionsInFlight(t *testing.T) {
	b := &blockingExtractor{entered: make(chan struct{}, 1), release: make(chan struct{})}
	r, err := lang.New(lang.WithExtractor(lang.Match{ID: "block-rows", Extensions: []string{".blk"}, Kinds: map[string]string{"RowDef": "function"}}, b))
	if err != nil {
		t.Fatal(err)
	}
	extractDone := make(chan error, 1)
	go func() {
		_, err := r.Extract(context.Background(), lang.File{Path: "a.blk", Src: []byte("x")})
		extractDone <- err
	}()
	<-b.entered
	closed := make(chan struct{})
	go func() { r.Close(); close(closed) }()
	select {
	case <-closed:
		t.Fatal("Close returned while an extraction was in flight")
	case <-time.After(150 * time.Millisecond):
	}
	close(b.release)
	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("Close did not return after the extraction finished")
	}
	if err := <-extractDone; err != nil {
		t.Fatalf("the extraction in flight failed: %v", err)
	}
	if b.done.Load() != 1 {
		t.Fatal("the extraction in flight did not finish")
	}
	if _, err := r.Extract(context.Background(), lang.File{Path: "a.blk", Src: []byte("x")}); err != lang.ErrClosed {
		t.Fatalf("Extract after Close: %v, want ErrClosed", err)
	}
}
