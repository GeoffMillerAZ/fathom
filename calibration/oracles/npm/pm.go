package main

// pm.go: the sibling edges as the subject's own package manager evaluates them
// (-workspace-pm). npm 11 rejects the `workspace:` protocol, so its join needs
// the spec rewritten (tier C). The protocol is yarn's (Berry) and pnpm's, so a
// subject that names one of them in packageManager is joined by it instead, on a
// scratch copy, offline, with no spec rewritten (tier A):
//
//   - yarn: the subject's own release (.yarnrc.yml yarnPath, committed under
//     .yarn/releases) and plugins run `workspaces list -v --json`, which reports,
//     per workspace, the workspaces its dependencies and devDependencies resolve
//     to (the protocol, transparent workspaces when enabled, and a plugin's own
//     protocol such as babel's `condition:`) and the descriptors that name a
//     workspace whose range they do not accept. Yarn installs nothing to answer.
//   - pnpm: `pnpm ls -r --depth -1 --json --filter '{<dir>}...'` selects a project
//     and the projects pnpm's workspace graph reaches from it (workspace: specs,
//     and ranges when the subject's link-workspace-packages says so; dependencies,
//     devDependencies, optionalDependencies and peerDependencies). The graph is
//     transitive, so every other project's dependency maps are emptied in the copy
//     and one project at a time is restored (the ablation rule): what the filter
//     then adds to the project is exactly the projects its own specs link to.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// yarnWorkspace is one line of `yarn workspaces list -v --json`.
type yarnWorkspace struct {
	Location   string   `json:"location"`
	Name       string   `json:"name"`
	Deps       []string `json:"workspaceDependencies"`
	Mismatched []string `json:"mismatchedWorkspaceDependencies"`
}

func parseYarnWorkspaces(b []byte) ([]yarnWorkspace, error) {
	var out []yarnWorkspace
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var w yarnWorkspace
		if err := json.Unmarshal(line, &w); err != nil {
			return nil, fmt.Errorf("yarn workspaces list: %v: %s", err, tail(string(line), 200))
		}
		out = append(out, w)
	}
	return out, sc.Err()
}

type pmCounts struct {
	listed         int // members the package manager lists (the root excluded)
	mismatched     int // descriptors naming a workspace whose range they do not accept
	unknownTarget  int // an edge to a location no listed member holds
	memberDisagree int
}

// yarnEdges are the member-to-member edges yarn reports, by member name.
func yarnEdges(ws []yarnWorkspace, members map[string]member) ([]edge, pmCounts) {
	var c pmCounts
	byDir := map[string]string{}
	for _, w := range ws {
		byDir[w.Location] = w.Name
	}
	var out []edge
	for _, w := range ws {
		if w.Location != "." {
			c.listed++
		}
		c.mismatched += len(w.Mismatched)
		if _, ok := members[w.Name]; !ok {
			continue
		}
		for _, loc := range w.Deps {
			to, ok := byDir[loc]
			if _, mem := members[to]; !ok || !mem {
				c.unknownTarget++
				continue
			}
			if to != w.Name {
				out = append(out, edge{w.Name, to})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].from != out[j].from {
			return out[i].from < out[j].from
		}
		return out[i].to < out[j].to
	})
	return out, c
}

// yarnPath is the release a .yarnrc.yml pins (yarnPath:), "" when none.
func yarnPath(yarnrc []byte) string {
	for _, l := range strings.Split(string(yarnrc), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimRight(l, "\r"), "yarnPath:"); ok {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}

// stripDependencies is a manifest with its four dependency maps removed and
// everything else kept (pnpm's isolation copy).
func stripDependencies(src []byte) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(src, &m); err != nil {
		return nil, err
	}
	for _, k := range depSections {
		delete(m, k)
	}
	return json.Marshal(m)
}

// pnpmSelection is the projects a `pnpm ls -r --depth -1 --json` lists, as
// directories relative to root ("." for the root), sorted.
func pnpmSelection(b []byte, root string) ([]string, error) {
	var nodes []struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(b, &nodes); err != nil {
		return nil, fmt.Errorf("pnpm ls: %v: %s", err, tail(string(b), 300))
	}
	var out []string
	for _, n := range nodes {
		rel, err := filepath.Rel(root, n.Path)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil, fmt.Errorf("pnpm ls: project %s outside %s", n.Path, root)
		}
		out = append(out, filepath.ToSlash(rel))
	}
	sort.Strings(out)
	return out, nil
}

// pmRun runs the package manager offline in dir with its state under work.
func pmRun(ctx context.Context, work, dir string, argv ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	st := filepath.Join(work, "pmstate")
	cmd.Env = append(os.Environ(),
		"YARN_ENABLE_NETWORK=0", "YARN_ENABLE_TELEMETRY=0", "YARN_ENABLE_GLOBAL_CACHE=0",
		"YARN_GLOBAL_FOLDER="+filepath.Join(st, "yarn-global"), "YARN_CACHE_FOLDER="+filepath.Join(st, "yarn-cache"),
		"npm_config_offline=true", "npm_config_store_dir="+filepath.Join(st, "pnpm-store"),
		"XDG_CACHE_HOME="+filepath.Join(st, "cache"), "XDG_DATA_HOME="+filepath.Join(st, "data"),
		"XDG_STATE_HOME="+filepath.Join(st, "state"), "XDG_CONFIG_HOME="+filepath.Join(st, "config"),
		"COREPACK_ENABLE_STRICT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s in %s: %v: %s", strings.Join(argv, " "), dir, err, tail(stderr.String(), 600))
	}
	return out, nil
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}

