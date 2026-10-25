package extractor

// tsextract_cudamacros.go — the CUDA dialect's pre-parse step (the cuda
// dialect.json's preprocess key "cuda-macros", LB-Q-cuda). CUDA's headers spell
// alignment as a function-like macro the compiler expands and tree-sitter-cuda
// cannot read between a type and its name: `__shared__ int __align__(8) s[16];`
// and `typedef struct __align__(16) { float x, y; } Pair;` parse as errors.
//
// `if (typeid(Real) == typeid(double))` fails the same way: the grammar reads a
// call's arguments as expressions, so a builtin type name inside typeid( ) is an
// error, while sizeof( ) takes a type. The step reads `typeid` as `sizeof`, the
// same six bytes.
//
// An explicit instantiation (`template class PiEstimator<float>;`) is an error
// to the grammar too, and defines nothing, so the step blanks the statement.
//
// The step blanks each `__align__(...)` (the word and its balanced argument
// list, found by cppLex so a comment or string never matches) and reads each
// `typeid` as `sizeof`, at equal byte length, so every line, column and byte
// offset is unchanged, and then runs the C++ step (cppMacroTexts) over that
// text, since a CUDA file holds the same export and inline macros a C++ file does. The first text that parses with no
// error is read (tsPreprocessed), so a file that parses as written is never
// rewritten.

// cudaMacroTexts is the text list tsPreprocessors["cuda-macros"] returns for a
// CUDA file that does not parse as written: the C++ step's text over src with
// its __align__ qualifiers blanked and typeid read as sizeof, else src with
// only those changes; nil when nothing changes.
func cudaMacroTexts(src []byte) [][]byte {
	base, changed := cudaRewrite(src)
	if !changed {
		return cppMacroTexts(src)
	}
	if out := cppMacroTexts(base); out != nil {
		return append(out, base)
	}
	return [][]byte{base}
}

// cudaRewrite is src with every `__align__(...)` blanked to spaces (newlines
// kept) and every `typeid` word read as `sizeof`; changed is false when src
// has neither.
func cudaRewrite(src []byte) (out []byte, changed bool) {
	toks, _ := cppLex(src)
	for i := 0; i+1 < len(toks); i++ {
		t := toks[i]
		if t.kind == 'w' && string(src[t.start:t.end]) == "typeid" {
			if out == nil {
				out = append([]byte(nil), src...)
			}
			copy(out[t.start:t.end], "sizeof")
			continue
		}
		if t.kind == 'w' && string(src[t.start:t.end]) == "template" && i+2 < len(toks) &&
			toks[i+1].kind == 'w' && (string(src[toks[i+1].start:toks[i+1].end]) == "class" || string(src[toks[i+1].start:toks[i+1].end]) == "struct") {
			end := -1
			for j := i + 2; j < len(toks); j++ {
				if toks[j].kind != 'p' {
					continue
				}
				if c := string(src[toks[j].start:toks[j].end]); c == ";" {
					end = toks[j].end
					break
				} else if c == "{" {
					break
				}
			}
			if end > 0 {
				if out == nil {
					out = append([]byte(nil), src...)
				}
				from := t.start
				if i > 0 && toks[i-1].kind == 'w' && string(src[toks[i-1].start:toks[i-1].end]) == "extern" {
					from = toks[i-1].start
				}
				for k := from; k < end; k++ {
					if out[k] != '\n' && out[k] != '\r' {
						out[k] = ' '
					}
				}
			}
			continue
		}
		if t.kind != 'w' || string(src[t.start:t.end]) != "__align__" {
			continue
		}
		open := toks[i+1]
		if open.kind != 'p' || string(src[open.start:open.end]) != "(" {
			continue
		}
		depth, end := 0, -1
		for j := i + 1; j < len(toks) && end < 0; j++ {
			if toks[j].kind != 'p' {
				continue
			}
			switch string(src[toks[j].start:toks[j].end]) {
			case "(":
				depth++
			case ")":
				depth--
				if depth == 0 {
					end = toks[j].end
				}
			}
		}
		if end < 0 {
			continue
		}
		if out == nil {
			out = append([]byte(nil), src...)
		}
		for k := t.start; k < end; k++ {
			if out[k] != '\n' && out[k] != '\r' {
				out[k] = ' '
			}
		}
	}
	return out, out != nil
}
