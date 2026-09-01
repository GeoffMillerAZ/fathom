package matdash

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPathDerivation verifies that Section, Group, Name, and FullName are
// derived correctly from the filesystem path, including the index.tsx→parent
// rule and deep nested paths.
func TestPathDerivation(t *testing.T) {
	type tc struct {
		rel          string // slash-separated path relative to srcRoot
		wantSection  string
		wantGroup    string
		wantName     string
		wantFullName string
	}

	cases := []tc{
		{
			rel:          "components/charts/ApexLineChart.tsx",
			wantSection:  "components",
			wantGroup:    "charts",
			wantName:     "ApexLineChart",
			wantFullName: "components/charts/ApexLineChart",
		},
		{
			rel:          "components/apps/chat/index.tsx",
			wantSection:  "components",
			wantGroup:    "apps/chat",
			wantName:     "chat",
			wantFullName: "components/apps/chat/chat",
		},
		{
			rel:          "components/landingpage/index.tsx",
			wantSection:  "components",
			wantGroup:    "landingpage",
			wantName:     "landingpage",
			wantFullName: "components/landingpage/landingpage",
		},
		{
			rel:          "views/dashboard/DashboardPage.tsx",
			wantSection:  "views",
			wantGroup:    "dashboard",
			wantName:     "DashboardPage",
			wantFullName: "views/dashboard/DashboardPage",
		},
		{
			rel:          "views/widgets/index.tsx",
			wantSection:  "views",
			wantGroup:    "widgets",
			wantName:     "widgets",
			wantFullName: "views/widgets/widgets",
		},
		{
			rel:          "layouts/FullLayout.tsx",
			wantSection:  "layouts",
			wantGroup:    "",
			wantName:     "FullLayout",
			wantFullName: "layouts/FullLayout",
		},
		{
			rel:          "components/form-components/Form-Vertical/BasicLayout.tsx",
			wantSection:  "components",
			wantGroup:    "form-components/Form-Vertical",
			wantName:     "BasicLayout",
			wantFullName: "components/form-components/Form-Vertical/BasicLayout",
		},
		{
			rel:          "components/tables/hover/index.tsx",
			wantSection:  "components",
			wantGroup:    "tables/hover",
			wantName:     "hover",
			wantFullName: "components/tables/hover/hover",
		},
		{
			rel:          "components/theme-pages/account-settings/index.tsx",
			wantSection:  "components",
			wantGroup:    "theme-pages/account-settings",
			wantName:     "account-settings",
			wantFullName: "components/theme-pages/account-settings/account-settings",
		},
	}

	tmpDir := t.TempDir()

	// Write a minimal stub .tsx for every case.
	for _, c := range cases {
		p := filepath.Join(tmpDir, filepath.FromSlash(c.rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("setup mkdir %s: %v", p, err)
		}
		if err := os.WriteFile(p, []byte(`const X = () => <div className="p-4 text-sm">x</div>`), 0o644); err != nil {
			t.Fatalf("setup write %s: %v", p, err)
		}
	}

	for _, c := range cases {
		t.Run(c.rel, func(t *testing.T) {
			nativeParts := strings.Split(filepath.FromSlash(c.rel), string(filepath.Separator))
			rec, err := buildRecord(tmpDir, filepath.FromSlash(c.rel), nativeParts)
			if err != nil {
				t.Fatalf("buildRecord: %v", err)
			}
			comp := rec.Component
			if comp.Section != c.wantSection {
				t.Errorf("Section: got %q want %q", comp.Section, c.wantSection)
			}
			if comp.Group != c.wantGroup {
				t.Errorf("Group: got %q want %q", comp.Group, c.wantGroup)
			}
			if comp.Name != c.wantName {
				t.Errorf("Name: got %q want %q", comp.Name, c.wantName)
			}
			if comp.FullName != c.wantFullName {
				t.Errorf("FullName: got %q want %q", comp.FullName, c.wantFullName)
			}
			// Verify variant and row consistency.
			if len(comp.Variants) != 1 {
				t.Errorf("Variants len: got %d want 1", len(comp.Variants))
			}
			if len(rec.Rows) != 1 {
				t.Errorf("Rows len: got %d want 1", len(rec.Rows))
			}
			row := rec.Rows[0]
			if row.FullName != c.wantFullName {
				t.Errorf("VariantRow.FullName: got %q want %q", row.FullName, c.wantFullName)
			}
			if row.Framework != "react" {
				t.Errorf("Framework: got %q want react", row.Framework)
			}
			if row.Language != "tsx" {
				t.Errorf("Language: got %q want tsx", row.Language)
			}
			if row.TailwindVersion != 4 {
				t.Errorf("TailwindVersion: got %d want 4", row.TailwindVersion)
			}
			if row.Mode != "none" {
				t.Errorf("Mode: got %q want none", row.Mode)
			}
			if row.ContentHash == "" {
				t.Error("ContentHash should not be empty")
			}
		})
	}
}

// TestSkipRules verifies that the Walk function correctly skips files matching
// skip criteria, and includes the files it should include.
func TestSkipRules(t *testing.T) {
	type fc struct {
		rel     string
		wantIdx bool // true = should be indexed by Walk
	}

	cases := []fc{
		// Should be indexed.
		{"components/charts/ApexLineChart.tsx", true},
		{"views/dashboard/DashboardPage.tsx", true},
		{"layouts/FullLayout.tsx", true},
		{"components/form-components/Form-Vertical/BasicLayout.tsx", true},
		// Wrong section — skip.
		{"utils/Helper.tsx", false},
		{"assets/Logo.tsx", false},
		// Code / Codes segment — skip.
		{"components/headless-form/Checkbox/Codes/WithLabelCode.tsx", false},
		{"components/form-components/Form-Elements/Codes/RoundedInputs.tsx", false},
		// App.tsx / main.tsx — skip.
		{"components/charts/App.tsx", false},
		// .d.ts — skip.
		{"components/charts/types.d.ts", false},
		// context dir segment — skip.
		{"components/context/MyContext.tsx", false},
	}

	tmpDir := t.TempDir()

	for _, c := range cases {
		p := filepath.Join(tmpDir, filepath.FromSlash(c.rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("setup mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(`const X = () => <div/>`), 0o644); err != nil {
			t.Fatalf("setup write: %v", err)
		}
	}

	indexed := map[string]bool{}
	_, _, err := Walk(tmpDir, func(r Record) error {
		indexed[r.Component.FullName] = true
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	// Build expected full names for indexable cases and verify.
	wantCount := 0
	for _, c := range cases {
		if !c.wantIdx {
			continue
		}
		wantCount++
		parts := strings.Split(c.rel, "/")
		section := parts[0]
		stem := strings.TrimSuffix(parts[len(parts)-1], ".tsx")
		var name string
		if stem == "index" && len(parts) >= 2 {
			name = parts[len(parts)-2]
		} else {
			name = stem
		}
		group := strings.Join(parts[1:len(parts)-1], "/")
		var fullName string
		if group == "" {
			fullName = section + "/" + name
		} else {
			fullName = section + "/" + group + "/" + name
		}
		if !indexed[fullName] {
			t.Errorf("expected %q indexed (fullName=%q) but it was not", c.rel, fullName)
		}
	}

	if got := len(indexed); got != wantCount {
		t.Errorf("indexed count: got %d want %d; indexed=%v", got, wantCount, indexed)
	}
}

// TestExtractClasses verifies the className scraper returns deduplicated tokens.
func TestExtractClasses(t *testing.T) {
	code := `
		<div className="flex items-center p-4 text-sm">
			<span className="text-sm font-bold">
				<a class="flex p-4">link</a>
			</span>
		</div>
	`
	got := extractClasses(code)
	if got == "" {
		t.Fatal("expected non-empty classes")
	}
	// "flex" and "p-4" appear twice each — they should appear first (highest freq).
	toks := strings.Fields(got)
	if len(toks) == 0 {
		t.Fatal("no tokens")
	}
	// Verify no duplicate tokens.
	seen := map[string]bool{}
	for _, tok := range toks {
		if seen[tok] {
			t.Errorf("duplicate token %q in classes", tok)
		}
		seen[tok] = true
	}
	// Highest-frequency tokens should appear (flex and p-4 each appear 2x).
	for _, want := range []string{"flex", "p-4"} {
		if !seen[want] {
			t.Errorf("expected token %q in classes %q", want, got)
		}
	}
}
