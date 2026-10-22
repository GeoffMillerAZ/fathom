package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Declared adaptations are consumer-owned even if the old ownership line
// remains. The plugin version is the existing version of the shipped skills.
var skillAdaptationMarker = regexp.MustCompile(`(?m)^[\t ]*<!-- fathom:adapted from=([a-z0-9-]+) version=(v?[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.+-]+)?) -->[\t ]*\r?$`)

func adaptedSkillVersion(body []byte, name string) string {
	for _, m := range skillAdaptationMarker.FindAllSubmatch(body, -1) {
		if string(m[1]) == name {
			return strings.TrimPrefix(string(m[2]), "v")
		}
	}
	return ""
}

func skillNameAt(dest string) string {
	if filepath.Base(dest) == "SKILL.md" {
		return filepath.Base(filepath.Dir(dest))
	}
	return strings.TrimSuffix(filepath.Base(dest), ".md")
}

func isAdaptedSkillFile(dest, name string) bool {
	b, err := os.ReadFile(dest)
	return err == nil && adaptedSkillVersion(b, name) != ""
}

func skillAdaptationNote(name, version string) string {
	return fmt.Sprintf("%s: adapted (shipped v%s, adapted from v%s)", name, pluginVersion, version)
}

func skillAdaptationDiff(dest, name string) string {
	return `fathom_diff_dir=$(mktemp -d); fathom plugin render --out "$fathom_diff_dir" && diff -u "$fathom_diff_dir/skills/` + name + `/SKILL.md" ` + shellQuote(dest)
}

// skillAdaptationVersionMatches compares release identity, ignoring build
// metadata. Both older and newer adaptations still need a comparison.
func skillAdaptationVersionMatches(version string) bool {
	return strings.SplitN(strings.TrimPrefix(version, "v"), "+", 2)[0] == strings.SplitN(pluginVersion, "+", 2)[0]
}

// adaptedSkillsDirectorySkips protects consumer adaptations in an earlier
// real .agents/skills directory, even on a machine with no ownership ledger.
// WalkDir visits files in lexical order and never follows links.
func adaptedSkillsDirectorySkips(root string) []string {
	base := filepath.Join(root, filepath.FromSlash(agentsSkillsDir))
	fi, err := os.Lstat(base)
	if err != nil || !fi.IsDir() {
		return nil
	}
	var notes []string
	_ = filepath.WalkDir(base, func(dest string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		body, err := os.ReadFile(dest)
		if err != nil {
			return nil
		}
		if marker := skillAdaptationMarker.FindSubmatch(body); marker != nil {
			rel, err := filepath.Rel(root, dest)
			if err != nil {
				return nil
			}
			notes = append(notes, "skip "+agentsSkillsDir+" link: adapted file "+filepath.ToSlash(rel)+"; preserved; compare: "+skillAdaptationDiff(dest, string(marker[1])))
		}
		return nil
	})
	return notes
}
