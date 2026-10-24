package fusion

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestCheckovClassification (LB-Q-checkov-policies, design §5): the dialect
// claims Checkov's check layout by path glob, ahead of the python dialect's .py
// and the YAML classifier; a .py or .yaml elsewhere is not its.
func TestCheckovClassification(t *testing.T) {
	rules := dialectRules()
	cases := []struct {
		path, want string
		by         string
	}{
		{"checkov/terraform/checks/resource/aws/S3Encryption.py", "checkov-policies", connector.ClassifiedByGlob},
		{"checks/resource/Custom.py", "checkov-policies", connector.ClassifiedByGlob},
		{"checkov/terraform/checks/data/aws/Policy.py", "checkov-policies", connector.ClassifiedByGlob},
		{"checkov/terraform/checks/provider/aws/Creds.py", "checkov-policies", connector.ClassifiedByGlob},
		{"checkov/terraform/checks/module/Mod.py", "checkov-policies", connector.ClassifiedByGlob},
		{"checkov/terraform/checks/graph_checks/aws/Open.yaml", "checkov-policies", connector.ClassifiedByGlob},
		{"checkov/kubernetes/checks/graph_checks/Open.yml", "checkov-policies", connector.ClassifiedByGlob},
		{"checkov/terraform/checks/graph_checks/aws/Open.json", "", connector.ClassifiedNone},
		{"checkov/terraform/checks/resource/aws/notes.md", "markdown", connector.ClassifiedByExtension},
		{"tools/checks.py", "python", connector.ClassifiedByExtension},
		{"src/resource/Custom.py", "python", connector.ClassifiedByExtension},
		{"config/graph_checks.yaml", "kubernetes", connector.ClassifiedByExtension},
	}
	for _, c := range cases {
		v := connector.ClassifyDialect(rules, c.path, nil)
		if v.Dialect != c.want || v.By != c.by {
			t.Errorf("ClassifyDialect(%q) = {%q %q}, want {%q %q}", c.path, v.Dialect, v.By, c.want, c.by)
		}
	}
}

func checkovCorpus() map[string]string {
	return map[string]string{
		"checks/resource/aws/AlbDrop.py": `from checkov.terraform.checks.resource.base_resource_check import BaseResourceCheck


class AlbDrop(BaseResourceCheck):
    def __init__(self):
        id = "CKV_X_1"
        supported_resources = ["aws_lb"]
        super().__init__(name="n", id=id, categories=[], supported_resources=supported_resources)

    def helper(self):
        return 1
`,
		"checks/graph_checks/aws/Open.yaml": "metadata:\n  id: CKV2_X_2\n  name: n\ndefinition:\n  cond_type: attribute\n  resource_types:\n    - aws_sg\n  attribute: a\n  operator: exists\n",
		"tools/helper.py":                   "def helper():\n    return 1\n",
		"config/other.yaml":                 "metadata:\n  id: not-a-check\ndefinition:\n  x: 1\n",
	}
}

// TestIngest_Checkov: an ingest reads a Checkov Python check and a YAML graph
// check through the checkov-policies dialect, a check as a symbol named for its
// id with its resource types as calls, leaves every other .py with the python
// dialect, and reads no unrelated YAML.
func TestIngest_Checkov(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, checkovCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "ck", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "ck", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, o := range obs {
		got[o.SourceRef+"|"+o.Symbol] = string(o.SourceKind)
	}
	want := map[string]string{
		"checks/resource/aws/AlbDrop.py|CKV_X_1":     "CheckovCheck",
		"checks/graph_checks/aws/Open.yaml|CKV2_X_2": "CheckovGraphCheck",
		"tools/helper.py|helper":                     "PythonFunc",
	}
	for k, kind := range want {
		if got[k] != kind {
			t.Errorf("%s: kind %q, want %q (all: %v)", k, got[k], kind, got)
		}
	}
	for k := range got {
		if k == "checks/resource/aws/AlbDrop.py|helper" || k == "checks/resource/aws/AlbDrop.py|AlbDrop" {
			t.Errorf("%s: a check file is read by the python dialect", k)
		}
		if len(k) >= len("config/other.yaml") && k[:len("config/other.yaml")] == "config/other.yaml" {
			t.Errorf("%s: unrelated YAML was read", k)
		}
	}
	for _, o := range obs {
		if o.SourceRef == "checks/resource/aws/AlbDrop.py" && o.Symbol == "CKV_X_1" {
			if len(o.Calls) != 1 || o.Calls[0] != "aws_lb" {
				t.Errorf("CKV_X_1 calls %v, want [aws_lb]", o.Calls)
			}
		}
	}
}
