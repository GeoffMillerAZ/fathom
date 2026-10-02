package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// astParser streams one translation unit's `-ast-dump=json` output. The dump
// writes a location's file only when it differs from the last location written
// (clang's JSONNodeDumper::writeBareSourceLocation), so the parser reads every
// location in text order and keeps that state; a location's position is its
// byte offset in that file. An `includedFrom` object names the includer, not
// the location's own file, and is no state change.
type astParser struct {
	r        *bufio.Reader
	dir      string
	files    *fileCache
	add      func(site)
	lastFile string
}

// bareLoc is one written location: the file it is in and its byte offset and
// token length there.
type bareLoc struct {
	file           string
	offset, tokLen int
	ok             bool
}

// srcLoc is a location as the dump writes it: inside a macro expansion a
// spelling and an expansion location, elsewhere one location for both.
type srcLoc struct{ spelling, expansion bareLoc }

// astNode is the part of one AST node the oracle reads.
type astNode struct {
	kind       string
	loc        srcLoc // "loc": a declaration's name
	begin, end srcLoc
	hasRange   bool
	name       string // "name" (a MemberExpr, an unresolved lookup) or "member"
	refKind    string // referencedDecl.kind (a DeclRefExpr)
	qualType   string // type.qualType
	castKind   string
}

func (p *astParser) top() error {
	c, err := p.next()
	if err != nil {
		return err
	}
	if c != '{' {
		return fmt.Errorf("clang-calls: dump does not start with an object (%q)", c)
	}
	return p.node()
}

// next returns the next byte that is not white space.
func (p *astParser) next() (byte, error) {
	for {
		c, err := p.r.ReadByte()
		if err != nil {
			return 0, err
		}
		switch c {
		case ' ', '\t', '\n', '\r':
			continue
		}
		return c, nil
	}
}

func (p *astParser) expect(want byte) error {
	c, err := p.next()
	if err != nil {
		return err
	}
	if c != want {
		return fmt.Errorf("clang-calls: want %q, have %q", want, c)
	}
	return nil
}

// str reads a string whose opening quote was consumed.
func (p *astParser) str() (string, error) {
	var b []byte
	for {
		c, err := p.r.ReadByte()
		if err != nil {
			return "", err
		}
		switch c {
		case '"':
			return string(b), nil
		case '\\':
			e, err := p.r.ReadByte()
			if err != nil {
				return "", err
			}
			switch e {
			case 'n':
				b = append(b, '\n')
			case 't':
				b = append(b, '\t')
			case 'r':
				b = append(b, '\r')
			case 'b':
				b = append(b, '\b')
			case 'f':
				b = append(b, '\f')
			case 'u':
				var hex [4]byte
				if _, err := io.ReadFull(p.r, hex[:]); err != nil {
					return "", err
				}
				n, err := strconv.ParseUint(string(hex[:]), 16, 32)
				if err != nil {
					return "", err
				}
				b = append(b, string(rune(n))...)
			default:
				b = append(b, e)
			}
		default:
			b = append(b, c)
		}
	}
}

// skipStr skips a string whose opening quote was consumed.
func (p *astParser) skipStr() error {
	for {
		c, err := p.r.ReadByte()
		if err != nil {
			return err
		}
		switch c {
		case '"':
			return nil
		case '\\':
			if _, err := p.r.ReadByte(); err != nil {
				return err
			}
		}
	}
}

// key reads an object key and its colon; ok is false at the object's end.
func (p *astParser) key() (string, bool, error) {
	for {
		c, err := p.next()
		if err != nil {
			return "", false, err
		}
		switch c {
		case ',':
			continue
		case '}':
			return "", false, nil
		case '"':
			k, err := p.str()
			if err != nil {
				return "", false, err
			}
			return k, true, p.expect(':')
		default:
			return "", false, fmt.Errorf("clang-calls: want a key, have %q", c)
		}
	}
}

// scalar reads a number, true, false or null whose first byte is c.
func (p *astParser) scalar(c byte) (string, error) {
	b := []byte{c}
	for {
		d, err := p.r.ReadByte()
		if err != nil {
			return string(b), err
		}
		switch d {
		case ',', '}', ']', ' ', '\n', '\t', '\r':
			return string(b), p.r.UnreadByte()
		}
		b = append(b, d)
	}
}

// skip skips one value. With track, a location written inside it still moves
// the file state (every "file" key but an includedFrom's).
func (p *astParser) skip(track bool) error {
	c, err := p.next()
	if err != nil {
		return err
	}
	switch c {
	case '{':
		for {
			k, ok, err := p.key()
			if err != nil || !ok {
				return err
			}
			switch {
			case k == "file" && track:
				if err := p.fileValue(); err != nil {
					return err
				}
			case k == "includedFrom":
				if err := p.skip(false); err != nil {
					return err
				}
			default:
				if err := p.skip(track); err != nil {
					return err
				}
			}
		}
	case '[':
		for {
			d, err := p.next()
			if err != nil {
				return err
			}
			switch d {
			case ']':
				return nil
			case ',':
				continue
			}
			if err := p.r.UnreadByte(); err != nil {
				return err
			}
			if err := p.skip(track); err != nil {
				return err
			}
		}
	case '"':
		return p.skipStr()
	default:
		_, err := p.scalar(c)
		return err
	}
}

