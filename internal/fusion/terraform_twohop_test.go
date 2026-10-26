package fusion

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// A sibling output reference needs the module call's labelled source directory.
func TestTerraformModuleOutputAcrossSiblingFiles(t *testing.T) {
	for _, tc := range []struct {
		name, source, value, extra                                                 string
		sameFile, missingCall, missingOutput, duplicateOutput, duplicateCall, want bool
	}{
		{name: "same-file-control", source: `"../modules/net"`, value: "module.net.id", sameFile: true, want: true},
		{name: "sibling-plain", source: `"../modules/net"`, value: "module.net.id", want: true},
		{name: "sibling-count", source: `"../modules/net"`, value: "module.net[0].id", extra: "count = 1\n", want: true},
		{name: "sibling-for-each", source: `"../modules/net"`, value: `module.net["one"].id`, extra: `for_each = { one = true }` + "\n", want: true},
		{name: "sibling-nested-attribute", source: `"../modules/net"`, value: "module.net.id.value", want: true},
		{name: "remote", source: `"git::https://example.invalid/net.git"`, value: "module.net.id"},
		{name: "registry", source: `"example/net/test"`, value: "module.net.id"},
		{name: "escape", source: `"../../outside"`, value: "module.net.id"},
		{name: "caller-directory", source: `"./"`, value: "module.net.id"},
		{name: "absolute", source: `"/outside/net"`, value: "module.net.id"},
		{name: "constant-expression", source: "var.module_source", value: "module.net.id"},
		{name: "missing-output", source: `"../modules/net"`, value: "module.net.id", missingOutput: true},
		{name: "missing-call", source: `"../modules/net"`, value: "module.net.id", missingCall: true},
		{name: "duplicate-output", source: `"../modules/net"`, value: "module.net.id", duplicateOutput: true},
		{name: "duplicate-call", source: `"../modules/net"`, value: "module.net.id", duplicateCall: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			main := fmt.Sprintf("module \"net\" {\n source = %s\n%s}\n", tc.source, tc.extra)
			output := fmt.Sprintf("output \"result\" {\n value = %s\n}\n", tc.value)
			corpus := map[string]string{
				"app/main.tf":           main,
				"modules/net/output.tf": "output \"id\" { value = \"fixture\" }\n",
				// A second caller with the same address must not steal the join.
				"other/main.tf":           "module \"net\" { source = \"../modules/other\" }\n",
				"modules/other/output.tf": "output \"id\" { value = \"other\" }\n",
			}
			if tc.missingCall {
				corpus["app/main.tf"] = "variable \"unused\" {}\n"
			}
			origin := "app/output.tf"
			if tc.sameFile {
				corpus["app/main.tf"] += output
				origin = "app/main.tf"
			} else {
				corpus[origin] = output
			}
			if tc.missingOutput {
				delete(corpus, "modules/net/output.tf")
			}
			if tc.duplicateOutput {
				corpus["modules/net/duplicate.tf"] = corpus["modules/net/output.tf"]
			}
			if tc.duplicateCall {
				corpus["app/duplicate.tf"] = main
			}
			var paths []string
			for p := range corpus {
				paths = append(paths, p)
			}
			slices.Sort(paths)
			x := extractor.NewHCLExtractor()
			var files []DocFile
			for _, p := range paths {
				r, err := x.ExtractDoc("probe", p, []byte(corpus[p]))
				if err != nil {
					t.Fatalf("extract %s: %v", p, err)
				}
				files = append(files, DocFile{Path: p, Result: r})
			}
			forward, stats := resolveDocXrefs(files, nil)
			tf := stats.Terraform
			if tf.Intents != 1 || tf.Resolved+tf.MissingCall+tf.AmbiguousCall+tf.NonlocalSource+tf.MissingOutput+tf.AmbiguousOutput != tf.Intents {
				t.Fatalf("nonconserving output intent counts: %+v", tf)
			}
			switch {
			case tc.missingCall:
				if tf.MissingCall != 1 {
					t.Fatalf("missing call count: %+v", tf)
				}
			case tc.duplicateCall:
				if tf.AmbiguousCall != 1 {
					t.Fatalf("ambiguous call count: %+v", tf)
				}
			case tc.duplicateOutput:
				if tf.AmbiguousOutput != 1 {
					t.Fatalf("ambiguous output count: %+v", tf)
				}
			case tc.missingOutput:
				if tf.MissingOutput != 1 {
					t.Fatalf("missing output count: %+v", tf)
				}
			case tc.want:
				if tf.Resolved != 1 {
					t.Fatalf("resolved count: %+v", tf)
				}
			default:
				if tf.NonlocalSource != 1 {
					t.Fatalf("nonlocal source count: %+v", tf)
				}
			}
			slices.Reverse(files)
			if reverse := ResolveDocXrefs(files); !reflect.DeepEqual(forward, reverse) {
				t.Fatal("file order changed resolved edges")
			}
			var got []string
			for _, edge := range forward {
				if edge.FromFile == origin && edge.From == "output.result" && edge.Edge == "module-output" {
					got = append(got, edge.ToFile+":"+edge.To)
				}
			}
			var want []string
			if tc.want {
				want = []string{"modules/net/output.tf:output.id"}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("module-output targets = %v, want %v", got, want)
			}
		})
	}
}

