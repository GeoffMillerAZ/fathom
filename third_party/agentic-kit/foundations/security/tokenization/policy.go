package tokenization

// RolePolicy is a simple role-based access control (RBAC) policy. It grants
// detokenization rights based on a static map from role name to the set of
// TokenClasses that role may access. A principal is allowed if any of their
// roles grants access to the requested class.
//
// RolePolicy is immutable after construction and safe for concurrent use.
type RolePolicy struct {
	// allow maps role → set of TokenClass values that role may detokenize.
	allow map[string]map[TokenClass]bool
}

// NewRolePolicy constructs a RolePolicy from a map of role → allowed classes.
// The input map and slices are defensively copied; mutating the original after
// construction has no effect.
//
// A nil or empty map produces a policy that denies all requests.
func NewRolePolicy(roles map[string][]TokenClass) *RolePolicy {
	allow := make(map[string]map[TokenClass]bool, len(roles))
	for role, classes := range roles {
		set := make(map[TokenClass]bool, len(classes))
		for _, c := range classes {
			set[c] = true
		}
		allow[role] = set
	}
	return &RolePolicy{allow: allow}
}

// Allow returns true if any of principal's roles grants access to class.
// The purpose parameter is accepted for interface compatibility but is not
// used by RolePolicy — purpose-based gating belongs in a higher-level policy.
func (p *RolePolicy) Allow(principal Principal, class TokenClass, purpose string) bool {
	for _, role := range principal.Roles {
		if set, ok := p.allow[role]; ok && set[class] {
			return true
		}
	}
	return false
}
