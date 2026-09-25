package qualityplan

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

var canonicalGates = []string{"data", "integration", "interaction", "lifecycle", "performance", "reproducibility", "security", "structure", "visual"}

var canonicalCore = []string{
	"core.analytics",
	"core.client-heavy-boundary",
	"core.shell-settings",
	"core.static-content",
	"core.themes-style-guide",
}

var standardCrosswalk = map[string][]string{
	"typed-config":          {"integration", "security", "structure"},
	"semantic-theme":        {"data", "interaction", "visual"},
	"native-interaction":    {"interaction", "lifecycle", "structure"},
	"honest-states":         {"data", "integration", "interaction"},
	"bounded-performance":   {"lifecycle", "performance"},
	"safe-composition":      {"integration", "security", "structure"},
	"reproducible-delivery": {"integration", "reproducibility"},
}

type ref struct {
	Path string `json:"path"`
}

type pinnedRef struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type review struct {
	By  string `json:"by"`
	Ref string `json:"ref"`
}

type status struct {
	Status           string     `json:"status"`
	Evidence         []ref      `json:"evidence"`
	Review           *review    `json:"review"`
	Receipt          *pinnedRef `json:"receipt"`
	SourceRevision   string     `json:"source_revision"`
	ArtifactRevision string     `json:"artifact_revision"`
}

type consumer struct {
	App      string `json:"app"`
	Revision struct {
		Kind   string `json:"kind"`
		SHA256 string `json:"sha256"`
	} `json:"revision"`
	IntegrationStatus string     `json:"integration_status"`
	Evidence          []ref      `json:"evidence"`
	Receipt           *pinnedRef `json:"receipt"`
	Review            *review    `json:"review"`
}

type selection struct {
	Original struct {
		Kind           string `json:"kind"`
		ID             string `json:"id"`
		RevisionSHA256 string `json:"revision_sha256"`
	} `json:"original"`
	Target struct {
		ID             string `json:"id"`
		RevisionSHA256 string `json:"revision_sha256"`
	} `json:"target"`
	AcceptanceContract pinnedRef `json:"acceptance_contract"`
	Workload           string    `json:"workload"`
	Budgets            []struct {
		Name  string `json:"name"`
		Limit string `json:"limit"`
	} `json:"budgets"`
}

type item struct {
	ID            string            `json:"id"`
	Kind          string            `json:"kind"`
	Profile       string            `json:"profile"`
	Priority      string            `json:"priority"`
	Core          bool              `json:"core"`
	State         string            `json:"state"`
	CapabilityIDs []string          `json:"capability_ids"`
	SourceRefs    []ref             `json:"source_refs"`
	Standards     map[string]status `json:"standards"`
	Gates         map[string]status `json:"gates"`
	Selection     *selection        `json:"selection"`
	Consumers     []consumer        `json:"consumers"`
}

type inventory struct {
	SchemaVersion int    `json:"schema_version"`
	Revision      string `json:"enhancement_standard_revision"`
	Standards     map[string]struct {
		GateRefs []string `json:"gate_refs"`
	} `json:"standards"`
	BacklogPolicy struct {
		Mode            string   `json:"mode"`
		Selection       []string `json:"selection_requires"`
		OwnerBatch      []string `json:"owner_batch_requires"`
		UnselectedClaim string   `json:"unselected_claim"`
	} `json:"backlog_policy"`
	Items []item `json:"items"`
}

type loadedInventory struct {
	ctx    *cue.Context
	schema cue.Value
	body   []byte
	doc    inventory
}

type contributionDecision struct {
	ID       string `json:"id"`
	Decision string `json:"decision"`
}

type contributionRehearsal struct {
	Schema    string `json:"schema"`
	Synthetic bool   `json:"synthetic"`
	WorkItem  struct {
		ID                string `json:"id"`
		KitRevisionSHA256 string `json:"kit_revision_sha256"`
	} `json:"work_item"`
	Originating struct {
		App               string `json:"app"`
		Task              string `json:"task"`
		AppRevisionSHA256 string `json:"app_revision_sha256"`
		Result            string `json:"result"`
	} `json:"originating"`
	Selection struct {
		OriginalKind           string      `json:"original_kind"`
		OriginalID             string      `json:"original_id"`
		OriginalRevisionSHA256 string      `json:"original_revision_sha256"`
		TargetID               string      `json:"target_id"`
		TargetRevisionSHA256   string      `json:"target_revision_sha256"`
		AcceptanceContract     pinnedRef   `json:"acceptance_contract"`
		Workload               string      `json:"workload"`
		Budgets                []jitBudget `json:"budgets"`
	} `json:"selection"`
	Standards []contributionDecision `json:"standards"`
	Gates     []contributionDecision `json:"gates"`
	Receipt   struct {
		Path   string `json:"path"`
		Review review `json:"review"`
	} `json:"receipt"`
	Contribution struct {
		ID                string `json:"id"`
		KitRevisionSHA256 string `json:"kit_revision_sha256"`
		Observed          string `json:"observed"`
		Expected          string `json:"expected"`
		Ownership         string `json:"ownership"`
		Disposition       string `json:"disposition"`
		Lesson            string `json:"lesson"`
		Test              string `json:"test"`
		NextAction        struct {
			Task    string `json:"task"`
			Owner   string `json:"owner"`
			Revisit string `json:"revisit"`
		} `json:"next_action"`
		Delivery struct {
			Kind                 string `json:"kind"`
			SelectionTargetID    string `json:"selection_target_id"`
			TargetRevisionSHA256 string `json:"target_revision_sha256"`
			OriginatingFork      string `json:"originating_fork"`
		} `json:"delivery"`
	} `json:"contribution"`
	Verification struct {
		Originating    string `json:"originating"`
		SecondConsumer string `json:"second_consumer"`
		Propagation    string `json:"propagation"`
		Promotion      string `json:"promotion"`
	} `json:"verification"`
	Limits []string `json:"limits"`
}

