package extractor

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// tsextract_members.go — the member pass (LB-F241, wave lx, item python): a
// definition a dialect's tags query marks with a member capture is named for
// the class it belongs to, not for its node's immediate structural parent,
// and is kept once per owner and name. A query with no member capture (every
// dialect but python) leaves the definitions untouched: scopeTSMembers returns
// the same slice it was given, so their output is byte-identical.
//
// The marks, written on the same match as the definition's @definition.<kind>
// and @name:
//
//   - @member: the nearest enclosing definition that is a container or a
//     callable decides. A container: the definition is that container's
//     member, named Owner<method_sep>name (a class attribute, also one inside
//     an `if` in the class body). None: it stays at module scope under its own
//     name (an assignment inside a module-level `if`/`for`/`with`/`try`). A
//     callable: it is a local, and dropped.
//   - @member.strict: as @member, but dropped at module scope too (a shape
//     that is a member's only, such as an annotation with no value).
//   - @member.self: an attribute a method sets on its instance (`self.x = v`):
//     the member of the nearest enclosing container, through any callables;
//     dropped when there is none.
//
// Two matches building the same definition (one node, name and kind: a
// general marked pattern overlapping an older, narrower unmarked one) leave
// one definition, the older pattern's when either was unmarked, so its name
// and kind are unchanged. A member is kept once per (owner, name), the first in
// source order, the position scip-python defines it at: a later assignment to
// the same attribute, and one that an unmarked definition of that owner
// already names (a method, a nested class, an annotated class attribute the
// legacy pattern names bare), is dropped. Module-scope names are not
// de-duplicated, as the unmarked module-level pattern's never were.
//
// The pass runs after qualifyTSDefs (parents and qualified names are known)
// and before attachTSCalls, so a dropped definition takes no calls: a call
// attachment (@calls.to) naming a dropped definition's node finds none and the
// call stays with its enclosing callable.

const (
	tsMemberCapture       = "member"
	tsMemberStrictCapture = "member.strict"
	tsMemberSelfCapture   = "member.self"
)

type tsMemberMark uint8

const (
	tsMemberNone tsMemberMark = iota
	tsMemberScoped
	tsMemberStrict
	tsMemberSelf
)

// tsMemberKey identifies a definition by what splitTSMatchesWith builds it
// from: its node, its name and its kind.
type tsMemberKey struct {
	start, end uint32
	name       string
	kind       string
}

func tsMemberKeyOf(d *tsDef) tsMemberKey {
	return tsMemberKey{d.start, d.end, d.name, string(d.kind)}
}

// tsMarked is one key's marked matches: their mark, and how many there are.
type tsMarked struct {
	mark  tsMemberMark
	count int
}

// tsMemberMarks reads the member marks off the query's matches, keyed by the
// marked definition (tsMemberKey). nil when no match carries one.
func tsMemberMarks(matches []tsengine.Match, src []byte, rules *tsDialectRules) map[tsMemberKey]tsMarked {
	norm := rules.nameNormalizer()
	var marks map[tsMemberKey]tsMarked
	for _, m := range matches {
		mark := tsMemberNone
		var def, name *tsengine.Node
		suffix := ""
		for i := range m.Captures {
			c := &m.Captures[i]
			switch {
			case c.Name == tsMemberCapture:
				mark = tsMemberScoped
			case c.Name == tsMemberStrictCapture:
				mark = tsMemberStrict
			case c.Name == tsMemberSelfCapture:
				mark = tsMemberSelf
			case c.Name == "name":
				n := c.Node
				name = &n
			case strings.HasPrefix(c.Name, "definition."):
				n := c.Node
				def = &n
				suffix = strings.TrimPrefix(c.Name, "definition.")
			}
		}
		dk, ok := rules.kinds[suffix]
		if mark == tsMemberNone || def == nil || name == nil || !ok {
			continue
		}
		text := name.Text(src)
		if norm != nil {
			text = norm(text)
		}
		if marks == nil {
			marks = map[tsMemberKey]tsMarked{}
		}
		k := tsMemberKey{def.StartByte, def.EndByte, text, string(dk.Kind)}
		marks[k] = tsMarked{mark: mark, count: marks[k].count + 1}
	}
	return marks
}

