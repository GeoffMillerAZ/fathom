package fusion

// ansible_classify.go — Ansible files by path and content (LB-F150,
// language-breadth iac wave, item k8s; LB-Q-ansible). A playbook, a role's
// task file or a YAML inventory is a file no basename claims, so the
// yaml-family classifier reads it, and last of all its signals: only a file no
// earlier signal claimed (Kubernetes, Compose, GitLab CI, OpenAPI,
// CloudFormation) can be one, so every earlier verdict is unchanged.
//
// Path signal: roles/<r>/{tasks,handlers,defaults,vars,meta}/*.yml,
// group_vars/ and host_vars/ (a file or a directory of files), and an
// inventory/ YAML whose top-level `all:` carries hosts or children.
// Content signal: a top-level sequence whose items carry `hosts:` or
// `import_playbook:` / `ansible.builtin.import_playbook:` (a playbook).
// Never a file under .github/workflows/.
//
// Two signals beyond the roles/ layout, because most Ansible a repository
// holds is not under a directory named roles/: a standalone role (one whose
// root holds tasks/main.yml, as every role repository does) reads like a role
// under roles/ (a sibling-index fact, so a one-file refresh stats the disk),
// and a YAML file under a tasks/ or handlers/ directory whose top level is a
// sequence of maps (the task files a playbook imports).

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// ToolAnsible: an Ansible playbook, role file, variable file or YAML
// inventory (LB-Q-ansible).
const ToolAnsible ToolKind = "ansible"

var (
	// ansiblePlayRe: a play's `hosts:` (or a play-level import_playbook) as a
	// key of a top-level sequence item: the first key (`- hosts:`) or a later
	// one at the item's indent.
	ansiblePlayRe         = regexp.MustCompile(`(?m)^(?:-[ \t]+|[ \t]{2,4})(?:hosts|import_playbook|ansible\.builtin\.import_playbook)[ \t]*:`)
	ansibleInventoryAllRe = regexp.MustCompile(`(?m)^all[ \t]*:[ \t]*(?:#.*)?\r?$`)
	ansibleInvKeyRe       = regexp.MustCompile(`(?m)^[ \t]+(?:hosts|children)[ \t]*:`)
	ansibleRolePathRe     = regexp.MustCompile(`(?:^|/)roles/[^/]+/(?:tasks|handlers|defaults|vars|meta)/(?:[^/]+/)*[^/]+\.ya?ml$`)
	ansibleVarsPathRe     = regexp.MustCompile(`(?:^|/)(?:group_vars|host_vars)/(?:[^/]+/)*[^/]+\.ya?ml$`)
	ansibleTaskDirRe      = regexp.MustCompile(`(?:^|/)(?:tasks|handlers)/(?:[^/]+/)*[^/]+\.ya?ml$`)
	ansibleTaskItemRe     = regexp.MustCompile(`(?m)^-[ \t]+[A-Za-z_][A-Za-z0-9_.]*[ \t]*:`)
	ansibleRoleSubdirs    = map[string]bool{"tasks": true, "handlers": true, "defaults": true, "vars": true, "meta": true}
)

// noteAnsibleRole records dir as a role root when base is tasks/main.yml's.
func (idx *siblingIndex) noteAnsibleRole(base, dir string) {
	if (base == "main.yml" || base == "main.yaml") && path.Base(dir) == "tasks" {
		idx.ansibleRoleRoots[path.Dir(dir)] = struct{}{}
	}
}

// ansibleRoleFile reports whether slash is a file under tasks/, handlers/,
// defaults/, vars/ or meta/ of a role root idx knows.
func ansibleRoleFile(slash string, idx *siblingIndex) bool {
	if idx == nil || len(idx.ansibleRoleRoots) == 0 {
		return false
	}
	segs := strings.Split(slash, "/")
	for i := 0; i < len(segs)-1; i++ {
		if !ansibleRoleSubdirs[segs[i]] {
			continue
		}
		root := "."
		if i > 0 {
			root = strings.Join(segs[:i], "/")
		}
		if _, ok := idx.ansibleRoleRoots[root]; ok {
			return true
		}
	}
	return false
}

// diskSiblingIndex is the sibling facts a one-file refresh needs for relPath,
// found by looking under repoPath (ingest reads them from the Discover list):
// the Helm chart roots and the Ansible role roots among relPath's ancestors, so
// ReingestFile classifies a chart's template and a role's file as ingest does.
func diskSiblingIndex(repoPath, relPath string) *siblingIndex {
	idx := &siblingIndex{chartDirs: map[string]struct{}{}, ansibleRoleRoots: map[string]struct{}{}}
	segs := strings.Split(filepath.ToSlash(relPath), "/")
	for i := 0; i < len(segs)-1; i++ {
		root := "."
		if i > 0 {
			root = strings.Join(segs[:i], "/")
		}
		dir := filepath.Join(repoPath, filepath.FromSlash(root))
		if _, err := os.Stat(filepath.Join(dir, "Chart.yaml")); err == nil {
			idx.chartDirs[root] = struct{}{}
		}
		for _, name := range []string{"main.yml", "main.yaml"} {
			if _, err := os.Stat(filepath.Join(dir, "tasks", name)); err == nil {
				idx.ansibleRoleRoots[root] = struct{}{}
			}
		}
	}
	return idx
}

// ansibleDetects reports whether a .yaml or .yml file at relPath is Ansible's
// by the path and content signals above.
func ansibleDetects(relPath string, content []byte, idx *siblingIndex) bool {
	slash := strings.TrimPrefix(path.Clean(strings.ReplaceAll(relPath, `\`, "/")), "./")
	if strings.Contains(slash, ".github/workflows/") {
		return false
	}
	if ansibleRolePathRe.MatchString(slash) || ansibleVarsPathRe.MatchString(slash) || ansibleRoleFile(slash, idx) {
		return true
	}
	if ansibleTaskDirRe.MatchString(slash) && topLevelSequence(content) && ansibleTaskItemRe.Match(content) {
		return true
	}
	if (strings.HasPrefix(slash, "inventory/") || strings.Contains(slash, "/inventory/") || strings.HasPrefix(slash, "inventories/") || strings.Contains(slash, "/inventories/")) &&
		ansibleInventoryAllRe.Match(content) && ansibleInvKeyRe.Match(content) {
		return true
	}
	return topLevelSequence(content) && ansiblePlayRe.Match(content)
}

// topLevelSequence reports whether the first content line of a YAML file (past
// comments, blank lines and a `---` marker) opens a block sequence at column 0.
func topLevelSequence(content []byte) bool {
	for _, line := range bytes.Split(content, []byte("\n")) {
		t := bytes.TrimRight(line, " \t\r")
		if len(t) == 0 || t[0] == '#' || bytes.Equal(t, []byte("---")) {
			continue
		}
		return t[0] == '-' && (len(t) == 1 || t[1] == ' ' || t[1] == '\t')
	}
	return false
}
