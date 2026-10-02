package extractor

import (
	"bytes"
	"strings"
)

// The doc engine's "ini" decode (LB-F125; Conan's conanfile.txt, whose sections
// hold one reference, option or generator per line rather than key = value
// pairs). The root is a map from section name to a sequence; each non-blank,
// non-comment line of a section is a map:
//
//	text   the line, trimmed
//	key    the text before its first "=" outside square brackets, trimmed (the
//	       whole text without one: a Conan version range [>=1.2 <2] is not a pair)
//	value  the text after that "=", trimmed (present only with one)
//
// A line whose first character is "#" or ";" is a comment; a "#" later in a
// line is text (a Conan reference writes its revision as name/1.0#rev). A line
// before the first header belongs to no section and is dropped; a header
// written twice extends the section its first occurrence made. A section
// header is "[name]", the name trimmed; a line that merely starts with "[" but
// has no closing "]" is an ordinary line. Nothing here fails: any bytes decode.
func init() { docDecoders["ini"] = true }

func decodeINIDoc(src []byte) (*docNode, error) {
	src = bytes.TrimPrefix(src, []byte("\xef\xbb\xbf"))
	root := &docNode{kind: docMap, line: 1}
	var sec *docNode
	for i, raw := range strings.Split(string(src), "\n") {
		lineNo := i + 1
		text := strings.TrimSpace(raw)
		if text == "" || text[0] == '#' || text[0] == ';' {
			continue
		}
		if text[0] == '[' && strings.HasSuffix(text, "]") {
			name := strings.TrimSpace(text[1 : len(text)-1])
			if existing := root.get(name); existing != nil {
				sec = existing
			} else {
				sec = &docNode{kind: docSeq, line: lineNo}
				root.set(name, sec)
			}
			continue
		}
		if sec == nil {
			continue
		}
		item := &docNode{kind: docMap, line: lineNo}
		item.set("text", &docNode{kind: docScalar, scalar: text, line: lineNo})
		key, value, hasValue := cutINIPair(text)
		item.set("key", &docNode{kind: docScalar, scalar: strings.TrimSpace(key), line: lineNo})
		if hasValue {
			item.set("value", &docNode{kind: docScalar, scalar: strings.TrimSpace(value), line: lineNo})
		}
		sec.items = append(sec.items, item)
	}
	return root, nil
}

// cutINIPair splits a line at its first "=" that is not inside square brackets.
func cutINIPair(text string) (key, value string, ok bool) {
	depth := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
			}
		case '=':
			if depth == 0 {
				return text[:i], text[i+1:], true
			}
		}
	}
	return text, "", false
}
