package qualityplan

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/catalog"
)

func changedReaderCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	c := readerCatalog(t)
	for i := range c.Entries {
		if c.Entries[i].ID == "analytics.comparison" {
			c.Entries[i].Usage += " Synthetic changed integration contract."
		}
	}
	body, err := json.Marshal(map[string]any{"schema_version": c.SchemaVersion, "entries": c.Entries})
	if err != nil {
		t.Fatal(err)
	}
	changed, err := catalog.Load(body)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Revision == c.Revision {
		t.Fatal("fixture did not change validated catalog metadata")
	}
	return changed
}

func catalogBindingIdentity(t *testing.T, value any) string {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	var revision string
	if err := json.Unmarshal(wire["catalog_revision"], &revision); err != nil {
		t.Fatalf("missing catalog identity: %v", err)
	}
	return revision
}

func TestCatalogBindingInventoryRevisionRemainsIndependent(t *testing.T) {
	original, changed := readerCatalog(t), changedReaderCatalog(t)
	first, err := loadSnapshot(embeddedInventory, original)
	if err != nil {
		t.Fatal(err)
	}
	second, err := loadSnapshot(embeddedInventory, changed)
	if err != nil {
		t.Fatal(err)
	}
	if first.revision != second.revision {
		t.Fatal("catalog-only change changed inventory revision")
	}
	for _, tc := range []struct {
		name string
		plan *Snapshot
		want string
	}{{"original", first, original.Revision}, {"changed", second, changed.Revision}} {
		t.Run(tc.name, func(t *testing.T) {
			list, err := tc.plan.List("analytics.comparison", "all")
			if err != nil {
				t.Fatal(err)
			}
			detail, err := tc.plan.Get("core.analytics", list.Revision)
			if err != nil {
				t.Fatal(err)
			}
			for _, out := range []any{list, detail} {
				if got := catalogBindingIdentity(t, out); got != tc.want {
					t.Fatalf("catalog identity = %s, want %s", got, tc.want)
				}
			}
		})
	}
	loaded := loadInventory(t)
	body := mutated(t, loaded.body, func(value map[string]any) {
		value["items"].([]any)[0].(map[string]any)["task"] = "Synthetic changed quality task."
	})
	other, err := loadSnapshot(body, original)
	if err != nil {
		t.Fatal(err)
	}
	if other.revision == first.revision {
		t.Fatal("inventory mutation did not change inventory revision")
	}
	list, err := other.List("", "all")
	if err != nil {
		t.Fatal(err)
	}
	if got := catalogBindingIdentity(t, list); got != original.Revision {
		t.Fatal("inventory-only mutation changed catalog identity")
	}
}

func TestCatalogBindingIgnoresDetachedInspectionMutation(t *testing.T) {
	c := readerCatalog(t)
	want := c.Revision
	c.Revision = strings.Repeat("f", 64)
	c.Entries = nil
	plan, err := loadSnapshot(embeddedInventory, c)
	if err != nil {
		t.Fatalf("validated catalog was replaced by detached inspection fields: %v", err)
	}
	list, err := plan.List("analytics.comparison", "all")
	if err != nil {
		t.Fatal(err)
	}
	if got := catalogBindingIdentity(t, list); got != want {
		t.Fatalf("captured detached revision %q, want %q", got, want)
	}
}

func TestCatalogBindingGuardedReaderRefusesIndependentDrift(t *testing.T) {
	original, changed := readerCatalog(t), changedReaderCatalog(t)
	plan, err := ForCatalog(changed)
	if err != nil {
		t.Fatal(err)
	}
	list, err := plan.List("analytics.comparison", "all")
	if err != nil {
		t.Fatal(err)
	}
	detail, err := plan.Get("core.analytics", list.Revision)
	if err != nil {
		t.Fatal(err)
	}
	for _, guard := range []string{"", changed.Revision} {
		gotList, err := plan.ListForCatalog("analytics.comparison", "all", guard)
		if err != nil || !reflect.DeepEqual(gotList, list) {
			t.Fatalf("current/omitted list changed: %v", err)
		}
		gotDetail, err := plan.GetForCatalog("core.analytics", list.Revision, guard)
		if err != nil || !reflect.DeepEqual(gotDetail, detail) {
			t.Fatalf("current/omitted detail changed: %v", err)
		}
	}
	for _, guard := range []string{original.Revision, "bad", strings.Repeat("A", 64), strings.Repeat("g", 64), strings.Repeat("x", 4096)} {
		gotList, err := plan.ListForCatalog("", "all", guard)
		if err == nil || !reflect.DeepEqual(gotList, ListResult{}) {
			t.Fatal("invalid catalog guard returned list payload")
		}
		gotDetail, err := plan.GetForCatalog("core.analytics", list.Revision, guard)
		if err == nil || !reflect.DeepEqual(gotDetail, DetailResult{}) {
			t.Fatal("invalid catalog guard returned detail payload")
		}
	}
	for _, empty := range []*Snapshot{nil, {}} {
		if _, err := empty.ListForCatalog("", "all", ""); err == nil {
			t.Fatal("uninitialized list accepted")
		}
		if _, err := empty.GetForCatalog("core.analytics", "", ""); err == nil {
			t.Fatal("uninitialized detail accepted")
		}
	}
	for _, empty := range []*catalog.Catalog{nil, {}} {
		if _, err := ForCatalog(empty); err == nil {
			t.Fatal("unvalidated catalog accepted")
		}
	}
}

func TestCatalogBindingReadsEveryValidatedCatalogPage(t *testing.T) {
	c := readerCatalog(t)
	for i := 0; i < 55; i++ {
		entry := c.Entries[0]
		entry.ID = fmt.Sprintf("aaa.synthetic-%02d", i)
		c.Entries = append(c.Entries, entry)
	}
	body, err := json.Marshal(map[string]any{"schema_version": c.SchemaVersion, "entries": c.Entries})
	if err != nil {
		t.Fatal(err)
	}
	large, err := catalog.Load(body)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := ForCatalog(large)
	if err != nil {
		t.Fatalf("catalog pages after the first50 were lost: %v", err)
	}
	list, err := plan.ListForCatalog("analytics.comparison", "all", large.Revision)
	if err != nil || len(list.Items) == 0 {
		t.Fatalf("authored capability after first page was not retained: %v", err)
	}
}

func TestCatalogBindingNextActionsRetainReviewIdentities(t *testing.T) {
	plan, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	matched, err := plan.List("analytics.comparison", "core")
	if err != nil {
		t.Fatal(err)
	}
	if matched.Selection.Status != "matched" || !strings.Contains(matched.Selection.NextAction, "expected_revision=revision") || !strings.Contains(matched.Selection.NextAction, "expected_catalog_revision=catalog_revision") {
		t.Fatal("matched next action omits independent identity guards")
	}
	filtered, err := plan.List("analytics.comparison", "on-demand")
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Selection.Status != "filtered-out" || !strings.Contains(filtered.Selection.NextAction, "expected_catalog_revision=catalog_revision") {
		t.Fatal("scope retry loses catalog review guard")
	}
	for _, capability := range readerCatalog(t).Entries {
		list, err := plan.List(capability.ID, "all")
		if err != nil {
			t.Fatal(err)
		}
		if list.Selection.Status == "untracked" {
			if !strings.Contains(list.Selection.NextAction, "revision and catalog_revision") {
				t.Fatal("untracked contribution handoff loses one identity")
			}
			return
		}
	}
	t.Fatal("fixture needs an untracked capability")
}
