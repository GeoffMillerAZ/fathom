package example.lib.util

import rego.v1

public_paths := {"/", "/health"}

is_public(path) if path in public_paths

is_public(path) if startswith(path, "/static/")

has_role(user, role) if {
	role in data.example.roles.users[user]
}

# a function whose signature spans lines
paginate(items,
	offset,
	limit) := array.slice(items, offset, offset + limit)
