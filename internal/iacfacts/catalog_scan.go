package iacfacts

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"unicode/utf8"
)

// catScan walks one JSON document without building it: it skips what it is not asked for with no allocation and
// captures only the requested value. It is the "streaming decoder" of design section 9. It enforces the depth bound
// and a capture bound, and treats a syntax fault in what it reads as corrupt. The rest of a document it never
// reaches is vouched for by the blob's content address, which the caller checks by draining through a hash.
type catScan struct {
	br       *bufio.Reader
	maxDepth int
	capMax   int
	// capHint is an upper bound on a captured value's size (the blob's size). A capture that outgrows 64 KiB grows
	// once to it, so a large fact costs one allocation instead of the doubling series that would cost twice its size.
	capHint int
	capOn   bool
	capBuf  []byte
	last    byte
	unread  bool
	key     []byte
}

const capChunk = 64 << 10

var (
	errScanDepth   = errors.New("json depth bound exceeded")
	errScanSyntax  = errors.New("json syntax")
	errScanCapture = errors.New("captured value over the fact bound")
)

func newCatScan(r io.Reader, maxDepth, capMax int) *catScan {
	return &catScan{br: bufio.NewReaderSize(r, 32<<10), maxDepth: maxDepth, capMax: capMax}
}

func (s *catScan) read() (byte, error) {
	var b byte
	if s.unread {
		s.unread = false
		b = s.last
	} else {
		var err error
		if b, err = s.br.ReadByte(); err != nil {
			if errors.Is(err, io.EOF) {
				return 0, io.ErrUnexpectedEOF
			}
			return 0, err
		}
		s.last = b
	}
	if s.capOn {
		s.capBuf = append(s.capBuf, b)
		if len(s.capBuf) > s.capMax {
			return 0, errScanCapture
		}
		if len(s.capBuf) == capChunk && s.capHint > capChunk {
			grown := make([]byte, capChunk, s.capHint)
			copy(grown, s.capBuf)
			s.capBuf = grown
		}
	}
	return b, nil
}

// back returns the byte read last. It may be called once after a read.
func (s *catScan) back() {
	s.unread = true
	if s.capOn {
		s.capBuf = s.capBuf[:len(s.capBuf)-1]
	}
}

func (s *catScan) nonWS() (byte, error) {
	for {
		b, err := s.read()
		if err != nil {
			return 0, err
		}
		if b != ' ' && b != '\t' && b != '\n' && b != '\r' {
			return b, nil
		}
	}
}

func (s *catScan) expect(lit string) error {
	for i := 0; i < len(lit); i++ {
		b, err := s.read()
		if err != nil {
			return err
		}
		if b != lit[i] {
			return errScanSyntax
		}
	}
	return nil
}

// skipString consumes a string whose opening quote is read.
func (s *catScan) skipString() error {
	for {
		b, err := s.read()
		if err != nil {
			return err
		}
		switch {
		case b == '"':
			return nil
		case b < 0x20:
			return errScanSyntax
		case b == '\\':
			e, err := s.read()
			if err != nil {
				return err
			}
			if e == 'u' {
				for i := 0; i < 4; i++ {
					h, err := s.read()
					if err != nil {
						return err
					}
					if hexVal(h) < 0 {
						return errScanSyntax
					}
				}
			} else if !bytes.ContainsRune([]byte(`"\/bfnrt`), rune(e)) {
				return errScanSyntax
			}
		}
	}
}

func hexVal(b byte) int {
	switch {
	case b >= '0' && b <= '9':
		return int(b - '0')
	case b >= 'a' && b <= 'f':
		return int(b-'a') + 10
	case b >= 'A' && b <= 'F':
		return int(b-'A') + 10
	}
	return -1
}

// readKey decodes a string whose opening quote is read into s.key.
func (s *catScan) readKey() error {
	s.key = s.key[:0]
	for {
		b, err := s.read()
		if err != nil {
			return err
		}
		switch {
		case b == '"':
			return nil
		case b < 0x20:
			return errScanSyntax
		case b != '\\':
			s.key = append(s.key, b)
			continue
		}
		e, err := s.read()
		if err != nil {
			return err
		}
		switch e {
		case '"', '\\', '/':
			s.key = append(s.key, e)
		case 'b':
			s.key = append(s.key, '\b')
		case 'f':
			s.key = append(s.key, '\f')
		case 'n':
			s.key = append(s.key, '\n')
		case 'r':
			s.key = append(s.key, '\r')
		case 't':
			s.key = append(s.key, '\t')
		case 'u':
			r, err := s.hex4()
			if err != nil {
				return err
			}
			if r >= 0xD800 && r < 0xDC00 {
				// a surrogate pair: the low half must follow.
				if b1, err := s.read(); err != nil || b1 != '\\' {
					return errScanSyntax
				}
				if b2, err := s.read(); err != nil || b2 != 'u' {
					return errScanSyntax
				}
				lo, err := s.hex4()
				if err != nil || lo < 0xDC00 || lo >= 0xE000 {
					return errScanSyntax
				}
				r = 0x10000 + (r-0xD800)<<10 + (lo - 0xDC00)
			} else if r >= 0xDC00 && r < 0xE000 {
				return errScanSyntax
			}
			s.key = utf8.AppendRune(s.key, r)
		default:
			return errScanSyntax
		}
	}
}

