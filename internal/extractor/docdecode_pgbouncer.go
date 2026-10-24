package extractor

import (
	"bytes"
	"strings"
)

// The doc engine's "pgbouncer" decode (LB-Q-pgbouncer; pgbouncer.ini). The root
// is a map from section name to a map of that section's entries, in file order:
//
//	[databases] [users] [peers]   an entry is "name = key=value key='quoted value' ...",
//	                              decoded as a map of the connect string's parameters
//	every other section           ([pgbouncer]) an entry is "setting = value", a scalar
//
// An entry's line is where it is written; a section's line is its header's. A
// comment opens with ";" or "#" at the head of a line. A header written twice
// extends the section its first occurrence made; a line before the first header
// belongs to no section and is dropped, as is a "%include" directive (the
// file it names is not read here). An entry key may be double-quoted. A
// parameter value is bare (up to a space) or single-quoted, with ” for a quote,
// and "=" may have spaces around it. Nothing here fails: any bytes decode.
func init() { docDecoders["pgbouncer"] = true }

// pgbouncerParamSections are the sections whose entries are connect strings.
var pgbouncerParamSections = map[string]bool{"databases": true, "users": true, "peers": true}

func decodePgBouncerDoc(src []byte) (*docNode, error) {
	src = bytes.TrimPrefix(src, []byte("\xef\xbb\xbf"))
	root := &docNode{kind: docMap, line: 1}
	var sec *docNode
	params := false
	for i, raw := range strings.Split(string(src), "\n") {
		lineNo := i + 1
		text := strings.TrimSpace(raw)
		if text == "" || text[0] == '#' || text[0] == ';' || strings.HasPrefix(text, "%include") {
			continue
		}
		if text[0] == '[' && strings.HasSuffix(text, "]") {
			name := strings.TrimSpace(text[1 : len(text)-1])
			if existing := root.get(name); existing != nil {
				sec = existing
			} else {
				sec = &docNode{kind: docMap, line: lineNo}
				root.set(name, sec)
			}
			params = pgbouncerParamSections[name]
			continue
		}
		if sec == nil {
			continue
		}
		key, value, ok := cutPgBouncerEntry(text)
		if !ok {
			continue
		}
		if !params {
			sec.set(key, &docNode{kind: docScalar, scalar: value, line: lineNo})
			continue
		}
		entry := &docNode{kind: docMap, line: lineNo}
		for _, p := range parsePgBouncerParams(value) {
			entry.set(p[0], &docNode{kind: docScalar, scalar: p[1], line: lineNo})
		}
		sec.set(key, entry)
	}
	return root, nil
}

// cutPgBouncerEntry splits "key = value" at the first "=" outside a
// double-quoted key; the key loses its quotes, both sides are trimmed.
func cutPgBouncerEntry(text string) (key, value string, ok bool) {
	if text[0] == '"' {
		if j := strings.IndexByte(text[1:], '"'); j >= 0 {
			rest := strings.TrimSpace(text[j+2:])
			if strings.HasPrefix(rest, "=") {
				return text[1 : j+1], strings.TrimSpace(rest[1:]), true
			}
			return "", "", false
		}
	}
	k, v, found := strings.Cut(text, "=")
	k = strings.TrimSpace(k)
	if !found || k == "" {
		return "", "", false
	}
	return k, strings.TrimSpace(v), true
}

// parsePgBouncerParams reads a connect string's key=value pairs in order.
func parsePgBouncerParams(s string) [][2]string {
	var out [][2]string
	i := 0
	skip := func() {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
	}
	for {
		skip()
		start := i
		for i < len(s) && s[i] != '=' && s[i] != ' ' && s[i] != '\t' {
			i++
		}
		key := s[start:i]
		skip()
		if key == "" || i >= len(s) || s[i] != '=' {
			if i >= len(s) {
				return out
			}
			continue // a word with no "=": skipped
		}
		i++
		skip()
		var val strings.Builder
		if i < len(s) && s[i] == '\'' {
			i++
			for i < len(s) {
				if s[i] == '\'' {
					if i+1 < len(s) && s[i+1] == '\'' {
						val.WriteByte('\'')
						i += 2
						continue
					}
					i++
					break
				}
				val.WriteByte(s[i])
				i++
			}
		} else {
			for i < len(s) && s[i] != ' ' && s[i] != '\t' {
				val.WriteByte(s[i])
				i++
			}
		}
		out = append(out, [2]string{key, val.String()})
	}
}
