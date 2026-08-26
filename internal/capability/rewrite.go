package capability

import (
	"fmt"
	"strconv"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/parser"
)

// SpliceSettingsBlock returns data with its single top-level `capabilities`
// declaration replaced by block, every other byte of the file untouched:
// the `budgets` block (BA-9), comments, an all-commented scaffold, a
// package clause — none of it is re-rendered, so none of it can be lost.
// When the file has no `capabilities` declaration the block is appended
// after a blank line. block is written verbatim, without a trailing
// newline added at the replace site (the bytes after the old declaration
// — usually "\n" — follow it as they were).
//
// Two shapes are refused rather than approximated, with the manual edit
// named: a file that does not parse, and a file whose `capabilities` the
// writer cannot see as one top-level declaration (two declarations that
// CUE would unify, or a value reached through an embedding or a
// reference). A rewrite that guessed at either would drop lines quietly,
// which is the defect class this package exists to remove.
func SpliceSettingsBlock(path string, data, block []byte) ([]byte, error) {
	f, err := parser.ParseFile(path, data, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("%s does not parse (%v) — fix it by hand before `fathom capabilities set` can rewrite its capabilities block", path, err)
	}
	start, end := -1, -1
	for _, d := range f.Decls {
		fld, ok := d.(*ast.Field)
		if !ok {
			continue
		}
		name, _, lerr := ast.LabelName(fld.Label)
		if lerr != nil || name != "capabilities" {
			continue
		}
		if start >= 0 {
			return nil, fmt.Errorf("%s has more than one top-level `capabilities` block — merge them into one by hand before `fathom capabilities set` can rewrite it", path)
		}
		start, end = fld.Pos().Offset(), fld.End().Offset()
	}
	if start < 0 {
		// No declaration the writer can replace. If CUE still resolves a
		// `capabilities` value — an embedding, a reference — appending a
		// second one would unify with it, and the next load would either
		// conflict or silently combine two sources. Refuse instead.
		v := cuecontext.New().CompileBytes(data, cue.Filename(path))
		if v.Err() == nil && v.LookupPath(cue.ParsePath("capabilities")).Exists() {
			return nil, fmt.Errorf("%s defines `capabilities` somewhere other than a plain top-level `capabilities: {…}` block — move it to one by hand before `fathom capabilities set` can rewrite it", path)
		}
		out := make([]byte, 0, len(data)+len(block)+2)
		out = append(out, data...)
		if len(out) > 0 {
			if out[len(out)-1] != '\n' {
				out = append(out, '\n')
			}
			out = append(out, '\n')
		}
		out = append(out, block...)
		out = append(out, '\n')
		return out, nil
	}
	out := make([]byte, 0, len(data)-(end-start)+len(block))
	out = append(out, data[:start]...)
	out = append(out, block...)
	out = append(out, data[end:]...)
	return out, nil
}

// FieldLabel spells a capability id as a CUE field label: bare when it is
// a valid identifier, quoted otherwise. "verify-gate" is a registered id
// and not an identifier — written bare it is a syntax error, and a file
// `set` writes that does not load back is the worst outcome the verb has.
func FieldLabel(id string) string {
	if ast.IsValidIdent(id) {
		return id
	}
	return strconv.Quote(id)
}
