package semantic

// rules_bugbot.go — GL-5 (docs/plans/governance-learning.md, design doc §3):
// Cursor Bugbot's review rules, `BUGBOT.md`, nested — at any directory, or
// inside that directory's own `.cursor/` (Bugbot's documented location).
// A nested file governs only its own directory: the compile-time scope
// floor (compile.go scopeFloor, instructionFileBases) keeps every claim
// inside it, with `<dir>/.cursor/BUGBOT.md` floored to <dir>. Bugbot rules
// are review rules, so every claim carries audience review: served to the
// review verbs (ObligationsOptions.Review), never to the edit hook by
// default.

import (
	"github.com/geoffmilleraz/fathom/internal/core"
)

const bugbotFile = "BUGBOT.md"

type bugbotRulesSource struct{}

func init() { RegisterRulesSource(bugbotRulesSource{}) }

func (bugbotRulesSource) ID() string { return "bugbot" }

func (bugbotRulesSource) Detect(root string) bool {
	return len(findNamedFiles(root, bugbotFile, ".cursor")) > 0
}

func (bugbotRulesSource) Read(root string) ([]core.SourceFile, error) {
	return bugbotRulesRead(root), nil
}

// bugbotRulesRead returns every BUGBOT.md in the tree, sorted by path.
func bugbotRulesRead(root string) []core.SourceFile {
	return readNamedFiles(root, bugbotFile, ".cursor")
}

func (bugbotRulesSource) DeclaredScope(p string, _ []byte) (DocScope, bool) {
	if baseName(p) != bugbotFile {
		return DocScope{}, false
	}
	return DocScope{Audience: core.AudienceReview}, true
}
