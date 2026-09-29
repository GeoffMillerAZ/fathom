package mcpclient

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func metadataObject(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return len(s) > 1 && s[0] == '{' && json.Valid(raw)
}

// validateMetadataJSON rejects ambiguous keys and excessive nesting on the
// verified path. Unknown extension fields remain allowed by the MCP contract.
func validateMetadataJSON(raw json.RawMessage) error {
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.UseNumber()
	if err := metadataValue(d, 0); err != nil {
		return fmt.Errorf("mcpclient: invalid metadata: %w", err)
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("mcpclient: trailing metadata JSON")
	}
	return nil
}

func metadataValue(d *json.Decoder, depth int) error {
	if depth > 32 {
		return fmt.Errorf("metadata nesting limit")
	}
	v, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := v.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			v, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := v.(string)
			if !ok || seen[strings.ToLower(key)] {
				return fmt.Errorf("duplicate metadata key")
			}
			seen[strings.ToLower(key)] = true
			if err := metadataValue(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := metadataValue(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected delimiter")
	}
	_, err = d.Token()
	return err
}
