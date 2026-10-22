package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAD11ResultRangePointers(t *testing.T) {
	cases := []struct{ tool, payload string }{
		{"read_source", `{"file":"sample.go","lines":[{"line":2,"text":"func F() {}"}]}`},
		{"symbol_context", `{"def":{"file_path":"sample.go","line":2},"snippet":[{"line":2,"text":"func F() {}"}]}`},
	}
	for _, c := range cases {
		var result map[string]any
		if err := json.Unmarshal([]byte(c.payload), &result); err != nil {
			t.Fatal(err)
		}
		got := ResultRangePointers(c.tool, result)
		if len(got) != 1 || got[0].Path != "sample.go" || got[0].Start != 2 || got[0].End != 2 || got[0].Hash == "" {
			t.Errorf("%s: %v", c.tool, got)
		}
		if len(got) == 1 && !strings.HasPrefix(got[0].Ref(), "sample.go#L2-L2") {
			t.Error(got)
		}
	}
}