// A source is admitted only through localModuleDir, and cannot resolve an
// identically named output in a different directory or an escaped directory.
func TestTerraformModuleSourceBounds(t *testing.T) {
	for _, tc := range []struct{ source, wantDir string }{
		{`"../modules/net"`, "modules/net"},
		{`"git::https://example.invalid/net.git"`, ""},
		{`"example/net/test"`, ""},
		{`"../../outside"`, ""},
		{`"./"`, ""},
		{`"/outside/net"`, ""},
		{`var.module_source`, ""},
	} {
		t.Run(tc.source, func(t *testing.T) {
			files := terraformTwoHopFiles(t, map[string]string{
				"app/main.tf":            "module \"net\" { source = " + tc.source + " }\n",
				"app/outputs.tf":         "output \"result\" { value = module.net.id }\n",
				"modules/net/outputs.tf": "output \"id\" { value = 1 }\n",
				"other/outputs.tf":       "output \"id\" { value = 2 }\n",
			})
			var calls, intents int
			for _, f := range files {
				for _, ls := range f.Result.Labels {
					if f.Path == "app/main.tf" && ls.Entity == "module.net" {
						calls++
						if got := ls.Labels["terraform.local_source_dir"]; got != tc.wantDir {
							t.Fatalf("local source label = %q, want %q", got, tc.wantDir)
						}
						if got := ls.Labels["terraform.source_present"]; got != "true" {
							t.Fatalf("explicit source label = %q, want true", got)
						}
					}
				}
				if f.Path == "app/outputs.tf" {
					for _, ref := range f.Result.CrossFile {
						if ref.Edge == "module-output" {
							intents++
							if ref.Strategy != "terraform-module-output" || ref.Scope != "terraform:app" || ref.Labels["terraform.module_call"] != "module.net" || ref.Labels["terraform.output"] != "output.id" {
								t.Fatalf("incorrect module output intent: %+v", ref)
							}
						}
					}
				}
			}
			if calls != 1 || intents != 1 {
				t.Fatalf("module labels = %d, output intents = %d; want 1 each", calls, intents)
			}
			var outputEdges int
			for _, edge := range ResolveDocXrefs(files) {
				if edge.FromFile == "app/outputs.tf" && edge.Edge == "module-output" {
					outputEdges++
					if edge.ToFile != "modules/net/outputs.tf" {
						t.Fatalf("source %s resolved output through %s", tc.source, edge.ToFile)
					}
				}
			}
			want := 0
			if tc.wantDir != "" {
				want = 1
			}
			if outputEdges != want {
				t.Fatalf("output edges = %d, want %d", outputEdges, want)
			}
		})
	}
}