func (s *catScan) hex4() (rune, error) {
	var r rune
	for i := 0; i < 4; i++ {
		b, err := s.read()
		if err != nil {
			return 0, err
		}
		v := hexVal(b)
		if v < 0 {
			return 0, errScanSyntax
		}
		r = r<<4 | rune(v)
	}
	return r, nil
}

// skipValue consumes one value. depth is the container nesting around it.
func (s *catScan) skipValue(depth int) error {
	c, err := s.nonWS()
	if err != nil {
		return err
	}
	return s.skipFrom(c, depth)
}

func (s *catScan) skipFrom(c byte, depth int) error {
	switch {
	case c == '"':
		return s.skipString()
	case c == 't':
		return s.expect("rue")
	case c == 'f':
		return s.expect("alse")
	case c == 'n':
		return s.expect("ull")
	case c == '-' || (c >= '0' && c <= '9'):
		for {
			b, err := s.read()
			if err != nil {
				return err
			}
			if !(b >= '0' && b <= '9') && b != '.' && b != 'e' && b != 'E' && b != '+' && b != '-' {
				s.back()
				return nil
			}
		}
	case c == '{':
		if depth+1 > s.maxDepth {
			return errScanDepth
		}
		return s.skipMembers(depth + 1)
	case c == '[':
		if depth+1 > s.maxDepth {
			return errScanDepth
		}
		return s.skipElements(depth + 1)
	}
	return errScanSyntax
}

// skipMembers consumes an object after its '{'.
func (s *catScan) skipMembers(depth int) error {
	c, err := s.nonWS()
	if err != nil {
		return err
	}
	if c == '}' {
		return nil
	}
	for {
		if c != '"' {
			return errScanSyntax
		}
		if err := s.skipString(); err != nil {
			return err
		}
		if c, err = s.nonWS(); err != nil || c != ':' {
			return orSyntax(err)
		}
		if err := s.skipValue(depth); err != nil {
			return err
		}
		if c, err = s.nonWS(); err != nil {
			return err
		}
		switch c {
		case '}':
			return nil
		case ',':
			if c, err = s.nonWS(); err != nil {
				return err
			}
		default:
			return errScanSyntax
		}
	}
}

// skipElements consumes an array after its '['.
func (s *catScan) skipElements(depth int) error {
	c, err := s.nonWS()
	if err != nil {
		return err
	}
	if c == ']' {
		return nil
	}
	s.back()
	for {
		if err := s.skipValue(depth); err != nil {
			return err
		}
		if c, err = s.nonWS(); err != nil {
			return err
		}
		switch c {
		case ']':
			return nil
		case ',':
		default:
			return errScanSyntax
		}
	}
}

func orSyntax(err error) error {
	if err != nil {
		return err
	}
	return errScanSyntax
}

// capture consumes one value and keeps its bytes in s.capBuf.
func (s *catScan) capture(depth int) error {
	_, err := s.nonWS()
	if err != nil {
		return err
	}
	s.back()
	s.capBuf = s.capBuf[:0]
	s.capOn = true
	defer func() { s.capOn = false }()
	return s.skipValue(depth)
}

// find walks path from the value at depth and captures the value it names. It reports false when a key or index is
// absent, and leaves the rest of the document unread.
func (s *catScan) find(path []string, depth int) (bool, error) {
	if len(path) == 0 {
		return true, s.capture(depth)
	}
	c, err := s.nonWS()
	if err != nil {
		return false, err
	}
	switch c {
	case '{':
		if depth+1 > s.maxDepth {
			return false, errScanDepth
		}
		return s.findMember(path, depth+1)
	case '[':
		if depth+1 > s.maxDepth {
			return false, errScanDepth
		}
		return s.findElement(path, depth+1)
	}
	return false, nil
}

func (s *catScan) findMember(path []string, depth int) (bool, error) {
	c, err := s.nonWS()
	if err != nil {
		return false, err
	}
	if c == '}' {
		return false, nil
	}
	for {
		if c != '"' {
			return false, errScanSyntax
		}
		if err := s.readKey(); err != nil {
			return false, err
		}
		match := string(s.key) == path[0]
		if c, err = s.nonWS(); err != nil || c != ':' {
			return false, orSyntax(err)
		}
		if match {
			return s.find(path[1:], depth)
		}
		if err := s.skipValue(depth); err != nil {
			return false, err
		}
		if c, err = s.nonWS(); err != nil {
			return false, err
		}
		switch c {
		case '}':
			return false, nil
		case ',':
			if c, err = s.nonWS(); err != nil {
				return false, err
			}
		default:
			return false, errScanSyntax
		}
	}
}

func (s *catScan) findElement(path []string, depth int) (bool, error) {
	want, ok := decimalIndex(path[0])
	c, err := s.nonWS()
	if err != nil {
		return false, err
	}
	if c == ']' {
		return false, nil
	}
	s.back()
	for i := 0; ; i++ {
		if ok && i == want {
			return s.find(path[1:], depth)
		}
		if !ok {
			return false, nil
		}
		if err := s.skipValue(depth); err != nil {
			return false, err
		}
		if c, err = s.nonWS(); err != nil {
			return false, err
		}
		switch c {
		case ']':
			return false, nil
		case ',':
		default:
			return false, errScanSyntax
		}
	}
}

func decimalIndex(s string) (int, bool) {
	if s == "" || len(s) > 9 {
		return 0, false
	}
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
		n = n*10 + int(s[i]-'0')
	}
	return n, true
}

// drain reads what is left of the document so a hash beneath it covers every byte.
func (s *catScan) drain() error {
	_, err := io.Copy(io.Discard, s.br)
	return err
}
