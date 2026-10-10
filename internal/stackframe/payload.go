package stackframe

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// shapesJSON is the payload-shape registry: universal defaults, data only.
//
//go:embed shapes.json
var shapesJSON []byte

// registry is shapes.json decoded.
type registry struct {
	About       string `json:"about"`
	FrameArrays []struct {
		Shape     string `json:"shape"`
		Key       string `json:"key"`
		File      string `json:"file"`
		Line      string `json:"line"`
		Func      string `json:"func"`
		InProject string `json:"in_project"`
	} `json:"frame_arrays"`
	TraceTexts []struct {
		Shape string `json:"shape"`
		Key   string `json:"key"`
		With  string `json:"with"`
	} `json:"trace_texts"`
	CodeAttributes []struct {
		Shape string `json:"shape"`
		File  string `json:"file"`
		Line  string `json:"line"`
		Func  string `json:"func"`
	} `json:"code_attributes"`
	// NeverRead names subtrees the walk never enters, whatever keys they
	// hold: message, breadcrumb, user and metaData text is never read.
	NeverRead []string `json:"never_read"`
	// Causes orders chained errors root cause first: a nested key (the
	// Lambda Java runtime's "cause") is walked before the object holding
	// it, and an array listing the error first and its causes after
	// (Bugsnag's "exceptions") is walked last-first.
	Causes struct {
		Nested         []string `json:"nested"`
		ListedRootLast []string `json:"listed_root_last"`
	} `json:"causes"`
	AttributeLists struct {
		Key     string   `json:"key"`
		Value   string   `json:"value"`
		Scalars []string `json:"scalars"`
	} `json:"attribute_lists"`
}

// loadRegistry decodes the embedded registry strictly. It is decoded per
// call (a few hundred bytes), never held in a package-level cache.
func loadRegistry() registry {
	var r registry
	dec := json.NewDecoder(bytes.NewReader(shapesJSON))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		panic("stackframe: shapes.json: " + err.Error()) // embedded data; TestRegistryDecodes pins it
	}
	return r
}

// IsPayload reports whether text is a JSON payload of a registered shape
// carrying at least one frame tuple.
func IsPayload(text string) bool {
	r, ok := ExtractPayload(text)
	return ok && len(r.Frames) > 0
}

// ExtractPayload reads text as a JSON payload. ok is false only when text
// is not a JSON object or array; then the caller reads it raw. A JSON value
// no registered shape matches is still a payload, with no frames: its
// unregistered fields are never read as a trace. Only the registry's keys
// are read: frames from frame arrays and trace texts first (in document
// order, object keys sorted), then frames from code attributes, which name
// the instrumented code rather than the throw.
func ExtractPayload(text string) (Result, bool) {
	t := strings.TrimSpace(text)
	if t == "" || (t[0] != '{' && t[0] != '[') {
		return Result{}, false
	}
	dec := json.NewDecoder(strings.NewReader(t))
	dec.UseNumber()
	var root any
	if err := dec.Decode(&root); err != nil {
		return Result{}, false
	}
	reg := loadRegistry()
	w := walker{reg: reg, shapes: map[string]bool{}}
	w.walk(root)
	shapes := make([]string, 0, len(w.shapes))
	for s := range w.shapes {
		shapes = append(shapes, s)
	}
	sort.Strings(shapes)
	return Result{Frames: finish(append(w.stack, w.code...)), Payload: true, Shapes: shapes}, true
}

type walker struct {
	reg         registry
	stack, code []Frame
	shapes      map[string]bool
}

