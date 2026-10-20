package techpack

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/pathglob"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// Section names, in reading order.
const (
	SectionGuards    = "guards"
	SectionGenerated = "generated"
	SectionFormats   = "formats"
)

// SectionReading is the harness reading of one pack section.
type SectionReading struct {
	Section  string   `json:"section"`
	Run      int      `json:"run"`
	Passed   int      `json:"passed"`
	Failed   int      `json:"failed"`
	Failures []string `json:"failures,omitempty"`
	// Withheld: the section has no fixtures or a failing one, so it is
	// never served.
	Withheld bool `json:"withheld"`
	// NoFixtures: the section carries entries but no fixture covers any.
	NoFixtures bool `json:"no_fixtures,omitempty"`
}

// Reading is the per-section reading of one pack. Only sections the pack
// carries appear.
type Reading struct {
	Pack     string           `json:"pack"`
	Version  int              `json:"version"`
	Sections []SectionReading `json:"sections"`
}

// OK reports whether every present section passes.
func (r Reading) OK() bool {
	for _, s := range r.Sections {
		if s.Withheld {
			return false
		}
	}
	return true
}

// Lines renders the reading, one line per section.
func (r Reading) Lines() []string {
	out := []string{fmt.Sprintf("pack %s@%d", r.Pack, r.Version)}
	for _, s := range r.Sections {
		switch {
		case s.NoFixtures:
			out = append(out, fmt.Sprintf("withheld: %s (no fixtures)", s.Section))
		case s.Withheld:
			out = append(out, fmt.Sprintf("withheld: %s (%d of %d failed)", s.Section, s.Failed, s.Run))
			for _, f := range s.Failures {
				out = append(out, "  failing: "+f)
			}
		default:
			out = append(out, fmt.Sprintf("served: %s (%d of %d passed)", s.Section, s.Passed, s.Run))
		}
	}
	return out
}

type packData struct {
	Pack struct {
		Name    string `json:"name"`
		Version int    `json:"version"`
	} `json:"pack"`
	Guards []struct {
		Scopes []string `json:"scopes"`
	} `json:"guards"`
	Generated []struct {
		Output string   `json:"output"`
		Source []string `json:"source"`
	} `json:"generated"`
	Formats []struct {
		Dialect string   `json:"dialect"`
		Globs   []string `json:"globs"`
	} `json:"formats"`
}

type fixtureData struct {
	Guards []struct {
		Guard   int      `json:"guard"`
		Match   []string `json:"match"`
		NoMatch []string `json:"nomatch"`
	} `json:"guards"`
	Generated []struct {
		Output string   `json:"output"`
		Source []string `json:"source"`
	} `json:"generated"`
	Formats []struct {
		Path    string `json:"path"`
		Dialect string `json:"dialect"`
	} `json:"formats"`
}

