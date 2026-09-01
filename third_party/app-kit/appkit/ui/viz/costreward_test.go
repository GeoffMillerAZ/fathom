package viz

import (
	"strings"
	"testing"
)

func crFixture() []CostRewardItem {
	return []CostRewardItem{
		{Label: "habitability", Reward: 5200, Cost: 240},
		{Label: "deposit", Reward: 2400, Cost: 120},
		{Label: "trial path", Reward: 9600, Cost: 1500},
	}
}

func TestCostReward_Golden(t *testing.T) {
	got := CostReward(crFixture(), CostRewardOptions{Prefix: "$", AriaLabel: "reward vs cost"})
	assertGolden(t, got, "costreward_default")
}

func TestCostReward_Empty(t *testing.T) {
	got := CostReward(nil, CostRewardOptions{})
	if !strings.Contains(got, "<svg") {
		t.Errorf("empty costreward missing svg")
	}
	if strings.Contains(got, "<rect") {
		t.Errorf("empty costreward should render no bars")
	}
}

func TestCostReward_NegativeClampSafe(t *testing.T) {
	got := CostReward([]CostRewardItem{{Label: "x", Reward: -5, Cost: -3}}, CostRewardOptions{Max: 10})
	if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") || strings.Contains(got, `width="-`) {
		t.Errorf("negative costreward produced bad output: %s", got)
	}
}

func TestCostReward_Roles(t *testing.T) {
	got := CostReward(crFixture(), CostRewardOptions{})
	if !strings.Contains(got, "ak-color-positive") || !strings.Contains(got, "ak-color-caution") {
		t.Errorf("default reward/cost roles missing: %s", got)
	}
}

func TestCostReward_Escaping(t *testing.T) {
	got := CostReward([]CostRewardItem{{Label: `<script>`, Reward: 1, Cost: 1}}, CostRewardOptions{Max: 1})
	if strings.Contains(got, "<script>") {
		t.Errorf("costreward did not escape label")
	}
}

func TestCostReward_Determinism(t *testing.T) {
	a := CostReward(crFixture(), CostRewardOptions{Prefix: "$"})
	b := CostReward(crFixture(), CostRewardOptions{Prefix: "$"})
	if a != b {
		t.Errorf("costreward non-deterministic")
	}
}

func TestCostReward_Accessibility(t *testing.T) {
	got := CostReward(crFixture(), CostRewardOptions{AriaLabel: "cr"})
	if !strings.Contains(got, `role="img"`) || !strings.Contains(got, "<title>") {
		t.Errorf("costreward missing accessibility")
	}
}
