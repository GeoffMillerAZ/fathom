# METADATA
# title: Example authorization policy
package example.authz

import data.example.lib.util
import data.example.roles as r
import rego.v1

default allow := false

allow if {
	input.method == "GET"
	util.is_public(input.path)
}

allow if {
	some role in r.bindings[input.user]
	data.example.roles.grants[role][_] == input.action
}

deny contains msg if {
	not allow
	msg := sprintf("denied: %s", [input.user])
}

reasons[code] := text if {
	some code, text in data.example.messages
}

max_size(kind) := n if {
	n := object.get(data.example.limits, kind, 10)
} else := 0

admins := {u | some u in r.users; util.has_role(u, "admin")}

test_allow_get if {
	allow with input as {"method": "GET", "path": "/"}
		with data.example.lib.util.is_public as true
}