// RunFixtures reads dir/pack.cue and dir/fixtures/*.cue as pure data and
// returns the per-section reading. Fixtures execute nothing. An error means
// the pack is unreadable or invalid.
func RunFixtures(dir string) (Reading, error) {
	src, err := os.ReadFile(filepath.Join(dir, "pack.cue"))
	if err != nil {
		return Reading{}, fmt.Errorf("read pack: %w", err)
	}
	if err := projectdir.ValidatePack(src, filepath.Join(dir, "pack.cue")); err != nil {
		return Reading{}, err
	}
	var pd packData
	if err := decode(src, "pack.cue", &pd); err != nil {
		return Reading{}, err
	}
	var fx fixtureData
	names, err := filepath.Glob(filepath.Join(dir, "fixtures", "*.cue"))
	if err != nil {
		return Reading{}, err
	}
	sort.Strings(names)
	for _, n := range names {
		b, err := os.ReadFile(n)
		if err != nil {
			return Reading{}, fmt.Errorf("read fixtures: %w", err)
		}
		var one fixtureData
		if err := decode(b, n, &one); err != nil {
			return Reading{}, err
		}
		fx.Guards = append(fx.Guards, one.Guards...)
		fx.Generated = append(fx.Generated, one.Generated...)
		fx.Formats = append(fx.Formats, one.Formats...)
	}
	r := Reading{Pack: pd.Pack.Name, Version: pd.Pack.Version}

	if len(pd.Guards) > 0 {
		s := SectionReading{Section: SectionGuards}
		covered := make([]bool, len(pd.Guards))
		for _, f := range fx.Guards {
			s.Run++
			name := fmt.Sprintf("guards[%d]", f.Guard)
			if f.Guard < 0 || f.Guard >= len(pd.Guards) {
				s.fail(name + ": no such guard")
				continue
			}
			covered[f.Guard] = true
			scopes := pd.Guards[f.Guard].Scopes
			bad := ""
			for _, p := range f.Match {
				if !pathglob.MatchAny(scopes, p) {
					bad = "must match " + p
					break
				}
			}
			if bad == "" {
				for _, p := range f.NoMatch {
					if pathglob.MatchAny(scopes, p) {
						bad = "must not match " + p
						break
					}
				}
			}
			if bad == "" && (len(f.Match) == 0 || len(f.NoMatch) == 0) {
				bad = "needs a positive and a negative path"
			}
			s.settle(name, bad)
		}
		for i, c := range covered {
			if !c {
				s.fail(fmt.Sprintf("guards[%d]: no fixture", i))
			}
		}
		r.add(s)
	}

	if len(pd.Generated) > 0 {
		s := SectionReading{Section: SectionGenerated}
		covered := make([]bool, len(pd.Generated))
		for _, f := range fx.Generated {
			s.Run++
			bad := "no generated entry binds " + f.Output
			for i, g := range pd.Generated {
				if sources, ok := bindSources(g.Output, g.Source, f.Output); ok && sameSet(sources, f.Source) {
					covered[i] = true
					bad = ""
					break
				}
			}
			s.settle("generated "+f.Output, bad)
		}
		for i, c := range covered {
			if !c {
				s.fail(fmt.Sprintf("generated[%d]: no fixture", i))
			}
		}
		r.add(s)
	}

	if len(pd.Formats) > 0 {
		s := SectionReading{Section: SectionFormats}
		rules := make([]connector.DialectRule, 0, len(pd.Formats))
		for _, f := range pd.Formats {
			rules = append(rules, connector.DialectRule{ID: f.Dialect, Globs: f.Globs})
		}
		seen := map[string]bool{}
		for _, f := range fx.Formats {
			s.Run++
			got := connector.ClassifyDialect(rules, f.Path, nil).Dialect
			bad := ""
			if got != f.Dialect {
				bad = fmt.Sprintf("classifies to %q, want %q", got, f.Dialect)
			}
			seen[f.Dialect] = true
			s.settle("formats "+f.Path, bad)
		}
		for i, f := range pd.Formats {
			if !seen[f.Dialect] {
				s.fail(fmt.Sprintf("formats[%d] %s: no fixture", i, f.Dialect))
			}
		}
		r.add(s)
	}
	return r, nil
}

func (s *SectionReading) settle(name, bad string) {
	if bad == "" {
		s.Passed++
		return
	}
	s.Failed++
	s.Failures = append(s.Failures, name+": "+bad)
}

// fail records an uncovered entry; it does not count as a fixture run.
func (s *SectionReading) fail(msg string) {
	s.Failed++
	s.Failures = append(s.Failures, msg)
}

func (r *Reading) add(s SectionReading) {
	sort.Strings(s.Failures)
	if s.Run == 0 {
		s.NoFixtures = true
		s.Withheld = true
		s.Failed, s.Failures = 0, nil
	} else if s.Failed > 0 {
		s.Withheld = true
	}
	r.Sections = append(r.Sections, s)
}

func decode(src []byte, name string, into any) error {
	v := cuecontext.New().CompileBytes(src, cue.Filename(name))
	if err := v.Err(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if err := v.Decode(into); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

var placeholder = regexp.MustCompile(`\{[a-z_]+\}`)

// bindSources binds the output template's placeholders against path and
// fills the source templates. {dir} spans separators, other placeholders do
// not.
func bindSources(outTmpl string, srcTmpls []string, path string) ([]string, bool) {
	var re strings.Builder
	re.WriteString("^")
	names := []string{}
	last := 0
	for _, loc := range placeholder.FindAllStringIndex(outTmpl, -1) {
		re.WriteString(regexp.QuoteMeta(outTmpl[last:loc[0]]))
		name := outTmpl[loc[0]+1 : loc[1]-1]
		names = append(names, name)
		if name == "dir" {
			re.WriteString("(.+)")
		} else {
			re.WriteString("([^/]+)")
		}
		last = loc[1]
	}
	re.WriteString(regexp.QuoteMeta(outTmpl[last:]) + "$")
	m := regexp.MustCompile(re.String()).FindStringSubmatch(path)
	if m == nil {
		return nil, false
	}
	bound := map[string]string{}
	for i, n := range names {
		if prev, ok := bound[n]; ok && prev != m[i+1] {
			return nil, false
		}
		bound[n] = m[i+1]
	}
	out := make([]string, 0, len(srcTmpls))
	for _, t := range srcTmpls {
		out = append(out, placeholder.ReplaceAllStringFunc(t, func(p string) string { return bound[p[1:len(p)-1]] }))
	}
	return out, true
}

func sameSet(a, b []string) bool {
	x, y := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	return strings.Join(x, "\x00") == strings.Join(y, "\x00")
}
