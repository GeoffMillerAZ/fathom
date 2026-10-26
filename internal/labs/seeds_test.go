package labs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
	"github.com/geoffmilleraz/fathom/internal/decide"
)

func seedSurfaces() []string {
	keys := []string{"mcp:find_symbol", "mcp:search", "mcp:symbol_context", "mcp:read_source", "mcp:blast_radius", "mcp:review_diff", "mcp:verify_plan", "mcp:obligations",
		"stanza:agents-md"}
	// The Claude Code hook bindings, keyed as cmd/fathom's labHookSurface keys
	// them (verb, then [event/capability event matcher]). cmd/fathom's
	// TestCommittedLabsCloseOverShippedSurfaces checks the full shipped set.
	keys = append(keys,
		"hook:config audit --hook --event file-changed [file-changed/config-audit FileChanged matcher=CLAUDE.md|AGENTS.md|concerns.cue|config.toml]",
		"hook:config audit --hook [config-change/config-audit ConfigChange matcher=]",
		"hook:doctor --brief [setup/setup-doctor Setup matcher=]",
		"hook:edit-hook --pre [pre-tool/discovery-observe PreToolUse matcher=Grep|Glob|Read]",
		"hook:edit-hook --pre [pre-tool/guard-pre-edit PreToolUse matcher=Edit|Write|MultiEdit]",
		"hook:edit-hook --pre [pre-tool/guard-pre-shell PreToolUse matcher=Bash]",
		"hook:edit-hook --pre [pre-tool/subagent-prime PreToolUse matcher=^(Agent|Task)$]",
		"hook:edit-hook [post-tool/discovery-observe PostToolUse matcher=mcp__fathom__(symbol_context|find_symbol|read_source)|mcp__plugin_[^_]*_fathom__(symbol_context|find_symbol|read_source)]",
		"hook:edit-hook [post-tool/obligations-post-edit PostToolUse matcher=Edit|Write|MultiEdit]",
		"hook:edit-hook [post-tool/obligations-post-shell PostToolUse matcher=Bash]",
		"hook:pre-compact-hook [pre-compact/deposit-pre-compact PreCompact matcher=manual|auto]",
		"hook:prompt-hook [user-prompt-submit/prompt-obligations UserPromptSubmit matcher=]",
		"hook:readout-hook --event post-tool [post-tool/readout PostToolUse matcher=Bash]",
		"hook:readout-hook --event pre-read [pre-tool/read-outline-hint PreToolUse matcher=Read]",
		"hook:readout-hook --event tool-failure [tool-failure/readout PostToolUseFailure matcher=Bash]",
		"hook:session-end-hook [session-end/deposit-session-end SessionEnd matcher=]",
		"hook:session-hook --event cwd-changed [cwd-changed/cwd-prime CwdChanged matcher=]",
		"hook:session-hook --event cwd-changed [cwd-changed/cwd-prime DirectoryAdded matcher=]",
		"hook:session-hook --event failed-test-run [tool-failure/test-run-line PostToolUseFailure matcher=Bash]",
		"hook:session-hook --event instructions-loaded [instructions-loaded/stanza-loaded InstructionsLoaded matcher=]",
		"hook:session-hook --event subagent-start [subagent-start/subagent-prime SubagentStart matcher=]",
		"hook:session-hook --event test-run [post-tool/test-run-line PostToolUse matcher=Bash]",
		"hook:session-hook --event tool-failure [tool-failure/tool-failure-advice PostToolUseFailure matcher=mcp__fathom__.*|mcp__plugin_[^_]*_fathom__.*]",
		"hook:session-hook [session-start/prime SessionStart matcher=startup|resume|clear|compact]",
		"hook:stop-hook --event subagent-stop [subagent-stop/claim-check SubagentStop matcher=]",
		"hook:stop-hook [stop/stop-verify Stop matcher=]",
	)
	for _, name := range skillsdist.Names {
		keys = append(keys, "skill:"+name)
	}
	for _, site := range append(append([]string{}, decide.AllSites...), decide.SessionSites...) {
		keys = append(keys, "decide:"+site)
	}
	return keys
}

func TestCommittedSeedCardsLoad(t *testing.T) {
	dir := filepath.Join("..", "..", "calibration", "labs.d")
	cards, err := LoadLabs(dir, seedSurfaces())
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) < 16 {
		t.Fatalf("need at least 16 seed cards, got %d", len(cards))
	}
	index, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, card := range cards {
		if card.Firing.Mode == "n/a" {
			if strings.TrimSpace(card.Firing.Reason) == "" {
				t.Errorf("%s: missing n/a reason", card.ID)
			}
		} else if strings.TrimSpace(card.Firing.ShouldNot) == "" {
			t.Errorf("%s: missing should-not rule", card.ID)
		}
		wantArm := "plugin-minus-" + strings.TrimPrefix(card.ID, "lab-")
		switch card.ID {
		case "lab-skills":
			wantArm = "plugin-minus-skill:{skill}"
		case "lab-decision-port":
			wantArm = "plugin-minus-decide:{site}"
		}
		if card.Helped.Arm != wantArm {
			t.Errorf("%s: missing minus-one-feature arm", card.ID)
		}
		if len(card.Kill) != 2 {
			t.Errorf("%s: want K4 and K1 only", card.ID)
			continue
		}
		wantReading := "docs/bench/readings/labs/" + card.ID + ".md"
		if card.Kill[0].Criterion != seedK4 || card.Kill[1].Criterion != seedK1 {
			t.Errorf("%s: owner ruling L-R1 not preserved", card.ID)
		}
		for _, kill := range card.Kill {
			if kill.Reading != wantReading {
				t.Errorf("%s: kill reading %q", card.ID, kill.Reading)
			}
		}
		if strings.Count(string(index), "| ["+card.ID+"](") != 1 {
			t.Errorf("%s: need one index row", card.ID)
		}
	}
}

const seedK4 = "K4: A knob that raises firing and leaves helped harmed, or inconclusive toward harm, is reverted."
const seedK1 = "K1: A classifier surface with TPR or TNR Wilson lower bound under 0.80 on 40 labeled points per class reports counts only."
