package catalogkit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
)

// DecodeSpec accepts one strict UTF-8 JSON value, as exported from CUE. Duplicate
// keys, unknown fields, nulls, trailing values and inputs above 16 MiB refuse.
// Resolve additionally validates graph, identity, grants and resource bounds.
func DecodeSpec(data []byte) (Spec, error) { var s Spec; err := decode(data, &s); return s, err }

// DecodePolicy decodes receiver-owned policy with the same strict rules.
func DecodePolicy(data []byte) (Policy, error) { var p Policy; err := decode(data, &p); return p, err }

// DecodeProfile accepts receiver-owned named selection under the strict JSON rules.
func DecodeProfile(data []byte) (Profile, error) {
	var p Profile
	err := decode(data, &p)
	return p, err
}

// DecodeDecisionIdentity accepts provider/model/adapter/policy identity under the
// strict JSON rules. DecisionCacheKey additionally checks all fields are nonempty.
func DecodeDecisionIdentity(data []byte) (DecisionIdentity, error) {
	var id DecisionIdentity
	err := decode(data, &id)
	return id, err
}

// DecodeBundle accepts one strict bundle and verifies its recorded digest.
func DecodeBundle(data []byte) (Bundle, error) {
	var b Bundle
	if err := decode(data, &b); err != nil {
		return Bundle{}, err
	}
	if err := VerifyBundle(b); err != nil {
		return Bundle{}, err
	}
	return b, nil
}

// DecodeBinding decodes a strict, provider-neutral criterion binding.
func DecodeBinding(data []byte) (Binding, error) {
	var b Binding
	err := decode(data, &b)
	return b, err
}

// DecodeEvidence decodes a strict list of collector-captured witnesses.
func DecodeEvidence(data []byte) ([]Evidence, error) {
	var e []Evidence
	err := decode(data, &e)
	return e, err
}

func decode(data []byte, out any) error {
	if len(data) > DefaultLimits().MaxBytes || !jsonwire.ValidUnicode(data) {
		return fmt.Errorf("catalogkit: invalid UTF-8 or input byte limit exceeded")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := uniqueValue(d, 0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("catalogkit: trailing JSON or malformed input")
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return fmt.Errorf("catalogkit: decode: %w", err)
	}
	return nil
}

func uniqueValue(d *json.Decoder, depth int) error {
	if depth > 1024 {
		return fmt.Errorf("catalogkit: JSON nesting limit exceeded")
	}
	v, err := d.Token()
	if err != nil {
		return err
	}
	if v == nil {
		return fmt.Errorf("catalogkit: null is not an authored value")
	}
	if delim, ok := v.(json.Delim); ok {
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				k, err := d.Token()
				if err != nil {
					return err
				}
				key, ok := k.(string)
				if !ok || seen[key] {
					return fmt.Errorf("catalogkit: duplicate or invalid JSON key %v", k)
				}
				seen[key] = true
				if err := uniqueValue(d, depth+1); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err := uniqueValue(d, depth+1); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("catalogkit: unexpected JSON delimiter")
		}
		_, err = d.Token()
		return err
	}
	return nil
}
