package console

// search.go — the command palette's index (design §2, OC-11). One search
// over projects, plans and plan rows, symbols (from the store), rules and
// records (intent invariants, decision records), verbs (the CLI registry)
// and pages (the console registry), grouped in a fixed order and ranked
// deterministically. GET /search renders it as a full page and as an HTML
// fragment for the palette; both come from this one function.

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// SearchGroup is one result group, rendered in SearchGroupOrder.
type SearchGroup string

const (
	GroupProjects SearchGroup = "Projects"
	GroupPlans    SearchGroup = "Plans and rows"
	GroupSymbols  SearchGroup = "Symbols"
	GroupRecords  SearchGroup = "Rules and records"
	GroupVerbs    SearchGroup = "Verbs"
	GroupPages    SearchGroup = "Pages"
)

// SearchGroupOrder is the groups' tie-break order (design §2): groups are
// listed by their best match, a tie goes to the smaller group, and groups
// still tied keep this order — so an exact hit is never rendered below a
// group of weaker ones, nor a lone exact page below a crowd of exact names.
var SearchGroupOrder = []SearchGroup{GroupProjects, GroupPlans, GroupSymbols, GroupRecords, GroupVerbs, GroupPages}

// Query bounds. Every token is scored against every document, and the page
// echoes the query, so an unbounded query is unbounded work on the daemon
// every agent shares (the survey: a 200 KB query pinned a core for 25 s).
const (
	MaxQueryRunes  = 200
	MaxQueryTokens = 8
)

// CheckQuery refuses a query past the bounds, naming the bound it crossed.
func CheckQuery(q string) error {
	if n := len([]rune(q)); n > MaxQueryRunes {
		return fmt.Errorf("the query is %d characters; search takes at most %d", n, MaxQueryRunes)
	}
	if n := len(strings.Fields(q)); n > MaxQueryTokens {
		return fmt.Errorf("the query has %d words; search takes at most %d", n, MaxQueryTokens)
	}
	return nil
}

// SearchDoc is one searchable thing. Href is set only when a registered
// route renders it; Command is the verb that shows it; Ref is file:line; ID
// is a row's or record's own identifier (OC-11, ADR-045, I31), matched
// exactly.
type SearchDoc struct {
	Group    SearchGroup `json:"group"`
	Kind     string      `json:"kind"`
	ID       string      `json:"id,omitempty"`
	Title    string      `json:"title"`
	Subtitle string      `json:"subtitle,omitempty"`
	Keywords []string    `json:"keywords,omitempty"`
	Service  string      `json:"service,omitempty"`
	Href     string      `json:"href,omitempty"`
	Command  string      `json:"command,omitempty"`
	Ref      string      `json:"ref,omitempty"`
}

// SymbolLookup answers the Symbols group from the store: the best limit
// matches for the lowered query tokens — every token matching, scored with
// ScoreFields and ordered by RankLess, exactly as Search ranks any group —
// and how many symbols matched in all (never capped: the group's total is
// that count).
type SymbolLookup func(tokens []string, limit int) (results []SearchResult, total int, err error)

// SearchIndex is everything Search reads.
type SearchIndex struct {
	Docs    []SearchDoc
	Symbols SymbolLookup
	// Notes are index-level caveats rendered above the results ("plans of 3
	// services could not be read").
	Notes []string
}

// SearchResult is a ranked doc.
type SearchResult struct {
	SearchDoc
	Score int `json:"score"`
}

// SearchGroupResult is one group's ranked results.
type SearchGroupResult struct {
	Group   SearchGroup    `json:"group"`
	Results []SearchResult `json:"results"`
	// Total is how many docs in the group matched, before the per-group cap.
	Total int    `json:"total"`
	Note  string `json:"note,omitempty"`
}

