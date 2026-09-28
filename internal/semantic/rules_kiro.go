package semantic

// rules_kiro.go — GL-4 (docs/plans/governance-learning.md, design doc §3):
// Kiro's steering files, `.kiro/steering/*.md`, whose frontmatter
// `inclusion` decides when Kiro loads them:
//
//   - `always` (Kiro's default when the key is absent) is repo scope;
//   - `fileMatch` loads the file for paths matching `fileMatchPattern`,
//     which becomes the claim scope (a pattern naming every file is repo
//     scope);
//   - `manual` loads it only when the user names it: compiled, but the
//     claims carry audience manual and are not served unasked. Any other
//     inclusion mode, and `fileMatch` with no pattern, is treated as manual
//     — never guessed wider.

import (
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

const kiroSteeringDir = ".kiro/steering"

type kiroSteeringSource struct{}

func init() { RegisterRulesSource(kiroSteeringSource{}) }

func (kiroSteeringSource) ID() string { return "kiro" }

func (kiroSteeringSource) Detect(root string) bool { return dirExists(root, kiroSteeringDir) }

func (kiroSteeringSource) Read(root string) ([]core.SourceFile, error) {
	return kiroSteeringRead(root)
}

// kiroSteeringRead returns every markdown steering file, sorted by path.
func kiroSteeringRead(root string) ([]core.SourceFile, error) {
	return readRulesDir(root, kiroSteeringDir, func(name string) bool {
		return strings.HasSuffix(name, ".md")
	})
}

func (kiroSteeringSource) DeclaredScope(p string, content []byte) (DocScope, bool) {
	if !underDir(p, kiroSteeringDir) || !strings.HasSuffix(p, ".md") {
		return DocScope{}, false
	}
	return kiroInclusionScope(content), true
}

// kiroInclusionScope maps a steering file's frontmatter onto a DocScope.
func kiroInclusionScope(content []byte) DocScope {
	fm, _ := parseFrontmatter(content)
	switch fm.first("inclusion") {
	case "", "always":
		return DocScope{}
	case "fileMatch":
		globs := fm["fileMatchPattern"]
		if len(globs) == 0 {
			return DocScope{Audience: core.AudienceManual}
		}
		for _, g := range globs {
			if isEveryFileGlob(g) {
				return DocScope{}
			}
		}
		return DocScope{Globs: globs}
	default:
		return DocScope{Audience: core.AudienceManual}
	}
}
