// vendor.go — `appkit vendor`, the kit's distribution channel for consumers
// that are not Go apps.
//
// # Why this exists
//
// `geoffmiller-cloud/spa/router.js` and `appkit/ui/router/router.js` were
// byte-identical for months — md5 ad73ef48ea3726d06946595ce0acf296, 179 lines
// each — and nothing in either repo could tell you that. A React 18 CDN site
// was running kit code by copy, with no version, no checksum and no way to
// learn that the kit had moved. ADR-0013 names copied code as the failure the
// kit was founded against; this command is what makes "consume by import" true
// for a repo whose shipped site is not Go.
//
// A sibling Go tool in the consumer's repo (`geoffmiller-cloud/tools/jsxbuild`
// is the precedent — its own go.mod, `GOWORK=off go run .`) imports the kit and
// runs this. `go get -u github.com/geoffmilleraz/app-kit` in that directory is
// then the upgrade path, which is ADR-0013's mechanism executed rather than
// widened.
//
// # What it will and will not write
//
// **L0/L1/L2 only** (ADR-0017 §1): CUE-derived CSS, portable CSS, and classic
// browser scripts. It never writes an L3b renderer or a templ component,
// because those need a Go compiler and a Tailwind scanner over kit source —
// giving a non-Go consumer files it cannot use is worse than giving it none.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/geoffmilleraz/app-kit/appkit/ui/appearance"
	"github.com/geoffmilleraz/app-kit/appkit/ui/chat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/motion"
	"github.com/geoffmilleraz/app-kit/appkit/ui/overlay"
	"github.com/geoffmilleraz/app-kit/appkit/ui/router"
	"github.com/geoffmilleraz/app-kit/appkit/ui/themes/copper"
	"github.com/geoffmilleraz/app-kit/appkit/ui/themes/geoffindigo"
	"github.com/geoffmilleraz/app-kit/appkit/ui/themes/geoffsynthwave"
	"github.com/geoffmilleraz/app-kit/appkit/ui/themes/lagoon"
	"github.com/geoffmilleraz/app-kit/appkit/ui/themes/orchard"
	"github.com/geoffmilleraz/app-kit/appkit/ui/themes/synthwave"
	"github.com/geoffmilleraz/app-kit/appkit/ui/tokens"
)

// lockName is the manifest `appkit vendor` writes beside the artifacts. It is
// the thing spa/router.js never had.
const lockName = "kit.lock"

// vendorSet names one --include group and the files it materialises. Content is
// a func rather than a string so nothing is built for groups that were not
// asked for, and so the tokens group can see the emit options.
type vendorSet struct {
	name  string
	layer string
	files func(o vendorOptions) map[string]string
}

type vendorOptions struct {
	darkVariant tokens.DarkVariant
	theme       string
}

// vendorSets is the complete catalogue. Adding an L3b package here is a bug,
// not a feature — see the file header.
var vendorSets = []vendorSet{
	{
		name:  "appearance",
		layer: "L2",
		files: func(vendorOptions) map[string]string { return map[string]string{"appearance.js": appearance.Script()} },
	},
	{
		name:  "tokens",
		layer: "L1",
		files: func(o vendorOptions) map[string]string {
			css := tokens.ThemeCSS()
			switch o.theme {
			case "synthwave":
				css = synthwave.CSS()
			case "copper":
				css = copper.CSS()
			case "lagoon":
				css = lagoon.CSS()
			case "orchard":
				css = orchard.CSS()
			case "geoff-indigo":
				css = geoffindigo.CSS()
			case "geoff-synthwave":
				css = geoffsynthwave.CSS()
			}
			if o.darkVariant == tokens.DarkVariantNone {
				css = tokens.StripDarkVariant(css)
			}
			return map[string]string{"theme.css": css}
		},
	},
	{
		name:  "motion",
		layer: "L1+L2",
		files: func(vendorOptions) map[string]string {
			return map[string]string{"motion.css": motion.CSS(), "motion.js": motion.Script()}
		},
	},
	{
		name:  "router",
		layer: "L2",
		files: func(vendorOptions) map[string]string {
			return map[string]string{"router.js": router.Script()}
		},
	},
	{
		name:  "overlay",
		layer: "L2",
		files: func(vendorOptions) map[string]string {
			return map[string]string{"overlay.js": overlay.Script()}
		},
	},
	{
		name:  "chat",
		layer: "L2",
		files: func(vendorOptions) map[string]string {
			return map[string]string{"chat.js": chat.Script()}
		},
	},
}

func vendorSetNames() []string {
	names := make([]string, 0, len(vendorSets))
	for _, s := range vendorSets {
		names = append(names, s.name)
	}
	return names
}

