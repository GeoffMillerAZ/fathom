package techpack

// A kit pack: header, a guard, a generated pair, a format and a concern.
kit: #Pack & {
	pack: {name: "app-kit", version: 3, about: "Go and templ UI kit"}
	guards: [{
		scopes:    ["**/*.templ"]
		note:      "Role utilities only."
		max_rules: 2
	}]
	generated: [{
		output:     "{dir}/{name}_templ.go"
		source:     ["{dir}/{name}.templ"]
		regenerate: "templ generate"
	}]
	formats: [{dialect: "templ", globs: ["**/*.templ"]}]
	concerns: stanza: {
		want:  "AGENTS.md carries the stanza"
		why:   "a stanza that was never wired routes nothing"
		check: {kind: "stanza_present", file: "AGENTS.md"}
	}
}

// The smallest pack is a header alone.
minimal: #Pack & {pack: {name: "x", version: 1, about: "y"}}

// An orientation entry: a path or path#Symbol, plus why.
entry: #Orientation & {anchor: "internal/verify/markdown.go#Parse", why: "the plan grammar parser"}
