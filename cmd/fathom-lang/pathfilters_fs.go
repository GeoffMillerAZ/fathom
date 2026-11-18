package main

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
)

// Use the same checkout view for discovery and the dependency reader's
// resolution probes. Hiding VCS entries only during discovery would still
// allow an import or an embed to read administrative files.
type pathFilterFS struct{ root *os.Root }

func (view pathFilterFS) Open(name string) (fs.File, error) {
	if pathFilterVCSPath(name) {
		return nil, fs.ErrNotExist
	}
	file, err := view.root.FS().Open(name)
	if dir, ok := file.(fs.ReadDirFile); ok && err == nil {
		return pathFilterDir{dir}, nil
	}
	return file, err
}

func (view pathFilterFS) Lstat(name string) (fs.FileInfo, error) {
	if pathFilterVCSPath(name) {
		return nil, fs.ErrNotExist
	}
	return view.root.Lstat(name)
}

type pathFilterDir struct{ fs.ReadDirFile }

func (dir pathFilterDir) ReadDir(n int) ([]fs.DirEntry, error) {
	for {
		entries, err := dir.ReadDirFile.ReadDir(n)
		entries = slices.DeleteFunc(entries, func(entry fs.DirEntry) bool {
			return pathFilterVCSPath(entry.Name())
		})
		if len(entries) > 0 || err != nil || n <= 0 {
			return entries, err
		}
	}
}

func inventoryPathFilterFiles(ctx context.Context, view fs.FS) ([]string, error) {
	var inventory []string
	var readErr error
	visited := 0
	err := fs.WalkDir(view, ".", func(name string, entry fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		visited++
		if visited > maxWalk {
			return errors.New("pathfilters inventory exceeds the walk limit")
		}
		if err != nil {
			readErr = errors.New("cannot read some of the pathfilters inventory")
			return nil
		}
		if entry.Type().IsRegular() {
			inventory = append(inventory, name)
		}
		return nil
	})
	slices.Sort(inventory)
	return inventory, errors.Join(readErr, err)
}

func pathFilterInventoryHas(inventory []string, name string) bool {
	_, ok := slices.BinarySearch(inventory, name)
	return ok
}

var errPathFilterSize = errors.New("file exceeds limit")

func readPathFilterFile(repo *os.Root, name string, limit int64) ([]byte, error) {
	for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
		info, err := repo.Lstat(parent)
		if err != nil || !info.IsDir() {
			return nil, errors.New("expected a real checkout directory")
		}
	}
	info, err := repo.Lstat(name)
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("expected regular checkout file")
	}
	if info.Size() > limit {
		return nil, errPathFilterSize
	}
	file, err := repo.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	src, err := io.ReadAll(io.LimitReader(file, limit+1))
	if int64(len(src)) > limit {
		return nil, errPathFilterSize
	}
	return src, err
}
