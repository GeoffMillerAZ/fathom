package main

import (
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/tokens"
)

// This file answers the question get_kit_theming creates but doesn't close:
// once you've contextually mapped a component's palette to kit role utilities
// (bg-accent, text-ink, rounded-lg), what makes those utilities actually
// render? The answer is the kit's CSS layer — the @theme block plus the
// --ak-color-* variables and the data-theme switching contract — built by the
// pinned Tailwind binary (no Node in this kit-owned build). cssSetupGuide()
// states that contract and enumerates the utility vocabulary from generated token
// names so it can't drift from what the kit emits.

// cssSetupGenArgs is the executable recipe shared by the guide and CLI canary.
// Explicit output paths keep generation app-local; an empty terminal output
// prevents this web-only recipe from writing a terminal palette.
func cssSetupGenArgs() []string {
	return []string{"gen", "-tokens", "app/ui/tokens.cue", "-css", "app/ui/tokens/theme.css", "-dtcg", "app/ui/tokens/tokens.dtcg.json", "-go", "app/ui/tokens/names_gen.go", "-terminal", ""}
}

func cssSetupOutputDirs() []string {
	return []string{"app/ui/tokens", "app/static"}
}

func cssSetupGenCommand() string {
	args := cssSetupGenArgs()
	for i, arg := range args {
		// These are fixed, shell-safe recipe literals, never caller input.
		if arg == "" {
			args[i] = "''"
		}
	}
	return "appkit " + strings.Join(args, " ")
}

// cssSetupSourceScript is a POSIX-shell example for one selected renderer.
// Missing directories fail before the local scanner fragment is written.
func cssSetupSourceScript() string {
	return `case "$APPKIT_CSS_MODULE_DIR" in
  /*) ;;
  *) printf '%s\n' 'Expected an absolute, resolved app-kit module directory.' >&2; exit 1 ;;
esac
case "$APPKIT_CSS_MODULE_DIR" in
  *\"*|*\\*|*[[:cntrl:]]*) printf '%s\n' 'Module path needs a separately reviewed CSS-string escape.' >&2; exit 1 ;;
esac
# Example selection: card. Add every selected utility-rendering dependency here.
APPKIT_CSS_PACKAGES='card'
for APPKIT_CSS_PACKAGE in $APPKIT_CSS_PACKAGES; do
  test -d "$APPKIT_CSS_MODULE_DIR/appkit/ui/$APPKIT_CSS_PACKAGE" || {
    printf '%s\n' "Missing selected kit source directory: $APPKIT_CSS_PACKAGE" >&2
    exit 1
  }
done
{
  for APPKIT_CSS_PACKAGE in $APPKIT_CSS_PACKAGES; do
    printf '@source "%s/appkit/ui/%s";\n' "$APPKIT_CSS_MODULE_DIR" "$APPKIT_CSS_PACKAGE"
  done
} > app/ui/kit.sources.css
`
}

// cssSetupInputCSS is the exact primary input shown to consumers and compiled by
// the source-closure test. Keep example prose outside this executable contract.
func cssSetupInputCSS() string {
	return `@import "tailwindcss" source(none);
@import "./tokens/theme.css";   /* the generated kit layer — defines the roles */

@import "./kit.sources.css";    /* local build input: selected module source paths */

/* Tailwind v4 scans these for utility classes. Every file that contains an
   adapted component MUST be covered, or its classes are never generated. */
@source "../**/*.templ";
@source "../**/*.go";
`
}

