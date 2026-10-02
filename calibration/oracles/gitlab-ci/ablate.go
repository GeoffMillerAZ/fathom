package main

// ablate.go: tier-A references by ablation (the kustomize oracle's rule). The
// tool evaluates a pipeline into its merged form, which erases extends: and
// inherits needs: through it, so what a job or hidden template writes is not
// in the tool's answer. Each written reference is therefore attested by
// removing it: the entry is cut from the file in a scratch copy of the
// pipeline, and the tool either rejects the pipeline or its --preview (the
// pipeline with defaults, includes, extends and !reference expanded) changes.
// A reference whose removal changes nothing is not attested, and is counted.
//
// The edit is made on the yaml.v3 node tree and the file re-encoded, so the
// baseline every ablation is compared against is the same file re-encoded
// without the cut (and that baseline is itself compared with the tool's answer
// on the original bytes: a re-encode that changes the tool's answer is counted).

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// parseDocs decodes every document of a YAML file, as document nodes.
func parseDocs(b []byte) ([]*yaml.Node, error) {
	dec := yaml.NewDecoder(bytes.NewReader(b))
	var docs []*yaml.Node
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				return docs, nil
			}
			return nil, err
		}
		d := doc
		docs = append(docs, &d)
	}
}

func encodeDocs(docs []*yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	for _, d := range docs {
		if err := enc.Encode(d); err != nil {
			return nil, err
		}
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func docRoot(d *yaml.Node) *yaml.Node {
	if d == nil || len(d.Content) == 0 {
		return nil
	}
	r := deref(d.Content[0])
	if r.Kind != yaml.MappingNode {
		return nil
	}
	return r
}

// lastTop is the mapping value of the last top-level key name across the
// documents (the one Psych and the tool read), nil when there is none.
func lastTop(docs []*yaml.Node, name string) *yaml.Node {
	var found *yaml.Node
	for _, d := range docs {
		r := docRoot(d)
		if r == nil {
			continue
		}
		for i := 0; i+1 < len(r.Content); i += 2 {
			if r.Content[i].Value == name {
				if v := deref(r.Content[i+1]); v.Kind == yaml.MappingNode {
					found = v
				} else {
					found = nil
				}
			}
		}
	}
	return found
}

// names reports whether an edge item names target: a scalar, or a needs map's job.
func names(it *yaml.Node, edge, target string) bool {
	it = deref(it)
	switch {
	case it.Kind == yaml.ScalarNode:
		return it.Value == target
	case it.Kind == yaml.MappingNode && edge == "needs":
		return scalar(get(it, "job")) == target
	}
	return false
}

// ablateEdge removes, from entity's last edge key, the item that names target;
// an edge written as one scalar loses its key. False when nothing was removed.
func ablateEdge(docs []*yaml.Node, entity, edge, target string) bool {
	m := lastTop(docs, entity)
	if m == nil {
		return false
	}
	at := -1
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == edge {
			at = i
		}
	}
	if at < 0 {
		return false
	}
	v := deref(m.Content[at+1])
	switch v.Kind {
	case yaml.ScalarNode:
		if v.Value != target {
			return false
		}
		m.Content = append(m.Content[:at], m.Content[at+2:]...)
		return true
	case yaml.SequenceNode:
		kept := v.Content[:0:0]
		removed := false
		for _, it := range v.Content {
			if names(it, edge, target) {
				removed = true
				continue
			}
			kept = append(kept, it)
		}
		if removed {
			v.Content = kept
			if len(kept) == 0 {
				v.Style = yaml.FlowStyle
			}
		}
		return removed
	}
	return false
}

// ablateDecl removes every top-level key name from the documents.
func ablateDecl(docs []*yaml.Node, name string) bool {
	removed := false
	for _, d := range docs {
		r := docRoot(d)
		if r == nil {
			continue
		}
		kept := r.Content[:0:0]
		for i := 0; i+1 < len(r.Content); i += 2 {
			if r.Content[i].Value == name {
				removed = true
				continue
			}
			kept = append(kept, r.Content[i], r.Content[i+1])
		}
		r.Content = kept
	}
	return removed
}

// ablator runs the tool's --preview on a scratch copy of one pipeline.
type ablator struct {
	bin, proj, file, stateRel string
	cacheDir                  string
	orig                      map[string][]byte // pipeline-relative file -> original bytes
	base                      map[string]string // file -> preview with that file re-encoded, uncut
	baseOK                    map[string]bool
	runs                      int
}

// newAblator copies the pipeline's files (subject-relative, under rel) into
// proj, a git repository whose origin is the subject's (the tool reads it),
// and records the tool's preview of the original bytes.
func newAblator(ctx context.Context, bin, subject, rel string, files []string, proj, root, state, cacheDir string) (*ablator, string, error) {
	if err := os.RemoveAll(proj); err != nil {
		return nil, "", err
	}
	a := &ablator{bin: bin, proj: proj, file: root, cacheDir: cacheDir, orig: map[string][]byte{}, base: map[string]string{}, baseOK: map[string]bool{}}
	for _, f := range files {
		pr := f
		if rel != "" {
			pr = strings.TrimPrefix(f, rel+"/")
		}
		b, err := os.ReadFile(filepath.Join(subject, filepath.FromSlash(f)))
		if err != nil {
			return nil, "", err
		}
		dst := filepath.Join(proj, filepath.FromSlash(pr))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, "", err
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return nil, "", err
		}
		a.orig[pr] = b
	}
	url := ""
	if out, err := exec.Command("git", "-C", subject, "remote", "get-url", "origin").Output(); err == nil {
		url = strings.TrimSpace(string(out))
	}
	// The tool diffs against the remote's default branch for rules:changes, so
	// origin/main is the one commit too (no file changed, in every run alike).
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", url}, {"add", "-A"},
		{"-c", "user.email=oracle@example.com", "-c", "user.name=oracle", "commit", "-q", "-m", "pipeline"},
		{"update-ref", "refs/remotes/origin/main", "HEAD"}, {"symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main"}} {
		if args[0] == "remote" && url == "" {
			continue
		}
		cmd := exec.Command("git", args...)
		cmd.Dir = proj
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
	}
	r, err := filepath.Rel(proj, state)
	if err != nil {
		return nil, "", err
	}
	a.stateRel = r
	p, err := a.preview(ctx)
	return a, p, err
}

