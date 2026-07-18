package evidencecitation

import "strings"

// Citation is one citation reference extracted from text by Parser.Parse.
//
// CUE mirror: #Citation in schema.cue.
type Citation struct {
	// Tag is the tag word of a "tag:id" ref (e.g. "span" in "[span:abc]").
	// Empty for bare refs (e.g. each ref in Cite output "[span-a; span-b]").
	Tag string

	// ID is the reference identifier. Never empty. May contain any
	// character except '[', ']', ';', '\n' and '\r' — including interior
	// colons, spaces, and non-ASCII runes.
	ID string

	// Start and End are byte offsets into the parsed text such that
	// text[Start:End] == Raw. They cover the ref only (e.g. "span:abc"),
	// not the surrounding brackets or separators, so callers can highlight
	// the exact reference.
	Start int
	End   int

	// Raw is the ref exactly as it appeared in the text, equal to
	// text[Start:End].
	Raw string
}

// Ref reconstructs the ref string this citation was parsed from:
// "tag:id" when Tag is set, otherwise just ID. For any text produced by
// Cite, Ref returns the original ref (see the round-trip note on Parse).
func (c Citation) Ref() string {
	if c.Tag == "" {
		return c.ID
	}
	return c.Tag + ":" + c.ID
}

// Parser extracts citations from text. The tag vocabulary is caller data —
// the kit hard-codes no tag names.
//
// A Parser is immutable after construction and safe for concurrent use.
type Parser struct {
	// tags is the accepted tag vocabulary; empty means unrestricted.
	tags map[string]struct{}
}

// NewParser constructs a Parser that accepts the given tag names
// (case-sensitive, exact match).
//
//   - With one or more tag names (e.g. NewParser("span", "authority")),
//     only "tag:id" refs whose tag is in the set are extracted; bare refs
//     and refs with other tags are ignored.
//   - With no tag names, the parser is unrestricted: it extracts every ref
//     in every well-formed block — "tag:id" refs with any tag word, and
//     bare refs (Tag == ""). Only the unrestricted parser can recover
//     Cite output, whose refs carry no tag of their own.
//
// Empty-string tag names are ignored; if none remain the parser is
// unrestricted. A tag name that is not a tag word (see Parse) can never
// match and is effectively inert.
func NewParser(tagNames ...string) *Parser {
	var tags map[string]struct{}
	for _, name := range tagNames {
		if name == "" {
			continue
		}
		if tags == nil {
			tags = make(map[string]struct{}, len(tagNames))
		}
		tags[name] = struct{}{}
	}
	return &Parser{tags: tags}
}

// Parse extracts citations from text, in document order (ascending byte
// offset). It returns nil when text contains no recognizable citations.
//
// Grammar (covers both the "[tag:id]" form models are prompted to emit and
// the "[ref1; ref2]" form Cite renders):
//
//	block   = "[" ref *( ";" ref ) "]"
//	ref     = *SP ( tagged / bare ) *SP
//	tagged  = tag ":" id
//	tag     = ALPHA *( ALPHA / DIGIT / "_" / "-" )   ; ASCII only
//	id,bare = 1*<any byte except "[" "]" ";" LF CR>  ; no leading/trailing SP
//
// A block is well-formed only if every ref segment is non-empty after
// trimming ASCII spaces; a block containing '[', a newline, or an empty
// segment yields no citations (a '[' inside a candidate block restarts
// scanning at that inner bracket, so "[junk [span:a]" still finds span:a).
// A ref whose text before the first colon is not a tag word, or whose text
// after it is empty, is a bare ref ("x:" and ":x" are bare, "1:2" is bare,
// "span:urn:x" is tag "span" with ID "urn:x").
//
// Round trip: for an unrestricted parser, Parse(Cite(refs)) recovers
// exactly the deduplicated, lexicographically sorted refs (via
// Citation.Ref) for every ref satisfying Parseable. Refs Cite accepts but
// Parseable rejects (containing '[', ']', ';', newlines, or
// leading/trailing spaces) render fine but cannot be parsed back.
//
// Honesty: parsing identifies WELL-FORMED tags only. It cannot detect
// citations the model omitted, mangled past recognition, or fabricated
// with plausible IDs — use Validate against a known-reference resolver to
// catch fabrication. An unrestricted parser also treats any bracketed
// prose ("[see above]") as a bare ref; callers needing precision should
// name their tags.
//
// Parse is pure and deterministic; it allocates only for the result.
func (p *Parser) Parse(text string) []Citation {
	var out []Citation
	for _, b := range p.blocks(text) {
		out = append(out, b.cites...)
	}
	return out
}

