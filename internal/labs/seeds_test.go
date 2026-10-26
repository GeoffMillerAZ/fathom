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
		"hook:stop-hook", "hook:edit-hook", "hook:edit-hook --pre", "hook:prompt-hook", "hook:session-hook", "hook:session-hook --event subagent-start",
		"hook:readout-hook --event post-tool", "hook:readout-hook --event tool-failure", "hook:readout-hook --event pre-read", "stanza:agents-md"}
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
