package intent

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
)

// family.go — K8: a living document split behind a router.
//
// docs/intent/<name>.md stays at its path as the router and
// docs/intent/<name>/*.md hold the records (the project-intent kit's
// ~150-line living-doc budget, which futures.md passed five times over).
// The corpus reads a router and its members as ONE document, in path order
// (the router sorts before its directory): a record's id and its
// per-record condition numbering carry across a member boundary exactly as
// they carried across a section heading in the single file, so a split
// changes where a record sits and never what it is called. A router's
// listing line (an id beside a pointer to the member that holds it) lists;
// it never defines. FamilyRouteBreaks is the zero-citation-break check.

// Family is one living document split behind a router.
type Family struct {
	Name    string   // "futures"
	Router  string   // "docs/intent/futures.md", repo-relative
	Members []string // "docs/intent/futures/f01-f09.md", …, sorted
	// Defined is every record definition in read order: a member's
	// `## F12.` heading, `**G2.` opener or (for a family with no id shape)
	// `## Section` heading, or one the router still holds itself.
	Defined []RecordSite
	// Routed is every listing the router makes: an id beside a pointer to
	// the member that defines it, in line order.
	Routed []RecordSite
}

// RecordSite is one place a family names a record.
type RecordSite struct {
	ID     string
	File   string // repo-relative
	Line   int    // 1-based
	Member string // a listing's target member, repo-relative; "" on a definition
}

// condState is the record attribution and per-record numbering a
// condition parse carries from line to line — across one file, or across a
// whole family read as one document.
type condState struct {
	record string
	counts map[string]int
	fileID string // the id a condition outside any record takes
}

func newCondState(fileID string) *condState {
	return &condState{counts: map[string]int{}, fileID: fileID}
}

// familyParse is one family's shared parse state during Load.
type familyParse struct {
	idx   int // into Corpus.Families
	cond  *condState
	shape bool // some file of the family defines a record of the doc's shape
}

// familyIDShapes are the families whose records carry ids: the definition
// opener (the parser's own record grammar) and the id token a router's
// listing line names. A family with none (domain.md) is keyed by its `##`
// section headings instead — they are what a citation names.
var familyIDShapes = map[string]struct{ def, token *regexp.Regexp }{
	"futures":    {futureRe, regexp.MustCompile(`\bF\d+\b`)},
	"scope":      {goalRe, regexp.MustCompile(`\bG\d+\b`)},
	"questions":  {questionRe, regexp.MustCompile(`\bQ\d+\b`)},
	"incidents":  {incidentRe, regexp.MustCompile(`\bINC-\d+\b`)},
	"invariants": {invRowRe, regexp.MustCompile(`\bI\d+\b`)},
}

var sectionHeadingRe = regexp.MustCompile(`^##\s+(.+?)\s*#*\s*$`)

