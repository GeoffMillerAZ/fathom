package deps

import (
	"context"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// Only module-scope statements may declare a binding. The proof also checks
// every statement, since an unrelated executable statement can mutate imports.
const pythonDeclarationQuery = `
(module (_) @top)
[(ERROR) (MISSING)] @error
(expression_statement . (_) @expression .) @expression-statement
(assignment !type left: (identifier) @binding right: (_) @value) @assignment
(import_statement name: (_) @operand) @import
(import_from_statement module_name: (_) @module name: (_) @operand) @from
(aliased_import name: (dotted_name) @name alias: (identifier) @alias) @aliased
(_) @node
`

type pythonDeclarationNode struct {
	kind string
	tsengine.Node
}

type pythonDeclarationOperand struct {
	name, alias, module string
}

func pythonDeclarations(ctx context.Context, src []byte) map[string]bool {
	if !utf8.Valid(src) || strings.HasPrefix(string(src), "\ufeff") || strings.ContainsRune(string(src), 0) ||
		strings.Contains(strings.ReplaceAll(string(src), "\r\n", ""), "\r") {
		return nil
	}
	matches, err := pythonQueryMatches(ctx, src, "dependency-python-declarations", pythonDeclarationQuery, true)
	if err != nil {
		return nil
	}
	var top, nodes []pythonDeclarationNode
	expressions := make(map[tsengine.Node]pythonDeclarationNode)
	assignments := make(map[tsengine.Node][]tsengine.Capture)
	imports := make(map[tsengine.Node][]tsengine.Node)
	modules := make(map[tsengine.Node]string)
	aliases := make(map[tsengine.Node]pythonDeclarationOperand)
	seenLine := -1
	for _, match := range matches {
		first := match.Captures[0]
		switch first.Name {
		case "error":
			return nil
		case "top":
			if first.Type == "comment" {
				if first.Node.Line() <= 2 && strings.Contains(first.Node.Text(src), "coding") {
					return nil
				}
				continue
			}
			// The grammar can recover file-level indentation without an ERROR.
			line := int(first.Node.StartRow)
			if line != seenLine && !pythonAtColumnZero(src, first.Node.StartByte) {
				return nil
			}
			seenLine = line
			top = append(top, pythonDeclarationNode{first.Type, first.Node})
		case "node":
			nodes = append(nodes, pythonDeclarationNode{first.Type, first.Node})
		case "expression-statement":
			child := match.Captures[1]
			expressions[first.Node] = pythonDeclarationNode{child.Type, child.Node}
		case "assignment":
			assignments[first.Node] = match.Captures[1:]
		case "import", "from":
			operand := match.Captures[1]
			if first.Name == "from" {
				modules[first.Node] = pythonCleanModule(operand.Node.Text(src))
				operand = match.Captures[2]
			}
			imports[first.Node] = append(imports[first.Node], operand.Node)
		case "aliased":
			aliases[first.Node] = pythonDeclarationOperand{name: pythonCleanModule(match.Captures[1].Node.Text(src)), alias: match.Captures[2].Node.Text(src)}
		}
	}

	slices.SortFunc(nodes, func(a, b pythonDeclarationNode) int { return int(a.StartByte) - int(b.StartByte) })
	bindings := make(map[string]bool)
	bind := func(name string) bool {
		if !pythonIdentifier(name) || pythonReservedDeclaration(name) || bindings[name] {
			return false
		}
		bindings[name] = true
		return true
	}
	for _, stmt := range top {
		if ctx.Err() != nil {
			return nil
		}
		switch stmt.kind {
		case "pass_statement":
		case "import_statement", "import_from_statement":
			operands := imports[stmt.Node]
			if len(operands) == 0 {
				return nil
			}
			for _, operand := range operands {
				decl, aliased := aliases[operand]
				if !aliased {
					decl.name = pythonCleanModule(operand.Text(src))
				}
				from := stmt.kind == "import_from_statement"
				decl.module = modules[stmt.Node]
				if !pythonDeclarativeImport(decl, from) {
					return nil
				}
				name := decl.alias
				if !aliased {
					name, _, _ = strings.Cut(decl.name, ".")
				}
				if !bind(name) {
					return nil
				}
			}
		case "expression_statement":
			value, ok := expressions[stmt.Node]
			if !ok {
				return nil
			}
			bare := value.kind != "assignment"
			for value.kind == "assignment" {
				parts := assignments[value.Node]
				if len(parts) != 2 || !bind(parts[0].Node.Text(src)) {
					return nil
				}
				value = pythonDeclarationNode{parts[1].Type, parts[1].Node}
			}
			if !pythonLiteral(src, value, nodes, bare) {
				return nil
			}
		default:
			return nil
		}
	}
	return bindings
}

func pythonAtColumnZero(src []byte, offset uint32) bool {
	return offset == 0 || src[offset-1] == '\n'
}

func pythonCleanModule(raw string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(strings.ReplaceAll(raw, "\\\r\n", ""), "\\\n", "")), "")
}

