package syslink

import (
	"testing"
	"time"
)

// emicklei/proto never returns for a file cut at an open bracket: the service
// links ask the extractor's guard first, and a file the parser cannot finish
// provides nothing.
func TestProtoProvidesReturnsForAFileCutAtABracket(t *testing.T) {
	done := make(chan []KeyRow, 1)
	go func() {
		done <- protoProvides("p/s.proto", []byte("syntax=\"proto3\";\nservice s{ rpc a(b) returns (c) {"))
	}()
	select {
	case rows := <-done:
		if len(rows) != 0 {
			t.Errorf("a truncated file provides %v", rows)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("protoProvides did not return in 20s")
	}
	if rows := protoProvides("p/s.proto", []byte("syntax=\"proto3\";\nservice s{ rpc a(b) returns (c); }\n")); len(rows) != 1 {
		t.Errorf("a balanced file provides %d rows, want 1: %v", len(rows), rows)
	}
}
