// Package skills is the Agent Skills specification's SKILL.md frontmatter
// (https://agentskills.io/specification, source docs/specification.mdx
// 2026-08-04), vendored as a CUE schema (docs/plans/reach-parity.md RP-2,
// ADR-029 "standards are adopted as vendored schemas under cue-vet").
//
// Every shipped dist/agents/skills/*/SKILL.md frontmatter vets against
// #Frontmatter (task cue-vet; cmd/fathom's TestShippedSkillFrontmatterVets
// adds the name == directory rule the spec states but YAML alone cannot).
// The struct is CLOSED: the six fields below are the whole spec, and a
// Claude Code extension (`paths`, `hooks`, `context`, …) is a hard error on
// every spec-only reader — cmd/fathom's agentsVariant strips them for the
// `.agents/skills` rendering, and this schema keeps the shipped source
// itself on the standard.
package skills

import "strings"

#Frontmatter: {
	// 1–64 chars; lowercase letters, digits and hyphens; no leading or
	// trailing hyphen; no "--". Must equal the parent directory name.
	name: string & =~"^[a-z0-9]([a-z0-9-]*[a-z0-9])?$" & !~"--" & strings.MaxRunes(64)
	// What the skill does and when to use it; 1–1024 chars.
	description: string & strings.MinRunes(1) & strings.MaxRunes(1024)
	// License name or a reference to a bundled license file.
	license?: string
	// Environment requirements; 1–500 chars when present.
	compatibility?: string & strings.MinRunes(1) & strings.MaxRunes(500)
	// Arbitrary string → string map.
	metadata?: {[string]: string}
	// Space-separated pre-approved tools (experimental in the spec).
	"allowed-tools"?: string
}
