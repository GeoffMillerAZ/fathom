package staticcontent

// Local task-screening ceilings, selected before acquisition. These do not
// establish cold/warm/degraded-network or cross-browser performance acceptance.
schema_version: "static-content-local-v1"
budgets: {
	decoded_bytes: 131072
	response_end_ms: 100
	dom_content_loaded_ms: 250
	load_end_ms: 500
}
lanes: [
	{width: 390, height: 1000, theme: "light"},
	{width: 390, height: 1000, theme: "dark"},
	{width: 1440, height: 1000, theme: "light"},
	{width: 1440, height: 1000, theme: "dark"},
]
javascript: false
cache: "disabled"
network: "unthrottled-owned-loopback"
assets: ["/static/app.css"]
paths: ["/content", "/content/choosing-a-profile", "/content/not-a-page"]
