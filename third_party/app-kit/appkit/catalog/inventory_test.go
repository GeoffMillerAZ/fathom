package catalog

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestInventoryAnalyticsAppearanceAndTypedNavigation(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	analytics, ok := c.Get("analytics.comparison")
	if !ok {
		t.Fatal("implemented analytics composition is missing from discovery")
	}
	if analytics.Kind != "recipe" || analytics.Maturity != "experimental" || analytics.Preview != "/analytics/recipe" ||
		!slices.Equal(analytics.Layers, []string{"L3b"}) || !slices.Equal(analytics.Profiles, []string{"go-web"}) || len(analytics.Consumers) != 0 {
		t.Fatalf("analytics overstates promotion, preview or profile fit: %+v", analytics)
	}
	for _, phrase := range []string{"Dataset.Validate", "analytics.WriteCSV", "analytics.RenderBars", "analytics.RenderDivergingBars", "/analytics/signed", "Max is the absolute extent", "separate current/prior datasets", "rectangular CSV adapter", "appkit/ui/vizcheck/README.md", "independent fixture", "static-only evidence limits"} {
		if !strings.Contains(analytics.Usage, phrase) {
			t.Errorf("analytics integration omits %q", phrase)
		}
	}
	for _, phrase := range []string{"not a permanently promoted", "consumer adoption", "Browser task", "200 rows", "RenderBars still refuses negative values", "not a lossless", "do not verify geometry", "source truth", "post-update browser evidence"} {
		if !strings.Contains(strings.Join(analytics.Gaps, " "), phrase) {
			t.Errorf("analytics gap disclosure omits %q", phrase)
		}
	}
	appearance, ok := c.Get("shell.appearance")
	if !ok {
		t.Fatal("implemented portable appearance script is missing from discovery")
	}
	if appearance.Kind != "primitive" || appearance.Maturity != "available" || appearance.Preview != "/shell/settings" ||
		!slices.Equal(appearance.Layers, []string{"L2"}) || !slices.Equal(appearance.Profiles, []string{"portable", "go-web", "static-web", "react-web"}) || len(appearance.Consumers) != 0 {
		t.Fatalf("appearance overstates renderer/profile/preview fit: %+v", appearance)
	}
	for _, phrase := range []string{"appearance.Script", "appearance.Handler", "appearance.BootstrapScript", "separate Go L3b", "portable fit covers the L2 script only", "/shell/settings Gallery preview uses the optional Go L3b renderer"} {
		if !strings.Contains(appearance.Usage, phrase) {
			t.Errorf("appearance adoption boundary omits %q", phrase)
		}
	}
	if strings.Contains(strings.Join(appearance.Gaps, " "), "No appearance preview") {
		t.Fatal("appearance still denies its implemented Gallery preview")
	}
	shell, ok := c.Get("shell.layout")
	if !ok {
		t.Fatal("existing shell identity was removed")
	}
	for _, symbol := range []string{"dashboard.RenderNav", "dashboard.NavGroup", "dashboard.NavItem", "dashboard.MatchPath", "dashboard.RenderShell"} {
		if !strings.Contains(shell.Usage, symbol) {
			t.Errorf("shell no longer discovers real typed navigation anchor %s", symbol)
		}
	}
	if shell.Kind != "primitive" || shell.Maturity != "available" {
		t.Fatal("typed navigation unexpectedly promotes a complete shell recipe")
	}

	for _, tc := range []struct {
		name  string
		query Query
		first string
	}{
		{"meaningful export", Query{Text: "analytics CSV", Profile: "go-web"}, "analytics.comparison"},
		{"experimental comparison", Query{Text: "period filter", Kind: "recipe", Maturity: "experimental", Profile: "go-web"}, "analytics.comparison"},
		{"portable appearance", Query{Text: "appearance preference", Layer: "L2", Profile: "portable"}, "shell.appearance"},
		{"typed navigation", Query{Text: "typed navigation", Layer: "L3b", Profile: "go-web"}, "shell.layout"},
		{"no portable Go appearance control", Query{Text: "appearance preference", Layer: "L3b", Profile: "portable"}, ""},
		{"no portable opinionated analytics", Query{Text: "analytics CSV", Layer: "L3b", Profile: "portable"}, ""},
		{"no proven recipe claim", Query{Kind: "recipe", Maturity: "proven"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page, err := c.Search(tc.query)
			if err != nil {
				t.Fatal(err)
			}
			if tc.first == "" {
				if page.Total != 0 || len(page.Items) != 0 {
					t.Fatalf("unsupported fit was recommended: %+v", page)
				}
			} else if len(page.Items) == 0 || page.Items[0].ID != tc.first {
				t.Fatalf("missing expected first match %s: %+v", tc.first, page)
			}
			body, err := json.Marshal(page)
			if err != nil {
				t.Fatal(err)
			}
			if len(body) > 16<<10 || len(page.Items) > 10 {
				t.Fatal("default discovery budget exceeded")
			}
			for _, full := range []string{`"source"`, `"usage"`, `"proof"`, `"dependencies"`} {
				if strings.Contains(string(body), full) {
					t.Errorf("discovery exposes full contract field %s", full)
				}
			}
		})
	}
}

