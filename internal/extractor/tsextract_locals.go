package extractor

// tsextract_locals.go — the tags engine's locals rule (LB-F254, design §3.4).
//
// tree-sitter-tags decides a pattern carrying `(#is-not? local)` with the
// grammar's locals query: a name that a @local.definition in a scope the
// reference can see binds at or before it is a local variable, never a tag.
// fathom's engine evaluates text predicates only (tsengine/predicates.go), so
// a dialect that needs the rule names its locals query in the manifest
// (queries.locals) and marks each call pattern the rule decides with
// `(#set! is-not "local")`. Ruby is the dialect: a bare identifier (`helper`,
// no receiver, no parentheses, no arguments) is a method call on self unless
// a local of that name was written before it in the same def, class, module
// or top level, or in one that a block or lambda around it sees.
//
// The locals query is appended to the tags query (one walk of the tree), and
// after the query ran, per file:
//
//   - a candidate (a match of a pattern set is-not "local") whose
//     @reference.call node another match names — its @name, its
//     @reference.call, an @ignore or a predicate's own capture (@_m) — is no
//     call of its own: one name node yields one tag, as tree-sitter-tags reads
//     a tags query (a method's own name, a call's method name, `def x=`);
//   - every other candidate is looked up through the locals query's scopes:
//     @local.scope nodes nest, the file is the outermost, and a scope set
//     `(#set! local.scope-inherits "false")` does not see its parent's
//     locals; a @local.definition belongs to the innermost scope holding it
//     and binds its name from its own start on. A candidate a definition
//     binds is dropped; the rest are calls;
//   - the locals query's own matches are dropped.
//
// Every other dialect names no locals query: its matches are untouched.

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// The names the locals rule reads.
const (
	settingIsNot       = "is-not" // tags query: (#set! is-not "local") marks a candidate
	isNotLocal         = "local"
	capLocalScope      = "local.scope"      // locals query
	capLocalDefinition = "local.definition" // locals query
	capIgnore          = "ignore"           // tags query: a name that is no tag of its own
	settingInherits    = "local.scope-inherits"
)

// loadLocalsQuery reads the manifest's queries.locals into r and appends it
// to r's query. A tags query that sets is-not "local" with no locals query to
// decide it is a manifest error, as is a locals query no tags pattern uses.
func (r *tsDialectRules) loadLocalsQuery(fsys fs.FS, d Dialect) error {
	uses := strings.Contains(r.query, "(#set! "+settingIsNot+` "`+isNotLocal+`")`)
	if d.Queries.Locals == "" {
		if uses {
			return fmt.Errorf("dialect %s: a tags pattern sets %s %q but the manifest names no queries.locals", d.ID, settingIsNot, isNotLocal)
		}
		return nil
	}
	if !uses {
		return fmt.Errorf("dialect %s: queries.locals %s decides nothing: no tags pattern sets %s %q", d.ID, d.Queries.Locals, settingIsNot, isNotLocal)
	}
	b, err := fs.ReadFile(fsys, path.Join("dialects", d.ID, d.Queries.Locals))
	if err != nil {
		return fmt.Errorf("dialect %s: %w", d.ID, err)
	}
	r.localsQuery = string(b)
	r.query += "\n" + r.localsQuery
	return nil
}

