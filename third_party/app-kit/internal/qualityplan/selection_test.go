package qualityplan

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

type selectionWire struct {
	CapabilityID    string   `json:"capability_id,omitempty"`
	Scope           string   `json:"scope"`
	Status          string   `json:"status"`
	AvailableScopes []string `json:"available_scopes"`
	NextAction      string   `json:"next_action"`
}

type listWire struct {
	Revision      string        `json:"revision"`
	ClaimBoundary string        `json:"claim_boundary"`
	Items         []Summary     `json:"items"`
	Selection     selectionWire `json:"selection"`
}

func selectionFixture(t *testing.T) (*Snapshot, []string) {
	t.Helper()
	base, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(base.catalogIDs))
	for id := range base.catalogIDs {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	if len(ids) < 4 {
		t.Fatal("selection fixture requires four catalog capabilities")
	}
	return &Snapshot{
		revision:                    "selection-test-revision",
		enhancementStandardRevision: "selection-test-standard",
		catalogIDs:                  base.catalogIDs,
		items: []Item{
			{ID: "core-only", Core: true, CapabilityIDs: []string{ids[0]}},
			{ID: "mixed-core", Core: true, CapabilityIDs: []string{ids[2]}},
			{ID: "mixed-on-demand", Core: false, CapabilityIDs: []string{ids[2]}},
			{ID: "on-demand-only", Core: false, CapabilityIDs: []string{ids[1]}},
		},
	}, ids
}

func wireList(t *testing.T, snapshot *Snapshot, capabilityID, scope string) listWire {
	t.Helper()
	result, err := snapshot.List(capabilityID, scope)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var wire listWire
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.Revision != result.Revision || wire.ClaimBoundary != result.ClaimBoundary || len(wire.Items) != len(result.Items) {
		t.Fatalf("selection metadata changed existing projection: %#v != %#v", wire, result)
	}
	return wire
}

func assertSelection(t *testing.T, got selectionWire, capabilityID, scope, status string, available []string) {
	t.Helper()
	if got.CapabilityID != capabilityID || got.Scope != scope || got.Status != status ||
		!slices.Equal(got.AvailableScopes, available) {
		t.Fatalf("selection mismatch: got %#v, want capability=%q scope=%q status=%q available=%v",
			got, capabilityID, scope, status, available)
	}
	if got.NextAction == "" || len(got.NextAction) > 240 {
		t.Fatalf("next action must be present and bounded: %q", got.NextAction)
	}
	lower := strings.ToLower(got.NextAction)
	for _, claim := range []string{"passed", "success", "certified", "ready"} {
		if strings.Contains(lower, claim) {
			t.Fatalf("next action makes a success claim %q: %q", claim, got.NextAction)
		}
	}
	for _, required := range map[string][]string{
		"matched":      {"get_quality_work", "revision"},
		"filtered-out": {"scope"},
		"untracked":    {"just-in-time-conversion", "contribution-loop", "docs/app-kit.md", "issue", "consumer review", "exact query/profile", "capability_id", "revision", "catalog_revision", "writes nothing"},
	}[status] {
		if !strings.Contains(lower, required) {
			t.Fatalf("%s next action lacks actionable provenance %q: %q", status, required, got.NextAction)
		}
	}
}

func TestListSelectionDistinguishesMatchedFilteredAndUntracked(t *testing.T) {
	snapshot, ids := selectionFixture(t)
	assertSelection(t, wireList(t, snapshot, ids[0], "").Selection,
		ids[0], "core", "matched", []string{"core"})
	assertSelection(t, wireList(t, snapshot, ids[0], "on-demand").Selection,
		ids[0], "on-demand", "filtered-out", []string{"core"})
	assertSelection(t, wireList(t, snapshot, ids[1], "core").Selection,
		ids[1], "core", "filtered-out", []string{"on-demand"})
	assertSelection(t, wireList(t, snapshot, ids[2], "core").Selection,
		ids[2], "core", "matched", []string{"core", "on-demand"})
	assertSelection(t, wireList(t, snapshot, ids[2], "all").Selection,
		ids[2], "all", "matched", []string{"core", "on-demand"})
	assertSelection(t, wireList(t, snapshot, ids[3], "all").Selection,
		ids[3], "all", "untracked", []string{})
}

func TestListSelectionHandlesUnfilteredEmptyInventoryWithoutFabricatingCapability(t *testing.T) {
	snapshot, _ := selectionFixture(t)
	snapshot.items = nil
	got := wireList(t, snapshot, "", "all")
	assertSelection(t, got.Selection, "", "all", "untracked", []string{})
	if len(got.Items) != 0 {
		t.Fatalf("empty inventory returned items: %#v", got.Items)
	}
}

func TestListSelectionWireScopesAreDeterministic(t *testing.T) {
	snapshot, ids := selectionFixture(t)
	first := wireList(t, snapshot, ids[2], "all").Selection
	assertSelection(t, first, ids[2], "all", "matched", []string{"core", "on-demand"})
	first.AvailableScopes[0] = "mutated"
	second := wireList(t, snapshot, ids[2], "all").Selection
	assertSelection(t, second, ids[2], "all", "matched", []string{"core", "on-demand"})
}

func TestListSelectionReturnedScopesAreDetached(t *testing.T) {
	snapshot, ids := selectionFixture(t)
	first, err := snapshot.List(ids[2], "all")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(first.Selection.AvailableScopes, []string{"core", "on-demand"}) {
		t.Fatalf("unexpected initial scopes: %v", first.Selection.AvailableScopes)
	}
	first.Selection.AvailableScopes[0] = "mutated"
	second, err := snapshot.List(ids[2], "all")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(second.Selection.AvailableScopes, []string{"core", "on-demand"}) {
		t.Fatalf("caller mutation escaped its returned result: %v", second.Selection.AvailableScopes)
	}
}

func TestListSelectionKeepsExistingInputRefusals(t *testing.T) {
	snapshot, _ := selectionFixture(t)
	for _, input := range []struct{ capability, scope string }{
		{"missing.capability", "all"},
		{"", "invalid"},
	} {
		if _, err := snapshot.List(input.capability, input.scope); err == nil {
			t.Fatalf("invalid selection was accepted: %#v", input)
		}
	}
}
