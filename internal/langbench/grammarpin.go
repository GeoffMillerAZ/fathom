package langbench

// grammarpin.go — pinning a queue item's grammar (design §8.2, §13.1 steps
// 5-6): the catalogue pre-fill (LB-F50) and the licence verdict (LB-F51).
//
// The catalogue is tree-sitter-language-pack (MIT): sources/
// language_definitions.json names a repo, a commit and a build recipe for
// 371 grammars, sources/license_cache.json the SPDX id of 349 repos. Reading it
// turns "find the repo, resolve the tag, guess the ABI" into a draft lock row
// a worker only confirms. The catalogue is a lead, not a proof: the verdict
// at pin (GrammarLicenceVerdict) reads the licence off the tree at the pinned
// commit, and that verdict wins.

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// Where the pre-fill lives, and whose data it is.
const (
	GrammarPrefillPath = "docs/plans/language-breadth.grammars.json"
	CatalogueSource    = "kreuzberg-dev/tree-sitter-language-pack"
	CatalogueLicence   = "MIT"
)

// Licence bases: what a licence was read from.
const (
	BasisCatalogue = "catalogue" // the catalogue's license_cache.json or inline license; a lead
	BasisFile      = "LICENSE"   // a top-level LICENSE*, LICENCE* or COPYING* file, identified by its text
	BasisDeclared  = "declared"  // package.json, Cargo.toml and tree-sitter.json agree; no LICENSE file
)

// catalogueDefaultABI is the pack's own default generation target
// (scripts/clone_vendors.py: abi_version defaults to 14). For a grammar that
// ships a committed parser.c the true ABI is that file's LANGUAGE_VERSION,
// which the build step reads; the pre-fill carries the pack's number.
const catalogueDefaultABI = 14

var hex40 = regexp.MustCompile(`^[0-9a-f]{40}$`)

// ---------------------------------------------------------------------------
// The catalogue

// CatalogueGrammar is one entry of sources/language_definitions.json (fields
// the pre-fill does not use are ignored).
type CatalogueGrammar struct {
	Repo       string `json:"repo"`
	Rev        string `json:"rev"`
	Branch     string `json:"branch"`
	Directory  string `json:"directory"`
	ABIVersion int    `json:"abi_version"`
	Generate   bool   `json:"generate"`
	CSymbol    string `json:"c_symbol"`
	License    string `json:"license"`
	Local      string `json:"local"`
}

// Catalogue is sources/ of a tree-sitter-language-pack checkout.
type Catalogue struct {
	Grammars map[string]CatalogueGrammar
	Licences map[string]string // "owner/repo" to SPDX
}

// LoadCatalogue reads language_definitions.json and license_cache.json under
// dir/sources.
func LoadCatalogue(dir string) (Catalogue, error) {
	var c Catalogue
	for _, f := range []struct {
		name string
		into any
	}{{"language_definitions.json", &c.Grammars}, {"license_cache.json", &c.Licences}} {
		b, err := os.ReadFile(filepath.Join(dir, "sources", f.name))
		if err != nil {
			return Catalogue{}, err
		}
		if err := json.Unmarshal(b, f.into); err != nil {
			return Catalogue{}, fmt.Errorf("%s: %w", f.name, err)
		}
	}
	return c, nil
}

// catalogueRepo is a catalogue repo URL as the lock spells a repo: owner/repo
// for GitHub, host/owner/repo for anything else, no scheme, no ".git".
func catalogueRepo(url string) string {
	r := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(url), "/"), ".git")
	r = strings.TrimPrefix(strings.TrimPrefix(r, "https://"), "http://")
	return strings.TrimPrefix(r, "github.com/")
}

