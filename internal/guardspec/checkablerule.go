// checkablerule.go — SR-45 (docs/plans/surface-reckoning.md, design doc §5
// row SR-45, F-26): a served LayerRule (VB-13, layerrule.go) is
// "checkable" exactly when its kind has a deterministic pre-write check —
// the SAME boundary EvaluateLayerRuleEnforcement already draws (enforce.go:
// only import_direction and boundary can be evaluated against a single
// proposed file's own text; no_cycles and cross_service_direction need the
// full index/fleet graph). guardFromCheckableRule compiles a checkable
// rule into an enforce:true guard so a caller (a layer-pack compiler, a
// `fathom rules` verb) can promote a served, advisory rule to enforcement
// without hand-editing `.fathom/guard.cue`; a rule this package cannot
// mechanically check is reported not-checkable (ok=false) instead of
// silently enforced — the "lint output" a caller renders for it.
package guardspec

import (
	"fmt"
	"path"
	"reflect"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/literal"
	"cuelang.org/go/cue/parser"

	"github.com/geoffmilleraz/fathom/internal/pathglob"
)

// LayerRuleIsCheckable reports whether rule's kind has a deterministic
// pre-write check (EvaluateLayerRuleEnforcement) — the vocabulary the
// obligations answer marks a served rule with (SR-45): import_direction
// and boundary are; no_cycles and cross_service_direction are not.
func LayerRuleIsCheckable(rule LayerRule) bool {
	switch rule.Kind {
	case ImportDirection, Boundary:
		return true
	default:
		return false
	}
}

// guardFromCheckableRule compiles rule into an enforce:true LayerRule when
// it is checkable (LayerRuleIsCheckable), matching LoadLayerRules' own
// refusal for any other kind (ADR-038: "enforce:true is not supported for
// kind %q"). ok is false for a not-checkable rule; compiled is then the
// zero LayerRule, never a partially-enforced one. Idempotent: a rule
// already carrying Enforce:true compiles to itself unchanged.
func guardFromCheckableRule(rule LayerRule) (compiled LayerRule, ok bool) {
	if !LayerRuleIsCheckable(rule) {
		return LayerRule{}, false
	}
	compiled = rule
	compiled.Enforce = true
	return compiled, true
}

// GuardFromCheckableRule is guardFromCheckableRule's exported face: the
// compile step `fathom obligations promote` (SR-45w, cmd/fathom
// obligations.go) runs a served rule through before it writes anything.
func GuardFromCheckableRule(rule LayerRule) (LayerRule, bool) {
	return guardFromCheckableRule(rule)
}

// NotCheckableReason is why rule has no deterministic pre-write check —
// the reason the obligations answer shows beside a not-checkable rule and
// the refusal `obligations promote` prints. "" for a checkable rule.
func NotCheckableReason(rule LayerRule) string {
	switch {
	case LayerRuleIsCheckable(rule):
		return ""
	case rule.Kind == NoCycles:
		return "no_cycles needs the whole import graph; one proposed file cannot show a cycle, so there is no pre-write check (ADR-038)"
	case rule.Kind == CrossServiceDirection:
		return "cross_service_direction needs the fleet's cross-service edges; one proposed file cannot show them, so there is no pre-write check (ADR-038)"
	default:
		return fmt.Sprintf("kind %q has no deterministic pre-write check (ADR-038)", rule.Kind)
	}
}

// LayerRuleGoverns reports whether rule constrains repoRelFile of service:
// the package directory a proposed import there would come from is the
// rule's `from` (import_direction), outside its `allow` list (boundary —
// any other importer can break it), inside its `scope` (no_cycles), or the
// service is its `from_service` (cross_service_direction). An empty
// repoRelFile is the root-scope question: every rule governs it.
func LayerRuleGoverns(rule LayerRule, service, repoRelFile string) bool {
	if repoRelFile == "" {
		return true
	}
	dir := path.Dir(repoRelFile)
	switch rule.Kind {
	case ImportDirection:
		return pathglob.MatchAny(rule.From, dir)
	case Boundary:
		return !pathglob.MatchAny(rule.Allow, dir)
	case NoCycles:
		return pathglob.MatchAny(rule.Scope, dir)
	case CrossServiceDirection:
		return service != "" && rule.FromService == service
	default:
		return false
	}
}

