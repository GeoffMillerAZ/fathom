package extractor

import (
	"strings"
	"testing"
	"testing/fstest"
)

// TestVariantTagsQuery (O732, LB-F385): a variant's own tags query
// (variants[].queries.tags) is compiled before its dialect's for the variant's
// extensions alone. TypeScript's tsx variant holds the React patterns only JSX
// can satisfy — a component recognised by the JSX its body writes, a JSX
// element as a call — so a .ts file, which cannot write JSX, never runs them:
// a PascalCase function whose string holds `</` is a function in .ts and a
// component in .tsx, and a .tsx element is a call. The patterns any TypeScript
// file can satisfy (hooks, props, a component a wrapper call or an alias
// makes, a class component) stay in the dialect's query for both.
func TestVariantTagsQuery(t *testing.T) {
	reg, err := tsRegistry()
	if err != nil {
		t.Fatal(err)
	}
	group := reg["javascript"]
	ts, tsx := group.variantForExt[".ts"].rules, group.variantForExt[".tsx"].rules
	for _, ext := range []string{".mts", ".cts"} {
		if group.variantForExt[ext].rules != ts {
			t.Errorf("%s does not read the dialect's own query", ext)
		}
	}
	if ts == tsx || !strings.HasSuffix(tsx.query, ts.query) || len(tsx.query) <= len(ts.query) {
		t.Fatalf("the tsx variant's query is not its own patterns before the dialect's")
	}
	if strings.Contains(ts.query, `"</|/>")) @definition.component`) {
		t.Error("the .ts query names a component by the JSX its body writes")
	}
	if tsx.queryFile != "dialects/typescript/tags.tsx.scm + tags.scm" {
		t.Errorf("tsx query file = %q", tsx.queryFile)
	}

	e, err := NewTSExtractor("javascript")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	src := "export function Banner(): string {\n  return \"</b>\";\n}\n"
	kindOf := func(path, src, name string) (string, []string) {
		nodes, err := e.Extract("svc", path, []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range nodes {
			if n.Name == name {
				return string(n.Kind), n.Calls
			}
		}
		return "", nil
	}
	if k, _ := kindOf("src/banner.ts", src, "Banner"); k != "JSFunc" {
		t.Errorf("Banner in .ts = %q, want JSFunc", k)
	}
	if k, _ := kindOf("src/banner.tsx", src, "Banner"); k != "JSComponent" {
		t.Errorf("Banner in .tsx = %q, want JSComponent", k)
	}
	if k, calls := kindOf("src/page.tsx", "export function Page() {\n  return <Banner title=\"x\" />;\n}\n", "Page"); k != "JSComponent" || len(calls) != 1 || calls[0] != "Banner" {
		t.Errorf("Page in .tsx = %q calling %v, want JSComponent calling [Banner]", k, calls)
	}
}

// TestLoadDialectsRefusesMissingVariantQuery: a variant naming a tags query
// its directory does not hold is refused at load, as the dialect's own is.
func TestLoadDialectsRefusesMissingVariantQuery(t *testing.T) {
	manifest := `{"id":"a","display":"X","kind":"language","engine":"tags","lang_tag":"x",
"match":{"extensions":[".a"]},"grammar":{"blob":"x","entry_point":"tree_sitter_x"},
"variants":[{"id":"b","entry_point":"tree_sitter_b","extensions":[".b"],"queries":{"tags":"tags.b.scm"}}],
"queries":{"tags":"tags.scm"},"kinds":{"f":{"source_kind":"AF","canonical":"function"}},
"required_kinds":["f"],"target_level":"L1","queue_id":"a"}`
	fsys := fstest.MapFS{
		"dialects/a/dialect.json": &fstest.MapFile{Data: []byte(manifest)},
		"dialects/a/tags.scm":     &fstest.MapFile{Data: []byte("")},
	}
	if _, err := loadDialects(fsys); err == nil || !strings.Contains(err.Error(), "variant b: query tags.b.scm") {
		t.Fatalf("err = %v, want the variant's missing query refused", err)
	}
	fsys["dialects/a/tags.b.scm"] = &fstest.MapFile{Data: []byte("")}
	if _, err := loadDialects(fsys); err != nil {
		t.Fatalf("with the variant's query present: %v", err)
	}
}
