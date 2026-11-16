package iacfacts

import "sort"

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
	paths := map[string]bool{}
	type reasonKey struct {
		code UnknownCode
		span Span
	}
	reasons := map[reasonKey]map[DecidableBy]bool{}
	for _, o := range observations {
		complete := o.Read && o.Complete
		if previous, exists := paths[o.Path]; exists {
			complete = complete && previous
		}
		paths[o.Path] = complete
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
			k := reasonKey{u.Code, u.Span}
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