// SearchResults is Search's answer. Group and Offset are set when the
// search was scoped to one group's page (SearchWith).
type SearchResults struct {
	Query    string              `json:"query"`
	Groups   []SearchGroupResult `json:"groups"`
	Total    int                 `json:"total"`
	Notes    []string            `json:"notes,omitempty"`
	Group    SearchGroup         `json:"group,omitempty"`
	Offset   int                 `json:"offset,omitempty"`
	PerGroup int                 `json:"per_group"`
}

// DefaultPerGroup caps each group.
const DefaultPerGroup = 8

// SearchOptions scopes a search: PerGroup caps each group (default
// DefaultPerGroup); Group, when set, answers only that group, from Offset.
type SearchOptions struct {
	PerGroup int
	Group    SearchGroup
	Offset   int
}

// Search ranks every doc against q and groups the matches. Every query
// token must match the doc somewhere; the score favors an exact title or
// ID, then a title prefix, then a word start, then a substring, then the
// subtitle or keywords, then a compact in-order fuzzy match. Ties break on
// the shorter title, then the title, then the href, so the order never
// depends on map order.
func Search(ix SearchIndex, q string, perGroup int) SearchResults {
	return SearchWith(ix, q, SearchOptions{PerGroup: perGroup})
}

// SearchWith is Search with a scope (one group, paged by Offset).
func SearchWith(ix SearchIndex, q string, o SearchOptions) SearchResults {
	perGroup := o.PerGroup
	if perGroup <= 0 {
		perGroup = DefaultPerGroup
	}
	offset := o.Offset
	if offset < 0 || o.Group == "" {
		offset = 0
	}
	out := SearchResults{Query: q, Notes: append([]string(nil), ix.Notes...), Group: o.Group, Offset: offset, PerGroup: perGroup}
	tokens := searchTokens(q)
	if len(tokens) == 0 {
		return out
	}
	wants := func(g SearchGroup) bool { return o.Group == "" || o.Group == g }
	byGroup := map[SearchGroup][]SearchResult{}
	totals := map[SearchGroup]int{}
	notes := map[SearchGroup]string{}
	for _, d := range ix.Docs {
		if !wants(d.Group) {
			continue
		}
		if s := ScoreDoc(d, tokens); s > 0 {
			byGroup[d.Group] = append(byGroup[d.Group], SearchResult{SearchDoc: d, Score: s})
		}
	}
	for g, rs := range byGroup {
		totals[g] = len(rs)
	}
	if ix.Symbols != nil && wants(GroupSymbols) {
		syms, total, err := ix.Symbols(tokens, offset+perGroup)
		if err != nil {
			notes[GroupSymbols] = "symbols could not be searched: " + err.Error()
		}
		for _, r := range syms {
			r.Group = GroupSymbols
			byGroup[GroupSymbols] = append(byGroup[GroupSymbols], r)
		}
		totals[GroupSymbols] = total
	}
	var groups []SearchGroupResult
	for _, g := range SearchGroupOrder {
		rs := byGroup[g]
		if len(rs) == 0 && notes[g] == "" && totals[g] == 0 {
			continue
		}
		sort.SliceStable(rs, func(i, j int) bool { return RankLess(rs[i], rs[j]) })
		gr := SearchGroupResult{Group: g, Total: totals[g], Note: notes[g]}
		if gr.Total < len(rs) {
			gr.Total = len(rs)
		}
		if offset >= len(rs) {
			rs = nil
		} else {
			rs = rs[offset:]
		}
		if len(rs) > perGroup {
			rs = rs[:perGroup]
		}
		gr.Results = rs
		out.Total += gr.Total
		groups = append(groups, gr)
	}
	// Best match first; on a tie the smaller — more specific — group leads
	// (the one Fleet page before 273 symbols that also match "fleet"
	// exactly somewhere), then the fixed order (stable sort).
	best := func(g SearchGroupResult) int {
		if len(g.Results) == 0 {
			return 0
		}
		return g.Results[0].Score
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if bi, bj := best(groups[i]), best(groups[j]); bi != bj {
			return bi > bj
		}
		return groups[i].Total < groups[j].Total
	})
	out.Groups = groups
	return out
}

