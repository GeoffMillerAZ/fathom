package adoption

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/catalog"
)

func pinFixtureV2(t *testing.T) (string, Selection) {
	t.Helper()
	root, s := adoptionCheckFixture(t)
	report := Pins(context.Background(), root, DefaultManifest)
	adoptionCheckOutcome(t, report.Report, 0, "observed")
	s.SchemaVersion, s.CatalogRevision = 2, ""
	s.CapabilityRevisions = &report.CapabilityRevisions
	adoptionCheckSave(t, root, s)
	return root, s
}

func TestCapabilityPinCanonicalEnvelope(t *testing.T) {
	entry := catalog.Entry{ID: "fixture.example", Name: "Example <&>"}
	pin, err := PinCapability(1, entry)
	if err != nil {
		t.Fatal(err)
	}
	// Independent literal fixes field order, escaping, null arrays and domain.
	canonical := `{"domain":"app-kit/catalog-entry","version":1,"catalog_schema_version":1,"entry":{"id":"fixture.example","name":"Example \u003c\u0026\u003e","kind":"","summary":"","tasks":null,"layers":null,"profiles":null,"dependencies":null,"source":{"path":"","symbols":null},"usage":"","states":null,"app_owns":null,"maturity":"","proof":null,"gaps":null,"provenance":{"source":"","license":"","audience":""},"consumers":null,"preview":""}}`
	sum := sha256.Sum256([]byte(canonical))
	if pin.ID != entry.ID || pin.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("canonical pin drift: %+v", pin)
	}
	other, err := PinCapability(2, entry)
	if err != nil || other.SHA256 == pin.SHA256 {
		t.Fatal("catalog schema version not bound")
	}
}

func TestCapabilityPinBindsEveryMetadataField(t *testing.T) {
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	entry, _ := c.Get("foundation.tokens")
	base, err := PinCapability(c.SchemaVersion, entry)
	if err != nil {
		t.Fatal(err)
	}
	var visit func(reflect.Value, []int, string)
	visit = func(value reflect.Value, indices []int, name string) {
		for i := 0; i < value.NumField(); i++ {
			field := value.Field(i)
			position := append(slices.Clone(indices), i)
			label := name + value.Type().Field(i).Name
			if field.Kind() == reflect.Struct {
				visit(field, position, label+".")
				continue
			}
			t.Run(label, func(t *testing.T) {
				changed, _ := c.Get(entry.ID)
				v := reflect.ValueOf(&changed).Elem().FieldByIndex(position)
				switch v.Kind() {
				case reflect.String:
					v.SetString(v.String() + " changed")
				case reflect.Slice:
					if v.Type() != reflect.TypeFor[[]string]() {
						t.Fatal("new metadata shape needs explicit pin contract review")
					}
					v.Set(reflect.ValueOf(append(slices.Clone(v.Interface().([]string)), "changed")))
				default:
					t.Fatal("new metadata shape needs explicit pin contract review")
				}
				got, err := PinCapability(c.SchemaVersion, changed)
				if err != nil || got.SHA256 == base.SHA256 {
					t.Fatalf("unbound field %s: %v", label, err)
				}
			})
		}
	}
	visit(reflect.ValueOf(entry), nil, "")
	entry.Tasks = []string{"one", "two"}
	a, _ := PinCapability(1, entry)
	slices.Reverse(entry.Tasks)
	b, _ := PinCapability(1, entry)
	if a.SHA256 == b.SHA256 {
		t.Fatal("metadata arrays must retain their order")
	}
}

