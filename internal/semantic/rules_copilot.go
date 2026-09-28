package semantic

// rules_copilot.go — GL-3 (docs/plans/governance-learning.md, design doc §3):
// GitHub Copilot's custom instructions. `.github/copilot-instructions.md`
// is repository-wide (repo scope: the model's proposed scope stands, as for
// a root CLAUDE.md). Each `.github/instructions/*.instructions.md` carries
// an `applyTo` frontmatter glob list that becomes the claim scope; an
// `applyTo` of "**" (every file) is repo scope, not a catch-all glob the
// over-broad check would reject; a file with no `applyTo` is one Copilot
// attaches only when asked, so it is compiled with audience manual.

import (
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

const (
	copilotRepoFile        = ".github/copilot-instructions.md"
	copilotInstructionsDir = ".github/instructions"
)

type copilotRulesSource struct{}

func init() { RegisterRulesSource(copilotRulesSource{}) }

func (copilotRulesSource) ID() string { return "copilot" }

func (copilotRulesSource) Detect(root string) bool {
	return fileExists(root, copilotRepoFile) || dirExists(root, copilotInstructionsDir)
}

func (copilotRulesSource) Read(root string) ([]core.SourceFile, error) {
	return copilotInstructionsRead(root)
}

// copilotInstructionsRead returns the repository-wide instructions file and
// every path-specific `*.instructions.md`, sorted by path.
func copilotInstructionsRead(root string) ([]core.SourceFile, error) {
	var out []core.SourceFile
	if f, ok := readRulesFile(root, copilotRepoFile); ok {
		out = append(out, f)
	}
	if dirExists(root, copilotInstructionsDir) {
		files, err := readRulesDir(root, copilotInstructionsDir, func(name string) bool {
			return strings.HasSuffix(name, ".instructions.md")
		})
		if err != nil {
			return nil, err
		}
		out = append(out, files...)
	}
	return out, nil
}

func (copilotRulesSource) DeclaredScope(p string, content []byte) (DocScope, bool) {
	switch {
	case p == copilotRepoFile:
		return DocScope{}, true
	case underDir(p, copilotInstructionsDir) && strings.HasSuffix(p, ".instructions.md"):
		return copilotApplyToScope(content), true
	}
	return DocScope{}, false
}

// copilotApplyToScope maps an instructions file's `applyTo` onto a DocScope.
func copilotApplyToScope(content []byte) DocScope {
	fm, _ := parseFrontmatter(content)
	globs := fm["applyTo"]
	if len(globs) == 0 {
		return DocScope{Audience: core.AudienceManual}
	}
	for _, g := range globs {
		if isEveryFileGlob(g) {
			return DocScope{}
		}
	}
	return DocScope{Globs: globs}
}

// isEveryFileGlob reports whether a declared glob names every file in the
// repository — a client's spelling of "always", i.e. repo scope.
func isEveryFileGlob(g string) bool {
	switch strings.TrimPrefix(strings.TrimSpace(g), "/") {
	case "**", "**/*", "*":
		return true
	}
	return false
}
