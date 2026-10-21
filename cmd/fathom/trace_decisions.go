package main

// Explicit trace decision inputs are literal, bounded reads of Git objects at
// COMMIT. The default builder keeps its original paths/parser unchanged.
import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	goldast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

const (
	traceDecisionSelectorLimit = 16
	traceDecisionFileLimit     = 256
	traceDecisionFileBytes     = 1 << 20
	traceDecisionTotalBytes    = 8 << 20
	traceDecisionTreeBytes     = 1 << 20
	traceDecisionPathBytes     = 4096
)

func traceDecisionPath(raw string) (string, error) {
	if raw == "" || len(raw) > traceDecisionPathBytes || !utf8.ValidString(raw) || strings.Contains(raw, `\`) || filepath.IsAbs(raw) || path.IsAbs(raw) {
		return "", fmt.Errorf("decision source path must be a bounded repository-relative literal: %q", raw)
	}
	if len(raw) >= 3 && raw[1] == ':' && raw[2] == '/' {
		return "", fmt.Errorf("decision source path must not be absolute: %q", raw)
	}
	for _, r := range raw {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("decision source path contains a control character: %q", raw)
		}
	}
	for _, component := range strings.Split(raw, "/") {
		if component == ".." {
			return "", fmt.Errorf("decision source traversal is forbidden: %q", raw)
		}
	}
	return path.Clean(raw), nil
}

func canonicalTraceDecisionSources(selectors []string) ([]string, error) {
	if len(selectors) == 0 {
		return nil, nil
	}
	seen := map[string]bool{}
	for _, raw := range selectors {
		p, err := traceDecisionPath(raw)
		if err != nil {
			return nil, err
		}
		seen[p] = true
		if len(seen) > traceDecisionSelectorLimit {
			return nil, fmt.Errorf("decision source selector limit exceeded (%d)", traceDecisionSelectorLimit)
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

// Git stderr is diagnostic, never an unbounded second output channel.
type traceGitDiagnostic []byte

func (b *traceGitDiagnostic) Write(p []byte) (int, error) {
	n := len(p)
	if remaining := (16 << 10) - len(*b); remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		*b = append(*b, p...)
	}
	return n, nil
}

func traceDecisionGit(ctx context.Context, repo string, limit int, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repo}, args...)...)
	var diagnostic traceGitDiagnostic
	cmd.Stderr = &diagnostic
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("decision source git: %w", err)
	}
	b, readErr := io.ReadAll(io.LimitReader(pipe, int64(limit)+1))
	if readErr != nil || len(b) > limit {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if readErr != nil {
			return nil, fmt.Errorf("decision source read: %w", readErr)
		}
		return nil, fmt.Errorf("decision source Git output limit exceeded (%d bytes)", limit)
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("decision source git: %w: %s", err, strings.TrimSpace(string(diagnostic)))
	}
	return b, nil
}

type traceDecisionBlob struct {
	path string
	size int
}

func traceDecisionBlobs(ctx context.Context, repo, commit string, selectors []string) ([]traceDecisionBlob, error) {
	args := []string{"ls-tree", "-r", "-z", "-l", "--full-tree", commit, "--"}
	for _, p := range selectors {
		args = append(args, ":(literal)"+p)
	}
	listing, err := traceDecisionGit(ctx, repo, traceDecisionTreeBytes, args...)
	if err != nil {
		return nil, err
	}
	matched := map[string]bool{}
	files := map[string]traceDecisionBlob{}
	total := 0
	for _, record := range bytes.Split(listing, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		meta, rawPath, ok := bytes.Cut(record, []byte{'\t'})
		if !ok {
			return nil, fmt.Errorf("decision source tree has malformed metadata")
		}
		p, err := traceDecisionPath(string(rawPath))
		if err != nil || p != string(rawPath) {
			return nil, fmt.Errorf("decision source tree has unsafe path %q", rawPath)
		}
		fields := strings.Fields(string(meta))
		if len(fields) != 4 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") {
			return nil, fmt.Errorf("decision source %q is not a regular Git blob (symlinks/gitlinks forbidden)", p)
		}
		if path.Ext(p) != ".md" {
			continue
		}
		for _, selector := range selectors {
			if selector == "." || p == selector || strings.HasPrefix(p, selector+"/") {
				matched[selector] = true
			}
		}
		if _, exists := files[p]; exists {
			continue
		}
		size, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil || size < 0 || size > traceDecisionFileBytes {
			return nil, fmt.Errorf("decision source %q file size limit exceeded (%d bytes)", p, traceDecisionFileBytes)
		}
		total += int(size)
		if len(files) == traceDecisionFileLimit || total > traceDecisionTotalBytes {
			return nil, fmt.Errorf("decision source file/total limit exceeded (%d files, %d bytes)", traceDecisionFileLimit, traceDecisionTotalBytes)
		}
		files[p] = traceDecisionBlob{path: p, size: int(size)}
	}
	for _, selector := range selectors {
		if !matched[selector] {
			return nil, fmt.Errorf("decision source %q has no committed Markdown file", selector)
		}
	}
	out := make([]traceDecisionBlob, 0, len(files))
	for _, blob := range files {
		out = append(out, blob)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out, nil
}

var (
	traceProjectADR       = regexp.MustCompile(`^(ADR(?:-| )?\d+)\b`)
	traceProjectInvariant = regexp.MustCompile(`^I\d+$`)
)

// Declarations come from real Markdown nodes, never filenames, numbered
// prose, or fenced/indented examples. IDs retain their exact spelling.
func traceProjectDecisionIDs(source []byte, decisions map[string]bool) {
	doc := goldmark.New(goldmark.WithExtensions(extension.Table)).Parser().Parse(text.NewReader(source))
	_ = goldast.Walk(doc, func(n goldast.Node, entering bool) (goldast.WalkStatus, error) {
		if !entering {
			return goldast.WalkContinue, nil
		}
		switch n.(type) {
		case *goldast.Heading:
			if match := traceProjectADR.FindSubmatch(bytes.TrimSpace(n.Text(source))); match != nil {
				decisions[string(match[1])] = true
			}
		case *extast.TableRow:
			if cell := n.FirstChild(); cell != nil {
				id := strings.TrimSpace(string(cell.Text(source)))
				if traceProjectInvariant.MatchString(id) {
					decisions[id] = true
				}
			}
		}
		return goldast.WalkContinue, nil
	})
}