func TestAnalyticsUsageMapsPublicAPIsToExactFiles(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := c.Get("analytics.comparison")
	if !ok {
		t.Fatal("implemented analytics composition is missing from discovery")
	}

	for _, anchor := range []struct {
		path   string
		symbol string
	}{
		{"appkit/ui/analytics/analytics.go", "Dataset.Validate"},
		{"appkit/ui/analytics/render.go", "RenderBars"},
		{"appkit/ui/analytics/diverging.go", "RenderDivergingBars"},
		{"appkit/ui/analytics/csv.go", "WriteCSV"},
	} {
		if !strings.Contains(entry.Usage, anchor.path+"#"+anchor.symbol) {
			t.Errorf("analytics usage omits exact source anchor %s#%s", anchor.path, anchor.symbol)
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "..", filepath.FromSlash(anchor.path)), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		declared := map[string]bool{}
		catalogDeclarations(file, declared)
		if !declared[anchor.symbol] {
			t.Errorf("analytics source anchor %s is not exported/declared in %s", anchor.symbol, anchor.path)
		}
	}
	for _, guidance := range []string{"appkit/ui/analytics/README.md", "appkit/ui/analytics/*.go", "appkit/ui/vizframe/*.go", "relative to the selected kit source root", "get_kit_css_setup"} {
		if !strings.Contains(entry.Usage, guidance) {
			t.Errorf("analytics usage omits integration guidance %q", guidance)
		}
	}
}

// These checks ground metadata in shipped declarations, not the search index or
// other catalog prose. They do not turn consumer history into new adoption proof.
func TestInventorySourceAndUsageAnchors(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Entries) == 0 {
		t.Fatal("embedded catalog has no entries")
	}
	qualified := regexp.MustCompile(`\b([A-Za-z][A-Za-z0-9_]*)\.([A-Z][A-Za-z0-9_]*)\b`)
	for _, entry := range c.Entries {
		t.Run(entry.ID, func(t *testing.T) {
			if !filepath.IsLocal(entry.Source.Path) || !strings.HasSuffix(entry.Source.Path, ".go") {
				t.Fatalf("source must name a repository-relative Go file: %q", entry.Source.Path)
			}
			path := filepath.Join("..", "..", filepath.FromSlash(entry.Source.Path))
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			declared := map[string]bool{}
			catalogDeclarations(file, declared)
			if len(entry.Source.Symbols) == 0 {
				t.Fatal("first-party entry has no exported integration anchor")
			}
			for _, symbol := range entry.Source.Symbols {
				if !ast.IsExported(symbol) || !declared[symbol] {
					t.Errorf("source anchor %s is not exported/declared in %s", symbol, entry.Source.Path)
				}
			}
			packageSymbols := map[string]bool{}
			dir, err := os.ReadDir(filepath.Dir(path))
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range dir {
				if item.IsDir() || !strings.HasSuffix(item.Name(), ".go") || strings.HasSuffix(item.Name(), "_test.go") {
					continue
				}
				f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(filepath.Dir(path), item.Name()), nil, 0)
				if err != nil {
					t.Fatal(err)
				}
				catalogDeclarations(f, packageSymbols)
			}
			for _, mention := range qualified.FindAllStringSubmatch(entry.Usage, -1) {
				key := mention[1] + "." + mention[2]
				if mention[1] == file.Name.Name {
					key = mention[2]
				}
				if !packageSymbols[key] {
					t.Errorf("usage mentions %s, absent from package %s", mention[0], filepath.Dir(entry.Source.Path))
				}
			}
			for _, proof := range entry.Proof {
				if !strings.HasPrefix(proof, "go test ./") {
					t.Errorf("proof must name a reproducible package check, got %q", proof)
					continue
				}
				proofPath := strings.TrimPrefix(proof, "go test ./")
				if !filepath.IsLocal(proofPath) {
					t.Errorf("proof path escapes repository: %s", proof)
					continue
				}
				info, err := os.Stat(filepath.Join("..", "..", proofPath))
				if err != nil || !info.IsDir() {
					t.Errorf("proof target unavailable: %s (%v)", proof, err)
				}
			}
		})
	}
}

func catalogDeclarations(file *ast.File, names map[string]bool) {
	for _, declaration := range file.Decls {
		switch d := declaration.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				names[d.Name.Name] = ast.IsExported(d.Name.Name)
			} else if len(d.Recv.List) == 1 {
				receiver := d.Recv.List[0].Type
				if pointer, ok := receiver.(*ast.StarExpr); ok {
					receiver = pointer.X
				}
				if name, ok := receiver.(*ast.Ident); ok {
					names[name.Name+"."+d.Name.Name] = ast.IsExported(d.Name.Name)
				}
			}
		case *ast.GenDecl:
			for _, specification := range d.Specs {
				switch s := specification.(type) {
				case *ast.TypeSpec:
					names[s.Name.Name] = ast.IsExported(s.Name.Name)
					if structure, ok := s.Type.(*ast.StructType); ok {
						for _, field := range structure.Fields.List {
							for _, name := range field.Names {
								names[s.Name.Name+"."+name.Name] = ast.IsExported(name.Name)
							}
						}
					}
				case *ast.ValueSpec:
					for _, name := range s.Names {
						names[name.Name] = ast.IsExported(name.Name)
					}
				}
			}
		}
	}
}
