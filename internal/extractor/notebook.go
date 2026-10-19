package extractor

// notebook.go -- the jupyter dialect (LB-Q-jupyter, docs/design/language-breadth.md
// section 13, engine "native": encoding/json over the notebook and the python dialect
// over its code cells). A Jupyter notebook is JSON whose code cells share one Python
// namespace; the dialect reads each code cell with the python tags engine and gives the
// definitions and calls it finds the line of the notebook file that holds the source
// line, so a function defined on the third line of a cell lands where an editor shows it.
//
// What is read (dialects/jupyter/PROVENANCE.md says what is not):
//
//   - notebook format 4: `cells[]`, each `cell_type`, `source` (a string or a list of
//     strings, joined as nbformat joins them). Markdown and raw cells and every cell's
//     `outputs` and `metadata` are never read.
//   - a notebook whose `metadata.language_info.name` (else `metadata.kernelspec.language`)
//     names a language other than Python reads as empty.
//   - IPython syntax is made Python before the cell is parsed: a line whose first
//     non-blank character is `%` or `!` becomes `pass` at the same indent, and a cell whose
//     first line is a cell magic other than the ones that run Python (time, timeit, capture,
//     prun, debug, python, python3, pypy) is skipped. The line count never changes.
//   - each code cell is a separate parse (a broken cell cannot break its neighbours); their
//     definitions are the notebook's, in order, and the calls no definition encloses are
//     the one `<module>` symbol's.
//
// Line mapping. A source line is mapped to the 1-based line of the notebook file that holds
// the JSON string it begins in: with the list form, one line per element; with a single
// string, every line of the cell maps to the line of that string. The offsets come from
// the JSON token stream (json.Decoder.InputOffset), not from a search of the text.

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// notebookKind maps the python dialect's source kinds to the notebook's own (a
// source kind is emitted by one dialect, the registry's rule).
func notebookKind(k core.SourceKind) (core.SourceKind, bool) {
	switch k {
	case "PythonClass":
		return "JupyterClass", true
	case "PythonFunc":
		return "JupyterFunc", true
	case "PythonVar":
		return "JupyterVar", true
	case "PythonTypeAlias":
		return "JupyterTypeAlias", true
	case "PythonField":
		return "JupyterField", true
	case "PythonModule":
		return "JupyterModule", true
	}
	return k, false
}

// notebookModuleName is the name of the notebook's own symbol, the python module
// symbol's name.
const notebookModuleName = "<module>"

// pythonCellMagic reports whether a cell magic runs its body as Python.
func pythonCellMagic(name string) bool {
	switch name {
	case "time", "timeit", "capture", "prun", "debug", "python", "python3", "pypy":
		return true
	}
	return false
}

// NotebookExtractor reads .ipynb files. py is the python tags extractor the cells are
// parsed with.
type NotebookExtractor struct {
	py Extractor
}

var _ Extractor = (*NotebookExtractor)(nil)

// NewNotebookExtractor returns an extractor that parses code cells with py, a
// python-dialect extractor (the pipeline's own, so one pooled module serves both).
func NewNotebookExtractor(py Extractor) *NotebookExtractor { return &NotebookExtractor{py: py} }

// NewNotebookDialectExtractor builds the jupyter dialect's stand-alone extractor, with a
// python extractor of its own (the harness measures it through
// NativeDialectExtractor).
func NewNotebookDialectExtractor() (*NotebookExtractor, error) {
	py, err := NewTSExtractor("python")
	if err != nil {
		return nil, err
	}
	return &NotebookExtractor{py: py}, nil
}

// nbPiece is one string of a cell's source and the notebook-file line that holds it.
type nbPiece struct {
	text string
	line int
}

// nbCell is a code cell: its source strings in order.
type nbCell struct {
	pieces []nbPiece
}

// nbDoc is what the reader keeps of a notebook.
type nbDoc struct {
	cells []nbCell
	lang  string
}

// nbScan walks a notebook's tokens.
type nbScan struct {
	dec *json.Decoder
	nl  []int64 // offsets of every '\n'
}

// lineOf is the 1-based line of the byte at offset off.
func (s *nbScan) lineOf(off int64) int {
	return sort.Search(len(s.nl), func(i int) bool { return s.nl[i] >= off }) + 1
}

