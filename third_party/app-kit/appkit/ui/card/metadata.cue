// Component registry metadata for appkit/ui/card.
// Validated by the appkit CUE registry schema (D7/D12).
package card

component: {
	name:        "Card"
	package:     "github.com/geoffmilleraz/app-kit/appkit/ui/card"
	description: "Generic token-driven card primitive with header/body/footer slots."
	since:       "0.1.0"

	params: [
		{name: "Header", type: "templ.Component", required: false, description: "Optional slot rendered above the body, separated by a border-line hairline."},
		{name: "Body", type: "templ.Component", required: false, description: "Primary content slot. Nil renders an empty padded body area."},
		{name: "Footer", type: "templ.Component", required: false, description: "Optional slot rendered below the body, separated by a border-line hairline."},
		{name: "Elevated", type: "bool", required: false, description: "Use surface-overlay instead of surface-raised. For modal/popover contexts."},
		{name: "NoBorder", type: "bool", required: false, description: "Suppresses the border-line border. Use inside surface-sunken containers."},
		{name: "ExtraClass", type: "string", required: false, description: "Additional Tailwind utility classes appended to the outer element."},
	]

	variants: [
		{name: "default", description: "surface-raised background, border-line border, no slots."},
		{name: "with-header-footer", description: "All three slots populated."},
		{name: "elevated", description: "Elevated=true, surface-overlay surface."},
		{name: "no-border", description: "NoBorder=true, for use inside sunken containers."},
	]

	// tokenRoles lists every kit token role referenced by this component.
	tokenRoles: ["surface-raised", "surface-overlay", "line", "ink", "ink-muted"]
}