// NotCheckableError refuses to promote a rule with no pre-write check.
type NotCheckableError struct {
	ID     string
	Kind   LayerRuleKind
	Reason string
}

func (e *NotCheckableError) Error() string {
	return fmt.Sprintf("%s (%s) is not checkable: %s; it stays advisory", e.ID, e.Kind, e.Reason)
}

// Promotion is PromoteLayerRuleSource's result: the compiled enforce:true
// rule, the rule's stanza as the file reads after the promotion, and the
// whole file. Changed is false when the rule already carried enforce:
// true (Source is then the input, byte for byte).
type Promotion struct {
	Rule    LayerRule
	Stanza  string
	Source  []byte
	Changed bool
}

// PromoteLayerRuleSource compiles the layer rule id in a `.fathom/guard.cue`
// file's bytes to an enforce:true guard (GuardFromCheckableRule) and
// returns the file with exactly that change: one `enforce: true` line
// added after the rule's `id` (aligned with it), or an existing `enforce`
// value replaced. Every other byte — comments, the other rules, the
// `guards` half — is untouched, and the result must load back through the
// same decoder with only that rule changed, or nothing is returned.
//
// Refused rather than approximated, with the manual edit named: a file
// that does not parse or does not decode, an unknown id, a not-checkable
// rule (*NotCheckableError), and a rule the writer cannot see as one
// literal stanza in a top-level `layer_rules: [...]` list.
func PromoteLayerRuleSource(filename string, data []byte, id string) (Promotion, error) {
	v := cuecontext.New().CompileBytes(data, cue.Filename(filename))
	if v.Err() != nil {
		return Promotion{}, fmt.Errorf("%s does not parse (%v); fix it by hand first", filename, v.Err())
	}
	set, err := decodeLayerRules(filename, v)
	if err != nil {
		return Promotion{}, err
	}
	var rule LayerRule
	var ids []string
	found := false
	for _, r := range set.Rules {
		ids = append(ids, r.ID)
		if r.ID == id {
			rule, found = r, true
		}
	}
	if !found {
		return Promotion{}, fmt.Errorf("no layer rule %q in %s (its rules: %s)", id, filename, strings.Join(ids, ", "))
	}
	compiled, ok := GuardFromCheckableRule(rule)
	if !ok {
		return Promotion{}, &NotCheckableError{ID: rule.ID, Kind: rule.Kind, Reason: NotCheckableReason(rule)}
	}

	lit, err := findLayerRuleStanza(filename, data, id)
	if err != nil {
		return Promotion{}, err
	}
	start, end := lit.Pos().Offset(), lit.End().Offset()
	// The stanza is shown with its own line's indentation, so it reads as
	// it sits in the file.
	indent := start
	for indent > 0 && (data[indent-1] == ' ' || data[indent-1] == '\t') {
		indent--
	}
	if indent > 0 && data[indent-1] != '\n' {
		indent = start
	}
	if rule.Enforce {
		return Promotion{Rule: compiled, Stanza: string(data[indent:end]), Source: data}, nil
	}

	var enforceFld, idFld *ast.Field
	for _, d := range lit.Elts {
		fld, ok := d.(*ast.Field)
		if !ok {
			continue
		}
		switch name, _, _ := ast.LabelName(fld.Label); name {
		case "enforce":
			enforceFld = fld
		case "id":
			idFld = fld
		}
	}
	var at, cut int
	var insert string
	switch {
	case enforceFld != nil:
		at, cut, insert = enforceFld.Value.Pos().Offset(), enforceFld.Value.End().Offset(), "true"
	case idFld != nil:
		at, cut, insert = enforceInsertion(data, idFld)
	default:
		return Promotion{}, fmt.Errorf("layer rule %q in %s has no literal id field to place `enforce: true` beside; add it by hand", id, filename)
	}
	out := make([]byte, 0, len(data)+len(insert))
	out = append(out, data[:at]...)
	out = append(out, insert...)
	out = append(out, data[cut:]...)

	// The rewrite must load back with exactly this one change.
	nv := cuecontext.New().CompileBytes(out, cue.Filename(filename))
	if nv.Err() != nil {
		return Promotion{}, fmt.Errorf("promoting %q would leave %s unparseable (%v); add `enforce: true` by hand", id, filename, nv.Err())
	}
	nset, err := decodeLayerRules(filename, nv)
	if err != nil || len(nset.Rules) != len(set.Rules) {
		return Promotion{}, fmt.Errorf("promoting %q would change %s beyond the one rule (%v); add `enforce: true` by hand", id, filename, err)
	}
	for i, r := range nset.Rules {
		want := set.Rules[i]
		if want.ID == id {
			want = compiled
		}
		if !reflect.DeepEqual(r, want) {
			return Promotion{}, fmt.Errorf("promoting %q would change rule %q in %s; add `enforce: true` by hand", id, r.ID, filename)
		}
	}
	delta := len(insert) - (cut - at)
	return Promotion{Rule: compiled, Stanza: string(out[indent : end+delta]), Source: out, Changed: true}, nil
}

