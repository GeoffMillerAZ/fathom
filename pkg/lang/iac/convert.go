package iac

import "github.com/geoffmilleraz/fathom/internal/iacfacts"

// convert maps the reader's set onto the public types. Every slice is non-nil, so equal inputs give byte-equal JSON.
func convert(in *iacfacts.ResourceSet) *ResourceSet {
	out := &ResourceSet{
		Evidence:     convertEvidence(in.Evidence),
		Resources:    make([]Resource, 0, len(in.Resources)),
		SyntaxErrors: in.SyntaxErrors,
		Dropped:      make([]Dropped, 0, len(in.Dropped)),
	}
	for _, r := range in.Resources {
		res := Resource{
			Tool: r.Tool, Mode: r.Mode, Type: r.Type, Name: r.Name, Provider: r.Provider,
			Span:  span(r.Span),
			Attrs: make([]Attr, 0, len(r.Attrs)),
			Multiplicity: Multiplicity{
				Kind: MultiplicityKind(r.Multiplicity.Kind), Class: Class(r.Multiplicity.Class),
				Refs: refs(r.Multiplicity.Refs), Span: span(r.Multiplicity.Span),
			},
		}
		for _, a := range r.Attrs {
			res.Attrs = append(res.Attrs, Attr{Path: a.Path, Class: Class(a.Class), Value: a.Value, Refs: refs(a.Refs), Span: span(a.Span)})
		}
		out.Resources = append(out.Resources, res)
	}
	for _, d := range in.Dropped {
		out.Dropped = append(out.Dropped, Dropped{Name: d})
	}
	return out
}

func span(s iacfacts.Span) Span {
	return Span{File: s.File, StartLine: s.StartLine, StartCol: s.StartCol, EndLine: s.EndLine, EndCol: s.EndCol}
}

func refs(in []string) []string {
	return append([]string{}, in...)
}

// convertEvidence copies every reason, action and span without sharing backing slices.
func convertEvidence(in iacfacts.ReadEvidence) ReadEvidence {
	out := ReadEvidence{Read: refs(in.Read), Unread: refs(in.Unread), Unknowns: make([]UnknownReason, 0, len(in.Unknowns))}
	for _, u := range in.Unknowns {
		reason := UnknownReason{Code: UnknownCode(u.Code), Span: span(u.Span), DecidableBy: make([]DecidableBy, 0, len(u.DecidableBy))}
		for _, a := range u.DecidableBy {
			reason.DecidableBy = append(reason.DecidableBy, DecidableBy(a))
		}
		out.Unknowns = append(out.Unknowns, reason)
	}
	return out
}
