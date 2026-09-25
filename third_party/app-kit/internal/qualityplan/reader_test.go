package qualityplan

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/catalog"
)

func readerCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func readerLoad(t *testing.T, body []byte) *Snapshot {
	t.Helper()
	snapshot, err := loadSnapshot(body, readerCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func readerDocument(t *testing.T) inventoryDocument {
	t.Helper()
	loaded := loadInventory(t)
	var document inventoryDocument
	if err := json.Unmarshal(loaded.body, &document); err != nil {
		t.Fatal(err)
	}
	// #Inventory supplies empty-list defaults for not-started evidence. The raw
	// package fixture omits those defaults; the validated reader materializes them.
	for itemIndex := range document.Items {
		for id, status := range document.Items[itemIndex].Standards {
			if status.Evidence == nil {
				status.Evidence = []Ref{}
				document.Items[itemIndex].Standards[id] = status
			}
		}
		for id, status := range document.Items[itemIndex].Gates {
			if status.Evidence == nil {
				status.Evidence = []Ref{}
				document.Items[itemIndex].Gates[id] = status
			}
		}
	}
	sort.Slice(document.Items, func(i, j int) bool { return document.Items[i].ID < document.Items[j].ID })
	return document
}

func TestDefaultFaithfullyProjectsCUEInventory(t *testing.T) {
	snapshot, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	want := readerDocument(t)
	list, err := snapshot.List("", "all")
	if err != nil {
		t.Fatal(err)
	}
	if list.Revision == "" || list.EnhancementStandardRevision != want.EnhancementStandardRevision || list.ClaimBoundary != claimBoundary {
		t.Fatalf("unexpected list identity: %#v", list)
	}
	if len(list.Items) != len(want.Items) {
		t.Fatalf("list has %d items; want %d", len(list.Items), len(want.Items))
	}
	for i, expected := range want.Items {
		summary := list.Items[i]
		if summary.ID != expected.ID || summary.Kind != expected.Kind || summary.Task != expected.Task || summary.Profile != expected.Profile || summary.Core != expected.Core || summary.State != expected.State || !reflect.DeepEqual(summary.CapabilityIDs, expected.CapabilityIDs) {
			t.Fatalf("summary %d differs:\n got %#v\nwant %#v", i, summary, expected)
		}
		detail, err := snapshot.Get(expected.ID, list.Revision)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(detail.Item, expected) || !reflect.DeepEqual(detail.Standards, want.Standards) || !reflect.DeepEqual(detail.BacklogPolicy, want.BacklogPolicy) {
			t.Fatalf("detail %q dropped authored fields:\nitem equal=%t\nstandards equal=%t\npolicy equal=%t\n got item=%#v\nwant item=%#v", expected.ID,
				reflect.DeepEqual(detail.Item, expected), reflect.DeepEqual(detail.Standards, want.Standards), reflect.DeepEqual(detail.BacklogPolicy, want.BacklogPolicy), detail.Item, expected)
		}
		if detail.Revision != list.Revision || detail.EnhancementStandardRevision != want.EnhancementStandardRevision || detail.ClaimBoundary != claimBoundary {
			t.Fatalf("detail %q identity differs: %#v", expected.ID, detail)
		}
	}
}

func TestRevisionIsCanonicalAcrossFormattingAndItemOrder(t *testing.T) {
	loaded := loadInventory(t)
	base := readerLoad(t, loaded.body)
	reordered := mutated(t, loaded.body, func(value map[string]any) {
		items := value["items"].([]any)
		for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
			items[left], items[right] = items[right], items[left]
		}
	})
	formatted := new(strings.Builder)
	var value any
	if err := json.Unmarshal(reordered, &value); err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(formatted)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	other := readerLoad(t, []byte(formatted.String()))
	if base.revision != other.revision {
		t.Fatalf("format/order changed revision: %s != %s", base.revision, other.revision)
	}
	changed := mutated(t, loaded.body, func(value map[string]any) {
		value["items"].([]any)[0].(map[string]any)["task"] = "A materially changed authored task."
	})
	if got := readerLoad(t, changed).revision; got == base.revision {
		t.Fatal("metadata change did not change revision")
	}
}

func TestListScopesCapabilityValidationAndEmptyKnownMatch(t *testing.T) {
	snapshot, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	core, err := snapshot.List("", "")
	if err != nil {
		t.Fatal(err)
	}
	onDemand, err := snapshot.List("", "on-demand")
	if err != nil {
		t.Fatal(err)
	}
	all, err := snapshot.List("", "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(core.Items)+len(onDemand.Items) != len(all.Items) {
		t.Fatalf("scopes do not partition inventory: core=%d on-demand=%d all=%d", len(core.Items), len(onDemand.Items), len(all.Items))
	}
	for _, summary := range core.Items {
		if !summary.Core {
			t.Fatalf("core scope returned non-core item %q", summary.ID)
		}
	}
	for _, summary := range onDemand.Items {
		if summary.Core {
			t.Fatalf("on-demand scope returned core item %q", summary.ID)
		}
	}

	represented := map[string]bool{}
	for _, summary := range all.Items {
		for _, id := range summary.CapabilityIDs {
			represented[id] = true
		}
	}
	var unrepresented string
	for _, entry := range readerCatalog(t).Entries {
		if !represented[entry.ID] {
			unrepresented = entry.ID
			break
		}
	}
	if unrepresented == "" {
		t.Fatal("test requires one known unrepresented capability")
	}
	empty, err := snapshot.List(unrepresented, "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.Items) != 0 {
		t.Fatalf("known unrepresented capability returned items: %#v", empty.Items)
	}
	if _, err := snapshot.List("missing.capability", "all"); err == nil {
		t.Fatal("unknown capability accepted")
	}
	if _, err := snapshot.List("shell.layout", "invalid"); err == nil {
		t.Fatal("unknown scope accepted")
	}
	long := strings.Repeat("x", 4096)
	if _, err := snapshot.List(long, "all"); err == nil || strings.Contains(err.Error(), long) {
		t.Fatalf("overlong capability was accepted or reflected: %v", err)
	}
	if _, err := snapshot.List("", long); err == nil || strings.Contains(err.Error(), long) {
		t.Fatalf("overlong scope was accepted or reflected: %v", err)
	}
}

func TestGetRejectsStaleRevisionAndUnknownInputs(t *testing.T) {
	snapshot, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	all, err := snapshot.List("", "all")
	if err != nil {
		t.Fatal(err)
	}
	id := all.Items[0].ID
	if _, err := snapshot.Get(id, strings.Repeat("0", 64)); err == nil {
		t.Fatal("stale revision accepted")
	}
	if _, err := snapshot.Get(id, strings.Repeat("A", 64)); err == nil {
		t.Fatal("uppercase expected revision accepted")
	}
	if _, err := snapshot.Get("missing.item", ""); err == nil {
		t.Fatal("unknown item accepted")
	}
	long := strings.Repeat("x", 4096)
	if _, err := snapshot.Get(long, ""); err == nil || strings.Contains(err.Error(), long) {
		t.Fatalf("overlong item was accepted or reflected: %v", err)
	}
}

func TestResultsAreDeepDetached(t *testing.T) {
	snapshot, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	list, err := snapshot.List("", "all")
	if err != nil {
		t.Fatal(err)
	}
	originalID := list.Items[0].CapabilityIDs[0]
	list.Items[0].CapabilityIDs[0] = "mutated"
	again, err := snapshot.List("", "all")
	if err != nil {
		t.Fatal(err)
	}
	if again.Items[0].CapabilityIDs[0] != originalID {
		t.Fatal("list mutation changed snapshot")
	}

	detail, err := snapshot.Get(again.Items[0].ID, again.Revision)
	if err != nil {
		t.Fatal(err)
	}
	standardID := "typed-config"
	gateID := "structure"
	detail.Item.CapabilityIDs[0] = "mutated"
	status := detail.Item.Standards[standardID]
	status.Status = "mutated"
	detail.Item.Standards[standardID] = status
	definition := detail.Standards[standardID]
	definition.GateRefs[0] = "mutated"
	detail.Standards[standardID] = definition
	detail.BacklogPolicy.SelectionRequires[0] = "mutated"
	detail.Item.Gates[gateID] = GateStatus{Status: "mutated"}
	next, err := snapshot.Get(again.Items[0].ID, again.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if next.Item.CapabilityIDs[0] != originalID || next.Item.Standards[standardID].Status == "mutated" || next.Standards[standardID].GateRefs[0] == "mutated" || next.BacklogPolicy.SelectionRequires[0] == "mutated" || next.Item.Gates[gateID].Status == "mutated" {
		t.Fatal("detail mutation changed snapshot")
	}
}

func TestReaderRetainsSelectionReceiptsReviewsAndConsumersWithoutCheckingFiles(t *testing.T) {
	loaded := loadInventory(t)
	hash := strings.Repeat("a", 64)
	body := mutated(t, loaded.body, func(value map[string]any) {
		items := value["items"].([]any)
		candidate := items[len(items)-1].(map[string]any)
		candidate["kind"] = "conversion"
		candidate["state"] = "in-progress"
		candidate["selection"] = map[string]any{
			"original":            map[string]any{"kind": "private-source", "id": "Original Name", "revision_sha256": hash},
			"target":              map[string]any{"id": "Target Name", "revision_sha256": hash},
			"acceptance_contract": map[string]any{"path": "does/not/exist.md", "sha256": hash},
			"workload":            "A bounded authored workload.",
			"budgets":             []any{map[string]any{"name": "payload", "limit": "At most one byte."}},
		}
		first := items[0].(map[string]any)
		first["gates"].(map[string]any)["structure"] = map[string]any{
			"status": "passed", "evidence": []any{map[string]any{"path": "does/not/exist.md", "note": "Authored only."}},
			"receipt":         map[string]any{"path": "does/not/exist.json", "sha256": hash},
			"source_revision": hash, "artifact_revision": hash,
			"review": map[string]any{"by": "reviewer", "ref": "does/not/exist.md"},
		}
		consumer := first["consumers"].([]any)[0].(map[string]any)
		consumer["integration_status"] = "adopted"
		consumer["receipt"] = map[string]any{"path": "does/not/exist.json", "sha256": hash}
		consumer["review"] = map[string]any{"by": "reviewer", "ref": "does/not/exist.md"}
	})
	snapshot := readerLoad(t, body)
	all, err := snapshot.List("", "all")
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := snapshot.Get("candidate.personal-site-series-navigation", all.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Item.Selection == nil || candidate.Item.Selection.AcceptanceContract.Path != "does/not/exist.md" || len(candidate.Item.Selection.Budgets) != 1 {
		t.Fatalf("selection dropped: %#v", candidate.Item.Selection)
	}
	first, err := snapshot.Get("core.shell-settings", all.Revision)
	if err != nil {
		t.Fatal(err)
	}
	gate := first.Item.Gates["structure"]
	if gate.Receipt == nil || gate.Review == nil || gate.SourceRevision == "" || gate.ArtifactRevision == "" {
		t.Fatalf("gate pins/review dropped: %#v", gate)
	}
	if len(first.Item.Consumers) != 1 || first.Item.Consumers[0].Receipt == nil || first.Item.Consumers[0].Review == nil || first.Item.Consumers[0].Revision.SHA256 == "" {
		t.Fatalf("consumer fields dropped: %#v", first.Item.Consumers)
	}
	if first.ClaimBoundary != claimBoundary {
		t.Fatal("authored-only claim boundary missing")
	}
}

func TestReaderRejectsSchemaDuplicateAndCatalogProfileDefects(t *testing.T) {
	loaded := loadInventory(t)
	cases := map[string]func(map[string]any){
		"schema": func(value map[string]any) {
			delete(value, "backlog_policy")
		},
		"duplicate": func(value map[string]any) {
			items := value["items"].([]any)
			value["items"] = append(items, items[0])
		},
		"unknown capability": func(value map[string]any) {
			value["items"].([]any)[0].(map[string]any)["capability_ids"] = []any{"missing.capability"}
		},
		"profile mismatch": func(value map[string]any) {
			value["items"].([]any)[0].(map[string]any)["profile"] = "portable"
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			body := mutated(t, loaded.body, change)
			if _, err := loadSnapshot(body, readerCatalog(t)); err == nil {
				t.Fatal("invalid inventory accepted")
			}
		})
	}
}

func TestReaderRefusesDuplicateConsumerIdentityWithinItem(t *testing.T) {
	loaded := loadInventory(t)
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"identical", func(map[string]any) {}},
		{"conflicting status", func(consumer map[string]any) { consumer["integration_status"] = "selected" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := mutated(t, loaded.body, func(value map[string]any) {
				item := value["items"].([]any)[0].(map[string]any)
				original := item["consumers"].([]any)[0].(map[string]any)
				duplicate := make(map[string]any, len(original))
				for key, field := range original {
					duplicate[key] = field
				}
				tc.change(duplicate)
				item["consumers"] = append(item["consumers"].([]any), duplicate)
			})
			if snapshot, err := loadSnapshot(body, readerCatalog(t)); err == nil || snapshot != nil || !strings.Contains(err.Error(), "duplicate consumer identity") {
				t.Fatalf("duplicate consumer reached snapshot: snapshot=%v err=%v", snapshot, err)
			}
		})
	}
}