func terraformTwoHopFiles(t *testing.T, corpus map[string]string) []DocFile {
	t.Helper()
	paths := make([]string, 0, len(corpus))
	for p := range corpus {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	x := extractor.NewHCLExtractor()
	files := make([]DocFile, 0, len(paths))
	for _, p := range paths {
		r, err := x.ExtractDoc("probe", p, []byte(corpus[p]))
		if err != nil {
			t.Fatalf("extract %s: %v", p, err)
		}
		files = append(files, DocFile{Path: p, Result: r})
	}
	return files
}

func TestTerraformModuleOutputIntentDedupeAndInputs(t *testing.T) {
	files := terraformTwoHopFiles(t, map[string]string{
		"app/main.tf":            "module \"net\" {\n source = \"../modules/net\"\n name = var.name\n}\nvariable \"name\" {}\n",
		"app/outputs.tf":         "output \"result\" { value = [module.net.id, module.net.id] }\n",
		"modules/net/inputs.tf":  "variable \"name\" {}\n",
		"modules/net/outputs.tf": "output \"id\" { value = 1 }\n",
	})
	var outputs, inputs int
	for _, edge := range ResolveDocXrefs(files) {
		if edge.FromFile == "app/outputs.tf" && edge.Edge == "module-output" && edge.ToFile == "modules/net/outputs.tf" {
			outputs++
		}
		if edge.FromFile == "app/main.tf" && edge.Edge == "module-input" && edge.ToFile == "modules/net/inputs.tf" {
			inputs++
		}
	}
	if outputs != 1 || inputs != 1 {
		t.Fatalf("output edges = %d, input edges = %d; want 1 each", outputs, inputs)
	}
	var callFound bool
	for _, f := range files {
		if f.Path != "app/outputs.tf" {
			continue
		}
		for _, n := range f.Result.Nodes {
			if n.Name == "output.result" {
				callFound = slices.Contains(n.Calls, "module.net")
			}
		}
	}
	if !callFound {
		t.Fatal("module call reference was lost")
	}
}

// Labels are only evidence for a call or output when backed by a node of the
// right Terraform kind. Colliding resource addresses and orphan labels cannot
// manufacture a join or make an otherwise unique join ambiguous.
func TestTerraformModuleOutputKindAndOrphanLabels(t *testing.T) {
	base := func() map[string]string {
		return map[string]string{
			"app/main.tf":            "module \"net\" { source = \"../modules/net\" }\n",
			"app/outputs.tf":         "output \"result\" { value = module.net.id }\n",
			"modules/net/outputs.tf": "output \"id\" { value = 1 }\n",
		}
	}
	for _, tc := range []struct {
		name       string
		change     func(map[string]string)
		orphanFile string
		orphan     extractor.DocLabelSet
		want       terraformOutputReason
	}{
		{
			name:   "resource-call-collision-with-real-call",
			change: func(c map[string]string) { c["app/collision.tf"] = "resource \"module\" \"net\" {}\n" },
			want:   terraformOutputResolved,
		},
		{
			name:   "resource-output-collision-with-real-output",
			change: func(c map[string]string) { c["modules/net/collision.tf"] = "resource \"output\" \"id\" {}\n" },
			want:   terraformOutputResolved,
		},
		{
			name:   "resource-call-alone",
			change: func(c map[string]string) { c["app/main.tf"] = "resource \"module\" \"net\" {}\n" },
			want:   terraformOutputMissingCall,
		},
		{
			name:   "resource-output-alone",
			change: func(c map[string]string) { c["modules/net/outputs.tf"] = "resource \"output\" \"id\" {}\n" },
			want:   terraformOutputMissingOutput,
		},
		{
			name:   "same-file-cross-kind-call",
			change: func(c map[string]string) { c["app/main.tf"] = "resource \"module\" \"net\" {}\n" + c["app/main.tf"] },
			want:   terraformOutputAmbiguousCall,
		},
		{
			name: "same-file-cross-kind-output",
			change: func(c map[string]string) {
				c["modules/net/outputs.tf"] = "resource \"output\" \"id\" {}\n" + c["modules/net/outputs.tf"]
			},
			want: terraformOutputAmbiguousOutput,
		},
		{
			name:       "orphan-call-label-with-real-call",
			change:     func(c map[string]string) { c["app/orphan.tf"] = "variable \"unused\" {}\n" },
			orphanFile: "app/orphan.tf",
			orphan:     extractor.DocLabelSet{Entity: "module.net", Scope: "terraform:app", Labels: map[string]string{"terraform.address": "module.net", "terraform.local_source_dir": "modules/net"}},
			want:       terraformOutputResolved,
		},
		{
			name:       "orphan-output-label-with-real-output",
			change:     func(c map[string]string) { c["modules/net/orphan.tf"] = "variable \"unused\" {}\n" },
			orphanFile: "modules/net/orphan.tf",
			orphan:     extractor.DocLabelSet{Entity: "output.id", Scope: "terraform:modules/net", Labels: map[string]string{"terraform.address": "output.id"}},
			want:       terraformOutputResolved,
		},
		{
			name:       "orphan-call-label-alone",
			change:     func(c map[string]string) { c["app/main.tf"] = "variable \"unused\" {}\n" },
			orphanFile: "app/main.tf",
			orphan:     extractor.DocLabelSet{Entity: "module.net", Scope: "terraform:app", Labels: map[string]string{"terraform.address": "module.net", "terraform.local_source_dir": "modules/net"}},
			want:       terraformOutputMissingCall,
		},
		{
			name:       "orphan-output-label-alone",
			change:     func(c map[string]string) { c["modules/net/outputs.tf"] = "variable \"unused\" {}\n" },
			orphanFile: "modules/net/outputs.tf",
			orphan:     extractor.DocLabelSet{Entity: "output.id", Scope: "terraform:modules/net", Labels: map[string]string{"terraform.address": "output.id"}},
			want:       terraformOutputMissingOutput,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			corpus := base()
			if tc.change != nil {
				tc.change(corpus)
			}
			files := terraformTwoHopFiles(t, corpus)
			if tc.orphanFile != "" {
				for i := range files {
					if files[i].Path == tc.orphanFile {
						files[i].Result.Labels = append(files[i].Result.Labels, tc.orphan)
					}
				}
			}
			xs, stats := resolveDocXrefs(files, nil)
			var gotReason terraformOutputReason
			st := stats.Terraform
			if st.Intents != 1 || st.Resolved+st.MissingCall+st.AmbiguousCall+st.NonlocalSource+st.MissingOutput+st.AmbiguousOutput != 1 {
				t.Fatalf("nonconserving output stats: %+v", st)
			}
			switch {
			case st.Resolved == 1:
				gotReason = terraformOutputResolved
			case st.MissingCall == 1:
				gotReason = terraformOutputMissingCall
			case st.AmbiguousCall == 1:
				gotReason = terraformOutputAmbiguousCall
			case st.NonlocalSource == 1:
				gotReason = terraformOutputNonlocalSource
			case st.MissingOutput == 1:
				gotReason = terraformOutputMissingOutput
			case st.AmbiguousOutput == 1:
				gotReason = terraformOutputAmbiguousOutput
			}
			if gotReason != tc.want {
				t.Fatalf("outcome = %v, want %v; stats %+v", gotReason, tc.want, st)
			}
			var edges int
			for _, x := range xs {
				if x.FromFile == "app/outputs.tf" && x.Edge == "module-output" {
					edges++
				}
			}
			wantEdges := 0
			if tc.want == terraformOutputResolved {
				wantEdges = 1
			}
			if edges != wantEdges {
				t.Fatalf("output edges = %d, want %d", edges, wantEdges)
			}
		})
	}
}

