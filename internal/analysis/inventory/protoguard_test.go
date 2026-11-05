package inventory

import (
	"testing"
	"time"
)

// emicklei/proto never returns for a file cut at an open bracket, and the surface
// inventory reads the .proto files of a repository, so it asks the extractor's
// guard first. A file the parser cannot finish contributes nothing.
func TestExtractProtoRPCReturnsForAFileCutAtABracket(t *testing.T) {
	done := make(chan []Item, 1)
	go func() {
		done <- extractProtoRPC(File{Path: "p/s.proto", Content: []byte("syntax=\"proto3\";\nservice s{ rpc a(b) returns (c) {")})
	}()
	select {
	case items := <-done:
		if len(items) != 0 {
			t.Errorf("a truncated file read: %v", items)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("extractProtoRPC did not return in 20s")
	}
	items := extractProtoRPC(File{Path: "p/s.proto", Content: []byte("syntax=\"proto3\";\nservice s{ rpc a(b) returns (c); }\n")})
	if len(items) != 1 {
		t.Errorf("a balanced file reads %d items, want 1: %v", len(items), items)
	}
}