// cssSetupGuide renders the CSS-layer + build contract a consuming project must
// satisfy for kit role utilities to resolve. It is built from tokens.ColorNames
// / RadiusNames / FontNames, so the vocabulary follows the generated kit tokens.
func cssSetupGuide() string {
	var b strings.Builder

	b.WriteString("# Wiring app-kit's CSS into a consuming project\n\n")
	b.WriteString("Adapting a component's classes (see `get_kit_theming`) only pays off if the ")
	b.WriteString("role utilities it produces — `bg-accent`, `text-ink`, `border-line`, ")
	b.WriteString("`rounded-lg` — actually resolve. They resolve through **the kit's CSS layer**, ")
	b.WriteString("built by the checksum-pinned Tailwind standalone binary. **No Node is required by this kit-owned build**: ")
	b.WriteString("no npm or PostCSS step is added. This does not prohibit a consumer's declared React/client toolchain (ADR-0017/0021/0023). ")
	b.WriteString("Portable CSS and L3a variable-themed SVG can stop before utility scanning; the recipe below applies to selected utility-based L3b/L4 components.\n\n")

	b.WriteString("## The build, end to end\n\n")
	b.WriteString("```sh\n")
	b.WriteString("# Run in the consumer root with the intended, pinned appkit CLI available.\n")
	b.WriteString("# 1. Author app/ui/tokens.cue with a top-level tokens: value conforming to #Tokens.\n")
	b.WriteString("#    The generator does not create output directories.\n")
	b.WriteString("mkdir -p " + strings.Join(cssSetupOutputDirs(), " ") + " || exit 1\n")
	b.WriteString(cssSetupGenCommand() + " || exit 1\n")
	b.WriteString("#    Writes theme.css, tokens.dtcg.json and names_gen.go; no terminal palette.\n\n")
	b.WriteString("# 2. Create input.css and its local kit.sources.css using the scanning steps below.\n")
	b.WriteString("# 3. Build CSS (first use may download; checksum is verified on every invocation):\n")
	b.WriteString("appkit css -i app/ui/input.css -o app/static/app.css || exit 1\n")
	b.WriteString("```\n\n")
	b.WriteString("The `-css`, `-dtcg`, `-go` and `-terminal` flags each accept `''` to skip that output; ")
	b.WriteString("non-Go consumers can skip Go names. Keep output paths app-owned and review existing files before regenerating. ")
	b.WriteString("Inside the kit checkout, `task gen` regenerates kit defaults and `task gallery:css` builds Gallery CSS; these are not consumer setup commands.\n\n")

	b.WriteString("## What `input.css` must contain\n\n")
	b.WriteString("Tailwind first, the generated kit layer second, then your sources to scan:\n\n")
	b.WriteString("```css\n")
	b.WriteString(cssSetupInputCSS())
	b.WriteString("```\n\n")
	b.WriteString("`source(none)` disables implicit checkout scanning. Only imported CSS and the explicit `@source` selections contribute here; unrelated notes, archived examples or generated previews must not alter the result. ")
	b.WriteString("The app globs are an example boundary, not a universal manifest: add the app-owned HTML/JS/TS/Vue paths your selected renderer actually uses, and exclude fixtures or archives that share those paths. Do not restore automatic scanning to hide an omitted source.\n\n")
	b.WriteString("Those app globs do **not** cover imported Go modules. For selected L3b/L4 packages, resolve the actual pinned module directory from the consumer's Go module:\n\n")
	b.WriteString("```sh\nAPPKIT_CSS_MODULE_DIR=\"$(GOWORK=off go list -mod=readonly -m -f '{{.Dir}}' github.com/geoffmilleraz/app-kit)\" || exit 1\n")
	b.WriteString(cssSetupSourceScript())
	b.WriteString("```\n\n")
	b.WriteString("If resolution fails, stop and materialize the reviewed exact dependency through the consumer's normal dependency workflow; do not guess a sibling path. ")
	b.WriteString("The POSIX-shell example writes app-local `app/ui/kit.sources.css` only after each selected directory exists; Tailwind itself can silently miss absent sources. ")
	b.WriteString("Change the `card` selection to the packages actually rendering utility classes, including their rendering dependencies. ")
	b.WriteString("Keep this fragment ignored and recreate it when dependency resolution changes; **never check in absolute module-cache paths**. ")
	b.WriteString("Pin the module and CLI deliberately. Local replacements are development-only evidence, not clean delivery proof. ")
	b.WriteString("L3a `viz` emits variable-based SVG without utility classes and needs no kit-source scan.\n\n")
	b.WriteString("The most common \"my `bg-accent` does nothing\" cause is a component whose file ")
	b.WriteString("isn't covered by an `@source` glob — Tailwind only emits utilities it sees used.\n\n")
	b.WriteString("Incremental builds have a second coverage contract. Every scanned renderer file must also be an input to the incremental CSS task, together with the scanner manifest, generated theme CSS, input CSS and pinned compiler identity. Otherwise `@source` can be correct while the task cache silently reuses stale CSS.\n\n")
	b.WriteString("The CLI and generated artifacts have their own revision contract. If running the whole CLI from a consumer module demands unrelated indexing dependency checksums, build the reviewed pinned CLI in its own resolved module directory using that module's go.sum; do not add those dependencies to the application merely to generate CSS. Verify the generator revision and font/radius aliases before accepting new output. Existing compiled CSS is not proof that the pinned generator can reproduce it.\n\n")

	b.WriteString("## The utility vocabulary the kit gives you\n\n")
	b.WriteString("These are the generated kit role names; an app's CUE token set may add roles. ")
	b.WriteString("Choose mappings from component context, not palette-family guesses. Preserve originals and intentional color/state distinctions until reviewed adaptation and rendered checks justify a change (ADR-0022).\n\n")

	b.WriteString("**Color roles** — each works with every color property (`bg-`, `text-`, ")
	b.WriteString("`border-`, `ring-`, `divide-`, `fill-`, `stroke-`, `outline-`, `placeholder-`):\n\n")
	b.WriteString(roleVocabularyTable())

	b.WriteString("\n**Radius** (`rounded-<name>`, wired to `--radius-<name>`): ")
	b.WriteString(codeJoin(tokens.RadiusNames, "rounded-"))
	b.WriteString("\n\n**Fonts** (`font-<name>`, wired to `--font-<name>`): ")
	b.WriteString(codeJoin(tokens.FontNames, "font-"))
	b.WriteString("\n\nNamed radius and font utilities (`rounded-lg`, `font-sans`) already point at ")
	b.WriteString("kit tokens — a pulled component that uses them needs **no rewrite**. ")
	b.WriteString("Other names or arbitrary values may retain their original definitions. ")
	b.WriteString("Do not automatically round them to a nearest kit step: preserve the reference's proportions, typography and density unless a contextual design decision justifies changing them.\n\n")

	b.WriteString("## Light / dark — variable switching, not automatic visual proof\n\n")
	b.WriteString("Role utilities resolve through `--ak-color-*` variables that re-bind per scheme. ")
	b.WriteString("Set the scheme on the document, never per component:\n\n")
	b.WriteString("- `<html>` — follow the OS (`prefers-color-scheme`).\n")
	b.WriteString("- `<html data-theme=\"light\">` / `<html data-theme=\"dark\">` — forced.\n")
	b.WriteString("- `--ak-font-scale` on `:root` scales the base font size (the display setting).\n\n")
	b.WriteString("`get_kit_theming` preserves original classes; do not delete `dark:` or hover/focus/contrast distinctions automatically. ")
	b.WriteString("A role's light/dark pair is not proof that two original states were equivalent. ")
	b.WriteString("Also, `appkit gen` emits an attribute-based `@custom-variant dark`, replacing Tailwind's default OS-based variant. ")
	b.WriteString("Review this when integrating an existing theme: variable switching and `dark:` selectors are separate contracts. ")
	b.WriteString("For default portable tokens, `appkit vendor -out app/ui/vendor/appkit-tokens --include tokens` leaves the consumer's dark variant unchanged by default; this supplies kit defaults, not your custom token set.\n\n")

	b.WriteString("## Motion — the same import, and it needs no `@source`\n\n")
	b.WriteString("`appkit/ui/motion` supplies portable CSS (L1) and optional browser JS (L2): a two-state transition ")
	b.WriteString("vocabulary, a bounded stagger, and `prefers-reduced-motion` welded to the ")
	b.WriteString("class that grants motion. It is **optional** and independent of the role ")
	b.WriteString("utilities above — take it or leave it.\n\n")
	b.WriteString("Opt in by vendoring only motion from the intended kit version to a new app-owned directory; review any existing directory before updating it:\n\n")
	b.WriteString("```sh\nappkit vendor -out app/ui/vendor/appkit-motion --include motion\nappkit vendor -out app/ui/vendor/appkit-motion --include motion --check\n```\n\n")
	b.WriteString("Keep the generated `kit.lock` alongside the files and run the same check in the consumer's gate. ")
	b.WriteString("The lock checks this vendor channel's bytes; it does not certify licensing, browser behavior or whole-app adoption (ADR-0024). ")
	b.WriteString("Import the app-local stylesheet after the theme layer:\n\n")
	b.WriteString("```css\n")
	b.WriteString("@import \"tailwindcss\" source(none);\n")
	b.WriteString("@import \"./tokens/theme.css\";\n")
	b.WriteString("@import \"./vendor/appkit-motion/motion.css\";   /* optional; relative to app/ui/input.css */\n")
	b.WriteString("```\n\n")
	b.WriteString("**It needs no `@source` entry.** Unlike the role utilities, `motion.css` is ")
	b.WriteString("plain class selectors (`.ak-motion`, `.ak-fade`, …), not Tailwind utilities ")
	b.WriteString("generated by scanning your source — so the classes resolve with no scanner ")
	b.WriteString("involved. A page can even serve `motion.css` on its own via ")
	b.WriteString("`motion.CSSHandler()` with no CSS build at all.\n\n")
	b.WriteString("The class vocabulary (put `ak-motion` on the element that owns both states; ")
	b.WriteString("`ak-state-b` selects which is showing):\n\n")
	b.WriteString("- **`ak-motion`** — enables kit transitions in a subtree AND carries the ")
	b.WriteString("reduced-motion cancel for everything inside it, including your own transitions.\n")
	b.WriteString("- **`ak-state-b`** on the same element means the subtree is showing state B; its absence, state A.\n")
	b.WriteString("- **`ak-when-a` / `ak-when-b`** mark elements belonging to one state; **`ak-fade` / `ak-tint` / `ak-shift`** pick which properties transition (one per element).\n")
	b.WriteString("- **`ak-step-1`…`ak-step-8`** stagger by index; **`ak-after`** delays one full duration.\n\n")
	b.WriteString("From Go, compose the constants instead of stringly-typed classes: ")
	b.WriteString("`motion.RootClass(inStateB bool)` renders the root's class list for either ")
	b.WriteString("state (server-render the initial state — it is correct with JS disabled), ")
	b.WriteString("`motion.Step(i)` returns the i-th stagger class (\"\" past `MaxStep`), and ")
	b.WriteString("`motion.DelayStyle(d)` emits a `--ak-motion-delay` style for a computed delay. ")
	b.WriteString("The optional `motion.Script()` helper (`AppKitMotion.flush/enter`) forces the ")
	b.WriteString("style recalculation a transition applied in the same frame as mount needs — ")
	b.WriteString("a server-rendered initial state needs none of it. The JS artifact is not loaded merely by importing motion CSS; load it explicitly only if needed.\n\n")

	b.WriteString("## Verify it worked\n\n")
	b.WriteString("Render an adapted component and confirm a role utility resolves to an ")
	b.WriteString("`oklch(...)` value (DevTools → computed style → the `--ak-color-*` variable is ")
	b.WriteString("set). Toggle `data-theme` on `<html>` and the same element should re-theme with ")
	b.WriteString("no rebuild. If a utility is inert, check (1) `theme.css` is imported, and ")
	b.WriteString("(2) both app markup and imported rendering packages are covered by `@source`. ")
	b.WriteString("Build success is not source/asset proof: verify emitted selectors, served CSS/JS/font assets and computed styles, then the representative task in light/dark, keyboard, narrow-screen and reduced-motion states. ")
	b.WriteString("Add a new source file with a new utility class as an invalidation canary: the rebuild must run, and verification must find the generated selector and browser-computed wrapping and contrast in the rendered task. ")
	b.WriteString("Also compile in two isolated consumer roots with the same declared inputs: output bytes must agree even when one root contains an unrelated utility-bearing file. Removing a selected source must fail a required-selector check even if Tailwind exits zero. Preserve the output location across repeats. These are source-closure checks, not visual certification. ")
	b.WriteString("A module declaration or `appkit adoption check` is a separate static observation, not regeneration, browser or host certification.\n")

	return b.String()
}

