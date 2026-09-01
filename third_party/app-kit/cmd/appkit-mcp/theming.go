package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// This file bridges the two halves of the kit's value: a TailwindPlus component
// (raw palette utilities like bg-indigo-600, dark: variants, text-gray-900) and
// an app-kit app, which themes through semantic role utilities (bg-accent,
// text-ink) that auto-switch light/dark via the token system (D7). The mapping
// is deterministic and tested against the kit's generated role names; it never
// rewrites component code (get_component stays byte-faithful) — it tells the
// agent exactly how to adapt what it pulled.

// semanticFamily maps Tailwind's accent/status color families to a kit role
// base. These are the unambiguous ones: any reasonable theme maps indigo→accent,
// red→danger, etc. Shade is collapsed (the role token carries its own ramp).
var semanticFamily = map[string]string{
	"indigo": "accent", "blue": "accent", "violet": "accent", "purple": "accent",
	"sky": "info", "cyan": "info",
	"green": "positive", "emerald": "positive", "teal": "positive", "lime": "positive",
	"red": "danger", "rose": "danger", "pink": "danger",
	"amber": "caution", "yellow": "caution", "orange": "caution",
}

// neutralFamilies are the gray ramps; their role depends on the property and
// shade, so they're mapped by neutralRole rather than the flat table above.
var neutralFamilies = map[string]bool{
	"gray": true, "slate": true, "zinc": true, "neutral": true, "stone": true,
}

// colorProps are the Tailwind utility prefixes that take a color value and have
// a direct app-kit equivalent (prop stays, only the color token changes).
var colorProps = map[string]bool{
	"bg": true, "text": true, "border": true, "ring": true, "divide": true,
	"fill": true, "stroke": true, "placeholder": true, "outline": true,
}

// retargetAction classifies what should happen to one utility class.
type retargetAction string

const (
	actionMap  retargetAction = "map"  // replace with a kit role utility
	actionDrop retargetAction = "drop" // remove (kit owns this concern)
	actionKeep retargetAction = "keep" // not a kit-owned color; leave as-is
)

// retarget is the verdict for one utility class.
type retarget struct {
	Original string         `json:"original"`
	Result   string         `json:"result,omitempty"` // empty when dropped/kept-unchanged
	Action   retargetAction `json:"action"`
	Note     string         `json:"note,omitempty"`
}

// retargetUtility maps a single Tailwind utility (optionally with variant
// prefixes like hover: or dark:) to its app-kit form. It is conservative: it
// only rewrites color utilities it can map with confidence, drops dark: color
// variants (the kit's role tokens already carry their dark value), and keeps
// everything else untouched — so an agent can apply it class-by-class without
// risk of a wrong guess.
func retargetUtility(util string) retarget {
	util = strings.TrimSpace(util)
	if util == "" {
		return retarget{Original: util, Action: actionKeep}
	}
	variants, base := splitVariants(util)
	prop, color, isColor := splitColorUtility(base)
	if !isColor {
		return retarget{Original: util, Action: actionKeep}
	}
	dark := false
	kept := variants[:0]
	for _, v := range variants {
		if v == "dark" {
			dark = true
			continue
		}
		kept = append(kept, v)
	}
	// A dark: color utility is redundant under the kit's theme system: the role
	// token resolves to its own dark value when data-theme="dark". Drop it.
	if dark {
		return retarget{
			Original: util, Action: actionDrop,
			Note: "the kit's role tokens carry their own dark value (data-theme), so dark: color variants are redundant",
		}
	}

	role, ok, note := roleFor(prop, color)
	if !ok {
		return retarget{Original: util, Action: actionKeep, Note: note}
	}
	result := prop + "-" + role
	if len(kept) > 0 {
		result = strings.Join(kept, ":") + ":" + result
	}
	return retarget{Original: util, Result: result, Action: actionMap, Note: note}
}

// roleFor resolves the kit role for a (prop, color) pair. color is the part
// after the prop, e.g. "indigo-600", "gray-200", or "white".
func roleFor(prop, color string) (role string, ok bool, note string) {
	family, shade, special := splitColor(color)
	if special == "transparent" || special == "current" || special == "inherit" {
		return "", false, ""
	}
	if r, isSemantic := semanticFamily[family]; isSemantic {
		// ring on an accent family is almost always a focus ring.
		if prop == "ring" && r == "accent" {
			return "focus", true, "accent ring → focus ring"
		}
		return r, true, ""
	}
	// white/black behave like the extremes of a neutral ramp.
	switch special {
	case "white":
		family, shade = "neutral", 0
	case "black":
		family, shade = "neutral", 1000
	}
	if neutralFamilies[family] || special == "white" || special == "black" {
		return neutralRole(prop, shade)
	}
	if family == "" { // arbitrary value like bg-[#fff]
		return "", false, "arbitrary color — replace with the matching --ak-color role by hand"
	}
	return "", false, ""
}

