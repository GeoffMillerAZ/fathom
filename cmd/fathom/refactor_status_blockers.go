package main

// refactor_status_blockers.go — CX-11w (design §4, U9): `fathom refactor
// status --migration` names each REMAINING site's blocker beside the site,
// through review.MigrationBlockers (CX-11). The library only matches facts;
// this file resolves them per repository:
//
//   - generated: the file carries a Go `// Code generated ... DO NOT EDIT.`
//     header (or any language's marker, connector.IsGenerated — already
//     computed on every SourceFile), or the repository's root
//     `.gitattributes` marks it `linguist-generated`. Nested .gitattributes
//     files are not read.
//   - governed: a layer rule in the repository's `.fathom/guard.cue`, of kind
//     import_direction or boundary, that forbids the file's package the
//     migration's To — the file is in the rule's `from` (or outside its
//     `allow`) and the To lives in a `to` (or `guarded`) directory. The rule
//     id is the citation. Package identity is the repo-relative directory,
//     as everywhere else layer rules are evaluated.
//
// Where the To lives: an import migration's To is a directory of the
// repository (as written, or relative to its go.mod module); a symbol
// migration's To is the directory of each indexed declaration of that name.
// A route, config or pkg migration names no package, so only generated code
// can block it. A To that resolves to no directory (a symbol not declared
// yet, an import outside the module) blocks nothing by a rule: a rule can
// only forbid a place it can name.

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/pathglob"
	"github.com/geoffmilleraz/fathom/internal/review"
)

// migrationBlockerFacts is what one service contributes to
// review.MigrationBlockers.
type migrationBlockerFacts struct {
	Governed  []review.GovernedSite
	Generated []review.GeneratedFile
	// Notes are named degradations (unreadable rules), never silent gaps.
	Notes []string
}

// migrationBlockerFactsFor resolves the generated and governed facts for
// remaining, one service's still-on-From sites. root is the service's
// recorded corpus root ("" when it has none: only the marker in the source
// can say a file is generated then, and no rule is read).
func migrationBlockerFactsFor(svc, root string, m review.Migration, remaining []review.MigrationSite, sources []core.SourceFile, nodes []core.SymbolNode) migrationBlockerFacts {
	var facts migrationBlockerFacts
	files := remainingFiles(remaining)
	if len(files) == 0 {
		return facts
	}

	marked := make(map[string]bool)
	for _, sf := range sources {
		if sf.Generated {
			marked[sf.Path] = true
		}
	}
	var attrs []gitAttrRule
	if root != "" {
		if data, err := os.ReadFile(filepath.Join(root, ".gitattributes")); err == nil {
			attrs = parseLinguistGenerated(data)
		}
	}
	for _, f := range files {
		if marked[f] || linguistGenerated(attrs, f) {
			facts.Generated = append(facts.Generated, review.GeneratedFile{Repository: svc, File: f})
		}
	}

	if root == "" {
		return facts
	}
	guard := filepath.Join(root, ".fathom", "guard.cue")
	if _, err := os.Stat(guard); err != nil {
		return facts // no rules file: nothing governs
	}
	rules, err := guardspec.LoadLayerRules(guard)
	if err != nil {
		facts.Notes = append(facts.Notes, fmt.Sprintf("%s: layer rules unreadable, no site checked against them: %v", svc, err))
		return facts
	}
	targets := migrationTargetDirs(m, root, nodes)
	if len(rules.Rules) == 0 || len(targets) == 0 {
		return facts
	}
	for _, f := range files {
		if rule := governingLayerRule(rules.Rules, path.Dir(f), targets); rule != "" {
			facts.Governed = append(facts.Governed, review.GovernedSite{Repository: svc, File: f, Rule: rule})
		}
	}
	return facts
}

// remainingFiles is the sorted, distinct files of the sites still on From.
func remainingFiles(sites []review.MigrationSite) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range sites {
		if !s.OnTarget && !seen[s.File] {
			seen[s.File] = true
			out = append(out, s.File)
		}
	}
	sort.Strings(out)
	return out
}

