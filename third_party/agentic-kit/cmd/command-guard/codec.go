package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
)

func strictFile(path string, v any) error {
	st, e := os.Lstat(path)
	if e != nil || !st.Mode().IsRegular() {
		return errors.New("input")
	}
	f, e := os.OpenFile(path, os.O_RDONLY|nonblock, 0)
	if e != nil {
		return errors.New("input")
	}
	defer f.Close()
	st, e = f.Stat()
	if e != nil || !st.Mode().IsRegular() {
		return errors.New("input")
	}
	b, e := io.ReadAll(io.LimitReader(f, 1<<20+1))
	if e != nil || len(b) > 1<<20 || !jsonwire.ValidUnicode(b) {
		return errors.New("input")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	if e = validateValue(d, 0); e != nil {
		return e
	}
	if _, e = d.Token(); e != io.EOF {
		return errors.New("input")
	}
	if _, ok := v.(*request); ok {
		var m map[string]json.RawMessage
		if json.Unmarshal(b, &m) != nil {
			return errors.New("input")
		}
		var kind string
		if json.Unmarshal(m["kind"], &kind) != nil {
			return errors.New("input")
		}
		if kind == "read-range" {
			if _, present := m["literal"]; present {
				return errors.New("input")
			}
		} else if kind == "search-literal" {
			if _, present := m["startLine"]; present {
				return errors.New("input")
			}
			if _, present := m["endLine"]; present {
				return errors.New("input")
			}
		}
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e = d.Decode(v); e != nil {
		return errors.New("input")
	}
	if q, ok := v.(*gitRequest); ok && !validateGitRequest(*q, b) {
		return errors.New("input")
	}
	if e := required(b, reflect.TypeOf(v).Elem()); e != nil {
		return e
	}
	if p, ok := v.(*policy); ok {
		sum := sha256.Sum256(b)
		p.sourceSHA256 = hex.EncodeToString(sum[:])
	}
	return nil
}
func validateValue(d *json.Decoder, depth int) error {
	if depth > 24 {
		return errors.New("input")
	}
	t, e := d.Token()
	if e != nil || t == nil {
		return errors.New("input")
	}
	switch t {
	case json.Delim('{'):
		seen := map[string]bool{}
		for d.More() {
			k, e := d.Token()
			if e != nil {
				return errors.New("input")
			}
			s, ok := k.(string)
			if !ok || seen[s] {
				return errors.New("input")
			}
			seen[s] = true
			if e = validateValue(d, depth+1); e != nil {
				return e
			}
		}
		_, e = d.Token()
		return e
	case json.Delim('['):
		for d.More() {
			if e = validateValue(d, depth+1); e != nil {
				return e
			}
		}
		_, e = d.Token()
		return e
	default:
		return nil
	}
}
func required(b []byte, t reflect.Type) error {
	var raw any
	if json.Unmarshal(b, &raw) != nil {
		return errors.New("input")
	}
	return checkRequired(raw, t)
}
func checkRequired(v any, t reflect.Type) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		m, ok := v.(map[string]any)
		if !ok {
			return errors.New("input")
		}
		known := map[string]reflect.Type{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			parts := strings.Split(f.Tag.Get("json"), ",")
			name := parts[0]
			if name == "" {
				name = f.Name
			}
			if name == "-" {
				continue
			}
			known[name] = f.Type
			if len(parts) == 1 || !strings.Contains(parts[1], "omitempty") {
				if _, ok = m[name]; !ok {
					return errors.New("input")
				}
			}
		}
		for k, x := range m {
			ft, ok := known[k]
			if !ok {
				return errors.New("input")
			}
			if e := checkRequired(x, ft); e != nil {
				return e
			}
		}
	case reflect.Slice:
		a, ok := v.([]any)
		if !ok {
			return errors.New("input")
		}
		for _, x := range a {
			if e := checkRequired(x, t.Elem()); e != nil {
				return e
			}
		}
	}
	return nil
}