func (p *astParser) fileValue() error {
	c, err := p.next()
	if err != nil {
		return err
	}
	if c != '"' {
		_, err := p.scalar(c)
		return err
	}
	f, err := p.str()
	if err != nil {
		return err
	}
	p.lastFile = f
	return nil
}

// strValue reads a value expected to be a string ("" for any other value).
func (p *astParser) strValue() (string, error) {
	c, err := p.next()
	if err != nil {
		return "", err
	}
	if c == '"' {
		return p.str()
	}
	if err := p.r.UnreadByte(); err != nil {
		return "", err
	}
	return "", p.skip(true)
}

// loc reads a location object.
func (p *astParser) loc() (srcLoc, error) {
	var out srcLoc
	if err := p.expect('{'); err != nil {
		return out, err
	}
	var bare bareLoc
	nested := false
	for {
		k, ok, err := p.key()
		if err != nil {
			return out, err
		}
		if !ok {
			break
		}
		switch k {
		case "offset", "tokLen":
			c, err := p.next()
			if err != nil {
				return out, err
			}
			s, err := p.scalar(c)
			if err != nil {
				return out, err
			}
			n, _ := strconv.Atoi(s)
			if k == "offset" {
				bare.offset, bare.ok = n, true
			} else {
				bare.tokLen = n
			}
		case "file":
			if err := p.fileValue(); err != nil {
				return out, err
			}
		case "includedFrom":
			if err := p.skip(false); err != nil {
				return out, err
			}
		case "spellingLoc":
			l, err := p.loc()
			if err != nil {
				return out, err
			}
			out.spelling, nested = l.spelling, true
		case "expansionLoc":
			l, err := p.loc()
			if err != nil {
				return out, err
			}
			out.expansion, nested = l.expansion, true
		default:
			if err := p.skip(true); err != nil {
				return out, err
			}
		}
	}
	if !nested {
		if bare.ok {
			bare.file = p.lastFile
		}
		out.spelling, out.expansion = bare, bare
	}
	return out, nil
}

// node reads one AST node whose opening brace was consumed, its children
// first, then reports it.
func (p *astParser) node() error {
	var n astNode
	for {
		k, ok, err := p.key()
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		switch k {
		case "kind":
			if n.kind, err = p.strValue(); err != nil {
				return err
			}
		case "range":
			if err := p.expect('{'); err != nil {
				return err
			}
			for {
				rk, ok, err := p.key()
				if err != nil {
					return err
				}
				if !ok {
					break
				}
				switch rk {
				case "begin":
					n.begin, err = p.loc()
				case "end":
					n.end, err = p.loc()
				default:
					err = p.skip(true)
				}
				if err != nil {
					return err
				}
			}
			n.hasRange = true
		case "loc":
			if n.loc, err = p.loc(); err != nil {
				return err
			}
		case "name", "member":
			if n.name, err = p.strValue(); err != nil {
				return err
			}
		case "castKind":
			if n.castKind, err = p.strValue(); err != nil {
				return err
			}
		case "referencedDecl", "type":
			c, err := p.next()
			if err != nil {
				return err
			}
			if c != '{' {
				if _, err := p.scalar(c); err != nil {
					return err
				}
				continue
			}
			for {
				ik, ok, err := p.key()
				if err != nil {
					return err
				}
				if !ok {
					break
				}
				switch {
				case k == "referencedDecl" && ik == "kind":
					n.refKind, err = p.strValue()
				case k == "type" && ik == "qualType":
					n.qualType, err = p.strValue()
				default:
					err = p.skip(true)
				}
				if err != nil {
					return err
				}
			}
		case "inner":
			if err := p.expect('['); err != nil {
				return err
			}
			for {
				c, err := p.next()
				if err != nil {
					return err
				}
				if c == ']' {
					break
				}
				if c == ',' {
					continue
				}
				if c != '{' {
					if _, err := p.scalar(c); err != nil {
						return err
					}
					continue
				}
				if err := p.node(); err != nil {
					return err
				}
			}
		default:
			if err := p.skip(true); err != nil {
				return err
			}
		}
	}
	if n.hasRange {
		p.report(&n)
	}
	if declKinds[n.kind] {
		p.reportDecl(&n)
	}
	return nil
}

// declKinds are the declarations of a function, whose name written before `(`
// is no call: the index writes a reference there for a declaration that is
// not a definition (`int close(int fd);`).
var declKinds = map[string]bool{"FunctionDecl": true, "CXXMethodDecl": true, "CXXConstructorDecl": true, "CXXDestructorDecl": true, "CXXConversionDecl": true}

