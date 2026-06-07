// Package license detects SPDX license identifiers from license file content
// and evaluates them against an allowed-list policy.
//
// All exported functions are pure, deterministic, and safe for concurrent use.
package license

import (
	"fmt"
	"strings"
)

// ── Fingerprints ─────────────────────────────────────────────────────────────

// fingerprint is a set of required keywords that, when all present in the
// license content, identify a specific SPDX expression.
type fingerprint struct {
	spdx     string
	required []string // all must be present (case-insensitive)
	// excluded is a set of strings; if any is present the fingerprint does NOT
	// match (used to disambiguate MIT vs ISC, BSD-2 vs BSD-3, etc.)
	excluded []string
}

// orderedFingerprints is checked in order; first match wins. More specific
// entries (e.g. GPL-3.0) are listed before looser ones.
var orderedFingerprints = []fingerprint{
	{
		spdx:     "Unlicense",
		required: []string{"this is free and unencumbered software released into the public domain"},
	},
	{
		spdx:     "GPL-3.0",
		required: []string{"gnu general public license", "version 3"},
	},
	{
		spdx:     "Apache-2.0",
		required: []string{"apache license", "version 2.0"},
	},
	{
		spdx: "BSD-3-Clause",
		required: []string{
			"redistribution and use in source and binary forms",
			"neither the name",
		},
	},
	{
		spdx: "BSD-2-Clause",
		required: []string{
			"redistribution and use in source and binary forms",
		},
		excluded: []string{"neither the name"},
	},
	{
		spdx: "ISC",
		required: []string{
			"permission to use, copy, modify, and/or distribute this software",
			"isc",
		},
	},
	{
		spdx: "MIT",
		required: []string{
			"permission is hereby granted, free of charge",
			"mit",
		},
	},
}

// Detect returns the SPDX identifier for the license contained in content, or
// "UNKNOWN" when no fingerprint matches. Detection is case-insensitive and
// ignores leading/trailing whitespace.
func Detect(content []byte) string {
	lower := strings.ToLower(string(content))

	for _, fp := range orderedFingerprints {
		if matches(lower, fp) {
			return fp.spdx
		}
	}
	return "UNKNOWN"
}

func matches(lower string, fp fingerprint) bool {
	for _, req := range fp.required {
		if !strings.Contains(lower, req) {
			return false
		}
	}
	for _, exc := range fp.excluded {
		if strings.Contains(lower, exc) {
			return false
		}
	}
	return true
}

// ── Policy ───────────────────────────────────────────────────────────────────

// Policy evaluates detected (a single SPDX id) against the allowed set.
// It returns ok=true and an empty reason when detected is in allowed.
// It returns ok=false and a human-readable reason otherwise.
// allowed may be nil or empty (always fail).
func Policy(detected string, allowed []string) (ok bool, reason string) {
	for _, a := range allowed {
		if a == detected {
			return true, ""
		}
	}
	if len(allowed) == 0 {
		return false, fmt.Sprintf("license %q is not permitted: no licenses are allowed", detected)
	}
	return false, fmt.Sprintf("license %q is not in the allowed list %v", detected, allowed)
}
