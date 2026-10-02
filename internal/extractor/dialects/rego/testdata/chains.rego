package chains

import future.keywords.if
import data.chains.lib as lib

f(x) = 1 if {
	input.x == x
} {
	contains(input.s, x)
}

comments["annotation_match"](str) if regex.match(`^a`, str)

a.b.c(x) := lib.g(x)

found.vars[rule][ctx] contains v if {
	some v in data.chains.lib.vars[rule]
}

docs["resolve_url"](url) := replace(url, "$base", docs.base_url)

f.not(x) := x
