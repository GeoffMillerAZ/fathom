package extractor

// docdecode_compdb.go — the document engine's compile-commands decoder (LB-Q-compile-commands, design §4):
// a Clang compilation database is a JSON array of {directory, command | arguments, file, output}, and the
// flags a translation unit is compiled with live in one string (`command`) or one array (`arguments`) that
// a selector cannot read. This decoder reads the JSON as decodeJSONDoc does, then adds to every entry the
// three derived keys the dialect's doc.json selects, each value the flag and its argument written together
// (`-I/usr/include`, `-DNDEBUG`, `-oa.o`) so that a reference never reads as the name of a symbol:
//
//	includes  the include directories (-I, -iquote, -isystem, -idirafter), as the flag that named them
//	defines   the macros defined (-D), NAME or NAME=value
//	out       the object written: the `-o` argument (relative to the entry's directory, as the compiler reads it),
//	          else the entry's own `output` (CMake writes that one relative to the top build directory)
//
// The flag grammar is the one internal/fusion's includeRoots (resolvestrategy.go) reads compile_commands.json
// with: the arguments array, else the command split on whitespace (`strings.Fields`, no shell quoting), a
// surrounding quote pair trimmed from each token; -I joined (`-Idir`) or separate (`-I dir`); -iquote,
// -isystem and -idirafter separate only. -D is read like -I (joined or separate), -o separate only. A flag
// that ends the list (`-I` with nothing after it) names nothing. A file that is not a JSON array of objects
// decodes as the JSON it is and yields no unit.

import (
	"strings"
)

// compdbIncludeFlags are the include-directory flags includeRoots reads, in its order. Only -I is also
// read joined to its directory.
var compdbIncludeFlags = []string{"-I", "-iquote", "-isystem", "-idirafter"}

func decodeCompDB(src []byte) (*docNode, error) {
	n, err := decodeJSONDoc(src)
	if err != nil || n == nil || n.kind != docSeq {
		return n, err
	}
	for _, it := range n.items {
		if it.kind != docMap {
			continue
		}
		args := compdbArgs(it)
		includes, defines, out := compdbFlags(args)
		if o := it.get("output"); out == "" && o != nil && o.kind == docScalar {
			out = o.scalar // no -o flag: the entry's own output
		}
		it.set("includes", compdbSeq(includes, it.line))
		it.set("defines", compdbSeq(defines, it.line))
		if out != "" {
			it.set("out", &docNode{kind: docScalar, scalar: "-o" + out, line: it.line})
		}
	}
	return n, nil
}

// compdbArgs is an entry's argument list: `arguments` when it has items, else `command` split on whitespace.
func compdbArgs(entry *docNode) []string {
	if a := entry.get("arguments"); a != nil && a.kind == docSeq && len(a.items) > 0 {
		out := make([]string, 0, len(a.items))
		for _, it := range a.items {
			if it.kind == docScalar {
				out = append(out, it.scalar)
			}
		}
		return out
	}
	if c := entry.get("command"); c != nil && c.kind == docScalar {
		return strings.Fields(c.scalar)
	}
	return nil
}

// compdbFlags reads the include directories (each as the flag that named it, written joined: `-I/usr/include`),
// macro definitions (`-D` removed: NAME or NAME=value come back prefixed `-D`) and `-o` output of one argument
// list. Include directories and macros keep first-seen order and repeat nothing.
func compdbFlags(args []string) (includes, defines []string, out string) {
	seenInc, seenDef := map[string]bool{}, map[string]bool{}
	for i := 0; i < len(args); i++ {
		a := strings.Trim(args[i], `"'`)
		var dir, used string
		for _, flag := range compdbIncludeFlags {
			if a == flag && i+1 < len(args) {
				dir, used = strings.Trim(args[i+1], `"'`), flag
				i++
				break
			}
			if v, ok := strings.CutPrefix(a, flag); ok && v != "" && flag == "-I" {
				dir, used = v, flag
				break
			}
		}
		if dir != "" {
			if d := used + dir; !seenInc[d] {
				seenInc[d] = true
				includes = append(includes, d)
			}
			continue
		}
		switch {
		case a == "-D" && i+1 < len(args):
			i++
			if d := strings.Trim(args[i], `"'`); d != "" && !seenDef["-D"+d] {
				seenDef["-D"+d] = true
				defines = append(defines, "-D"+d)
			}
		case strings.HasPrefix(a, "-D") && len(a) > 2:
			// A joined macro keeps its own quotes (-DNAME="x"): only a token wrapped whole in a quote pair is trimmed.
			d := a
			if strings.HasPrefix(args[i], "-D") {
				d = args[i]
			}
			if !seenDef[d] {
				seenDef[d] = true
				defines = append(defines, d)
			}
		case a == "-o" && i+1 < len(args):
			i++
			out = strings.Trim(args[i], `"'`)
		}
	}
	return includes, defines, out
}

func compdbSeq(vals []string, line int) *docNode {
	n := &docNode{kind: docSeq, line: line}
	for _, v := range vals {
		n.items = append(n.items, &docNode{kind: docScalar, scalar: v, line: line})
	}
	return n
}
