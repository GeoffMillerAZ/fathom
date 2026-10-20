package main

// doctor_techpacks.go -- the one `tech_packs` row (docs/design/fathom-dir.md
// sections 7.2 and 7.5): each tech pack the repository's own pins put on
// disk, its pin and path, the sections the fixture harness serves and the
// ones it withholds, and whether the `tech-packs` capability or
// `exclude_packs` keeps it from being served. A pinned module that is not on
// disk is named and never fetched (I56). The row reads Discover directly:
// it must see the packs the filter drops, so it is the one consumer that
// does not go through techpack.Served.

import (
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/techpack"
)

// techPacksMissingShown caps how many absent pinned modules the row spells
// out; the rest are counted.
const techPacksMissingShown = 6

func checkTechPacks(root string) doctorCheck {
	const name = "tech_packs"
	pol := techpack.PolicyFor(root)
	d, err := techpack.Discover(root)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorWarn,
			Reason: fmt.Sprintf("tech packs could not be discovered from the repository's pins: %v", err),
			Remedy: "fix the pin file the error names; packs are not served until it reads"}
	}

	var parts []string
	warn := false
	for _, p := range d.Packs {
		served, withheld := "none", "none"
		if r, rerr := techpack.RunFixtures(p.Dir); rerr != nil {
			withheld = "unreadable (" + rerr.Error() + ")"
			warn = true
		} else {
			var s, w []string
			for _, sec := range r.Sections {
				switch {
				case sec.NoFixtures:
					w = append(w, sec.Section+" (no fixtures)")
				case sec.Withheld:
					w = append(w, fmt.Sprintf("%s (%d of %d failed)", sec.Section, sec.Failed, sec.Run))
				default:
					s = append(s, sec.Section)
				}
			}
			if len(s) > 0 {
				served = strings.Join(s, ", ")
			}
			if len(w) > 0 {
				withheld = strings.Join(w, ", ")
				warn = true
			}
		}
		excluded := "no"
		switch {
		case !pol.Enabled:
			excluded = "yes, by the " + techpack.CapabilityID + " capability (" + string(pol.Scope) + ")"
		case pol.Excluded(p.Name):
			excluded = "yes, by exclude_packs"
		}
		parts = append(parts, fmt.Sprintf("%s pin %s@%s (%s) at %s, served: %s, withheld: %s, excluded: %s",
			p.Tag(), p.Module, p.Pin, p.Source, p.Dir, served, withheld, excluded))
	}
	for _, iv := range d.Invalid {
		parts = append(parts, fmt.Sprintf("invalid pack in %s (not served): %s", iv.Dir, iv.Err))
		warn = true
	}

	var notes []string
	if len(d.Missing) > 0 {
		shown := d.Missing
		if len(shown) > techPacksMissingShown {
			shown = shown[:techPacksMissingShown]
		}
		ms := make([]string, len(shown))
		for i, m := range shown {
			ms[i] = m.String()
		}
		note := "pinned but not on disk, never fetched: " + strings.Join(ms, ", ")
		if extra := len(d.Missing) - len(shown); extra > 0 {
			note += fmt.Sprintf(" and %d more", extra)
		}
		notes = append(notes, note)
	}
	if pol.Err != nil {
		notes = append(notes, "tech pack settings did not fully read, so no exclusion applies from the part that failed: "+pol.Err.Error())
		warn = true
	}
	if len(pol.Exclude) > 0 {
		notes = append(notes, "exclude_packs names "+strings.Join(pol.Exclude, ", "))
	}

	var reason string
	switch {
	case len(parts) == 0 && !pol.Enabled:
		reason = "no tech packs discovered; the " + techpack.CapabilityID + " capability is off (" + string(pol.Scope) + ")"
	case len(parts) == 0:
		reason = "no tech packs discovered in the repository's pins"
	default:
		reason = fmt.Sprintf("%d tech pack(s): %s", len(d.Packs), strings.Join(parts, " | "))
	}
	if len(notes) > 0 {
		reason += "; " + strings.Join(notes, "; ")
	}
	c := doctorCheck{Name: name, Status: doctorPass, Reason: reason}
	if warn {
		c.Status = doctorWarn
		c.Remedy = "add fixtures to the withheld sections or fix the pack (`fathom rules fixtures <pack dir>` reads them), or drop the pack with exclude_packs in .fathom/modules.cue"
	}
	return c
}
