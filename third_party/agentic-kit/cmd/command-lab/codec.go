package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
)

const maxControl = 1 << 20

func readStrict(path string, target any) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("input_unreadable")
	}
	f, err := os.Open(path)
	if err != nil {
		return errors.New("input_unreadable")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() {
		return errors.New("input_unreadable")
	}
	b, err := io.ReadAll(io.LimitReader(f, maxControl+1))
	if err != nil || len(b) > maxControl {
		return errors.New("input_oversize")
	}
	if !jsonwire.ValidUnicode(b) {
		return errors.New("input_utf8")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	v, err := parse(d, 0)
	if err != nil {
		return err
	}
	if _, err = d.Token(); err != io.EOF {
		return errors.New("input_trailing")
	}
	if err = fields(v, reflect.TypeOf(target).Elem()); err != nil {
		return err
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(target); err != nil {
		return errors.New("input_type")
	}
	return nil
}
func parse(d *json.Decoder, depth int) (any, error) {
	return parseValue(d, depth, false)
}
func parseValue(d *json.Decoder, depth int, allowNull bool) (any, error) {
	if depth > 24 {
		return nil, errors.New("input_depth")
	}
	t, e := d.Token()
	if e != nil {
		return nil, errors.New("input_json")
	}
	if t == nil {
		if allowNull {
			return nil, nil
		}
		return nil, errors.New("input_null")
	}
	switch t {
	case json.Delim('{'):
		m := map[string]any{}
		for d.More() {
			k, e := d.Token()
			if e != nil {
				return nil, errors.New("input_json")
			}
			s, ok := k.(string)
			if !ok {
				return nil, errors.New("input_json")
			}
			if _, exists := m[s]; exists {
				return nil, errors.New("input_duplicate")
			}
			v, e := parseValue(d, depth+1, allowNull)
			if e != nil {
				return nil, e
			}
			m[s] = v
		}
		if _, e = d.Token(); e != nil {
			return nil, errors.New("input_json")
		}
		return m, nil
	case json.Delim('['):
		a := []any{}
		for d.More() {
			v, e := parseValue(d, depth+1, allowNull)
			if e != nil {
				return nil, e
			}
			a = append(a, v)
		}
		if _, e = d.Token(); e != nil {
			return nil, errors.New("input_json")
		}
		return a, nil
	default:
		return t, nil
	}
}
func fields(v any, t reflect.Type) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		m, ok := v.(map[string]any)
		if !ok {
			return errors.New("input_type")
		}
		known := map[string]reflect.Type{}
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
			if name == "-" {
				continue
			}
			known[name] = f.Type
			choiceOnlyZero := t == reflect.TypeOf(decision.Answer{}) && (name == "Score" || name == "Noul")
			if !choiceOnlyZero && (len(tag) == 1 || !strings.Contains(tag[1], "omitempty")) {
				required[name] = true
			}
		}
		for k := range required {
			if _, ok := m[k]; !ok {
				return errors.New("input_missing")
			}
		}
		for k, x := range m {
			ft, ok := known[k]
			if !ok {
				return errors.New("input_unknown")
			}
			if e := fields(x, ft); e != nil {
				return e
			}
		}
	case reflect.Slice:
		a, ok := v.([]any)
		if !ok {
			return errors.New("input_type")
		}
		for _, x := range a {
			if e := fields(x, t.Elem()); e != nil {
				return e
			}
		}
	case reflect.Map:
		m, ok := v.(map[string]any)
		if !ok {
			return errors.New("input_type")
		}
		for _, x := range m {
			if e := fields(x, t.Elem()); e != nil {
				return e
			}
		}
	}
	return nil
}
