// Package recordids checks authored Markdown record identities without
// changing plan grammar or using the index. Cross-plan checks are opt-in.
package recordids

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

type Family struct {
	Name    string   `json:"name"`
	Files   []string `json:"files"`
	Pattern string   `json:"pattern,omitempty"`
	Plans   bool     `json:"plans,omitempty"`
	PerFile bool     `json:"per_file,omitempty"`
}

type Exception struct {
	ID    string   `json:"id"`
	Paths []string `json:"paths"`
}

type Config struct {
	Families    []Family    `json:"families,omitempty"`
	CrossPlan   bool        `json:"cross_plan"`
	Grandfather []Exception `json:"grandfather,omitempty"`
}

type Location struct {
	Path string
	Line int
}
type Duplicate struct {
	Family, ID    string
	First, Second Location
}

func (d Duplicate) String() string {
	return fmt.Sprintf("%s %s: %s:%d and %s:%d", d.Family, d.ID, d.First.Path, d.First.Line, d.Second.Path, d.Second.Line)
}

func Defaults() Config {
	return Config{Families: []Family{
		{Name: "adr", Files: []string{"ADR.md"}, Pattern: `^##\s+(ADR-[0-9]+)\b`},
		{Name: "invariant", Files: []string{"docs/intent/invariants.md"}, Pattern: `^\|\s*(I[0-9]+)\s*\|`},
		{Name: "future", Files: []string{"docs/intent/futures.md", "docs/intent/futures/*.md"}, Pattern: `^#{2,6}\s+(F[0-9]+)\b`},
		{Name: "incident", Files: []string{"docs/intent/incidents.md"}, Pattern: `^(?:#{2,6}\s+|-\s+|\*\*)(INC-[0-9]+)\b`},
		{Name: "plan", Files: []string{"docs/plans/*.md"}, Plans: true, PerFile: true},
	}}
}

// Load overlays the default families; explicit [] disables all families.
func Load(path string) (Config, error) {
	cfg := Defaults()
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func archived(path string) bool {
	return strings.HasPrefix(filepath.ToSlash(filepath.Clean(path)), "docs/plans/archive/")
}

func excepted(cfg Config, id string, a, b Location) bool {
	if a.Path == b.Path {
		return false
	}
	for _, e := range cfg.Grandfather {
		if e.ID == id && slices.Contains(e.Paths, a.Path) && slices.Contains(e.Paths, b.Path) {
			return true
		}
	}
	return false
}

// Check reports duplicates in stable family, path, then line order. Patterns
// use filepath.Glob syntax. Archived plans are always excluded, even if a
// configured pattern names them explicitly. Exceptions only cover named
// cross-plan pairs and never excuse a duplicate within one plan.
func Check(root string, cfg Config) ([]Duplicate, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("repository root is not a directory")
	}
	var out []Duplicate
	planIDs := map[string]map[string]bool{}
	for _, family := range cfg.Families {
		var re *regexp.Regexp
		if !family.Plans {
			var err error
			re, err = regexp.Compile(family.Pattern)
			if err != nil {
				return nil, fmt.Errorf("%s pattern: %w", family.Name, err)
			}
			if re.NumSubexp() != 1 {
				return nil, fmt.Errorf("%s pattern must capture exactly one record id", family.Name)
			}
		}
		var files []string
		for _, pattern := range family.Files {
			if filepath.IsAbs(pattern) || strings.HasPrefix(filepath.Clean(pattern), "..") {
				return nil, fmt.Errorf("%s: file patterns must stay repository-relative", family.Name)
			}
			found, err := filepath.Glob(filepath.Join(root, pattern))
			if err != nil {
				return nil, err
			}
			files = append(files, found...)
		}
		slices.Sort(files)
		files = slices.Compact(files)
		seen := map[string]Location{}
		for _, file := range files {
			rel, err := filepath.Rel(root, file)
			if err != nil {
				return nil, err
			}
			rel = filepath.ToSlash(rel)
			if archived(rel) {
				continue
			}
			b, err := os.ReadFile(file)
			if err != nil {
				return nil, err
			}
			record := func(id string, line int) {
				key := id
				if family.PerFile && !(family.Plans && cfg.CrossPlan) {
					key = rel + "\x00" + id
				}
				at := Location{Path: rel, Line: line}
				if family.Plans {
					if planIDs[id] == nil {
						planIDs[id] = map[string]bool{}
					}
					planIDs[id][rel] = true
				}
				if first, ok := seen[key]; ok {
					// Always check each plan separately, including an excepted plan pair.
					if !family.Plans || !cfg.CrossPlan || !excepted(cfg, id, first, at) {
						out = append(out, Duplicate{Family: family.Name, ID: id, First: first, Second: at})
					}
				} else {
					seen[key] = at
				}
			}
			lines := strings.Split(string(b), "\n")
			fenced := map[int]bool{}
			fence := ""
			for i, line := range lines {
				trim := strings.TrimSpace(line)
				if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
					marker := trim[:3]
					if fence == "" {
						fence = marker
					} else if marker == fence {
						fence = ""
					}
					fenced[i+1] = true
					continue
				}
				if fence != "" {
					fenced[i+1] = true
					continue
				}
				if !family.Plans {
					if m := re.FindStringSubmatch(line); m != nil {
						record(m[1], i+1)
					}
				}
			}
			if family.Plans {
				local := map[string]Location{}
				for _, task := range verify.ParseMarkdownPlan("", b).Tasks {
					if fenced[task.Line] {
						continue
					}
					ann, _, _ := verify.ParseAnnotation(strings.Join(lines[task.Line-1:task.EndLine], " "))
					id := ann["id"]
					if id == "" {
						continue
					}
					at := Location{Path: rel, Line: task.Line}
					if first, ok := local[id]; ok && cfg.CrossPlan {
						out = append(out, Duplicate{Family: family.Name, ID: id, First: first, Second: at})
						continue
					}
					local[id] = at
					record(id, task.Line)
				}
			}
		}
	}
	if cfg.CrossPlan {
		for _, e := range cfg.Grandfather {
			paths := slices.Clone(e.Paths)
			slices.Sort(paths)
			paths = slices.Compact(paths)
			if len(paths) < 2 {
				return nil, fmt.Errorf("stale exception %s: requires at least two distinct plan paths", e.ID)
			}
			for _, path := range paths {
				if !planIDs[e.ID][path] {
					return nil, fmt.Errorf("stale exception %s: %s no longer collides", e.ID, path)
				}
			}
		}
	}
	slices.SortFunc(out, func(a, b Duplicate) int {
		return cmp.Or(strings.Compare(a.Family, b.Family), strings.Compare(a.ID, b.ID),
			strings.Compare(a.First.Path, b.First.Path), cmp.Compare(a.First.Line, b.First.Line),
			strings.Compare(a.Second.Path, b.Second.Path), cmp.Compare(a.Second.Line, b.Second.Line))
	})
	return out, nil
}