// reportDecl adds a declaration's name as a site of class declaration.
func (p *astParser) reportDecl(n *astNode) {
	l := n.loc.spelling
	if !l.ok {
		return
	}
	f := p.files.get(p.dir, l.file)
	if f == nil || l.offset+l.tokLen > len(f.src) || l.tokLen <= 0 {
		return
	}
	name, at, ok := calledName(f.src, l.offset, l.offset+l.tokLen)
	if !ok || at != l.offset {
		return
	}
	line, col := f.lineCol(at)
	p.add(site{Path: f.rel, Line: line, Col: col, Name: name, Class: "declaration"})
}

// report classifies a node and adds its site when it is call-shaped and
// written in a subject file.
func (p *astParser) report(n *astNode) {
	class, shape := classify(n)
	if class == "" {
		return
	}
	b, e := n.begin.spelling, n.end.spelling
	if !b.ok || !e.ok || b.file != e.file {
		return
	}
	f := p.files.get(p.dir, b.file)
	if f == nil || b.offset > e.offset || e.offset+e.tokLen > len(f.src) {
		return
	}
	var name string
	var at int
	ok := false
	switch shape {
	case shapeName:
		name, at, ok = calledName(f.src, b.offset, e.offset+e.tokLen)
	case shapeType:
		name, at, ok = constructedType(f.src, b.offset, e.offset+e.tokLen)
		if ok && n.kind == "CXXConstructExpr" && lastIdent(n.qualType) != name {
			ok = false // a declaration's direct initialisation, `Foo f(1)`: written as no call
		}
	}
	if !ok {
		return
	}
	line, col := f.lineCol(at)
	p.add(site{Path: f.rel, Line: line, Col: col, Name: name, Class: class})
}

const (
	shapeName = iota // the callee is a name written before `(`
	shapeType        // the callee is a type written before `(` or `{`
)

// classify is a node's class and the shape its site is written in ("" when it
// is no call site).
func classify(n *astNode) (string, int) {
	switch n.kind {
	case "DeclRefExpr":
		switch n.refKind {
		case "FunctionDecl", "CXXMethodDecl", "CXXConversionDecl":
			return "resolved", shapeName
		case "VarDecl", "ParmVarDecl", "FieldDecl", "BindingDecl", "NonTypeTemplateParmDecl":
			return "object", shapeName
		}
	case "MemberExpr":
		if n.qualType == "<bound member function type>" {
			return "resolved", shapeName
		}
		return "object", shapeName
	case "CXXDependentScopeMemberExpr", "UnresolvedMemberExpr", "UnresolvedLookupExpr", "DependentScopeDeclRefExpr":
		return "dependent", shapeName
	case "CXXUnresolvedConstructExpr":
		return "dependent", shapeType
	case "CXXTemporaryObjectExpr", "CXXConstructExpr":
		return "construct", shapeType
	case "CXXFunctionalCastExpr":
		if n.castKind == "ConstructorConversion" {
			return "construct", shapeType
		}
		return "cast", shapeType
	}
	return "", 0
}

// fileCache holds the subject files a dump names, read once.
type fileCache struct {
	root string
	mu   sync.Mutex
	m    map[string]*srcFile // absolute path -> file (nil: outside the subject)
	err  error               // the first subject file that could not be read
}

type srcFile struct {
	rel   string
	src   []byte
	lines []int // byte offset of each line's start
}

func newFileCache(root string) *fileCache {
	return &fileCache{root: root, m: map[string]*srcFile{}}
}

func (c *fileCache) get(dir, name string) *srcFile {
	if name == "" || strings.HasPrefix(name, "<") {
		return nil
	}
	if !filepath.IsAbs(name) {
		name = filepath.Join(dir, name)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if f, ok := c.m[name]; ok {
		return f
	}
	var f *srcFile
	abs := filepath.Clean(name)
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	if rel, err := filepath.Rel(c.root, abs); err == nil && !strings.HasPrefix(rel, "..") {
		src, err := os.ReadFile(abs)
		for attempt := 1; err != nil && !os.IsNotExist(err) && attempt < retries; attempt++ {
			time.Sleep(2 * time.Second) // an unreadable file can be transient
			src, err = os.ReadFile(abs)
		}
		if err != nil {
			// A subject file the compiler read and this process cannot is a
			// lost file, never a file with no calls: the run fails.
			if c.err == nil {
				c.err = err
			}
			return nil
		}
		f = &srcFile{rel: filepath.ToSlash(rel), src: src, lines: []int{0}}
		for i, b := range src {
			if b == '\n' {
				f.lines = append(f.lines, i+1)
			}
		}
	}
	c.m[name] = f
	return f
}

// lineCol is the 1-based line and byte column of offset.
func (f *srcFile) lineCol(offset int) (int, int) {
	lo, hi := 0, len(f.lines)
	for lo+1 < hi {
		mid := (lo + hi) / 2
		if f.lines[mid] <= offset {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo + 1, offset - f.lines[lo] + 1
}
