package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/geoffmilleraz/fathom/internal/iacfacts"
)

// pinFile is calibration/iac/pins.yaml: provider, major line, release and archive sha256.
type pinFile struct {
	Registry  string        `yaml:"registry"`
	Platform  string        `yaml:"platform"`
	Providers []pinProvider `yaml:"providers"`
}

type pinProvider struct {
	Source  string    `yaml:"source"`
	Licence string    `yaml:"licence"`
	Lines   []pinLine `yaml:"lines"`
}

type pinLine struct {
	Line     int          `yaml:"line"`
	Releases []pinRelease `yaml:"releases"`
	// Source, Licence and the file's registry and platform are filled in by selectLines, so a line carries
	// everything a build needs.
	Source   string `yaml:"-"`
	Licence  string `yaml:"-"`
	Registry string `yaml:"-"`
	Platform string `yaml:"-"`
}

type pinRelease struct {
	Version string `yaml:"version"`
	SHA256  string `yaml:"sha256"`
}

var sha256RE = regexp.MustCompile(`^[0-9a-f]{64}$`)

// loadPins reads and validates a pin list.
func loadPins(path string) (*pinFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var pf pinFile
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(&pf); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &pf, pf.validate()
}

func (pf *pinFile) validate() error {
	if pf.Registry == "" || pf.Platform == "" {
		return fmt.Errorf("pins: registry and platform are required")
	}
	seen := map[string]bool{}
	for _, p := range pf.Providers {
		if strings.Count(p.Source, "/") != 1 {
			return fmt.Errorf("pins: source %q is not namespace/name", p.Source)
		}
		for _, l := range p.Lines {
			key := p.Source + "@" + strconv.Itoa(l.Line)
			if seen[key] {
				return fmt.Errorf("pins: %s appears twice", key)
			}
			seen[key] = true
			if len(l.Releases) == 0 {
				return fmt.Errorf("pins: %s pins no release", key)
			}
			var prev iacfacts.IaCVersion
			for i, r := range l.Releases {
				v, err := iacfacts.ParseIaCVersion(r.Version)
				if err != nil || v.String() != r.Version {
					return fmt.Errorf("pins: %s release %q is not MAJOR.MINOR.PATCH", key, r.Version)
				}
				if v.Pre != "" {
					return fmt.Errorf("pins: %s release %s is a pre-release; a catalog holds stable releases", key, r.Version)
				}
				if v.Major != l.Line {
					return fmt.Errorf("pins: %s release %s is outside line %d", key, r.Version, l.Line)
				}
				if !sha256RE.MatchString(r.SHA256) {
					return fmt.Errorf("pins: %s release %s needs a 64 hex digit sha256", key, r.Version)
				}
				if i > 0 && iacfacts.CompareIaCVersions(prev, v) >= 0 {
					return fmt.Errorf("pins: %s releases must ascend and not repeat, at %s", key, r.Version)
				}
				prev = v
			}
		}
	}
	return nil
}

// selectLines returns the lines matching source (empty for all) and line (zero for all), in pin-file order.
func (pf *pinFile) selectLines(source string, line int) []pinLine {
	var out []pinLine
	for _, p := range pf.Providers {
		if source != "" && p.Source != source {
			continue
		}
		for _, l := range p.Lines {
			if line != 0 && l.Line != line {
				continue
			}
			l.Source, l.Licence, l.Registry, l.Platform = p.Source, p.Licence, pf.Registry, pf.Platform
			out = append(out, l)
		}
	}
	return out
}
