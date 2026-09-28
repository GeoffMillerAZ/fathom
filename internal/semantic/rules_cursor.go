package semantic

// rules_cursor.go — GL-2 (docs/plans/governance-learning.md, design doc §3):
// Cursor's project rules, `.cursor/rules/*.mdc` — frontmatter
// (`description`, `globs`, `alwaysApply`) over a markdown body, one rule
// file per concern. The frontmatter decides scope the way Cursor itself
// applies the file:
//
//   - `alwaysApply: true` is repo scope — the model's proposed scope stands,
//     verified as for a root CLAUDE.md (alwaysApply wins over globs, as in
//     Cursor);
//   - `globs` become the claim scope, replacing the model's;
//   - neither is Cursor's on-request rule (attached only when the agent or
//     the user asks for it by name): compiled, but audience manual, so it is
//     not served unasked — the same treatment as Kiro's `inclusion: manual`.

import (
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// cursorRulesDir is Cursor's project rules directory.
const cursorRulesDir = ".cursor/rules"

type cursorRulesSource struct{}

func init() { RegisterRulesSource(cursorRulesSource{}) }

func (cursorRulesSource) ID() string { return "cursor" }

func (cursorRulesSource) Detect(root string) bool { return dirExists(root, cursorRulesDir) }

func (cursorRulesSource) Read(root string) ([]core.SourceFile, error) {
	return cursorRulesRead(root)
}

// cursorRulesRead returns every `.mdc` rule file under .cursor/rules.
func cursorRulesRead(root string) ([]core.SourceFile, error) {
	return readRulesDir(root, cursorRulesDir, func(name string) bool {
		return strings.HasSuffix(name, ".mdc")
	})
}

func (cursorRulesSource) DeclaredScope(p string, content []byte) (DocScope, bool) {
	if !underDir(p, cursorRulesDir) || !strings.HasSuffix(p, ".mdc") {
		return DocScope{}, false
	}
	return cursorRuleScope(content), true
}

// cursorRuleScope maps one rule file's frontmatter onto a DocScope.
func cursorRuleScope(content []byte) DocScope {
	fm, _ := parseFrontmatter(content)
	switch {
	case fm.isTrue("alwaysApply"):
		return DocScope{}
	case len(fm["globs"]) > 0:
		return DocScope{Globs: fm["globs"]}
	default:
		return DocScope{Audience: core.AudienceManual}
	}
}
