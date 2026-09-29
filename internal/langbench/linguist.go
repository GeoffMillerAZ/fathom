package langbench

// linguist.go — github-linguist's classification data as a source of match
// drafts (design docs/design/language-breadth.md §5, §13.2 step 9).
//
// github-linguist/linguist (MIT, Copyright (c) 2017 GitHub, Inc.) knows,
// for most languages a worker will be asked to register, the extensions,
// filenames and interpreters that identify it and the content rules that
// settle an extension several languages share. This file imports that data
// for the queue's items, as plain data at a pinned revision
// (docs/plans/language-breadth.linguist.json), and drafts a dialect
// manifest's match block from it. Nothing here changes how the connector
// classifies a file: the data feeds drafts and the recipe, and a worker
// still reads, edits and proves the block.
//
// The only hand-kept part of the data file is items.<id>.linguist (the
// linguist language names a queue id stands for) and, for a language two
// items share, items.<id>.allow_filenames. Regenerating re-reads them.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Linguist provenance recorded in the data file.
const (
	LinguistSource  = "github-linguist/linguist"
	LinguistLicence = "MIT"
)

// LinguistDataPath is the committed data file under root.
func LinguistDataPath(root string) string {
	return filepath.Join(root, "docs", "plans", "language-breadth.linguist.json")
}

var (
	linguistRevPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
	// matchExtPattern is schema/dialect/dialect.cue #Match's extension shape.
	matchExtPattern = regexp.MustCompile(`^\.[A-Za-z0-9._+-]+$`)
)

// LinguistData is docs/plans/language-breadth.linguist.json.
type LinguistData struct {
	Source   string                  `json:"source"`
	Revision string                  `json:"revision"`
	Licence  string                  `json:"licence"`
	Stats    LinguistStats           `json:"stats"`
	Items    map[string]LinguistItem `json:"items"`
}

// LinguistStats counts the heuristic patterns of the disambiguation blocks
// the import kept, once per block however many items claim it, after
// named_pattern expansion. A pattern that does not compile under Go's RE2
// is dropped and counted; a rule left with no alternative for a condition
// it stated is dropped too, and counted separately.
type LinguistStats struct {
	Patterns     int `json:"patterns"`
	Dropped      int `json:"dropped_patterns"`
	DroppedRules int `json:"dropped_rules"`
}

// LinguistItem is one queue item's linguist data. Linguist and
// AllowFilenames are the hand-kept mapping; everything else is generated.
type LinguistItem struct {
	// Linguist names the linguist language(s) the queue id stands for.
	Linguist []string `json:"linguist"`
	// AllowFilenames, when present (empty included), keeps only those of the
	// languages' filenames: required for a tooling item whose language
	// another queue item shares (Starlark: bazel, tiltfile).
	AllowFilenames *[]string `json:"allow_filenames,omitempty"`

	Extensions   []string `json:"extensions"`
	Filenames    []string `json:"filenames"`
	Interpreters []string `json:"interpreters"`
	// Heuristics are linguist's rules for the extensions the item claims
	// that linguist disambiguates.
	Heuristics []LinguistHeuristic `json:"heuristics,omitempty"`
}

// LinguistHeuristic is one linguist disambiguation block, limited to the
// extensions the item claims; Rules are in linguist's order (first match
// wins).
type LinguistHeuristic struct {
	Extensions []string       `json:"extensions"`
	Rules      []LinguistRule `json:"rules"`
}

// LinguistRule is one rule. Language is the queue id when the linguist name
// is mapped to exactly one (a language item wins over a tooling one), else
// the linguist name. A rule with no condition always matches (linguist's
// fallback). Pattern is a union (any alternative), NegativePattern a union
// that must not match, And a list of conditions that must all hold.
type LinguistRule struct {
	Language []string `json:"language"`
	LinguistCond
	And []LinguistCond `json:"and,omitempty"`
}

// LinguistCond is a rule's (or an and-clause's) positive and negative
// pattern unions, named_pattern already expanded.
type LinguistCond struct {
	Pattern         []string `json:"pattern,omitempty"`
	NegativePattern []string `json:"negative_pattern,omitempty"`
}

