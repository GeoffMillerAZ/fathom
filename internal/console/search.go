package console

// search.go — the command palette's index (design §2, OC-11). One search
// over projects, plans and plan rows, symbols (from the store), rules and
// records (intent invariants, decision records), verbs (the CLI registry)
// and pages (the console registry), grouped in a fixed order and ranked
// deterministically. GET /search renders it as a full page and as an HTML
// fragment for the palette; both come from this one function.

import (
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

// SearchGroupOrder is the fixed group order (design §2).
var SearchGroupOrder = []SearchGroup{GroupProjects, GroupPlans, GroupSymbols, GroupRecords, GroupVerbs, GroupPages}

// SearchDoc is one searchable thing. Href is set only when a registered
// route renders it; Command is the verb that shows it; Ref is file:line.
type SearchDoc struct {
	Group    SearchGroup `json:"group"`
	Kind     string      `json:"kind"`
	Title    string      `json:"title"`
	Subtitle string      `json:"subtitle,omitempty"`
	Keywords []string    `json:"keywords,omitempty"`
	Service  string      `json:"service,omitempty"`
	Href     string      `json:"href,omitempty"`
	Command  string      `json:"command,omitempty"`
	Ref      string      `json:"ref,omitempty"`
}

// SymbolLookup answers the Symbols group from the store. It returns at most
// limit candidates whose names contain the query; Search ranks them.
type SymbolLookup func(query string, limit int) ([]SearchDoc, error)

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
	// Total is how many docs in the group matched before the per-group cap.
	Total int    `json:"total"`
	Note  string `json:"note,omitempty"`
}

// SearchResults is Search's answer.
type SearchResults struct {
	Query  string              `json:"query"`
	Groups []SearchGroupResult `json:"groups"`
	Total  int                 `json:"total"`
	Notes  []string            `json:"notes,omitempty"`
}

// DefaultPerGroup caps each group.
const DefaultPerGroup = 8

// Search ranks every doc against q and groups the matches. Every query
// token must match the doc somewhere; the score favors an exact title, then
// a title prefix, then a word start, then a substring, then the subtitle or
// keywords, then an in-order fuzzy match. Ties break on the shorter title,
// then the title, then the href, so the order never depends on map order.
func Search(ix SearchIndex, q string, perGroup int) SearchResults {
	if perGroup <= 0 {
		perGroup = DefaultPerGroup
	}
	out := SearchResults{Query: q, Notes: append([]string(nil), ix.Notes...)}
	tokens := searchTokens(q)
	if len(tokens) == 0 {
		return out
	}
	byGroup := map[SearchGroup][]SearchResult{}
	notes := map[SearchGroup]string{}
	for _, d := range ix.Docs {
		if s := scoreDoc(d, tokens); s > 0 {
			byGroup[d.Group] = append(byGroup[d.Group], SearchResult{SearchDoc: d, Score: s})
		}
	}
	if ix.Symbols != nil {
		syms, err := ix.Symbols(strings.TrimSpace(q), perGroup*8)
		if err != nil {
			notes[GroupSymbols] = "symbols could not be searched: " + err.Error()
		}
		for _, d := range syms {
			d.Group = GroupSymbols
			if s := scoreDoc(d, tokens); s > 0 {
				byGroup[GroupSymbols] = append(byGroup[GroupSymbols], SearchResult{SearchDoc: d, Score: s})
			}
		}
	}
	for _, g := range SearchGroupOrder {
		rs := byGroup[g]
		if len(rs) == 0 && notes[g] == "" {
			continue
		}
		sort.SliceStable(rs, func(i, j int) bool {
			a, b := rs[i], rs[j]
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
		})
		gr := SearchGroupResult{Group: g, Total: len(rs), Note: notes[g]}
		if len(rs) > perGroup {
			rs = rs[:perGroup]
		}
		gr.Results = rs
		out.Total += gr.Total
		out.Groups = append(out.Groups, gr)
	}
	return out
}

func searchTokens(q string) []string {
	return strings.FieldsFunc(strings.ToLower(q), func(r rune) bool { return unicode.IsSpace(r) })
}

// scoreDoc is 0 when any token misses the doc entirely.
func scoreDoc(d SearchDoc, tokens []string) int {
	title := strings.ToLower(d.Title)
	rest := strings.ToLower(d.Subtitle + " " + strings.Join(d.Keywords, " ") + " " + d.Service + " " + d.Ref + " " + d.Command)
	total := 0
	for _, t := range tokens {
		s := scoreToken(title, rest, t)
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

func scoreToken(title, rest, t string) int {
	switch {
	case title == t:
		return 100
	case strings.HasPrefix(title, t):
		return 80
	case wordStart(title, t):
		return 60
	case strings.Contains(title, t):
		return 40
	case strings.Contains(rest, t):
		return 20
	case subsequence(title, t):
		return 10
	}
	return 0
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

// subsequence reports whether every rune of t appears in s in order.
func subsequence(s, t string) bool {
	if len(t) < 3 {
		return false // two letters match almost anything in order
	}
	j := 0
	tr := []rune(t)
	for _, r := range s {
		if j < len(tr) && r == tr[j] {
			j++
		}
	}
	return j == len(tr)
}
