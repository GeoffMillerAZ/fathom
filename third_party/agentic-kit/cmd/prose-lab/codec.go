package main

import (
	"errors"
	"io"
	"os"
	"regexp"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
)

var labIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,95}$`)
var modelIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,255}$`)

func validLabID(s string) bool { return labIDPattern.MatchString(s) }

func decodeLabJSON(b []byte, v any) error { return jsonwire.DecodeStrict(b, v) }

func readStudyJSON(path string, v *canaryStudy) ([]byte, error) {
	b, err := readLabBytes(path, 1<<20)
	if err != nil {
		return nil, err
	}
	if err = decodeLabJSON(b, v); err != nil {
		return nil, err
	}
	// The shared decoder checks map shape, not each map value's required
	// fields. Pricing data must remain explicit even when its zero is valid.
	obj, err := jsonwire.Object(b)
	if err != nil {
		return nil, err
	}
	lanes, err := jsonwire.Object(obj["lanes"])
	if err != nil {
		return nil, err
	}
	for _, raw := range lanes {
		var l lane
		if err := decodeLabJSON(raw, &l); err != nil {
			return nil, err
		}
	}
	return b, nil
}

func readLabBytes(path string, max int64) ([]byte, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > max {
		return nil, errors.New("study input must be a bounded regular non-symlink file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(st, opened) {
		return nil, errors.New("study input changed during acquisition")
	}
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, errors.New("study byte bound")
	}
	return b, nil
}
