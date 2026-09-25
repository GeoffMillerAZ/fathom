package staticexport

// Prospective local screening only. These ceilings are not a general performance gate.
schema_version: "static-export-local-v1"
manifest_schema_version: 1
base_path: "/reading/"
javascript: false
cache: "disabled"
network: "unthrottled-owned-loopback"
budgets: {
	decoded_bytes: 131072
	response_end_ms: 100
	dom_content_loaded_ms: 250
	load_end_ms: 500
}
files: [
	"index.html", "index-light.html", "index-dark.html",
	"profile.html", "profile-light.html", "profile-dark.html",
	"404.html", "404-light.html", "404-dark.html",
	"app.css", "manifest.json",
]
lanes: [
	{width: 390, height: 1000, theme: "system", color_scheme: "dark"},
	{width: 390, height: 1000, theme: "light", color_scheme: "light"},
	{width: 390, height: 1000, theme: "dark", color_scheme: "dark"},
	{width: 1440, height: 1000, theme: "system", color_scheme: "dark"},
	{width: 1440, height: 1000, theme: "light", color_scheme: "light"},
	{width: 1440, height: 1000, theme: "dark", color_scheme: "dark"},
]
paths: {
	index: "index.html"
	profile: "profile.html"
	not_found: "404.html"
	deep_missing: "/reading/missing/deep/page.html"
}
oracles: {
	index_heading: "Choose a profile before you integrate a capability"
	profile_heading: "Choosing a profile"
	problem_heading: "Content destination unavailable"
	faq_question: "Which profile should I choose?"
	faq_destination_label: "Read the related guide"
	return_label: "Return to the integration guide"
	resource_labels: ["Match the runtime to the task", "Review delivery evidence"]
	resource_fragments: ["content-runtime-fit", "content-delivery-evidence"]
	toc_fragments: ["content-guide", "start-with-task", "keep-policy-local", "content-faq"]
}
