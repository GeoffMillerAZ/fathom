package semantic

// rulesread.go — the file access the GL-2..GL-6 readers share. Readers run
// at compile, never at request (design doc §9 "Rules readers run at
// compile"); each reads only its own client's directory or its own file
// name, with a per-file size cap, and a nested walk prunes the connector's
// own skip set plus every hidden directory — the same tree the index sees.

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// maxRulesFileBytes caps one rules file: a rules file is prose a human
// wrote for an agent, and anything past this is not one.
const maxRulesFileBytes = 256 << 10

// dirExists reports whether root/rel is a directory.
func dirExists(root, rel string) bool {
	fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil && fi.IsDir()
}

// fileExists reports whether root/rel is a regular file.
func fileExists(root, rel string) bool {
	fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil && fi.Mode().IsRegular()
}

// readRulesFile reads root/rel into a SourceFile; ok=false for a missing,
// non-regular or oversize file.
func readRulesFile(root, rel string) (core.SourceFile, bool) {
	p := filepath.Join(root, filepath.FromSlash(rel))
	fi, err := os.Stat(p)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxRulesFileBytes {
		return core.SourceFile{}, false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return core.SourceFile{}, false
	}
	return core.SourceFile{Path: rel, Content: b, Lang: "markdown"}, true
}

// readRulesDir reads every regular file under root/dir (recursively, the
// client's own rules directory only) whose name satisfies keep, sorted by
// path.
func readRulesDir(root, dir string, keep func(name string) bool) ([]core.SourceFile, error) {
	base := filepath.Join(root, filepath.FromSlash(dir))
	var out []core.SourceFile
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == base {
				return err
			}
			return nil
		}
		if d.IsDir() || !keep(d.Name()) {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return nil
		}
		if f, ok := readRulesFile(root, filepath.ToSlash(rel)); ok {
			out = append(out, f)
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, err
}

// findNamedFiles returns every repo-relative path under root whose base
// name is name, pruning the connector's skip set and hidden directories
// (except the hidden directory names in keepHidden — Bugbot's own
// `.cursor/`), sorted. Used for the nested files (BUGBOT.md, GEMINI.md) a
// client reads at any depth.
func findNamedFiles(root, name string, keepHidden ...string) []string {
	var out []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() && p != root {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if p != root && (connector.IsSkippedDir(d.Name()) || strings.HasPrefix(d.Name(), ".") && !slices.Contains(keepHidden, d.Name())) {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() != name {
			return nil
		}
		if rel, rerr := filepath.Rel(root, p); rerr == nil {
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// readNamedFiles reads every file findNamedFiles names.
func readNamedFiles(root, name string, keepHidden ...string) []core.SourceFile {
	var out []core.SourceFile
	for _, rel := range findNamedFiles(root, name, keepHidden...) {
		if f, ok := readRulesFile(root, rel); ok {
			out = append(out, f)
		}
	}
	return out
}

// underDir reports whether a slash path sits under dir/ (dir itself
// excluded).
func underDir(p, dir string) bool {
	return strings.HasPrefix(path.Clean(p), dir+"/")
}

// baseName is a slash path's last element.
func baseName(p string) string { return path.Base(p) }
