package boundaries

import (
	"cmp"
	"io/fs"
	"regexp"
	"slices"
	"strings"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
)

const withheld = "***"

// FileDeps can also be assembled by callers. Apply the dependency contract's
// identity and redaction rules before using those records as evidence.
var (
	dependencyPath      = regexp.MustCompile(`^[A-Za-z0-9._@+~ -]+(/[A-Za-z0-9._@+~ -]+)*$`)
	dependencyExternal  = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}:@?[A-Za-z0-9._+~-]+(:[0-9]+/|/)?[A-Za-z0-9._+~/\-]*$`)
	invalidExternalPart = regexp.MustCompile(`^repo:|//|/$|[:/]\.\.?(/|$)`)
	dependencyVersion   = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._+!~-]*|sha(256|384|512):[0-9A-Fa-f]+)$`)
)

func validDependencyPath(name string) bool {
	return len(name) <= 512 && name != "." && fs.ValidPath(name) && dependencyPath.MatchString(name)
}

func safeOperand(raw string) string {
	switch raw {
	case "", ".", withheld:
		return raw
	}
	if validDependencyPath(raw) && !strings.Contains(raw, " ") {
		return raw
	}
	return withheld
}

// checkedFiles leaves unsafe identities out of the evidence. If a file path
// is malformed, its scope is unknowable, so every rule discloses the gap.
func checkedFiles(files []deps.FileDeps) ([]deps.FileDeps, bool) {
	checked := make([]deps.FileDeps, 0, len(files))
	unknownScope := false
	for _, file := range files {
		if !validDependencyPath(file.Path) {
			unknownScope = true
			continue
		}
		file.Complete = file.Complete && (file.Measured || file.Trusted) && len(file.Unknowns) == 0
		if file.Dialect != "" && !ruleID.MatchString(file.Dialect) {
			file.Dialect = ""
			file.Complete = false
		}
		var imports []deps.Edge
		for _, edge := range file.Edges {
			if edge.Kind != deps.KindImport {
				continue
			}
			if !validImport(edge, file.Path) {
				file.Complete = false
				continue
			}
			edge.Raw = safeOperand(edge.Raw)
			imports = append(imports, edge)
		}
		file.Edges = imports
		file.Unknowns = checkedUnknowns(file.Path, file.Unknowns)
		checked = append(checked, file)
	}
	return checked, unknownScope
}

func validImport(edge deps.Edge, file string) bool {
	if edge.From != file || edge.Line < 0 {
		return false
	}
	switch edge.Provenance {
	case deps.ProvenanceBuiltin, deps.ProvenancePack, deps.ProvenanceExtension, deps.ProvenanceDeclared:
	default:
		return false
	}
	switch edge.Resolution {
	case deps.ResolvedFile:
		return edge.Version == "" && validDependencyPath(edge.To)
	case deps.ResolvedDirectory:
		return edge.Version == "" && (edge.To == "." || validDependencyPath(edge.To))
	case deps.ResolvedExternal:
		return len(edge.To) <= 512 && dependencyExternal.MatchString(edge.To) && !invalidExternalPart.MatchString(edge.To) &&
			(edge.Version == "" || len(edge.Version) <= 512 && dependencyVersion.MatchString(edge.Version))
	default:
		return false
	}
}

func checkedUnknowns(file string, in []deps.Unknown) []deps.Unknown {
	out := make([]deps.Unknown, 0, len(in))
	for _, unknown := range in {
		switch unknown.Code {
		case deps.UnknownDynamicImport, deps.UnknownComputedPath, deps.UnknownUnresolvedAlias, deps.UnknownMissingConfig,
			deps.UnknownUnboundedGlob, deps.UnknownUnsupportedConstruct, deps.UnknownParseError, deps.UnknownUnmeasuredReader,
			deps.UnknownExternalUnpinned, deps.UnknownReadLimit:
		default:
			continue
		}
		sources := make([]deps.DecidableBy, 0, len(unknown.DecidableBy))
		for _, source := range unknown.DecidableBy {
			switch source {
			case deps.DecideToolchain, deps.DecideLockfile, deps.DecideConfigFile, deps.DecideObservation, deps.DecideDeclaration:
				sources = append(sources, source)
			}
		}
		slices.Sort(sources)
		out = append(out, deps.Unknown{
			From: file, Code: unknown.Code, Raw: safeOperand(unknown.Raw), Line: max(0, unknown.Line), DecidableBy: sources,
		})
	}
	slices.SortFunc(out, compareUnknowns)
	return out
}

func compareUnknowns(a, b deps.Unknown) int {
	return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Code, b.Code), cmp.Compare(a.Raw, b.Raw),
		slices.Compare(a.DecidableBy, b.DecidableBy))
}

func compareUnverified(a, b UnverifiedFile) int {
	return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Dialect, b.Dialect), compareFlag(a.Measured, b.Measured),
		compareFlag(a.Trusted, b.Trusted), slices.CompareFunc(a.Unknowns, b.Unknowns, compareUnknowns))
}

func compareFlag(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	default:
		return -1
	}
}