// MarshalLinguist is the data file's canonical form: sorted (map keys by
// encoding/json, lists by the importer), 2-space indented, patterns not
// HTML-escaped, one trailing newline.
func MarshalLinguist(d LinguistData) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ParseLinguist decodes the data file strictly.
func ParseLinguist(b []byte) (LinguistData, error) {
	var d LinguistData
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return LinguistData{}, fmt.Errorf("linguist data: %w", err)
	}
	return d, nil
}

// LoadLinguistData reads and decodes the data file at path.
func LoadLinguistData(path string) (LinguistData, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return LinguistData{}, err
	}
	d, err := ParseLinguist(b)
	if err != nil {
		return LinguistData{}, fmt.Errorf("%s: %w", path, err)
	}
	return d, nil
}

// LoadLinguistMapping reads the hand-kept part of a (possibly stale or
// hand-seeded) data file: items.<id>.linguist and items.<id>.allow_filenames.
// Every other field is ignored, so a seed holding only the mapping loads.
func LoadLinguistMapping(path string) (map[string]LinguistItem, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f struct {
		Items map[string]struct {
			Linguist       []string  `json:"linguist"`
			AllowFilenames *[]string `json:"allow_filenames"`
		} `json:"items"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	out := make(map[string]LinguistItem, len(f.Items))
	for id, it := range f.Items {
		out[id] = LinguistItem{Linguist: it.Linguist, AllowFilenames: it.AllowFilenames}
	}
	return out, nil
}

// --- reading linguist's yml ---

type linguistLang struct {
	Extensions   []string `yaml:"extensions"`
	Filenames    []string `yaml:"filenames"`
	Interpreters []string `yaml:"interpreters"`
}

// ymlList is a yml value that is a string or a list of strings (linguist's
// pattern, negative_pattern, language and named-pattern values).
type ymlList []string

func (l *ymlList) UnmarshalYAML(n *yaml.Node) error {
	switch n.Kind {
	case yaml.ScalarNode:
		*l = ymlList{n.Value}
	case yaml.SequenceNode:
		out := make(ymlList, 0, len(n.Content))
		for _, c := range n.Content {
			if c.Kind != yaml.ScalarNode {
				return fmt.Errorf("line %d: a list of strings was expected", c.Line)
			}
			out = append(out, c.Value)
		}
		*l = out
	default:
		return fmt.Errorf("line %d: a string or a list of strings was expected", n.Line)
	}
	return nil
}

type ymlRule struct {
	Language        ymlList   `yaml:"language"`
	Pattern         ymlList   `yaml:"pattern"`
	NegativePattern ymlList   `yaml:"negative_pattern"`
	NamedPattern    string    `yaml:"named_pattern"`
	And             []ymlRule `yaml:"and"`
}

type ymlBlock struct {
	Extensions []string  `yaml:"extensions"`
	Rules      []ymlRule `yaml:"rules"`
}

type ymlHeuristics struct {
	Disambiguations []ymlBlock         `yaml:"disambiguations"`
	NamedPatterns   map[string]ymlList `yaml:"named_patterns"`
}

// strictYAML decodes b into v, refusing a key v does not name: a rule field
// this importer does not understand must fail, not be silently lost.
func strictYAML(b []byte, v any) error {
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(v); err != nil && err != io.EOF {
		return err
	}
	return nil
}

// ReadLinguistCheckout reads lib/linguist/languages.yml and
// lib/linguist/heuristics.yml from a linguist checkout.
func ReadLinguistCheckout(dir string) (languages, heuristics []byte, err error) {
	languages, err = os.ReadFile(filepath.Join(dir, "lib", "linguist", "languages.yml"))
	if err != nil {
		return nil, nil, err
	}
	heuristics, err = os.ReadFile(filepath.Join(dir, "lib", "linguist", "heuristics.yml"))
	if err != nil {
		return nil, nil, err
	}
	return languages, heuristics, nil
}

// heuristicPatternFlags is how connector.ClassifyDialect compiles a sniff
// (internal/connector/classify_dialect.go sniffRegexp): multi-line, since
// linguist's Ruby patterns anchor lines with ^ and $.
const heuristicPatternFlags = "(?m)"

// compilesAsSniff reports whether p compiles exactly as ClassifyDialect
// compiles a sniff.
func compilesAsSniff(p string) bool {
	_, err := regexp.Compile(heuristicPatternFlags + p)
	return err == nil
}

// --- the import ---

// ImportLinguist is LB-F48: for each item of mapping, the extensions,
// filenames and interpreters of the linguist languages it names, from
// linguist's languages.yml, plus (LB-F49) linguist's heuristics for the
// extensions the item claims, from heuristics.yml, at revision rev.
//
// A tooling queue item takes linguist filenames only, never a language's
// extensions or interpreters. A linguist language two mapped items share
// (Starlark: bazel, tiltfile) is refused unless every tooling item naming
// it carries an explicit allow_filenames, of which only those are kept.
// Every list is sorted and the result is a pure function of its inputs.
func ImportLinguist(languagesYML, heuristicsYML []byte, rev string, q []QueueEntry, mapping map[string]LinguistItem) (LinguistData, error) {
	if !linguistRevPattern.MatchString(rev) {
		return LinguistData{}, fmt.Errorf("linguist revision %q must be a 40-hex commit", rev)
	}
	var langs map[string]linguistLang
	if err := yaml.Unmarshal(languagesYML, &langs); err != nil {
		return LinguistData{}, fmt.Errorf("languages.yml: %w", err)
	}
	var heur ymlHeuristics
	if err := strictYAML(heuristicsYML, &heur); err != nil {
		return LinguistData{}, fmt.Errorf("heuristics.yml: %w", err)
	}

	kind := map[string]string{}
	for _, e := range q {
		kind[e.ID] = e.Kind
	}
	ids := make([]string, 0, len(mapping))
	for id := range mapping {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	// Validate the mapping, and find the languages more than one item names.
	named := map[string][]string{} // linguist name -> item ids
	for _, id := range ids {
		m := mapping[id]
		if _, ok := kind[id]; !ok {
			return LinguistData{}, fmt.Errorf("mapping names %q, which is not a queue id", id)
		}
		if len(m.Linguist) == 0 {
			return LinguistData{}, fmt.Errorf("item %s names no linguist language (items.%s.linguist)", id, id)
		}
		for _, n := range m.Linguist {
			if _, ok := langs[n]; !ok {
				return LinguistData{}, fmt.Errorf("item %s names linguist language %q, which languages.yml does not define", id, n)
			}
			named[n] = append(named[n], id)
		}
	}
	for n, owners := range named {
		if len(owners) < 2 {
			continue
		}
		for _, id := range owners {
			if kind[id] == "tooling" && mapping[id].AllowFilenames == nil {
				return LinguistData{}, fmt.Errorf("linguist language %q is shared by %s: tooling item %s needs an explicit allow_filenames (empty keeps none)", n, strings.Join(owners, ", "), id)
			}
		}
	}

	// A rule's language becomes a queue id when exactly one item stands for
	// it, a language item winning over tooling ones.
	nameID := map[string]string{}
	for n, owners := range named {
		pick := owners
		if len(pick) > 1 {
			var langOnly []string
			for _, id := range owners {
				if kind[id] == "language" {
					langOnly = append(langOnly, id)
				}
			}
			pick = langOnly
		}
		if len(pick) == 1 {
			nameID[n] = pick[0]
		}
	}

	out := LinguistData{Source: LinguistSource, Revision: rev, Licence: LinguistLicence, Items: map[string]LinguistItem{}}
	blocks := &heuristicBlocks{heur: heur, nameID: nameID, done: map[int]*compiledBlock{}}

	for _, id := range ids {
		m := mapping[id]
		it := LinguistItem{Linguist: slices.Clone(m.Linguist), AllowFilenames: cloneAllow(m.AllowFilenames)}
		var exts, files, interps []string
		for _, n := range m.Linguist {
			l := langs[n]
			exts = append(exts, l.Extensions...)
			files = append(files, l.Filenames...)
			interps = append(interps, l.Interpreters...)
		}
		if kind[id] == "tooling" {
			exts, interps = nil, nil
		}
		if it.AllowFilenames != nil {
			have := map[string]bool{}
			for _, f := range files {
				have[f] = true
			}
			for _, f := range *it.AllowFilenames {
				if !have[f] {
					return LinguistData{}, fmt.Errorf("item %s allows filename %q, which linguist's %s do not list", id, f, strings.Join(m.Linguist, ", "))
				}
			}
			files = slices.DeleteFunc(files, func(f string) bool { return !slices.Contains(*it.AllowFilenames, f) })
		}
		it.Extensions, it.Filenames, it.Interpreters = sortedSet(exts), sortedSet(files), sortedSet(interps)
		if len(it.Extensions) > 0 {
			hs, err := blocks.forExtensions(it.Extensions)
			if err != nil {
				return LinguistData{}, fmt.Errorf("item %s: %w", id, err)
			}
			it.Heuristics = hs
		}
		out.Items[id] = it
	}
	out.Stats = blocks.stats
	return out, nil
}

func cloneAllow(p *[]string) *[]string {
	if p == nil {
		return nil
	}
	c := sortedSet(*p)
	return &c
}

// sortedSet is s deduplicated and sorted, never nil (the data prints [] for
// none).
func sortedSet(s []string) []string {
	out := slices.Clone(s)
	sort.Strings(out)
	out = slices.Compact(out)
	if out == nil {
		out = []string{}
	}
	return out
}

// heuristicBlocks compiles each disambiguation block once, on first use, and
// accumulates the stats over the blocks used.
type heuristicBlocks struct {
	heur   ymlHeuristics
	nameID map[string]string
	done   map[int]*compiledBlock
	stats  LinguistStats
}

type compiledBlock struct {
	rules []LinguistRule
}

// forExtensions returns the blocks whose extensions include any of exts
// (compared case-insensitively, as linguist does), limited to those, sorted.
func (b *heuristicBlocks) forExtensions(exts []string) ([]LinguistHeuristic, error) {
	var out []LinguistHeuristic
	for i, blk := range b.heur.Disambiguations {
		var hit []string
		for _, x := range exts {
			if slices.ContainsFunc(blk.Extensions, func(y string) bool { return strings.EqualFold(x, y) }) {
				hit = append(hit, x)
			}
		}
		if len(hit) == 0 {
			continue
		}
		cb, ok := b.done[i]
		if !ok {
			var err error
			if cb, err = b.compile(blk); err != nil {
				return nil, fmt.Errorf("heuristics for %s: %w", strings.Join(blk.Extensions, " "), err)
			}
			b.done[i] = cb
		}
		out = append(out, LinguistHeuristic{Extensions: sortedSet(hit), Rules: cb.rules})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.Join(out[i].Extensions, " ") < strings.Join(out[j].Extensions, " ")
	})
	return out, nil
}

func (b *heuristicBlocks) compile(blk ymlBlock) (*compiledBlock, error) {
	cb := &compiledBlock{}
	for _, r := range blk.Rules {
		rule, ok, err := b.rule(r)
		if err != nil {
			return nil, err
		}
		if !ok {
			b.stats.DroppedRules++
			continue
		}
		cb.rules = append(cb.rules, rule)
	}
	return cb, nil
}

// rule converts one yml rule. ok is false when a condition it stated has no
// alternative left after the patterns RE2 cannot compile were dropped.
func (b *heuristicBlocks) rule(r ymlRule) (LinguistRule, bool, error) {
	if len(r.Language) == 0 {
		return LinguistRule{}, false, fmt.Errorf("a rule names no language")
	}
	rule := LinguistRule{}
	for _, n := range r.Language {
		if id, ok := b.nameID[n]; ok {
			n = id
		}
		rule.Language = append(rule.Language, n)
	}
	cond, alive, err := b.cond(r.Pattern, r.NegativePattern, r.NamedPattern)
	if err != nil {
		return LinguistRule{}, false, err
	}
	rule.LinguistCond = cond
	for _, a := range r.And {
		if len(a.Language) > 0 || len(a.And) > 0 {
			return LinguistRule{}, false, fmt.Errorf("an and-clause names a language or nests another and (unsupported shape)")
		}
		c, ok, err := b.cond(a.Pattern, a.NegativePattern, a.NamedPattern)
		if err != nil {
			return LinguistRule{}, false, err
		}
		alive = alive && ok
		rule.And = append(rule.And, c)
	}
	if !alive {
		return LinguistRule{}, false, nil
	}
	return rule, true, nil
}

// cond expands and compiles one condition. A positive or negative union
// whose every alternative failed to compile makes the condition dead.
func (b *heuristicBlocks) cond(pattern, negative ymlList, named string) (LinguistCond, bool, error) {
	pos := slices.Clone(pattern)
	if named != "" {
		if len(pos) > 0 {
			return LinguistCond{}, false, fmt.Errorf("a condition has both pattern and named_pattern %q (unsupported shape)", named)
		}
		np, ok := b.heur.NamedPatterns[named]
		if !ok {
			return LinguistCond{}, false, fmt.Errorf("named_pattern %q is not defined", named)
		}
		pos = slices.Clone(np)
	}
	var c LinguistCond
	alive := true
	keep := func(in []string) []string {
		var kept []string
		for _, p := range in {
			b.stats.Patterns++
			if compilesAsSniff(p) {
				kept = append(kept, p)
			} else {
				b.stats.Dropped++
			}
		}
		if len(in) > 0 && len(kept) == 0 {
			alive = false
		}
		return kept
	}
	c.Pattern = keep(pos)
	c.NegativePattern = keep(negative)
	return c, alive, nil
}

// --- the draft ---

// DraftMatch is a dialect manifest's match block, langbench.ManifestMatch's
// shape plus the shebangs ManifestMatch does not decode and #Match
// (schema/dialect/dialect.cue) carries. Sniff is null when there is none.
type DraftMatch struct {
	Extensions []string       `json:"extensions"`
	Basenames  []string       `json:"basenames"`
	Globs      []string       `json:"globs"`
	Shebangs   []string       `json:"shebangs"`
	Sniff      *ManifestSniff `json:"sniff"`
}

// ExtClaim is one claimant of an extension: a queue id (or a registered
// dialect's id) and where the claim is written.
type ExtClaim struct {
	ID     string
	Source string
}

// ExtensionClaims maps a lower-case extension to everything that claims it.
type ExtensionClaims map[string][]ExtClaim

// The sources of an extension claim.
const (
	ClaimLinguistData      = "linguist data"
	ClaimQueueName         = "queue name"
	ClaimRegisteredDialect = "registered dialect"
)

const (
	// sniffPriorityStep spaces sniff priorities by the position of the
	// item's first rule in linguist's block, leaving room to interleave.
	sniffPriorityStep = 10
	// multiDotGlobPrefix turns a multi-dot suffix into a manifest glob.
	multiDotGlobPrefix = "**/*"
)

// LinguistExtensionClaims gathers who claims each extension: the queue
// items' extensions in the linguist data, the extensions their queue names
// write (every clause, sniffed or not), and the registered dialects' match
// extensions and sniff extensions.
func LinguistExtensionClaims(q []QueueEntry, d LinguistData, registered map[string]ManifestMatch) ExtensionClaims {
	claims := ExtensionClaims{}
	seen := map[string]bool{}
	add := func(ext, id, src string) {
		ext = strings.ToLower(ext)
		key := ext + "\x00" + id + "\x00" + src
		if seen[key] {
			return
		}
		seen[key] = true
		claims[ext] = append(claims[ext], ExtClaim{ID: id, Source: src})
	}
	for id, it := range d.Items {
		for _, x := range it.Extensions {
			add(x, id, ClaimLinguistData)
		}
	}
	for _, e := range q {
		_, all := nameExtensions(e.Name)
		for _, x := range all {
			add(x, e.ID, ClaimQueueName)
		}
	}
	for id, m := range registered {
		for _, x := range m.Extensions {
			add(x, id, ClaimRegisteredDialect)
		}
		if m.Sniff != nil {
			for _, x := range m.Sniff.Extensions {
				add(x, id, ClaimRegisteredDialect)
			}
		}
	}
	for x := range claims {
		sort.Slice(claims[x], func(i, j int) bool {
			a, b := claims[x][i], claims[x][j]
			if a.ID != b.ID {
				return a.ID < b.ID
			}
			return a.Source < b.Source
		})
	}
	return claims
}

// RegisteredMatches reads the match block of every registered dialect under
// root (the embedded incumbents and internal/extractor/dialects/*), by id.
func RegisteredMatches(root string) (map[string]ManifestMatch, error) {
	out := map[string]ManifestMatch{}
	specs, err := Incumbents()
	if err != nil {
		return nil, err
	}
	for _, s := range specs {
		out[s.Manifest.ID] = s.Manifest.Match
	}
	paths, _ := filepath.Glob(filepath.Join(root, "internal", "extractor", "dialects", "*", "dialect.json"))
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		m, err := ParseManifest(b)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		out[m.ID] = m.Match
	}
	return out, nil
}

// DraftMatchFromLinguist is LB-F49: the draft match block for queue id from
// its imported linguist data (§13.2 step 9): extensions, basenames from the
// filenames, shebangs from the interpreters, and a sniff built from linguist's
// rules for an extension another queue item or a registered dialect claims.
// Such an extension moves into the sniff (the shape matlab's manifest has);
// the sniff's priority is ten times the position, in linguist's block, of the
// item's first rule, and its pattern the union of the item's rules' positive
// patterns. The result satisfies dialect.cue #Match. The warnings say what a
// worker must decide: each claimed extension, each rule that no single RE2
// pattern can express, each extension left out.
func DraftMatchFromLinguist(d LinguistData, id string, claims ExtensionClaims) (DraftMatch, []string, error) {
	it, ok := d.Items[id]
	if !ok {
		return DraftMatch{}, nil, fmt.Errorf("no linguist data for %q (add items.%s.linguist to the data file and regenerate)", id, id)
	}
	var warns []string
	warnf := func(format string, a ...any) { warns = append(warns, fmt.Sprintf(format, a...)) }

	m := DraftMatch{Extensions: []string{}, Basenames: []string{}, Globs: []string{}, Shebangs: []string{}}
	m.Basenames = nonEmptyStrings(it.Filenames)
	m.Shebangs = nonEmptyStrings(it.Interpreters)

	// The item's extensions, lower-case (ClassifyDialect compares folded),
	// each as a plain claim, a glob (a multi-dot suffix filepath.Ext never
	// sees) or left out (#Match refuses it).
	shared := map[string]bool{}
	for _, x := range sortedSet(lowerAll(it.Extensions)) {
		switch {
		case !matchExtPattern.MatchString(x):
			warnf("extension %q is left out: dialect.cue #Match accepts %s", x, matchExtPattern)
			continue
		case strings.Count(x, ".") > 1:
			g := multiDotGlobPrefix + x
			m.Globs = append(m.Globs, g)
			warnf("extension %s has two dots, which filepath.Ext never sees: drafted as glob %s", x, g)
			continue
		}
		m.Extensions = append(m.Extensions, x)
		if others := claimedByOthers(claims, x, id); len(others) > 0 {
			shared[x] = true
			warnf("extension %s is also claimed by %s", x, strings.Join(others, "; "))
		}
	}

	sn := draftSniff(it, id, shared, &warns)
	if sn != nil {
		inSniff := map[string]bool{}
		for _, x := range sn.Extensions {
			inSniff[x] = true
		}
		m.Extensions = slices.DeleteFunc(m.Extensions, func(x string) bool { return inSniff[x] })
		m.Sniff = sn
	}
	sort.Strings(warns)
	return m, warns, nil
}

func nonEmptyStrings(s []string) []string {
	out := []string{}
	for _, x := range s {
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}

func lowerAll(s []string) []string {
	out := make([]string, len(s))
	for i, x := range s {
		out[i] = strings.ToLower(x)
	}
	return out
}

// claimedByOthers renders the claimants of ext other than id: "matlab
// (linguist data, queue name)", sorted.
func claimedByOthers(claims ExtensionClaims, ext, id string) []string {
	var order []string
	srcs := map[string][]string{}
	for _, c := range claims[strings.ToLower(ext)] {
		if c.ID == id {
			continue
		}
		if _, ok := srcs[c.ID]; !ok {
			order = append(order, c.ID)
		}
		srcs[c.ID] = append(srcs[c.ID], c.Source)
	}
	out := make([]string, 0, len(order))
	for _, o := range order {
		out = append(out, o+" ("+strings.Join(srcs[o], ", ")+")")
	}
	return out
}

// draftSniff builds the one sniff a manifest can carry from the item's
// heuristic blocks for the extensions in shared. It returns nil when no
// shared extension has a rule of the item's that yields a pattern.
func draftSniff(it LinguistItem, id string, shared map[string]bool, warns *[]string) *ManifestSniff {
	warnf := func(format string, a ...any) { *warns = append(*warns, fmt.Sprintf(format, a...)) }
	var (
		exts     []string
		patterns []string
		priority = -1
		covered  = map[string]bool{}
	)
	for _, h := range it.Heuristics {
		var blockExts []string
		for _, x := range h.Extensions {
			if shared[strings.ToLower(x)] {
				blockExts = append(blockExts, strings.ToLower(x))
			}
		}
		if len(blockExts) == 0 {
			continue
		}
		var blockPats []string
		blockPrio := -1
		for i, r := range h.Rules {
			if !ruleIsOf(r, id, it) {
				continue
			}
			pos, why := rulePositives(r)
			for _, w := range why {
				warnf("%s: %s", strings.Join(blockExts, " "), w)
			}
			if len(pos) == 0 {
				if len(why) == 0 {
					warnf("%s: linguist's rule for %s has no pattern (a fallback: %s is what remains when no other rule matches), so no sniff can be drafted from it", strings.Join(blockExts, " "), id, id)
				}
				continue
			}
			blockPats = append(blockPats, pos...)
			if blockPrio < 0 {
				blockPrio = i * sniffPriorityStep
			}
		}
		if len(blockPats) == 0 {
			continue
		}
		for _, x := range blockExts {
			covered[x] = true
		}
		exts = append(exts, blockExts...)
		patterns = append(patterns, blockPats...)
		if priority < 0 || blockPrio < priority {
			priority = blockPrio
		}
	}
	for x := range shared {
		if !covered[x] {
			warnf("extension %s is shared but linguist gives %s no pattern for it: kept as a plain claim, which leaves the file ambiguous until a sniff settles it", x, id)
		}
	}
	if len(patterns) == 0 {
		return nil
	}
	patterns = uniqueInOrder(patterns)
	pattern := patterns[0]
	if len(patterns) > 1 {
		parts := make([]string, len(patterns))
		for i, p := range patterns {
			parts[i] = "(?:" + p + ")"
		}
		pattern = strings.Join(parts, "|")
	}
	if !compilesAsSniff(pattern) {
		warnf("the union of the sniff patterns does not compile under RE2: no sniff drafted")
		return nil
	}
	return &ManifestSniff{Extensions: sortedSet(exts), Pattern: pattern, Priority: priority}
}

// ruleIsOf reports whether a linguist rule returns the item's language: its
// queue id, or one of its linguist names.
func ruleIsOf(r LinguistRule, id string, it LinguistItem) bool {
	for _, l := range r.Language {
		if l == id || slices.Contains(it.Linguist, l) {
			return true
		}
	}
	return false
}

// rulePositives returns the positive alternatives a single RE2 pattern can
// stand for, and what had to be given up. A rule with one positive union
// (top-level, or in one and-clause) yields it, its negative half dropped
// with a note (RE2 has no lookahead); a rule of several positive clauses is
// not expressible and yields nothing.
func rulePositives(r LinguistRule) (pos []string, notes []string) {
	positives := [][]string{}
	var negative bool
	if len(r.Pattern) > 0 {
		positives = append(positives, r.Pattern)
	}
	negative = negative || len(r.NegativePattern) > 0
	for _, c := range r.And {
		if len(c.Pattern) > 0 {
			positives = append(positives, c.Pattern)
		}
		negative = negative || len(c.NegativePattern) > 0
	}
	switch {
	case len(positives) == 0 && negative:
		return nil, []string{"linguist's rule is only a negative_pattern: nothing to sniff for"}
	case len(positives) > 1:
		return nil, []string{"linguist's rule needs several patterns to all match (an and-rule): not one RE2 pattern, write the sniff by hand"}
	case len(positives) == 0:
		return nil, nil
	}
	if negative {
		notes = append(notes, "negative_pattern dropped from the rule (RE2 has no lookahead): the sniff is broader than linguist's")
	}
	return positives[0], notes
}

func uniqueInOrder(s []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range s {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
