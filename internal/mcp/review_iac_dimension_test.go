package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/iacfacts"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

type laneIaCDatedBackend struct{ reviewFakeBackend }

func (b *laneIaCDatedBackend) ObligationsAsOf(context.Context, string, string, int, time.Time) (semantic.Obligations, error) {
	return semantic.Obligations{}, nil
}
func (b *laneIaCDatedBackend) ListCommits(context.Context, string) ([]mcp.CommitInfo, error) {
	return []mcp.CommitInfo{{Hash: "abcdef0123456789", Date: "2026-11-01T00:00:00Z"}}, nil
}

func laneIaCDimension(t *testing.T, p map[string]any, checked bool) map[string]any {
	t.Helper()
	dims, ok := p["dimensions"].([]any)
	if !ok {
		t.Fatalf("missing dimensions: %v", p)
	}
	if len(dims) != 9 {
		t.Errorf("got %d dimensions, want existing eight plus iac", len(dims))
	}
	checkedCount := 0
	for _, v := range dims {
		if v.(map[string]any)["checked"] == true {
			checkedCount++
		}
	}
	summary, _ := p["summary"].(string)
	if !strings.Contains(summary, fmt.Sprintf("%d/9 dimensions checked", checkedCount)) || !strings.Contains(summary, "core active finding(s)") || !strings.Contains(summary, "IaC finding(s)") {
		t.Errorf("inconsistent summary: %q", summary)
	}
	for _, v := range dims {
		d := v.(map[string]any)
		if d["name"] != "iac" {
			continue
		}
		if d["checked"] != checked {
			t.Errorf("checked = %v, want %v: %v", d["checked"], checked, d)
		}
		reason, _ := d["reason"].(string)
		if checked && reason != "" || !checked && reason == "" {
			t.Errorf("inconsistent checked/reason: %v", d)
		}
		if _, ok := d["findings"].([]any); !ok {
			t.Fatalf("findings must be an array: %v", d)
		}
		return d
	}
	t.Fatal("registered review_diff omitted iac dimension")
	return nil
}

// A catalog without nested block rules permits a complete check with current readers.
func laneIaCSimpleHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("FATHOM_HOME", home)
	file := iacTestExport(t)
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	provider := schema["provider_schemas"].(map[string]any)[iacTestSource].(map[string]any)
	block := provider["resource_schemas"].(map[string]any)["widget_thing"].(map[string]any)["block"].(map[string]any)
	delete(block, "block_types")
	raw, err = json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := iacfacts.ImportIaCExports(filepath.Join(home, "iac"), []iacfacts.IaCImportInput{{Version: "1.2.0", Path: file}}, iacfacts.IaCWriterOptions{Tool: "test"}); err != nil {
		t.Fatal(err)
	}
}

// Keep legacy-equivalence comparisons independent of the response cap. The
// existing capped IaC tests and lane tests continue to use the default budget.
func laneIaCUncappedCall(t *testing.T, srv *mcp.Server, tool string, args map[string]any) (map[string]any, int) {
	t.Helper()
	raw, err := json.Marshal(request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: map[string]any{"name": tool, "arguments": args}})
	if err != nil {
		t.Fatal(err)
	}
	response, ok := srv.HandleRequest(mcp.WithoutReviewBudgetForTest(context.Background()), raw)
	if !ok {
		t.Fatal("no response")
	}
	var p map[string]any
	if err := json.Unmarshal(response, &p); err != nil {
		t.Fatal(err)
	}
	result, _ := json.Marshal(p["result"])
	return extToolPayload(t, p), len(result)
}

func laneWithoutIaCDimension(p map[string]any) map[string]any {
	out := stripVolatile(p)
	delete(out, "summary") // The summary now includes the additive dimension and separate IaC count.
	if dims, ok := out["dimensions"].([]any); ok {
		retained := []any{}
		for _, value := range dims {
			if value.(map[string]any)["name"] != "iac" {
				retained = append(retained, value)
			}
		}
		out["dimensions"] = retained
	}
	return out
}

