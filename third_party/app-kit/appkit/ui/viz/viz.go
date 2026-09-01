// Package viz generates server-rendered SVG visualization components for kit
// dashboards. All functions return a plain string of SVG markup.
//
// # Consumption
//
// Outputs are trusted markup — the package escapes every caller-supplied text
// internally (html.EscapeString), so consumers embed directly:
//
//	// In a templ component:
//	@templ.Raw(viz.Sparkline(values, viz.SparklineOptions{}))
//
//	// In html/template:
//	{{ template.HTML(viz.Donut(segs, viz.DonutOptions{})) }}
//
// # Theming
//
// Colors are never literal values. Every color parameter is a semantic role
// name resolved at render time to var(--ak-color-<role>). Components therefore
// re-theme with light/dark automatically by following the kit token pipeline
// (appkit/ui/tokens — EmitThemeCSS for the --ak-color-* convention).
//
// # Accessibility
//
// Every component emits role="img" + aria-label + <title>. Pass a meaningful
// AriaLabel; if empty a sensible default is derived.
//
// # Determinism
//
// Same inputs → byte-identical output. No time.Now, no rand, no map-order
// iteration in any output path. Golden-file tests in testdata/ enforce this.
package viz
