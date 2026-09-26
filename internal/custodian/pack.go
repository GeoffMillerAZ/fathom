package custodian

// pack.go — CC-12 (docs/plans/custodian.md, vision 22 §7): deterministic-
// validation packs. A pack is a curated, versioned set of concerns shipped
// as DATA — one CUE file per pack under packs/, embedded in the binary,
// parsed by the same loader as a concerns.cue — never as Go: knowledge
// about what a Claude Code, Codex, lefthook or Taskfile setup must keep
// true is content, and hardcoding it would make fathom's releases the
// bottleneck for editing it.
//
// Enabling and overriding are law 13's shape, a third time:
//
//	packs: ["claude-code", "lefthook"]   // in either scope; the union is enabled
//	lefthook_runs_gate: {off: true}      // disable one entry (named, never hidden)
//	lefthook_runs_gate: {want: …, check: {…}}  // replace one entry wholesale
//
// Precedence is pack < user < project: a same-ID entry in either file
// replaces the pack's wholesale — no field-level blending, which would
// manufacture a concern nobody wrote. The admission rule is vision 22 §7's:
// every pack entry names, in `why`, the real failure it catches
// (TestPacksAreCuratedData refuses one that does not).

import (
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"

	"cuelang.org/go/cue"
)

//go:embed packs/*.cue
var packFS embed.FS

// Pack is one shipped pack: its identity and its concerns (Scope =
// ScopePack, Pack = Name).
type Pack struct {
	Name     string
	Version  int
	About    string
	Concerns []Concern
}

// packMeta is a pack file's `pack` block.
type packMeta struct {
	name    string
	version int
	about   string
}

// KnownPacks lists the shipped pack names, sorted.
func KnownPacks() []string {
	entries, err := packFS.ReadDir("packs")
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if n, ok := strings.CutSuffix(e.Name(), ".cue"); ok && !e.IsDir() {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

func isKnownPack(name string) bool {
	for _, p := range KnownPacks() {
		if p == name {
			return true
		}
	}
	return false
}

// LoadPack parses one shipped pack. A pack that does not load is a
// shipped-data defect (the tests hold every pack to loading), reported as
// a *LoadError naming the pack file, never skipped.
func LoadPack(name string) (Pack, error) {
	if !isKnownPack(name) {
		return Pack{}, fmt.Errorf("unknown pack %q (known packs: %s)", name, strings.Join(KnownPacks(), ", "))
	}
	file := path.Join("packs", name+".cue")
	data, err := packFS.ReadFile(file)
	if err != nil {
		return Pack{}, fmt.Errorf("pack %s: %w", name, err)
	}
	doc, errs := parseDocument(data, file, ScopePack, true)
	if len(errs) == 0 {
		switch {
		case !doc.sawMeta:
			errs = append(errs, fmt.Errorf("pack file has no `pack` block (name, version, about)"))
		case doc.meta.name != name:
			errs = append(errs, fmt.Errorf("pack block names %q but the file is %s.cue", doc.meta.name, name))
		}
		for _, c := range doc.concerns {
			if c.Off {
				errs = append(errs, fmt.Errorf("concern %q: a pack entry cannot ship off — a disabled default is not a curated want", c.ID))
			}
		}
	}
	if len(errs) > 0 {
		return Pack{}, &LoadError{Path: "pack:" + name, Scope: ScopePack, Errs: errs}
	}
	for i := range doc.concerns {
		doc.concerns[i].Pack = name
	}
	return Pack{Name: name, Version: doc.meta.version, About: doc.meta.about, Concerns: doc.concerns}, nil
}

// PackConcern returns one entry of a shipped pack by ID — how a surface
// that renders a pack's knowledge (doctor, CC-13) asks for it instead of
// re-deriving it.
func PackConcern(pack, id string) (Concern, error) {
	p, err := LoadPack(pack)
	if err != nil {
		return Concern{}, err
	}
	for _, c := range p.Concerns {
		if c.ID == id {
			return c, nil
		}
	}
	return Concern{}, fmt.Errorf("pack %s ships no concern %q", pack, id)
}

func parsePackMeta(v cue.Value) (packMeta, []error) {
	var m packMeta
	var errs []error
	fields, err := v.Fields(cue.Optional(true))
	if err != nil {
		return m, []error{fmt.Errorf("pack: %w", err)}
	}
	for fields.Next() {
		name := fields.Selector().Unquoted()
		if fields.IsOptional() {
			errs = append(errs, fmt.Errorf("pack.%s is declared OPTIONAL — write it without the question mark", name))
			continue
		}
		switch name {
		case "name":
			s, err := fields.Value().String()
			if err != nil {
				errs = append(errs, fmt.Errorf("pack.name must be a string: %w", err))
			}
			m.name = s
		case "version":
			n, err := fields.Value().Int64()
			if err != nil || n <= 0 {
				errs = append(errs, fmt.Errorf("pack.version must be a positive integer"))
			}
			m.version = int(n)
		case "about":
			s, err := fields.Value().String()
			if err != nil {
				errs = append(errs, fmt.Errorf("pack.about must be a string: %w", err))
			}
			m.about = s
		default:
			errs = append(errs, fmt.Errorf("pack has unknown field %q (known: name, version, about)", name))
		}
	}
	if m.name == "" || m.version == 0 || m.about == "" {
		errs = append(errs, fmt.Errorf("pack block needs name, version and about"))
	}
	return m, errs
}

// Resolve is the full law-13 resolution (CC-12): the packs either scope
// enables (their union) contribute their entries at the lowest precedence;
// the user file's entries replace same-ID pack entries; the project file's
// replace both. Output sorted by ID.
//
// Two enabled packs may ship the same entry (the claude-code and codex
// packs share the agent-config-wide ones); identical definitions collapse
// to one concern naming both packs, and differing ones are a shipped-data
// defect reported as an error rather than an arbitrary winner.
func Resolve(user, project ScopeFile) ([]Concern, error) {
	names := map[string]bool{}
	for _, n := range user.Packs {
		names[n] = true
	}
	for _, n := range project.Packs {
		names[n] = true
	}
	enabled := make([]string, 0, len(names))
	for n := range names {
		enabled = append(enabled, n)
	}
	sort.Strings(enabled)

	byID := map[string]Concern{}
	for _, name := range enabled {
		p, err := LoadPack(name)
		if err != nil {
			return nil, err
		}
		for _, c := range p.Concerns {
			prev, dup := byID[c.ID]
			if !dup {
				byID[c.ID] = c
				continue
			}
			if !sameWant(prev, c) {
				return nil, fmt.Errorf("packs %s and %s both ship concern %q with different definitions", prev.Pack, name, c.ID)
			}
			prev.Pack += "+" + name
			byID[c.ID] = prev
		}
	}
	packs := make([]Concern, 0, len(byID))
	for _, c := range byID {
		packs = append(packs, c)
	}
	return Merge(Merge(packs, user.Concerns), project.Concerns), nil
}

// sameWant compares two concern definitions, ignoring provenance.
func sameWant(a, b Concern) bool {
	return a.ID == b.ID && a.Want == b.Want && a.Why == b.Why && a.Off == b.Off && a.Check == b.Check
}
