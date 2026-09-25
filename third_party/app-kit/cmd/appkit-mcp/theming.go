package main

import (
	"regexp"
	"strings"
)

// The class-only API has no component intent, rendered contrast, or consumer
// role mapping. It can identify candidates for review, but cannot prove that a
// palette substitution or removal preserves the source design.

// retargetAction retains the existing response vocabulary for clients.
type retargetAction string

const (
	actionMap  retargetAction = "map"  // reserved for a proven replacement
	actionDrop retargetAction = "drop" // reserved for a proven redundant class
	actionKeep retargetAction = "keep" // preserve the original; Note may request review
)

type retarget struct {
	Original string         `json:"original"`
	Result   string         `json:"result,omitempty"` // only an applied, proven replacement; never advice
	Action   retargetAction `json:"action"`
	Note     string         `json:"note,omitempty"`
}

// This is only a review hint for familiar palette spellings, not a Tailwind
// parser or an inference from hue to semantic meaning. Unrecognized syntax is
// preserved just as recognized syntax is.
var rawPaletteUtility = regexp.MustCompile(`(?:^|:)!?(?:bg|text|border|ring|divide|fill|stroke|placeholder|outline)-(?:white|black|(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-[0-9]+)(?:/[0-9]+)?!?$`)

func retargetUtility(util string) retarget {
	v := retarget{Original: util, Action: actionKeep}
	switch {
	case strings.ContainsAny(util, "[]"):
		v.Note = "Review: preserve arbitrary variants and values intact. Any candidate role needs explicit component intent, appearance and interaction context; no substitution is applied."
	case rawPaletteUtility.MatchString(util):
		v.Note = "Review: preserve the original palette, text-on-fill contrast, dark appearance and distinct interaction states. Choose candidate roles from component intent and the consumer's mapping, adding consumer roles where needed; no substitution is applied."
	}
	return v
}

// RetargetClassList returns advisory verdicts and the exact original class
// string, including whitespace, order and duplicates. With no mapping context,
// no automatic rewrite can be demonstrated to preserve the component's design.
func RetargetClassList(classes string) (verdicts []retarget, rewritten string) {
	for _, tok := range strings.Fields(classes) {
		verdicts = append(verdicts, retargetUtility(tok))
	}
	return verdicts, classes
}

func themingGuide() string {
	const guide = `# Adapting a design reference to app-kit

Preserve the original design first: its hierarchy, palette relationships, typography,
spacing, contrast, appearance modes and interaction states. get_component returns the
source verbatim. get_kit_theming is advisory: its rewritten field preserves the exact
input, every verdict uses action=keep, and a Note beginning "Review:" identifies work
that needs context. Result is reserved for an applied replacement and stays absent;
candidate advice is never an automatic rewrite.

## Establish intent before mapping roles

A class list cannot establish semantic meaning. Pink does not establish danger;
blue does not establish accent. A shade can distinguish a hover state, a selected
item or a visual hierarchy. Preserve those distinctions until the consumer defines
an explicit mapping and verifies the rendered result.

- Keep dark: classes, including typography and layout. A semantic token's dark value
  does not prove an original dark override redundant; do not delete it automatically.
- Preserve arbitrary variants and values intact, including colons inside brackets.
- Check button text-on-fill contrast against the actual fill in every appearance and
  state. text-white is not evidence for faint text. An accent fill alone does not
  establish the correct foreground.
- Keep base, hover, focus, active, selected and disabled states distinct where the
  source distinguishes them. Do not collapse their shades into one role.
- Record the consumer's role mapping and add consumer roles when the existing
  vocabulary cannot preserve the design. Do not invent a kit utility that is not
  emitted by the selected token set.

These are candidate roles only after intent is established, not hue-to-role rules:

| Confirmed design intent | Candidate to evaluate |
|---|---|
| Application ground or panel | surface / surface-raised / surface-sunken |
| Text hierarchy | ink / ink-muted / ink-faint, with measured contrast |
| Primary action | accent (bg-accent), plus a verified text-on-fill and state mapping |
| Destructive action or error | danger, only when the application assigns that meaning |
| Keyboard focus indicator | focus (ring-focus), with a visible focus-state check |

For example, this source stays exactly as supplied:

    bg-indigo-600 text-white hover:bg-indigo-700 dark:bg-indigo-500 focus:ring-indigo-400

Replacing both background shades with bg-accent would erase the hover distinction.
The fill, foreground, focus and dark appearance require a reviewed mapping together.

## Select the source reference and renderer separately

Source-reference selection chooses the useful design and exact framework, Tailwind
major, appearance mode, dependencies and provenance. Renderer selection follows the
consumer's own adopted layers, runtime profile and existing application. A React,
Vue or HTML reference does not itself require a renderer migration.

Preserve the reference's interaction contract when implementing it in the chosen
renderer. Tailwind Plus Elements is an optional candidate for compatible HTML
interactions; availability of its source is not proof of app-kit compatibility.
Verify its dependency/version, integration, keyboard/focus behavior and appearance
in the actual consumer before claiming support.

## Apply the selected layer's contract

ADR-0017 and ADR-0021 scope build and runtime rules to the selected layers/profile.
The kit's Go/CUE build uses a pinned standalone Tailwind binary with no Node build;
its isolated browser-test tooling exception remains scoped. This is not a universal
ban on Node or a requirement to replace a React/Vue consumer's own build.

L0/L1 portable tokens/CSS and optional L2 browser JS can serve any consumer framework
without requiring Go, Datastar, NATS or a desktop shell. Consumers own their palette
and identity. Shared kit renderers use semantic roles rather than literal colors.
Preserving a licensed source reference is not a claim that it is a kit renderer.

L3b/L4 role utilities require the selected theme CSS and source scanning; use
get_kit_css_setup for that build contract. L3a appkit/ui/viz emits no class attributes:
it uses var(--ak-color-<role>) in SVG presentation attributes and needs no Tailwind
scan. Semantic variables in presentation or style attributes are valid; utility
classes are not the only way to consume the tokens.

After an explicit mapping, compare the running component with its reference across
light/dark/system behavior, interaction states, keyboard input and narrow viewports.
Measure foreground/fill contrast and verify emitted CSS. A class-list review does
not prove visual fidelity, accessibility or runtime compatibility.

The following hazards apply to the selected kit token/rendering integration:
`
	return guide + themingHazards()
}