// licenceOf is the catalogue's licence for g at repo: the entry's own
// license field, else the cache (case-insensitive on the repo), else "".
func (c Catalogue) licenceOf(g CatalogueGrammar, repo string) string {
	if g.License != "" {
		return g.License
	}
	if l, ok := c.Licences[repo]; ok {
		return l
	}
	for r, l := range c.Licences {
		if strings.EqualFold(r, repo) {
			return l
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// The pre-fill data file

// PrefillEntry is one queue item's pre-filled pin. Catalogue is the pack's
// name for the grammar ("" when the pack has no upstream for it); Licence is
// the SPDX id the licence_basis vouches for; Flags say what would stop the
// row (grammar-differs is information, the rest block).
type PrefillEntry struct {
	Catalogue    string   `json:"catalogue"`
	Repo         string   `json:"repo"`
	Commit       string   `json:"commit"`
	ABI          int      `json:"abi"`
	Generate     bool     `json:"generate"`
	Directory    string   `json:"directory,omitempty"`
	CSymbol      string   `json:"c_symbol,omitempty"`
	Licence      string   `json:"licence"`
	LicenceBasis string   `json:"licence_basis"`
	Flags        []string `json:"flags"`
}

// PinVerdict is the licence verdict recorded for a queue item's own grammar
// (queue grammar_source), checked at Commit. It is kept by hand: the
// catalogue may name a different repo for the item (prolog) or none (abap),
// and the verdict at pin is what the owner decision rests on.
type PinVerdict struct {
	Repo   string `json:"repo"`
	Commit string `json:"commit"`
	LicenceVerdict
}

// GrammarPrefill is docs/plans/language-breadth.grammars.json. Overrides
// (queue id to catalogue name) and Verdicts are hand-kept and re-read when
// the file is regenerated; Grammars is generated, keyed by queue id.
type GrammarPrefill struct {
	Source    string                  `json:"source"`
	Revision  string                  `json:"revision"`
	Licence   string                  `json:"licence"`
	Overrides map[string]string       `json:"overrides"`
	Verdicts  map[string]PinVerdict   `json:"verdicts"`
	Grammars  map[string]PrefillEntry `json:"grammars"`
}

// ParseGrammarPrefill decodes the data file strictly.
func ParseGrammarPrefill(b []byte) (GrammarPrefill, error) {
	var d GrammarPrefill
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return GrammarPrefill{}, fmt.Errorf("grammar pre-fill: %w", err)
	}
	return d, nil
}

// LoadGrammarPrefill reads the data file at file; a missing file is the empty
// prior (the first generation).
func LoadGrammarPrefill(file string) (GrammarPrefill, error) {
	b, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return GrammarPrefill{}, nil
	}
	if err != nil {
		return GrammarPrefill{}, err
	}
	return ParseGrammarPrefill(b)
}

// MarshalPrefill renders d as two-space indented JSON with a trailing
// newline; map keys sort, so the bytes depend on the content alone.
func MarshalPrefill(d GrammarPrefill) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// firstToken is the first whitespace-separated word of a queue
// grammar_source, the repo the queue names ("a/b (weak)" is "a/b").
func firstToken(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

var repoShape = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9._-]+$`)

// queueRepo is tok when it is an owner/repo; "" for a word ("none") or a
// non-repository source ("pure-go:encoding/json").
func queueRepo(tok string) string {
	if repoShape.MatchString(tok) {
		return tok
	}
	return ""
}

// PrefillLockRows builds the pre-fill for every tags-engine queue item (engine
// tags or tags+doc) whose grammar is not yet in the lock: not a locked blob,
// not a locked repo (a variant or a port, §13.1 step 8), not a pure-go source.
// The catalogue name is the queue id, unless prior.Overrides names another. A
// prior verdict for the item's own repo supplies the licence when it was
// checked at the same commit (or the catalogue has none). rev is the catalogue
// revision the data was read at.
func PrefillLockRows(q []QueueEntry, lock tsengine.GrammarLock, cat Catalogue, prior GrammarPrefill, rev string) (GrammarPrefill, error) {
	if !hex40.MatchString(rev) {
		return GrammarPrefill{}, fmt.Errorf("catalogue revision %q is not 40 lowercase hex", rev)
	}
	out := GrammarPrefill{
		Source: CatalogueSource, Revision: rev, Licence: CatalogueLicence,
		Overrides: map[string]string{}, Verdicts: map[string]PinVerdict{}, Grammars: map[string]PrefillEntry{},
	}
	queueIDs := map[string]bool{}
	for _, e := range q {
		queueIDs[e.ID] = true
	}
	for id, name := range prior.Overrides {
		if !queueIDs[id] {
			return GrammarPrefill{}, fmt.Errorf("override %q is not a queue id", id)
		}
		if _, ok := cat.Grammars[name]; !ok {
			return GrammarPrefill{}, fmt.Errorf("override %q names %q, which is not in the catalogue", id, name)
		}
		out.Overrides[id] = name
	}
	for id, v := range prior.Verdicts {
		if !queueIDs[id] {
			return GrammarPrefill{}, fmt.Errorf("verdict %q is not a queue id", id)
		}
		out.Verdicts[id] = v
	}
	lockedBlob := map[string]bool{}
	lockedRepo := map[string]bool{}
	for _, r := range lock.Grammars {
		lockedBlob[r.Blob] = true
		lockedRepo[strings.ToLower(r.Repo)] = true
	}
	for _, e := range q {
		if e.Engine != "tags" && e.Engine != "tags+doc" {
			continue
		}
		src := firstToken(e.GrammarSource)
		if strings.HasPrefix(src, "pure-go:") || lockedBlob[e.ID] || lockedRepo[strings.ToLower(src)] {
			continue
		}
		name := e.ID
		if o, ok := out.Overrides[e.ID]; ok {
			name = o
		}
		var v *PinVerdict
		if pv, ok := out.Verdicts[e.ID]; ok {
			v = &pv
		}
		out.Grammars[e.ID] = prefillEntry(src, cat, name, v)
	}
	return out, nil
}

// prefillEntry is one item's entry: src is the queue's first grammar token,
// name the catalogue name to look up, v the item's recorded verdict if any.
func prefillEntry(src string, cat Catalogue, name string, v *PinVerdict) PrefillEntry {
	var flags []string
	g, ok := cat.Grammars[name]
	e := PrefillEntry{}
	if ok && g.Repo != "" && g.Rev != "" {
		e.Catalogue = name
		e.Repo = catalogueRepo(g.Repo)
		e.Commit = g.Rev
		e.ABI = g.ABIVersion
		if e.ABI == 0 {
			e.ABI = catalogueDefaultABI
		}
		e.Generate = g.Generate
		e.Directory = g.Directory
		e.CSymbol = g.CSymbol
		if e.Licence = cat.licenceOf(g, e.Repo); e.Licence != "" {
			e.LicenceBasis = BasisCatalogue
		}
		if !strings.EqualFold(src, e.Repo) {
			flags = append(flags, "grammar-differs:"+src)
		}
		if e.ABI < 13 || e.ABI > 15 {
			flags = append(flags, fmt.Sprintf("abi-outside:%d", e.ABI))
		}
	} else {
		// No upstream to pin: the pack has no entry, or vendors the grammar in
		// tree (a "local" entry). The queue's own repo is the only lead.
		e.Repo = queueRepo(src)
		flags = append(flags, "not-in-catalogue")
	}
	if v != nil && strings.EqualFold(v.Repo, e.Repo) && (e.Commit == "" || e.Commit == v.Commit) {
		e.Licence, e.LicenceBasis = v.Licence, v.Basis
	}
	switch {
	case e.Licence == "":
		flags = append(flags, "unlicensed")
	case !tsengine.LicenceAllowed(e.Licence):
		flags = append(flags, "refused:"+e.Licence)
	}
	sort.Strings(flags)
	if flags == nil {
		flags = []string{}
	}
	e.Flags = flags
	return e
}

// PinDraft is the draft of one lock row for recipe step 6, with what to tell
// the worker. Row's digests and sizes stay empty for build.sh; Scanner is
// false until the worker sees src/scanner.c. Refused is true when the item
// must not be pinned as queued (a refused or unlicensed licence); Reasons say
// why and what the catalogue offers instead.
type PinDraft struct {
	Row     tsengine.GrammarLockRow
	Flags   []string
	Refused bool
	Reasons []string
}

// Draft returns id's draft lock row. It is an error for an id with no entry
// (not a tags item, or its grammar is already in the lock when the data was
// generated).
func (d GrammarPrefill) Draft(id string) (PinDraft, error) {
	e, ok := d.Grammars[id]
	if !ok {
		return PinDraft{}, fmt.Errorf("no pre-fill for %q: it is not a tags-engine queue item, or its grammar was already in the lock when %s was generated", id, GrammarPrefillPath)
	}
	v, hasVerdict := d.Verdicts[id]
	commit := e.Commit
	if commit == "" && hasVerdict && strings.EqualFold(v.Repo, e.Repo) {
		commit = v.Commit
	}
	sym := e.Catalogue
	if e.CSymbol != "" {
		sym = e.CSymbol
	}
	if sym == "" {
		sym = strings.ReplaceAll(id, "-", "_")
	}
	row := tsengine.GrammarLockRow{
		Blob: id, Repo: e.Repo, Tag: commit, Commit: commit, Licence: e.Licence,
		EntryPoints: []string{"tree_sitter_" + sym}, Generate: e.Generate, ABI: e.ABI,
	}
	if e.Directory != "" {
		row.Subdirs = []string{e.Directory}
	}
	dr := PinDraft{Row: row, Flags: append([]string(nil), e.Flags...)}
	for _, f := range e.Flags {
		switch {
		case f == "unlicensed":
			dr.Refused = true
			dr.Reasons = append(dr.Reasons, fmt.Sprintf("unlicensed: %s has no licence on record (%s)", e.Repo, licenceHint))
		case strings.HasPrefix(f, "refused:"):
			dr.Refused = true
			dr.Reasons = append(dr.Reasons, fmt.Sprintf("refused: %s is %s, off the allowlist (design §8.2)", e.Repo, strings.TrimPrefix(f, "refused:")))
		}
	}
	if hasVerdict && v.Refused && !strings.EqualFold(v.Repo, e.Repo) {
		dr.Refused = true
		dr.Reasons = append(dr.Reasons, fmt.Sprintf("refused: the queue's grammar %s @ %s is %s (%s)", v.Repo, shortCommit(v.Commit), v.Flag(), v.Reason))
		if e.Commit != "" && !dr.hasBlocking() {
			dr.Reasons = append(dr.Reasons, fmt.Sprintf("the catalogue offers %s @ %s (%s) instead; re-pinning is a queue decision", e.Repo, shortCommit(e.Commit), e.Licence))
		}
	}
	return dr, nil
}

const licenceHint = "run lang-pin -tree on the tree at the pinned commit"

func (p PinDraft) hasBlocking() bool {
	for _, f := range p.Flags {
		if f == "unlicensed" || strings.HasPrefix(f, "refused:") {
			return true
		}
	}
	return false
}

func shortCommit(c string) string {
	if len(c) > 12 {
		return c[:12]
	}
	return c
}

// ---------------------------------------------------------------------------
// The licence verdict at pin (LB-F51)

// LicenceVerdict is what the tree at a pinned commit says about its licence.
// Licence is "" when none could be established (no licence, an unrecognised
// text, declarations that disagree, or a licence file that disagrees with the
// decision): unlicensed and refused. Basis is BasisFile or BasisDeclared.
type LicenceVerdict struct {
	Licence string `json:"licence"`
	Basis   string `json:"licence_basis"`
	Refused bool   `json:"refused"`
	Reason  string `json:"reason,omitempty"`
}

// Flag is the pre-fill flag for the verdict: "" when accepted, "unlicensed"
// when no licence was established, "refused:<SPDX>" otherwise.
func (v LicenceVerdict) Flag() string {
	switch {
	case !v.Refused:
		return ""
	case v.Licence == "":
		return "unlicensed"
	default:
		return "refused:" + v.Licence
	}
}

const agplRefusal = "AGPL is refused (ADR-048 amendment, 2026-09-29)"

// gnuTitles are the GNU licence titles by family; the earliest in a text names
// it (the GPL-3.0 text mentions the AGPL and the LGPL, the LGPL the GPL).
var gnuTitles = []struct{ title, family string }{
	{"gnu affero general public license", "AGPL"},
	{"gnu lesser general public license", "LGPL"},
	{"gnu library general public license", "LGPL"},
	{"gnu general public license", "GPL"},
}

// refusedFamilies are licences the allowlist names as refused, recognised by
// a phrase of their text so a verdict can name them.
var refusedFamilies = []struct{ phrase, spdx string }{
	{"server side public license", "SSPL-1.0"},
	{"business source license", "BUSL-1.1"},
	{"functional source license", "FSL-1.1"},
	{"polyform", "PolyForm"},
	{"commons clause", "Commons-Clause"},
	{"european union public licen", "EUPL-1.2"},
	{"do what the fuck you want", "WTFPL"},
}

func normLicenceText(s string) string {
	var b strings.Builder
	space := true
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

// IdentifyLicenceText names the SPDX id a licence file's text states, or ""
// when it is not one it knows. It knows the allowlist (MIT, Apache-2.0,
// BSD-2/3-Clause, ISC, Unlicense, CC0-1.0, Zlib, 0BSD, MPL-2.0), the GNU
// family (GPL, LGPL, AGPL), the other families §8.2 refuses, and the near
// misses (MIT-0, BSD-4-Clause, MPL-1.1) that look allowlisted and are not.
func IdentifyLicenceText(text string) string {
	n := normLicenceText(text)
	if n == "" {
		return ""
	}
	has := func(p string) bool { return strings.Contains(n, p) }

	first, family := -1, ""
	for _, g := range gnuTitles {
		if i := strings.Index(n, g.title); i >= 0 && (first < 0 || i < first) {
			first, family = i, g.family
		}
	}
	switch family {
	case "AGPL":
		return "AGPL-3.0"
	case "LGPL":
		switch {
		case has("version 2 1"):
			return "LGPL-2.1"
		case has("library general public license") && has("version 2") && !has("version 3"):
			return "LGPL-2.0"
		}
		return "LGPL-3.0"
	case "GPL":
		if !has("version 3") && has("version 2") {
			return "GPL-2.0"
		}
		return "GPL-3.0"
	}
	for _, f := range refusedFamilies {
		if has(f.phrase) {
			return f.spdx
		}
	}
	switch {
	case has("mozilla public license"):
		if has("version 2 0") {
			return "MPL-2.0"
		}
		return "MPL-1.1"
	case has("apache license") && has("version 2 0"):
		return "Apache-2.0"
	case has("permission is hereby granted free of charge to any person obtaining a copy"):
		if has("the above copyright notice and this permission notice shall be included in all copies") {
			return "MIT"
		}
		return "MIT-0"
	case has("permission to use copy modify and or distribute this software for any purpose with or without fee is hereby granted"),
		has("permission to use copy modify and distribute this software for any purpose with or without fee is hereby granted"):
		if has("provided that the above copyright notice and this permission notice appear in all copies") {
			return "ISC"
		}
		return "0BSD"
	case has("redistribution and use in source and binary forms with or without modification are permitted provided that the following conditions are met"):
		switch {
		case has("all advertising materials mentioning features"):
			return "BSD-4-Clause"
		case has("neither the name of"), has("endorse or promote"):
			return "BSD-3-Clause"
		}
		return "BSD-2-Clause"
	case has("this is free and unencumbered software released into the public domain"):
		return "Unlicense"
	case has("cc0 1 0 universal"):
		return "CC0-1.0"
	case has("this software is provided as is without any express or implied warranty") && has("altered source versions must be plainly marked"):
		return "Zlib"
	}
	return ""
}

// GrammarLicenceVerdict decides a grammar tree's licence from its files
// (LicenceTreeFiles or LicenceTreeFilesTar), design §8.2:
//
//   - a top-level LICENSE*, LICENCE* or COPYING* decides, identified by its
//     text (several: all must be on the allowlist; the first by name is the
//     licence, the others are named in the reason);
//   - with none, package.json license, Cargo.toml package.license and
//     tree-sitter.json metadata.license count when every one present agrees on
//     one allowlisted SPDX id (basis "declared"); disagreeing or absent
//     declarations leave the grammar unlicensed;
//   - AGPL is always refused, wherever it is read;
//   - a licence file under queries/ or src/ that names another licence, or
//     none it knows, refuses.
//
// A verdict with Refused set must not be pinned.
func GrammarLicenceVerdict(files map[string][]byte) LicenceVerdict {
	var top, deep []string
	for p := range files {
		if !isLicenceFileName(path.Base(p)) {
			continue
		}
		if !strings.Contains(p, "/") {
			top = append(top, p)
		} else if underQueriesOrSrc(p) {
			deep = append(deep, p)
		}
	}
	sort.Strings(top)
	sort.Strings(deep)

	var v LicenceVerdict
	accepted := map[string]bool{}
	if len(top) > 0 {
		v = verdictFromLicenceFiles(files, top, accepted)
	} else {
		v = verdictFromManifests(files)
		if v.Licence != "" {
			accepted[v.Licence] = true
		}
	}
	if v.Licence == "" && v.Refused {
		return v // unlicensed already; nothing under queries/ or src/ improves that
	}
	for _, p := range deep {
		id := IdentifyLicenceText(string(files[p]))
		switch {
		case isAGPL(id):
			return LicenceVerdict{Licence: id, Basis: v.Basis, Refused: true, Reason: fmt.Sprintf("%s: %s reads as %s", agplRefusal, p, id)}
		case v.Refused:
			// already refused for its own reason
		case id == "":
			return LicenceVerdict{Refused: true, Reason: fmt.Sprintf("%s: licence text not recognised; the decision was %s (licence unclear)", p, v.Licence)}
		case !accepted[id]:
			return LicenceVerdict{Refused: true, Reason: fmt.Sprintf("%s reads as %s but the tree is %s (%s): a mismatch refuses (§8.2)", p, id, v.Licence, v.Basis)}
		}
	}
	return v
}

func isAGPL(spdx string) bool { return strings.HasPrefix(spdx, "AGPL") }

// verdictFromLicenceFiles decides from the top-level licence files (sorted),
// filling accepted with the licences a nested file may state.
func verdictFromLicenceFiles(files map[string][]byte, top []string, accepted map[string]bool) LicenceVerdict {
	type read struct{ file, spdx string }
	var reads []read
	for _, p := range top {
		id := IdentifyLicenceText(string(files[p]))
		if id == "" {
			return LicenceVerdict{Refused: true, Reason: fmt.Sprintf("%s: licence text not recognised (licence unclear)", p)}
		}
		reads = append(reads, read{p, id})
	}
	for _, r := range reads {
		if isAGPL(r.spdx) {
			return LicenceVerdict{Licence: r.spdx, Basis: BasisFile, Refused: true, Reason: fmt.Sprintf("%s: %s reads as %s", agplRefusal, r.file, r.spdx)}
		}
	}
	for _, r := range reads {
		if !tsengine.LicenceAllowed(r.spdx) {
			return LicenceVerdict{Licence: r.spdx, Basis: BasisFile, Refused: true, Reason: fmt.Sprintf("%s reads as %s, which is not on the allowlist (§8.2)", r.file, r.spdx)}
		}
	}
	v := LicenceVerdict{Licence: reads[0].spdx, Basis: BasisFile}
	var also []string
	for _, r := range reads {
		accepted[r.spdx] = true
		if r != reads[0] {
			also = append(also, fmt.Sprintf("%s: %s", r.file, r.spdx))
		}
	}
	if len(also) > 0 {
		v.Reason = "also " + strings.Join(also, ", ")
	}
	return v
}

// manifestLicences reads the three declarations: package.json license,
// Cargo.toml package.license and tree-sitter.json metadata.license. A
// manifest that is present but does not parse is an error naming it; one that
// declares nothing (or declares it in a form that is not one SPDX string, a
// workspace-inherited Cargo licence) contributes nothing.
func manifestLicences(files map[string][]byte) (declared []declaredLicence, err error) {
	if b, ok := files["package.json"]; ok {
		var m struct {
			License any `json:"license"`
		}
		if e := json.Unmarshal(b, &m); e != nil {
			return nil, fmt.Errorf("package.json: %w", e)
		}
		switch l := m.License.(type) {
		case string:
			declared = append(declared, declaredLicence{"package.json", l})
		case map[string]any: // the legacy {"type": "MIT", "url": ...}
			if t, ok := l["type"].(string); ok {
				declared = append(declared, declaredLicence{"package.json", t})
			}
		}
	}
	if b, ok := files["Cargo.toml"]; ok {
		var m struct {
			Package struct {
				License any `toml:"license"`
			} `toml:"package"`
		}
		if e := toml.Unmarshal(b, &m); e != nil {
			return nil, fmt.Errorf("Cargo.toml: %w", e)
		}
		if l, ok := m.Package.License.(string); ok {
			declared = append(declared, declaredLicence{"Cargo.toml", l})
		}
	}
	if b, ok := files["tree-sitter.json"]; ok {
		var m struct {
			Metadata struct {
				License any `json:"license"`
			} `json:"metadata"`
		}
		if e := json.Unmarshal(b, &m); e != nil {
			return nil, fmt.Errorf("tree-sitter.json: %w", e)
		}
		if l, ok := m.Metadata.License.(string); ok {
			declared = append(declared, declaredLicence{"tree-sitter.json", l})
		}
	}
	var kept []declaredLicence
	for _, d := range declared {
		d.spdx = strings.TrimSpace(strings.Trim(strings.TrimSpace(d.spdx), "()"))
		if d.spdx != "" {
			kept = append(kept, d)
		}
	}
	return kept, nil
}

type declaredLicence struct{ file, spdx string }

func verdictFromManifests(files map[string][]byte) LicenceVerdict {
	declared, err := manifestLicences(files)
	if err != nil {
		return LicenceVerdict{Refused: true, Reason: fmt.Sprintf("no LICENSE file and %v (licence unclear)", err)}
	}
	if len(declared) == 0 {
		return LicenceVerdict{Refused: true, Reason: "no LICENSE file and no licence declared in package.json, Cargo.toml or tree-sitter.json"}
	}
	distinct := map[string]bool{}
	for _, d := range declared {
		distinct[d.spdx] = true
	}
	if len(distinct) > 1 {
		var parts []string
		for _, d := range declared {
			parts = append(parts, d.file+" "+d.spdx)
		}
		return LicenceVerdict{Refused: true, Reason: "no LICENSE file and the declared licences disagree: " + strings.Join(parts, ", ")}
	}
	spdx := declared[0].spdx
	var from []string
	for _, d := range declared {
		from = append(from, d.file)
	}
	switch {
	case isAGPL(spdx):
		return LicenceVerdict{Licence: spdx, Basis: BasisDeclared, Refused: true, Reason: fmt.Sprintf("%s: declared in %s", agplRefusal, strings.Join(from, ", "))}
	case !tsengine.LicenceAllowed(spdx):
		return LicenceVerdict{Licence: spdx, Basis: BasisDeclared, Refused: true, Reason: fmt.Sprintf("declared %s in %s, which is not on the allowlist (§8.2)", spdx, strings.Join(from, ", "))}
	}
	return LicenceVerdict{Licence: spdx, Basis: BasisDeclared, Reason: "declared in " + strings.Join(from, ", ")}
}

// isLicenceFileName: LICENSE*, LICENCE* or COPYING* by name, case-insensitive,
// that is text, not code (src/license.c is not a licence).
func isLicenceFileName(base string) bool {
	l := strings.ToLower(base)
	if !strings.HasPrefix(l, "license") && !strings.HasPrefix(l, "licence") && !strings.HasPrefix(l, "copying") {
		return false
	}
	switch path.Ext(l) {
	case ".c", ".h", ".cc", ".cpp", ".go", ".rs", ".js", ".mjs", ".ts", ".py", ".rb", ".java", ".sh", ".json", ".toml", ".yaml", ".yml", ".scm", ".lock", ".sum", ".mod":
		return false
	}
	return true
}

func underQueriesOrSrc(p string) bool {
	segs := strings.Split(p, "/")
	for _, s := range segs[:len(segs)-1] {
		if s == "queries" || s == "src" {
			return true
		}
	}
	return false
}

// hasSegment reports whether name is one of p's directory segments.
func hasSegment(p, name string) bool {
	segs := strings.Split(p, "/")
	for _, s := range segs[:len(segs)-1] {
		if s == name {
			return true
		}
	}
	return false
}

// maxLicenceFileBytes bounds one file read for a verdict; the longest licence
// text (GPL-3.0) is 35 KiB.
const maxLicenceFileBytes = 1 << 20

// treeManifests are the root files a declared licence is read from.
var treeManifests = map[string]bool{"package.json": true, "Cargo.toml": true, "tree-sitter.json": true}

// wantTreeFile reports whether a tree file matters to the verdict: a licence
// file at the top or under a queries/ or src/ directory, or a root manifest.
func wantTreeFile(rel string) bool {
	if hasSegment(rel, "node_modules") || hasSegment(rel, ".git") {
		return false
	}
	if treeManifests[rel] {
		return true
	}
	if !isLicenceFileName(path.Base(rel)) {
		return false
	}
	return !strings.Contains(rel, "/") || underQueriesOrSrc(rel)
}

// LicenceTreeFiles collects the files a verdict reads from a checkout:
// path (slash-separated, relative to the root) to content.
func LicenceTreeFiles(fsys fs.FS) (map[string][]byte, error) {
	out := map[string][]byte{}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != "." && (d.Name() == ".git" || d.Name() == "node_modules") {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !wantTreeFile(p) {
			return nil
		}
		f, err := fsys.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		b, err := io.ReadAll(io.LimitReader(f, maxLicenceFileBytes))
		if err != nil {
			return err
		}
		out[p] = b
		return nil
	})
	return out, err
}

// LicenceTreeFilesTar is LicenceTreeFiles for a tarball, plain or gzipped. A
// GitHub tarball puts every path under one top directory, which is stripped; a
// tarball whose files share no top directory is read as it is.
func LicenceTreeFilesTar(r io.Reader) (map[string][]byte, error) {
	br := bufio.NewReader(r)
	if magic, err := br.Peek(2); err == nil && magic[0] == 0x1f && magic[1] == 0x8b {
		zr, err := gzip.NewReader(br)
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		r = zr
	} else {
		r = br
	}
	type candidate struct {
		name string
		data []byte
	}
	var cands []candidate
	top, sameTop, n := "", true, 0
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		name := path.Clean(h.Name)
		seg, rel, hasDir := strings.Cut(name, "/")
		switch {
		case !hasDir:
			sameTop = false
		case n == 0:
			top = seg
		case seg != top:
			sameTop = false
		}
		n++
		if wantTreeFile(name) || (hasDir && wantTreeFile(rel)) {
			b, err := io.ReadAll(io.LimitReader(tr, maxLicenceFileBytes))
			if err != nil {
				return nil, err
			}
			cands = append(cands, candidate{name, b})
		}
	}
	out := map[string][]byte{}
	for _, c := range cands {
		rel := c.name
		if sameTop {
			_, rel, _ = strings.Cut(c.name, "/")
		}
		if wantTreeFile(rel) {
			out[rel] = c.data
		}
	}
	return out, nil
}

// LicenceTreeFilesFromPath reads a checkout directory or a tarball file.
func LicenceTreeFilesFromPath(p string) (map[string][]byte, error) {
	st, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return LicenceTreeFiles(os.DirFS(p))
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LicenceTreeFilesTar(f)
}
