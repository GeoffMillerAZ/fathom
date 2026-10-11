package mcp

// file_role.go — the role facet served (RF-4, docs/plans/repository-facts.md;
// vision 42 D2): `search` names each hit's file role and basis, and
// `symbol_context` on a file path answers the file's role block. No new
// tool and no new parameter (RF-X4): both read repofacts.Role on the file
// as it is on disk, computed on read (no store column, RF-D1), and every
// answer that uses the role prints how many of its files the rules left
// `unknown` (I65).

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/repofacts"
)

// fileRoleDTO is symbol_context's `file` block for a file-path query.
type fileRoleDTO struct {
	Path  string `json:"path"`
	Role  string `json:"role"`
	Basis string `json:"basis,omitempty"`
	// RoleUnknown counts the answer's files no rule decided: 1 when this
	// file's role is unknown, else 0, always printed (I65).
	RoleUnknown int `json:"role_unknown"`
	// Unread names a .gitattributes on the file's chain that exists and
	// could not be read: its declarations did not decide (I38).
	Unread []string `json:"unread,omitempty"`
}

// roleDialectRules is every embedded dialect manifest's match block, built
// once from the extractor's embedded manifests the way fusion's dialect
// gate builds it: the file's dialect is repofacts' dialect basis input.
var roleDialectRules = sync.OnceValue(func() []connector.DialectRule {
	ds, err := extractor.LoadDialects()
	if err != nil {
		return nil
	}
	rules := make([]connector.DialectRule, 0, len(ds))
	for _, d := range ds {
		r := connector.DialectRule{ID: d.ID, Basenames: d.Match.Basenames, Globs: d.Match.Globs, Shebangs: d.Match.Shebangs}
		r.Extensions = append(r.Extensions, d.Match.Extensions...)
		for _, v := range d.Variants {
			r.Extensions = append(r.Extensions, v.Extensions...)
		}
		if s := d.Match.Sniff; s != nil {
			r.Sniff = &connector.DialectSniffRule{Extensions: s.Extensions, Pattern: s.Pattern, Priority: s.Priority}
		}
		rules = append(rules, r)
	}
	return rules
})

// fileDialect is the file's dialect id by the manifests' match rules over
// its path and head, else connector's language tag ("go" has no manifest).
func fileDialect(rel string, head []byte) string {
	if v := connector.ClassifyDialect(roleDialectRules(), rel, head); v.Dialect != "" {
		return v.Dialect
	}
	if lang := connector.LanguageForPath(rel); lang != core.LangUnclassified {
		return lang
	}
	return ""
}

// readHead reads up to repofacts.HeaderBytes of root/rel; nil when the file
// cannot be read (the header basis then decides nothing).
func readHead(root, rel string) []byte {
	if root == "" {
		return nil
	}
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return nil
	}
	defer f.Close()
	buf := make([]byte, repofacts.HeaderBytes)
	n, _ := io.ReadFull(f, buf)
	return buf[:n]
}

// fileRoleBlock classifies one file under root (a service's corpus root,
// or search's root; "" when unknown, leaving the path and dialect bases
// alone to decide): its dialect, the .gitattributes chain from root down,
// and its head bytes, through repofacts.Role. head may be passed when the
// caller already read it (nil reads it here).
func fileRoleBlock(root, rel string, head []byte) (repofacts.Fact, repofacts.Attributes) {
	if head == nil {
		head = readHead(root, rel)
	}
	var attrs repofacts.Attributes
	if root != "" {
		attrs = repofacts.ReadAttributes(os.DirFS(root), rel)
	}
	return repofacts.Role(rel, fileDialect(rel, head), attrs, head), attrs
}

// fileRoleDTOFor renders fileRoleBlock as symbol_context's `file` block.
func fileRoleDTOFor(root, rel string) fileRoleDTO {
	fact, attrs := fileRoleBlock(root, rel, nil)
	out := fileRoleDTO{Path: rel, Role: string(fact.Role), Basis: string(fact.Basis), Unread: attrs.Unread}
	if fact.Role == repofacts.Unknown {
		out.RoleUnknown = 1
	}
	return out
}

// symbolContextFilePath reports whether query names a file of the service
// rather than a symbol: a file path some indexed symbol is declared in, or
// a regular file under the corpus root (a script, a config file, a doc that
// declares no symbol). It returns the cleaned slash path. A path that
// escapes the root, or reaches outside it through a symlink, is no file.
func symbolContextFilePath(nodes []core.SymbolNode, root, query string) (string, bool) {
	if query == "" || strings.Contains(query, "::") {
		return "", false
	}
	rel := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(filepath.FromSlash(query))), "./")
	for _, n := range nodes {
		if n.FilePath == rel {
			return rel, true
		}
	}
	if root == "" {
		return "", false
	}
	clean, full, err := validateRelPath(root, rel)
	if err != nil {
		return "", false
	}
	if _, err := resolveRealPath(root, full, rel); err != nil {
		return "", false
	}
	if info, err := os.Stat(full); err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return clean, true
}