type jitBudget struct {
	Name  string `json:"name"`
	Limit string `json:"limit"`
}

func packageDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func repoRoot(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join(packageDir(t), "..", ".."))
}

func loadInventory(t *testing.T) loadedInventory {
	t.Helper()
	ctx := cuecontext.New()
	instances := load.Instances([]string{"."}, &load.Config{Dir: packageDir(t)})
	if len(instances) != 1 || instances[0].Err != nil {
		t.Fatalf("load CUE package: %#v", instances)
	}
	pkg := ctx.BuildInstance(instances[0])
	if err := pkg.Err(); err != nil {
		t.Fatal(err)
	}
	schema := pkg.LookupPath(cue.ParsePath("#Inventory"))
	body, err := pkg.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	value := ctx.CompileBytes(body, cue.Filename("inventory.json")).Unify(schema)
	if err := value.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("concrete inventory does not satisfy #Inventory: %v", err)
	}
	var doc inventory
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	return loadedInventory{ctx: ctx, schema: schema, body: body, doc: doc}
}

func mutated(t *testing.T, body []byte, change func(map[string]any)) []byte {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatal(err)
	}
	change(value)
	out, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func cueAccepts(loaded loadedInventory, body []byte) bool {
	return loaded.ctx.CompileBytes(body).Unify(loaded.schema).Validate(cue.Concrete(true)) == nil
}

func decodeInventory(t *testing.T, body []byte) inventory {
	t.Helper()
	var doc inventory
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(body))
}

func sorted(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func mapKeys[V any](values map[string]V) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	return sorted(out)
}

func cueFields(t *testing.T, value cue.Value) []string {
	t.Helper()
	iter, err := value.Fields(cue.Optional(true), cue.Definitions(false), cue.Hidden(false))
	if err != nil {
		t.Fatal(err)
	}
	var fields []string
	for iter.Next() {
		fields = append(fields, iter.Label())
	}
	return sorted(fields)
}

func designReceiptGateNames(t *testing.T) []string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "designreceipts", "schema.cue"))
	if err != nil {
		t.Fatal(err)
	}
	value := cuecontext.New().CompileBytes(body, cue.Filename("designreceipts/schema.cue"))
	if err := value.Err(); err != nil {
		t.Fatal(err)
	}
	return cueFields(t, value.LookupPath(cue.ParsePath("#Gates")))
}

type catalogEntry struct {
	ID       string   `json:"id"`
	Profiles []string `json:"profiles"`
}

func catalogProfiles(t *testing.T) map[string]map[string]bool {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "appkit", "catalog", "catalog.cue"))
	if err != nil {
		t.Fatal(err)
	}
	value := cuecontext.New().CompileBytes(body, cue.Filename("catalog.cue"))
	if err := value.Err(); err != nil {
		t.Fatal(err)
	}
	jsonBody, err := value.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Entries []catalogEntry `json:"entries"`
	}
	if err := json.Unmarshal(jsonBody, &catalog); err != nil {
		t.Fatal(err)
	}
	out := make(map[string]map[string]bool, len(catalog.Entries))
	for _, entry := range catalog.Entries {
		profiles := make(map[string]bool, len(entry.Profiles))
		for _, profile := range entry.Profiles {
			profiles[profile] = true
		}
		out[entry.ID] = profiles
	}
	return out
}

func regularRepoFile(root, path string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rootResolved, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", err
	}
	candidate := filepath.Clean(filepath.Join(rootAbs, filepath.FromSlash(path)))
	rel, err := filepath.Rel(rootAbs, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes repository")
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}
	rel, err = filepath.Rel(rootResolved, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("symlink escapes repository")
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("not a regular file")
	}
	return resolved, nil
}