// pmEdges evaluates the sibling edges with the subject's package manager on a
// scratch copy of its manifests (-workspace-pm). It returns the edges, the
// directories the package manager lists as members (the root excluded), the
// tool's description and the counts.
func pmEdges(ctx context.Context, mode, pmBin, subject, work string, rels []string, members map[string]member) ([]edge, []string, string, pmCounts, error) {
	var c pmCounts
	dir := filepath.Join(work, "pm")
	if err := copyManifests(subject, dir, rels); err != nil {
		return nil, nil, "", c, err
	}
	switch mode {
	case "yarn":
		rc, err := os.ReadFile(filepath.Join(subject, ".yarnrc.yml"))
		if err != nil {
			return nil, nil, "", c, fmt.Errorf("-workspace-pm yarn: %v", err)
		}
		yp := yarnPath(rc)
		if yp == "" {
			return nil, nil, "", c, fmt.Errorf("-workspace-pm yarn: .yarnrc.yml pins no yarnPath")
		}
		if err := os.WriteFile(filepath.Join(dir, ".yarnrc.yml"), rc, 0o644); err != nil {
			return nil, nil, "", c, err
		}
		// The release and plugins the subject commits (.yarn/releases, .yarn/plugins).
		for _, sub := range []string{"releases", "plugins"} {
			root := filepath.Join(subject, ".yarn", sub)
			_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(subject, p)
				return copyFile(p, filepath.Join(dir, rel))
			})
		}
		node := pmBin
		if node == "" {
			node = "node"
		}
		vb, err := pmRun(ctx, work, dir, node, filepath.Join(dir, yp), "--version")
		if err != nil {
			return nil, nil, "", c, err
		}
		b, err := pmRun(ctx, work, dir, node, filepath.Join(dir, yp), "workspaces", "list", "-v", "--json")
		if err != nil {
			return nil, nil, "", c, err
		}
		ws, err := parseYarnWorkspaces(b)
		if err != nil {
			return nil, nil, "", c, err
		}
		edges, yc := yarnEdges(ws, members)
		var listed []string
		for _, w := range ws {
			if w.Location != "." {
				listed = append(listed, w.Location)
			}
		}
		sort.Strings(listed)
		return edges, listed, "yarn " + strings.TrimSpace(string(vb)) + " workspaces list -v (the subject's own release, " + yp + ")", yc, nil
	case "pnpm":
		if pmBin == "" {
			pmBin = "pnpm"
		}
		for _, f := range []string{"pnpm-workspace.yaml", ".npmrc"} {
			if _, err := os.Stat(filepath.Join(subject, f)); err == nil {
				if err := copyFile(filepath.Join(subject, f), filepath.Join(dir, f)); err != nil {
					return nil, nil, "", c, err
				}
			}
		}
		vb, err := pmRun(ctx, work, dir, pmBin, "--version")
		if err != nil {
			return nil, nil, "", c, err
		}
		b, err := pmRun(ctx, work, dir, pmBin, "ls", "-r", "--depth", "-1", "--json")
		if err != nil {
			return nil, nil, "", c, err
		}
		projects, err := pnpmSelection(b, evalDir(dir))
		if err != nil {
			return nil, nil, "", c, err
		}
		nameOf := map[string]string{}
		for _, m := range members {
			nameOf[m.dir] = m.name
		}
		var listed []string
		orig := map[string][]byte{}
		for _, p := range projects {
			if p != "." {
				listed = append(listed, p)
				c.listed++
			}
			mf := filepath.Join(dir, filepath.FromSlash(p), "package.json")
			src, err := os.ReadFile(mf)
			if err != nil {
				return nil, nil, "", c, err
			}
			orig[p] = src
			stripped, err := stripDependencies(src)
			if err != nil {
				return nil, nil, "", c, fmt.Errorf("%s: %v", p, err)
			}
			if err := os.WriteFile(mf, stripped, 0o644); err != nil {
				return nil, nil, "", c, err
			}
		}
		var edges []edge
		for _, p := range projects {
			from := nameOf[p]
			if from == "" {
				continue // the root: its edges to members are npm's workspaces evaluation
			}
			mf := filepath.Join(dir, filepath.FromSlash(p), "package.json")
			if err := os.WriteFile(mf, orig[p], 0o644); err != nil {
				return nil, nil, "", c, err
			}
			sel, err := pmRun(ctx, work, dir, pmBin, "ls", "-r", "--depth", "-1", "--json", "--filter", "{./"+p+"}...")
			if err != nil {
				return nil, nil, "", c, err
			}
			got, err := pnpmSelection(sel, evalDir(dir))
			if err != nil {
				return nil, nil, "", c, err
			}
			for _, q := range got {
				if q == p {
					continue
				}
				to := nameOf[q]
				if to == "" {
					c.unknownTarget++
					continue
				}
				edges = append(edges, edge{from, to})
			}
			stripped, _ := stripDependencies(orig[p])
			if err := os.WriteFile(mf, stripped, 0o644); err != nil {
				return nil, nil, "", c, err
			}
		}
		sort.Slice(edges, func(i, j int) bool {
			if edges[i].from != edges[j].from {
				return edges[i].from < edges[j].from
			}
			return edges[i].to < edges[j].to
		})
		return edges, listed, "pnpm " + strings.TrimSpace(string(vb)) + " ls -r --filter '{<project>}...' (one project's dependency maps at a time)", c, nil
	}
	return nil, nil, "", c, fmt.Errorf("-workspace-pm %q: want yarn or pnpm", mode)
}
