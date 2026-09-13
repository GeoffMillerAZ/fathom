package extractor

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// CMakeExtractor implements Extractor for CMakeLists.txt and *.cmake module
// files (docs/plans/cpp-toolchain.md CT-4). CMake is the intent layer of a
// C++ build — build.ninja is generated FROM it and is never indexed (CT-9)
// — so targets, not build steps, are the symbols.
//
// It uses a hand-written command scanner (no external dependency): a CMake
// file is a flat sequence of `name(args…)` invocations, with `#` line
// comments between them and quoted arguments that may contain anything.
// Emitted:
//
//   - KindCMakeProject     — project(Name …)
//   - KindCMakeTarget      — add_executable / add_library / add_custom_target;
//     Signature records the flavor ("executable", "library STATIC", "custom").
//     Calls carries target_link_libraries edges (link keywords excluded,
//     "::" namespaced targets normalized to "." per CT-1, variable refs
//     dropped as unresolvable).
//   - KindCMakeFunction    — function(name …) and macro(name …)
//   - KindCMakeOption      — option(NAME …)
//   - KindCMakeDependency  — find_package(Pkg …)
//
// All symbols carry TierPreScan: this is a heuristic scan, not CMake's own
// evaluator — variable indirection and generator expressions are dropped,
// never guessed. Known blind spot: #[[ bracket comments ]] are not
// recognized; a command invocation inside one would extract. Extract never
// returns a non-nil error.
type CMakeExtractor struct{}

// NewCMakeExtractor returns a new CMakeExtractor.
func NewCMakeExtractor() *CMakeExtractor { return &CMakeExtractor{} }

// cmakeCommand is one scanned `name(args…)` invocation.
type cmakeCommand struct {
	name    string   // lower-cased command name
	args    []string // split on whitespace, quotes stripped
	line    int      // 1-based line of the command name
	endLine int      // 1-based line of the invocation's closing paren
}

