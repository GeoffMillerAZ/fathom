package main

// gc_strays.go — `fathom gc`'s report-only stewardship classes (SV-4,
// ADR-060 decision 4, vision 41 §4.3): untracked, non-ignored files outside
// the declared homes (workspace/ and the ADR-039 build-cache roots) and
// executable binaries committed or lying in the tree. Both are reported and
// never acted on: they are not fathom's to remove (I63), so they carry
// action "report", which --apply skips and the reclaimable totals leave out.
// The git reads go through gitro, so no index is locked or rewritten.

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/gitro"
)

const (
	gcClassStrays   = "strays"
	gcClassBinaries = "binaries"
)

// gcDeclaredHomes are the repository-relative directories where untracked
// files belong by convention (the workspace/ rule, AGENTS.md).
var gcDeclaredHomes = []string{"workspace/"}

// gcCacheRootsFromEnv names the ADR-039 build-cache roots the environment
// declares: FATHOM_CACHE_ROOT and the Go caches derived from it.
func gcCacheRootsFromEnv() []string {
	var roots []string
	for _, k := range []string{"FATHOM_CACHE_ROOT", "GOCACHE", "GOMODCACHE", "GOTMPDIR"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" && filepath.IsAbs(v) {
			roots = append(roots, v)
		}
	}
	return roots
}

// gcStrays plans the strays and binaries classes for the repository at
// root. cacheRoots are absolute; one inside root is a declared home.
func gcStrays(root string, cacheRoots []string) ([]gcItem, []string) {
	ctx := context.Background()
	var errs []string
	homes := append([]string{}, gcDeclaredHomes...)
	for _, c := range cacheRoots {
		if rel, err := filepath.Rel(root, c); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			homes = append(homes, filepath.ToSlash(rel)+"/")
		}
	}
	inHome := func(rel string) bool {
		for _, h := range homes {
			if strings.HasPrefix(rel, h) || rel+"/" == h {
				return true
			}
		}
		return false
	}

	var items []gcItem
	untracked, err := gitro.Output(ctx, root, "-c", "core.quotePath=false", "ls-files", "-z", "--others", "--exclude-standard", "--directory")
	if err != nil {
		return nil, []string{fmt.Sprintf("strays: git ls-files in %s: %v", root, err)}
	}
	for _, rel := range splitNUL(untracked) {
		if inHome(rel) {
			continue
		}
		p := filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(rel, "/")))
		if strings.HasSuffix(rel, "/") {
			bins := executableBinariesUnder(p)
			for _, b := range bins {
				items = append(items, binaryItem(b, "an executable binary lying untracked in the tree"))
			}
			items = append(items, gcItem{Class: gcClassStrays, Path: p, Action: "report", Bytes: dirSize(p),
				Reason: "an untracked directory outside workspace/ and the build-cache roots: commit it, ignore it or move it under workspace/ (fathom never removes it)"})
			continue
		}
		if isExecutableBinary(p) {
			items = append(items, binaryItem(p, "an executable binary lying untracked in the tree"))
			continue
		}
		var size int64
		if st, err := os.Lstat(p); err == nil {
			size = st.Size()
		}
		items = append(items, gcItem{Class: gcClassStrays, Path: p, Action: "report", Bytes: size,
			Reason: "an untracked file outside workspace/ and the build-cache roots: commit it, ignore it or move it under workspace/ (fathom never removes it)"})
	}

	// Committed executables: the index's 100755 entries whose bytes are a
	// binary, not a script.
	staged, err := gitro.Output(ctx, root, "-c", "core.quotePath=false", "ls-files", "-z", "--stage")
	if err != nil {
		errs = append(errs, fmt.Sprintf("binaries: git ls-files in %s: %v", root, err))
	}
	for _, ent := range splitNUL(staged) {
		meta, rel, ok := strings.Cut(ent, "\t")
		if !ok || !strings.HasPrefix(meta, "100755 ") {
			continue
		}
		p := filepath.Join(root, filepath.FromSlash(rel))
		if isExecutableBinary(p) {
			items = append(items, binaryItem(p, "an executable binary committed to the tree: a build output belongs under workspace/ or a release, not in history (fathom never removes it)"))
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Class != items[j].Class {
			return items[i].Class < items[j].Class
		}
		return items[i].Path < items[j].Path
	})
	return items, errs
}

func binaryItem(p, why string) gcItem {
	var size int64
	if st, err := os.Lstat(p); err == nil {
		size = st.Size()
	}
	return gcItem{Class: gcClassBinaries, Path: p, Action: "report", Bytes: size, Reason: why}
}

// executableBinariesUnder lists the executable binaries inside an untracked
// directory (a nested repository's own files included: they lie in the tree).
func executableBinariesUnder(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.IsDir() && isExecutableBinary(p) {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// binaryMagics are the leading bytes of the executable formats fathom
// recognises: ELF, Mach-O (both widths and byte orders, and universal) and
// PE.
var binaryMagics = [][]byte{
	{0x7f, 'E', 'L', 'F'},
	{0xfe, 0xed, 0xfa, 0xce}, {0xfe, 0xed, 0xfa, 0xcf},
	{0xce, 0xfa, 0xed, 0xfe}, {0xcf, 0xfa, 0xed, 0xfe},
	{0xca, 0xfe, 0xba, 0xbe},
	{'M', 'Z'},
}

// isExecutableBinary reports whether p is a regular file with an execute bit
// whose first bytes are an executable format (a script with a shebang is
// not).
func isExecutableBinary(p string) bool {
	st, err := os.Lstat(p)
	if err != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0o111 == 0 {
		return false
	}
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 4)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	for _, m := range binaryMagics {
		if len(head) >= len(m) && bytes.Equal(head[:len(m)], m) {
			return true
		}
	}
	return false
}

func splitNUL(b []byte) []string {
	var out []string
	for _, s := range strings.Split(string(b), "\x00") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
