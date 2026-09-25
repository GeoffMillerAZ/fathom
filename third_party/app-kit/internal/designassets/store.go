package designassets

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func openCache(name string, create bool) (*os.Root, error) {
	if name == "" {
		return nil, fmt.Errorf("designassets: cache root is required")
	}
	// Lstat("link/") and Lstat("link/.") follow the final symlink on some
	// platforms. Normalize aliases before applying the no-symlink boundary.
	name = filepath.Clean(name)
	if create {
		if err := os.MkdirAll(name, 0700); err != nil {
			return nil, err
		}
	}
	info, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("designassets: cache root must be a real directory")
	}
	return os.OpenRoot(name)
}

func openRegular(root *os.Root, name string) (*os.File, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("designassets: non-regular cache/input file %q", name)
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("designassets: non-regular file %q", name)
	}
	return f, nil
}

func archiveDirectory(root *os.Root, create bool) (*os.Root, error) {
	if create {
		if err := root.Mkdir("archives", 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	info, err := root.Lstat("archives")
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("designassets: archives must be a real directory")
	}
	return root.OpenRoot("archives")
}

func temporary(root *os.Root) (string, *os.File, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", nil, err
	}
	name := ".import-" + hex.EncodeToString(nonce[:]) + ".tmp"
	f, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	return name, f, err
}

func verifiedArchive(ctx context.Context, root *os.Root, a Archive) (*os.File, error) {
	dir, err := archiveDirectory(root, false)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	f, err := openRegular(dir, a.SHA256+".zip")
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || info.Size() != a.Size {
		f.Close()
		return nil, fmt.Errorf("designassets: archive size mismatch for %s", a.ID)
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(contextReader{ctx, f}, MaxArchiveBytes+1))
	if err != nil || n != a.Size || hex.EncodeToString(h.Sum(nil)) != a.SHA256 {
		f.Close()
		return nil, fmt.Errorf("designassets: archive integrity mismatch for %s: %w", a.ID, errors.Join(err, errors.New("SHA256 or size mismatch")))
	}
	return f, nil
}

// Import validates known official download filenames without trusting their
// names as evidence of authenticity. It preserves all prior immutable revisions.
// ZIPs are never extracted. A process lock refuses concurrent imports; an
// abandoned .import.lock requires explicit operator review/removal before retry.
func Import(ctx context.Context, srcDir, root string) (*Catalog, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	src, err := openCache(srcDir, false)
	if err != nil {
		return nil, fmt.Errorf("designassets: source: %w", err)
	}
	defer src.Close()
	sourceDir, err := src.Open(".")
	if err != nil {
		return nil, err
	}
	entries, err := sourceDir.ReadDir(4097)
	sourceDir.Close()
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(entries) > 4096 {
		return nil, fmt.Errorf("designassets: source directory exceeds entry bound")
	}
	var inputs []string
	for _, entry := range entries {
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".zip") {
			continue
		}
		if _, ok := productFor(entry.Name()); !ok {
			return nil, fmt.Errorf("designassets: unsupported download filename %q", entry.Name())
		}
		inputs = append(inputs, entry.Name())
	}
	if len(inputs) == 0 || len(inputs) > MaxInputArchives {
		return nil, fmt.Errorf("designassets: expected 1..%d supported ZIP downloads", MaxInputArchives)
	}
	slices.Sort(inputs)
	r, err := openCache(root, true)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	lock, err := r.OpenFile(".import.lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("designassets: acquire import lock (another import or abandoned lock): %w", err)
	}
	lock.Close()
	defer r.Remove(".import.lock")
	dir, err := archiveDirectory(r, true)
	if err != nil {
		return nil, err
	}
	dir.Close()
	prior, err := loadCatalog(r)
	if errors.Is(err, os.ErrNotExist) {
		prior = &Catalog{Archives: []Archive{}, archives: []Archive{}}
	} else if err != nil {
		return nil, err
	}
	archives := prior.List()
	for _, a := range archives {
		f, err := verifiedArchive(ctx, r, a)
		if err != nil {
			return nil, err
		}
		if err := verifyMetadata(ctx, f, a); err != nil {
			f.Close()
			return nil, err
		}
		f.Close()
	}
	type staged struct {
		name    string
		archive Archive
	}
	var stagedFiles []staged
	defer func() {
		for _, s := range stagedFiles {
			_ = r.Remove(s.name)
		}
	}()
	compressed, expanded := int64(0), int64(0)
	for _, name := range inputs {
		original, err := openRegular(src, name)
		if err != nil {
			return nil, err
		}
		info, err := original.Stat()
		if err != nil {
			original.Close()
			return nil, err
		}
		if info.Size() <= 0 || info.Size() > MaxArchiveBytes || compressed+info.Size() > MaxImportBytes {
			original.Close()
			return nil, fmt.Errorf("designassets: input archive/batch size exceeds bounds")
		}
		compressed += info.Size()
		tempName, temp, err := temporary(r)
		if err != nil {
			original.Close()
			return nil, err
		}
		stagedFiles = append(stagedFiles, staged{name: tempName})
		h := sha256.New()
		n, copyErr := io.Copy(io.MultiWriter(temp, h), io.LimitReader(contextReader{ctx, original}, MaxArchiveBytes+1))
		original.Close()
		if copyErr != nil || n != info.Size() {
			temp.Close()
			return nil, fmt.Errorf("designassets: source copy changed or failed: %w", errors.Join(copyErr, errors.New("size/copy verification failed")))
		}
		a, total, err := inspect(ctx, temp, n, name, hex.EncodeToString(h.Sum(nil)))
		if err != nil {
			temp.Close()
			return nil, err
		}
		expanded += total
		if expanded > MaxImportExpanded {
			temp.Close()
			return nil, fmt.Errorf("designassets: expanded import exceeds bounds")
		}
		if err := temp.Sync(); err != nil {
			temp.Close()
			return nil, err
		}
		if err := temp.Close(); err != nil {
			return nil, err
		}
		stagedFiles[len(stagedFiles)-1].archive = a
		if _, ok := prior.Get(a.ID); !ok {
			archives = append(archives, a)
		}
	}
	body, err := encodeCatalog(archives)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// No archive or manifest is published until every input passes inspection.
	var published []string
	committed := false
	defer func() {
		if !committed {
			for _, name := range published {
				_ = r.Remove(name)
			}
		}
	}()
	for _, s := range stagedFiles {
		name := "archives/" + s.archive.SHA256 + ".zip"
		if err := r.Link(s.name, name); err != nil {
			if !errors.Is(err, os.ErrExist) {
				return nil, err
			}
			f, err := verifiedArchive(ctx, r, s.archive)
			if err != nil {
				return nil, err
			}
			f.Close()
		} else {
			published = append(published, name)
		}
	}
	manifestName, manifest, err := temporary(r)
	if err != nil {
		return nil, err
	}
	defer r.Remove(manifestName)
	if _, err = manifest.Write(body); err == nil {
		err = manifest.Sync()
	}
	closeErr := manifest.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := r.Rename(manifestName, "catalog.json"); err != nil {
		return nil, err
	}
	committed = true
	return decodeCatalog(body)
}