// vendorCmd implements `appkit vendor`.
//
//	appkit vendor -out DIR --include tokens,motion,router,overlay,chat,appearance
//	              [--theme appkit|synthwave|copper|lagoon|orchard|geoff-indigo|geoff-synthwave]
//	              [--dark-variant none|attribute] [--check]
func vendorCmd(args []string) error {
	fs := flag.NewFlagSet("appkit vendor", flag.ContinueOnError)
	var (
		out = fs.String("out", "", "directory to write the vendored artifacts into (required)")
		inc = fs.String("include", strings.Join(vendorSetNames(), ","),
			"comma-separated artifact groups: "+strings.Join(vendorSetNames(), ", "))
		theme = fs.String("theme", "appkit", "tokens theme: appkit (default), synthwave, copper, lagoon, orchard, geoff-indigo or geoff-synthwave; writes one active theme.css")
		// none is the default here and attribute is the default in `appkit
		// gen`, deliberately. See the hazard note on
		// tokens.EmitThemeCSSOpts: emitting @custom-variant dark re-keys the
		// consuming app's own dark: utilities off prefers-color-scheme with no
		// error. A Go kit app sets data-theme and wants it; an app vendoring a
		// palette into an existing Tailwind build almost certainly does not.
		darkVariant = fs.String("dark-variant", string(tokens.DarkVariantNone),
			"how to define Tailwind's dark: variant — none (default; leaves your app's dark: on prefers-color-scheme) or attribute (keys it on data-theme)")
		check = fs.Bool("check", false, "verify against kit.lock instead of writing; explicit --theme also checks selected CSS; exits 1 on drift")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	explicitTheme := false
	fs.Visit(func(f *flag.Flag) { explicitTheme = explicitTheme || f.Name == "theme" })
	if *out == "" {
		fs.Usage()
		return errors.New("vendor: -out is required")
	}

	opts := vendorOptions{darkVariant: tokens.DarkVariant(*darkVariant), theme: *theme}
	switch opts.darkVariant {
	case tokens.DarkVariantNone, tokens.DarkVariantAttribute:
	default:
		return fmt.Errorf("vendor: --dark-variant %q: want none or attribute", *darkVariant)
	}
	switch opts.theme {
	case "appkit", "synthwave", "copper", "lagoon", "orchard", "geoff-indigo", "geoff-synthwave":
	default:
		return fmt.Errorf("vendor: --theme %q: want appkit, synthwave, copper, lagoon, orchard, geoff-indigo or geoff-synthwave", *theme)
	}

	sets, err := selectVendorSets(*inc)
	if err != nil {
		return err
	}
	if opts.theme != "appkit" {
		tokensSelected := false
		for _, s := range sets {
			tokensSelected = tokensSelected || s.name == "tokens"
		}
		if !tokensSelected {
			return fmt.Errorf("vendor: --theme %q requires tokens in --include", *theme)
		}
	}

	files := map[string]string{}
	for _, s := range sets {
		for name, body := range s.files(opts) {
			files[name] = body
		}
	}

	if *check {
		// Without --theme, retain the historical disk-vs-lock contract: a
		// pinned older artifact can be clean even when current kit bytes differ.
		// An explicit theme selection additionally verifies that one CSS file.
		if desired, selected := files["theme.css"]; explicitTheme && selected {
			actual, err := os.ReadFile(filepath.Join(*out, "theme.css"))
			if err != nil || string(actual) != desired {
				return fmt.Errorf("vendor --check: theme.css does not match explicitly selected --theme %q and --dark-variant %q; re-run `appkit vendor` with those options", opts.theme, opts.darkVariant)
			}
		}
		return checkKitLock(*out, files)
	}
	return writeVendor(*out, files, sets, opts)
}

// selectVendorSets resolves --include, rejecting an unknown name rather than
// silently vendoring less than the caller asked for.
func selectVendorSets(inc string) ([]vendorSet, error) {
	byName := map[string]vendorSet{}
	for _, s := range vendorSets {
		byName[s.name] = s
	}
	var out []vendorSet
	seen := map[string]bool{}
	for _, raw := range strings.Split(inc, ",") {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		s, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("vendor: unknown --include group %q (have: %s)", name, strings.Join(vendorSetNames(), ", "))
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, errors.New("vendor: --include selected nothing")
	}
	return out, nil
}

func writeVendor(dir string, files map[string]string, sets []vendorSet, opts vendorOptions) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("vendor: mkdir %s: %w", dir, err)
	}
	for _, name := range sortedKeys(files) {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(files[name]), 0o644); err != nil {
			return fmt.Errorf("vendor: write %s: %w", path, err)
		}
		fmt.Printf("appkit vendor: wrote %s (%d bytes)\n", path, len(files[name]))
	}
	lock := renderKitLock(files, sets, opts)
	lockPath := filepath.Join(dir, lockName)
	if err := os.WriteFile(lockPath, []byte(lock), 0o644); err != nil {
		return fmt.Errorf("vendor: write %s: %w", lockPath, err)
	}
	fmt.Printf("appkit vendor: wrote %s (%s)\n", lockPath, moduleVersion())
	return nil
}

