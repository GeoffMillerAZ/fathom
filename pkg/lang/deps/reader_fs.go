package deps

import (
	"errors"
	"io/fs"
	"path"
	"strings"
)

// repositoryFS checks path components before delegating reads. The supplied
// root and the stability and accuracy of its entries are caller contracts.
type repositoryFS struct {
	view       fs.FS
	tsPackages *tsjsPackageInventory
}

func (r repositoryFS) Open(name string) (fs.File, error) {
	if _, err := r.Stat(name); err != nil {
		return nil, err
	}
	return r.view.Open(name)
}

func (r repositoryFS) Stat(name string) (fs.FileInfo, error) {
	if name == "." {
		return fs.Stat(r.view, name)
	}
	if !validPath(name) {
		return nil, fs.ErrInvalid
	}
	parent := "."
	for remaining := name; ; {
		part, rest, more := strings.Cut(remaining, "/")
		info, err := r.component(parent, part)
		if err != nil {
			return nil, err
		}
		mode := info.Mode().Type()
		if mode != fs.ModeDir && (more || mode != 0) {
			return nil, fs.ErrInvalid
		}
		if !more {
			return info, nil
		}
		parent, remaining = path.Join(parent, part), rest
	}
}

func (r repositoryFS) component(parent, part string) (fs.FileInfo, error) {
	// fs.Sub can expose Lstat even when the underlying FS only supports
	// following Stat. For subtree wrappers, inspect the parent entries.
	_, sub := r.view.(fs.SubFS)
	view, hasLstat := r.view.(interface {
		Lstat(string) (fs.FileInfo, error)
	})
	if hasLstat && !sub {
		info, err := view.Lstat(path.Join(parent, part))
		if !errors.Is(err, errors.ErrUnsupported) {
			return info, err
		}
	}
	entries, err := fs.ReadDir(r.view, parent)
	if err != nil {
		return nil, fs.ErrInvalid
	}
	for _, entry := range entries {
		if entry.Name() == part {
			if entry.Type()&fs.ModeSymlink != 0 {
				return nil, fs.ErrInvalid
			}
			info, err := entry.Info()
			if err != nil {
				return nil, fs.ErrInvalid
			}
			return info, nil
		}
	}
	return nil, fs.ErrNotExist
}
