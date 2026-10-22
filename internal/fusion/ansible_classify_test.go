package fusion

// ansible_classify_test.go — LB-F150: the Ansible signal is the classifier's
// last, so every verdict an earlier signal gave is unchanged; a playbook, a
// role's files, a variable file and a YAML inventory are Ansible's by path or
// content, a workflow, a Compose file or a plain mapping is not; ingest reads
// them through the ansible dialect and stores the include, role and notify
// edges; ReingestFile decides what ingest did.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const (
	anPlaybook  = "---\n- name: Configure\n  hosts: all\n  roles:\n    - web\n  tasks:\n    - import_tasks: tasks/setup.yml\n"
	anPlaybook2 = "- hosts: dbservers\n  become: true\n  tasks: []\n"
	anImport    = "# the site playbook\n---\n- import_playbook: web.yml\n- ansible.builtin.import_playbook: db.yml\n"
	anTasks     = "---\n- name: Install\n  package:\n    name: nginx\n  notify: restart nginx\n\n- include_tasks: more.yml\n"
	anHandlers  = "---\n- name: restart nginx\n  service:\n    name: nginx\n    state: restarted\n"
	anVars      = "---\nnginx_port: 80\nnginx_user: www\n"
	anInventory = "all:\n  children:\n    web:\n      hosts:\n        web1.example.com:\n"
	anMeta      = "---\ngalaxy_info:\n  author: Example\ndependencies:\n  - common\n"
)

func TestAnsibleClassify(t *testing.T) {
	idx := buildSiblingIndex([]core.SourceRef{{Path: "charts/app/Chart.yaml"}, {Path: "myrole/tasks/main.yml"}, {Path: "tasks/main.yml"}})
	cases := []struct {
		name    string
		path    string
		content string
		want    ToolKind
	}{
		// Ansible by content: a top-level sequence of plays.
		{"playbook", "site.yml", anPlaybook, ToolAnsible},
		{"playbook with the hosts key after others", "play.yaml", anPlaybook2, ToolAnsible},
		{"playbook of imports", "all.yml", anImport, ToolAnsible},
		// Ansible by path.
		{"role tasks", "roles/web/tasks/main.yml", anTasks, ToolAnsible},
		{"role handlers", "roles/web/handlers/main.yml", anHandlers, ToolAnsible},
		{"role defaults", "roles/web/defaults/main.yml", anVars, ToolAnsible},
		{"role vars", "x/roles/web/vars/Debian.yml", anVars, ToolAnsible},
		{"role meta", "roles/web/meta/main.yml", anMeta, ToolAnsible},
		{"nested role task file", "roles/web/tasks/install/main.yml", anTasks, ToolAnsible},
		{"group vars", "group_vars/web.yml", anVars, ToolAnsible},
		{"group vars directory", "inventory/group_vars/web/main.yml", anVars, ToolAnsible},
		{"host vars", "host_vars/web1.yaml", anVars, ToolAnsible},
		{"yaml inventory", "inventory/hosts.yml", anInventory, ToolAnsible},
		{"yaml inventory under inventories", "ops/inventories/prod.yml", anInventory, ToolAnsible},
		// The task files a playbook imports, and a standalone role's files (its root holds tasks/main.yml).
		{"a task file under tasks/", "playbooks/tasks/setup.yml", anTasks, ToolAnsible},
		{"a handlers file under handlers/", "handlers/main.yml", anHandlers, ToolAnsible},
		{"a standalone role's vars", "vars/Debian.yml", anVars, ToolAnsible},
		{"a standalone role's defaults", "defaults/main.yml", anVars, ToolAnsible},
		{"a standalone role's meta", "meta/main.yml", anMeta, ToolAnsible},
		{"a standalone role below the root", "myrole/vars/main.yml", anVars, ToolAnsible},
		// Not Ansible. A version-3 task map is a Taskfile even under tasks/.
		{"a mapping under tasks/ (a Taskfile include)", "ci/tasks/build.yml", "version: '3'\ntasks:\n  build:\n    cmds: [make]\n", ToolTaskfile},
		{"a malformed task list under tasks/", "ci/tasks/not-taskfile.yml", "version: '3'\ntasks:\n  - build\n", ToolUnclassified},
		{"a vars directory with no role beside it", "other/vars/Debian.yml", anVars, ToolUnclassified},
		{"a vars file outside the vars directories", "config/vars.yml", anVars, ToolUnclassified},
		{"an inventory-shaped file outside inventory/", "config/hosts.yml", anInventory, ToolUnclassified},
		{"a plain mapping", "settings.yml", "name: x\nvalue: 1\n", ToolUnclassified},
		{"a sequence of items with a hosts key nested deep", "list.yml", "items:\n  - name: a\n    hosts: x\n", ToolUnclassified},
		{"a workflow under .github/workflows", ".github/workflows/roles/tasks/ci.yml", "on: push\njobs:\n  b:\n    runs-on: x\n", ToolUnclassified},
		// Every earlier verdict is unchanged, whatever the path says.
		{"helm template", "charts/app/templates/roles/web/tasks/main.yml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n", ToolHelmTemplateYAML},
		{"crd", "roles/web/tasks/crd.yml", "apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: x.example.org\n", ToolK8sCRD},
		{"kubernetes under a role path", "roles/web/files/deploy.yml", "apiVersion: v1\nkind: Pod\nmetadata:\n  name: p\n", ToolK8sManifest},
		{"compose under group_vars", "group_vars/compose.yml", "services:\n  web:\n    image: nginx\n", ToolCompose},
		{"gitlab ci", ".gitlab-ci.yml", "stages: [build]\nbuild:\n  script: [make]\n", ToolGitLabCI},
		{"openapi under a role path", "roles/web/files/api.yml", "openapi: 3.0.0\npaths: {}\n", ToolOpenAPI},
		{"cloudformation under host_vars", "host_vars/stack.yml", "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n  B:\n    Type: AWS::S3::Bucket\n", ToolCloudFormation},
		{"flux object", "roles/web/files/flux.yml", crFluxKustomization, ToolK8sManifest},
	}
	for _, c := range cases {
		if got := Classify(c.path, []byte(c.content), idx); got != c.want {
			t.Errorf("%s: Classify(%q) = %q, want %q", c.name, c.path, got, c.want)
		}
	}
}