func TestReaderAllowsDistinctConsumerIdentities(t *testing.T) {
	loaded := loadInventory(t)
	for _, tc := range []struct {
		name      string
		change    func(map[string]any)
		otherItem bool
	}{
		{"different app", func(consumer map[string]any) { consumer["app"] = "another-app" }, false},
		{"different revision kind", func(consumer map[string]any) {
			revision := consumer["revision"].(map[string]any)
			consumer["revision"] = map[string]any{"kind": "source", "sha256": revision["sha256"]}
		}, false},
		{"different revision hash", func(consumer map[string]any) {
			revision := consumer["revision"].(map[string]any)
			consumer["revision"] = map[string]any{"kind": revision["kind"], "sha256": strings.Repeat("a", 64)}
		}, false},
		{"same identity in another item", func(map[string]any) {}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := mutated(t, loaded.body, func(value map[string]any) {
				items := value["items"].([]any)
				first := items[0].(map[string]any)
				original := first["consumers"].([]any)[0].(map[string]any)
				consumer := make(map[string]any, len(original))
				for key, field := range original {
					consumer[key] = field
				}
				tc.change(consumer)
				target := first
				if tc.otherItem {
					target = items[1].(map[string]any)
				}
				target["consumers"] = append(target["consumers"].([]any), consumer)
			})
			if snapshot, err := loadSnapshot(body, readerCatalog(t)); err != nil || snapshot == nil {
				t.Fatalf("distinct consumer identity rejected: %v", err)
			}
		})
	}
}

