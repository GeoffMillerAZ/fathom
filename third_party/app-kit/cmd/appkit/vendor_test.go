package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/tokens"
)

// TestVendorWritesRequestedArtifactsOnly — the file set must equal the
// --include set exactly. Over-writing is the failure that matters: an L3b
// renderer or a templ file landing in a non-Go consumer's tree is a file it
// cannot compile, cannot theme (no Tailwind scanner over kit source) and will
// eventually edit in place.
func TestVendorWritesRequestedArtifactsOnly(t *testing.T) {
	dir := t.TempDir()
	if err := vendorCmd([]string{"-out", dir, "--include", "tokens,motion"}); err != nil {
		t.Fatalf("vendor: %v", err)
	}

	got := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		got[e.Name()] = true
	}
	for _, want := range []string{"theme.css", "motion.css", "motion.js", "kit.lock"} {
		if !got[want] {
			t.Errorf("vendor did not write %s", want)
		}
		delete(got, want)
	}
	for extra := range got {
		t.Errorf("vendor wrote %s, which was not in the --include set", extra)
	}

	// No emitted artifact carries a literal color (ADR-0017 §2) — that
	// invariant is what makes a consuming app's ~25-line alias file able to
	// re-theme every kit artifact at once.
	literal := regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b|\brgba?\(|\bhsla?\(`)
	for _, name := range []string{"theme.css", "motion.css"} {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		// The kit token set is authored in oklch(); a THEME legitimately carries
		// values (it is the thing that defines the roles). motion.css must not.
		if name == "motion.css" && literal.Match(body) {
			t.Errorf("%s contains a literal color: %s", name, literal.Find(body))
		}
	}
}

// TestVendorRejectsUnknownInclude — silently vendoring less than was asked for
// produces a consumer that is missing a file and does not know it.
func TestVendorRejectsUnknownInclude(t *testing.T) {
	dir := t.TempDir()
	err := vendorCmd([]string{"-out", dir, "--include", "tokens,callout"})
	if err == nil {
		t.Fatal("vendoring an unknown group succeeded")
	}
	if !strings.Contains(err.Error(), "callout") {
		t.Errorf("error does not name the unknown group: %v", err)
	}
	// It must also refuse before writing anything.
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("vendor wrote %d files despite failing", len(entries))
	}
}

// TestVendorDarkVariantDefaultIsNone — the default protects a consumer that has
// its own dark: utilities. `@custom-variant dark (…)` REPLACES Tailwind's
// built-in variant, re-keying every dark: utility off prefers-color-scheme onto
// an attribute the app may never set, with no error anywhere.
func TestVendorDarkVariantDefaultIsNone(t *testing.T) {
	dir := t.TempDir()
	if err := vendorCmd([]string{"-out", dir, "--include", "tokens"}); err != nil {
		t.Fatalf("vendor: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "theme.css"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "@custom-variant") {
		t.Error("vendor's default emitted @custom-variant; it must default to --dark-variant none")
	}
	// And the theming itself still ships.
	if !strings.Contains(string(body), "--ak-color-surface") {
		t.Error("vendored theme.css lost its palette")
	}

	attr := t.TempDir()
	if err := vendorCmd([]string{"-out", attr, "--include", "tokens", "--dark-variant", "attribute"}); err != nil {
		t.Fatalf("vendor --dark-variant attribute: %v", err)
	}
	body, _ = os.ReadFile(filepath.Join(attr, "theme.css"))
	if !strings.Contains(string(body), "@custom-variant dark") {
		t.Error("--dark-variant attribute did not emit the custom variant")
	}
	if string(body) != tokens.ThemeCSS() {
		t.Error("--dark-variant attribute must reproduce the kit's committed theme.css byte for byte")
	}

	if err := vendorCmd([]string{"-out", t.TempDir(), "--include", "tokens", "--dark-variant", "media"}); err == nil {
		t.Error("an unknown --dark-variant was accepted")
	}
}

// TestVendorCheckDetectsDrift — the assertion this whole command exists for.
// spa/router.js was a byte-identical copy of kit code for months with nothing
// able to notice. --check is what notices.
func TestVendorCheckDetectsDrift(t *testing.T) {
	dir := t.TempDir()
	args := []string{"-out", dir, "--include", "tokens,router,motion"}
	if err := vendorCmd(args); err != nil {
		t.Fatalf("vendor: %v", err)
	}
	if err := vendorCmd(append(args, "--check")); err != nil {
		t.Fatalf("a freshly vendored directory failed --check: %v", err)
	}

	// One byte, in one file.
	path := filepath.Join(dir, "router.js")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(body, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	err = vendorCmd(append(args, "--check"))
	if err == nil {
		t.Fatal("--check passed after a vendored file was edited")
	}
	if !strings.Contains(err.Error(), "router.js") {
		t.Errorf("--check error does not name the drifted file: %v", err)
	}

	// A deleted file is also drift.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := vendorCmd(append(args, "--check")); err == nil {
		t.Fatal("--check passed after a vendored file was deleted")
	}
}

// TestVendorIsDeterministic — re-vendoring the same kit must be an empty diff,
// or `--check` in CI becomes noise the consumer learns to ignore.
func TestVendorIsDeterministic(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	args := func(d string) []string {
		return []string{"-out", d, "--include", "tokens,motion,router,overlay,chat"}
	}
	if err := vendorCmd(args(a)); err != nil {
		t.Fatal(err)
	}
	if err := vendorCmd(args(b)); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(a)
	if len(entries) == 0 {
		t.Fatal("nothing vendored")
	}
	for _, e := range entries {
		x, _ := os.ReadFile(filepath.Join(a, e.Name()))
		y, _ := os.ReadFile(filepath.Join(b, e.Name()))
		if string(x) != string(y) {
			t.Errorf("%s differs between two runs of the same kit", e.Name())
		}
	}
}

// TestVendorNeverWritesUtilityThemedArtifacts — a standing guard on the
// catalogue itself (ADR-0017 §1). An L3b package added to vendorSets would hand
// a non-Go consumer files it cannot render.
func TestVendorNeverWritesUtilityThemedArtifacts(t *testing.T) {
	allowed := map[string]bool{"tokens": true, "motion": true, "router": true, "overlay": true, "chat": true, "appearance": true}
	for _, s := range vendorSets {
		if !allowed[s.name] {
			t.Errorf("vendor set %q is not an L0/L1/L2 group; vendoring an L3b renderer or a templ component gives a non-Go consumer files it cannot use", s.name)
		}
		for name := range s.files(vendorOptions{}) {
			if ext := filepath.Ext(name); ext != ".css" && ext != ".js" {
				t.Errorf("vendor set %q emits %q; only .css and .js cross the target boundary", s.name, name)
			}
		}
	}
}

func TestVendorAppearanceIncludesOnlyPortableScript(t *testing.T) {
	out := t.TempDir()
	if err := vendorCmd([]string{"-out", out, "-include", "appearance"}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("want only appearance.js and kit.lock, got %v", entries)
	}
	for _, entry := range entries {
		if entry.Name() != "appearance.js" && entry.Name() != "kit.lock" {
			t.Errorf("non-portable/unselected artifact: %s", entry.Name())
		}
	}
	if err := vendorCmd([]string{"-out", out, "-include", "appearance", "-check"}); err != nil {
		t.Fatal(err)
	}
}