func (s *nbScan) tok() (json.Token, bool) {
	t, err := s.dec.Token()
	return t, err == nil
}

// skip consumes one value whose first token is t.
func (s *nbScan) skip(t json.Token) bool {
	d, ok := t.(json.Delim)
	if !ok {
		return true
	}
	if d != '{' && d != '[' {
		return true
	}
	depth := 1
	for depth > 0 {
		t, ok := s.tok()
		if !ok {
			return false
		}
		if d, ok := t.(json.Delim); ok {
			switch d {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			}
		}
	}
	return true
}

// readNotebook parses src. ok is false for anything that is not a JSON object.
func readNotebook(src []byte) (nbDoc, bool) {
	s := &nbScan{dec: json.NewDecoder(bytes.NewReader(src))}
	for i, b := range src {
		if b == '\n' {
			s.nl = append(s.nl, int64(i))
		}
	}
	var doc nbDoc
	t, ok := s.tok()
	if d, isDelim := t.(json.Delim); !ok || !isDelim || d != '{' {
		return doc, false
	}
	for s.dec.More() {
		kt, ok := s.tok()
		if !ok {
			return doc, false
		}
		key, _ := kt.(string)
		vt, ok := s.tok()
		if !ok {
			return doc, false
		}
		switch key {
		case "cells":
			if !s.cells(vt, &doc) {
				return doc, false
			}
		case "metadata":
			if !s.metadata(vt, &doc) {
				return doc, false
			}
		default:
			if !s.skip(vt) {
				return doc, false
			}
		}
	}
	return doc, true
}

func (s *nbScan) cells(open json.Token, doc *nbDoc) bool {
	if d, ok := open.(json.Delim); !ok || d != '[' {
		return s.skip(open)
	}
	for s.dec.More() {
		t, ok := s.tok()
		if !ok {
			return false
		}
		if d, ok := t.(json.Delim); !ok || d != '{' {
			if !s.skip(t) {
				return false
			}
			continue
		}
		var code bool
		var cell nbCell
		for s.dec.More() {
			kt, ok := s.tok()
			if !ok {
				return false
			}
			key, _ := kt.(string)
			vt, ok := s.tok()
			if !ok {
				return false
			}
			switch key {
			case "cell_type":
				v, _ := vt.(string)
				code = v == "code"
			case "source":
				if !s.source(vt, &cell) {
					return false
				}
			default:
				if !s.skip(vt) {
					return false
				}
			}
		}
		if _, ok := s.tok(); !ok { // the cell's closing brace
			return false
		}
		if code {
			doc.cells = append(doc.cells, cell)
		}
	}
	_, ok := s.tok() // the array's closing bracket
	return ok
}

// source reads a cell's source: one string or a list of strings.
func (s *nbScan) source(first json.Token, c *nbCell) bool {
	switch v := first.(type) {
	case string:
		c.pieces = append(c.pieces, nbPiece{v, s.lineOf(s.dec.InputOffset() - 1)})
		return true
	case json.Delim:
		if v != '[' {
			return s.skip(first)
		}
		for s.dec.More() {
			t, ok := s.tok()
			if !ok {
				return false
			}
			if str, isStr := t.(string); isStr {
				c.pieces = append(c.pieces, nbPiece{str, s.lineOf(s.dec.InputOffset() - 1)})
				continue
			}
			if !s.skip(t) {
				return false
			}
		}
		_, ok := s.tok()
		return ok
	}
	return true
}

// metadata reads the notebook's language: language_info.name, else kernelspec.language.
func (s *nbScan) metadata(open json.Token, doc *nbDoc) bool {
	if d, ok := open.(json.Delim); !ok || d != '{' {
		return s.skip(open)
	}
	var info, kernel string
	for s.dec.More() {
		kt, ok := s.tok()
		if !ok {
			return false
		}
		key, _ := kt.(string)
		vt, ok := s.tok()
		if !ok {
			return false
		}
		if key != "language_info" && key != "kernelspec" {
			if !s.skip(vt) {
				return false
			}
			continue
		}
		if d, ok := vt.(json.Delim); !ok || d != '{' {
			if !s.skip(vt) {
				return false
			}
			continue
		}
		want := "name"
		if key == "kernelspec" {
			want = "language"
		}
		for s.dec.More() {
			ik, ok := s.tok()
			if !ok {
				return false
			}
			name, _ := ik.(string)
			iv, ok := s.tok()
			if !ok {
				return false
			}
			if v, isStr := iv.(string); isStr && name == want {
				if key == "kernelspec" {
					kernel = v
				} else {
					info = v
				}
				continue
			}
			if !s.skip(iv) {
				return false
			}
		}
		if _, ok := s.tok(); !ok {
			return false
		}
	}
	if _, ok := s.tok(); !ok {
		return false
	}
	doc.lang = strings.ToLower(info)
	if doc.lang == "" {
		doc.lang = strings.ToLower(kernel)
	}
	return true
}