// migrationTargetDirs is where m's To lives, as repo-relative directories.
func migrationTargetDirs(m review.Migration, root string, nodes []core.SymbolNode) []string {
	seen := map[string]bool{}
	add := func(d string) {
		if d != "" {
			seen[path.Clean(d)] = true
		}
	}
	switch m.Kind {
	case review.MigrationImport:
		add(m.To)
		if mod := goModulePath(root); mod != "" {
			switch {
			case m.To == mod:
				add(".")
			case strings.HasPrefix(m.To, mod+"/"):
				add(strings.TrimPrefix(m.To, mod+"/"))
			}
		}
	case review.MigrationSymbol:
		bare := bareSymbolName(m.To)
		for _, n := range nodes {
			if n.Name == bare && n.FilePath != "" {
				add(path.Dir(n.FilePath))
			}
		}
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// goModulePath is root's go.mod module path, or "".
func goModulePath(root string) string {
	if root == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range bytes.Split(data, []byte("\n")) {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(string(line)), "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`)
		}
	}
	return ""
}

// governingLayerRule names the rule that forbids the package in fileDir the
// places in targets: an import_direction rule with the file in `from` and a
// target in `to`, or a boundary rule with the file outside `allow` and a
// target in `guarded`. An error-severity rule is preferred to a warn one
// (which is named as such), then the lowest id; "" when none forbids it.
func governingLayerRule(rules []guardspec.LayerRule, fileDir string, targets []string) string {
	sorted := append([]guardspec.LayerRule(nil), rules...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	warn := ""
	for _, r := range sorted {
		if !ruleForbids(r, fileDir, targets) {
			continue
		}
		if r.Severity != guardspec.SeverityWarn {
			return r.ID
		}
		if warn == "" {
			warn = r.ID + " (warn)"
		}
	}
	return warn
}

func ruleForbids(r guardspec.LayerRule, fileDir string, targets []string) bool {
	for _, t := range targets {
		if t == fileDir {
			continue // a package does not import itself
		}
		switch r.Kind {
		case guardspec.ImportDirection:
			if pathglob.MatchAny(r.From, fileDir) && pathglob.MatchAny(r.To, t) {
				return true
			}
		case guardspec.Boundary:
			if !pathglob.MatchAny(r.Allow, fileDir) && pathglob.MatchAny(r.Guarded, t) {
				return true
			}
		}
	}
	return false
}

// gitAttrRule is one .gitattributes line that sets or unsets
// linguist-generated for a pattern.
type gitAttrRule struct {
	pattern   string
	generated bool
}

// parseLinguistGenerated reads the lines of a .gitattributes that set
// (`linguist-generated`, `=true`) or unset (`-linguist-generated`,
// `!linguist-generated`, `=false`) the attribute, in file order.
func parseLinguistGenerated(data []byte) []gitAttrRule {
	var out []gitAttrRule
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(fields[0], "[attr]") {
			continue
		}
		state, set := false, false
		for _, a := range fields[1:] {
			switch a {
			case "linguist-generated", "linguist-generated=true":
				state, set = true, true
			case "-linguist-generated", "!linguist-generated", "linguist-generated=false":
				state, set = false, true
			}
		}
		if set {
			out = append(out, gitAttrRule{pattern: fields[0], generated: state})
		}
	}
	return out
}

// linguistGenerated reports whether file (repo-relative, slash-separated) is
// marked generated: the last matching line wins. A pattern with no slash
// matches the file name at any depth; one with a slash is anchored at the
// root and may use "**". A trailing-slash pattern names a directory, which
// gitattributes never matches.
func linguistGenerated(rules []gitAttrRule, file string) bool {
	generated := false
	for _, r := range rules {
		if strings.HasSuffix(r.pattern, "/") {
			continue
		}
		p := strings.TrimPrefix(r.pattern, "/")
		var hit bool
		if p != r.pattern || strings.Contains(p, "/") {
			hit = pathglob.Match(p, file)
		} else {
			hit, _ = path.Match(p, path.Base(file))
		}
		if hit {
			generated = r.generated
		}
	}
	return generated
}
