package iacfacts

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// catalogSuffix ends every installed catalog's file name.
const catalogSuffix = ".iaccat.zip"

// IaCCatalogInfo describes one installed catalog.
type IaCCatalogInfo struct {
	// Name is the file name in the catalog directory.
	Name     string
	Path     string
	Bytes    int64
	Manifest IaCCatalogManifest
	// SHA256 is the digest of the file, set by ListIaCCatalogs when asked and always by Add and Import.
	SHA256 string
}

// AddIaCCatalog installs the catalog file at src into dir after verifying it whole. src must be a local file: a
// URL is refused, since the product has no download path. An existing catalog of the same provider line is
// replaced atomically. It returns the installed catalog, whose SHA256 the caller prints so the operator can
// check it against `gh attestation verify`.
func AddIaCCatalog(dir, src string) (IaCCatalogInfo, error) {
	if strings.Contains(src, "://") {
		return IaCCatalogInfo{}, fmt.Errorf("iac add takes a local file path, not %q: fetch it with your own tools (gh release download, curl), then pass the path", src)
	}
	st, err := os.Stat(src)
	if err != nil {
		return IaCCatalogInfo{}, err
	}
	if !st.Mode().IsRegular() {
		return IaCCatalogInfo{}, fmt.Errorf("iac add: %s is not a regular file", src)
	}
	if st.Size() > MaxCatalogBytes {
		return IaCCatalogInfo{}, refuse(RefuseCatalogBytes, "%s is %d bytes, over the %d byte bound", src, st.Size(), MaxCatalogBytes)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return IaCCatalogInfo{}, err
	}
	tmp, err := os.CreateTemp(dir, ".add-*")
	if err != nil {
		return IaCCatalogInfo{}, err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	in, err := os.Open(src)
	if err != nil {
		return IaCCatalogInfo{}, err
	}
	// Copy at most cap + 1 bytes: a file that grows past the bound while it is read still refuses.
	n, err := io.Copy(tmp, io.LimitReader(in, MaxCatalogBytes+1))
	in.Close()
	if err != nil {
		return IaCCatalogInfo{}, err
	}
	if n > MaxCatalogBytes {
		return IaCCatalogInfo{}, refuse(RefuseCatalogBytes, "%s is over the %d byte bound", src, MaxCatalogBytes)
	}
	if err := tmp.Close(); err != nil {
		return IaCCatalogInfo{}, err
	}
	return installVerified(dir, tmp.Name())
}

// installVerified verifies the file at path, then moves it into dir under the name its manifest gives.
func installVerified(dir, path string) (IaCCatalogInfo, error) {
	m, err := VerifyIaCCatalog(path)
	if err != nil {
		return IaCCatalogInfo{}, err
	}
	sum, err := fileSHA256(path)
	if err != nil {
		return IaCCatalogInfo{}, err
	}
	name := CatalogFileName(m)
	if !safeCatalogName(name) {
		return IaCCatalogInfo{}, fmt.Errorf("iac catalog: the manifest source %q does not make a safe file name", m.Source)
	}
	final := filepath.Join(dir, name)
	if err := os.Chmod(path, 0o644); err != nil {
		return IaCCatalogInfo{}, err
	}
	if err := os.Rename(path, final); err != nil {
		return IaCCatalogInfo{}, err
	}
	st, err := os.Stat(final)
	if err != nil {
		return IaCCatalogInfo{}, err
	}
	return IaCCatalogInfo{Name: name, Path: final, Bytes: st.Size(), Manifest: m, SHA256: sum}, nil
}

func safeCatalogName(name string) bool {
	if name == "" || name != filepath.Base(name) || strings.HasPrefix(name, ".") {
		return false
	}
	for _, r := range name {
		ok := r == '.' || r == '_' || r == '-' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if !ok {
			return false
		}
	}
	return true
}

// IaCImportInput is one export file and the provider version it came from.
type IaCImportInput struct {
	Version string
	Path    string
}

// ImportIaCExports builds a catalog from an operator's own `tofu providers schema -json` exports and installs it.
// It is how a private provider, or a version no published catalog holds, gets facts 1 to 4 and 7. It carries no
// replacement facts. All inputs must be the same provider and one major line.
func ImportIaCExports(dir string, inputs []IaCImportInput, opts IaCWriterOptions) (IaCCatalogInfo, []IaCExportStats, error) {
	if len(inputs) == 0 {
		return IaCCatalogInfo{}, nil, errors.New("iac import: no export was named")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return IaCCatalogInfo{}, nil, err
	}
	w, err := NewIaCCatalogWriter(opts)
	if err != nil {
		return IaCCatalogInfo{}, nil, err
	}
	defer w.Close()
	var stats []IaCExportStats
	for _, in := range inputs {
		f, err := os.Open(in.Path)
		if err != nil {
			return IaCCatalogInfo{}, nil, err
		}
		st, err := w.AddExport(in.Version, f)
		f.Close()
		if err != nil {
			return IaCCatalogInfo{}, nil, fmt.Errorf("%s: %w", in.Path, err)
		}
		stats = append(stats, st)
	}
	tmp, err := os.CreateTemp(dir, ".import-*")
	if err != nil {
		return IaCCatalogInfo{}, nil, err
	}
	tmp.Close()
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := w.WriteFile(tmp.Name()); err != nil {
		return IaCCatalogInfo{}, nil, err
	}
	info, err := installVerified(dir, tmp.Name())
	return info, stats, err
}

// ListIaCCatalogs reads the manifest of every catalog in dir, sorted by name. A file that is not a readable
// catalog is returned in the second result with its reason and never dropped. withDigest also hashes each file.
func ListIaCCatalogs(dir string, withDigest bool) ([]IaCCatalogInfo, map[string]error) {
	ents, err := os.ReadDir(dir)
	bad := map[string]error{}
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			bad[dir] = err
		}
		return nil, bad
	}
	var out []IaCCatalogInfo
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), catalogSuffix) || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		m, err := ReadIaCCatalogManifest(p)
		if err != nil {
			bad[e.Name()] = err
			continue
		}
		st, err := os.Stat(p)
		if err != nil {
			bad[e.Name()] = err
			continue
		}
		info := IaCCatalogInfo{Name: e.Name(), Path: p, Bytes: st.Size(), Manifest: m}
		if withDigest {
			if info.SHA256, err = fileSHA256(p); err != nil {
				bad[e.Name()] = err
				continue
			}
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, bad
}

// RemoveIaCCatalog deletes one installed catalog by file name. The name may omit the suffix. It never reaches
// outside dir.
func RemoveIaCCatalog(dir, name string) (string, error) {
	if name == "" || name != filepath.Base(name) || strings.HasPrefix(name, ".") {
		return "", fmt.Errorf("iac remove: %q is not a catalog name (see `fathom iac list`)", name)
	}
	if !strings.HasSuffix(name, catalogSuffix) {
		name += catalogSuffix
	}
	p := filepath.Join(dir, name)
	st, err := os.Lstat(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("iac remove: no catalog %q in %s (see `fathom iac list`)", name, dir)
		}
		return "", err
	}
	if !st.Mode().IsRegular() {
		return "", fmt.Errorf("iac remove: %s is not a regular file", p)
	}
	return name, os.Remove(p)
}
