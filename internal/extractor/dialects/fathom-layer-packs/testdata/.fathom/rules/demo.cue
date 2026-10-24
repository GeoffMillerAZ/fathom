// A rule pack as plain data: no package clause, converted against
// schema/rules/archconformance #RuleSet.
let pkgs = ["alpha", "beta"]

rules: [
	{from_pkg: "connector", to_pkg: "query", allow: false},
	for p in pkgs {
		{from_pkg: p, to_pkg: "store", allow: false}
	},
]

deny_by_default: false