// RankLess is the one result order every group shares: score, then the
// shorter title, then the title, the href and the ref.
func RankLess(a, b SearchResult) bool {
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	if len(a.Title) != len(b.Title) {
		return len(a.Title) < len(b.Title)
	}
	if a.Title != b.Title {
		return a.Title < b.Title
	}
	if a.Href != b.Href {
		return a.Href < b.Href
	}
	return a.Ref < b.Ref
}

// searchTokens lowers and splits q, keeping at most MaxQueryTokens.
func searchTokens(q string) []string {
	tokens := strings.FieldsFunc(strings.ToLower(q), func(r rune) bool { return unicode.IsSpace(r) })
	if len(tokens) > MaxQueryTokens {
		tokens = tokens[:MaxQueryTokens]
	}
	return tokens
}

// ScoreDoc scores d against lowered tokens; 0 when any token misses it.
func ScoreDoc(d SearchDoc, tokens []string) int {
	rest := []string{strings.ToLower(d.Subtitle), strings.ToLower(d.Service), strings.ToLower(d.Ref), strings.ToLower(d.Command)}
	for _, k := range d.Keywords {
		rest = append(rest, strings.ToLower(k))
	}
	return ScoreFields(strings.ToLower(d.Title), strings.ToLower(d.ID), rest, tokens)
}

// ScoreFields scores already-lowered fields: the title, the doc's own ID,
// and the rest (subtitle, service, ref, command, keywords), each searched on
// its own. 0 when any token misses every field.
func ScoreFields(title, id string, rest []string, tokens []string) int {
	total := 0
	for _, t := range tokens {
		s := scoreToken(title, id, rest, t)
		if s == 0 {
			return 0
		}
		total += s
	}
	if len(tokens) > 1 && strings.Contains(title, strings.Join(tokens, " ")) {
		total += 50 // the whole phrase, in order, in the title
	}
	return total
}

func scoreToken(title, id string, rest []string, t string) int {
	switch {
	case title == t || (id != "" && id == t):
		return 100
	case strings.HasPrefix(title, t):
		return 80
	case wordStart(title, t):
		return 60
	case strings.Contains(title, t):
		return 40
	case anyContains(rest, t):
		return 20
	case compactSubsequence(title, t):
		return 10
	}
	return 0
}

func anyContains(fields []string, t string) bool {
	for _, f := range fields {
		if strings.Contains(f, t) {
			return true
		}
	}
	return false
}

// wordStart reports whether t begins a word inside s (after a separator or
// at a camelCase boundary the lower-casing already flattened, so separators
// only: space, punctuation, path and package delimiters).
func wordStart(s, t string) bool {
	for i := 1; i < len(s); i++ {
		c := s[i-1]
		if c == ' ' || c == '/' || c == '.' || c == '_' || c == '-' || c == ':' || c == '(' || c == '*' {
			if strings.HasPrefix(s[i:], t) {
				return true
			}
		}
	}
	return false
}

// compactSubsequence reports whether every rune of t appears in s in order
// within a span of at most twice t's length — a typo or an abbreviation
// ("flet" for "fleet", "rslvlog"), never letters scattered across a
// sentence, which matched 1,314 plan rows for "fleet" and buried the exact
// page under them.
func compactSubsequence(s, t string) bool {
	tr := []rune(t)
	if len(tr) < 3 {
		return false // two letters match almost anything in order
	}
	sr := []rune(s)
	maxSpan := 2 * len(tr)
	for start := range sr {
		if sr[start] != tr[0] {
			continue
		}
		j := 1
		end := start
		for i := start + 1; i < len(sr) && j < len(tr) && i-start < maxSpan; i++ {
			if sr[i] == tr[j] {
				j++
				end = i
			}
		}
		if j == len(tr) && end-start < maxSpan {
			return true
		}
	}
	return false
}