func validatePinned(root string, pinned pinnedRef) error {
	path, err := regularRepoFile(root, pinned.Path)
	if err != nil {
		return err
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	got := fmt.Sprintf("%x", sha256.Sum256(body))
	if got != pinned.SHA256 {
		return fmt.Errorf("sha256 mismatch: got %s", got)
	}
	return nil
}

func validateInventory(doc inventory, profiles map[string]map[string]bool, root string) []string {
	var failures []string
	seen, core := map[string]bool{}, []string{}
	for _, it := range doc.Items {
		if seen[it.ID] {
			failures = append(failures, "duplicate item "+it.ID)
		}
		seen[it.ID] = true
		if it.Core {
			core = append(core, it.ID)
		}
		for _, capability := range it.CapabilityIDs {
			allowed, ok := profiles[capability]
			if !ok {
				failures = append(failures, fmt.Sprintf("%s: unknown capability %s", it.ID, capability))
			} else if !allowed[it.Profile] {
				failures = append(failures, fmt.Sprintf("%s: capability %s does not support %s", it.ID, capability, it.Profile))
			}
		}
		if !reflect.DeepEqual(mapKeys(it.Gates), canonicalGates) {
			failures = append(failures, it.ID+": gate set drift")
		}
		if !reflect.DeepEqual(mapKeys(it.Standards), mapKeys(standardCrosswalk)) {
			failures = append(failures, it.ID+": standard set drift")
		}
		refs := append([]ref(nil), it.SourceRefs...)
		var pins []pinnedRef
		for _, state := range it.Standards {
			refs = append(refs, state.Evidence...)
			if state.Review != nil {
				refs = append(refs, ref{Path: state.Review.Ref})
			}
		}
		for _, gate := range it.Gates {
			refs = append(refs, gate.Evidence...)
			if gate.Review != nil {
				refs = append(refs, ref{Path: gate.Review.Ref})
			}
			if gate.Receipt != nil {
				pins = append(pins, *gate.Receipt)
			}
		}
		if it.Selection != nil {
			pins = append(pins, it.Selection.AcceptanceContract)
		}
		for _, consumer := range it.Consumers {
			refs = append(refs, consumer.Evidence...)
			if consumer.Review != nil {
				refs = append(refs, ref{Path: consumer.Review.Ref})
			}
			if consumer.Receipt != nil {
				pins = append(pins, *consumer.Receipt)
			}
		}
		for _, reference := range refs {
			if _, err := regularRepoFile(root, reference.Path); err != nil {
				failures = append(failures, fmt.Sprintf("%s: invalid file reference %s: %v", it.ID, reference.Path, err))
			}
		}
		for _, pinned := range pins {
			if err := validatePinned(root, pinned); err != nil {
				failures = append(failures, fmt.Sprintf("%s: invalid pinned file %s: %v", it.ID, pinned.Path, err))
			}
		}
	}
	if !reflect.DeepEqual(sorted(core), canonicalCore) {
		failures = append(failures, "core scope drift")
	}
	return sorted(failures)
}

func TestInventoryCompilesAndTracksIndependentFacts(t *testing.T) {
	loaded := loadInventory(t)
	if !reflect.DeepEqual(designReceiptGateNames(t), canonicalGates) {
		t.Fatalf("design receipt gate names drifted: %v", designReceiptGateNames(t))
	}
	if loaded.doc.SchemaVersion != 1 || loaded.doc.Revision != "converted-core-v1" {
		t.Fatalf("unexpected inventory identity: %#v", loaded.doc)
	}
	if loaded.doc.BacklogPolicy.Mode != "just-in-time" || len(loaded.doc.BacklogPolicy.Selection) < 3 || len(loaded.doc.BacklogPolicy.OwnerBatch) < 2 || loaded.doc.BacklogPolicy.UnselectedClaim == "" {
		t.Fatalf("long-tail policy is not explicit: %#v", loaded.doc.BacklogPolicy)
	}
	for id, gates := range standardCrosswalk {
		got, ok := loaded.doc.Standards[id]
		if !ok || !reflect.DeepEqual(sorted(got.GateRefs), sorted(gates)) {
			t.Fatalf("standard %s crosswalk drift: got %v want %v", id, got.GateRefs, gates)
		}
	}
	if failures := validateInventory(loaded.doc, catalogProfiles(t), repoRoot(t)); len(failures) != 0 {
		t.Fatalf("inventory semantic failures:\n%s", strings.Join(failures, "\n"))
	}
	if len(loaded.doc.Items[0].Consumers) != 1 || loaded.doc.Items[0].Consumers[0].Revision.Kind == "" || loaded.doc.Items[0].Consumers[0].Revision.SHA256 == "" || loaded.doc.Items[0].Consumers[0].IntegrationStatus == "" {
		t.Fatalf("consumer revision/status were not retained: %#v", loaded.doc.Items[0].Consumers)
	}
}

func TestCUESchemaRejectsFalseGateClaims(t *testing.T) {
	loaded := loadInventory(t)
	hash := strings.Repeat("a", 64)
	validPass := func() map[string]any {
		return map[string]any{
			"status":            "passed",
			"evidence":          []any{map[string]any{"path": "internal/qualityplan/schema.cue", "note": "Exact retained evidence."}},
			"receipt":           map[string]any{"path": "internal/qualityplan/inventory.cue", "sha256": hash},
			"source_revision":   hash,
			"artifact_revision": hash,
			"review":            map[string]any{"by": "independent reviewer", "ref": "docs/design/converted-component-standard.md"},
		}
	}
	validImplemented := func() map[string]any {
		return map[string]any{
			"status":   "implemented",
			"evidence": []any{map[string]any{"path": "internal/qualityplan/schema.cue", "note": "Exact implementation evidence."}},
			"review":   map[string]any{"by": "independent reviewer", "ref": "docs/design/converted-component-standard.md"},
		}
	}

	cases := map[string]func(map[string]any){
		"false acceptance": func(v map[string]any) {
			v["items"].([]any)[0].(map[string]any)["state"] = "accepted"
		},
		"accepted with partial standard": func(v map[string]any) {
			item := v["items"].([]any)[0].(map[string]any)
			item["state"] = "accepted"
			for name := range item["gates"].(map[string]any) {
				item["gates"].(map[string]any)[name] = validPass()
			}
		},
		"accepted with incomplete gate": func(v map[string]any) {
			item := v["items"].([]any)[0].(map[string]any)
			item["state"] = "accepted"
			for name := range item["standards"].(map[string]any) {
				item["standards"].(map[string]any)[name] = validImplemented()
			}
		},
		"adopted consumer missing reviewed receipt": func(v map[string]any) {
			consumer := v["items"].([]any)[0].(map[string]any)["consumers"].([]any)[0].(map[string]any)
			consumer["integration_status"] = "adopted"
		},
		"candidate cannot start without conversion selection": func(v map[string]any) {
			items := v["items"].([]any)
			items[len(items)-1].(map[string]any)["state"] = "in-progress"
		},
		"invalid status": func(v map[string]any) {
			v["items"].([]any)[0].(map[string]any)["gates"].(map[string]any)["structure"].(map[string]any)["status"] = "green"
		},
		"unjustified not applicable": func(v map[string]any) {
			gate := v["items"].([]any)[0].(map[string]any)["gates"].(map[string]any)["structure"].(map[string]any)
			gate["status"], gate["evidence"] = "not-applicable", []any{}
			delete(gate, "reason")
			delete(gate, "review")
		},
		"unknown profile": func(v map[string]any) {
			v["items"].([]any)[0].(map[string]any)["profile"] = "quantum-web"
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			if cueAccepts(loaded, mutated(t, loaded.body, change)) {
				t.Fatal("invalid inventory accepted")
			}
		})
	}

	for _, missing := range []string{"evidence", "receipt", "source_revision", "artifact_revision", "review"} {
		t.Run("passed missing "+missing, func(t *testing.T) {
			body := mutated(t, loaded.body, func(v map[string]any) {
				gate := validPass()
				if missing == "evidence" {
					gate[missing] = []any{}
				} else {
					delete(gate, missing)
				}
				v["items"].([]any)[0].(map[string]any)["gates"].(map[string]any)["structure"] = gate
			})
			if cueAccepts(loaded, body) {
				t.Fatal("incomplete passed gate accepted")
			}
		})
	}
}

