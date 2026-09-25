package matdash

import (
	"crypto/sha256"
	"fmt"
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
	writeManifest(t, tmpDir, `{"devDependencies":{"tailwindcss":"^3.4.16"}}`)

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
			if row.TailwindVersion != 3 {
				t.Errorf("TailwindVersion: got %d want 3", row.TailwindVersion)
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
	writeManifest(t, tmpDir, `{"devDependencies":{"tailwindcss":"^3.4.16"}}`)

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

func writeManifest(t *testing.T, dir, manifest string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWalkSourceTailwindVersion(t *testing.T) {
	for _, tc := range []struct {
		name     string
		manifest string
		major    int
		wantErr  string
	}{
		{"v3-dev", `{"devDependencies":{"tailwindcss":"^3.4.16"}}`, 3, ""},
		{"v3-exact", `{"dependencies":{"tailwindcss":"3.4.16"}}`, 3, ""},
		{"v4-tilde", `{"dependencies":{"tailwindcss":"~4.1.0"}}`, 4, ""},
		{"v4-caret", `{"devDependencies":{"tailwindcss":"^4.0.0"}}`, 4, ""},
		{"v4-major", `{"peerDependencies":{"tailwindcss":"4"}}`, 4, ""},
		{"v3-optional", `{"optionalDependencies":{"tailwindcss":"3.4"}}`, 3, ""},
		{"same-major", `{"dependencies":{"tailwindcss":"^3.4.0"},"devDependencies":{"tailwindcss":"3.4.16"}}`, 3, ""},
		{"no-manifest", "", 0, "no package.json found"},
		{"no-dependency", `{}`, 0, "missing tailwindcss dependency"},
		{"malformed", `{`, 0, "parse "},
		{"non-string", `{"dependencies":{"tailwindcss":4}}`, 0, "parse "},
		{"unsupported-v2", `{"dependencies":{"tailwindcss":"^2.2.0"}}`, 0, "unsupported or ambiguous"},
		{"unsupported-v5", `{"dependencies":{"tailwindcss":"5.0.0"}}`, 0, "unsupported or ambiguous"},
		{"tag", `{"dependencies":{"tailwindcss":"latest"}}`, 0, "unsupported or ambiguous"},
		{"wildcard", `{"dependencies":{"tailwindcss":"*"}}`, 0, "unsupported or ambiguous"},
		{"cross-major-range", `{"dependencies":{"tailwindcss":">=3"}}`, 0, "unsupported or ambiguous"},
		{"alternative-majors", `{"dependencies":{"tailwindcss":"^3.4.16 || ^4.0.0"}}`, 0, "unsupported or ambiguous"},
		{"prerelease", `{"dependencies":{"tailwindcss":"4.0.0-beta.1"}}`, 0, "unsupported or ambiguous"},
		{"conflict", `{"dependencies":{"tailwindcss":"^3.4.16"},"devDependencies":{"tailwindcss":"^4.0.0"}}`, 0, "conflicting tailwindcss majors"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			packageRoot := t.TempDir()
			srcRoot := filepath.Join(packageRoot, "src")
			componentDir := filepath.Join(srcRoot, "components")
			if err := os.MkdirAll(componentDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if tc.manifest != "" {
				writeManifest(t, packageRoot, tc.manifest)
			}
			const code = "const Card = () => <div className=\"p-4 shadow-sm\">Original source</div>\r\n"
			if err := os.WriteFile(filepath.Join(componentDir, "Card.tsx"), []byte(code), 0o644); err != nil {
				t.Fatal(err)
			}
			var records []Record
			walked, _, err := Walk(srcRoot, func(r Record) error {
				records = append(records, r)
				return nil
			})
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), "package.json") {
					t.Fatalf("Walk error = %v, want package.json diagnostic containing %q", err, tc.wantErr)
				}
				if walked != 0 || len(records) != 0 {
					t.Fatalf("invalid metadata emitted records: walked=%d records=%d", walked, len(records))
				}
				return
			}
			if err != nil || walked != 1 || len(records) != 1 {
				t.Fatalf("Walk: walked=%d records=%d err=%v", walked, len(records), err)
			}
			record := records[0]
			variant, row := record.Component.Variants[0], record.Rows[0]
			if variant.TailwindVersion != tc.major || row.TailwindVersion != tc.major {
				t.Errorf("source Tailwind major: variant=%d row=%d want=%d", variant.TailwindVersion, row.TailwindVersion, tc.major)
			}
			if variant.Code != code || row.Code != code || row.ContentHash != fmt.Sprintf("%x", sha256.Sum256([]byte(code))) {
				t.Error("source bytes or source content hash changed")
			}
		})
	}
}

func TestWalkUsesNearestPackageManifest(t *testing.T) {
	for _, tc := range []struct {
		name     string
		manifest string
		wantErr  bool
	}{
		{"nested-version", `{"devDependencies":{"tailwindcss":"^4.1.0"}}`, false},
		{"no-parent-fallback", `{}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			packageRoot := t.TempDir()
			writeManifest(t, packageRoot, `{"devDependencies":{"tailwindcss":"^3.4.16"}}`)
			srcRoot := filepath.Join(packageRoot, "src")
			nestedRoot := filepath.Join(srcRoot, "components", "nested")
			if err := os.MkdirAll(nestedRoot, 0o755); err != nil {
				t.Fatal(err)
			}
			writeManifest(t, nestedRoot, tc.manifest)
			if err := os.WriteFile(filepath.Join(nestedRoot, "Card.tsx"), []byte(`const Card = () => <div/>`), 0o644); err != nil {
				t.Fatal(err)
			}
			walked, _, err := Walk(srcRoot, func(r Record) error {
				if r.Rows[0].TailwindVersion != 4 {
					t.Errorf("Tailwind major = %d, want nearest package's 4", r.Rows[0].TailwindVersion)
				}
				return nil
			})
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), filepath.Join(nestedRoot, "package.json")) || walked != 0 {
					t.Fatalf("expected nearest manifest failure without parent fallback: walked=%d err=%v", walked, err)
				}
			} else if err != nil || walked != 1 {
				t.Fatalf("Walk: walked=%d err=%v", walked, err)
			}
		})
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
