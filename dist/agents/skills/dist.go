// Package skillsdist embeds fathom's shipped agent-skill sources
// (docs/plans/pack-taxonomy-adoption.md AK-1) so `fathom init --skills`
// (AK-2, cmd/fathom's installSkills) can render them into a target repo
// without requiring fathom's own source tree to be present at install
// time — the compiled binary carries the data.
//
// Each skill under this directory ships two rendered shapes from one
// authored source: SKILL.md (Claude Code's skill format — YAML frontmatter
// with name/description, then a Markdown body) and codex-prompt.md (a
// single Codex custom-prompt file, installed as
// ~/.codex/prompts/<name>.md — see any codex-prompt.md's own header for the
// honesty note about Codex's newer native skills mechanism). Both carry the
// same fathom:owned marker line (see cmd/fathom's skillMarker) so a re-run
// of `fathom init --skills` can safely re-render a file it previously
// wrote, while refusing to touch a same-named file it didn't write.
package skillsdist

import "embed"

//go:embed fathom-plan fathom-verify fathom-onboard fathom-feedback fathom-doctor fathom-impact fathom-rewire fathom-activate fathom-custodian fathom-findings
var FS embed.FS

// Names is the ordered, canonical list of shipped skill names — the one
// place this list is maintained; installers range over it instead of
// hard-coding six names in more than one spot.
var Names = []string{"fathom-plan", "fathom-verify", "fathom-onboard", "fathom-feedback", "fathom-doctor", "fathom-impact", "fathom-rewire", "fathom-activate", "fathom-custodian", "fathom-findings"}
