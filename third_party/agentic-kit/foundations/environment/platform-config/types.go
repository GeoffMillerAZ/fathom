// Package platformconfig loads and validates runtime platform configuration.
// Configuration is supplied as JSON and decoded into a Config struct.
// No third-party dependencies are used.
//
// CUE is authoritative for the shape of Config; Go is authoritative for
// Load and Validate behaviour.
//
// Concurrency contract: Config is a plain value type; a loaded Config is safe
// to read concurrently. Load and Validate are pure functions with no shared
// mutable state and may be called from multiple goroutines simultaneously.
package platformconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Config holds the platform-level configuration for a single environment
// context. It mirrors #Config in schema.cue.
type Config struct {
	// Environment is the name of the target environment (e.g. "production").
	// Must be non-empty.
	Environment string `json:"Environment"`

	// Settings is an arbitrary map of string key/value pairs consumed by
	// platform components. Must be non-nil; individual values may be empty.
	Settings map[string]string `json:"Settings"`
}

// Validate enforces the same constraints that #Config encodes in schema.cue.
// It returns a non-nil error when any constraint is violated.
func (c Config) Validate() error {
	var errs []string

	if strings.TrimSpace(c.Environment) == "" {
		errs = append(errs, "Environment must be non-empty")
	}
	if c.Settings == nil {
		errs = append(errs, "Settings must be non-nil")
	}
	for k := range c.Settings {
		if strings.TrimSpace(k) == "" {
			errs = append(errs, "Settings keys must be non-empty")
			break
		}
	}

	if len(errs) > 0 {
		return errors.New("Config: " + strings.Join(errs, "; "))
	}
	return nil
}

// Load decodes jsonBytes into a Config and validates the result.
// It returns a populated Config and a nil error on success, or the zero
// value and a descriptive error on failure.
func Load(jsonBytes []byte) (Config, error) {
	if len(jsonBytes) == 0 {
		return Config{}, errors.New("Load: input must not be empty")
	}

	var c Config
	if err := json.Unmarshal(jsonBytes, &c); err != nil {
		return Config{}, fmt.Errorf("Load: JSON decode: %w", err)
	}

	if err := c.Validate(); err != nil {
		return Config{}, fmt.Errorf("Load: %w", err)
	}

	return c, nil
}
