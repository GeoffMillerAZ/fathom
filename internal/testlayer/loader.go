package testlayer

import (
	"fmt"
	"os"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// LayerSpec is one decoded #Layer entry (schema/testlayer/schema.cue).
type LayerSpec struct {
	Name      string
	Globs     []string
	BuildTags []string
	Dirs      []string
}

// LayerSet is the decoded runtime form of a testlayer config file.
type LayerSet struct {
	Layers []LayerSpec
}

// DefaultLayers is the built-in LayerSet LoadLayers falls back to when no
// config file is present, and that Classify uses directly: the
// "integration" and "e2e" directory/build-tag conventions most Go repos
// already follow, needing zero configuration. A project with different
// conventions writes its own schema/testlayer/schema.cue-shaped file and
// calls LoadLayers/ClassifyWith.
func DefaultLayers() LayerSet {
	return LayerSet{Layers: []LayerSpec{
		{Name: "integration", BuildTags: []string{"integration"}, Dirs: []string{"integration"}},
		{Name: "e2e", BuildTags: []string{"e2e"}, Dirs: []string{"e2e"}},
	}}
}

// FieldError names one layer field that failed to load.
type FieldError struct {
	Path  string
	Field string
	Err   error
}

func (e *FieldError) Error() string {
	return fmt.Sprintf("%s: field %q: %v", e.Path, e.Field, e.Err)
}

func (e *FieldError) Unwrap() error { return e.Err }

// LoadError collects every FieldError one LoadLayers call found.
type LoadError struct {
	Path   string
	Fields []*FieldError
}

func (e *LoadError) Error() string {
	msgs := make([]string, 0, len(e.Fields))
	for _, f := range e.Fields {
		msgs = append(msgs, fmt.Sprintf("field %q: %v", f.Field, f.Err))
	}
	return fmt.Sprintf("testlayer: %s: %s", e.Path, strings.Join(msgs, "; "))
}

func fieldErr(path, field string, err error) *FieldError {
	return &FieldError{Path: path, Field: field, Err: err}
}

// LoadLayers reads and decodes the testlayer config at path — the dual-mode
// entry point (RC-39): a missing file (os.IsNotExist) returns
// DefaultLayers() with no error — a repo that never configured testlayer
// still gets integration/e2e classification for free, exactly matching
// Classify's own built-in behavior. A present file is parsed and used
// EXACTLY as declared — no merge with DefaultLayers: an author who also
// wants unit-layer behavior gets it for free from Classify/ClassifyWith's
// own unconditional _test.go-implies-"unit" base case, never from a
// silently re-added default layer list. An explicit `layers: []` is a
// legitimate "no custom layers beyond the base case" config and returns an
// empty, error-free LayerSet — unlike internal/guardspec's guard file
// (where an empty list governs nothing and is treated as a likely
// mistake), an empty layer list is a normal, working testlayer config. A
// malformed present file fails loudly with every field error collected
// into one *LoadError, mirroring internal/guardspec's LoadGuards.
func LoadLayers(path string) (LayerSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultLayers(), nil
		}
		return LayerSet{}, fmt.Errorf("testlayer: read %s: %w", path, err)
	}

	cctx := cuecontext.New()
	v := cctx.CompileBytes(data, cue.Filename(path))
	if v.Err() != nil {
		return LayerSet{}, &LoadError{Path: path, Fields: []*FieldError{fieldErr(path, "<parse>", v.Err())}}
	}

	list := v.LookupPath(cue.ParsePath("layers"))
	if !list.Exists() {
		return LayerSet{}, nil
	}
	iter, err := list.List()
	if err != nil {
		return LayerSet{}, &LoadError{Path: path, Fields: []*FieldError{fieldErr(path, "layers", err)}}
	}

	var errs []*FieldError
	var out LayerSet
	idx := 0
	for iter.Next() {
		lv := iter.Value()
		prefix := fmt.Sprintf("layers[%d]", idx)
		idx++

		l := LayerSpec{
			Name:      lookupString(lv, "name"),
			Globs:     lookupStrings(lv, "globs"),
			BuildTags: lookupStrings(lv, "build_tags"),
			Dirs:      lookupStrings(lv, "dirs"),
		}
		if l.Name == "" {
			errs = append(errs, fieldErr(path, prefix+".name", fmt.Errorf("must be non-empty")))
		}
		for _, spec := range []struct {
			field string
			vals  []string
		}{{"globs", l.Globs}, {"build_tags", l.BuildTags}, {"dirs", l.Dirs}} {
			f := lv.LookupPath(cue.ParsePath(spec.field))
			if f.Exists() && len(spec.vals) == 0 {
				errs = append(errs, fieldErr(path, prefix+"."+spec.field, fmt.Errorf("must be non-empty when present")))
			}
		}
		out.Layers = append(out.Layers, l)
	}
	if len(errs) > 0 {
		return LayerSet{}, &LoadError{Path: path, Fields: errs}
	}
	return out, nil
}

func lookupString(v cue.Value, field string) string {
	f := v.LookupPath(cue.ParsePath(field))
	if !f.Exists() {
		return ""
	}
	s, _ := f.String()
	return s
}

func lookupStrings(v cue.Value, field string) []string {
	f := v.LookupPath(cue.ParsePath(field))
	if !f.Exists() {
		return nil
	}
	iter, err := f.List()
	if err != nil {
		return nil
	}
	var out []string
	for iter.Next() {
		s, serr := iter.Value().String()
		if serr == nil {
			out = append(out, s)
		}
	}
	return out
}