// detectFamilies finds every docs/intent/<name>.md that has a
// docs/intent/<name>/ beside it holding Markdown. A directory with no router
// beside it (decisions/) is not a family: its files parse as they always
// did.
func (c *Corpus) detectFamilies(rels []string) {
	present := map[string]bool{}
	for _, r := range rels {
		present[r] = true
	}
	intentDir := filepath.ToSlash(routerRelDir)
	byName := map[string]*Family{}
	for _, r := range rels {
		dir := path.Dir(r)
		if path.Dir(dir) != intentDir {
			continue
		}
		name := path.Base(dir)
		router := intentDir + "/" + name + ".md"
		if name == "README" || !present[router] {
			continue
		}
		f := byName[name]
		if f == nil {
			f = &Family{Name: name, Router: router}
			byName[name] = f
		}
		f.Members = append(f.Members, r)
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	c.famByFile = map[string]*familyParse{}
	for i, n := range names {
		f := byName[n]
		sort.Strings(f.Members)
		c.Families = append(c.Families, *f)
		fp := &familyParse{idx: i, cond: newCondState(n)}
		c.famParses = append(c.famParses, fp)
		c.famByFile[f.Router] = fp
		for _, m := range f.Members {
			c.famByFile[m] = fp
		}
	}
}

// docName is the living document a file belongs to: its family's name, or
// its own base name.
func (c *Corpus) docName(rel string) (string, *familyParse) {
	if fp := c.famByFile[rel]; fp != nil {
		return c.Families[fp.idx].Name, fp
	}
	return strings.TrimSuffix(filepath.Base(rel), ".md"), nil
}

// isListing reports whether a router line lists a record: it names a
// member of the family and the record's key. Only the router lists.
func (c *Corpus) isListing(fp *familyParse, rel, line string) bool {
	if fp == nil {
		return false
	}
	f := &c.Families[fp.idx]
	if rel != f.Router {
		return false
	}
	_, id := listing(f, strings.TrimSpace(line))
	return id != ""
}

// listing reads a router line as a listing: the member it points at and the
// record key it names, or "" when the line is not one.
func listing(f *Family, t string) (member, id string) {
	member = memberPointer(f, t)
	if member == "" {
		return "", ""
	}
	if shape, ok := familyIDShapes[f.Name]; ok {
		return member, shape.token.FindString(t)
	}
	// A family keyed by section headings lists them in a table: the first
	// cell names the section.
	if !strings.HasPrefix(t, "|") {
		return member, ""
	}
	cells := tableCells(t)
	if len(cells) == 0 {
		return member, ""
	}
	key := strings.TrimSpace(strings.Trim(stripEmphasis(cells[0]), "` "))
	if key == "" || strings.Contains(cells[0], ".md") {
		return member, ""
	}
	return member, key
}

// memberPointer returns the first member of f a line points at — a
// backticked path or a markdown link, relative to the router or to the repo
// root — or "".
func memberPointer(f *Family, t string) string {
	prefix := path.Dir(f.Router) + "/" + f.Name + "/"
	var toks []string
	for _, m := range backtickRe.FindAllStringSubmatch(t, -1) {
		toks = append(toks, m[1])
	}
	for _, m := range mdLinkRe.FindAllStringSubmatch(t, -1) {
		toks = append(toks, m[1])
	}
	for _, tok := range toks {
		tok = strings.TrimSpace(stripFragment(tok))
		if !strings.HasSuffix(tok, ".md") {
			continue
		}
		for _, cand := range []string{path.Clean(path.Join(path.Dir(f.Router), tok)), path.Clean(tok)} {
			if strings.HasPrefix(cand, prefix) {
				return cand
			}
		}
	}
	return ""
}

// definitionID is the record a line defines in a family, or "".
func definitionID(name, t string) string {
	if shape, ok := familyIDShapes[name]; ok {
		if m := shape.def.FindStringSubmatch(t); m != nil {
			return m[1]
		}
		return ""
	}
	if m := sectionHeadingRe.FindStringSubmatch(t); m != nil {
		return strings.TrimSpace(stripEmphasis(m[1]))
	}
	return ""
}

// collectSites records where one file of a family defines and lists
// records.
func (c *Corpus) collectSites(fp *familyParse, rel, body string) {
	f := &c.Families[fp.idx]
	for i, ln := range splitLines(body) {
		t := strings.TrimSpace(ln)
		if rel == f.Router {
			if member, id := listing(f, t); id != "" {
				f.Routed = append(f.Routed, RecordSite{ID: id, File: rel, Line: i + 1, Member: member})
				continue
			}
		}
		if id := definitionID(f.Name, t); id != "" {
			f.Defined = append(f.Defined, RecordSite{ID: id, File: rel, Line: i + 1})
		}
	}
}

// finishFamilies runs the healthy-empty guard once per family, across all
// of its files: a router that lists and members that define is the shape,
// and only a family where NO file defines a record is a finding — named at
// the router, the file a reader opens.
func (c *Corpus) finishFamilies() {
	for _, fp := range c.famParses {
		f := c.Families[fp.idx]
		what := shapeWhat[f.Name]
		if what == "" || fp.shape {
			continue
		}
		c.Unparsed = append(c.Unparsed, Unparsed{
			File: f.Router, Line: 1,
			Reason: "no " + what + " found in " + f.Router + " or its members under " + path.Dir(f.Router) + "/" + f.Name + "/ — the document's shape changed or it is empty",
		})
	}
}

// shapeWhat names the record shape a living document must carry (the
// healthy-empty guard), by document.
var shapeWhat = map[string]string{
	"scope":   "goal (`**G<n>.`)",
	"futures": "future (`## F<n>.`)",
}

// FamilyRouteBreaks is the zero-citation-break check (K8, U15): for every
// family, a record a member defines that the router does not list (a
// reader following the router never finds it), a record the router lists
// that no file defines or that sits in a different member than the router
// says (a citation that resolves to nothing), and a record defined more
// than once (a citation that resolves to whichever copy a reader opens).
// A record the router still holds itself is routed by construction.
func FamilyRouteBreaks(c Corpus) []Claim {
	var out []Claim
	for _, f := range c.Families {
		defs := map[string][]RecordSite{}
		for _, d := range f.Defined {
			defs[d.ID] = append(defs[d.ID], d)
		}
		routes := map[string][]RecordSite{}
		for _, r := range f.Routed {
			routes[r.ID] = append(routes[r.ID], r)
		}
		ref := func(id string) string { return f.Router + " " + id }
		for _, id := range sortedSiteKeys(defs) {
			sites := defs[id]
			if len(sites) > 1 {
				var at []string
				for _, s := range sites {
					at = append(at, s.File+":"+itoa(s.Line))
				}
				out = append(out, Claim{
					Kind: "U15", Ref: ref(id),
					Detail:   fmt.Sprintf("%s is defined %d times in the %s family (%s): a citation resolves to whichever copy a reader opens first", id, len(sites), f.Name, strings.Join(at, ", ")),
					Remedy:   "keep one definition; delete the other or give it a new id, and fix the router row",
					Evidence: at[1],
				})
			}
			inRouter := false
			for _, s := range sites {
				if s.File == f.Router {
					inRouter = true
				}
			}
			if !inRouter && len(routes[id]) == 0 {
				s := sites[0]
				out = append(out, Claim{
					Kind: "U15", Ref: ref(id),
					Detail:   fmt.Sprintf("%s is defined in a member (%s:%d) that the router %s does not list, so a reader following the router never finds it", id, s.File, s.Line, f.Router),
					Remedy:   fmt.Sprintf("add a row for %s (id, title, `%s`) to %s", id, relFromRouter(f.Router, s.File), f.Router),
					Evidence: s.File + ":" + itoa(s.Line),
				})
			}
		}
		for _, id := range sortedSiteKeys(routes) {
			sites := defs[id]
			for _, r := range routes[id] {
				ev := r.File + ":" + itoa(r.Line)
				if len(sites) == 0 {
					out = append(out, Claim{
						Kind: "U15", Ref: ref(id),
						Detail:   fmt.Sprintf("%s routed %s to %s, and no file of the %s family defines it: a citation of %s resolves to nothing", ev, id, r.Member, f.Name, id),
						Remedy:   "restore the record in the member, or drop the router row if the record was deleted on purpose",
						Evidence: ev,
					})
					continue
				}
				found := false
				for _, s := range sites {
					if s.File == r.Member {
						found = true
					}
				}
				if !found {
					out = append(out, Claim{
						Kind: "U15", Ref: ref(id),
						Detail:   fmt.Sprintf("%s routed %s to %s, but it is defined at %s:%d", ev, id, r.Member, sites[0].File, sites[0].Line),
						Remedy:   fmt.Sprintf("point the router row at `%s`", relFromRouter(f.Router, sites[0].File)),
						Evidence: ev,
					})
				}
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Ref < out[j].Ref })
	return out
}

// auditFamilyRoutes adds the family routing breaks to the audit (U15).
func (a *Audit) auditFamilyRoutes(in AuditInput) {
	a.Unproven = append(a.Unproven, FamilyRouteBreaks(in.Corpus)...)
}

func sortedSiteKeys(m map[string][]RecordSite) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// relFromRouter writes target relative to the directory of from, slash-separated.
func relFromRouter(from, target string) string {
	if r, err := filepath.Rel(filepath.Dir(filepath.FromSlash(from)), filepath.FromSlash(target)); err == nil {
		return filepath.ToSlash(r)
	}
	return target
}

// FamilyFiles lists a living document's files the way the corpus reads
// them as one document: docs/intent/<name>.md first, then
// docs/intent/<name>/*.md sorted. The router is listed whether or not it
// exists (a reader names its own absence); a member directory that exists
// and cannot be listed is an error, never an empty family (I38). Product
// readers outside this package (console search, the progress page's cache
// key) take their file set from here, so a split never hides a member.
func FamilyFiles(root, name string) ([]string, error) {
	dir := filepath.ToSlash(routerRelDir)
	out := []string{dir + "/" + name + ".md"}
	entries, err := os.ReadDir(filepath.Join(root, routerRelDir, name))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			return out, nil
		}
		return out, fmt.Errorf("%s/%s/: %w", dir, name, err)
	}
	var members []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		members = append(members, dir+"/"+name+"/"+e.Name())
	}
	sort.Strings(members)
	return append(out, members...), nil
}

// LivingDocOf names the living document a corpus file belongs to: a
// file's own base name, or — for a file in a directory under docs/intent/ —
// that directory's name ("docs/intent/futures/f01-f09.md" → "futures").
// Path-only, so a caller classifying a condition by its file keeps a split
// document one document.
func LivingDocOf(rel string) string {
	rel = filepath.ToSlash(rel)
	dir := path.Dir(rel)
	if path.Dir(dir) == filepath.ToSlash(routerRelDir) {
		return path.Base(dir)
	}
	return strings.TrimSuffix(path.Base(rel), ".md")
}