func pythonIdentifier(name string) bool {
	if name == "" || strings.Contains(" False None True and as assert async await break class continue def del elif else except finally for from global if import in is lambda nonlocal not or pass raise return try while with yield ", " "+name+" ") {
		return false
	}
	for i, b := range []byte(name) {
		if b != '_' && (b < 'a' || b > 'z') && (b < 'A' || b > 'Z') && (i == 0 || b < '0' || b > '9') {
			return false
		}
	}
	return true
}

func pythonReservedDeclaration(name string) bool {
	switch name {
	case "__builtins__", "__cached__", "__class__", "__debug__", "__dict__", "__doc__", "__file__", "__getattr__",
		"__import__", "__loader__", "__name__", "__package__", "__path__", "__spec__", "declare_namespace",
		"eval", "exec", "exec_module", "extend_path", "getattr", "globals", "import_module", "load_module",
		"locals", "meta_path", "path_hooks", "path_importer_cache", "setattr", "spec_from_file_location":
		return true
	}
	return false
}

func pythonDeclarativeImport(operand pythonDeclarationOperand, from bool) bool {
	module := operand.name
	if from {
		if !pythonIdentifier(operand.name) || pythonReservedDeclaration(operand.name) || operand.module == "" {
			return false
		}
		module = strings.TrimLeft(operand.module, ".")
		if operand.module == "sys" && (operand.name == "path" || operand.name == "modules") {
			return false
		}
	}
	head, _, _ := strings.Cut(module, ".")
	switch head {
	case "__future__", "builtins", "imp", "importlib", "pkg_resources", "pkgutil", "runpy", "zipimport":
		return false
	}
	if module == "" {
		return from
	}
	for part := range strings.SplitSeq(module, ".") {
		if !pythonIdentifier(part) || pythonReservedDeclaration(part) {
			return false
		}
	}
	return true
}

var pythonNumber = regexp.MustCompile(`(?i)^(?:(?:0x(?:_?[a-f0-9])+|0o(?:_?[0-7])+|0b(?:_?[01])+)|(?:0(?:_?0)*|[1-9](?:_?[0-9])*)|(?:[0-9](?:_?[0-9])*(?:\.(?:[0-9](?:_?[0-9])*)?)?|\.[0-9](?:_?[0-9])*)(?:e[+-]?[0-9](?:_?[0-9])*)?j?)$`)

func pythonLiteral(src []byte, value pythonDeclarationNode, nodes []pythonDeclarationNode, bare bool) bool {
	switch value.kind {
	case "integer", "float", "string", "concatenated_string", "none", "true", "false", "ellipsis":
	case "tuple", "list", "dictionary", "set", "parenthesized_expression", "unary_operator":
		if bare {
			return false
		}
	default:
		return false
	}
	first := sort.Search(len(nodes), func(i int) bool { return nodes[i].StartByte >= value.StartByte })
	for _, n := range nodes[first:] {
		if n.StartByte >= value.EndByte {
			break
		}
		if n.EndByte > value.EndByte {
			continue
		}
		if n.Node == value.Node && (n.kind == "module" || n.kind == "expression_statement") {
			continue
		}
		switch n.kind {
		case "integer", "float":
			number := n.Text(src)
			if len(number) > 4300 || !pythonNumber.MatchString(number) {
				return false
			}
			// Python rejects decimal integers with a nonzero leading-zero value.
			if len(number) > 1 && number[0] == '0' && !strings.ContainsAny(number, "xXoObB.eEjJ") &&
				strings.Trim(number, "0_") != "" {
				return false
			}
		case "string_start":
			prefix := strings.TrimRight(strings.ToLower(n.Text(src)), "\"'")
			if prefix != "" && prefix != "r" && prefix != "u" {
				return false
			}
		case "string_content":
			if strings.ContainsRune(n.Text(src), '\\') {
				return false
			}
		case "unary_operator":
			raw := n.Text(src)
			if len(raw) < 2 {
				return false
			}
			operand := strings.TrimSpace(raw[1:])
			if operand == "" || !strings.ContainsRune("+-", rune(raw[0])) || !strings.ContainsRune("0123456789.", rune(operand[0])) {
				return false
			}
		case "comment", "string", "concatenated_string", "string_end", "none", "true", "false", "ellipsis",
			"tuple", "list", "dictionary", "pair", "set", "parenthesized_expression":
		default:
			return false
		}
	}
	return true
}
