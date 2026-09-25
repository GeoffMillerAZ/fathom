package designadapt

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Filesystem assurance only: callers own fixed inventories and must complete
// their source-specific replay before invoking this unexported mechanism.
// No manifest, adapter registry or new certification schema belongs here.
type privateDeliveryFile struct {
	name string
	body []byte
}

// Inventories are authored by the adapters, never caller paths. Refuse a future
// programmer error before any filesystem access, rather than partially publishing
// a duplicate, nested path or early completion marker.
func validatePrivateDeliveryFiles(files []privateDeliveryFile) error {
	if len(files) == 0 || files[len(files)-1].name != "artifact.json" {
		return fmt.Errorf("private delivery: completion marker must be last")
	}
	seen := make(map[string]bool, len(files))
	for _, item := range files {
		if item.name == "" || item.name == "." || item.name == ".." || strings.ContainsAny(item.name, "/\\\x00") || seen[item.name] {
			return fmt.Errorf("private delivery: invalid fixed inventory")
		}
		seen[item.name] = true
	}
	return nil
}

func writePrivateDelivery(dir string, files []privateDeliveryFile, label string) error {
	if err := validatePrivateDeliveryFiles(files); err != nil {
		return err
	}
	if dir == "" {
		return fmt.Errorf("%s: output path required", label)
	}
	dir = filepath.Clean(dir)
	parent, err := openPrivateDirectory(filepath.Dir(dir))
	if err != nil {
		return fmt.Errorf("%s: existing real parent required: %w", label, err)
	}
	defer parent.Close()
	leaf := filepath.Base(dir)
	// Mkdir is exclusive. Rename-over-an-empty-directory is NOT no-clobber.
	if err := parent.Mkdir(leaf, 0700); err != nil {
		return fmt.Errorf("%s: create new destination: %w", label, err)
	}
	if err := writePrivateFiles(parent, leaf, files); err != nil {
		return fmt.Errorf("%s: incomplete output left at %q; inspect it and choose a new path: %w", label, dir, err)
	}
	return nil
}

func writePrivateFiles(parent *os.Root, leaf string, files []privateDeliveryFile) error {
	info, err := parent.Lstat(leaf)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("new destination is not a real directory")
	}
	root, err := parent.OpenRoot(leaf)
	if err != nil {
		return err
	}
	defer root.Close()
	actual, err := root.Stat(".")
	if err != nil || !os.SameFile(info, actual) {
		return fmt.Errorf("new destination changed during open")
	}
	for _, item := range files {
		f, err := root.OpenFile(item.name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, writeErr := f.Write(item.body)
		syncErr := f.Sync()
		closeErr := f.Close()
		if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
			return err
		}
	}
	return nil
}

func openPrivateDirectory(dir string) (*os.Root, error) {
	info, err := os.Lstat(filepath.Clean(dir))
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("expected real directory")
	}
	root, err := os.OpenRoot(filepath.Clean(dir))
	if err != nil {
		return nil, err
	}
	actual, err := root.Stat(".")
	if err != nil || !os.SameFile(info, actual) {
		root.Close()
		return nil, fmt.Errorf("directory changed during open")
	}
	return root, nil
}

func checkPrivateDelivery(dir string, files []privateDeliveryFile, label string, drift error) error {
	if err := validatePrivateDeliveryFiles(files); err != nil {
		return err
	}
	if dir == "" {
		return fmt.Errorf("%s: check path required", label)
	}
	// Refuse unsupported secure-read platforms before inspecting any contents.
	flags, err := privateDeliveryReadFlags(label)
	if err != nil {
		return err
	}
	info, err := os.Lstat(filepath.Clean(dir))
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: directory missing", drift)
	}
	if err != nil {
		return err
	}
	if info.Mode() != os.ModeDir|0700 {
		return fmt.Errorf("%w: expected private real directory", drift)
	}
	root, err := openPrivateDirectory(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	listing, err := root.Open(".")
	if err != nil {
		return err
	}
	entries, readErr := listing.ReadDir(len(files) + 1)
	closeErr := listing.Close()
	if readErr != nil && readErr != io.EOF {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	if len(entries) != len(files) {
		return fmt.Errorf("%w: missing or extra entries", drift)
	}
	for _, item := range files {
		before, err := root.Lstat(item.name)
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: missing %s", drift, item.name)
		}
		if err != nil {
			return err
		}
		if !privateDeliveryRegular(before) || before.Size() != int64(len(item.body)) || before.Mode() != 0600 {
			return fmt.Errorf("%w: invalid file %s", drift, item.name)
		}
		// NOFOLLOW rejects an in-root symlink swap; NONBLOCK prevents a FIFO swap
		// from hanging before f.Stat can reject it. Reads remain exactly bounded.
		f, err := root.OpenFile(item.name, flags, 0)
		if err != nil {
			return err
		}
		actual, statErr := f.Stat()
		if statErr != nil || !privateDeliveryRegular(actual) || !os.SameFile(before, actual) || actual.Mode() != 0600 || actual.Size() != before.Size() {
			f.Close()
			return fmt.Errorf("%w: file changed during open", drift)
		}
		body, readErr := io.ReadAll(io.LimitReader(f, int64(len(item.body))+1))
		closeErr := f.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return err
		}
		if !bytes.Equal(body, item.body) {
			return fmt.Errorf("%w: modified %s", drift, item.name)
		}
	}
	return nil
}