func TestSelectionV2PinsAreExactNormalizedAndRequired(t *testing.T) {
	object := selectionTestObject(t)
	object["schema_version"] = 2
	delete(object, "catalog_revision")
	object["capability_revisions"] = []CapabilityRevision{{"shell.layout", strings.Repeat("a", 64)}, {"shell.appearance", strings.Repeat("b", 64)}}
	body := selectionTestJSON(t, object)
	original := bytes.Clone(body)
	s, err := Decode(body)
	if err != nil {
		t.Fatal(err)
	}
	if s.CatalogRevision != "" || s.CapabilityRevisions == nil || (*s.CapabilityRevisions)[0].ID != "shell.appearance" || !bytes.Equal(original, body) {
		t.Fatal("normalization changed declarations/input")
	}
	again, err := Decode(selectionTestJSON(t, s))
	if err != nil || !reflect.DeepEqual(again, s) {
		t.Fatalf("v2 roundtrip: %v", err)
	}
	for name, mutate := range map[string]func(map[string]any){
		"missing":   func(m map[string]any) { delete(m, "capability_revisions") },
		"null":      func(m map[string]any) { m["capability_revisions"] = nil },
		"empty":     func(m map[string]any) { m["capability_revisions"] = []any{} },
		"wrong ID":  func(m map[string]any) { m["capability_revisions"].([]any)[0].(map[string]any)["id"] = "unknown" },
		"duplicate": func(m map[string]any) { p := m["capability_revisions"].([]any); p[1] = p[0] },
		"extra": func(m map[string]any) {
			m["capability_revisions"] = append(m["capability_revisions"].([]any), map[string]any{"id": "other", "sha256": strings.Repeat("a", 64)})
		},
		"unknown field": func(m map[string]any) { m["capability_revisions"].([]any)[0].(map[string]any)["accepted"] = true },
		"case alias":    func(m map[string]any) { m["capability_revisions"].([]any)[0].(map[string]any)["ID"] = "shell.layout" },
		"bad hash": func(m map[string]any) {
			m["capability_revisions"].([]any)[0].(map[string]any)["sha256"] = strings.Repeat("A", 64)
		},
		"legacy field":       func(m map[string]any) { m["catalog_revision"] = strings.Repeat("a", 64) },
		"empty legacy field": func(m map[string]any) { m["catalog_revision"] = "" },
		"v1 new field":       func(m map[string]any) { m["schema_version"] = 1; m["catalog_revision"] = strings.Repeat("a", 64) },
		"unknown version":    func(m map[string]any) { m["schema_version"] = 3 },
	} {
		t.Run(name, func(t *testing.T) {
			var m map[string]any
			if err := json.Unmarshal(body, &m); err != nil {
				t.Fatal(err)
			}
			mutate(m)
			if _, err := Decode(selectionTestJSON(t, m)); err == nil {
				t.Fatal("ambiguous/missing pin accepted")
			}
		})
	}
	for _, explicitNull := range []bool{false, true} {
		m := selectionTestObject(t)
		m["schema_version"] = 2
		delete(m, "catalog_revision")
		m["capabilities"] = []string{}
		m["capability_revisions"] = []CapabilityRevision{}
		if explicitNull {
			m["capability_revisions"] = nil
		}
		got, err := Decode(selectionTestJSON(t, m))
		if explicitNull {
			if err == nil {
				t.Fatal("null accepted")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		encoded := selectionTestJSON(t, got)
		if !bytes.Contains(encoded, []byte(`"capability_revisions":[]`)) {
			t.Fatal("required empty array lost")
		}
	}
}

func TestCheckV2RetainsEverySelectedBoundary(t *testing.T) {
	for name, mutate := range map[string]func(*testing.T, string, *Selection){
		"pin drift": func(t *testing.T, r string, s *Selection) {
			(*s.CapabilityRevisions)[0].SHA256 = strings.Repeat("0", 64)
		},
		"unknown ID": func(t *testing.T, r string, s *Selection) {
			s.Capabilities[0] = "absent"
			(*s.CapabilityRevisions)[0].ID = "absent"
		},
		"profile": func(t *testing.T, r string, s *Selection) { s.Profile = "cli" },
		"layers":  func(t *testing.T, r string, s *Selection) { s.Layers = []string{"L4"} },
		"artifact drift": func(t *testing.T, r string, s *Selection) {
			adoptionCheckWrite(t, r, s.Artifacts[0].Path, []byte("changed"))
		},
		"missing artifact": func(t *testing.T, r string, s *Selection) {
			if err := os.Remove(filepath.Join(r, s.Artifacts[0].Path)); err != nil {
				t.Fatal(err)
			}
		},
		"router drift": func(t *testing.T, r string, s *Selection) {
			adoptionCheckWrite(t, r, "AGENTS.md", []byte("unrelated instructions only"))
		},
		"module": func(t *testing.T, r string, s *Selection) {
			s.Module = &ModuleRequirement{File: "go.mod", Version: "v1.0.0"}
			adoptionCheckWrite(t, r, "go.mod", []byte("module synthetic\nrequire github.com/geoffmilleraz/app-kit v0.0.0\n"))
		},
	} {
		t.Run(name, func(t *testing.T) {
			root, s := pinFixtureV2(t)
			adoptionCheckOutcome(t, Check(context.Background(), root, DefaultManifest), 0, "passed")
			mutate(t, root, &s)
			adoptionCheckSave(t, root, s)
			before := adoptionCheckSnapshot(t, root)
			adoptionCheckOutcome(t, Check(context.Background(), root, DefaultManifest), 1, "failed")
			if !reflect.DeepEqual(before, adoptionCheckSnapshot(t, root)) {
				t.Fatal("checker wrote consumer")
			}
		})
	}
	root, s := pinFixtureV2(t)
	s.Capabilities = []string{}
	empty := []CapabilityRevision{}
	s.CapabilityRevisions = &empty
	adoptionCheckSave(t, root, s)
	adoptionCheckOutcome(t, Check(context.Background(), root, DefaultManifest), 0, "passed")
	adoptionCheckWrite(t, root, s.Artifacts[0].Path, bytes.Repeat([]byte("x"), MaxFileBytes+1))
	adoptionCheckOutcome(t, Check(context.Background(), root, DefaultManifest), 2, "unavailable")
}

func TestSelectedMetadataPinsIgnoreOnlyUnselectedRecords(t *testing.T) {
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	selected, _ := c.Get("foundation.tokens")
	pin, err := PinCapability(c.SchemaVersion, selected)
	if err != nil {
		t.Fatal(err)
	}
	for _, changeSelected := range []bool{false, true} {
		entries := slices.Clone(c.Entries)
		changed := false
		for i := range entries {
			if (entries[i].ID == selected.ID) == changeSelected {
				entries[i].Summary += " Reviewed metadata delta."
				changed = true
				break
			}
		}
		if !changed {
			t.Fatal("canary target absent")
		}
		body := selectionTestJSON(t, struct {
			SchemaVersion int             `json:"schema_version"`
			Entries       []catalog.Entry `json:"entries"`
		}{c.SchemaVersion, entries})
		updated, err := catalog.Load(body)
		if err != nil {
			t.Fatal(err)
		}
		if updated.Revision == c.Revision {
			t.Fatal("global catalog did not change")
		}
		r := newReport("check")
		r.checkCapabilityRevisions(updated, []CapabilityRevision{pin})
		got := r.finish()
		if changeSelected {
			adoptionCheckOutcome(t, got, 1, "failed")
		} else {
			adoptionCheckOutcome(t, got, 0, "passed")
		}
	}
}

func TestPinsAreReadOnlyMetadataObservations(t *testing.T) {
	root, s := adoptionCheckFixture(t)
	s.CatalogRevision = strings.Repeat("0", 64)
	s.Profile = "cli"
	adoptionCheckSave(t, root, s)
	// Pin authoring must not read artifacts or execute a project to observe metadata.
	if err := os.Remove(filepath.Join(root, s.Artifacts[0].Path)); err != nil {
		t.Fatal(err)
	}
	before := adoptionCheckSnapshot(t, root)
	first := Pins(context.Background(), root, DefaultManifest)
	again := Pins(context.Background(), root, DefaultManifest)
	adoptionCheckOutcome(t, first.Report, 0, "observed")
	if len(first.CapabilityRevisions) != 1 || len(first.Entries) != 1 || first.Entries[0].ID != first.CapabilityRevisions[0].ID {
		t.Fatal("pin has no full reviewable record")
	}
	if !reflect.DeepEqual(first, again) || !reflect.DeepEqual(before, adoptionCheckSnapshot(t, root)) {
		t.Fatal("non-deterministic or writing pins")
	}
	adoptionCheckOutcome(t, Check(context.Background(), root, DefaultManifest), 1, "failed")
	s.Capabilities = []string{"absent"}
	adoptionCheckSave(t, root, s)
	adoptionCheckOutcome(t, Pins(context.Background(), root, DefaultManifest).Report, 1, "failed")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	adoptionCheckOutcome(t, Pins(ctx, root, DefaultManifest).Report, 2, "unavailable")
}
