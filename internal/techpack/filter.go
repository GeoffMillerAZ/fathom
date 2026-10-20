package techpack

// filter.go -- FD-20: the one filter every consumer of Discover calls
// (docs/design/fathom-dir.md section 7.2). Whether a discovered pack is
// served is decided here and nowhere else: the `tech-packs` capability
// (default on) and `exclude_packs` in .fathom/modules.cue.

import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/capability"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// CapabilityID is the capability that turns tech pack serving on or off.
const CapabilityID = "tech-packs"

// Policy is whether root serves tech packs and which it drops by name.
type Policy struct {
	// Enabled is the `tech-packs` capability's effective value.
	Enabled bool
	// Scope is where that value came from (built-in, user or project).
	Scope capability.Scope
	// Exclude is the sorted, de-duplicated `exclude_packs` of modules.cue.
	Exclude []string
	// Err is a problem reading the capability files or the exclude list.
	// It never turns serving off or drops a pack: an unreadable switch
	// leaves the default, and doctor names the error.
	Err error
}

// Excluded reports whether name is named by exclude_packs.
func (p Policy) Excluded(name string) bool {
	i := sort.SearchStrings(p.Exclude, name)
	return i < len(p.Exclude) && p.Exclude[i] == name
}

// PolicyFor reads root's tech pack policy offline. The capability resolves
// like every other (project over user over the built-in default).
func PolicyFor(root string) Policy {
	pol := Policy{Enabled: true, Scope: capability.ScopeBuiltIn}
	if root == "" {
		return pol
	}
	reg := capability.Default()
	c, ok := reg.Get(CapabilityID)
	if !ok {
		return pol
	}
	res := capability.Resolution{}
	if up := capability.DefaultUserPath(); up != "" {
		s, err := reg.LoadSettings(up, capability.ScopeUser)
		if err != nil {
			pol.Err = err
		} else {
			res.User = s
		}
	}
	s, err := reg.LoadSettings(capability.ProjectPath(root), capability.ScopeProject)
	if err != nil {
		pol.Err = err
	} else {
		res.Project = s
	}
	set := res.For(c)
	pol.Enabled, pol.Scope = set.On, set.Scope

	proj, err := projectdir.Load(root)
	if err != nil {
		pol.Err = err
		return pol
	}
	if k, ok := proj.Key("exclude_packs"); ok {
		if k.Err != nil {
			pol.Err = k.Err
		} else {
			var names []string
			if derr := k.Value.Decode(&names); derr != nil {
				pol.Err = derr
			} else {
				pol.Exclude = normalize(names)
			}
		}
	}
	return pol
}

func normalize(names []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range names {
		if n != "" && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// Filter is what a consumer is allowed to serve from d under pol: with the
// capability off, nothing (the zero Discovery, so output matches a
// repository with no pack); otherwise d without the excluded packs. Missing
// and Invalid are disclosure for doctor and stay.
func (pol Policy) Filter(d Discovery) Discovery {
	if !pol.Enabled {
		return Discovery{}
	}
	if len(pol.Exclude) == 0 {
		return d
	}
	out := Discovery{Missing: d.Missing, Invalid: d.Invalid}
	for _, p := range d.Packs {
		if !pol.Excluded(p.Name) {
			out.Packs = append(out.Packs, p)
		}
	}
	return out
}

// Served is the one entry point every consumer of Discover calls: root's
// discovered tech packs, filtered by its policy. With the capability off it
// does not read the pins at all.
func Served(root string) (Discovery, error) {
	pol := PolicyFor(root)
	if !pol.Enabled {
		return Discovery{}, nil
	}
	d, err := Discover(root)
	if err != nil {
		return Discovery{}, err
	}
	return pol.Filter(d), nil
}
