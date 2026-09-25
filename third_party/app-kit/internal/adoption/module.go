package adoption

import (
	"errors"
	"unicode/utf8"

	"golang.org/x/mod/modfile"
)

const observedKitModule = "github.com/geoffmilleraz/app-kit"
const maxObservedModuleBytes = 1 << 20

// ModuleEvidence describes declarations in one go.mod, not module selection by
// Go's build list, workspace replacements, source integrity, or compatibility.
type ModuleEvidence struct {
	DeclaredVersion string             `json:"declared_version"`
	Replacement     *ModuleReplacement `json:"replacement,omitempty"`
	Resolution      string             `json:"resolution"`
}

// ModuleReplacement retains the declared path verbatim after Go string-literal
// decoding. In particular, local paths are neither cleaned nor followed.
type ModuleReplacement struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

// ObserveModule parses bounded local input without executing Go or resolving any
// dependency. Duplicate app-kit requirements or replacement keys are refused,
// including identical duplicates, rather than selecting by source order.
func ObserveModule(body []byte) (ModuleEvidence, error) {
	if len(body) > maxObservedModuleBytes || !utf8.Valid(body) {
		return ModuleEvidence{}, errors.New("adoption: module input exceeds bounds or is not valid UTF-8")
	}
	file, err := modfile.Parse("go.mod", body, nil)
	if err != nil || file.Module == nil || file.Module.Mod.Path == "" {
		// Parser errors may contain original source, module paths, and filenames.
		// Keep the observer's errors safe for diagnostic and machine output.
		return ModuleEvidence{}, errors.New("adoption: invalid go.mod declarations")
	}
	evidence := ModuleEvidence{Resolution: "declarations-only"}
	seenRequire := false
	for _, requirement := range file.Require {
		if requirement.Mod.Path != observedKitModule {
			continue
		}
		if seenRequire {
			return ModuleEvidence{}, errors.New("adoption: duplicate module requirement")
		}
		seenRequire = true
		evidence.DeclaredVersion = requirement.Mod.Version
	}
	replacements := make(map[string]bool)
	var wildcard, specific *ModuleReplacement
	for _, replacement := range file.Replace {
		if replacement.Old.Path != observedKitModule {
			continue
		}
		key := replacement.Old.Version
		if replacements[key] {
			return ModuleEvidence{}, errors.New("adoption: duplicate module replacement")
		}
		replacements[key] = true
		if evidence.DeclaredVersion == "" {
			continue
		}
		value := &ModuleReplacement{Path: replacement.New.Path, Version: replacement.New.Version}
		if replacement.Old.Version == "" {
			wildcard = value
		} else if replacement.Old.Version == evidence.DeclaredVersion {
			specific = value
		}
	}
	evidence.Replacement = specific
	if evidence.Replacement == nil {
		evidence.Replacement = wildcard
	}
	return evidence, nil
}
