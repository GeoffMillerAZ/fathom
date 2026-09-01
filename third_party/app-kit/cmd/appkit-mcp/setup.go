package main

import (
	"fmt"
	"strings"

	"github.com/geoffmilleraz/app-kit/appkit/ui/tokens"
)

// This file answers the question get_kit_theming creates but doesn't close:
// once you've rewritten a component's palette utilities to kit role utilities
// (bg-accent, text-ink, rounded-lg), what makes those utilities actually
// render? The answer is the kit's CSS layer — the @theme block plus the
// --ak-color-* variables and the data-theme switching contract — built by the
// pinned Tailwind binary (no Node, D2). cssSetupGuide() states that contract
// exactly, and enumerates the utility vocabulary from the generated token
// names so it can't drift from what the kit emits.

// cssSetupGuide renders the CSS-layer + build contract a consuming project must
// satisfy for kit role utilities to resolve. It is built from tokens.ColorNames
// / RadiusNames / FontNames, so the vocabulary it advertises is exactly what
// `appkit gen` emits.
func cssSetupGuide() string {
	var b strings.Builder

	b.WriteString("# Wiring app-kit's CSS into a consuming project\n\n")
	b.WriteString("Adapting a component's classes (see `get_kit_theming`) only pays off if the ")
	b.WriteString("role utilities it produces — `bg-accent`, `text-ink`, `border-line`, ")
	b.WriteString("`rounded-lg` — actually resolve. They resolve through **the kit's CSS layer**, ")
	b.WriteString("built by the pinned Tailwind standalone binary. **No Node, ever (D2)**: there is ")
	b.WriteString("no `package.json`, no PostCSS, no `npx tailwindcss`.\n\n")

	b.WriteString("## The build, end to end\n\n")
	b.WriteString("```sh\n")
	b.WriteString("# 1. Author your app's tokens (or reuse the kit default set) and generate\n")
	b.WriteString("#    the theme layer + Go role constants from them:\n")
	b.WriteString("appkit gen -tokens app/ui/tokens.cue -out app/ui/tokens\n")
	b.WriteString("#    → writes theme.css (the @theme + variable layer), tokens.dtcg.json, names_gen.go\n\n")
	b.WriteString("# 2. Build your app CSS with the pinned binary (downloads + sha256-verifies once):\n")
	b.WriteString("appkit css -i app/ui/input.css -o app/static/app.css\n")
	b.WriteString("```\n\n")
	b.WriteString("Inside the kit checkout the convenience targets are `task gen` and `task css:build`.\n\n")

	b.WriteString("## What `input.css` must contain\n\n")
	b.WriteString("Tailwind first, the generated kit layer second, then your sources to scan:\n\n")
	b.WriteString("```css\n")
	b.WriteString("@import \"tailwindcss\";\n")
	b.WriteString("@import \"./tokens/theme.css\";   /* the generated kit layer — defines the roles */\n\n")
	b.WriteString("/* Tailwind v4 scans these for utility classes. Every file that contains an\n")
	b.WriteString("   adapted component MUST be covered, or its classes are never generated. */\n")
	b.WriteString("@source \"../**/*.templ\";\n")
	b.WriteString("@source \"../**/*.go\";\n")
	b.WriteString("```\n\n")
	b.WriteString("The most common \"my `bg-accent` does nothing\" cause is a component whose file ")
	b.WriteString("isn't covered by an `@source` glob — Tailwind only emits utilities it sees used.\n\n")

	b.WriteString("## The utility vocabulary the kit gives you\n\n")
	b.WriteString("These are the utilities `theme.css` makes available. Targeting any other ")
	b.WriteString("palette utility (e.g. `bg-indigo-600`) means you skipped the adaptation step.\n\n")

	b.WriteString("**Color roles** — each works with every color property (`bg-`, `text-`, ")
	b.WriteString("`border-`, `ring-`, `divide-`, `fill-`, `stroke-`, `outline-`, `placeholder-`):\n\n")
	b.WriteString(roleVocabularyTable())

	b.WriteString("\n**Radius** (`rounded-<name>`, wired to `--radius-<name>`): ")
	b.WriteString(codeJoin(tokens.RadiusNames, "rounded-"))
	b.WriteString("\n\n**Fonts** (`font-<name>`, wired to `--font-<name>`): ")
	b.WriteString(codeJoin(tokens.FontNames, "font-"))
	b.WriteString("\n\nNamed radius and font utilities (`rounded-lg`, `font-sans`) already point at ")
	b.WriteString("kit tokens — a pulled component that uses them needs **no rewrite**. Only ")
	b.WriteString("off-scale values (`rounded-2xl`, arbitrary `rounded-[7px]`) fall back to ")
	b.WriteString("Tailwind defaults; map those to the nearest kit step if you want them themed.\n\n")

	b.WriteString("## Light / dark — you get it for free\n\n")
	b.WriteString("Role utilities resolve through `--ak-color-*` variables that re-bind per scheme. ")
	b.WriteString("Set the scheme on the document, never per component:\n\n")
	b.WriteString("- `<html>` — follow the OS (`prefers-color-scheme`).\n")
	b.WriteString("- `<html data-theme=\"light\">` / `<html data-theme=\"dark\">` — forced.\n")
	b.WriteString("- `--ak-font-scale` on `:root` scales the base font size (the display setting).\n\n")
	b.WriteString("This is why `get_kit_theming` drops `dark:` *color* variants: the role token ")
	b.WriteString("already carries its dark value, so `dark:bg-gray-800` is redundant.\n\n")

	b.WriteString("## Motion — the same import, and it needs no `@source`\n\n")
	b.WriteString("`appkit/ui/motion` is the kit's motion layer (L2): a two-state transition ")
	b.WriteString("vocabulary, a bounded stagger, and `prefers-reduced-motion` welded to the ")
	b.WriteString("class that grants motion. It is **optional** and independent of the role ")
	b.WriteString("utilities above — take it or leave it.\n\n")
	b.WriteString("Import `motion.css` after the theme layer, exactly as you imported it:\n\n")
	b.WriteString("```css\n")
	b.WriteString("@import \"tailwindcss\";\n")
	b.WriteString("@import \"./tokens/theme.css\";\n")
	b.WriteString("@import \"../../../appkit/ui/motion/motion.css\";   /* the motion vocabulary — optional */\n")
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
	b.WriteString("a server-rendered initial state needs none of it.\n\n")

	b.WriteString("## Verify it worked\n\n")
	b.WriteString("Render an adapted component and confirm a role utility resolves to an ")
	b.WriteString("`oklch(...)` value (DevTools → computed style → the `--ak-color-*` variable is ")
	b.WriteString("set). Toggle `data-theme` on `<html>` and the same element should re-theme with ")
	b.WriteString("no rebuild. If a utility is inert, check (1) `theme.css` is imported, and ")
	b.WriteString("(2) the component's file is covered by an `@source` glob.\n")

	return b.String()
}

// roleVocabularyTable groups the generated color roles into families for a
// compact, stable reference. Roles are taken from tokens.ColorNames so the
// table is exactly the kit's emitted vocabulary.
func roleVocabularyTable() string {
	// Group by the part before the first '-' (surface, ink, line, accent, …),
	// preserving the kit's intent buckets without hardcoding the role list.
	order := []string{"surface", "ink", "line", "accent", "positive", "caution", "danger", "info", "focus"}
	groups := map[string][]string{}
	for _, r := range tokens.ColorNames {
		base := r
		if i := strings.IndexByte(r, '-'); i > 0 {
			base = r[:i]
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
