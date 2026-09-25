package adoption

import (
	"runtime"
	"runtime/debug"
)

// BuildEvidence identifies the running checker, not the consumer's resolved
// dependencies. VCSModified is a build-time dirty bit, not a working-tree hash.
type BuildEvidence struct {
	MainModule  string `json:"main_module"`
	MainVersion string `json:"main_version"`
	GoVersion   string `json:"go_version"`
	GOOS        string `json:"goos"`
	GOARCH      string `json:"goarch"`
	VCSRevision string `json:"vcs_revision"`
	VCSModified *bool  `json:"vcs_modified"`
}

// CurrentBuild reports an allowlisted subset of this process's build metadata.
// It exposes no build environment, repository paths, dependency list or time.
func CurrentBuild() BuildEvidence {
	info, _ := debug.ReadBuildInfo()
	return buildEvidence(info, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

func buildEvidence(info *debug.BuildInfo, goVersion, goos, goarch string) BuildEvidence {
	evidence := BuildEvidence{GoVersion: goVersion, GOOS: goos, GOARCH: goarch}
	if info == nil {
		return evidence
	}
	evidence.MainModule = info.Main.Path
	evidence.MainVersion = info.Main.Version
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			evidence.VCSRevision = setting.Value
		case "vcs.modified":
			evidence.VCSModified = nil
			if setting.Value == "true" || setting.Value == "false" {
				modified := setting.Value == "true"
				evidence.VCSModified = &modified
			}
		}
	}
	return evidence
}