// preview is the tool's --preview of the copy as it stands, as a digest (only
// equality matters). Each answer is kept under cacheDir by a digest of the
// copy's files, so a run stopped by the machine resumes where it was.
func (a *ablator) preview(ctx context.Context) (string, error) {
	h := sha256.New()
	names := make([]string, 0, len(a.orig))
	for f := range a.orig {
		names = append(names, f)
	}
	sort.Strings(names)
	for _, f := range names {
		b, err := os.ReadFile(filepath.Join(a.proj, filepath.FromSlash(f)))
		if err != nil {
			envStop(err)
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", f, len(b))
		h.Write(b)
	}
	key := filepath.Join(a.cacheDir, hex.EncodeToString(h.Sum(nil))+".json")
	type cached struct {
		Out string `json:"out"`
		Err string `json:"err,omitempty"`
	}
	if b, err := os.ReadFile(key); err == nil {
		var c cached
		if json.Unmarshal(b, &c) == nil {
			if c.Err != "" {
				return "", errors.New(c.Err)
			}
			return c.Out, nil
		}
	}
	out, err := a.run(ctx)
	if err != nil {
		// A rejection is the tool's answer only when it repeats.
		if out2, err2 := a.run(ctx); err2 == nil {
			out, err = out2, nil
		}
	}
	c := cached{Out: out}
	if err != nil {
		c.Err = err.Error()
	}
	if b, merr := json.Marshal(c); merr == nil {
		_ = os.MkdirAll(a.cacheDir, 0o755)
		_ = os.WriteFile(key, b, 0o644)
	}
	return out, err
}

func (a *ablator) run(ctx context.Context) (string, error) {
	a.runs++
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, a.bin, "--file", a.file, "--state-dir", a.stateRel, "--preview")
	cmd.Dir = a.proj
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	if err != nil {
		msg := stderr.String()
		// A failure of the machine, not of the pipeline, is never a rejection:
		// stop rather than attest a reference by it.
		reason := ""
		switch l := strings.ToLower(msg); {
		case strings.Contains(l, "operation not permitted"), strings.Contains(msg, "EPERM"):
			reason = "EPERM (operation not permitted)"
		case strings.Contains(msg, "ENOSPC"):
			reason = "ENOSPC"
		case ctx.Err() != nil:
			reason = "timeout"
		}
		if reason != "" {
			fmt.Fprintf(os.Stderr, "gitlab-ci-oracle: the tool failed for a reason outside the pipeline (%s), stopping: %s\n", reason, tail(msg, 300))
			os.Exit(3)
		}
		return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(tail(msg, 300)))
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// with writes file f as cut(docs) re-encoded, runs the preview, and restores f.
// cut false means nothing could be cut (no run is made).
func (a *ablator) with(ctx context.Context, f string, cut func([]*yaml.Node) bool) (out string, ran bool, err error) {
	src, ok := a.orig[f]
	if !ok {
		return "", false, fmt.Errorf("%s is not a file of the pipeline copy", f)
	}
	docs, perr := parseDocs(src)
	if perr != nil {
		return "", false, perr
	}
	if !cut(docs) {
		return "", false, nil
	}
	b, eerr := encodeDocs(docs)
	if eerr != nil {
		return "", false, eerr
	}
	dst := filepath.Join(a.proj, filepath.FromSlash(f))
	if werr := os.WriteFile(dst, b, 0o644); werr != nil {
		envStop(werr)
		return "", false, werr
	}
	defer os.WriteFile(dst, a.orig[f], 0o644)
	out, err = a.preview(ctx)
	return out, true, err
}

// baseline is the preview with f re-encoded and nothing cut (cached).
func (a *ablator) baseline(ctx context.Context, f string) (string, bool) {
	if _, ok := a.baseOK[f]; ok {
		return a.base[f], a.baseOK[f]
	}
	out, _, err := a.with(ctx, f, func([]*yaml.Node) bool { return true })
	a.base[f], a.baseOK[f] = out, err == nil
	return out, err == nil
}

// changes reports whether cutting changes the tool's answer on f: it rejects
// the pipeline, or its preview differs from f's baseline. why says why nothing
// was compared: the baseline failed, or nothing could be cut.
func (a *ablator) changes(ctx context.Context, f string, cut func([]*yaml.Node) bool) (changed bool, why string) {
	base, bok := a.baseline(ctx, f)
	if !bok {
		return false, "the baseline failed"
	}
	out, ran, err := a.with(ctx, f, cut)
	if !ran {
		if err != nil {
			return false, err.Error()
		}
		return false, "nothing to cut"
	}
	return err != nil || out != base, ""
}

// envStop stops the run on a failure of the machine (a file the process may
// not read or write): such a failure must never read as the tool's answer.
func envStop(err error) {
	if err != nil && (errors.Is(err, os.ErrPermission) || strings.Contains(strings.ToLower(err.Error()), "operation not permitted")) {
		fmt.Fprintf(os.Stderr, "gitlab-ci-oracle: a file operation failed outside the pipeline (EPERM (operation not permitted)), stopping: %v\n", err)
		os.Exit(3)
	}
}