func TestReaderRefusesOversizedSerializedResults(t *testing.T) {
	loaded := loadInventory(t)
	largeList := mutated(t, loaded.body, func(value map[string]any) {
		original := value["items"].([]any)[0].(map[string]any)
		items := make([]any, 32)
		for i := range items {
			body, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			var clone map[string]any
			if err := json.Unmarshal(body, &clone); err != nil {
				t.Fatal(err)
			}
			clone["id"] = "core.bulk" + string(rune('a'+i/26)) + string(rune('a'+i%26))
			clone["task"] = strings.Repeat("x", 2048)
			items[i] = clone
		}
		value["items"] = items
	})
	if _, err := readerLoad(t, largeList).List("", "core"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized list not refused: %v", err)
	}

	largeDetail := mutated(t, loaded.body, func(value map[string]any) {
		item := value["items"].([]any)[0].(map[string]any)
		for name := range item["gates"].(map[string]any) {
			evidence := make([]any, 32)
			for i := range evidence {
				evidence[i] = map[string]any{"path": "evidence/file", "note": strings.Repeat("x", 600)}
			}
			item["gates"].(map[string]any)[name] = map[string]any{
				"status": "partial", "reason": "Bounded authored partial claim.", "next_step": "Run the remaining gate.", "evidence": evidence,
			}
		}
	})
	snapshot := readerLoad(t, largeDetail)
	if _, err := snapshot.Get("core.shell-settings", ""); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized detail not refused: %v", err)
	}
}