func (w *walker) walk(v any) {
	switch x := v.(type) {
	case []any:
		for _, it := range x {
			w.walk(it)
		}
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			if !w.neverRead(k) {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		// A nested cause is the deeper error: its frames come first.
		for _, k := range keys {
			if contains(w.reg.Causes.Nested, k) {
				w.walk(x[k])
			}
		}
		w.object(w.view(x))
		for _, k := range keys {
			if contains(w.reg.Causes.Nested, k) {
				continue
			}
			if arr, ok := x[k].([]any); ok && contains(w.reg.Causes.ListedRootLast, k) {
				for i := len(arr) - 1; i >= 0; i-- {
					w.walk(arr[i])
				}
				continue
			}
			w.walk(x[k])
		}
	}
}

func (w *walker) neverRead(k string) bool { return contains(w.reg.NeverRead, k) }

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// view is an object's own keys plus the keys of every OTLP attribute list
// it holds ({"attributes": [{"key": k, "value": {"stringValue": v}}]}).
func (w *walker) view(obj map[string]any) map[string]any {
	al := w.reg.AttributeLists
	view := make(map[string]any, len(obj))
	keys := make([]string, 0, len(obj))
	for k, v := range obj {
		if w.neverRead(k) {
			continue
		}
		view[k] = v
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		list, ok := obj[k].([]any)
		if !ok {
			continue
		}
		for _, it := range list {
			kv, ok := it.(map[string]any)
			if !ok {
				continue
			}
			name, ok := kv[al.Key].(string)
			if !ok {
				continue
			}
			val, ok := kv[al.Value].(map[string]any)
			if !ok {
				continue
			}
			for _, s := range al.Scalars {
				if sv, ok := val[s]; ok {
					if _, dup := view[name]; !dup {
						view[name] = sv
					}
					break
				}
			}
		}
	}
	return view
}

// object applies the registry's rules to one object's view.
func (w *walker) object(view map[string]any) {
	for _, fa := range w.reg.FrameArrays {
		arr, ok := view[fa.Key].([]any)
		if !ok {
			continue
		}
		var frames []Frame
		anyInProject := false
		var inProject []bool
		for _, it := range arr {
			o, ok := it.(map[string]any)
			if !ok {
				continue
			}
			file := str(o[fa.File])
			fn := str(o[fa.Func])
			f := Frame{File: normalizePath(file), Line: num(o[fa.Line]), Func: fn}
			if isJVMFile(file) {
				f = jvmFrame(fn, file, f.Line)
			}
			ip, _ := o[fa.InProject].(bool)
			anyInProject = anyInProject || ip
			frames = append(frames, f)
			inProject = append(inProject, ip)
		}
		if len(frames) == 0 {
			continue
		}
		w.shapes[fa.Shape] = true
		for i, f := range frames {
			if anyInProject && !inProject[i] {
				continue // the notifier marked the app's own frames
			}
			w.stack = append(w.stack, f)
		}
	}
	for _, tt := range w.reg.TraceTexts {
		v, ok := view[tt.Key]
		if !ok {
			continue
		}
		if tt.With != "" {
			if _, ok := view[tt.With]; !ok {
				continue
			}
		}
		var text string
		switch x := v.(type) {
		case string:
			text = x
		case []any:
			var lines []string
			for _, it := range x {
				if s, ok := it.(string); ok {
					lines = append(lines, s)
				}
			}
			text = strings.Join(lines, "\n")
		default:
			continue
		}
		w.shapes[tt.Shape] = true
		w.stack = append(w.stack, Parse(text)...)
	}
	for _, ca := range w.reg.CodeAttributes {
		file, line := str(view[ca.File]), num(view[ca.Line])
		if file == "" || line <= 0 {
			continue
		}
		w.shapes[ca.Shape] = true
		w.code = append(w.code, Frame{File: normalizePath(file), Line: line, Func: str(view[ca.Func])})
	}
}

func isJVMFile(f string) bool {
	for _, ext := range []string{".java", ".kt", ".kts", ".scala", ".groovy", ".clj"} {
		if strings.HasSuffix(f, ext) {
			return true
		}
	}
	return false
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func num(v any) int {
	switch x := v.(type) {
	case json.Number:
		n, err := strconv.Atoi(x.String())
		if err != nil {
			return 0
		}
		return n
	case string:
		n, err := strconv.Atoi(x)
		if err != nil {
			return 0
		}
		return n
	}
	return 0
}
