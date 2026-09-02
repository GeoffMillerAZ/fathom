package harness

// adoption.go — RP-18 (docs/plans/reach-parity.md): the adoption matrix
// docs/adoption/other-harnesses.md is a RENDERING of the registry, never
// hand-maintained. One row per descriptor: tier (after the demotion
// clock), the instruction file(s) fathom writes or the harness reads, the
// skills path, the MCP config path with the harness's own `mcp add` line,
// and what the descriptor was verified against — or the literal
// `unverified`. An unverified row says so in its own cell; nothing here
// promises a channel the descriptor does not declare.

import (
	"fmt"
	"strings"
	"time"
)

// AdoptionRow is one line of the matrix, as data (`fathom harness matrix
// --json`).
type AdoptionRow struct {
	ID              string   `json:"id"`
	Display         string   `json:"display"`
	Tier            string   `json:"tier"`
	InstructionFile []string `json:"instruction_files"`
	SkillsPath      []string `json:"skills_paths"`
	MCPConfig       []string `json:"mcp_config_files"`
	MCPAdd          string   `json:"mcp_add"`
	VerifiedAgainst string   `json:"verified_against"`
}

// AdoptionRows computes the matrix for every descriptor in registry order.
// url is the daemon base URL the `mcp add` line is rendered with; now
// drives the demotion clock.
func AdoptionRows(url string, now time.Time) []AdoptionRow {
	rows := make([]AdoptionRow, 0, len(registry))
	for _, d := range All() {
		r := AdoptionRow{
			ID:              d.ID,
			Display:         d.Display,
			Tier:            EffectiveTier(d, now),
			VerifiedAgainst: d.VerifiedAgainst,
			MCPAdd:          d.MCPAddLine(url),
		}
		for _, f := range d.Instructions {
			label := f.Path
			switch f.Mode {
			case "mirror":
				label += " (mirror)"
			case "settings-key":
				label += " (settings key)"
			}
			r.InstructionFile = append(r.InstructionFile, label)
		}
		for _, p := range d.Skills.RepoDirs {
			r.SkillsPath = append(r.SkillsPath, strings.TrimSuffix(strings.ReplaceAll(p, "/{{name}}/SKILL.md", ""), "/"))
		}
		if len(r.SkillsPath) == 0 {
			r.SkillsPath = []string{"—"}
		}
		r.MCPConfig = append(r.MCPConfig, d.MCP.ConfigFiles...)
		if len(r.MCPConfig) == 0 {
			r.MCPConfig = []string{"(the harness's own MCP config)"}
		}
		rows = append(rows, r)
	}
	return rows
}

// RenderAdoptionMatrix renders AdoptionRows as a Markdown table — the ONE
// source docs/adoption/other-harnesses.md's matrix section is pinned to
// (TestAdoptionDocMatrixIsGenerated). Regenerate the doc's block with
// `fathom harness matrix`.
func RenderAdoptionMatrix(url string, now time.Time) string {
	var sb strings.Builder
	sb.WriteString("| Harness | Tier | Instruction file | Skills path | MCP config + `mcp add` | Verified against |\n|---|---|---|---|---|---|\n")
	for _, r := range AdoptionRows(url, now) {
		verified := r.VerifiedAgainst
		if verified == "unverified" {
			verified = "**unverified** — no session on this build has run here"
		}
		add := strings.ReplaceAll(r.MCPAdd, "|", "\\|")
		if !strings.Contains(add, "`") {
			add = "`" + add + "`" // a command line; prose that already carries code spans stays prose
		}
		fmt.Fprintf(&sb, "| `%s` (%s) | %s | %s | %s | %s<br>%s | %s |\n",
			r.ID, r.Display, r.Tier,
			codeList(r.InstructionFile), codeList(r.SkillsPath), codeList(r.MCPConfig),
			add, verified)
	}
	return sb.String()
}

func codeList(xs []string) string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if x == "—" || strings.HasPrefix(x, "(") {
			out = append(out, x)
			continue
		}
		out = append(out, "`"+x+"`")
	}
	return strings.Join(out, ", ")
}