// Validate filters citations through the caller's resolver and returns the
// ones the resolver does not recognize, preserving document order. It
// returns nil when every citation resolves.
//
// This is the anti-fabrication seam: LLMs invent plausible-looking
// citation IDs, and a grounded-answer consumer must reject any citation
// that does not resolve to real evidence. Wire known to your evidence
// store (e.g. func(tag, id string) bool { return store.Has(id) }) and
// treat a non-empty result as a grounding failure.
//
// Fail closed: a nil resolver resolves nothing, so every citation is
// returned as unknown.
func Validate(citations []Citation, known func(tag, id string) bool) []Citation {
	if known == nil {
		if len(citations) == 0 {
			return nil
		}
		return append([]Citation(nil), citations...)
	}
	var unknown []Citation
	for _, c := range citations {
		if !known(c.Tag, c.ID) {
			unknown = append(unknown, c)
		}
	}
	return unknown
}

// Strip removes citation blocks from text so callers can show clean display
// prose. Removal is block-granular: every block that yields at least one
// citation under this parser is removed in full (including refs within it
// that the tag filter would skip); blocks that yield none — e.g. "[1]"
// footnotes under a restricted parser — are left untouched.
//
// To keep prose readable, each removed block also consumes one immediately
// preceding ASCII space when present ("signed [span:x]." → "signed.").
// A block at the very start of the text may leave a leading space behind;
// callers wanting tighter control should use Replace.
//
// Strip never re-scans its own output: on adversarial input, removing a
// block can juxtapose bytes that themselves form a new well-formed block,
// and that new block survives. Strip is offset-safe — it cuts only at
// block boundaries, so it never splits a multi-byte rune.
func (p *Parser) Strip(text string) string {
	blocks := p.blocks(text)
	if len(blocks) == 0 {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	prev := 0
	for _, blk := range blocks {
		start := blk.start
		if start > prev && text[start-1] == ' ' {
			start--
		}
		b.WriteString(text[prev:start])
		prev = blk.end
	}
	b.WriteString(text[prev:])
	return b.String()
}

// Replace rewrites each citation block that yields at least one citation
// under this parser, substituting repl's return value verbatim (so callers
// can render their own anchors). raw is the full block including brackets
// (text[start:end]); cites are that block's citations with offsets into the
// original text. Blocks yielding no citations are left untouched, and no
// whitespace tidying is applied. A nil repl returns text unchanged.
//
// Replace is offset-safe: with repl returning raw unchanged, the output is
// byte-identical to the input.
func (p *Parser) Replace(text string, repl func(raw string, cites []Citation) string) string {
	if repl == nil {
		return text
	}
	blocks := p.blocks(text)
	if len(blocks) == 0 {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	prev := 0
	for _, blk := range blocks {
		b.WriteString(text[prev:blk.start])
		b.WriteString(repl(text[blk.start:blk.end], blk.cites))
		prev = blk.end
	}
	b.WriteString(text[prev:])
	return b.String()
}

// Parseable reports whether a ref rendered by Cite can be recovered exactly
// by an unrestricted Parser (the round-trip precondition documented on
// Parse): non-empty, no '[' ']' ';' or newline bytes, and no leading or
// trailing ASCII space.
func Parseable(ref string) bool {
	if ref == "" {
		return false
	}
	if ref[0] == ' ' || ref[len(ref)-1] == ' ' {
		return false
	}
	return !strings.ContainsAny(ref, "[];\n\r")
}

// block is one well-formed citation block: text[start:end] including the
// brackets, plus the citations it yielded under the parser's tag filter.
// Blocks with zero accepted citations are not recorded.
type block struct {
	start, end int
	cites      []Citation
}

// blocks scans text once, left to right, and returns every well-formed
// block that yields at least one citation, in document order. The scan is
// O(len(text)): each candidate '[' either closes, aborts at an inner '['
// (where scanning resumes), or aborts at a newline/end of text.
func (p *Parser) blocks(text string) []block {
	var out []block
	for i := 0; i < len(text); {
		rel := strings.IndexByte(text[i:], '[')
		if rel < 0 {
			break
		}
		open := i + rel
		next, body, ok := scanBody(text, open)
		if ok {
			if cites, valid := p.parseBody(body, open+1); valid && len(cites) > 0 {
				out = append(out, block{start: open, end: next, cites: cites})
			}
		}
		i = next
	}
	return out
}

// scanBody scans for the ']' closing the '[' at open. It returns the body
// between the brackets and the offset scanning should resume at. ok is
// false when the candidate aborts: at an inner '[' (resume there), at a
// newline (blocks never span lines), or at end of text (unclosed).
func scanBody(text string, open int) (next int, body string, ok bool) {
	for j := open + 1; j < len(text); j++ {
		switch text[j] {
		case ']':
			return j + 1, text[open+1 : j], true
		case '[':
			return j, "", false
		case '\n', '\r':
			return j + 1, "", false
		}
	}
	return len(text), "", false
}

// parseBody splits a block body into ';'-separated ref segments. base is
// the absolute byte offset of the body within the parsed text. valid is
// false when any segment is empty after trimming ASCII spaces, in which
// case the whole block yields nothing. Refs the tag filter skips are
// dropped without invalidating the block.
func (p *Parser) parseBody(body string, base int) (cites []Citation, valid bool) {
	if body == "" {
		return nil, false
	}
	segStart := 0
	for k := 0; k <= len(body); k++ {
		if k != len(body) && body[k] != ';' {
			continue
		}
		// Trim ASCII spaces while tracking offsets.
		ls, le := segStart, k
		for ls < le && body[ls] == ' ' {
			ls++
		}
		for le > ls && body[le-1] == ' ' {
			le--
		}
		if ls == le {
			return nil, false // empty segment: malformed block
		}
		if c, accepted := p.classify(body[ls:le], base+ls, base+le); accepted {
			cites = append(cites, c)
		}
		segStart = k + 1
	}
	return cites, true
}

// classify splits one trimmed ref into (tag, id) and applies the parser's
// tag filter. start/end are absolute byte offsets of ref in the parsed
// text.
func (p *Parser) classify(ref string, start, end int) (Citation, bool) {
	tag, id := splitRef(ref)
	if len(p.tags) > 0 {
		if tag == "" {
			return Citation{}, false
		}
		if _, ok := p.tags[tag]; !ok {
			return Citation{}, false
		}
	}
	return Citation{Tag: tag, ID: id, Start: start, End: end, Raw: ref}, true
}

// splitRef splits ref at its first colon into a tag word and an ID. When
// there is no colon, the text before it is not a tag word, or the ID part
// is empty, the entire ref is a bare ID.
func splitRef(ref string) (tag, id string) {
	c := strings.IndexByte(ref, ':')
	if c <= 0 || c == len(ref)-1 {
		return "", ref
	}
	if !isTagWord(ref[:c]) {
		return "", ref
	}
	return ref[:c], ref[c+1:]
}

// isTagWord reports whether s matches the tag grammar: an ASCII letter
// followed by ASCII letters, digits, '_' or '-'.
func isTagWord(s string) bool {
	if s == "" || !isASCIILetter(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		b := s[i]
		if !isASCIILetter(b) && (b < '0' || b > '9') && b != '_' && b != '-' {
			return false
		}
	}
	return true
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
