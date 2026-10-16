package main

// filedeps.go (LB-F458, wave lf8): a chart's file:// dependencies graded by
// Helm itself, offline, tier A for what it attests. Every chart the walk finds
// is listed by `helm dependency list` (helm v4.3.0), so the dependencies are
// the ones Helm's loader reads (Chart.yaml's, and an apiVersion v1 chart's
// requirements.yaml), never the walk's; for each whose repository is a file://
// path Helm reports its status:
//
//   - "unpacked": Helm loaded a subchart of that name from the chart's charts/
//     directory (bitnami's kube-prometheus-crds); the edge's target is that
//     subchart's Chart.yaml.
//   - "missing": nothing under charts/ yet. When every dependency of the chart
//     is a file:// path, a scratch copy of the subject is given
//     `helm dependency update --skip-refresh` (offline: Helm resolves each
//     file:// path against the chart directory, loads the chart there, checks
//     its version against the constraint and packages it into charts/), then
//     `helm dependency list` again: "ok" means the archive Helm packaged is a
//     chart of the dependency's name at a version the constraint admits, so
//     the edge's target is the Chart.yaml of the directory the path names.
//     A chart that also names a remote dependency (oci://, https://) cannot be
//     built offline and is counted, never graded.
//
// Any other status (wrong version, misnamed, corrupt) is no edge and is
// counted by status. The output is cross_references at file granularity in
// the walk's shape (the parent Chart.yaml and chart name, the target Chart.yaml
// and the dependency's name), tier A, with the counts disclosed.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// depRow is one row of `helm dependency list`.
type depRow struct {
	Name, Version, Repository, Status string
}

// parseDepList reads `helm dependency list`'s table: NAME, VERSION,
// REPOSITORY and STATUS columns, the status possibly two words ("wrong
// version"). A row with an empty repository column (a dependency vendored
// under charts/ with none) has three fields; its repository is "". A
// repository is one token (Helm's table pads, never splits it), so a file://
// or a remote row is always read whole.
func parseDepList(out string) []depRow {
	var rows []depRow
	for _, ln := range strings.Split(out, "\n") {
		f := strings.Fields(ln)
		switch {
		case len(f) >= 4 && f[0] == "NAME" && f[2] == "REPOSITORY":
		case len(f) == 3 && !strings.Contains(f[2], "://"):
			rows = append(rows, depRow{Name: f[0], Version: f[1], Status: f[2]})
		case len(f) >= 4:
			rows = append(rows, depRow{Name: f[0], Version: f[1], Repository: f[2], Status: strings.Join(f[3:], " ")})
		}
	}
	return rows
}

// remoteRepo is a repository Helm fetches: an OCI registry or a chart
// repository URL.
func remoteRepo(r string) bool {
	return strings.HasPrefix(r, "oci://") || strings.HasPrefix(r, "https://") || strings.HasPrefix(r, "http://")
}

// localPath is the directory a file:// repository names, Helm's rule
// (resolver.GetLocalPath): the path after file://, joined to the chart
// directory unless absolute. Repository-relative and slash-separated; ""
// when it leaves the subject or is absolute.
func localPath(chartDir, repo string) string {
	p := strings.TrimPrefix(repo, "file://")
	if strings.HasPrefix(p, "/") {
		return ""
	}
	j := path.Join(chartDir, p)
	if j == ".." || strings.HasPrefix(j, "../") {
		return ""
	}
	return j
}

// helmDepList runs `helm dependency list` on dir.
func helmDepList(bin, dir string, env []string) (string, error) {
	return runHelm(bin, env, "dependency", "list", "--max-col-width", "0", dir)
}

