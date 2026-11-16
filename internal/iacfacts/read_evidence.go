package iacfacts

import (
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ReadObservation records one actual input, including files declaring only defaults.
// Read distinguishes an opened input from a refusal; Complete describes extraction.
type ReadObservation struct {
	Path           string
	Read, Complete bool
	Unknowns       []UnknownReason
}

// BuildReadEvidence combines classified observations without reading any inputs.
// An incomplete observation wins over a complete observation of the same path.
// It copies, sorts and deduplicates reasons and actions without retaining values.
func BuildReadEvidence(observations []ReadObservation) ReadEvidence {
	out := ReadEvidence{Read: []string{}, Unread: []string{}, Unknowns: []UnknownReason{}}
	logical := logicalEvidencePaths(observations)
	paths := map[string]bool{}
	type reasonKey struct {
		code UnknownCode
		span Span
	}
	reasons := map[reasonKey]map[DecidableBy]bool{}
	for _, o := range observations {
		complete := o.Read && o.Complete
		p := logical[o.Path]
		if previous, exists := paths[p]; exists {
			complete = complete && previous
		}
		paths[p] = complete
		unknowns := o.Unknowns
		if (!o.Read || !o.Complete) && len(unknowns) == 0 {
			code := UnknownUnsupportedForm
			actions := []DecidableBy{}
			if !o.Read {
				code = UnknownUnreadInput
				actions = []DecidableBy{DecideContextFile}
			}
			unknowns = []UnknownReason{{Code: code, Span: Span{File: o.Path}, DecidableBy: actions}}
		}
		for _, u := range unknowns {
			span := u.Span
			span.File = logical[span.File]
			k := reasonKey{u.Code, span}
			if reasons[k] == nil {
				reasons[k] = map[DecidableBy]bool{}
			}
			for _, a := range u.DecidableBy {
				reasons[k][a] = true
			}
		}
	}
	for p, complete := range paths {
		if complete {
			out.Read = append(out.Read, p)
		} else {
			out.Unread = append(out.Unread, p)
		}
	}
	sort.Strings(out.Read)
	sort.Strings(out.Unread)
	for k, actions := range reasons {
		u := UnknownReason{Code: k.code, Span: k.span, DecidableBy: []DecidableBy{}}
		for a := range actions {
			u.DecidableBy = append(u.DecidableBy, a)
		}
		sort.Slice(u.DecidableBy, func(i, j int) bool { return u.DecidableBy[i] < u.DecidableBy[j] })
		out.Unknowns = append(out.Unknowns, u)
	}
	sort.Slice(out.Unknowns, func(i, j int) bool {
		a, b := out.Unknowns[i], out.Unknowns[j]
		if a.Span != b.Span {
			return a.Span.less(b.Span)
		}
		return a.Code < b.Code
	})
	return out
}

// logicalEvidencePaths maps evidence names only; legacy resource/attribute spans
// still name the caller's input. Relative names are already logical and stay exact.
// Absolute names use the shortest unambiguous suffix across the whole observation
// set, so basename collisions never combine inputs or detach their reasons. No
// working directory or file-system access is needed to choose these names.
func logicalEvidencePaths(observations []ReadObservation) map[string]string {
	names := map[string]string{}
	for _, o := range observations {
		names[o.Path] = o.Path
		for _, u := range o.Unknowns {
			names[u.Span.File] = u.Span.File
		}
	}
	used := map[string]bool{}
	suffixes := map[string][]string{}
	counts := map[string]int{}
	absolute := []string{}
	for name := range names {
		if !filepath.IsAbs(name) {
			used[name] = true
			continue
		}
		absolute = append(absolute, name)
		// Strip the volume/root before making suffixes. Clean only absolute names:
		// relative spellings and distinct caller identities must not be merged.
		clean := filepath.ToSlash(filepath.Clean(name))
		clean = strings.TrimLeft(strings.TrimPrefix(clean, filepath.ToSlash(filepath.VolumeName(name))), "/")
		parts := strings.Split(clean, "/")
		for i := len(parts) - 1; i >= 0; i-- {
			suffix := strings.Join(parts[i:], "/")
			if suffix == "" {
				continue
			}
			suffixes[name] = append(suffixes[name], suffix)
			counts[suffix]++
		}
	}
	sort.Strings(absolute)
	for index, name := range absolute {
		chosen := ""
		for _, suffix := range suffixes[name] {
			if counts[suffix] == 1 && !used[suffix] {
				chosen = suffix
				break
			}
		}
		if chosen == "" {
			// A root-level absolute name may collide with a relative name, or distinct
			// absolute spellings may clean to the same path. Allocate a deterministic
			// unused logical namespace rather than drop or merge either input.
			base := path.Base(filepath.ToSlash(filepath.Clean(name)))
			if base == "/" || base == "." {
				base = "input"
			}
			for n := index + 1; ; n++ {
				chosen = "input-" + strconv.Itoa(n) + "/" + base
				if !used[chosen] && counts[chosen] == 0 {
					break
				}
			}
		}
		names[name] = chosen
		used[chosen] = true
	}
	return names
}

// recordReadGaps describes the existing reader's incomplete syntax and dropped sections.
func recordReadGaps(o *ReadObservation, syntax, dropped int) {
	if syntax > 0 {
		o.Complete = false
		o.Unknowns = append(o.Unknowns, UnknownReason{Code: UnknownSyntax, Span: Span{File: o.Path}, DecidableBy: []DecidableBy{}})
	}
	if dropped > 0 {
		o.Complete = false
		// The reader records each dropped section at its own span.
	}
}

// resourceReadEvidence adapts the classes already produced by a reader. There is no
// second parse, source retention or value-dependent classification here.
func resourceReadEvidence(set *ResourceSet, observations []ReadObservation) ReadEvidence {
	action := DecidePlan
	if set.Tool != ToolTerraform {
		action = DecideRender
	}
	unknowns := []UnknownReason{}
	add := func(class Class, span Span) {
		code := UnknownExpression
		if class == ClassUnknown {
			code = UnknownUnsupportedForm
		} else if class != ClassExpression {
			return
		}
		unknowns = append(unknowns, UnknownReason{Code: code, Span: span, DecidableBy: []DecidableBy{action}})
	}
	for _, r := range set.Resources {
		for _, a := range r.Attrs {
			add(a.Class, a.Span)
		}
		add(r.Multiplicity.Class, r.Multiplicity.Span)
		if r.Mode == ModeModule {
			unknowns = append(unknowns, UnknownReason{Code: UnknownUnsupportedForm, Span: r.Span, DecidableBy: []DecidableBy{DecideVendoredModule}})
		}
	}
	// Reasons attach to existing file observations, never create a read from a span.
	byPath := map[string]int{}
	for i, o := range observations {
		byPath[o.Path] = i
	}
	for _, u := range unknowns {
		if i, ok := byPath[u.Span.File]; ok {
			observations[i].Unknowns = append(observations[i].Unknowns, u)
		}
	}
	return BuildReadEvidence(observations)
}
