package extractor

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// libconfigBareRe is a scalar libconfig reads without quotes: a boolean or a
// number (decimal, hex or float, an L or LL suffix on an integer). Any other bare
// word is a syntax error.
var libconfigBareRe = regexp.MustCompile(`^(?i:true|false|[-+]?(?:0x[0-9a-f]+|[0-9]+(?:\.[0-9]*)?(?:e[-+]?[0-9]+)?|\.[0-9]+(?:e[-+]?[0-9]+)?)(?:l|ll)?)$`)

// The doc engine's "libconfig" decode (LB-Q-proxysql; proxysql.cnf, which
// ProxySQL parses with libconfig). The root is the implicit group of the file:
//
//	group  { name = value ... }   a map, in file order
//	list   ( value, ... )         a sequence
//	array  [ scalar, ... ]        a sequence
//	scalar                        its text: a string without its quotes (adjacent
//	                              strings joined, \" \\ \n \t \r read), a number or
//	                              a boolean as written
//
// A setting is `name = value` or `name : value`, optionally ended by ";" or ",";
// a name is a letter or "*" then letters, digits and "_*-". A setting's line is
// where its name is written, a list item's where its first token is, and a group
// or list ends at its closing bracket. Comments are "#" and "//" to the end of
// the line and "/* */"; an "@include" directive is dropped (the file it names is
// not read). Anything libconfig refuses is an error: a file ProxySQL does not
// load reads as no entities.
func init() { docDecoders["libconfig"] = true }

type libconfigParser struct {
	src  []byte
	pos  int
	line int
}

func decodeLibconfigDoc(src []byte) (*docNode, error) {
	p := &libconfigParser{src: bytes.TrimPrefix(src, []byte("\xef\xbb\xbf")), line: 1}
	root := &docNode{kind: docMap, line: 1}
	if err := p.settings(root, 0); err != nil {
		return nil, err
	}
	return root, nil
}

func (p *libconfigParser) errf(format string, a ...any) error {
	return fmt.Errorf("libconfig: line %d: %s", p.line, fmt.Sprintf(format, a...))
}

// skip passes whitespace, comments and an @include directive.
func (p *libconfigParser) skip() error {
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case c == '\n':
			p.line++
			p.pos++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f':
			p.pos++
		case c == '#' || (c == '/' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '/'):
			p.eol()
		case c == '/' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '*':
			end := bytes.Index(p.src[p.pos+2:], []byte("*/"))
			if end < 0 {
				return p.errf("unterminated comment")
			}
			p.line += bytes.Count(p.src[p.pos:p.pos+2+end], []byte("\n"))
			p.pos += end + 4
		case c == '@' && bytes.HasPrefix(p.src[p.pos:], []byte("@include")):
			p.eol()
		default:
			return nil
		}
	}
	return nil
}

func (p *libconfigParser) eol() {
	for p.pos < len(p.src) && p.src[p.pos] != '\n' {
		p.pos++
	}
}

func libconfigNameStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '*'
}

func libconfigNameByte(c byte) bool {
	return libconfigNameStart(c) || c >= '0' && c <= '9' || c == '_' || c == '-'
}

// settings reads name/value pairs into m until the closing brace (close '}')
// or the end of the file (close 0); it returns with the closer consumed and
// m.endLine set.
func (p *libconfigParser) settings(m *docNode, close byte) error {
	for {
		if err := p.skip(); err != nil {
			return err
		}
		if p.pos >= len(p.src) {
			if close != 0 {
				return p.errf("missing %q", close)
			}
			return nil
		}
		c := p.src[p.pos]
		if c == close && close != 0 {
			p.pos++
			m.endLine = p.line
			return nil
		}
		if !libconfigNameStart(c) {
			return p.errf("unexpected %q", c)
		}
		start, nameLine := p.pos, p.line
		for p.pos < len(p.src) && libconfigNameByte(p.src[p.pos]) {
			p.pos++
		}
		name := string(p.src[start:p.pos])
		if err := p.skip(); err != nil {
			return err
		}
		if p.pos >= len(p.src) || (p.src[p.pos] != '=' && p.src[p.pos] != ':') {
			return p.errf("setting %q has no = or :", name)
		}
		p.pos++
		v, err := p.value()
		if err != nil {
			return err
		}
		v.line = nameLine
		m.set(name, v)
		if err := p.skip(); err != nil {
			return err
		}
		if p.pos < len(p.src) && (p.src[p.pos] == ';' || p.src[p.pos] == ',') {
			p.pos++
		}
	}
}

// value reads one value and sets its first line to the line of its first token.
func (p *libconfigParser) value() (*docNode, error) {
	if err := p.skip(); err != nil {
		return nil, err
	}
	if p.pos >= len(p.src) {
		return nil, p.errf("missing value")
	}
	first := p.line
	switch c := p.src[p.pos]; c {
	case '{':
		p.pos++
		m := &docNode{kind: docMap, line: first}
		if err := p.settings(m, '}'); err != nil {
			return nil, err
		}
		return m, nil
	case '(', '[':
		p.pos++
		closer := byte(')')
		if c == '[' {
			closer = ']'
		}
		seq := &docNode{kind: docSeq, line: first}
		for {
			if err := p.skip(); err != nil {
				return nil, err
			}
			if p.pos >= len(p.src) {
				return nil, p.errf("missing %q", closer)
			}
			if p.src[p.pos] == closer {
				p.pos++
				seq.endLine = p.line
				return seq, nil
			}
			item, err := p.value()
			if err != nil {
				return nil, err
			}
			seq.items = append(seq.items, item)
			if err := p.skip(); err != nil {
				return nil, err
			}
			if p.pos < len(p.src) && p.src[p.pos] == ',' {
				p.pos++
			} else if p.pos < len(p.src) && p.src[p.pos] != closer {
				return nil, p.errf("expected , or %q", closer)
			}
		}
	case '"':
		return p.str(first)
	default:
		start := p.pos
		for p.pos < len(p.src) && strings.IndexByte(" \t\r\n,;)]}#", p.src[p.pos]) < 0 {
			p.pos++
		}
		if p.pos == start {
			return nil, p.errf("unexpected %q", c)
		}
		word := string(p.src[start:p.pos])
		if !libconfigBareRe.MatchString(word) {
			return nil, p.errf("%q is neither a number, a boolean nor a quoted string", word)
		}
		return &docNode{kind: docScalar, scalar: word, line: first, endLine: first}, nil
	}
}

// str reads a string, and any adjacent strings, into one scalar.
func (p *libconfigParser) str(first int) (*docNode, error) {
	var b strings.Builder
	end := first
	for p.pos < len(p.src) && p.src[p.pos] == '"' {
		p.pos++
		for {
			if p.pos >= len(p.src) {
				return nil, p.errf("unterminated string")
			}
			c := p.src[p.pos]
			p.pos++
			if c == '"' {
				break
			}
			if c == '\n' {
				p.line++
			}
			if c == '\\' && p.pos < len(p.src) {
				e := p.src[p.pos]
				p.pos++
				switch e {
				case 'n':
					c = '\n'
				case 't':
					c = '\t'
				case 'r':
					c = '\r'
				case 'f':
					c = '\f'
				default:
					c = e
				}
			}
			b.WriteByte(c)
		}
		end = p.line
		if err := p.skip(); err != nil {
			return nil, err
		}
	}
	return &docNode{kind: docScalar, scalar: b.String(), line: first, endLine: end}, nil
}