// roleVocabularyTable groups the generated color roles into families for a
// compact, stable reference. Roles are taken from tokens.ColorNames so the
// table is exactly the kit's emitted vocabulary.
func roleVocabularyTable() string {
	// Group by the part before the first '-' (surface, ink, line, accent, …),
	// preserving the kit's intent buckets without hardcoding the role list.
	order := []string{"surface", "ink", "line", "accent", "positive", "caution", "danger", "info", "focus", "scrim"}
	groups := map[string][]string{}
	known := map[string]bool{}
	for _, base := range order {
		known[base] = true
	}
	for _, r := range tokens.ColorNames {
		base := r
		if i := strings.IndexByte(r, '-'); i > 0 {
			base = r[:i]
		}
		if !known[base] {
			// A family the bucket list does not name still appears, last and
			// in generated (sorted) order, rather than silently vanishing.
			known[base] = true
			order = append(order, base)
		}
		groups[base] = append(groups[base], r)
	}
	var b strings.Builder
	b.WriteString("| Family | Roles |\n|---|---|\n")
	for _, base := range order {
		roles, ok := groups[base]
		if !ok {
			continue
		}
		b.WriteString("| " + base + " | " + codeJoin(roles, "") + " |\n")
	}
	return b.String()
}

// codeJoin renders names as a comma-separated list of `prefix+name` in
// backticks.
func codeJoin(names []string, prefix string) string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = fmt.Sprintf("`%s%s`", prefix, n)
	}
	return strings.Join(out, ", ")
}
