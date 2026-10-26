package jsonwire

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
)

var errInvalid = errors.New("invalid JSON wire")

// ValidateStrictJSON checks a complete JSON value for Unicode validity,
// duplicate keys, nulls, excess nesting and trailing values. Callers enforce
// the value's schema separately. It retains no state and is safe for concurrent use.
func ValidateStrictJSON(b []byte) error {
	_, err := scan(b, false)
	return err
}

// DecodeStrict decodes a closed struct wire with exact required JSON names.
// It retains no state and is safe for concurrent use.
func DecodeStrict(b []byte, v any) error {
	t := reflect.TypeOf(v)
	if t == nil || t.Kind() != reflect.Pointer || reflect.ValueOf(v).IsNil() {
		return errInvalid
	}
	if _, err := scan(b, false); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		return errInvalid
	}
	var tree any
	if json.Unmarshal(b, &tree) != nil || required(tree, t.Elem()) != nil {
		return errInvalid
	}
	return nil
}

// Object validates a complete JSON object, including unused native fields,
// and returns its raw members. It retains no state and is safe for concurrent use.
func Object(b []byte) (map[string]json.RawMessage, error) {
	if _, err := scan(b, true); err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(b, &m) != nil || m == nil {
		return nil, errInvalid
	}
	return m, nil
}

func scan(b []byte, allowNull bool) (any, error) {
	if !ValidUnicode(b) {
		return nil, errInvalid
	}
	d := json.NewDecoder(bytes.NewReader(b))
	if err := scanValue(d, 0, allowNull); err != nil {
		return nil, errInvalid
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, errInvalid
	}
	return nil, nil
}

func scanValue(d *json.Decoder, depth int, allowNull bool) error {
	if depth > 24 {
		return errInvalid
	}
	t, err := d.Token()
	if err != nil || t == nil && !allowNull {
		return errInvalid
	}
	switch t {
	case json.Delim('{'):
		seen := map[string]bool{}
		for d.More() {
			k, err := d.Token()
			s, ok := k.(string)
			if err != nil || !ok || seen[s] {
				return errInvalid
			}
			seen[s] = true
			if scanValue(d, depth+1, allowNull) != nil {
				return errInvalid
			}
		}
		if _, err = d.Token(); err != nil {
			return errInvalid
		}
	case json.Delim('['):
		for d.More() {
			if scanValue(d, depth+1, allowNull) != nil {
				return errInvalid
			}
		}
		if _, err = d.Token(); err != nil {
			return errInvalid
		}
	}
	return nil
}

func required(v any, t reflect.Type) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		m, ok := v.(map[string]any)
		if !ok {
			return errInvalid
		}
		allowed := map[string]bool{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := strings.Split(f.Tag.Get("json"), ",")
			if len(tag) == 0 || tag[0] == "-" {
				continue
			}
			allowed[tag[0]] = true
			x, exists := m[tag[0]]
			if !exists {
				if strings.Contains(f.Tag.Get("json"), "omitempty") {
					continue
				}
				return errInvalid
			}
			if required(x, f.Type) != nil {
				return errInvalid
			}
		}
		for k := range m {
			if !allowed[k] {
				return errInvalid
			}
		}
	case reflect.Slice:
		a, ok := v.([]any)
		if !ok {
			return errInvalid
		}
		for _, x := range a {
			if required(x, t.Elem()) != nil {
				return errInvalid
			}
		}
	default:
		if v == nil {
			return errInvalid
		}
	}
	return nil
}