func TestLaneHazardReviewDimension(t *testing.T) {
	review := func(t *testing.T, root string, files []string, checked bool) (map[string]any, map[string]any) {
		t.Helper()
		p, size := iacCall(t, iacServer(root), "review_diff", map[string]any{"service": "svc", "changed_files": files})
		if size > 16000 {
			t.Fatalf("response exceeds byte cap: %d", size)
		}
		return p, laneIaCDimension(t, p, checked)
	}
	t.Run("bound facts and references", func(t *testing.T) {
		laneIaCSimpleHome(t)
		root := iacRepo(t, map[string]string{"main.tf": iacProviders + `resource "widget_thing" "a" {
 name = "secret-value-canary"
 old = "x"
}
`, "second.tf": ""})
		p, d := review(t, root, []string{"second.tf", "main.tf"}, true)
		if d["as_of"] != "" {
			t.Fatalf("live as_of: %v", d)
		}
		findings := d["findings"].([]any)
		if len(findings) == 0 {
			t.Fatal("bound catalog produced no findings")
		}
		for _, v := range findings {
			f := citedFact(t, v.(map[string]any)["fact"])
			got, err := iacfacts.ReadIaCCatalog(filepath.Join(os.Getenv("FATHOM_HOME"), "iac", f["catalog"].(string)), iacfacts.IaCCatalogRequest{Version: f["version"].(string), Kind: iacfacts.SectionResource, Type: "widget_thing", Path: strings.Split(f["path"].(string), ".")})
			if err != nil || got.Digest != f["digest"] {
				t.Fatalf("unresolvable reference %v: %v", f, err)
			}
		}
		if !reflect.DeepEqual(findings, iacBlock(t, p)["findings"]) {
			t.Fatal("legacy findings changed")
		}
		_, reordered := review(t, root, []string{"main.tf", "second.tf", "main.tf"}, true)
		if !reflect.DeepEqual(d, reordered) {
			t.Fatal("dimension depends on input order or duplicates")
		}
		raw, _ := json.Marshal(p)
		if strings.Contains(string(raw), "secret-value-canary") {
			t.Fatal("source value leaked")
		}
	})
	t.Run("complete clean read", func(t *testing.T) {
		laneIaCSimpleHome(t)
		_, d := review(t, iacRepo(t, map[string]string{"main.tf": iacProviders + "resource \"widget_thing\" \"a\" { name = \"secret-value-canary\" }\n"}), []string{"main.tf"}, true)
		if len(d["findings"].([]any)) != 0 {
			t.Fatalf("clean read: %v", d)
		}
	})
	t.Run("dropped resource is incomplete", func(t *testing.T) {
		laneIaCSimpleHome(t)
		source := iacProviders + `resource "widget_thing" "a" { name = "x" }
check "health" {
 data "widget_thing" "probe" {}
}
`
		review(t, iacRepo(t, map[string]string{"main.tf": source}), []string{"main.tf"}, false)
	})
	t.Run("source read cap", func(t *testing.T) {
		laneIaCSimpleHome(t)
		review(t, iacRepo(t, map[string]string{"main.tf": strings.Repeat(" ", iacfacts.MaxFileBytes+1)}), []string{"main.tf"}, false)
	})
	t.Run("capped findings stay unchecked", func(t *testing.T) {
		laneIaCSimpleHome(t)
		var source strings.Builder
		source.WriteString(iacProviders)
		for i := 0; i < 100; i++ {
			fmt.Fprintf(&source, "resource \"widget_thing\" \"r%03d\" {\n name = \"x\"\n bogus = 1\n}\n", i)
		}
		p, d := review(t, iacRepo(t, map[string]string{"main.tf": source.String()}), []string{"main.tf"}, false)
		if d["count"] != float64(100) || len(d["findings"].([]any)) >= 100 {
			t.Fatalf("capped findings/count: %v", d)
		}
		status := p["dimension_status"].(map[string]any)["iac"].(map[string]any)
		if status["checked"] != false || status["unchecked_reason"] == "" {
			t.Fatalf("capped coverage: %v", status)
		}
	})
	t.Run("no catalog", func(t *testing.T) {
		iacHome(t, false)
		p, _ := review(t, iacRepo(t, map[string]string{"main.tf": iacMain}), []string{"main.tf"}, false)
		if _, ok := p["iac"]; ok {
			t.Fatal("legacy absent catalog payload changed")
		}
	})
	t.Run("corrupt catalog", func(t *testing.T) {
		iacHome(t, true)
		entries, err := os.ReadDir(filepath.Join(os.Getenv("FATHOM_HOME"), "iac"))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				if err := os.WriteFile(filepath.Join(os.Getenv("FATHOM_HOME"), "iac", e.Name()), []byte("invalid catalog"), 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
		review(t, iacRepo(t, map[string]string{"main.tf": iacMain}), []string{"main.tf"}, false)
	})
	for _, tc := range []struct{ name, file, body string }{
		{"missing source", "missing.tf", ""},
		{"incomplete syntax", "main.tf", iacProviders + "resource \"widget_thing\" \"a\" {\n"},
		{"unsupported module input", "main.tf.json", `{ "resource": {} }`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			iacHome(t, true)
			files := map[string]string{}
			if tc.body != "" {
				files[tc.file] = tc.body
			}
			review(t, iacRepo(t, files), []string{tc.file}, false)
		})
	}
	t.Run("unreadable source escape", func(t *testing.T) {
		iacHome(t, true)
		outside := iacRepo(t, map[string]string{"main.tf": iacMain})
		root := iacRepo(t, nil)
		if err := os.Symlink(filepath.Join(outside, "main.tf"), filepath.Join(root, "main.tf")); err != nil {
			t.Fatal(err)
		}
		review(t, root, []string{"main.tf"}, false)
	})
	t.Run("unknown root", func(t *testing.T) {
		iacHome(t, true)
		review(t, "", []string{"main.tf"}, false)
	})
	t.Run("unbound type", func(t *testing.T) {
		iacHome(t, true)
		review(t, iacRepo(t, map[string]string{"main.tf": iacProviders + `resource "widget_missing" "a" {}`}), []string{"main.tf"}, false)
	})
	t.Run("no relevant change", func(t *testing.T) {
		iacHome(t, true)
		p, d := review(t, iacRepo(t, map[string]string{"readme.md": "text"}), []string{"readme.md"}, false)
		if len(d["findings"].([]any)) != 0 {
			t.Fatal("unrelated findings")
		}
		if _, ok := p["iac"]; ok {
			t.Fatal("legacy unrelated payload changed")
		}
	})
	t.Run("module directory cap", func(t *testing.T) {
		iacHome(t, true)
		files := map[string]string{}
		changed := []string{}
		for i := 0; i < 33; i++ {
			f := fmt.Sprintf("m%02d/main.tf", i)
			files[f] = iacMain
			changed = append(changed, f)
		}
		review(t, iacRepo(t, files), changed, false)
	})
	t.Run("cloudformation", func(t *testing.T) {
		cfnHome(t, true)
		template := "AWSTemplateFormatVersion: \"2010-09-09\"\nResources:\n  Widget:\n    Type: Example::Widget::Thing\n    Properties:\n      Name: secret-value-canary\n      Colour: red\n"
		p, d := review(t, iacRepo(t, map[string]string{"stack.yaml": template}), []string{"stack.yaml"}, true)
		if len(d["findings"].([]any)) == 0 || iacBlock(t, p)["cloudformation"] == nil {
			t.Fatal("CFN projection missing")
		}
		review(t, iacRepo(t, nil), []string{"stack.yaml"}, false)
		review(t, iacRepo(t, map[string]string{"stack.yaml": strings.Repeat(" ", (2<<20)+1)}), []string{"stack.yaml"}, false)
	})
	t.Run("dated exact revision", func(t *testing.T) {
		iacHome(t, true)
		root := iacRepo(t, map[string]string{"main.tf": iacMain})
		for _, revision := range []string{"abcdef0", "2026-11-01T02:00:00+02:00"} {
			backend := &laneIaCDatedBackend{reviewFakeBackend: reviewFakeBackend{extendedFakeBackend: extendedFakeBackend{svcPath: root, services: []string{"svc"}}}}
			p, _ := iacCall(t, mcp.NewServer(backend), "review_diff", map[string]any{"service": "svc", "changed_files": []string{"main.tf"}, "as_of": revision})
			d := laneIaCDimension(t, p, false)
			if d["as_of"] != revision {
				t.Fatalf("as_of = %v, want exact %q", d["as_of"], revision)
			}
			if _, ok := p["iac"]; ok {
				t.Fatal("dated review gained current facts")
			}
		}
	})
	t.Run("dated revision survives registered status-only cap", func(t *testing.T) {
		iacHome(t, true)
		root := iacRepo(t, map[string]string{"main.tf": iacMain})
		// RFC3339 accepts fractional precision beyond nanoseconds. Keep the
		// supplied revision exactly even though the resolved instant is shorter.
		revision := "2026-11-01T00:00:00." + strings.Repeat("0", 3800) + "Z"
		backend := &laneIaCDatedBackend{reviewFakeBackend: reviewFakeBackend{extendedFakeBackend: extendedFakeBackend{svcPath: root, services: []string{"svc"}}}}
		p, size := iacCall(t, mcp.NewServer(backend), "review_diff", map[string]any{"service": "svc", "changed_files": []string{"main.tf"}, "as_of": revision})
		if size > 16000 {
			t.Fatalf("over cap: %d", size)
		}
		dims := p["dimensions"].([]any)
		if len(dims) != 0 {
			t.Fatalf("wanted status-only cap, retained %d dimensions (%d bytes, status=%v)", len(dims), size, p["dimension_status"])
		}
		status := p["dimension_status"].(map[string]any)["iac"].(map[string]any)
		checkedCount := 0
		for _, value := range p["dimension_status"].(map[string]any) {
			if value.(map[string]any)["checked"] == true {
				checkedCount++
			}
		}
		if !strings.Contains(p["summary"].(string), fmt.Sprintf("%d/9 dimensions checked", checkedCount)) {
			t.Fatalf("status-only summary inconsistent: %v", p["summary"])
		}
		if status["as_of"] != revision || status["checked"] != false || status["unchecked_reason"] == "" {
			t.Fatalf("revision or unchecked status lost: %v", status)
		}
	})
	t.Run("cancellation and deadline", func(t *testing.T) {
		iacHome(t, true)
		root := iacRepo(t, map[string]string{"main.tf": iacMain})
		raw, _ := json.Marshal(request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: map[string]any{"name": "review_diff", "arguments": map[string]any{"service": "svc", "changed_files": []string{"main.tf"}}}})
		for _, deadline := range []bool{false, true} {
			var ctx context.Context
			var cancel context.CancelFunc
			if deadline {
				ctx, cancel = context.WithDeadline(context.Background(), time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
			} else {
				ctx, cancel = context.WithCancel(context.Background())
				cancel()
			}
			response, ok := iacServer(root).HandleRequest(ctx, raw)
			cancel()
			if !ok || !strings.Contains(string(response), "error") {
				t.Fatalf("cancelled review did not refuse: %s", response)
			}
		}
	})
}