// scopeTSMembers applies the member marks to defs (see the file comment) and
// returns the definitions kept, in their original order. defs is returned
// unchanged when matches carry no member mark.
func scopeTSMembers(matches []tsengine.Match, defs []*tsDef, src []byte, rules *tsDialectRules) []*tsDef {
	if !rules.marksMembers { // compileDialectRules: every other dialect skips the pass outright
		return defs
	}
	marks := tsMemberMarks(matches, src, rules)
	if len(marks) == 0 {
		return defs
	}
	// One definition per key: matches of two patterns on one node and name
	// (a general marked pattern overlapping an older unmarked one) build the
	// same definition twice. The first is kept; it is unmarked when any of
	// the key's matches was (the older pattern's naming wins), marked when
	// every one was. The others are dropped.
	markOf := make([]tsMemberMark, len(defs))
	dup := make([]bool, len(defs))
	seen := map[tsMemberKey]bool{}
	perKey := map[tsMemberKey]int{}
	for _, d := range defs {
		perKey[tsMemberKeyOf(d)]++
	}
	for i, d := range defs {
		k := tsMemberKeyOf(d)
		m := marks[k]
		if m.count == 0 {
			continue // no marked match built it: untouched
		}
		if seen[k] {
			dup[i] = true
			continue
		}
		seen[k] = true
		if m.count >= perKey[k] {
			markOf[i] = m.mark
		}
	}
	// nearest returns the index of i's nearest ancestor that is a container
	// (or, when stopAtCallable, a callable), -1 when none.
	nearest := func(i int, stopAtCallable bool) int {
		for p := defs[i].parent; p >= 0; p = defs[p].parent {
			if defs[p].container || (stopAtCallable && defs[p].callable) {
				return p
			}
		}
		return -1
	}

	type ownedName struct {
		owner int
		name  string
	}
	// taken: the (owner, name) pairs the unmarked definitions already hold —
	// a method, a nested class, an attribute an older pattern names.
	taken := map[ownedName]bool{}
	for i, d := range defs {
		if markOf[i] != tsMemberNone || dup[i] {
			continue
		}
		if p := nearest(i, true); p >= 0 && defs[p].container {
			taken[ownedName{p, d.name}] = true
		}
	}

	keep := make([]bool, len(defs))
	owner := make([]int, len(defs))
	var members []int
	for i := range defs {
		owner[i] = -1
		if dup[i] {
			continue
		}
		switch markOf[i] {
		case tsMemberNone:
			keep[i] = true
			continue
		case tsMemberSelf:
			owner[i] = nearest(i, false)
		default:
			owner[i] = nearest(i, true)
			if owner[i] >= 0 && !defs[owner[i]].container {
				continue // a local: its nearest scope is a callable
			}
		}
		if owner[i] < 0 {
			keep[i] = markOf[i] == tsMemberScoped // module scope
			continue
		}
		members = append(members, i)
	}
	// One member per (owner, name): the first in source order.
	sort.SliceStable(members, func(a, b int) bool {
		da, db := defs[members[a]], defs[members[b]]
		if da.line != db.line {
			return da.line < db.line
		}
		return da.start < db.start
	})
	for _, i := range members {
		k := ownedName{owner[i], defs[i].name}
		if taken[k] {
			continue
		}
		taken[k] = true
		keep[i] = true
		defs[i].qualName = defs[owner[i]].qualName + rules.methodSep + defs[i].name
	}

	out := defs[:0:0]
	for i, d := range defs {
		if keep[i] {
			out = append(out, d)
		}
	}
	return out
}