// cellLines splits a cell into its lines, each with the notebook line of the piece
// holding the line's first byte.
func cellLines(pieces []nbPiece) (lines []string, phys []int) {
	starts := make([]int, len(pieces))
	var sb strings.Builder
	for i, p := range pieces {
		starts[i] = sb.Len()
		sb.WriteString(p.text)
	}
	full := sb.String()
	for pos := 0; pos < len(full); {
		end := strings.IndexByte(full[pos:], '\n')
		var line string
		next := len(full)
		if end < 0 {
			line = full[pos:]
		} else {
			line = full[pos : pos+end]
			next = pos + end + 1
		}
		pi := sort.Search(len(starts), func(i int) bool { return starts[i] > pos }) - 1
		lines = append(lines, strings.TrimSuffix(line, "\r"))
		phys = append(phys, pieces[pi].line)
		pos = next
	}
	return lines, phys
}

// pythonCell turns a cell's lines into Python: a line magic becomes `pass` at its
// indent, a leading Python cell magic line is blanked. skip is true for a cell that
// is not Python.
func pythonCell(lines []string) (out []string, skip bool) {
	out = append([]string(nil), lines...)
	if len(out) == 0 {
		return nil, true
	}
	if first := strings.TrimLeft(out[0], " \t"); strings.HasPrefix(first, "%%") {
		f := strings.Fields(first[2:])
		if len(f) == 0 || !pythonCellMagic(f[0]) {
			return nil, true
		}
		out[0] = ""
	}
	for i, l := range out {
		t := strings.TrimLeft(l, " \t")
		if strings.HasPrefix(t, "%") || strings.HasPrefix(t, "!") {
			out[i] = l[:len(l)-len(t)] + "pass"
		}
	}
	return out, false
}

// Extract implements Extractor.
func (x *NotebookExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	doc, ok := readNotebook(src)
	if !ok || (doc.lang != "" && doc.lang != "python") {
		return nil, nil
	}
	var nodes []core.SymbolNode
	var module *core.SymbolNode
	moduleCalls := map[string]bool{}
	firstLine, lastLine := 0, 0
	for _, c := range doc.cells {
		lines, phys := cellLines(c.pieces)
		py, skip := pythonCell(lines)
		if skip {
			continue
		}
		res, err := x.py.Extract(service, relPath, []byte(strings.Join(py, "\n")+"\n"))
		if err != nil {
			continue
		}
		at := func(n int) int {
			switch {
			case len(phys) == 0:
				return n
			case n < 1:
				return phys[0]
			case n > len(phys):
				return phys[len(phys)-1]
			}
			return phys[n-1]
		}
		if firstLine == 0 && len(phys) > 0 {
			firstLine = phys[0]
		}
		if len(phys) > 0 {
			lastLine = phys[len(phys)-1]
		}
		for _, n := range res {
			if k, ok := notebookKind(n.Kind); ok {
				n.Kind = k
			}
			if n.Kind == "JupyterModule" {
				if module == nil {
					m := n
					module = &m
				}
				for _, call := range n.Calls {
					moduleCalls[call] = true
				}
				continue
			}
			n.Line = at(n.Line)
			if n.EndLine > 0 {
				n.EndLine = at(n.EndLine)
			}
			nodes = append(nodes, n)
		}
	}
	if module != nil {
		m := *module
		m.Name, m.Line, m.EndLine = notebookModuleName, firstLine, lastLine
		m.Calls = m.Calls[:0:0]
		for call := range moduleCalls {
			m.Calls = append(m.Calls, call)
		}
		sort.Strings(m.Calls)
		nodes = append([]core.SymbolNode{m}, nodes...)
	}
	return nodes, nil
}
