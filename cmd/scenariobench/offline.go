package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

// Each offline control owns its factory and help alongside its implementation.
type offlineHarnessConfig struct {
	Corpus   scenariobench.Corpus
	PatchDir string
	CacheDir string
}
type offlineHarnessFactory struct {
	help string
	new  func(offlineHarnessConfig) scenariobench.Harness
}

var offlineHarnesses = map[string]offlineHarnessFactory{}
var offlineCommands = map[string]func([]string) int{}

func harnessHelp() string {
	help := "harness to run sessions with: claude (real spend), fake (replays committed transcripts, no spend), null (no edits or answer, no spend)"
	names := make([]string, 0, len(offlineHarnesses))
	for name := range offlineHarnesses {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		help += ", " + name + " (" + offlineHarnesses[name].help + ")"
	}
	return help
}
func commandUsage() string {
	names := []string{"plan", "run", "score", "ledger", "void"}
	for name := range offlineCommands {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, "|")
}

func harnessNames() string {
	names := []string{"claude", "fake", "null"}
	for name := range offlineHarnesses {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, "|")
}

func validateOfflineHarnessVersion(name, version string) error {
	// Reserve the control versions in both directions, including built-in null.
	versions := map[string]string{"null": "null-1"}
	for control, factory := range offlineHarnesses {
		versions[control] = factory.new(offlineHarnessConfig{}).Version()
	}
	if expected, ok := versions[name]; ok && version != expected {
		return fmt.Errorf("-harness %s requires -harness-version %s", name, expected)
	}
	for _, control := range []string{"null", "oracle", "wrong-patch"} {
		if version == versions[control] && name != control {
			return fmt.Errorf("-harness-version %s is reserved for -harness %s", version, control)
		}
	}
	return nil
}
