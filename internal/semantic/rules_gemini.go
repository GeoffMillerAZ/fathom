package semantic

// rules_gemini.go — GL-6 (docs/plans/governance-learning.md, design doc §3):
// Gemini CLI's context files, `GEMINI.md`, nested, read exactly as
// AGENTS.md is: prose with no declared scope, a nested file floored to its
// own subtree by the compile-time scope floor (instructionFileBases), a
// root file keeping the model's verified scope. No ScopeDeclarer: there is
// nothing in the file's shape to declare.

import (
	"github.com/geoffmilleraz/fathom/internal/core"
)

const geminiFile = "GEMINI.md"

type geminiRulesSource struct{}

func init() { RegisterRulesSource(geminiRulesSource{}) }

func (geminiRulesSource) ID() string { return "gemini" }

func (geminiRulesSource) Detect(root string) bool {
	return len(findNamedFiles(root, geminiFile)) > 0
}

func (geminiRulesSource) Read(root string) ([]core.SourceFile, error) {
	return geminiRulesRead(root), nil
}

// geminiRulesRead returns every GEMINI.md in the tree, sorted by path.
func geminiRulesRead(root string) []core.SourceFile {
	return readNamedFiles(root, geminiFile)
}
