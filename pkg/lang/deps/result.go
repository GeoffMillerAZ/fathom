package deps

import (
	"cmp"
	"io/fs"
	"regexp"
	"slices"
	"strings"
)

type admission uint8

const (
	unmeasured admission = iota
	measured
	detectOnly
)

// Only preregistered dialect readings set this table. The Go fixture is a
// control of the grading instrument and supplies no admission.
var admissions = map[string]admission{}

const maxIdentityBytes = 512

var (
	pathToken     = regexp.MustCompile(`^[A-Za-z0-9._@+~ -]+(/[A-Za-z0-9._@+~ -]+)*$`)
	dialectToken  = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
	externalToken = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}:@?[A-Za-z0-9._+~-]+(:[0-9]+/|/)?[A-Za-z0-9._+~/\-]*$`)
	externalBad   = regexp.MustCompile(`^repo:|//|/$|[:/]\.\.?(/|$)`)
	versionToken  = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._+!~-]*|sha(256|384|512):[0-9A-Fa-f]+)$`)
)

func validPath(s string) bool {
	return s != "." && len(s) <= maxIdentityBytes && fs.ValidPath(s) && pathToken.MatchString(s)
}

func validDialect(s string) bool { return dialectToken.MatchString(s) }

func validExternal(s string) bool {
	return len(s) <= maxIdentityBytes && externalToken.MatchString(s) && !externalBad.MatchString(s)
}

func safeRaw(s string) string {
	if s == "" || s == "." || (validPath(s) && !strings.Contains(s, " ")) {
		return s
	}
	return "***"
}

func validEdge(e Edge) bool {
	switch e.Kind {
	case Import, Include, Embed, BuildInput, Exec, ConfigRef:
	default:
		return false
	}
	if e.Line < 0 {
		return false
	}
	switch e.Resolution {
	case File:
		return e.Version == "" && validPath(e.To)
	case Directory:
		return e.Version == "" && (validPath(e.To) || e.To == ".")
	case External:
		return validExternal(e.To) && len(e.Version) <= maxIdentityBytes &&
			(e.Version == "" || versionToken.MatchString(e.Version))
	case UnknownTarget:
		return e.To == "" && e.Version == ""
	}
	return false
}

func validUnknownCode(c UnknownCode) bool {
	switch c {
	case DynamicImport, ComputedPath, UnresolvedAlias, MissingConfig, UnboundedGlob,
		UnsupportedConstruct, ParseError, UnmeasuredReader, ExternalUnpinned, ReadLimit:
		return true
	}
	return false
}

func validEvidence(s DecidableBy) bool {
	switch s {
	case Toolchain, Lockfile, ConfigFile, Observation, Declaration:
		return true
	}
	return false
}

type result struct{ FileDeps }

func newResult(file, dialect string) result {
	return result{FileDeps{Path: file, Dialect: dialect, Edges: []Edge{}, Unknowns: []Unknown{}}}
}

func (f *result) addEdge(e Edge) {
	switch e.Provenance {
	case Builtin, Pack, Extension, Declared:
	default:
		f.addUnknown(Unknown{Code: UnsupportedConstruct, Line: e.Line})
		return
	}
	if !validEdge(e) {
		f.addUnknown(Unknown{Code: UnsupportedConstruct, Line: e.Line})
		return
	}
	e.From, e.Line, e.Raw = f.Path, max(1, e.Line), safeRaw(e.Raw)
	f.Edges = append(f.Edges, e)
	if e.Resolution == External && e.Version == "" {
		f.addUnknown(Unknown{Code: ExternalUnpinned, Line: e.Line, Raw: e.Raw,
			DecidableBy: []DecidableBy{Lockfile, ConfigFile}})
	}
}

func (f *result) addUnknown(u Unknown) {
	u.From, u.Line = f.Path, max(1, u.Line)
	sources := []DecidableBy{}
	if !validUnknownCode(u.Code) {
		u.Code, u.Raw = UnsupportedConstruct, ""
	} else {
		u.Raw = safeRaw(u.Raw)
		if (u.Code == DynamicImport || u.Code == ComputedPath) && u.Raw != "" {
			u.Raw = "***"
		}
		for _, source := range u.DecidableBy {
			if validEvidence(source) {
				sources = append(sources, source)
			}
		}
	}
	slices.Sort(sources)
	u.DecidableBy = slices.Compact(sources)
	f.Unknowns = append(f.Unknowns, u)
}

func (f *result) finish(state admission, known, trust bool) {
	f.Measured = known && state == measured
	f.Trusted = known && state == unmeasured && trust
	if !known {
		f.addUnknown(Unknown{Code: UnsupportedConstruct})
	}
	if !f.Measured && !f.Trusted {
		f.addUnknown(Unknown{Code: UnmeasuredReader})
	}
	f.Complete = len(f.Unknowns) == 0 && (f.Measured || f.Trusted)

	slices.SortFunc(f.Edges, compareEdges)
	f.Edges = slices.Compact(f.Edges)
	slices.SortFunc(f.Unknowns, compareUnknowns)
	f.Unknowns = slices.CompactFunc(f.Unknowns, func(a, b Unknown) bool { return compareUnknowns(a, b) == 0 })
}

func compareEdges(a, b Edge) int {
	return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Raw, b.Raw),
		cmp.Compare(a.Resolution, b.Resolution), cmp.Compare(a.To, b.To),
		cmp.Compare(a.Version, b.Version), cmp.Compare(a.Provenance, b.Provenance))
}

func compareUnknowns(a, b Unknown) int {
	return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Code, b.Code),
		cmp.Compare(a.Raw, b.Raw), slices.Compare(a.DecidableBy, b.DecidableBy))
}

func (f *result) resolveExtension(repo fs.FS, resolver Resolver, raw string, line int) {
	edges, unknown := resolver.Resolve(repo, f.Path, raw)
	requiresUnknown := len(edges) == 0
	for _, e := range edges {
		e.Line, e.Raw, e.Provenance = line, raw, Extension
		requiresUnknown = requiresUnknown || e.Resolution == UnknownTarget
		if validEdge(e) && (e.Resolution == File || e.Resolution == Directory) {
			info, err := fs.Lstat(repo, e.To)
			if err != nil || (e.Resolution == File && !info.Mode().IsRegular()) ||
				(e.Resolution == Directory && !info.IsDir()) {
				requiresUnknown = true
				continue
			}
		}
		f.addEdge(e)
	}
	if unknown != nil {
		u := *unknown
		u.Line, u.Raw = line, raw
		f.addUnknown(u)
	} else if requiresUnknown {
		f.addUnknown(Unknown{Code: UnresolvedAlias, Raw: raw, Line: line})
	}
}
