package tokens

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A role that does not exist fails silently, in both directions:
//
//   - `var(--ak-color-typo)` is invalid at computed-value time, so the
//     declaration is dropped — the mark renders at the property's initial value
//     (black fill, no border) or disappears. No console error, nothing to see
//     in review.
//   - `text-neon-pink` where `neon-pink` is not in the active token set means
//     Tailwind never emits the utility at all — the class is inert, and the
//     built CSS contains no trace of it to grep for.
//
// These two tests detect both from source, with no browser and no build.

var (
	akColorRefRe  = regexp.MustCompile(`--ak-color-([a-z][a-z0-9-]*)`)
	akColorDefRe  = regexp.MustCompile(`--ak-color-([a-z][a-z0-9-]*)\s*:`)
	roleUtilityRe = regexp.MustCompile(`(?:^|[\s"'` + "`" + `:])(?:bg|text|border|ring|divide|fill|stroke|outline|placeholder|from|via|to)-([a-z][a-z0-9-]*)`)
)

// repoRoot is the kit checkout, relative to this package.
const repoRoot = "../../.."

// TestNoDanglingRoleReferences: every literal var(--ak-color-<role>) written in
// kit source must name a role the default token set defines. A role that only
// some themes define belongs in TestNoThemeOnlyRoleUtilities' territory, not in
// a component.
func TestNoDanglingRoleReferences(t *testing.T) {
	valid := map[string]bool{}
	for _, r := range ColorNames {
		valid[r] = true
	}
	for _, f := range kitSourceFiles(t) {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, m := range akColorRefRe.FindAllStringSubmatch(string(body), -1) {
			if !valid[m[1]] {
				t.Errorf("%s references var(--ak-color-%s), which the kit token set does not define "+
					"— the declaration is invalid at computed-value time and is silently dropped",
					rel(f), m[1])
			}
		}
	}
}

// themeOnlyUsage records the one place kit source depends on a role only a
// non-default theme defines. Ratchet, not an excuse list: a new occurrence
// fails, and removing a listed one ALSO fails (with a note to delete the
// entry). Documented in docs/theming-hazards.md §"Known open in this kit".
//
//	kanban / neon-pink — appkit/ui/kanban/kanban.templ:73 sets
//	`hover:text-neon-pink` on the card title. `neon-pink` exists only in
//	appkit/ui/themes/synthwave. Under the default token set the utility is never
//	generated (the built apps/gallery/ui/app.css contains zero occurrences of
//	`neon-pink`), so the hover does nothing at all — silently.
var themeOnlyUsage = map[string]bool{
	"kanban: neon-pink": true,
}

// TestNoThemeOnlyRoleUtilities: a component in appkit/ui must not style itself
// with a role that only one shipped theme defines. If it does, the utility is
// simply never emitted under every other token set — the most invisible failure
// in the CSS layer, because there is nothing wrong to see in the source, the
// built CSS, or the browser.
func TestNoThemeOnlyRoleUtilities(t *testing.T) {
	defaultRoles := map[string]bool{}
	for _, r := range ColorNames {
		defaultRoles[r] = true
	}

	// Roles defined by a shipped theme but absent from the default set.
	themeOnly := map[string]bool{}
	themeDir := filepath.Join(repoRoot, "appkit", "ui", "themes")
	err := filepath.WalkDir(themeDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Base(path) != "theme.css" {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range akColorDefRe.FindAllStringSubmatch(string(body), -1) {
			if !defaultRoles[m[1]] {
				themeOnly[m[1]] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", themeDir, err)
	}
	if len(themeOnly) == 0 {
		t.Skip("no shipped theme defines a role outside the default set")
	}

	seen := map[string]bool{}
	for _, f := range kitSourceFiles(t) {
		if !strings.Contains(filepath.ToSlash(f), "/appkit/ui/") {
			continue // apps may use their own token set; the kit may not
		}
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		pkg := filepath.Base(filepath.Dir(f))
		for _, m := range roleUtilityRe.FindAllStringSubmatch(string(body), -1) {
			if !themeOnly[m[1]] {
				continue
			}
			key := pkg + ": " + m[1]
			if seen[key] {
				continue
			}
			seen[key] = true
			if !themeOnlyUsage[key] {
				t.Errorf("%s styles with role %q, which only a non-default theme defines — "+
					"the utility is never generated under the kit's own token set, so the rule "+
					"silently does not exist", rel(f), m[1])
			}
		}
	}
	for key := range themeOnlyUsage {
		if !seen[key] {
			t.Errorf("themeOnlyUsage lists %q but nothing uses it any more — delete the entry "+
				"and its note in docs/theming-hazards.md", key)
		}
	}
}

// paletteUtilityRe matches a Tailwind palette-color utility (bg-black/40,
// text-gray-500, ring-white): a fixed color that ignores the token set, so it
// reads the same under every theme and in both schemes.
var paletteUtilityRe = regexp.MustCompile(`(?:^|[\s"'` + "`" + `:])(?:bg|text|border|ring|divide|fill|stroke|outline|placeholder|from|via|to|shadow|decoration|caret)-` +
	`(black|white|slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)(?:[^a-z0-9-]|-[0-9]|$)`)

// TestComponentsUseRoleColorsOnly: every color a kit component paints comes
// from a role, so a consumer that enforces role utilities over its rendered
// markup never needs an exception for kit-owned classes. FATHOM-S0-KIT-02 found
// the one breach: the overlay backdrop was bg-black/40 (now the scrim role).
func TestComponentsUseRoleColorsOnly(t *testing.T) {
	for _, f := range kitSourceFiles(t) {
		if !strings.Contains(filepath.ToSlash(f), "/appkit/ui/") {
			continue // apps may use their own token set; the kit may not
		}
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for i, line := range strings.Split(string(body), "\n") {
			if m := paletteUtilityRe.FindStringSubmatch(line); m != nil {
				t.Errorf("%s:%d paints with the palette color %q instead of a token role: %s",
					rel(f), i+1, m[1], strings.TrimSpace(line))
			}
		}
	}
}

// kitSourceFiles returns every hand-written and generated source file that can
// carry a role name: Go, templ, and the kit's vanilla JS (Tailwind scans JS too).
func kitSourceFiles(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, root := range []string{"appkit", "apps"} {
		err := filepath.WalkDir(filepath.Join(repoRoot, root), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" || d.Name() == "node_modules" {
					return fs.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil // tests discuss role names; they never render them
			}
			switch filepath.Ext(path) {
			case ".go", ".templ", ".js":
				out = append(out, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if len(out) == 0 {
		t.Fatal("no kit source files found — is repoRoot still correct?")
	}
	return out
}

// rel renders a path relative to the repo root for readable failures.
func rel(p string) string {
	if r, err := filepath.Rel(repoRoot, p); err == nil {
		return filepath.ToSlash(r)
	}
	return p
}