func TestTerraformModuleOutputRepeatedAddressIndex(t *testing.T) {
	const duplicates = 512
	for _, manyCalls := range []bool{false, true} {
		name := "repeated-labels-one-call"
		if manyCalls {
			name = "many-distinct-calls-same-address"
		}
		t.Run(name, func(t *testing.T) {
			corpus := map[string]string{
				"app/main.tf":            "module \"net\" { source = \"../modules/net\" }\n",
				"app/outputs.tf":         "output \"result\" { value = module.net.id }\n",
				"modules/net/outputs.tf": "output \"id\" { value = 1 }\n",
			}
			if manyCalls {
				for i := 0; i < duplicates; i++ {
					corpus[fmt.Sprintf("app/call-%04d.tf", i)] = corpus["app/main.tf"]
				}
			}
			files := terraformTwoHopFiles(t, corpus)
			for i := range files {
				if manyCalls {
					if strings.HasPrefix(files[i].Path, "app/call-") {
						files[i].Result.Labels = append(files[i].Result.Labels, files[i].Result.Labels[0])
					}
				} else if files[i].Path == "app/main.tf" {
					for j := 0; j < duplicates; j++ {
						files[i].Result.Labels = append(files[i].Result.Labels, files[i].Result.Labels[0])
					}
				}
			}
			xs, stats := resolveDocXrefs(files, nil)
			tf := stats.Terraform
			if tf.Intents != 1 || tf.Resolved+tf.MissingCall+tf.AmbiguousCall+tf.NonlocalSource+tf.MissingOutput+tf.AmbiguousOutput != 1 {
				t.Fatalf("nonconserving counts: %+v", tf)
			}
			var edges int
			for _, x := range xs {
				if x.FromFile == "app/outputs.tf" && x.Edge == "module-output" {
					edges++
				}
			}
			if manyCalls {
				if tf.AmbiguousCall != 1 || edges != 0 {
					t.Fatalf("many calls: stats %+v, edges %d", tf, edges)
				}
			} else if tf.Resolved != 1 || edges != 1 {
				t.Fatalf("repeated labels: stats %+v, edges %d", tf, edges)
			}
		})
	}
}

