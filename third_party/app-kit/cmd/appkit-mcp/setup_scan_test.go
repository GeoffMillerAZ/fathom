package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/tokens"
	"github.com/geoffmilleraz/app-kit/internal/tailwind"
)

const (
	cssSetupAppTemplSelector = `.bg-closure-canary`
	cssSetupAppGoSelector    = `.h-\[39rem\]`
	cssSetupModuleSelector   = `.min-h-\[41rem\]`
	cssSetupAddedSelector    = `.w-\[43rem\]`
	cssSetupLeakSelector     = `.min-w-\[1937px\]`
)

// TestCSSSetupGuideSourceClosure compiles the exact input.css contract served
// to consumers. It proves that only the declared app and selected-module
// sources affect output, and that a successful compiler exit is not sufficient
// evidence that every selected renderer was scanned.
func TestCSSSetupGuideSourceClosure(t *testing.T) {
	if testing.Short() {
		t.Skip("end-to-end CSS source-closure build skipped in -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	compiler, err := tailwind.Resolve(ctx)
	if err != nil {
		t.Fatalf("resolve checksum-pinned Tailwind compiler: %v", err)
	}

	input := cssSetupInputCSS()
	if !strings.Contains(cssSetupGuide(), input) {
		t.Fatal("compiled input.css contract is not embedded verbatim in the CSS setup guide")
	}
	clean := newCSSSetupScanFixture(t, ctx, input, false)
	poisoned := newCSSSetupScanFixture(t, ctx, input, true)
	if clean.moduleDir == poisoned.moduleDir {
		t.Fatal("fixtures unexpectedly share a selected-module source tree")
	}

	cleanCSS := compileCSSSetupFixture(t, ctx, compiler, clean)
	poisonedCSS := compileCSSSetupFixture(t, ctx, compiler, poisoned)
	requiredSelectors := []string{
		cssSetupAppTemplSelector,
		cssSetupAppGoSelector,
		cssSetupModuleSelector,
	}
	if missing := missingCSSSetupSelectors(cleanCSS, requiredSelectors); len(missing) != 0 {
		t.Fatalf("declared source closure missing required selectors: %q", missing)
	}
	if bytes.Contains(poisonedCSS, []byte(cssSetupLeakSelector)) {
		t.Errorf("unrelated unignored HTML leaked into CSS: found %q", cssSetupLeakSelector)
	}
	if !bytes.Equal(cleanCSS, poisonedCSS) {
		t.Fatalf("unrelated checkout file changed CSS: clean sha256=%x poisoned sha256=%x",
			sha256.Sum256(cleanCSS), sha256.Sum256(poisonedCSS))
	}

	writeCSSSetupFixtureFile(t, clean.root, "app/pages/added.go", `package pages

const addedClass = "w-[43rem]"
`)
	addedCSS := compileCSSSetupFixture(t, ctx, compiler, clean)
	assertCSSSelector(t, addedCSS, cssSetupAddedSelector, "new allowed Go source")
	if bytes.Equal(cleanCSS, addedCSS) {
		t.Fatal("adding an allowed Go source did not change compiled CSS")
	}

	// Tailwind accepts an empty imported scanner fragment. The independent
	// selector oracle must therefore catch the selected renderer's omission.
	writeCSSSetupFixtureFile(t, clean.root, "app/ui/kit.sources.css", "")
	omittedCSS := compileCSSSetupFixture(t, ctx, compiler, clean)
	missing := missingCSSSetupSelectors(omittedCSS, requiredSelectors)
	if len(missing) != 1 || missing[0] != cssSetupModuleSelector {
		t.Fatalf("required-selector oracle got missing %q after selected @source removal, want only %q", missing, cssSetupModuleSelector)
	}
	assertCSSSelector(t, omittedCSS, cssSetupAddedSelector, "successful build with selected module omitted")

	const guardedImport = `@import "tailwindcss" source(none);`
	const ordinaryImport = `@import "tailwindcss";`
	unguarded := strings.Replace(input, guardedImport, ordinaryImport, 1)
	if unguarded == input {
		t.Fatalf("primary input lacks the automatic-source guard %q", guardedImport)
	}
	writeCSSSetupFixtureFile(t, poisoned.root, "app/ui/input.css", unguarded)
	unguardedCSS := compileCSSSetupFixture(t, ctx, compiler, poisoned)
	assertCSSSelector(t, unguardedCSS, cssSetupLeakSelector, "guard-disabled causal fixture")
	if bytes.Equal(poisonedCSS, unguardedCSS) {
		t.Fatal("disabling source(none) did not change poisoned fixture CSS")
	}
	t.Logf("CSS closure receipts: clean bytes=%d sha256=%x; poisoned bytes=%d sha256=%x; added bytes=%d sha256=%x; omitted bytes=%d sha256=%x; unguarded bytes=%d sha256=%x",
		len(cleanCSS), sha256.Sum256(cleanCSS),
		len(poisonedCSS), sha256.Sum256(poisonedCSS),
		len(addedCSS), sha256.Sum256(addedCSS),
		len(omittedCSS), sha256.Sum256(omittedCSS),
		len(unguardedCSS), sha256.Sum256(unguardedCSS))
}

type cssSetupScanFixture struct {
	root      string
	moduleDir string
}

func newCSSSetupScanFixture(t *testing.T, ctx context.Context, input string, poisoned bool) cssSetupScanFixture {
	t.Helper()
	f := cssSetupScanFixture{root: t.TempDir(), moduleDir: t.TempDir()}
	for path, body := range map[string]string{
		".gitignore":              "/app/static/\n",
		"app/ui/input.css":        input,
		"app/ui/tokens/theme.css": cssSetupSyntheticTheme(),
		"app/pages/page.templ":    `<main class="bg-closure-canary"></main>` + "\n",
		"app/pages/classes.go":    "package pages\n\nconst heightClass = \"h-[39rem]\"\n",
		"app/static/.keep":        "",
	} {
		writeCSSSetupFixtureFile(t, f.root, path, body)
	}
	writeCSSSetupFixtureFile(t, f.moduleDir, "appkit/ui/card/card.go", "package card\n\nconst minimumHeightClass = \"min-h-[41rem]\"\n")
	if poisoned {
		writeCSSSetupFixtureFile(t, f.root, "notes/sample.html", `<aside class="min-w-[1937px]">unrelated</aside>`+"\n")
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", cssSetupSourceScript())
	cmd.Dir = f.root
	cmd.Env = append(os.Environ(), "APPKIT_CSS_MODULE_DIR="+f.moduleDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate selected source fragment: %v\n%s", err, output)
	}
	wantSource := `@source "` + filepath.ToSlash(f.moduleDir) + `/appkit/ui/card";` + "\n"
	gotSource, err := os.ReadFile(filepath.Join(f.root, "app/ui/kit.sources.css"))
	if err != nil || string(gotSource) != wantSource {
		t.Fatalf("generated selected source fragment got %q, want %q: %v", gotSource, wantSource, err)
	}
	return f
}

func compileCSSSetupFixture(t *testing.T, ctx context.Context, compiler string, f cssSetupScanFixture) []byte {
	t.Helper()
	output := filepath.Join(f.root, "app/static/app.css")
	cmd := exec.CommandContext(ctx, compiler, "-i", "app/ui/input.css", "-o", output, "--minify")
	cmd.Dir = f.root
	if diagnostics, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile exact CSS setup input: %v\n%s", err, diagnostics)
	}
	css, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read compiled CSS: %v", err)
	}
	return css
}

func writeCSSSetupFixtureFile(t *testing.T, root, relative, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertCSSSelector(t *testing.T, css []byte, selector, source string) {
	t.Helper()
	if !bytes.Contains(css, []byte(selector)) {
		t.Errorf("compiled CSS missing %s selector %q", source, selector)
	}
}

func missingCSSSetupSelectors(css []byte, required []string) []string {
	var missing []string
	for _, selector := range required {
		if !bytes.Contains(css, []byte(selector)) {
			missing = append(missing, selector)
		}
	}
	return missing
}

func cssSetupSyntheticTheme() string {
	const light = "oklch(0.91 0.04 270)"
	const dark = "oklch(0.21 0.04 270)"
	fixture := tokens.Tokens{
		Name:   "css-setup-source-closure",
		Fonts:  tokens.Fonts{Sans: "Synthetic Sans, sans-serif", Mono: "monospace"},
		Radius: tokens.Radius{SM: "2px", MD: "5px", LG: "9px", XL: "13px", Full: "9999px"},
		Colors: map[string]tokens.ColorPair{},
	}
	for _, role := range tokens.ColorNames {
		fixture.Colors[role] = tokens.ColorPair{Light: light, Dark: dark}
	}
	fixture.Colors["closure-canary"] = tokens.ColorPair{Light: light, Dark: dark}
	return tokens.EmitThemeCSS(fixture)
}
