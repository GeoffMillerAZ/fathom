// Parse-gate measures the pinned Jinja subjects with the embedded WASM parser.
// File-level evidence is written only when -details names an ignored workspace path.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

type subject struct {
	Name           string `json:"name"`
	Files          int    `json:"files"`
	Bytes          int64  `json:"bytes"`
	ContentSHA256  string `json:"content_sha256"`
	Errors         int    `json:"errors"`
	ErrorSetSHA256 string `json:"error_set_sha256"`
	Faults         int    `json:"faults"`
	FaultSetSHA256 string `json:"fault_set_sha256"`
}
type detail struct {
	Name   string   `json:"name"`
	Errors []string `json:"errors"`
	Faults []string `json:"faults"`
}

func setHash(names []string) string {
	h := sha256.New()
	for _, name := range names {
		h.Write([]byte(name))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func measure(name, root, subpath, commit string, p *tsengine.Parser, q *tsengine.Query) (subject, detail, error) {
	s := subject{Name: name}
	d := detail{Name: name, Errors: []string{}, Faults: []string{}}
	repo := filepath.Dir(filepath.Dir(root))
	if filepath.ToSlash(filepath.Clean(root)) != filepath.ToSlash(filepath.Clean(filepath.Join(repo, subpath))) {
		return s, d, fmt.Errorf("%s: expected pinned subject subpath", name)
	}
	head, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil || strings.TrimSpace(string(head)) != commit {
		return s, d, fmt.Errorf("%s: subject HEAD differs from pinned commit", name)
	}
	status, err := exec.Command("git", "-C", repo, "status", "--porcelain", "--untracked-files=all", "--ignored", "--", subpath).Output()
	if err != nil || len(status) != 0 {
		return s, d, fmt.Errorf("%s: pinned subject subtree must be clean, with no untracked or ignored files", name)
	}
	paths := []string{}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		switch filepath.Ext(path) {
		case ".j2", ".jinja", ".jinja2":
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return s, d, err
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return s, d, fmt.Errorf("%s: no templates found", name)
	}
	h := sha256.New()
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return s, d, readErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return s, d, relErr
		}
		rel = filepath.ToSlash(rel)
		sum := sha256.Sum256(data)
		h.Write([]byte(rel))
		h.Write([]byte{0})
		h.Write(sum[:])
		h.Write([]byte{0})
		s.Files++
		s.Bytes += int64(len(data))
		tree, parseErr := p.Parse(data)
		if parseErr != nil {
			d.Faults = append(d.Faults, rel)
			continue
		}
		matches, queryErr := q.Run(tree)
		tree.Close()
		if queryErr != nil {
			d.Faults = append(d.Faults, rel)
		} else if len(matches) > 0 {
			d.Errors = append(d.Errors, rel)
		}
	}
	s.ContentSHA256 = hex.EncodeToString(h.Sum(nil))
	s.Errors = len(d.Errors)
	s.ErrorSetSHA256 = setHash(d.Errors)
	s.Faults = len(d.Faults)
	s.FaultSetSHA256 = setHash(d.Faults)
	return s, d, nil
}

func run() error {
	kolla := flag.String("kolla", "workspace/calibration/kolla/kolla/docker", "pinned kolla template root")
	ansible := flag.String("kolla-ansible", "workspace/calibration/kolla-ansible/ansible/roles", "pinned kolla-ansible template root")
	details := flag.String("details", "", "private file-level JSON under workspace/")
	flag.Parse()
	if *details != "" {
		workspace, err := filepath.Abs("workspace")
		if err != nil {
			return err
		}
		target, err := filepath.Abs(*details)
		if err != nil {
			return err
		}
		parent, err := filepath.EvalSymlinks(filepath.Dir(target))
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(workspace, filepath.Join(parent, filepath.Base(target)))
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("-details must be under this checkout's workspace/")
		}
	}
	p, err := tsengine.NewParser(context.Background(), "jinja2", "tree_sitter_jinja")
	if err != nil {
		return err
	}
	defer p.Close()
	q, err := p.CompileQuery("[(ERROR) (MISSING)] @error")
	if err != nil {
		return err
	}
	defer q.Close()
	specs := []struct{ name, root, subpath, commit string }{
		{"kolla", *kolla, "kolla/docker", "27e42412114c29669f70a298f89c54abc6a7f4ac"},
		{"kolla-ansible", *ansible, "ansible/roles", "4b493ef36ace2b90257ecf94feb00e2c6e8014c9"},
	}
	rows := []subject{}
	private := []detail{}
	for _, spec := range specs {
		s, d, e := measure(spec.name, spec.root, spec.subpath, spec.commit, p, q)
		if e != nil {
			return e
		}
		rows = append(rows, s)
		private = append(private, d)
	}
	if *details != "" {
		b, e := json.MarshalIndent(private, "", "  ")
		if e != nil {
			return e
		}
		if e = os.WriteFile(*details, append(b, '\n'), 0600); e != nil {
			return e
		}
	}
	return json.NewEncoder(os.Stdout).Encode(rows)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
