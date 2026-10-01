package main

import (
	"testing"

	_ "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	"gopkg.in/yaml.v3"
)

func TestWrapUnknown(t *testing.T) {
	in := map[string]any{"a": map[string]any{"@type": "type.googleapis.com/envoy.extensions.nope.v3.Nope", "x": 1.0},
		"b": []any{map[string]any{"@type": "type.googleapis.com/envoy.config.cluster.v3.Cluster", "name": "c"}}}
	out, n := wrapUnknown(in)
	if n != 1 {
		t.Fatalf("wrapped %d, want 1", n)
	}
	a := out.(map[string]any)["a"].(map[string]any)
	if a["@type"] != "type.googleapis.com/google.protobuf.Struct" {
		t.Errorf("unknown type not wrapped: %v", a)
	}
}

func TestNodeAtLineConvention(t *testing.T) {
	var n yaml.Node
	if err := yaml.Unmarshal([]byte("a:\n  route_config:\n    name: r\n  items:\n  - name: x\n"), &n); err != nil {
		t.Fatal(err)
	}
	if node, line := nodeAt(&n, []seg{{"a", -1}, {"route_config", -1}}); node == nil || line != 2 {
		t.Errorf("a message in a field is at its key's line: got %d", line)
	}
	if node, line := nodeAt(&n, []seg{{"a", -1}, {"items", 0}}); node == nil || line != 5 {
		t.Errorf("a list item is at its own first line: got %d", line)
	}
	if node, _ := nodeAt(&n, []seg{{"a", -1}, {"missing", -1}}); node != nil {
		t.Error("a missing field has no node")
	}
}