// keepNonLocalCalls is matches less every candidate the locals rule reads as
// no call and less the locals query's own matches (see the file comment).
// Matches keep their order.
func keepNonLocalCalls(q *tsengine.Query, src []byte, matches []tsengine.Match) []tsengine.Match {
	const (
		other = iota
		candidate
		locals
	)
	roles := make([]uint8, len(matches))
	claimed := map[[2]uint32]bool{}
	var scopeMatches []tsengine.Match
	for i := range matches {
		m := &matches[i]
		if q.PatternSetting(m.PatternIndex, settingIsNot) == isNotLocal {
			roles[i] = candidate
			continue
		}
		for _, c := range m.Captures {
			switch {
			case c.Name == capLocalScope || c.Name == capLocalDefinition:
				roles[i] = locals
			case c.Name == "name" || c.Name == "reference.call" || c.Name == capIgnore || strings.HasPrefix(c.Name, "_"):
				claimed[[2]uint32{c.Node.StartByte, c.Node.EndByte}] = true
			}
		}
		if roles[i] == locals {
			scopeMatches = append(scopeMatches, *m)
		}
	}
	sc := newLocalScopes(q, scopeMatches, src)
	out := matches[:0:0]
	for i := range matches {
		switch roles[i] {
		case locals:
			continue
		case candidate:
			n, ok := callSiteNode(&matches[i])
			if !ok || claimed[[2]uint32{n.StartByte, n.EndByte}] || sc.binds(n.Text(src), n.StartByte, n.EndByte) {
				continue
			}
		}
		out = append(out, matches[i])
	}
	return out
}

func callSiteNode(m *tsengine.Match) (tsengine.Node, bool) {
	for _, c := range m.Captures {
		if c.Name == "reference.call" {
			return c.Node, true
		}
	}
	return tsengine.Node{}, false
}

// localScope is one @local.scope (index 0 is the file).
type localScope struct {
	start, end uint32
	inherits   bool
	parent     int
	defs       map[string]uint32 // name -> the earliest definition's start
}

// localScopes is a file's scope tree, sorted by start (outer first on a tie).
type localScopes []localScope

func newLocalScopes(q *tsengine.Query, ms []tsengine.Match, src []byte) localScopes {
	type span struct {
		start, end uint32
		inherits   bool
	}
	byRange := map[[2]uint32]int{}
	var spans []span
	var defs []tsengine.Node
	for _, m := range ms {
		for _, c := range m.Captures {
			switch c.Name {
			case capLocalScope:
				k := [2]uint32{c.Node.StartByte, c.Node.EndByte}
				inh := q.PatternSetting(m.PatternIndex, settingInherits) != "false"
				if i, seen := byRange[k]; seen {
					spans[i].inherits = spans[i].inherits && inh
					continue
				}
				byRange[k] = len(spans)
				spans = append(spans, span{c.Node.StartByte, c.Node.EndByte, inh})
			case capLocalDefinition:
				defs = append(defs, c.Node)
			}
		}
	}
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].start != spans[j].start {
			return spans[i].start < spans[j].start
		}
		return spans[i].end > spans[j].end
	})
	sc := make(localScopes, 0, len(spans)+1)
	sc = append(sc, localScope{start: 0, end: uint32(len(src)), parent: -1})
	stack := []int{0}
	for _, s := range spans {
		for len(stack) > 1 && sc[stack[len(stack)-1]].end <= s.start {
			stack = stack[:len(stack)-1]
		}
		sc = append(sc, localScope{start: s.start, end: s.end, inherits: s.inherits, parent: stack[len(stack)-1]})
		stack = append(stack, len(sc)-1)
	}
	for _, d := range defs {
		i := sc.innermost(d.StartByte, d.EndByte)
		if sc[i].defs == nil {
			sc[i].defs = map[string]uint32{}
		}
		name := d.Text(src)
		if at, seen := sc[i].defs[name]; !seen || d.StartByte < at {
			sc[i].defs[name] = d.StartByte
		}
	}
	return sc
}

// innermost is the smallest scope holding [start, end).
func (sc localScopes) innermost(start, end uint32) int {
	i := sort.Search(len(sc), func(i int) bool { return sc[i].start > start }) - 1
	for i > 0 && !(sc[i].start <= start && end <= sc[i].end) {
		i = sc[i].parent
	}
	return max(i, 0)
}

// binds reports whether a local named name is in scope at [start, end).
func (sc localScopes) binds(name string, start, end uint32) bool {
	for i := sc.innermost(start, end); i >= 0; i = sc[i].parent {
		if at, ok := sc[i].defs[name]; ok && at <= start {
			return true
		}
		if !sc[i].inherits {
			return false
		}
	}
	return false
}