// Extract scans src and returns SymbolNodes sorted by Line then Name.
func (c *CMakeExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	pkg := pkgFromPath(relPath)
	cmds := scanCMakeCommands(string(src))

	var nodes []core.SymbolNode
	targetIdx := map[string]int{} // target name → index into nodes
	links := map[string][]string{}

	add := func(name string, kind core.SourceKind, line, endLine int, sig string) int {
		nodes = append(nodes, core.SymbolNode{
			Service:   service,
			Package:   pkg,
			Name:      name,
			Kind:      kind,
			FilePath:  relPath,
			Line:      line,
			EndLine:   endLine,
			Tier:      core.TierPreScan,
			Signature: sig,
		})
		return len(nodes) - 1
	}

	for _, cmd := range cmds {
		if len(cmd.args) == 0 {
			continue
		}
		first := cmd.args[0]
		switch cmd.name {
		case "project":
			add(first, core.KindCMakeProject, cmd.line, cmd.endLine, "")
		case "option":
			add(first, core.KindCMakeOption, cmd.line, cmd.endLine, "")
		case "find_package":
			add(first, core.KindCMakeDependency, cmd.line, cmd.endLine, "")
		case "add_executable":
			targetIdx[first] = add(first, core.KindCMakeTarget, cmd.line, cmd.endLine, "executable")
		case "add_library":
			sig := "library"
			if len(cmd.args) > 1 && cmakeLibraryType(cmd.args[1]) {
				sig = "library " + cmd.args[1]
			}
			targetIdx[first] = add(first, core.KindCMakeTarget, cmd.line, cmd.endLine, sig)
		case "add_custom_target":
			targetIdx[first] = add(first, core.KindCMakeTarget, cmd.line, cmd.endLine, "custom")
		case "function", "macro":
			add(first, core.KindCMakeFunction, cmd.line, cmd.endLine, "")
		case "target_link_libraries":
			if strings.Contains(first, "$") {
				continue // variable indirection: unresolvable, dropped not guessed
			}
			for _, a := range cmd.args[1:] {
				if cmakeLinkKeyword(a) || strings.Contains(a, "$") {
					continue
				}
				links[first] = append(links[first], strings.ReplaceAll(a, "::", "."))
			}
		}
	}

	// Attach link edges to their targets. A link for a target not defined in
	// this file (add_subdirectory splits) has nowhere to attach and is
	// dropped — calibration measures the cost before anything fancier.
	for target, callees := range links {
		i, ok := targetIdx[target]
		if !ok {
			continue
		}
		seen := map[string]struct{}{}
		var calls []string
		for _, c := range callees {
			if _, dup := seen[c]; dup {
				continue
			}
			seen[c] = struct{}{}
			calls = append(calls, c)
		}
		sort.Strings(calls)
		nodes[i].Calls = calls
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

// cmakeLibraryType reports whether s is an add_library type keyword.
func cmakeLibraryType(s string) bool {
	switch s {
	case "STATIC", "SHARED", "MODULE", "OBJECT", "INTERFACE", "ALIAS", "IMPORTED", "UNKNOWN":
		return true
	}
	return false
}

// cmakeLinkKeyword reports whether s is a target_link_libraries keyword
// rather than a library reference.
func cmakeLinkKeyword(s string) bool {
	switch s {
	case "PRIVATE", "PUBLIC", "INTERFACE",
		"LINK_PRIVATE", "LINK_PUBLIC", "LINK_INTERFACE_LIBRARIES",
		"debug", "optimized", "general":
		return true
	}
	return false
}

// scanCMakeCommands walks src as a flat sequence of command invocations.
// Between commands, `#` starts a line comment. Inside an argument list,
// double-quoted spans keep their content (including whitespace, parens and
// `#`), an unquoted `#` still starts a comment, and parens nest (generator
// expressions, condition groups).
func scanCMakeCommands(src string) []cmakeCommand {
	var out []cmakeCommand
	line := 1
	i := 0
	n := len(src)
	for i < n {
		ch := src[i]
		switch {
		case ch == '\n':
			line++
			i++
		case ch == '#':
			for i < n && src[i] != '\n' {
				i++
			}
		case identByte(ch) && !(ch >= '0' && ch <= '9'):
			start := i
			for i < n && identByte(src[i]) {
				i++
			}
			name := src[start:i]
			nameLine := line
			// Optional whitespace (not newlines — CMake allows them, but a
			// name/paren split across lines is vanishingly rare and skipping
			// it keeps line accounting simple).
			j := i
			for j < n && (src[j] == ' ' || src[j] == '\t') {
				j++
			}
			if j >= n || src[j] != '(' {
				continue // bare identifier, not an invocation
			}
			args, next, nl := scanCMakeArgs(src, j+1)
			out = append(out, cmakeCommand{name: strings.ToLower(name), args: args, line: nameLine, endLine: nameLine + nl})
			line += nl
			i = next
		default:
			i++
		}
	}
	return out
}

// scanCMakeArgs collects the argument list starting just past the opening
// paren. Returns the split args, the index just past the closing paren, and
// how many newlines were consumed.
func scanCMakeArgs(src string, from int) (args []string, next int, newlines int) {
	var (
		cur     strings.Builder
		haveCur bool
		depth   = 1
		inQuote bool
		i       = from
		n       = len(src)
	)
	flush := func() {
		if haveCur {
			args = append(args, cur.String())
			cur.Reset()
			haveCur = false
		}
	}
	for i < n {
		ch := src[i]
		if inQuote {
			switch ch {
			case '\\':
				if i+1 < n {
					cur.WriteByte(src[i+1])
					if src[i+1] == '\n' {
						newlines++
					}
					i += 2
					continue
				}
			case '"':
				inQuote = false
			case '\n':
				newlines++
				cur.WriteByte(ch)
			default:
				cur.WriteByte(ch)
			}
			i++
			continue
		}
		switch ch {
		case '"':
			inQuote = true
			haveCur = true
		case '(':
			depth++
			cur.WriteByte(ch)
			haveCur = true
		case ')':
			depth--
			if depth == 0 {
				flush()
				return args, i + 1, newlines
			}
			cur.WriteByte(ch)
		case '#':
			for i < n && src[i] != '\n' {
				i++
			}
			continue
		case ' ', '\t':
			flush()
		case '\n':
			newlines++
			flush()
		default:
			cur.WriteByte(ch)
			haveCur = true
		}
		i++
	}
	flush()
	return args, n, newlines
}