func TestTerraformModuleOutputOverrides(t *testing.T) {
	for _, tc := range []struct {
		name, callOverride, outputOverride, wantFile string
		want                                         terraformOutputReason
	}{
		{"output-merge", "", `output "id" { value = 2 }`, "modules/net/outputs.tf", terraformOutputResolved},
		{"call-merge", `module "net" { source = "../modules/net" }`, "", "modules/net/outputs.tf", terraformOutputResolved},
		{"source-inherited", `module "net" { depends_on = [] }`, "", "modules/net/outputs.tf", terraformOutputResolved},
		{"source-changed", `module "net" { source = "../modules/other" }`, "", "modules/other/outputs.tf", terraformOutputResolved},
		{"source-remote", `module "net" { source = "example/net/test" }`, "", "", terraformOutputNonlocalSource},
		{"source-dynamic", `module "net" { source = var.module_source }`, "", "", terraformOutputNonlocalSource},
	} {
		t.Run(tc.name, func(t *testing.T) {
			corpus := map[string]string{
				"app/main.tf":              `module "net" { source = "../modules/net" }`,
				"app/outputs.tf":           `output "result" { value = module.net.id }`,
				"modules/net/outputs.tf":   `output "id" { value = 1 }`,
				"modules/other/outputs.tf": `output "id" { value = 3 }`,
			}
			if tc.callOverride != "" {
				corpus["app/override.tf"] = tc.callOverride
			}
			if tc.outputOverride != "" {
				corpus["modules/net/outputs_override.tf"] = tc.outputOverride
			}
			files := terraformTwoHopFiles(t, corpus)
			if tc.name == "source-inherited" {
				for _, f := range files {
					if f.Path == "app/override.tf" {
						if got := f.Result.Labels[0].Labels["terraform.source_present"]; got != "" {
							t.Fatalf("source-absent override marked present: %q", got)
						}
						if got := f.Result.Labels[0].Labels["terraform.local_source_dir"]; got != "" {
							t.Fatalf("extractor read sibling source: %q", got)
						}
					}
				}
			}
			for _, order := range []string{"forward", "reverse"} {
				if order == "reverse" {
					slices.Reverse(files)
				}
				resolve := resolveTerraformModuleOutputs(files)
				ref := extractor.DocCrossFileRef{Strategy: "terraform-module-output", Edge: "module-output", Scope: "terraform:app", Labels: map[string]string{"terraform.module_call": "module.net", "terraform.output": "output.id"}}
				got, reason := resolve(ref)
				if reason != tc.want || got.file != tc.wantFile {
					t.Fatalf("%s: got %q / %v, want %q / %v", order, got.file, reason, tc.wantFile, tc.want)
				}
				var targets []string
				for _, edge := range ResolveDocXrefs(files) {
					if edge.FromFile == "app/outputs.tf" && edge.Edge == "module-output" {
						targets = append(targets, edge.ToFile)
					}
				}
				var want []string
				if tc.want == terraformOutputResolved {
					want = []string{tc.wantFile}
				}
				if !reflect.DeepEqual(targets, want) {
					t.Fatalf("%s: targets %v, want %v", order, targets, want)
				}
			}
		})
	}
}

