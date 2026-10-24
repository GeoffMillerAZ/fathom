// A rule pack as plain data: a comprehension over a hidden table.
_internal: ["analysis", "report"]

rules: [
	{from_pkg: "connector", to_pkg: "query", allow: false},
	for p in _internal {
		{from_pkg: p, to_pkg: "mcp", allow: false}
	},
]

deny_by_default: false
