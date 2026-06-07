// Package config handles sextant's runtime configuration and named profiles.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Well-known profile names.
const (
	ProfileCoding        = "coding"
	ProfileInvestigation = "investigation"
	ProfileCI            = "ci"
	ProfilePlatform      = "platform"
)

// ProfileCfg holds per-profile settings.
type ProfileCfg struct {
	// MaxResults caps the number of query results returned (0 = no cap).
	MaxResults int `json:"maxResults"`
	// SemanticWeight is the default blending weight for semantic search
	// (0.0 = keyword-only, 1.0 = semantic-only).
	SemanticWeight float64 `json:"semanticWeight"`
	// EnableCostTracking turns on per-tool cost accumulation when true.
	EnableCostTracking bool `json:"enableCostTracking"`
}

// Config is the top-level runtime configuration.
type Config struct {
	// DefaultProfile selects the active profile when none is specified.
	DefaultProfile string `json:"defaultProfile"`
	// StoreDir is the local path used as the on-disk store root.
	StoreDir string `json:"storeDir"`
	// Profiles is the map of named profile configurations.
	Profiles map[string]ProfileCfg `json:"profiles"`
}

// defaults returns a Config pre-populated with sensible baseline values.
func defaults() Config {
	home, _ := os.UserHomeDir()
	return Config{
		DefaultProfile: ProfileCoding,
		StoreDir:       filepath.Join(home, ".sextant", "store"),
		Profiles: map[string]ProfileCfg{
			ProfileCoding: {
				MaxResults:         20,
				SemanticWeight:     0.3,
				EnableCostTracking: false,
			},
			ProfileInvestigation: {
				MaxResults:         50,
				SemanticWeight:     0.5,
				EnableCostTracking: true,
			},
			ProfileCI: {
				MaxResults:         10,
				SemanticWeight:     0.0,
				EnableCostTracking: true,
			},
			ProfilePlatform: {
				MaxResults:         30,
				SemanticWeight:     0.4,
				EnableCostTracking: true,
			},
		},
	}
}

// Load parses jsonBytes into a Config, overlaying values onto the built-in
// defaults so any omitted field keeps its default value.
func Load(jsonBytes []byte) (Config, error) {
	cfg := defaults()
	if len(jsonBytes) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(jsonBytes, &cfg); err != nil {
		return Config{}, fmt.Errorf("config: parse error: %w", err)
	}
	// Ensure Profiles map is never nil after decode.
	if cfg.Profiles == nil {
		cfg.Profiles = defaults().Profiles
	}
	return cfg, nil
}

// Validate checks cfg for semantic errors that json.Unmarshal cannot catch.
func (c *Config) Validate() error {
	if c.StoreDir == "" {
		return errors.New("config: storeDir must not be empty")
	}
	if c.DefaultProfile == "" {
		return errors.New("config: defaultProfile must not be empty")
	}
	if _, ok := c.Profiles[c.DefaultProfile]; !ok {
		return fmt.Errorf("config: defaultProfile %q not found in profiles", c.DefaultProfile)
	}
	for name, p := range c.Profiles {
		if p.SemanticWeight < 0 || p.SemanticWeight > 1 {
			return fmt.Errorf("config: profile %q semanticWeight %v out of range [0,1]", name, p.SemanticWeight)
		}
		if p.MaxResults < 0 {
			return fmt.Errorf("config: profile %q maxResults must be >= 0", name)
		}
	}
	return nil
}

// Profile returns the ProfileCfg for the given name plus a boolean indicating
// whether it was found.
func (c *Config) Profile(name string) (ProfileCfg, bool) {
	p, ok := c.Profiles[name]
	return p, ok
}