// findLayerRuleStanza returns the struct literal in the top-level
// `layer_rules: [...]` list whose literal `id` is id.
func findLayerRuleStanza(filename string, data []byte, id string) (*ast.StructLit, error) {
	f, err := parser.ParseFile(filename, data, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("%s does not parse (%v); fix it by hand first", filename, err)
	}
	for _, d := range f.Decls {
		fld, ok := d.(*ast.Field)
		if !ok {
			continue
		}
		if name, _, lerr := ast.LabelName(fld.Label); lerr != nil || name != "layer_rules" {
			continue
		}
		list, ok := fld.Value.(*ast.ListLit)
		if !ok {
			continue
		}
		for _, e := range list.Elts {
			lit, ok := e.(*ast.StructLit)
			if !ok {
				continue
			}
			for _, sd := range lit.Elts {
				sf, ok := sd.(*ast.Field)
				if !ok {
					continue
				}
				if name, _, _ := ast.LabelName(sf.Label); name != "id" {
					continue
				}
				if bl, ok := sf.Value.(*ast.BasicLit); ok {
					if s, uerr := literal.Unquote(bl.Value); uerr == nil && s == id {
						return lit, nil
					}
				}
			}
		}
	}
	return nil, fmt.Errorf("layer rule %q in %s is not one literal stanza in a top-level `layer_rules: [...]` list; add `enforce: true` to it by hand", id, filename)
}

// enforceInsertion places `enforce: true` after the rule's id field: on a
// line of its own, indented and value-aligned like the id line, when the
// id line holds nothing after its value but a comma or a comment; right
// after the value, comma-separated, in a one-line stanza.
func enforceInsertion(data []byte, idFld *ast.Field) (at, cut int, insert string) {
	valEnd := idFld.Value.End().Offset()
	lineEnd := valEnd
	for lineEnd < len(data) && data[lineEnd] != '\n' {
		lineEnd++
	}
	tail := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(data[valEnd:lineEnd])), ","))
	if tail != "" && !strings.HasPrefix(tail, "//") {
		return valEnd, valEnd, ", enforce: true" // a one-line stanza: more fields follow on this line
	}
	fldStart := idFld.Pos().Offset()
	lineStart := fldStart
	for lineStart > 0 && data[lineStart-1] != '\n' {
		lineStart--
	}
	indent := string(data[lineStart:fldStart])
	if strings.TrimSpace(indent) != "" {
		return valEnd, valEnd, ", enforce: true"
	}
	const label = "enforce:"
	pad := idFld.Value.Pos().Offset() - fldStart - len(label)
	if pad < 1 {
		pad = 1
	}
	return lineEnd, lineEnd, "\n" + indent + label + strings.Repeat(" ", pad) + "true"
}