// neutralRole maps a neutral utility to a kit role by property and shade. It
// only commits to the idiomatic cases; ambiguous mid-range darks (e.g. a
// bg-gray-800 panel) are left to the agent with a note.
func neutralRole(prop string, shade int) (string, bool, string) {
	switch prop {
	case "bg":
		switch {
		case shade <= 50:
			return "surface", true, ""
		case shade <= 100:
			return "surface-sunken", true, ""
		default:
			return "", false, "dark neutral background — choose surface / surface-raised / ink-inverse for the kit's intent"
		}
	case "text", "placeholder", "fill":
		switch {
		case shade <= 400:
			return "ink-faint", true, ""
		case shade <= 600:
			return "ink-muted", true, ""
		default:
			return "ink", true, ""
		}
	case "border", "divide", "ring", "outline", "stroke":
		switch {
		case shade <= 300:
			return "line", true, ""
		default:
			return "line-strong", true, ""
		}
	}
	return "", false, ""
}

// splitVariants splits "hover:dark:bg-indigo-600" into (["hover","dark"], "bg-indigo-600").
func splitVariants(util string) ([]string, string) {
	parts := strings.Split(util, ":")
	if len(parts) == 1 {
		return nil, util
	}
	return parts[:len(parts)-1], parts[len(parts)-1]
}

// splitColorUtility splits a base utility into its color prop and the color
// value, reporting whether it is a color utility the kit can retarget. Negative
// utilities and non-color props return isColor=false.
func splitColorUtility(base string) (prop, color string, isColor bool) {
	i := strings.IndexByte(base, '-')
	if i <= 0 || i == len(base)-1 {
		return "", "", false
	}
	prop, color = base[:i], base[i+1:]
	if !colorProps[prop] {
		return "", "", false
	}
	return prop, color, true
}

// splitColor parses a color value into (family, shade, special). For named
// colors without a shade (white, black, transparent, current, inherit) it
// returns them via special. For arbitrary values (bg-[#fff]) family is "".
func splitColor(color string) (family string, shade int, special string) {
	switch color {
	case "white", "black", "transparent", "current", "inherit":
		return "", 0, color
	}
	if strings.HasPrefix(color, "[") {
		return "", 0, "" // arbitrary value
	}
	i := strings.LastIndexByte(color, '-')
	if i <= 0 {
		return color, 0, "" // family with no shade
	}
	n, err := strconv.Atoi(color[i+1:])
	if err != nil {
		return color, 0, ""
	}
	return color[:i], n, ""
}

// RetargetClassList runs retargetUtility over a space-separated class string and
// returns the per-class verdicts and a suggested rewritten class string.
func RetargetClassList(classes string) (verdicts []retarget, rewritten string) {
	var out []string
	for _, tok := range strings.Fields(classes) {
		v := retargetUtility(tok)
		verdicts = append(verdicts, v)
		switch v.Action {
		case actionMap:
			out = append(out, v.Result)
		case actionKeep:
			out = append(out, v.Original)
		case actionDrop:
			// omit
		}
	}
	return verdicts, strings.Join(out, " ")
}

