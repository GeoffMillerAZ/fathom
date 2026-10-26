package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

type parameterProtocol struct {
	Protocol string `json:"protocol"`
	Rule     string `json:"rule"`
	Subjects []struct {
		Name    string                      `json:"name"`
		SHA     string                      `json:"sha"`
		Configs map[string][]map[string]any `json:"configs"`
	} `json:"subjects"`
}

// readParameterProtocol binds a fixed, bounded input matrix to a checkout.
// It never derives inputs from oracle results or skips a failed case.
func readParameterProtocol(p, name, sha string) (map[string][]map[string]any, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var protocol parameterProtocol
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err = dec.Decode(&protocol); err != nil {
		return nil, err
	}
	var found map[string][]map[string]any
	for _, s := range protocol.Subjects {
		if s.Name != name {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("duplicate subject %q", name)
		}
		if s.SHA != sha {
			return nil, fmt.Errorf("subject identity mismatch: registered %s, checkout %s", s.SHA, sha)
		}
		found = s.Configs
		for rel, cases := range found {
			if len(cases) == 0 || len(cases) > 256 {
				return nil, fmt.Errorf("%s: cases must number 1..256", rel)
			}
			for _, c := range cases {
				if c == nil {
					return nil, fmt.Errorf("%s: cases must be objects", rel)
				}
				for k, v := range c {
					switch v.(type) {
					case bool, string, float64:
					default:
						return nil, fmt.Errorf("%s: %s must be scalar", rel, k)
					}
				}
			}
		}
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("subject %q has no registered configurations", name)
	}
	return found, nil
}

func mergeCompiled(dst, src *compiled) {
	for name := range src.jobs {
		dst.jobs[name] = true
	}
	for w, js := range src.workflows {
		byName := map[string]map[string]bool{}
		for _, list := range [][]cwjob{dst.workflows[w], js} {
			for _, j := range list {
				if byName[j.name] == nil {
					byName[j.name] = map[string]bool{}
				}
				for _, r := range j.requires {
					byName[j.name][r] = true
				}
			}
		}
		names := make([]string, 0, len(byName))
		for n := range byName {
			names = append(names, n)
		}
		sort.Strings(names)
		dst.workflows[w] = nil
		for _, n := range names {
			j := cwjob{name: n}
			for r := range byName[n] {
				j.requires = append(j.requires, r)
			}
			sort.Strings(j.requires)
			dst.workflows[w] = append(dst.workflows[w], j)
		}
	}
}
