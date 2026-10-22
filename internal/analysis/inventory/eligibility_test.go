package inventory

import "testing"

func TestAccumulatorAcceptsPathMatchesExtractorSurface(t *testing.T) {
	base := []struct {
		path string
		want bool
	}{
		{"asset.png", false}, {"README.md", false}, {"go.mod", false},
		{"api.go", true}, {"api_test.go", false}, {"schema.proto", true},
		{"app.jsx", true}, {"app.cjs", true}, {"config.yaml", true},
	}
	for _, tc := range base {
		if got := NewAccumulator(nil).AcceptsPath(tc.path); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.path, got, tc.want)
		}
	}
	for _, tc := range []struct {
		name     string
		patterns []Pattern
		path     string
		want     bool
		wantKey  string
	}{
		{"narrow match", []Pattern{{Kind: "asset", Glob: "**/*.png", Regex: `ID: (\w+)`}}, "assets/asset.png", true, "asset:widget"},
		{"narrow miss", []Pattern{{Kind: "asset", Glob: "**/*.png", Regex: `ID: (\w+)`}}, "asset.bin", false, ""},
		{"empty glob", []Pattern{{Kind: "asset", Regex: `ID: (\w+)`}}, "unknown.bin", true, "asset:widget"},
		{"invalid regex", []Pattern{{Kind: "asset", Regex: "["}}, "unknown.bin", false, ""},
		{"no capture", []Pattern{{Kind: "asset", Regex: "ID"}}, "unknown.bin", false, ""},
		{"no kind", []Pattern{{Regex: `ID: (\w+)`}}, "unknown.bin", false, ""},
		{"test excluded", []Pattern{{Kind: "asset", Regex: `ID: (\w+)`}}, "api_test.go", false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewAccumulator(tc.patterns)
			if got := a.AcceptsPath(tc.path); got != tc.want {
				t.Fatalf("AcceptsPath(%q)=%v, want %v", tc.path, got, tc.want)
			}
			content := []byte("ID: widget\n")
			a.Add(File{Path: tc.path, Content: content})
			items := a.Items()
			if tc.wantKey == "" {
				if len(items) != 0 {
					t.Fatalf("unexpected items: %+v", items)
				}
			} else if len(items) != 1 || items[0].Key != tc.wantKey || items[0].Kind != "asset" {
				t.Fatalf("configured item: got %+v, want %s kind asset", items, tc.wantKey)
			}
		})
	}
}

func TestAccumulatorEligibleBuiltinsRetainFacts(t *testing.T) {
	files := []File{
		{Path: "api/api.go", Content: []byte("package api\nfunc Exported() {}\n")},
		{Path: "schema.proto", Content: []byte("syntax = \"proto3\"; package svc; service API { rpc Call (Req) returns (Resp); }\n")},
		{Path: "app.js", Content: []byte("fastify.get('/users', listUsers)\nconst token = process.env.API_KEY\n")},
	}
	a := NewAccumulator(nil)
	for _, f := range files {
		if !a.AcceptsPath(f.Path) {
			t.Fatalf("eligible builtin skipped: %s", f.Path)
		}
		a.Add(f)
	}
	want := map[string]string{
		"api:api.Exported": "api",
		"rpc:svc.API/Call": "rpc",
		"route:GET /users": "route",
		"env:API_KEY":      "env",
	}
	for _, item := range a.Items() {
		if want[item.Key] != item.Kind {
			t.Fatalf("unexpected or mistyped item: %+v", item)
		}
		delete(want, item.Key)
	}
	if len(want) != 0 {
		t.Fatalf("eligible builtin facts missing: %v", want)
	}
}
