package edge["cases"].v1

import data.example.lib.util as u

# a comment with { braces and "quotes"
q.r.s := 1

items[x] if some x in input.list

flags.debug contains "trace" if input.verbose

greet(name) := $"hello {upper(name)}!" if true

pattern := `^[a-z]+{2}#$`

check(x) if
	u.has_role(x, "ops")

empty := set()

counted := count([1 | some _ in data.edge.items]) if true
