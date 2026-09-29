// Package explmap is the exploration-map model (docs/design/exploration-maps.md
// section 2, ADR-056, vision 40): a map holds its own anchored entries and
// `includes` references to other maps, never copies of their entries. The
// references form a DAG (shared bases allowed, cycles refused, depth capped at
// MaxDepth); a handoff carries ONE id, a minted composite made only of
// references; resolution is a union deduplicated by anchor with every
// conflicting annotation kept with its provenance; every entry is re-resolved at
// read time against the content hash it was filed with; and lifetimes follow
// references.
//
// The package is deterministic and does no I/O: storage is the caller's (thread
// fields in the existing threads store, no new table or cache — R5, ADR-049
// decision 2), source text reaches it only through the Source interface, and
// clocks are the caller's timestamps. It needs no model and no network (provider
// none is the whole design).
package explmap

import (
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// MaxDepth caps the include chain below any map: a root including a base four
// levels down is allowed, a fifth level is refused, so an expansion is bounded
// however the maps were composed (design section 2).
const MaxDepth = 4

// CompositePrefix begins every minted composite's id. It is reserved: an
// ephemeral or named map may not take an id with it, so an id alone says
// whether a map is made only of references.
const CompositePrefix = "xm-"

// Kind is how a map lives.
type Kind string

const (
	// KindEphemeral is a map filed by a run: its own entries plus includes,
	// kept while referenced or until its thread condition holds, then idle.
	KindEphemeral Kind = "ephemeral"
	// KindComposite is a minted map made only of references (XM-3).
	KindComposite Kind = "composite"
	// KindLive is a stored query recomputed on read (XM-D1, deferred). The
	// value is reserved so the lifetime rule already treats one as alive.
	KindLive Kind = "live"
)

// Entry is one anchored entry: an anchor (a symbol key, or `path#Lx-Ly`), the
// span its content hash covers, the hash itself, and a why line.
type Entry struct {
	// ID is the storage row that holds the entry. Appending mints a fresh id,
	// so no append can ever overwrite another agent's entry (XM-14).
	ID string
	// Anchor is what the filer named: a symbol key or `path#Lx-Ly`.
	Anchor string
	// Path, Start and End are the span the hash covers at filing (1-based,
	// inclusive, repo-relative); a symbol anchor records its resolved span.
	Path       string
	Start, End int
	// Symbol is the symbol key when the anchor named one: the key a drifted
	// entry is re-resolved by (XM-5). Empty for a span anchor.
	Symbol string
	// Hash is core.HashLines over the span at filing: the same content-hash
	// contract a layer claim's citation uses (semantic-layers.md 4.3).
	Hash string
	// Why is the one line saying why this code matters for the map's purpose.
	Why string
	// AgentID is the appending sub-agent's id (provenance, XM-14); empty when
	// the map's own session filed it.
	AgentID string
	// Session is the harness session that filed it, when one was known.
	Session string
	// FiledAt is when the entry was filed.
	FiledAt time.Time
	// Commit is the repository HEAD at filing: the base a stale entry's diff
	// size is measured against.
	Commit string
}

// SpanRef is the entry's span as a citation, `path#Lx-Ly`.
func (e Entry) SpanRef() string {
	if e.Path == "" || e.Start < 1 {
		return ""
	}
	return core.CiteRef(e.Path, e.Start, e.End)
}

// Key is the dedup key of resolution (XM-4): the anchor (the symbol key when
// the entry names a symbol, else its span) plus the content hash. Two entries
// with one key name the same code in the same state, whoever filed them.
func (e Entry) Key() string {
	if e.Symbol != "" {
		return "sym:" + e.Symbol + "@" + e.Hash
	}
	return "span:" + e.SpanRef() + "@" + e.Hash
}

// IncludeDrop records an include a map stopped carrying, and when: the event that
// restarts the dropped map's idle clock (XM-6).
type IncludeDrop struct {
	ID string
	At time.Time
}

// Map is one exploration map.
type Map struct {
	ID   string
	Kind Kind
	// Name makes a map named: durable (XM-32: it outlives the condition it was
	// filed under), and never without a Purpose (XM-7).
	Name string
	// Purpose is the question the map answers (authored, one line).
	Purpose string
	// WhenToUse is the optional authored when-to-use line.
	WhenToUse string
	// Includes are the maps this one references, in priority order: at equal
	// depth an earlier include is nearer.
	Includes []string
	// Entries are the map's own entries, in filing order. A composite has none.
	Entries []Entry
	// CreatedBy is the creating agent id or session.
	CreatedBy string
	CreatedAt time.Time
	// LastUsed is the latest write to the map (an append, a header change).
	LastUsed time.Time
	// EndedAt is when the map's own thread condition held or it was closed;
	// zero while it has not ended. A named map records it and is still not
	// Ended (see Ended).
	EndedAt time.Time
	// Drops are the includes this map stopped carrying.
	Drops []IncludeDrop
}

// Named reports whether the map carries a name.
func (m Map) Named() bool { return strings.TrimSpace(m.Name) != "" }

// Ended reports whether the map's own condition has ended it. A named map is
// never ended (XM-32, O143): naming is what makes a map durable, so the
// condition a run filed it under (the end of that session) does not end it,
// whether the name came before the condition held or after.
func (m Map) Ended() bool { return !m.EndedAt.IsZero() && !m.Named() }

// Lookup finds a map by id.
type Lookup interface {
	Map(id string) (Map, bool)
}

// Maps is the whole set of known maps by id: the Lookup a caller builds from
// its storage.
type Maps map[string]Map

// Map implements Lookup.
func (s Maps) Map(id string) (Map, bool) {
	m, ok := s[id]
	return m, ok
}

// IDs returns the set's ids, sorted.
func (s Maps) IDs() []string {
	out := make([]string, 0, len(s))
	for id := range s {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// overlay is a Lookup that sees one extra map over a base set, so a proposed
// write (a new include, a minted composite) is checked before it is stored.
type overlay struct {
	base  Lookup
	extra Map
}

func (o overlay) Map(id string) (Map, bool) {
	if id == o.extra.ID {
		return o.extra, true
	}
	return o.base.Map(id)
}

// With returns a Lookup that sees m over base (replacing a map of the same id),
// for checking a proposed change before it is written.
func With(base Lookup, m Map) Lookup { return overlay{base: base, extra: m} }

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// ValidID reports whether id is a well-formed map id: a letter or digit, then
// up to 63 letters, digits, dots, underscores or hyphens.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// ParseAnchor splits an anchor into a span (`path#Lx-Ly`, ok=true and span set)
// or a symbol key (ok=true and symbol set). ok is false for an empty anchor or a
// malformed span.
func ParseAnchor(anchor string) (path string, start, end int, symbol string, ok bool) {
	a := strings.TrimSpace(anchor)
	if a == "" {
		return "", 0, 0, "", false
	}
	if strings.Contains(a, "#L") {
		p, s, e, ok := core.ParseCiteRef(a)
		if !ok {
			return "", 0, 0, "", false
		}
		return p, s, e, "", true
	}
	if strings.ContainsAny(a, " \t\n") {
		return "", 0, 0, "", false
	}
	return "", 0, 0, a, true
}

// HashText is the content hash of a span's text, computed by core.HashLines
// over the text as a whole: the one hash contract layer claims use, so a map
// entry and a claim citation over the same lines agree.
func HashText(text string) string {
	n := strings.Count(strings.ReplaceAll(text, "\r\n", "\n"), "\n") + 1
	h, _ := core.HashLines([]byte(text), 1, n)
	return h
}
