// Component registry metadata for appkit/ui/router.
// Validated by the appkit CUE registry schema (D7/D12).
package router

component: {
	name:        "ViewRouter"
	package:     "github.com/geoffmilleraz/app-kit/appkit/ui/router"
	description: "Framework-agnostic History-API client router for static-client surfaces (e.g. GitHub Pages SPAs). Zero dependencies, no build step. Embeds router.js and exposes Script() / Handler() for serving."
	since:       "0.1.0"

	// config documents the options accepted by AppKitRouter.create().
	config: [
		{name: "routes",   type: "[]Route",      required: true,  description: "Ordered list of route descriptors. First route is the implicit fallback when notFound is omitted."},
		{name: "notFound", type: "string",        required: false, description: "Route ID to activate when no path matches. Defaults to the first route."},
		{name: "onChange", type: "function",      required: false, description: "Callback invoked on every navigation. Receives a RouteState. Additional listeners can be added via router.onChange()."},
		{name: "base",     type: "string",        required: false, description: "Optional path prefix stripped before matching (e.g. \"/app\"). Trailing slashes ignored."},
		{name: "titleFor", type: "function",      required: false, description: "Optional function(state) returning a document title string. Overrides the route's built-in title field."},
	]

	types: {
		Route: {
			fields: [
				{name: "id",    type: "string", description: "Unique route identifier. Used as the argument to navigate()."},
				{name: "path",  type: "string", description: "Absolute pathname (e.g. \"/resume\"). Must start with /."},
				{name: "title", type: "string", description: "Optional document.title value set on activation."},
			]
		}
		RouteState: {
			fields: [
				{name: "id",      type: "string",            description: "ID of the active route (or the fallback ID when unmatched)."},
				{name: "path",    type: "string",            description: "Canonical path of the active route."},
				{name: "params",  type: "Record<string,string>", description: "Decoded query-string parameters. A trailing path segment (e.g. /resume/fcb) is also available as params.id."},
				{name: "hash",    type: "string",            description: "URL fragment without the leading #."},
				{name: "matched", type: "bool",              description: "True when the URL matched a declared route; false for the fallback."},
			]
		}
	}

	// api documents the methods on the router object returned by AppKitRouter.create().
	api: [
		{name: "start",    signature: "() => router",                             description: "Attach the popstate listener, read the current URL, and emit the initial state. Returns the router for chaining."},
		{name: "navigate", signature: "(idOrPath, opts?) => void",                description: "Push (or replace) a history entry and emit. Pass a route ID or an absolute path. opts: { params, replace }."},
		{name: "onChange", signature: "(fn) => unlisten",                         description: "Register an additional onChange listener. Returns an unlisten function."},
		{name: "current",  signature: "() => RouteState",                         description: "Return the RouteState for the current URL without emitting."},
		{name: "url",      signature: "(id, params?) => string",                  description: "Build a URL for the given route ID and optional query params without navigating."},
		{name: "routes",   signature: "Route[]",                                  description: "The route list passed to create(), exposed for inspection."},
	]

	// usageBoundary documents where this component is and is not appropriate.
	usageBoundary: {
		use:    "Static-client apps with no server router — GitHub Pages SPAs, Wails static-asset shells, offline-capable PWAs served from a CDN."
		doNot:  "Kit apps that have a real HTTP server. Those get routing from real URLs (ADR-0003 §2.4); adding a client router on top is anti-pattern 5.4."
	}

	dependencies: []
}