func TestAcceptedItemIsValidOutsideTheCurrentSeed(t *testing.T) {
	loaded := loadInventory(t)
	receiptPath := "internal/qualityplan/inventory.cue"
	receiptSHA := fileSHA256(t, filepath.Join(repoRoot(t), receiptPath))
	body := mutated(t, loaded.body, func(v map[string]any) {
		item := v["items"].([]any)[0].(map[string]any)
		item["state"] = "accepted"
		for name := range item["standards"].(map[string]any) {
			item["standards"].(map[string]any)[name] = map[string]any{
				"status":   "implemented",
				"evidence": []any{map[string]any{"path": "internal/qualityplan/schema.cue", "note": "Exact implementation evidence."}},
				"review":   map[string]any{"by": "independent reviewer", "ref": "docs/design/converted-component-standard.md"},
			}
		}
		for name := range item["gates"].(map[string]any) {
			item["gates"].(map[string]any)[name] = map[string]any{
				"status":            "passed",
				"evidence":          []any{map[string]any{"path": "internal/qualityplan/schema.cue", "note": "Exact retained evidence."}},
				"receipt":           map[string]any{"path": receiptPath, "sha256": receiptSHA},
				"source_revision":   receiptSHA,
				"artifact_revision": receiptSHA,
				"review":            map[string]any{"by": "independent reviewer", "ref": "docs/design/converted-component-standard.md"},
			}
		}
	})
	if !cueAccepts(loaded, body) {
		t.Fatal("complete accepted item rejected by CUE")
	}
	doc := decodeInventory(t, body)
	if failures := validateInventory(doc, catalogProfiles(t), repoRoot(t)); len(failures) != 0 {
		t.Fatalf("complete accepted item rejected semantically: %v", failures)
	}
}