func TestTerraformModuleOutputOverrideOrder(t *testing.T) {
	corpus := map[string]string{
		"app/main.tf":              `module "net" { source = "../modules/net" }`,
		"app/outputs.tf":           `output "result" { value = module.net.id }`,
		"app/override.tf":          `module "net" { source = "../modules/other" }`,
		"app/z_override.tf":        `module "net" { source = "../modules/last" }`,
		"modules/net/outputs.tf":   `output "id" { value = 1 }`,
		"modules/other/outputs.tf": `output "id" { value = 2 }`,
		"modules/last/outputs.tf":  `output "id" { value = 3 }`,
	}
	files := terraformTwoHopFiles(t, corpus)
	for i := 0; i < 2; i++ {
		resolve := resolveTerraformModuleOutputs(files)
		got, reason := resolve(extractor.DocCrossFileRef{Strategy: "terraform-module-output", Edge: "module-output", Scope: "terraform:app", Labels: map[string]string{"terraform.module_call": "module.net", "terraform.output": "output.id"}})
		if reason != terraformOutputResolved || got.file != "modules/last/outputs.tf" {
			t.Fatalf("pass %d: got %q / %v", i, got.file, reason)
		}
		slices.Reverse(files)
	}
}

// An override block with no primary declaration is a new block. Its symbol
// site is the earliest override; subsequent files can still change source.
func TestTerraformModuleOutputOverrideOnly(t *testing.T) {
	files := terraformTwoHopFiles(t, map[string]string{
		"app/outputs.tf":            `output "result" { value = module.net.id }`,
		"app/override.tf":           `module "net" { source = "../modules/other" }`,
		"app/a_override.tf":         `module "net" { source = "../modules/net" }`,
		"modules/net/override.tf":   `output "id" { value = 1 }`,
		"modules/net/z_override.tf": `output "id" { value = 2 }`,
		"modules/other/outputs.tf":  `output "id" { value = 3 }`,
	})
	ref := extractor.DocCrossFileRef{Strategy: "terraform-module-output", Edge: "module-output", Scope: "terraform:app", Labels: map[string]string{"terraform.module_call": "module.net", "terraform.output": "output.id"}}
	for i := 0; i < 2; i++ {
		got, reason := resolveTerraformModuleOutputs(files)(ref)
		if reason != terraformOutputResolved || got.file != "modules/net/override.tf" {
			t.Fatalf("pass %d: got %q / %v", i, got.file, reason)
		}
		slices.Reverse(files)
	}
}

func TestTerraformModuleOutputIngestReplacement(t *testing.T) {
	dir := t.TempDir()
	corpus := map[string]string{
		"app/main.tf":            "module \"net\" { source = \"../modules/net\" }\n",
		"app/outputs.tf":         "output \"result\" { value = module.net.id }\n",
		"modules/net/outputs.tf": "output \"id\" { value = 1 }\n",
	}
	writeCorpus(t, dir, corpus)
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	var diagnostics strings.Builder
	p.SetLogger(dlog.New(&diagnostics, dlog.LevelInfo, time.Now))
	ctx := context.Background()
	svc := core.ServiceRef{ID: "tf", Path: dir}
	check := func(want int) {
		t.Helper()
		if _, err := p.Ingest(ctx, svc); err != nil {
			t.Fatal(err)
		}
		edges, err := st.TypedEdges(ctx, svc.ID)
		if err != nil {
			t.Fatal(err)
		}
		var got int
		for _, e := range edges {
			if e.Kind == core.EdgeFileResolved && e.FromFile == "app/outputs.tf" && e.ToSymbol == tfKey("modules/net/outputs.tf", "output.id") {
				got++
			}
		}
		if got != want {
			t.Fatalf("persisted module-output edges = %d, want %d", got, want)
		}
	}
	check(1)
	if got := diagnostics.String(); !strings.Contains(got, "terraform module outputs: 1 intents, 1 resolved, 0 unresolved") {
		t.Fatalf("missing ingest output resolution diagnostic: %q", got)
	}
	writeCorpus(t, dir, map[string]string{"app/main.tf": "module \"net\" { source = \"example/net/test\" }\n"})
	check(0)
	if got := diagnostics.String(); !strings.Contains(got, "1 unresolved (missing call 0, ambiguous call 0, nonlocal or nonliteral source 1, missing output 0, ambiguous output 0)") {
		t.Fatalf("missing ingest unresolved diagnostic: %q", got)
	}
	writeCorpus(t, dir, map[string]string{"app/outputs.tf": "output \"result\" { value = 1 }\n"})
	check(0)
	if got := diagnostics.String(); strings.Count(got, "terraform module outputs:") != 2 {
		t.Fatalf("unexpected replacement diagnostic count: %q", got)
	}
}
