package iacfacts

import (
	"fmt"
	"strconv"
	"strings"
)

// IaCVersion is a provider version: MAJOR.MINOR.PATCH with an optional pre-release. Build metadata is dropped.
type IaCVersion struct {
	Major, Minor, Patch int
	Pre                 string
	Raw                 string
}

// ParseIaCVersion parses a full three-part version.
func ParseIaCVersion(s string) (IaCVersion, error) {
	v, n, err := parseVersionParts(s)
	if err != nil {
		return v, err
	}
	if n != 3 {
		return v, fmt.Errorf("version %q needs three numeric parts", s)
	}
	return v, nil
}

// parseVersionParts parses one to three numeric parts and reports how many were written.
func parseVersionParts(s string) (IaCVersion, int, error) {
	raw := s
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	var pre string
	if i := strings.IndexByte(s, '-'); i >= 0 {
		s, pre = s[:i], s[i+1:]
		if pre == "" {
			return IaCVersion{}, 0, fmt.Errorf("version %q has an empty pre-release", raw)
		}
	}
	parts := strings.Split(s, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return IaCVersion{}, 0, fmt.Errorf("version %q is not MAJOR[.MINOR[.PATCH]]", raw)
	}
	var nums [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || len(p) > 9 || (len(p) > 1 && p[0] == '0') {
			return IaCVersion{}, 0, fmt.Errorf("version %q has a non-numeric part %q", raw, p)
		}
		nums[i] = n
	}
	return IaCVersion{Major: nums[0], Minor: nums[1], Patch: nums[2], Pre: pre, Raw: raw}, len(parts), nil
}

// String is the canonical MAJOR.MINOR.PATCH[-pre] spelling.
func (v IaCVersion) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	return s
}

// CompareIaCVersions orders by precedence: -1, 0 or 1.
func CompareIaCVersions(a, b IaCVersion) int {
	for _, d := range [3][2]int{{a.Major, b.Major}, {a.Minor, b.Minor}, {a.Patch, b.Patch}} {
		if d[0] != d[1] {
			if d[0] < d[1] {
				return -1
			}
			return 1
		}
	}
	switch {
	case a.Pre == b.Pre:
		return 0
	case a.Pre == "":
		return 1
	case b.Pre == "":
		return -1
	}
	ap, bp := strings.Split(a.Pre, "."), strings.Split(b.Pre, ".")
	for i := 0; i < len(ap) && i < len(bp); i++ {
		an, aerr := strconv.Atoi(ap[i])
		bn, berr := strconv.Atoi(bp[i])
		switch {
		case aerr == nil && berr == nil && an != bn:
			if an < bn {
				return -1
			}
			return 1
		case aerr == nil && berr != nil:
			return -1
		case aerr != nil && berr == nil:
			return 1
		case aerr != nil && berr != nil && ap[i] != bp[i]:
			if ap[i] < bp[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(ap) < len(bp):
		return -1
	case len(ap) > len(bp):
		return 1
	}
	return 0
}

type constraintClause struct {
	op   string
	v    IaCVersion
	segs int
}

// IaCConstraint is a Terraform version constraint: clauses joined by commas, all of which must hold.
type IaCConstraint struct {
	Raw     string
	clauses []constraintClause
}

// ParseIaCConstraint parses `>= 5.0, < 6.0`, `~> 5.1` and the other Terraform operators (=, !=, >, >=, <, <=, ~>).
// An empty constraint admits every release.
func ParseIaCConstraint(s string) (IaCConstraint, error) {
	c := IaCConstraint{Raw: strings.TrimSpace(s)}
	if c.Raw == "" {
		return c, nil
	}
	for _, part := range strings.Split(c.Raw, ",") {
		part = strings.TrimSpace(part)
		op := "="
		for _, o := range []string{"~>", ">=", "<=", "!=", "=", ">", "<"} {
			if strings.HasPrefix(part, o) {
				op, part = o, strings.TrimSpace(part[len(o):])
				break
			}
		}
		// OpenTofu's constraint parser has no `v` prefix: `v4.0.6` is an invalid constraint though 4.0.6 is not.
		if strings.HasPrefix(part, "v") {
			return IaCConstraint{}, fmt.Errorf("constraint %q: a version in a constraint takes no v prefix", s)
		}
		v, n, err := parseVersionParts(part)
		if err != nil {
			return IaCConstraint{}, fmt.Errorf("constraint %q: %w", s, err)
		}
		c.clauses = append(c.clauses, constraintClause{op: op, v: v, segs: n})
	}
	return c, nil
}

// Matches reports whether v satisfies every clause. A pre-release is never selected by a range, or by no constraint
// at all. Only an `=` clause that names it exactly makes it eligible, and the other clauses must still hold. This is
// what `tofu init` does: `>= 6.0.0` over 6.0.0 and 6.1.0-beta1 installs 6.0.0, `>= 6.1.0-beta1` finds nothing, and
// `= 6.1.0-beta1, >= 6.0.0` installs the pre-release.
func (c IaCConstraint) Matches(v IaCVersion) bool {
	if v.Pre != "" {
		named := false
		for _, cl := range c.clauses {
			if cl.op == "=" && CompareIaCVersions(v, cl.v) == 0 {
				named = true
			}
		}
		if !named {
			return false
		}
	}
	for _, cl := range c.clauses {
		cmp := CompareIaCVersions(v, cl.v)
		ok := false
		switch cl.op {
		case "=":
			ok = cmp == 0
		case "!=":
			ok = cmp != 0
		case ">":
			ok = cmp > 0
		case ">=":
			ok = cmp >= 0
		case "<":
			ok = cmp < 0
		case "<=":
			ok = cmp <= 0
		case "~>":
			ok = cmp >= 0 && pessimisticHolds(v, cl)
		}
		if !ok {
			return false
		}
	}
	return true
}

// pessimisticHolds is OpenTofu's Ruby-style rule for provider constraints (apparentlymart/go-versions, ruby_style.go):
// three written parts hold the major and minor and let the patch rise, and one or two written parts hold the major
// and let the minor rise. So `~> 5` is `>= 5.0, < 6`, the same as `~> 5.0`, `~> 5.1` is `>= 5.1, < 6` and `~> 5.1.2`
// is `>= 5.1.2, < 5.2`. Terraform's go-version reads `~> 5` as `>= 5` instead, which OpenTofu does not.
func pessimisticHolds(v IaCVersion, cl constraintClause) bool {
	vs := [3]int{v.Major, v.Minor, v.Patch}
	cs := [3]int{cl.v.Major, cl.v.Minor, cl.v.Patch}
	last := cl.segs - 1
	if last < 1 {
		last = 1
	}
	for i := 0; i < last; i++ {
		if vs[i] != cs[i] {
			return false
		}
	}
	return vs[last] >= cs[last]
}

// LowerLine names the major line the constraint's lowest admitted release sits in, for a disclosure that tells an
// operator which catalog to add. It reports false when the constraint has no lower bound.
func (c IaCConstraint) LowerLine() (string, bool) {
	for _, cl := range c.clauses {
		switch cl.op {
		case "=", ">=", ">", "~>":
			return strconv.Itoa(cl.v.Major), true
		}
	}
	return "", false
}
