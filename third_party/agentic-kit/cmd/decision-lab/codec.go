package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"unicode/utf8"
)

const maxJSON = 1 << 20

func digest(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }

func readJSON(path string, target any) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxJSON+1))
	if err != nil || len(b) > maxJSON {
		return "", errors.New("input exceeds 1 MiB")
	}
	if !utf8.Valid(b) {
		return "", errors.New("invalid UTF-8")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	v, err := parseValue(dec, 0)
	if err != nil {
		return "", err
	}
	if _, err := dec.Token(); err != io.EOF {
		return "", errors.New("trailing data")
	}
	if err := exactFields(v, reflect.TypeOf(target).Elem()); err != nil {
		return "", err
	}
	dec = json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return "", errors.New("invalid field type")
	}
	return digest(b), nil
}

func parseValue(d *json.Decoder, depth int) (any, error) {
	if depth > 24 {
		return nil, errors.New("nesting exceeds 24")
	}
	t, err := d.Token()
	if err != nil {
		return nil, errors.New("malformed JSON")
	}
	if t == nil {
		return nil, errors.New("null is not allowed")
	}
	switch t {
	case json.Delim('{'):
		m := map[string]any{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return nil, errors.New("malformed JSON")
			}
			s, ok := k.(string)
			if !ok {
				return nil, errors.New("invalid key")
			}
			if _, exists := m[s]; exists {
				return nil, fmt.Errorf("duplicate field %q", s)
			}
			v, err := parseValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			m[s] = v
		}
		if _, err := d.Token(); err != nil {
			return nil, errors.New("malformed object")
		}
		return m, nil
	case json.Delim('['):
		a := []any{}
		for d.More() {
			v, err := parseValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			a = append(a, v)
		}
		if _, err := d.Token(); err != nil {
			return nil, errors.New("malformed array")
		}
		return a, nil
	default:
		return t, nil
	}
}

func exactFields(v any, t reflect.Type) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		m, ok := v.(map[string]any)
		if !ok {
			return errors.New("expected object")
		}
		fields := map[string]reflect.Type{}
		required := map[string]bool{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			tag := strings.Split(f.Tag.Get("json"), ",")
			name := tag[0]
			if name == "" {
				name = f.Name
			}
			if name != "-" {
				fields[name] = f.Type
				optional := len(tag) > 1 && strings.Contains(tag[1], "omitempty")
				if !optional && (t.PkgPath() == reflect.TypeOf(Pack{}).PkgPath() || t.Name() == "Usage") {
					required[name] = true
				}
			}
		}
		if t.Name() == "Answer" {
			for _, name := range []string{"ID", "Kind", "Choice", "Probabilities", "Confidence"} {
				required[name] = true
			}
		}
		for name := range required {
			if _, ok := m[name]; !ok {
				return fmt.Errorf("missing required field %q", name)
			}
		}
		for k, value := range m {
			ft, ok := fields[k]
			if !ok {
				return fmt.Errorf("unknown or noncanonical field %q", k)
			}
			if err := exactFields(value, ft); err != nil {
				return err
			}
		}
	case reflect.Slice:
		a, ok := v.([]any)
		if !ok {
			return errors.New("expected array")
		}
		for _, x := range a {
			if err := exactFields(x, t.Elem()); err != nil {
				return err
			}
		}
	case reflect.Map:
		m, ok := v.(map[string]any)
		if !ok {
			return errors.New("expected map")
		}
		for _, x := range m {
			if err := exactFields(x, t.Elem()); err != nil {
				return err
			}
		}
	}
	return nil
}

func canonical(v any) []byte { b, _ := json.Marshal(v); return b }