func TestSelectedConversionRequiresPinnedContractAndBudgets(t *testing.T) {
	loaded := loadInventory(t)
	contractPath := "docs/design/converted-component-standard.md"
	contractSHA := fileSHA256(t, filepath.Join(repoRoot(t), contractPath))
	addSelection := func(v map[string]any) {
		items := v["items"].([]any)
		item := items[len(items)-1].(map[string]any)
		item["kind"] = "conversion"
		item["state"] = "in-progress"
		item["selection"] = map[string]any{
			"original":            map[string]any{"kind": "catalog-capability", "id": "interaction.chrome", "revision_sha256": strings.Repeat("a", 64)},
			"target":              map[string]any{"id": "series-navigation", "revision_sha256": strings.Repeat("b", 64)},
			"acceptance_contract": map[string]any{"path": contractPath, "sha256": contractSHA},
			"workload":            "Render and navigate a twelve-step series at narrow and wide widths.",
			"budgets":             []any{map[string]any{"name": "render-bytes", "limit": "At most 16 KiB generated HTML."}},
		}
	}

	valid := mutated(t, loaded.body, addSelection)
	if !cueAccepts(loaded, valid) {
		t.Fatal("complete selected conversion rejected by CUE")
	}
	doc := decodeInventory(t, valid)
	selected := doc.Items[len(doc.Items)-1].Selection
	if selected == nil || selected.Original.Kind == "" || selected.Original.ID == "" || selected.Original.RevisionSHA256 == "" || selected.Target.ID == "" || selected.Target.RevisionSHA256 == "" || selected.Workload == "" || len(selected.Budgets) == 0 {
		t.Fatalf("conversion selection fields were not retained: %#v", selected)
	}
	if failures := validateInventory(doc, catalogProfiles(t), repoRoot(t)); len(failures) != 0 {
		t.Fatalf("complete selected conversion rejected semantically: %v", failures)
	}

	for _, tc := range []struct {
		name   string
		change func(map[string]any)
	}{
		{name: "missing contract", change: func(selection map[string]any) { delete(selection, "acceptance_contract") }},
		{name: "missing budget", change: func(selection map[string]any) { selection["budgets"] = []any{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := mutated(t, valid, func(v map[string]any) {
				items := v["items"].([]any)
				tc.change(items[len(items)-1].(map[string]any)["selection"].(map[string]any))
			})
			if cueAccepts(loaded, body) {
				t.Fatal("incomplete selected conversion accepted")
			}
		})
	}

	t.Run("contract hash drift", func(t *testing.T) {
		body := mutated(t, valid, func(v map[string]any) {
			items := v["items"].([]any)
			contract := items[len(items)-1].(map[string]any)["selection"].(map[string]any)["acceptance_contract"].(map[string]any)
			contract["sha256"] = strings.Repeat("c", 64)
		})
		if !cueAccepts(loaded, body) {
			t.Fatal("well-shaped drift canary should reach pin validation")
		}
		if failures := validateInventory(decodeInventory(t, body), catalogProfiles(t), repoRoot(t)); len(failures) == 0 {
			t.Fatal("drifted acceptance contract pin accepted")
		}
	})
}

func TestAdoptedConsumerRequiresAndChecksReviewedReceipt(t *testing.T) {
	loaded := loadInventory(t)
	receiptPath := "internal/qualityplan/inventory.cue"
	receiptSHA := fileSHA256(t, filepath.Join(repoRoot(t), receiptPath))
	body := mutated(t, loaded.body, func(v map[string]any) {
		consumer := v["items"].([]any)[0].(map[string]any)["consumers"].([]any)[0].(map[string]any)
		consumer["integration_status"] = "adopted"
		consumer["receipt"] = map[string]any{"path": receiptPath, "sha256": receiptSHA}
		consumer["review"] = map[string]any{"by": "independent reviewer", "ref": "docs/design/converted-component-standard.md"}
	})
	if !cueAccepts(loaded, body) {
		t.Fatal("adopted consumer with reviewed receipt rejected by CUE")
	}
	if failures := validateInventory(decodeInventory(t, body), catalogProfiles(t), repoRoot(t)); len(failures) != 0 {
		t.Fatalf("adopted consumer with reviewed receipt rejected: %v", failures)
	}
}

func TestReferencesAreContainedRegularFilesAndPinsMatch(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "inside.txt")
	if err := os.WriteFile(inside, []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := regularRepoFile(root, "inside.txt"); err != nil {
		t.Fatalf("regular repository file rejected: %v", err)
	}
	if _, err := regularRepoFile(root, "."); err == nil {
		t.Fatal("directory accepted as evidence")
	}
	outsideRoot := t.TempDir()
	outside := filepath.Join(outsideRoot, "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if _, err := regularRepoFile(root, "escape"); err == nil {
		t.Fatal("external symlink accepted as evidence")
	}
	if err := validatePinned(root, pinnedRef{Path: "inside.txt", SHA256: strings.Repeat("d", 64)}); err == nil {
		t.Fatal("drifted receipt hash accepted")
	}
}

func TestInventoryRejectsUnknownCapabilitiesDuplicatesAndCoreDrift(t *testing.T) {
	loaded := loadInventory(t)
	profiles, root := catalogProfiles(t), repoRoot(t)
	cases := map[string]func(map[string]any){
		"unknown capability": func(v map[string]any) {
			v["items"].([]any)[0].(map[string]any)["capability_ids"] = []any{"missing.capability"}
		},
		"incompatible profile": func(v map[string]any) {
			v["items"].([]any)[0].(map[string]any)["profile"] = "portable"
		},
		"duplicate IDs": func(v map[string]any) {
			items := v["items"].([]any)
			v["items"] = append(items, items[0])
		},
		"core scope drift": func(v map[string]any) {
			v["items"].([]any)[0].(map[string]any)["id"] = "core.unreviewed"
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			body := mutated(t, loaded.body, change)
			if name != "duplicate IDs" && !cueAccepts(loaded, body) {
				t.Fatal("mutation should reach semantic admission")
			}
			if failures := validateInventory(decodeInventory(t, body), profiles, root); len(failures) == 0 {
				t.Fatal("semantic mutation accepted")
			}
		})
	}
}

