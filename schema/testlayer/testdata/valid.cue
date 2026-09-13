package testlayer

// A well-formed testlayer config: two declared layers leaning on every
// signal kind, one minimal single-glob layer, and the legitimate
// "no custom layers" empty form (Classify's own unit/non-test base case
// still applies with zero declared layers — unlike schema/guard, an empty
// list here is not dead-by-construction).
withEverything: #Layers & {
	layers: [
		{name: "integration", build_tags: ["integration"], dirs: ["integration"]},
		{name: "e2e", build_tags: ["e2e"], dirs: ["e2e"], globs: ["**/e2e/**"]},
	]
}

minimal: #Layers & {
	layers: [{name: "contract", globs: ["contract/**"]}]
}

empty: #Layers & {
	layers: []
}