func ansibleRepoCorpus() map[string]string {
	return map[string]string{
		"site.yml":                      "---\n- name: Configure\n  hosts: all\n  roles:\n    - web\n  tasks:\n    - import_tasks: tasks/setup.yml\n    - ansible.builtin.include_vars: vars/site.yml\n",
		"tasks/setup.yml":               "---\n- name: Setup\n  debug:\n    msg: setup\n",
		"vars/site.yml":                 "---\nsite_name: example\n",
		"roles/web/tasks/main.yml":      "---\n- name: Install nginx\n  package:\n    name: nginx\n  notify: restart nginx\n\n- include_tasks: extra.yml\n",
		"roles/web/tasks/extra.yml":     "---\n- name: Extra\n  debug:\n    msg: extra\n",
		"roles/web/handlers/main.yml":   "---\n- name: restart nginx\n  service:\n    name: nginx\n    state: restarted\n",
		"roles/web/defaults/main.yml":   "---\nnginx_port: 80\n",
		"inventory/hosts.yml":           anInventory,
		"misc/notes.yml":                "---\ntitle: notes\n",
		".github/workflows/ci.yml":      "on: push\njobs:\n  b:\n    runs-on: ubuntu-latest\n    steps:\n      - run: ansible-playbook site.yml\n",
		"k8s/deploy.yaml":               crDeployment,
		"compose/docker-compose.yml":    "services:\n  web:\n    image: nginx\n",
		"roles/web/files/not-ansible.k": "ignored\n",
	}
}

// TestIngest_AnsibleRepo: ingest reads each Ansible file through the dialect
// and stores the cross-file edges: a play's import_tasks and include_vars, a
// role named in roles:, a role's include_tasks, and a notify naming a handler
// of another file.
func TestIngest_AnsibleRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, ansibleRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "an", Path: dir}); err != nil {
		t.Fatal(err)
	}
	got := crSourceKinds(t, st, "an")
	for _, rel := range []string{"site.yml", "roles/web/tasks/main.yml", "roles/web/tasks/extra.yml", "roles/web/handlers/main.yml", "roles/web/defaults/main.yml", "inventory/hosts.yml"} {
		if !crHasKind(got[rel], "AnsibleFile") {
			t.Errorf("%s read as %q, want an Ansible file", rel, got[rel])
		}
	}
	for _, rel := range []string{"misc/notes.yml", ".github/workflows/ci.yml", "k8s/deploy.yaml", "compose/docker-compose.yml"} {
		if crHasKind(got[rel], "AnsibleFile") {
			t.Errorf("%s was read as Ansible: %q", rel, got[rel])
		}
	}
	edges := crEdges(t, st, "an")
	for _, w := range []string{
		"site.yml -> an::roles/web/tasks/main.main.yml",
		"roles/web/tasks/main.yml -> an::roles/web/tasks/extra.extra.yml",
		"roles/web/tasks/main.yml -> an::roles/web/handlers/main.restart nginx",
	} {
		found := false
		for _, e := range edges {
			if e == w {
				found = true
			}
		}
		if !found {
			t.Errorf("no stored edge %q; edges: %q", w, edges)
		}
	}
}

// TestReingestFile_AnsibleRepo: a one-file re-ingest decides what ingest did.
func TestReingestFile_AnsibleRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, ansibleRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "an", Path: dir}); err != nil {
		t.Fatal(err)
	}
	before := crSourceKinds(t, st, "an")
	for rel := range ansibleRepoCorpus() {
		if filepath.Ext(rel) != ".yml" && filepath.Ext(rel) != ".yaml" {
			continue
		}
		path := filepath.Join(dir, filepath.FromSlash(rel))
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(b, []byte("# edited\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ReingestFile(ctx, st, "an", dir, rel); err != nil {
			t.Fatal(err)
		}
	}
	after := crSourceKinds(t, st, "an")
	if !reflect.DeepEqual(before, after) {
		t.Errorf("ReingestFile changed what a file reads as\nbefore: %v\n after: %v", before, after)
	}
}