// themingGuide renders the authoritative adaptation guide as markdown, built
// from the same tables retargetUtility uses so the doc can't drift from the code.
func themingGuide() string {
	var b strings.Builder
	b.WriteString("# Adapting a TailwindPlus component to app-kit\n\n")
	b.WriteString("TailwindPlus components ship with raw palette utilities (`bg-indigo-600`, ")
	b.WriteString("`text-gray-900`, `dark:bg-gray-800`). app-kit themes through **semantic role ")
	b.WriteString("utilities** that auto-switch light/dark via the token system (D7) — so adapting ")
	b.WriteString("a component means swapping palette utilities for roles and dropping `dark:` color ")
	b.WriteString("variants. `get_component` returns the original source verbatim; apply these rules to it.\n\n")

	b.WriteString("## Rules\n\n")
	b.WriteString("1. **Color families → roles.** Replace the palette family with a kit role; the role carries its own shade ramp, so collapse shades.\n")
	b.WriteString("2. **Drop `dark:` color variants.** The role token already resolves to its dark value when `data-theme=\"dark\"`. Keep non-color `dark:` utilities only if truly needed (rare).\n")
	b.WriteString("3. **Focus rings → `ring-focus`.** Accent-colored rings are focus affordances.\n")
	b.WriteString("4. **Neutrals depend on the property** (see table). Genuinely ambiguous dark panels are left for you to choose deliberately.\n")
	// Rule 5 used to read "No Node, no inline color styles. Use utilities only
	// (D2)" — which the kit's own appkit/ui/viz violates on every element it
	// emits, so an agent that obeyed this guidance could not ship an L3a chart.
	// The kit has two theming tiers and this is the surface that has to say so
	// (ADR-0017 §1). The invariant both tiers share is not "utilities" — it is
	// "never a literal colour".
	b.WriteString("5. **No Node — and no *literal* color, in either tier.** The kit's CSS comes from the pinned Tailwind binary (D2). ")
	b.WriteString("Role utilities are the **L3b** path, and they are what you want for the TailwindPlus component you just pulled. ")
	b.WriteString("They are not the only correct path: `appkit/ui/viz` is **L3a** — it emits zero `class=` and resolves every color through ")
	b.WriteString("`var(--ak-color-<role>)` as an SVG presentation attribute. That is correct, not a violation, and it is what lets a kit chart ")
	b.WriteString("render correctly themed in a page with no Tailwind and no build step at all (ADR-0017 §1). ")
	b.WriteString("What is banned in **both** tiers is a literal — `#3b82f6`, `rgb(…)`, a raw `oklch(…)` — written into markup, a `style` attribute, or a presentation attribute. ")
	b.WriteString("Specificity footnote: a presentation attribute is (0,0,0), so any app stylesheet rule beats it — which is the mechanism that makes an L3a chart re-themable by a consumer.\n\n")

	b.WriteString("## Semantic families\n\n")
	// Group families by their role for a compact, stable table.
	byRole := map[string][]string{}
	for fam, role := range semanticFamily {
		byRole[role] = append(byRole[role], fam)
	}
	roles := make([]string, 0, len(byRole))
	for r := range byRole {
		roles = append(roles, r)
	}
	sort.Strings(roles)
	b.WriteString("| Tailwind family | app-kit role |\n|---|---|\n")
	for _, r := range roles {
		fams := byRole[r]
		sort.Strings(fams)
		fmt.Fprintf(&b, "| %s | `%s` (e.g. `bg-%s`, `text-%s`) |\n", strings.Join(fams, ", "), r, r, r)
	}

	b.WriteString("\n## Neutrals (gray, slate, zinc, neutral, stone) by property\n\n")
	b.WriteString("| Property | Light shades | Dark shades |\n|---|---|---|\n")
	b.WriteString("| `bg-` | `50`→`bg-surface`, `100`→`bg-surface-sunken` | choose `surface` / `surface-raised` / `ink-inverse` |\n")
	b.WriteString("| `text-`, `placeholder-`, `fill-` | `≤400`→`ink-faint`, `≤600`→`ink-muted` | `≥700`→`ink` |\n")
	b.WriteString("| `border-`, `divide-`, `ring-`, `outline-` | `≤300`→`line` | `≥400`→`line-strong` |\n")

	b.WriteString("\n## Example\n\n")
	example := "dark:bg-gray-800 bg-white text-gray-900 ring-indigo-500 border-gray-200 flex"
	_, rewritten := RetargetClassList(example)
	fmt.Fprintf(&b, "`%s`\n\n→ `%s`\n", example, rewritten)
	b.WriteString("\n(`flex` is untouched; `dark:bg-gray-800` is dropped; the rest map to roles.)\n")

	b.WriteString("\n## Make the roles actually render\n\n")
	b.WriteString("These role utilities only resolve if the consuming project includes the kit's ")
	b.WriteString("CSS layer (the `@theme` block + `--ak-color-*` variables, built with no Node). ")
	b.WriteString("If an adapted class appears to do nothing, call **`get_kit_css_setup`** for the ")
	b.WriteString("`input.css` contract, the `@source` scanning requirement, and the full role/radius/font vocabulary.\n")

	// Retargeting a component is the moment the theming hazards become
	// reachable, so the checklist rides along with the guide rather than
	// waiting behind a tool the agent has no reason to call (hazards.go).
	b.WriteString(themingHazards())
	return b.String()
}
