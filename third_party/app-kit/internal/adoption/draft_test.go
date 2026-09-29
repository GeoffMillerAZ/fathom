package adoption

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/catalog"
)

func draftOptions(t *testing.T, root string) DraftOptions {
	t.Helper()
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	adoptionCheckWrite(t, root, "docs/app-kit.md", []byte("# Reviewed synthetic contract\n"))
	adoptionCheckWrite(t, root, "web/z.css", []byte("z\n"))
	adoptionCheckWrite(t, root, "web/a.css", []byte("a\n"))
	return DraftOptions{
		Consumer:                "synthetic-consumer",
		Profile:                 "portable",
		Contract:                "docs/app-kit.md",
		ExpectedCatalogRevision: c.Revision,
		Layers:                  []string{"L1", "L0"},
		Capabilities:            []string{"foundation.tokens", "foundation.theme.geoff-synthwave"},
		Artifacts:               []string{"web/z.css", "web/a.css"},
	}
}

func draftJSON(t *testing.T, selection Selection) []byte {
	t.Helper()
	body, err := json.MarshalIndent(selection, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func requireDraftError(t *testing.T, ctx context.Context, root string, opts DraftOptions) {
	t.Helper()
	got, err := Draft(ctx, root, opts)
	if err == nil || !reflect.DeepEqual(got, Selection{}) {
		t.Fatalf("Draft() = %+v, %v; want zero selection and error", got, err)
	}
}

func TestDraftRoundTripIsStableReadOnlyAndCheckable(t *testing.T) {
	root := t.TempDir()
	opts := draftOptions(t, root)
	// An old or malformed manifest is neither an input nor an upgrade target.
	adoptionCheckWrite(t, root, DefaultManifest, []byte("not an adoption manifest\n"))
	before := adoptionCheckSnapshot(t, root)
	wantOptions := DraftOptions{
		Consumer: opts.Consumer, Profile: opts.Profile, Contract: opts.Contract,
		ExpectedCatalogRevision: opts.ExpectedCatalogRevision,
		Layers:                  append([]string(nil), opts.Layers...),
		Capabilities:            append([]string(nil), opts.Capabilities...),
		Artifacts:               append([]string(nil), opts.Artifacts...),
	}

	first, err := Draft(context.Background(), root, opts)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Draft(context.Background(), root, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(opts, wantOptions) {
		t.Fatal("Draft mutated caller-owned options or slices")
	}
	if !bytes.Equal(draftJSON(t, first), draftJSON(t, second)) {
		t.Fatal("identical explicit inputs produced unstable manifest bytes")
	}
	if !reflect.DeepEqual(before, adoptionCheckSnapshot(t, root)) {
		t.Fatal("Draft wrote or changed the consumer")
	}
	if first.SchemaVersion != 2 || first.CatalogRevision != "" || first.CapabilityRevisions == nil || first.Module != nil || first.Agents {
		t.Fatalf("Draft emitted the wrong v2 boundary: %+v", first)
	}
	if !reflect.DeepEqual(first.Layers, []string{"L0", "L1"}) || !reflect.DeepEqual(first.Capabilities, []string{"foundation.theme.geoff-synthwave", "foundation.tokens"}) || first.Artifacts[0].Path != "web/a.css" {
		t.Fatalf("Draft did not normalize explicit selections: %+v", first)
	}
	decoded, err := Decode(draftJSON(t, first))
	if err != nil || !reflect.DeepEqual(decoded, first) {
		t.Fatalf("draft does not satisfy the public selection schema: %v", err)
	}

	adoptionCheckSave(t, root, first) // deliberate consumer action, not Draft.
	adoptionCheckOutcome(t, Pins(context.Background(), root, DefaultManifest).Report, 0, "observed")
	adoptionCheckOutcome(t, Check(context.Background(), root, DefaultManifest), 0, "passed")

	selected := filepath.Join(root, first.Artifacts[0].Path)
	original, err := os.ReadFile(selected)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(selected); err != nil {
		t.Fatal(err)
	}
	adoptionCheckOutcome(t, Check(context.Background(), root, DefaultManifest), 1, "failed")
	adoptionCheckWrite(t, root, first.Artifacts[0].Path, append(original, []byte("changed\n")...))
	adoptionCheckOutcome(t, Check(context.Background(), root, DefaultManifest), 1, "failed")
}

func TestDraftOptionalModuleAndAgentsAreDeclarationsNotReads(t *testing.T) {
	root := t.TempDir()
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	opts := DraftOptions{
		Consumer: "synthetic-cli", Profile: "cli", Contract: "missing/contract.md",
		ExpectedCatalogRevision: c.Revision,
		Capabilities:            []string{"cli.command"},
		Module:                  &ModuleRequirement{File: "missing/go.mod", Version: "v1.2.3", AllowLocalReplace: true},
		Agents:                  true,
	}
	before := adoptionCheckSnapshot(t, root)
	selection, err := Draft(context.Background(), root, opts)
	if err != nil {
		t.Fatal(err)
	}
	if selection.Module == nil || !selection.Agents || len(selection.Layers) != 0 || len(selection.Artifacts) != 0 {
		t.Fatalf("explicit optional declarations lost: %+v", selection)
	}
	if !reflect.DeepEqual(before, adoptionCheckSnapshot(t, root)) {
		t.Fatal("Draft tried to create or inspect missing contract/module/router files")
	}
	adoptionCheckSave(t, root, selection)
	report := Check(context.Background(), root, DefaultManifest)
	adoptionCheckOutcome(t, report, 1, "failed")
	for _, id := range []string{"contract", "module", "router:AGENTS.md"} {
		adoptionCheckFinding(t, report, id, "fail")
	}
	// AGENTS.md is the one required instruction file (ADR-0088); an absent
	// CLAUDE.md is not a failure.
	adoptionCheckFinding(t, report, "router:CLAUDE.md", "not-applicable")
}

func TestDraftRejectsInvalidOrUnavailableInputsWithZeroSelection(t *testing.T) {
	root := t.TempDir()
	valid := draftOptions(t, root)
	missingRoot := filepath.Join(root, "missing-root")

	for name, mutate := range map[string]func(*DraftOptions){
		"empty requirements":      func(o *DraftOptions) { o.Capabilities, o.Artifacts, o.Module, o.Agents = nil, nil, nil, false },
		"stale catalog guard":     func(o *DraftOptions) { o.ExpectedCatalogRevision = strings.Repeat("0", 64) },
		"malformed catalog guard": func(o *DraftOptions) { o.ExpectedCatalogRevision = "latest" },
		"unknown profile":         func(o *DraftOptions) { o.Profile = "native-future" },
		"unknown layer":           func(o *DraftOptions) { o.Layers = []string{"L9"} },
		"unknown capability":      func(o *DraftOptions) { o.Capabilities = []string{"missing.capability"} },
		"profile mismatch":        func(o *DraftOptions) { o.Profile = "cli"; o.Layers = nil },
		"layer mismatch":          func(o *DraftOptions) { o.Layers = []string{"L3a"} },
		"duplicate capability":    func(o *DraftOptions) { o.Capabilities = []string{"foundation.tokens", "foundation.tokens"} },
		"duplicate artifact":      func(o *DraftOptions) { o.Artifacts = []string{"web/a.css", "web/a.css"} },
		"unsafe artifact":         func(o *DraftOptions) { o.Artifacts = []string{"../outside.css"} },
		"invalid UTF-8 artifact":  func(o *DraftOptions) { o.Artifacts = []string{string([]byte{'w', 0xff})} },
		"too many capabilities": func(o *DraftOptions) {
			o.Capabilities = make([]string, 65)
			for i := range o.Capabilities {
				o.Capabilities[i] = fmt.Sprintf("fixture.%02d", i)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			opts := valid
			opts.Layers = append([]string(nil), valid.Layers...)
			opts.Capabilities = append([]string(nil), valid.Capabilities...)
			opts.Artifacts = append([]string(nil), valid.Artifacts...)
			mutate(&opts)
			requireDraftError(t, context.Background(), root, opts)
		})
	}
	requireDraftError(t, context.Background(), "", valid)
	requireDraftError(t, context.Background(), missingRoot, valid)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	requireDraftError(t, canceled, root, valid)
}

func TestDraftModuleOnlyAndAgentsOnlyRetainRequiredEmptyArrays(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts DraftOptions
	}{
		{"module", DraftOptions{Consumer: "module-only", Profile: "cli", Contract: "docs/app-kit.md", Module: &ModuleRequirement{File: "go.mod", Version: "v1.2.3"}}},
		{"agents", DraftOptions{Consumer: "agents-only", Profile: "cli", Contract: "docs/app-kit.md", Agents: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selection, err := Draft(context.Background(), t.TempDir(), tc.opts)
			if err != nil {
				t.Fatal(err)
			}
			body := draftJSON(t, selection)
			for _, field := range [][]byte{[]byte(`"layers": []`), []byte(`"capabilities": []`), []byte(`"capability_revisions": []`), []byte(`"artifacts": []`)} {
				if !bytes.Contains(body, field) {
					t.Fatalf("%s draft lost required empty array %s: %s", tc.name, field, body)
				}
			}
		})
	}
}

func TestDraftSharesAggregateArtifactReadBudget(t *testing.T) {
	root := t.TempDir()
	paths := make([]string, 9)
	for i := range paths {
		paths[i] = fmt.Sprintf("artifact-%02d.bin", i)
		name := filepath.Join(root, paths[i])
		if err := os.WriteFile(name, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Truncate(name, MaxFileBytes); err != nil {
			t.Fatal(err)
		}
	}
	requireDraftError(t, context.Background(), root, DraftOptions{
		Consumer: "aggregate-budget", Profile: "cli", Contract: "docs/app-kit.md", Artifacts: paths,
	})
}

func TestDraftArtifactReaderRefusesUnsafeMissingAndOversizedFiles(t *testing.T) {
	for _, tc := range []struct {
		name string
		prep func(*testing.T, string) string
	}{
		{"missing", func(_ *testing.T, _ string) string { return "web/missing.css" }},
		{"symlink", func(t *testing.T, root string) string {
			adoptionCheckWrite(t, root, "outside.css", []byte("outside\n"))
			if err := os.Symlink(filepath.Join(root, "outside.css"), filepath.Join(root, "linked.css")); err != nil {
				t.Fatal(err)
			}
			return "linked.css"
		}},
		{"directory", func(t *testing.T, root string) string {
			if err := os.Mkdir(filepath.Join(root, "selected-dir"), 0o700); err != nil {
				t.Fatal(err)
			}
			return "selected-dir"
		}},
		{"oversized", func(t *testing.T, root string) string {
			path := filepath.Join(root, "large.css")
			if err := os.WriteFile(path, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Truncate(path, MaxFileBytes+1); err != nil {
				t.Fatal(err)
			}
			return "large.css"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := tc.prep(t, root)
			requireDraftError(t, context.Background(), root, DraftOptions{
				Consumer: "artifact-only", Profile: "cli", Contract: "docs/app-kit.md", Artifacts: []string{path},
			})
		})
	}
}
