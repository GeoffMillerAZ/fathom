package assetrefs

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

const ContextSchema = "fathom.unity.asset-context/v2"

func checkSourceMemberCount(raw, assets, settings int) error {
	remaining := memberLimit
	for _, count := range []int{raw, assets, settings} {
		if count < 0 || count > remaining {
			return refused("combined source/context membership exceeds50000")
		}
		remaining -= count
	}
	return nil
}

func authoredContextPath(p string) bool {
	if !relative(p) || !strings.HasPrefix(p, "Assets/") {
		return false
	}
	switch strings.ToLower(path.Ext(p)) {
	case ".cs", ".asmdef", ".asmref", ".rsp", ".meta":
		return true
	}
	return false
}
func settingsContextPath(p string) bool {
	return relative(p) && strings.HasPrefix(p, "ProjectSettings/")
}

func contextBytes(c contextInventory) []byte {
	files := append([]fileHash(nil), c.AssetsCode...)
	files = append(files, c.ProjectSettings...)
	slices.SortFunc(files, func(a, b fileHash) int { return strings.Compare(a.Path, b.Path) })
	var b strings.Builder
	b.WriteString(ContextSchema + "\n")
	for _, file := range files {
		b.WriteString("file\x00" + file.Path + "\x00" + file.SHA256 + "\n")
	}
	return []byte(b.String())
}

// Context discovery does not inherit the asset parser's vendor exclusion.
// Vendor/ThirdParty source and all Assets metadata may change local type/GUID
// or importer identity. These bytes are opaque semantic context, never parsed,
// catalogued or exposed as code by this pass.
func contextMembers(ctx context.Context, projectRoot, tree string) ([]string, error) {
	var result []string
	governor := tenancy.NewRowGovernor(ctx, tenancy.ClassTree)
	err := filepath.WalkDir(filepath.Join(projectRoot, tree), func(p string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := governor.Row(); err != nil {
			return err
		}
		rel, err := filepath.Rel(projectRoot, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.Type()&fs.ModeSymlink != 0 {
			return refused("symlink semantic context %s", rel)
		}
		if entry.IsDir() {
			return nil
		}
		if tree == "Assets" && !authoredContextPath(rel) {
			return nil
		}
		if !entry.Type().IsRegular() || !relative(rel) {
			return refused("unsafe/nonregular semantic context %s", rel)
		}
		result = append(result, rel)
		if len(result) > memberLimit {
			return refused("semantic context membership exceeds bound")
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: semantic context discovery: %w", ErrRefused, err)
	}
	slices.Sort(result)
	return result, nil
}

func validateContext(ctx context.Context, projectRoot string, c *contextInventory, alreadyRead int) error {
	if c == nil || c.Schema != ContextSchema || !shaPattern.MatchString(c.Before) || c.Before != c.After || digest(contextBytes(*c)) != c.Before || len(c.AssetsCode)+len(c.ProjectSettings) > memberLimit || len(c.ProjectSettings) == 0 {
		return refused("missing/incomplete semantic context inventory")
	}
	sets := []struct {
		root  string
		files []fileHash
		allow func(string) bool
	}{{"Assets", c.AssetsCode, authoredContextPath}, {"ProjectSettings", c.ProjectSettings, settingsContextPath}}
	governor := tenancy.NewRowGovernor(ctx, tenancy.ClassTree)
	total := alreadyRead
	for _, set := range sets {
		paths := make([]string, 0, len(set.files))
		last := ""
		for _, file := range set.files {
			if err := governor.Row(); err != nil {
				return err
			}
			if !set.allow(file.Path) || file.Path <= last || !shaPattern.MatchString(file.SHA256) {
				return refused("duplicate/unsafe/unsorted semantic input")
			}
			last = file.Path
			paths = append(paths, file.Path)
			raw, err := readHash(ctx, projectRoot, file, inputLimit)
			if err != nil {
				return err
			}
			total += len(raw)
			if total > sourceLimit {
				return refused("combined asset/tool/context source closure exceeds64MiB")
			}
		}
		actual, err := contextMembers(ctx, projectRoot, set.root)
		if err != nil {
			return err
		}
		if !slices.Equal(actual, paths) {
			return refused("stale semantic context membership")
		}
	}
	return nil
}
