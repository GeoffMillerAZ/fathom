package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
)

// fsEntry is one path's state in a snapshot: its kind and, for a file, a
// digest of its bytes (for a link, its target).
type fsEntry struct{ kind, sum string }

// snapshotTree records every directory, file and link under base.
func snapshotTree(t *testing.T, base string) map[string]fsEntry {
	t.Helper()
	m := map[string]fsEntry{}
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			target, rerr := os.Readlink(p)
			if rerr != nil {
				return rerr
			}
			m[p] = fsEntry{"link", target}
		case d.IsDir():
			m[p] = fsEntry{"dir", ""}
		default:
			data, rerr := os.ReadFile(p)
			if rerr != nil {
				return rerr
			}
			sum := sha256.Sum256(data)
			m[p] = fsEntry{"file", hex.EncodeToString(sum[:])}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", base, err)
	}
	return m
}

// touchedFiles is every non-directory path whose state differs between two
// snapshots: created, changed or removed. Sorted.
func touchedFiles(before, after map[string]fsEntry) []string {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for p, a := range after {
		if a.kind == "dir" {
			continue
		}
		if b, ok := before[p]; !ok || b != a {
			add(p)
		}
	}
	for p, b := range before {
		if b.kind == "dir" {
			continue
		}
		if _, ok := after[p]; !ok {
			add(p)
		}
	}
	sort.Strings(out)
	return out
}

var dryRunFileLine = regexp.MustCompile(`^would (create|update|link|remove) (.+)$`)

// dryRunList parses the files a dry run names into absolute paths: a
// repo-relative path is joined to root, a "~/" path to home.
func dryRunList(out, root, home string) []string {
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		m := dryRunFileLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		p := m[2]
		switch {
		case strings.HasPrefix(p, "~/"):
			p = filepath.Join(home, filepath.FromSlash(p[2:]))
		case !filepath.IsAbs(p):
			p = filepath.Join(root, filepath.FromSlash(p))
		}
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// countingInitDaemon is fakeInitDaemon's shape with a count of the ingest
// calls, so a dry run can be held to asking the daemon for none.
func countingInitDaemon(t *testing.T, service string, ingests *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var inner string
		switch req.Params.Name {
		case "ingest":
			ingests.Add(1)
			inner = fmt.Sprintf(`{"service":%q,"ok":true}`, service)
		case "list_services":
			inner = fmt.Sprintf(`{"services":[{"service":%q,"symbol_count":7,"languages":["go"]}]}`, service)
		default:
			http.Error(w, "unknown tool", http.StatusBadRequest)
			return
		}
		env := map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": inner}}}}
		_ = json.NewEncoder(w).Encode(env)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestInitDryRunListsExactlyTheRealRunsTouchedSet holds `fathom init
// --dry-run` to two promises: it writes nothing (the tree, the home and the
// state directory are byte-identical after it, directories included, and
// the daemon is asked for no ingest), and the files it lists are exactly
// the files the real run then creates, changes or removes — the touched set
// read off the disk, not off init's own account of itself.
func TestInitDryRunListsExactlyTheRealRunsTouchedSet(t *testing.T) {
	cases := []struct {
		name  string
		flags []string
		seed  func(t *testing.T, root string, run func([]string) (int, string))
	}{
		{name: "a fresh repository, default flags"},
		{
			name:  "skills and the user ignore seed, over an existing AGENTS.md and CLAUDE.md",
			flags: []string{"--skills", "--seed-user-ignore"},
			seed: func(t *testing.T, root string, _ func([]string) (int, string)) {
				for f, body := range map[string]string{"AGENTS.md": "# Project rules\n", "CLAUDE.md": "# Claude\n"} {
					if err := os.WriteFile(filepath.Join(root, f), []byte(body), 0o644); err != nil {
						t.Fatal(err)
					}
				}
			},
		},
		{
			name:  "a second run over a repository init already wired",
			flags: []string{"--skills"},
			seed: func(t *testing.T, _ string, run func([]string) (int, string)) {
				if code, out := run(nil); code != 0 {
					t.Fatalf("first real run exit %d:\n%s", code, out)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			home := filepath.Join(base, "home")
			root := filepath.Join(base, "repo")
			for _, d := range []string{home, root} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\n\ngo 1.22\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("HOME", home)
			t.Setenv("XDG_STATE_HOME", "")
			t.Setenv("FATHOM_STATE_DIR", "")
			t.Setenv("FATHOM_HOOKS_SCOPE", "")

			var ingests atomic.Int32
			srv := countingInitDaemon(t, "demo-svc", &ingests)
			args := []string{"--path", root, "--service", "demo-svc", "--url", srv.URL, "--bin", testBinPath, "--no-scip"}
			args = append(args, tc.flags...)
			run := func(extra []string) (int, string) {
				var out, errb bytes.Buffer
				code := cmdInit(append(append([]string{}, args...), extra...), &out, &errb)
				return code, out.String() + errb.String()
			}
			if tc.seed != nil {
				tc.seed(t, root, run)
			}

			before := snapshotTree(t, base)
			ingests.Store(0)
			code, dryOut := run([]string{"--dry-run"})
			if code != 0 {
				t.Fatalf("dry run exit %d, want 0:\n%s", code, dryOut)
			}
			if after := snapshotTree(t, base); len(touchedFiles(before, after)) != 0 || len(after) != len(before) {
				t.Fatalf("the dry run wrote: %v (entries %d -> %d)\n%s", touchedFiles(before, after), len(before), len(after), dryOut)
			}
			if n := ingests.Load(); n != 0 {
				t.Fatalf("the dry run asked the daemon for %d ingest(s), want none", n)
			}
			if !strings.Contains(dryOut, "would ingest") {
				t.Errorf("the dry run does not name the daemon ingest it skipped:\n%s", dryOut)
			}
			listed := dryRunList(dryOut, root, home)

			code, realOut := run(nil)
			if code != 0 {
				t.Fatalf("real run exit %d, want 0:\n%s", code, realOut)
			}
			touched := touchedFiles(before, snapshotTree(t, base))
			if strings.Join(listed, "\n") != strings.Join(touched, "\n") {
				t.Fatalf("the dry run's list is not the real run's touched set\nlisted:\n  %s\ntouched:\n  %s\ndry run output:\n%s",
					strings.Join(listed, "\n  "), strings.Join(touched, "\n  "), dryOut)
			}
			if tc.seed == nil && len(listed) == 0 {
				t.Fatalf("a fresh repository listed nothing:\n%s", dryOut)
			}
		})
	}
}
