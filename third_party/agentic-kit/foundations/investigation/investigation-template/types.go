// Package investigationtemplate provides named, reusable investigation
// structures. A Template captures the ordered steps expected for a class of
// investigation (e.g. "gap-analysis", "regression-analysis") so every
// investigation of that class starts from a consistent baseline.
//
// CUE contracts live in schema.cue; this file is the Go counterpart and must
// stay in sync with that schema.
//
// Concurrency contract: Registry is not safe for concurrent use; callers that
// share a Registry across goroutines must guard access with their own
// synchronization. Template is a plain value type safe to read concurrently.
package investigationtemplate

import "fmt"

// Template is a named investigation structure with an ordered list of steps.
//
// CUE mirror: #Template in schema.cue.
type Template struct {
	// Name uniquely identifies this template. Must be non-empty.
	Name string

	// Steps is an ordered list of investigation step labels. Must not be
	// empty; each individual step label must be non-empty.
	Steps []string
}

// Registry is a named store of Template values. The zero value is ready to
// use.
type Registry struct {
	m map[string]Template
}

// Register adds t to the registry. If a template with the same Name already
// exists it is silently overwritten. Returns an error if:
//   - t.Name is empty.
//   - t.Steps is empty.
//   - Any step label in t.Steps is empty.
func (r *Registry) Register(t Template) error {
	if t.Name == "" {
		return fmt.Errorf("investigationtemplate: Name must not be empty")
	}
	if len(t.Steps) == 0 {
		return fmt.Errorf("investigationtemplate: Steps must not be empty")
	}
	for i, s := range t.Steps {
		if s == "" {
			return fmt.Errorf("investigationtemplate: Steps[%d] must not be empty", i)
		}
	}
	if r.m == nil {
		r.m = make(map[string]Template)
	}
	// Store a defensive copy of the steps slice so callers cannot mutate the
	// registered template after the fact.
	steps := make([]string, len(t.Steps))
	copy(steps, t.Steps)
	r.m[t.Name] = Template{Name: t.Name, Steps: steps}
	return nil
}

// Get returns the template with the given name and true, or the zero Template
// and false when not found.
func (r *Registry) Get(name string) (Template, bool) {
	if r.m == nil {
		return Template{}, false
	}
	t, ok := r.m[name]
	return t, ok
}
