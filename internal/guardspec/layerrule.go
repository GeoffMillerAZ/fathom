// layerrule.go — the `layer_rules` half of a `.fathom/guard.cue` file
// (VB-13, docs/vision/29-verbs-not-lookups.md §3 wave C; ADR-035 decision
// 3): compiled governance rules checked live against the index at
// obligations time (EvaluateLayerRule, layerrule_eval.go) and compiled to
// a Go test (`fathom rules gen-test`, internal/guardspec/layerrules) so
// the same rule text fails `task validate` and any CI, not just an
// agent's edit — "governance moves from advice to enforcement."
//
// Mirrors LoadGuards' LoadX shape exactly: a lenient READER against the
// SAME `.fathom/guard.cue` file, three-state absent/malformed handling
// (see LoadGuards' doc comment), unknown fields never inspected. Kind-
// specific required fields are enforced here in Go, not in the CUE schema
// — the same split LoadGuards uses for "at least one of scopes/
// command_scopes": CUE cannot cleanly express "these fields are required
// only for THIS kind" on an open struct.
package guardspec

import (
	"fmt"
	"os"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// LayerRuleKind selects which structural check a #LayerRule performs.
type LayerRuleKind string

const (
	// ImportDirection: packages matching any `From` glob may not import
	// packages matching any `To` glob.
	ImportDirection LayerRuleKind = "import_direction"
	// NoCycles: no import cycle may exist among packages matching `Scope`.
	NoCycles LayerRuleKind = "no_cycles"
	// Boundary: only a package matching an `Allow` glob may import a
	// package matching a `Guarded` glob.
	Boundary LayerRuleKind = "boundary"
	// CrossServiceDirection: FromService must never depend on ToService,
	// over the fleet's typed cross-service edges.
	CrossServiceDirection LayerRuleKind = "cross_service_direction"
)

// Severity controls whether a violated rule fails the generated test
// (`fathom rules gen-test`): SeverityError does, SeverityWarn is served
// live but never fails it.
type Severity string

const (
	SeverityError Severity = "error"
	SeverityWarn  Severity = "warn"
)

// LayerRule is one decoded #LayerRule entry.
type LayerRule struct {
	ID       string
	Kind     LayerRuleKind
	Sentence string
	Severity Severity

	// import_direction
	From []string
	To   []string

	// no_cycles
	Scope []string

	// boundary
	Allow   []string
	Guarded []string

	// cross_service_direction
	FromService string
	ToService   string
}

// LayerRuleSet is the decoded runtime form of a `.fathom/guard.cue` file's
// `layer_rules` list.
type LayerRuleSet struct {
	Rules []LayerRule
}

// LoadLayerRules reads and decodes path's "layer_rules" field.
//
// Same three-state contract as LoadGuards (see its doc comment): the
// top-level "layer_rules" field being ABSENT (a guard.cue predating this
// field, or one that only sets "guards") is an EMPTY config — nil error,
// zero Rules — never the same as an explicit `layer_rules: []`, which is
// present-but-declaring-nothing and stays a loud error only when it is
// actually malformed (an empty list on its own is the sanctioned inert
// form, same IO-7 vocabulary `guards: []` already established).
func LoadLayerRules(path string) (LayerRuleSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LayerRuleSet{}, fmt.Errorf("guardspec: read %s: %w", path, err)
	}
	cctx := cuecontext.New()
	v := cctx.CompileBytes(data, cue.Filename(path))
	if v.Err() != nil {
		return LayerRuleSet{}, &LoadError{Path: path, Fields: []*FieldError{fieldErr(path, "<parse>", v.Err())}}
	}
	return decodeLayerRules(path, v)
}

func decodeLayerRules(path string, v cue.Value) (LayerRuleSet, error) {
	list := v.LookupPath(cue.ParsePath("layer_rules"))
	if !list.Exists() {
		// Absent, not malformed — see the doc comment above.
		return LayerRuleSet{}, nil
	}
	iter, err := list.List()
	if err != nil {
		return LayerRuleSet{}, &LoadError{Path: path, Fields: []*FieldError{fieldErr(path, "layer_rules", err)}}
	}

	var errs []*FieldError
	var out LayerRuleSet
	seen := map[string]bool{}
	idx := 0
	for iter.Next() {
		rv := iter.Value()
		prefix := fmt.Sprintf("layer_rules[%d]", idx)
		idx++

		r := LayerRule{
			ID:          lookupString(rv, "id"),
			Kind:        LayerRuleKind(lookupString(rv, "kind")),
			Sentence:    lookupString(rv, "sentence"),
			Severity:    Severity(lookupStringDefault(rv, "severity", string(SeverityError))),
			From:        lookupStrings(rv, "from"),
			To:          lookupStrings(rv, "to"),
			Scope:       lookupStrings(rv, "scope"),
			Allow:       lookupStrings(rv, "allow"),
			Guarded:     lookupStrings(rv, "guarded"),
			FromService: lookupString(rv, "from_service"),
			ToService:   lookupString(rv, "to_service"),
		}

		if r.ID == "" {
			errs = append(errs, fieldErr(path, prefix+".id", fmt.Errorf("required")))
		} else if seen[r.ID] {
			errs = append(errs, fieldErr(path, prefix+".id", fmt.Errorf("duplicate id %q — every layer rule needs a repo-unique id", r.ID)))
		}
		seen[r.ID] = true
		if r.Sentence == "" {
			errs = append(errs, fieldErr(path, prefix+".sentence", fmt.Errorf("required")))
		}
		switch r.Severity {
		case SeverityError, SeverityWarn:
		default:
			errs = append(errs, fieldErr(path, prefix+".severity", fmt.Errorf("must be \"error\" or \"warn\", got %q", r.Severity)))
		}
		switch r.Kind {
		case ImportDirection:
			if len(r.From) == 0 || len(r.To) == 0 {
				errs = append(errs, fieldErr(path, prefix, fmt.Errorf("kind %q requires non-empty from and to", r.Kind)))
			}
		case NoCycles:
			if len(r.Scope) == 0 {
				errs = append(errs, fieldErr(path, prefix+".scope", fmt.Errorf("kind %q requires a non-empty scope", r.Kind)))
			}
		case Boundary:
			if len(r.Allow) == 0 || len(r.Guarded) == 0 {
				errs = append(errs, fieldErr(path, prefix, fmt.Errorf("kind %q requires non-empty allow and guarded", r.Kind)))
			}
		case CrossServiceDirection:
			if r.FromService == "" || r.ToService == "" {
				errs = append(errs, fieldErr(path, prefix, fmt.Errorf("kind %q requires from_service and to_service", r.Kind)))
			}
		default:
			errs = append(errs, fieldErr(path, prefix+".kind", fmt.Errorf("unknown kind %q", r.Kind)))
		}
		out.Rules = append(out.Rules, r)
	}
	if len(errs) > 0 {
		return LayerRuleSet{}, &LoadError{Path: path, Fields: errs}
	}
	return out, nil
}

func lookupStringDefault(v cue.Value, p, def string) string {
	fv := v.LookupPath(cue.ParsePath(p))
	if !fv.Exists() {
		return def
	}
	s, err := fv.String()
	if err != nil {
		return def
	}
	return s
}