// renderKitLock produces the manifest: the kit version these bytes came from,
// how they were emitted, and a SHA-256 per file. Deterministic — sorted keys,
// no timestamp — so a re-vendor of the same version is an empty diff and a
// changed byte is a one-line diff naming the file.
func renderKitLock(files map[string]string, sets []vendorSet, opts vendorOptions) string {
	var b strings.Builder
	b.WriteString("# kit.lock — generated by `appkit vendor`. Do not edit.\n")
	b.WriteString("# Re-run the vendor command to update; run it with --check in CI.\n")
	b.WriteString("# A vendored file edited in place is a fork: fix the kit, or record a\n")
	b.WriteString("# DIVERGENCE in the consuming repo (ADR-0017 §2).\n")
	fmt.Fprintf(&b, "module\t%s\n", modulePath)
	fmt.Fprintf(&b, "version\t%s\n", moduleVersion())
	fmt.Fprintf(&b, "dark-variant\t%s\n", opts.darkVariant)
	if opts.theme != "" && opts.theme != "appkit" {
		fmt.Fprintf(&b, "theme\t%s\n", opts.theme)
	}
	names := make([]string, 0, len(sets))
	for _, s := range sets {
		names = append(names, s.name+"("+s.layer+")")
	}
	sort.Strings(names)
	fmt.Fprintf(&b, "include\t%s\n", strings.Join(names, ","))
	b.WriteString("\n")
	for _, name := range sortedKeys(files) {
		fmt.Fprintf(&b, "sha256\t%s\t%s\n", sum(files[name]), name)
	}
	return b.String()
}

// checkKitLock re-hashes the vendored files against the lock and returns an
// error naming the first file that drifted.
//
// This is the mechanism that would have caught spa/router.js. Wire it into the
// consumer's CI: without it the vendored copy is exactly what it replaced — an
// unversioned copy that nothing compares to anything.
func checkKitLock(dir string, want map[string]string) error {
	lockPath := filepath.Join(dir, lockName)
	raw, err := os.ReadFile(lockPath)
	if err != nil {
		return fmt.Errorf("vendor --check: %s not found — this directory was not produced by `appkit vendor`: %w", lockPath, err)
	}

	recorded := map[string]string{}
	var lockedVersion string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		switch {
		case fields[0] == "version" && len(fields) == 2:
			lockedVersion = fields[1]
		case fields[0] == "sha256" && len(fields) == 3:
			recorded[fields[2]] = fields[1]
		}
	}
	if len(recorded) == 0 {
		return fmt.Errorf("vendor --check: %s records no files", lockPath)
	}

	var problems []string
	for _, name := range sortedKeys(recorded) {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: recorded in %s but missing from disk", name, lockName))
			continue
		}
		if got := sum(string(body)); got != recorded[name] {
			problems = append(problems, fmt.Sprintf("%s: on-disk sha256 %s does not match %s (%s) — the vendored copy was edited, or the lock is stale",
				name, short(got), lockName, short(recorded[name])))
		}
	}
	// A file the kit would emit that the lock does not know about means the
	// include set changed under the consumer; report it rather than passing.
	for _, name := range sortedKeys(want) {
		if _, ok := recorded[name]; !ok {
			problems = append(problems, fmt.Sprintf("%s: this kit emits it but %s does not record it — re-run `appkit vendor` (kit moved from %s)", name, lockName, lockedVersion))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("vendor --check: %d problem(s) in %s:\n  %s", len(problems), dir, strings.Join(problems, "\n  "))
	}
	fmt.Printf("appkit vendor --check: %s clean (%d files, kit %s)\n", dir, len(recorded), lockedVersion)
	return nil
}

const modulePath = "github.com/geoffmilleraz/app-kit"

// moduleVersion reports the kit version these bytes came from, read from the
// build info rather than a constant so it cannot be wrong.
//
// It is "(devel)" for a `go run ./cmd/appkit` inside the kit checkout and for
// every consumer today, because there is no released tag and all seven Go
// consumers pin v0.0.0 through a `replace`. A lock that says "(devel)" is
// telling the truth about that; ADR-0017 §4 makes cutting v0.1.0 the
// prerequisite for the lock to say anything more useful.
func moduleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "(unknown)"
	}
	if info.Main.Path == modulePath && info.Main.Version != "" {
		return info.Main.Version
	}
	for _, dep := range info.Deps {
		if dep.Path == modulePath && dep.Version != "" {
			return dep.Version
		}
	}
	return "(devel)"
}

func sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func short(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
