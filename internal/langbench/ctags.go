package langbench

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// OracleDef is one definition an oracle reports.
type OracleDef struct {
	Path   string
	Name   string
	Line   int
	End    int // 0 when the oracle gives no end
	Kind   string
	Scoped bool // the definition is a member of a type (ctags scope other than a package/namespace/module)
}

// CtagsArgs is the tier-B invocation (§1.3): JSON output with line, kind
// name, scope and end, anonymous definitions dropped. The file list is
// passed with -L so the oracle reads exactly fathom's files (§1.4).
var CtagsArgs = []string{"--output-format=json", "--fields=+nKzSe", "--extras=-{anonymous}", "-f", "-"}

// ParseCtagsJSON reads universal-ctags JSON lines (pseudo-tags skipped).
// A malformed line is an error: a truncated oracle is never scored.
func ParseCtagsJSON(r io.Reader) ([]OracleDef, error) {
	var out []OracleDef
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var tag struct {
			Type      string `json:"_type"`
			Name      string `json:"name"`
			Path      string `json:"path"`
			Line      int    `json:"line"`
			Kind      string `json:"kind"`
			End       int    `json:"end"`
			Scope     string `json:"scope"`
			ScopeKind string `json:"scopeKind"`
		}
		if err := json.Unmarshal([]byte(line), &tag); err != nil {
			return nil, fmt.Errorf("ctags output line %d: %w", n, err)
		}
		if tag.Type != "tag" {
			continue
		}
		out = append(out, OracleDef{Path: tag.Path, Name: tag.Name, Line: tag.Line, End: tag.End, Kind: tag.Kind, Scoped: memberScope(tag.Scope, tag.ScopeKind)})
	}
	return out, sc.Err()
}

// memberScope reports whether a ctags scope makes the tag a member of a
// type: a package, namespace, module or file scope does not (a Go func's
// scope is its package).
func memberScope(scope, kind string) bool {
	if scope == "" {
		return false
	}
	switch kind {
	case "package", "packageName", "namespace", "module", "file":
		return false
	}
	return true
}
