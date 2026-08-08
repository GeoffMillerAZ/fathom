package main

// capabilityenforce.go — the enforcement side of the capability registry.
//
// Without this file the registry was a lie: `fathom capabilities set guards
// off` wrote a config file, the table then reported "switched off by this
// repository", and guards kept firing on every edit. Nothing outside the
// two reporting surfaces read the resolution. That is a config read
// rendered as a behavioural fact — exactly the failure mode the registry
// exists to remove (law 14), shipped inside the thing built to prevent it.
//
// Every switchable capability is enforced HERE, at the site that would
// otherwise act, and TestSwitchableCapabilitiesAreEnforced proves that
// flipping the setting changes observed behaviour.

import (
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/capability"
)

// capabilityOffAt reports whether capability id is switched OFF for the
// repository rooted at root.
//
// It fails OPEN — an unreadable or malformed config yields "not off", so
// the capability keeps working. That direction is deliberate: this runs
// inside hooks, where the house contract is that operational failure is
// silent and non-blocking, and where silently *withdrawing* a safety
// feature because a file failed to parse is far worse than keeping it. The
// malformed file is reported loudly by `fathom capabilities` and failed
// loudly by `fathom doctor`; it does not get to disarm a guard by
// accident.
func capabilityOffAt(root, id string) bool {
	if root == "" {
		return false
	}
	reg := capability.Default()
	c, ok := reg.Get(id)
	if !ok || !c.Switchable {
		// A status-only capability has no switch to read. Treating an
		// unknown id as "off" would let a typo disable a safety feature —
		// the worst failure this package can have.
		return false
	}
	res, errs := loadCapabilityConfig(reg, root)
	if len(errs) > 0 {
		return false // fail open; see the doc comment
	}
	return !res.For(c).On
}

// capabilityOffForFile is the hook-path convenience: resolve the governing
// repository root for a file, then ask. Hooks know a file path, not a
// repository.
func capabilityOffForFile(projDir, filePath, id string) bool {
	root := projDir
	if filePath != "" {
		for _, cand := range serviceCandidates(projDir) {
			if _, ok := relativeToRoot(cand.root, filePath); ok {
				root = cand.root
				break
			}
		}
	}
	return capabilityOffAt(filepath.Clean(root), id)
}
