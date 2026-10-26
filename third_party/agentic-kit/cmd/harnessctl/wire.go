package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
)

var errSource = errors.New("source")
var errPolicy = errors.New("policy")
var errRegistration = errors.New("registration")
var errProject = errors.New("project")
var errClock = errors.New("clock")
var errUnavailable = errors.New("unavailable")
var errChanged = errors.New("changed")
var errEnvelope = errors.New("envelope")
var errSignature = errors.New("signature")
var errBinding = errors.New("binding")
var errPreference = errors.New("preference")

func strictJSON(b []byte, v any) error {
	if jsonwire.DecodeStrict(b, v) != nil {
		return errSource
	}
	return nil
}

type snapshot struct {
	data []byte
	info os.FileInfo
	path string
}

func canonical(path string) bool {
	if len(path) > 4096 || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return false
	}
	for p := path; ; p = filepath.Dir(p) {
		st, e := os.Lstat(p)
		if e != nil || st.Mode()&os.ModeSymlink != 0 {
			return false
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return true
}
func within(path, root string) bool {
	rootInfo, e := os.Stat(root)
	if e != nil {
		return true
	}
	for p := path; ; p = filepath.Dir(p) {
		st, e := os.Stat(p)
		if e != nil {
			return true
		}
		if os.SameFile(rootInfo, st) {
			return true
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return false
}
func readSnapshot(path string, limit int) (snapshot, error) {
	if !canonical(path) {
		return snapshot{}, errSource
	}
	before, e := os.Lstat(path)
	if e != nil || !before.Mode().IsRegular() {
		return snapshot{}, errSource
	}
	f, e := os.OpenFile(path, os.O_RDONLY|nonblock, 0)
	if e != nil {
		return snapshot{}, errSource
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || !os.SameFile(before, st) {
		return snapshot{}, errSource
	}
	b, e := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if e != nil || len(b) > limit {
		return snapshot{}, errSource
	}
	return snapshot{b, st, path}, nil
}
