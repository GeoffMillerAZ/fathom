package main

import (
	"fmt"
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