// runHelm runs bin with args, retrying a revoked volume grant as render does.
func runHelm(bin string, env []string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	var err error
	for i := 0; i < 6; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		cmd := exec.CommandContext(ctx, bin, args...)
		cmd.Env = env
		stdout.Reset()
		stderr.Reset()
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err = cmd.Run()
		cancel()
		if !strings.Contains(strings.ToLower(stderr.String()), "operation not permitted") {
			break
		}
		if c := os.Getenv("ORACLE_TCC_RESET"); c != "" {
			_ = exec.Command("sh", "-c", c).Run()
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	if err != nil {
		return stdout.String(), fmt.Errorf("helm %s: %v: %s", strings.Join(args[:2], " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// fileDepStats counts the file:// dependencies and what Helm said of each.
type fileDepStats struct {
	charts, deps, attested, unpacked, built int
	// notGraded counts the dependencies no edge is drawn for, by reason.
	notGraded map[string]int
}

// fileDeps grades every file:// dependency of the subject's charts.
func fileDeps(subject, bin, scratch string, charts []chart) ([]xref, fileDepStats, error) {
	st := fileDepStats{notGraded: map[string]int{}}
	env := helmEnv(scratch)
	names := map[string]string{} // Chart.yaml path -> chart name
	for _, c := range charts {
		names[c.path] = c.name
	}
	copyRoot := ""
	var xs []xref
	seen := map[xref]bool{}
	add := func(x xref) {
		if !seen[x] {
			seen[x] = true
			xs = append(xs, x)
			st.attested++
		}
	}
	for _, c := range charts {
		chartDir := path.Dir(c.path)
		out, err := helmDepList(bin, filepath.Join(subject, filepath.FromSlash(chartDir)), env)
		if err != nil {
			for _, r := range c.repos {
				if strings.HasPrefix(r, "file://") {
					st.notGraded["chart-not-loadable"]++
				}
			}
			continue
		}
		var rows []depRow
		remote := 0
		for _, r := range parseDepList(out) {
			switch {
			case strings.HasPrefix(r.Repository, "file://"):
				rows = append(rows, r)
			case remoteRepo(r.Repository):
				remote++
			}
		}
		if len(rows) == 0 {
			continue
		}
		st.charts++
		st.deps += len(rows)
		var missing []depRow
		for _, r := range rows {
			switch r.Status {
			case "unpacked":
				t := unpackedTarget(chartDir, r.Name, names)
				if t == "" {
					st.notGraded["unpacked-unlocated"]++
					continue
				}
				st.unpacked++
				add(xref{c.path, c.name, t, r.Name})
			case "missing":
				missing = append(missing, r)
			default:
				st.notGraded["status-"+strings.ReplaceAll(r.Status, " ", "-")]++
			}
		}
		if len(missing) == 0 {
			continue
		}
		if remote > 0 {
			st.notGraded["mixed-remote-not-built"] += len(missing)
			continue
		}
		if copyRoot == "" {
			copyRoot = filepath.Join(scratch, "file-deps")
			if err := os.RemoveAll(copyRoot); err != nil {
				return nil, st, err
			}
			if err := os.CopyFS(copyRoot, os.DirFS(subject)); err != nil {
				return nil, st, fmt.Errorf("copy %s: %w", subject, err)
			}
		}
		dir := filepath.Join(copyRoot, filepath.FromSlash(chartDir))
		if _, err := runHelm(bin, env, "dependency", "update", "--skip-refresh", dir); err != nil {
			st.notGraded["update-failed"] += len(missing)
			fmt.Fprintf(os.Stderr, "helm-oracle: file-deps: %s: %v\n", c.path, err)
			continue
		}
		after, err := helmDepList(bin, dir, env)
		if err != nil {
			st.notGraded["chart-not-loadable"] += len(missing)
			continue
		}
		status := map[string]string{}
		for _, r := range parseDepList(after) {
			status[r.Name+"\x00"+r.Repository] = r.Status
		}
		for _, r := range missing {
			s := status[r.Name+"\x00"+r.Repository]
			t := localPath(chartDir, r.Repository)
			switch {
			case s != "ok":
				st.notGraded["built-status-"+strings.ReplaceAll(orNone(s), " ", "-")]++
			case t == "" || names[path.Join(t, "Chart.yaml")] != r.Name:
				st.notGraded["built-target-unlocated"]++
			default:
				st.built++
				add(xref{c.path, c.name, path.Join(t, "Chart.yaml"), r.Name})
			}
		}
	}
	slices.SortFunc(xs, func(a, b xref) int {
		return strings.Compare(a.Path+"\x00"+a.From+"\x00"+a.ToPath+"\x00"+a.To, b.Path+"\x00"+b.From+"\x00"+b.ToPath+"\x00"+b.To)
	})
	return xs, st, nil
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// unpackedTarget is the Chart.yaml of the subchart named name that Helm
// loads from chartDir's charts/ directory: charts/<d>/Chart.yaml whose chart
// name is name ("" when none is, or several are).
func unpackedTarget(chartDir, name string, names map[string]string) string {
	prefix := path.Join(chartDir, "charts") + "/"
	var hit []string
	for p, n := range names {
		rest, ok := strings.CutPrefix(p, prefix)
		if ok && n == name && strings.Count(rest, "/") == 1 && strings.HasSuffix(rest, "/Chart.yaml") {
			hit = append(hit, p)
		}
	}
	if len(hit) != 1 {
		return ""
	}
	return hit[0]
}

// fileDepsOut is -file-deps-out's file: tier-A cross_references only.
type fileDepsOut struct {
	Tool            string   `json:"tool"`
	Version         string   `json:"version"`
	Tier            string   `json:"tier"`
	Definitions     []def    `json:"definitions"`
	CrossReferences []xref   `json:"cross_references"`
	Disclose        []string `json:"disclose,omitempty"`
}

// writeFileDeps runs fileDeps and writes its file.
func writeFileDeps(subject, bin, scratch, outPath string, charts []chart) error {
	ver, err := helmVersion(bin)
	if err != nil {
		return err
	}
	xs, st, err := fileDeps(subject, bin, scratch, charts)
	if err != nil {
		return err
	}
	var ng []string
	for _, k := range slices.Sorted(maps.Keys(st.notGraded)) {
		ng = append(ng, fmt.Sprintf("%s %d", k, st.notGraded[k]))
	}
	disclose := fmt.Sprintf("file:// dependencies graded by helm dependency list (helm %s, offline, tier A; LB-F458): %d in %d charts, %d edges attested (%d unpacked under charts/, %d packaged by helm dependency update --skip-refresh from the directory the path names); not graded: [%s]",
		ver, st.deps, st.charts, st.attested, st.unpacked, st.built, strings.Join(ng, ", "))
	fmt.Fprintln(os.Stderr, "helm-oracle: "+disclose)
	if xs == nil {
		xs = []xref{}
	}
	b, err := json.MarshalIndent(fileDepsOut{
		Tool:            "helm dependency list (file:// dependencies; helm dependency update --skip-refresh in a scratch copy for one not under charts/)",
		Version:         "helm " + ver,
		Tier:            "A",
		Definitions:     []def{},
		CrossReferences: xs,
		Disclose:        []string{disclose},
	}, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, append(b, '\n'), 0o644)
}
