package readout

// pack.go — RO-34 (docs/design/readouts.md section 11, fathom-dir design
// section 7.2): a tech pack's `readouts` key loads as repository scope with
// RO-33's add-only limit. Packs come from techpack.Discover (the
// repository's own pins, offline). A pack's rules rank below the
// repository's own file, which ranks below user scope. The pin is the trust,
// so a pack rule needs no sha256 entry, but a rule with no fixture, or a
// failing one, is withheld and named, never served.

import (
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/geoffmilleraz/fathom/internal/techpack"
)

// PackReadouts is one pack's readout rules as the loader saw them: the ids
// it serves and the refusals it names.
type PackReadouts struct {
	Pack     string // name@version
	Served   []string
	Withheld []Refusal
}

// LoadPackRules loads the readout rules of root's pinned tech packs into rs
// below every rule already there. It never fails: a pack that cannot be
// read, and each rule that cannot be served, is a Refusal. No pack, or no
// pack carrying readouts, is silent.
func LoadPackRules(rs *RuleSet, root string) {
	if rs == nil || root == "" {
		return
	}
	d, err := techpack.Discover(root)
	if err != nil {
		return // pins that cannot be read: nothing to serve, and doctor's pack rows name it
	}
	loadPackRules(rs, d.Packs)
}

func loadPackRules(rs *RuleSet, packs []techpack.Pack) {
	for _, p := range packs {
		if len(p.Readouts) == 0 {
			continue
		}
		src := filepath.Join(p.Dir, techpack.PackFile)
		pr := PackReadouts{Pack: p.Tag()}
		withhold := func(id, reason string) {
			f := Refusal{Source: src, ID: id, Reason: reason}
			pr.Withheld = append(pr.Withheld, f)
			rs.Refused = append(rs.Refused, f)
		}
		reading, err := techpack.RunFixtures(p.Dir)
		if err != nil {
			withhold("", "pack unreadable: "+oneLineErr(err))
			rs.Packs = append(rs.Packs, pr)
			continue
		}
		var sec *techpack.SectionReading
		for i := range reading.Sections {
			if reading.Sections[i].Section == techpack.SectionReadouts {
				sec = &reading.Sections[i]
			}
		}
		if sec != nil && sec.NoFixtures {
			for _, m := range p.Readouts {
				id, _ := m["id"].(string)
				withhold(id, "no fixtures: the pack's readouts section is withheld")
			}
			rs.Packs = append(rs.Packs, pr)
			continue
		}
		var rules []Rule
		for _, m := range p.Readouts {
			id, _ := m["id"].(string)
			if tests, _ := m["test"].([]any); len(tests) == 0 {
				withhold(id, "no fixtures: a pack rule is served only with a passing fixture")
				continue
			}
			data, err := toml.Marshal(map[string]any{"schema": 1, "rule": []any{tomlSafe(m)}})
			if err != nil {
				withhold(id, oneLineErr(err))
				continue
			}
			one, err := parseRuleFile(data)
			if err != nil || len(one) != 1 {
				if err == nil {
					err = fmt.Errorf("expected one rule")
				}
				withhold(id, oneLineErr(err))
				continue
			}
			rules = append(rules, one[0])
		}
		before := len(rs.Refused)
		added := admitAddOnly(rs, rules, src, os.DirFS(p.Dir), false)
		for _, f := range rs.Refused[before:] {
			pr.Withheld = append(pr.Withheld, f)
		}
		for _, r := range added {
			pr.Served = append(pr.Served, r.ID)
		}
		rs.Rules = append(added, rs.Rules...)
		rs.Packs = append(rs.Packs, pr)
	}
}

// tomlSafe turns the whole-number floats a decoded pack carries back into
// ints, so the rule re-encodes as the integer the rules schema asks for.
func tomlSafe(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = tomlSafe(e)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = tomlSafe(e)
		}
		return out
	case float64:
		if x == float64(int64(x)) {
			return int64(x)
		}
	}
	return v
}

func oneLineErr(err error) string {
	s := err.Error()
	for i, c := range s {
		if c == '\n' {
			return s[:i]
		}
	}
	return s
}