func readContributionRehearsal(t *testing.T) (contributionRehearsal, []byte) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(packageDir(t), "testdata", "jit-contribution-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record contributionRehearsal
	if err := json.Unmarshal(body, &record); err != nil {
		t.Fatal(err)
	}
	return record, body
}

func cloneContributionRehearsal(t *testing.T, record contributionRehearsal) contributionRehearsal {
	t.Helper()
	body, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	var clone contributionRehearsal
	if err := json.Unmarshal(body, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func cloneItem(t *testing.T, it item) item {
	t.Helper()
	body, err := json.Marshal(it)
	if err != nil {
		t.Fatal(err)
	}
	var clone item
	if err := json.Unmarshal(body, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

// validateContributionRehearsal validates this test-only returned-contribution
// example. It is deliberately not a production contribution schema or host policy.
func validateContributionRehearsal(record contributionRehearsal, it item, receiptSHA, root string) []string {
	var failures []string
	if record.Schema != "appkit-jit-contribution-rehearsal-v1" || !record.Synthetic {
		failures = append(failures, "fixture identity is not explicitly synthetic")
	}
	if record.WorkItem.ID != it.ID {
		failures = append(failures, "work-item ID mismatch")
	}
	if record.WorkItem.KitRevisionSHA256 == "" || record.WorkItem.KitRevisionSHA256 != record.Contribution.KitRevisionSHA256 {
		failures = append(failures, "contribution kit revision mismatch")
	}
	if it.Selection == nil {
		failures = append(failures, "missing conversion selection")
	} else {
		s := it.Selection
		if s.Original.Kind != record.Selection.OriginalKind || s.Original.ID != record.Selection.OriginalID || s.Original.RevisionSHA256 != record.Selection.OriginalRevisionSHA256 {
			failures = append(failures, "original selection mismatch")
		}
		if record.Selection.OriginalKind == "catalog-capability" && !contains(it.CapabilityIDs, record.Selection.OriginalID) {
			failures = append(failures, "catalog original is not a work-item capability")
		}
		if s.Target.ID != record.Selection.TargetID || s.Target.RevisionSHA256 != record.Selection.TargetRevisionSHA256 {
			failures = append(failures, "target selection mismatch")
		}
		if s.AcceptanceContract != record.Selection.AcceptanceContract || validatePinned(root, s.AcceptanceContract) != nil {
			failures = append(failures, "acceptance contract missing or drifted")
		}
		if s.Workload != record.Selection.Workload || len(s.Budgets) != len(record.Selection.Budgets) {
			failures = append(failures, "workload or budget mismatch")
		} else {
			for i, budget := range s.Budgets {
				if budget.Name != record.Selection.Budgets[i].Name || budget.Limit != record.Selection.Budgets[i].Limit {
					failures = append(failures, "workload budget mismatch")
				}
			}
		}
	}
	standardDecisions, gateDecisions := map[string]string{}, map[string]string{}
	for _, decision := range record.Standards {
		standardDecisions[decision.ID] = decision.Decision
	}
	for _, decision := range record.Gates {
		gateDecisions[decision.ID] = decision.Decision
	}
	if len(record.Standards) != len(standardCrosswalk) || !reflect.DeepEqual(mapKeys(standardDecisions), mapKeys(standardCrosswalk)) {
		failures = append(failures, "standard decisions are incomplete")
	}
	if len(record.Gates) != len(canonicalGates) || !reflect.DeepEqual(mapKeys(gateDecisions), canonicalGates) {
		failures = append(failures, "gate decisions are incomplete")
	}
	for name, state := range it.Standards {
		if standardDecisions[name] != state.Status || state.Status != "implemented" || state.Review == nil || state.Review.Ref != record.Receipt.Review.Ref {
			failures = append(failures, "standard decision/review mismatch: "+name)
		}
	}
	for name, state := range it.Gates {
		if gateDecisions[name] != state.Status || state.Status != "passed" || state.Receipt == nil || state.Receipt.Path != record.Receipt.Path || state.Receipt.SHA256 != receiptSHA || state.Review == nil || *state.Review != record.Receipt.Review || state.SourceRevision != record.Selection.OriginalRevisionSHA256 || state.ArtifactRevision != record.Selection.TargetRevisionSHA256 {
			failures = append(failures, "gate receipt/review mismatch: "+name)
		}
	}
	if err := validatePinned(root, pinnedRef{Path: record.Receipt.Path, SHA256: receiptSHA}); err != nil {
		failures = append(failures, "record receipt bytes are missing or drifted")
	}
	if len(it.Consumers) != 1 {
		failures = append(failures, "originating consumer count mismatch")
	} else {
		consumer := it.Consumers[0]
		if consumer.App != record.Originating.App || consumer.Revision.Kind != "source" || consumer.Revision.SHA256 != record.Originating.AppRevisionSHA256 || consumer.IntegrationStatus != "adopted" || consumer.Receipt == nil || consumer.Receipt.Path != record.Receipt.Path || consumer.Receipt.SHA256 != receiptSHA || consumer.Review == nil || *consumer.Review != record.Receipt.Review {
			failures = append(failures, "originating consumer receipt/review mismatch")
		}
	}
	if record.Originating.Task == "" || record.Originating.Result == "" {
		failures = append(failures, "originating task/result missing")
	}
	if record.Contribution.ID != "contribution.synthetic-series-navigation-return" || record.Contribution.Observed == "" || record.Contribution.Expected == "" || record.Contribution.Ownership == "" || record.Contribution.Disposition != "returned" || record.Contribution.Lesson == "" || record.Contribution.Test != "TestJITContributionPathRehearsal" {
		failures = append(failures, "returned contribution linkage mismatch")
	}
	if record.Contribution.NextAction.Task == "" || record.Contribution.NextAction.Owner == "" || record.Contribution.NextAction.Revisit == "" {
		failures = append(failures, "contribution next action is incomplete")
	}
	if _, err := regularRepoFile(root, record.Contribution.Lesson); err != nil {
		failures = append(failures, "linked lesson is not retained")
	}
	if record.Contribution.Delivery.Kind != "test-only-worked-example" || record.Contribution.Delivery.SelectionTargetID != record.Selection.TargetID || record.Contribution.Delivery.TargetRevisionSHA256 != record.Selection.TargetRevisionSHA256 || record.Contribution.Delivery.OriginatingFork == "" {
		failures = append(failures, "contribution delivery mismatch")
	}
	// These are the deliberately local claims of this fixture, not global
	// restrictions on ADR-0066's other dispositions or promotion paths.
	if record.Verification.Originating != "reviewed-local-only" || record.Verification.SecondConsumer != "unverified" || record.Verification.Propagation != "unverified" || record.Verification.Promotion != "experimental" {
		failures = append(failures, "unsupported verification or promotion claim")
	}
	if !contains(record.Limits, "Synthetic fixture only; it contains no real host path, consumer data, or paid source.") || !contains(record.Limits, "Receipt identity is not semantic acceptance; the parsed decisions and joins are checked independently.") || !contains(record.Limits, "Returned does not mean originating-app adoption or second-consumer propagation.") || !contains(record.Limits, "Authored revisions, workload, budgets, and reviewer identity are fixture inputs, not independently observed facts.") {
		failures = append(failures, "epistemic limits missing")
	}
	return sorted(failures)
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestJITContributionPathRehearsal(t *testing.T) {
	loaded := loadInventory(t)
	record, receiptBody := readContributionRehearsal(t)
	receiptSHA := fmt.Sprintf("%x", sha256.Sum256(receiptBody))
	joined := mutated(t, loaded.body, func(v map[string]any) {
		items := v["items"].([]any)
		it := items[len(items)-1].(map[string]any)
		it["kind"], it["state"] = "conversion", "accepted"
		it["selection"] = map[string]any{
			"original":            map[string]any{"kind": record.Selection.OriginalKind, "id": record.Selection.OriginalID, "revision_sha256": record.Selection.OriginalRevisionSHA256},
			"target":              map[string]any{"id": record.Selection.TargetID, "revision_sha256": record.Selection.TargetRevisionSHA256},
			"acceptance_contract": map[string]any{"path": record.Selection.AcceptanceContract.Path, "sha256": record.Selection.AcceptanceContract.SHA256},
			"workload":            record.Selection.Workload,
			"budgets":             record.Selection.Budgets,
		}
		for _, decision := range record.Standards {
			it["standards"].(map[string]any)[decision.ID] = map[string]any{"status": decision.Decision, "evidence": []any{map[string]any{"path": record.Receipt.Path, "note": "Synthetic deliberate standard decision."}}, "review": record.Receipt.Review}
		}
		for _, decision := range record.Gates {
			it["gates"].(map[string]any)[decision.ID] = map[string]any{"status": decision.Decision, "evidence": []any{map[string]any{"path": record.Receipt.Path, "note": "Synthetic deliberate gate decision."}}, "receipt": map[string]any{"path": record.Receipt.Path, "sha256": receiptSHA}, "source_revision": record.Selection.OriginalRevisionSHA256, "artifact_revision": record.Selection.TargetRevisionSHA256, "review": record.Receipt.Review}
		}
		it["consumers"] = []any{map[string]any{"app": record.Originating.App, "revision": map[string]any{"kind": "source", "sha256": record.Originating.AppRevisionSHA256}, "integration_status": "adopted", "evidence": []any{map[string]any{"path": record.Receipt.Path, "note": "Synthetic originating-app result."}}, "receipt": map[string]any{"path": record.Receipt.Path, "sha256": receiptSHA}, "review": record.Receipt.Review}}
	})
	if !cueAccepts(loaded, joined) {
		t.Fatal("joined rehearsal rejected by CUE")
	}
	doc := decodeInventory(t, joined)
	if failures := validateInventory(doc, catalogProfiles(t), repoRoot(t)); len(failures) != 0 {
		t.Fatalf("joined rehearsal rejected semantically: %v", failures)
	}
	it := doc.Items[len(doc.Items)-1]
	if failures := validateContributionRehearsal(record, it, receiptSHA, repoRoot(t)); len(failures) != 0 {
		t.Fatalf("valid returned local-only rehearsal rejected: %v", failures)
	}

	for _, tc := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"missing contract", func(it map[string]any) { delete(it["selection"].(map[string]any), "acceptance_contract") }},
		{"missing gate receipt", func(it map[string]any) { delete(it["gates"].(map[string]any)["data"].(map[string]any), "receipt") }},
		{"missing gate review", func(it map[string]any) { delete(it["gates"].(map[string]any)["data"].(map[string]any), "review") }},
		{"missing consumer receipt", func(it map[string]any) { delete(it["consumers"].([]any)[0].(map[string]any), "receipt") }},
		{"missing consumer review", func(it map[string]any) { delete(it["consumers"].([]any)[0].(map[string]any), "review") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := mutated(t, joined, func(v map[string]any) { tc.change(v["items"].([]any)[len(v["items"].([]any))-1].(map[string]any)) })
			if cueAccepts(loaded, body) {
				t.Fatal("incomplete joined rehearsal accepted by CUE")
			}
		})
	}

	for _, tc := range []struct {
		name   string
		change func(*contributionRehearsal, *item)
	}{
		{"contract drift", func(_ *contributionRehearsal, it *item) {
			it.Selection.AcceptanceContract.SHA256 = strings.Repeat("e", 64)
		}},
		{"gate receipt drift", func(_ *contributionRehearsal, it *item) {
			state := it.Gates["data"]
			state.Receipt.SHA256 = strings.Repeat("e", 64)
			it.Gates["data"] = state
		}},
		{"gate review drift", func(_ *contributionRehearsal, it *item) {
			state := it.Gates["data"]
			state.Review.Ref = "docs/design/converted-component-standard.md"
			it.Gates["data"] = state
		}},
		{"consumer receipt drift", func(_ *contributionRehearsal, it *item) { it.Consumers[0].Receipt.SHA256 = strings.Repeat("e", 64) }},
		{"consumer review drift", func(_ *contributionRehearsal, it *item) {
			it.Consumers[0].Review.Ref = "docs/design/converted-component-standard.md"
		}},
		{"correlated nonexistent catalog original", func(record *contributionRehearsal, it *item) {
			record.Selection.OriginalID = "missing.capability"
			it.Selection.Original.ID = "missing.capability"
		}},
		{"standard decision drift", func(record *contributionRehearsal, _ *item) { record.Standards[0].Decision = "not-applicable" }},
		{"gate decision drift", func(record *contributionRehearsal, _ *item) { record.Gates[0].Decision = "not-applicable" }},
		{"missing contribution ID", func(record *contributionRehearsal, _ *item) { record.Contribution.ID = "" }},
		{"missing observed result", func(record *contributionRehearsal, _ *item) { record.Contribution.Observed = "" }},
		{"missing expected result", func(record *contributionRehearsal, _ *item) { record.Contribution.Expected = "" }},
		{"missing ownership", func(record *contributionRehearsal, _ *item) { record.Contribution.Ownership = "" }},
		{"missing next task", func(record *contributionRehearsal, _ *item) { record.Contribution.NextAction.Task = "" }},
		{"missing next owner", func(record *contributionRehearsal, _ *item) { record.Contribution.NextAction.Owner = "" }},
		{"missing revisit condition", func(record *contributionRehearsal, _ *item) { record.Contribution.NextAction.Revisit = "" }},
		{"contribution revision mismatch", func(record *contributionRehearsal, _ *item) {
			record.Contribution.KitRevisionSHA256 = strings.Repeat("e", 64)
		}},
		{"contribution disposition mismatch", func(record *contributionRehearsal, _ *item) { record.Contribution.Disposition = "incubating" }},
		{"contribution delivery mismatch", func(record *contributionRehearsal, _ *item) {
			record.Contribution.Delivery.SelectionTargetID = "different-target"
		}},
		{"unsupported second verified claim", func(record *contributionRehearsal, _ *item) { record.Verification.SecondConsumer = "verified" }},
		{"unsupported promotion claim", func(record *contributionRehearsal, _ *item) { record.Verification.Promotion = "permanent" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			badRecord := cloneContributionRehearsal(t, record)
			badItem := cloneItem(t, it)
			tc.change(&badRecord, &badItem)
			if failures := validateContributionRehearsal(badRecord, badItem, receiptSHA, repoRoot(t)); len(failures) == 0 {
				t.Fatal("drifted joined rehearsal accepted")
			}
		})
	}
}
