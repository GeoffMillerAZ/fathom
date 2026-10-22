package mcp

// budgeted_handler.go — SU-3 (docs/plans/substrate.md §A, ADR-041 decision
// 3): a dispatch-level backstop so a tool handler that forgets to budget
// its own result (O90's defect class: a ~63k-token symbol_context answer
// shipped before that tool had any budget line at all) cannot regress past
// defaultCharBudget either. review_diff instead bounds its complete MCP
// result after dispatch assembles both payload copies and metadata.
//
// A handler that already keeps its result under budget sees NO difference:
// budgetedResult only touches the marshaled bytes when they exceed
// byteBudget, so an already-disciplined tool's response is byte-identical
// to what it always was (TestBudgetedHandler_UnderBudgetIsByteIdentical).

import (
	"context"
	"encoding"
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
	"unicode/utf8"
)

// budgetedHandler wraps h so its result passes through the SAME byte
// budget + elision seam fitToolBudget/applyBudget already give disciplined
// handlers (applyBudget in budget.go), applied generically over the
// handler's marshaled JSON result rather than a typed slice. A tool that
// honours a declared budget (declared, toolDescriptor.declaredBudget) is cut
// at the caller's declaration instead when ctx carries one
// (WithDeclaredBudget, O1169); without one, the default applies.
func budgetedHandler(h func(ctx context.Context, args json.RawMessage) (any, *rpcError), declared bool) func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
	return func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		result, rpcErr := h(ctx, args)
		if rpcErr != nil {
			return nil, rpcErr
		}
		budget := defaultCharBudget
		if n := declaredBudgetFrom(ctx); declared && n > 0 {
			budget = n
		}
		return budgetedResult(result, budget), nil
	}
}

// budgetedResult returns result unchanged when its own JSON encoding fits
// byteBudget (the common case: nearly every handler already budgets
// itself, so this is a pure passthrough for them). When it does not, the
// largest top-level JSON array in the result — the result itself if it IS
// an array, else the largest array-valued field of a top-level object — is
// trimmed with fitToBudget's own deterministic nested-prefix discipline,
// and the standard elided block (elisionFor) is attached so the cut is
// never silent. A result with no array shape to trim (a giant scalar or
// deeply nested object with no top-level array) falls back to a labelled
// truncated preview — rare, since every existing over-budget tool this row
// was written against (find_callers, outline, symbol_context,
// staleness_report, query_symbols) returns a list or an object wrapping
// one.
func budgetedResult(result any, byteBudget int) any {
	if byteBudget <= 0 {
		byteBudget = defaultCharBudget
	}
	// RT-90 (§4.5): size first, without a copy of the whole — jsonSize
	// encodes a top-level map or slice one element at a time into a
	// counter and stops once past the budget, so an under-budget result
	// (the common case) passes through with no marshaled copy at all.
	if n, err := jsonSize(result, byteBudget); err != nil || n <= byteBudget {
		return result
	}
	// Preserve the answer-first envelope when its body needs trimming too.
	switch e := result.(type) {
	case *summaryEnvelope:
		copy := *e
		copy.body = budgetedResult(e.body, max(1, byteBudget-jsonStringLen(e.summary)-11))
		return &copy
	}
	// A top-level map: the largest array-valued field is found by
	// measuring (no copy), and only that field's elements are encoded; the
	// other fields pass through as the handler built them.
	if out, ok := trimLargestMapArray(result, byteBudget); ok {
		return out
	}
	b, err := json.Marshal(result)
	if err != nil {
		return result
	}
	first := firstJSONByte(b)

	// Case 1: the result itself is a top-level JSON array.
	if first == '[' {
		var arr []json.RawMessage
		if err := json.Unmarshal(b, &arr); err == nil {
			kept, el := applyBudget(arr, byteBudget, "as returned", "narrow the query")
			out := map[string]any{"items": kept}
			if el != nil {
				out["elided"] = el
			}
			return out
		}
	}

	// Case 2: a top-level object — trim its largest array-valued field,
	// found by its raw bytes (no per-field decode); only that field is
	// decoded.
	if first == '{' {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(b, &obj); err == nil {
			if field, items, ok := largestArrayField(obj); ok {
				kept, el := applyBudget(items, byteBudget, "as returned", "narrow the query")
				keptRaw, mErr := json.Marshal(kept)
				if mErr == nil {
					obj[field] = keptRaw
					if el != nil {
						if elRaw, eErr := json.Marshal(el); eErr == nil {
							obj["elided"] = elRaw
						}
					}
					if rawSummary, ok := obj["summary"]; ok {
						var summary string
						if json.Unmarshal(rawSummary, &summary) == nil {
							delete(obj, "summary")
							return withSummary(summary, obj)
						}
					}
					return obj
				}
			}
		}
	}

	// Fallback: no array field to trim at all — a truncated, labelled
	// preview rather than silently shipping the oversized payload.
	cut := byteBudget
	if cut > len(b) {
		cut = len(b)
	}
	preview := map[string]any{
		"truncated": true,
		"note":      "response exceeded the dispatch byte budget and had no array field to trim",
		"preview":   string(b[:cut]),
	}
	if e, ok := result.(summaryEnvelope); ok {
		return withSummary(e.summary, preview)
	}
	return preview
}

// jsonSize is v's JSON encoding length, or a number past limit once it is
// known to exceed it. Maps with string keys, slices and arrays are
// measured element by element, recursively (an encoder writing into a
// counter, its buffer pooled), so peak memory is the largest leaf, never
// the whole; a struct or any other leaf is encoded once into the counter.
func jsonSize(v any, limit int) (int, error) {
	var cw countingWriter
	js := jsonSizer{enc: json.NewEncoder(&cw), cw: &cw, limit: limit}
	if err := js.size(reflect.ValueOf(v)); err != nil {
		return 0, err
	}
	return cw.n, nil
}

type jsonSizer struct {
	enc   *json.Encoder
	cw    *countingWriter
	limit int
}

func (js jsonSizer) size(rv reflect.Value) error {
	if rv.IsValid() && rv.CanInterface() {
		switch e := rv.Interface().(type) {
		case summaryEnvelope:
			return js.summary(e)
		case *summaryEnvelope:
			if e != nil {
				return js.summary(*e)
			}
		}
	}
	for rv.IsValid() && (rv.Kind() == reflect.Interface || rv.Kind() == reflect.Pointer) && !rv.IsNil() && !implementsMarshaler(rv) {
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		js.cw.n += 4 // null
		return nil
	}
	if js.cw.n > js.limit {
		return nil
	}
	switch {
	case rv.Kind() == reflect.Map && rv.Type().Key().Kind() == reflect.String && !implementsMarshaler(rv):
		if rv.IsNil() {
			js.cw.n += 4
			return nil
		}
		js.cw.n += 2 // {}
		iter := rv.MapRange()
		first := true
		for iter.Next() {
			if !first {
				js.cw.n++
			}
			first = false
			js.cw.n += jsonStringLen(iter.Key().String())
			js.cw.n++ // :
			if err := js.size(iter.Value()); err != nil {
				return err
			}
			if js.cw.n > js.limit {
				return nil
			}
		}
		return nil
	case (rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() != reflect.Uint8 || rv.Kind() == reflect.Array) && !implementsMarshaler(rv):
		if rv.Kind() == reflect.Slice && rv.IsNil() {
			js.cw.n += 4
			return nil
		}
		js.cw.n += 2 // []
		for i := 0; i < rv.Len(); i++ {
			if i > 0 {
				js.cw.n++
			}
			if err := js.size(rv.Index(i)); err != nil {
				return err
			}
			if js.cw.n > js.limit {
				return nil
			}
		}
		return nil
	}
	return js.leaf(rv)
}

// summary sizes the typed envelope without invoking its MarshalJSON method.
func (js jsonSizer) summary(e summaryEnvelope) error {
	before := js.cw.n
	if err := js.size(reflect.ValueOf(e.body)); err != nil {
		return err
	}
	comma := 1
	if js.cw.n-before == 2 {
		comma = 0
	}
	js.cw.n += 10 + jsonStringLen(e.summary) + comma
	return nil
}

// leaf adds rv's encoded length: strings, booleans and integers are
// measured directly (strings with invalid UTF-8 probe the encoder once);
// anything else is encoded into the counter (less Encode's trailing newline).
func (js jsonSizer) leaf(rv reflect.Value) error {
	if !rv.CanInterface() {
		return nil
	}
	if !implementsMarshaler(rv) {
		switch rv.Kind() {
		case reflect.String:
			js.cw.n += jsonStringLen(rv.String())
			return nil
		case reflect.Bool:
			js.cw.n += 5
			if rv.Bool() {
				js.cw.n--
			}
			return nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			var buf [24]byte
			js.cw.n += len(strconv.AppendInt(buf[:0], rv.Int(), 10))
			return nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			var buf [24]byte
			js.cw.n += len(strconv.AppendUint(buf[:0], rv.Uint(), 10))
			return nil
		}
	}
	if err := js.enc.Encode(rv.Interface()); err != nil {
		return err
	}
	js.cw.n--
	return nil
}

func implementsMarshaler(rv reflect.Value) bool {
	t := rv.Type()
	m := reflect.TypeOf((*json.Marshaler)(nil)).Elem()
	tm := reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
	return t.Implements(m) || t.Implements(tm)
}

// trimLargestMapArray is budgetedResult's case 2 for a map[string]T
// result, without marshaling the whole: ok is false for any other shape
// or a map with no array-valued field.
func trimLargestMapArray(result any, byteBudget int) (map[string]any, bool) {
	rv := reflect.ValueOf(result)
	if !rv.IsValid() || rv.Kind() != reflect.Map || rv.Type().Key().Kind() != reflect.String || implementsMarshaler(rv) {
		return nil, false
	}
	keys := make([]string, 0, rv.Len())
	for _, k := range rv.MapKeys() {
		keys = append(keys, k.String())
	}
	sort.Strings(keys)
	best, bestSize := "", -1
	for _, k := range keys {
		v := rv.MapIndex(reflect.ValueOf(k).Convert(rv.Type().Key()))
		for v.IsValid() && v.Kind() == reflect.Interface && !v.IsNil() {
			v = v.Elem()
		}
		if !v.IsValid() || implementsMarshaler(v) || !(v.Kind() == reflect.Slice && v.Type().Elem().Kind() != reflect.Uint8 && !v.IsNil() || v.Kind() == reflect.Array) {
			continue
		}
		n, err := jsonSize(v.Interface(), int(^uint(0)>>1))
		if err != nil {
			return nil, false
		}
		if n > bestSize {
			best, bestSize = k, n
		}
	}
	if bestSize < 0 {
		return nil, false
	}
	field := rv.MapIndex(reflect.ValueOf(best).Convert(rv.Type().Key()))
	for field.Kind() == reflect.Interface {
		field = field.Elem()
	}
	items := make([]json.RawMessage, 0, field.Len())
	for i := 0; i < field.Len(); i++ {
		b, err := json.Marshal(field.Index(i).Interface())
		if err != nil {
			return nil, false
		}
		items = append(items, b)
	}
	kept, el := applyBudget(items, byteBudget, "as returned", "narrow the query")
	out := make(map[string]any, len(keys)+1)
	for _, k := range keys {
		out[k] = rv.MapIndex(reflect.ValueOf(k).Convert(rv.Type().Key())).Interface()
	}
	out[best] = kept
	if el != nil {
		out["elided"] = el
	}
	return out, true
}

// jsonStringLen is the length encoding/json gives s (HTML-escaped, as
// json.Marshal does): quotes, two-byte escapes for \\ " \b \f \n \r
// \t, six-byte \u escapes for other controls, < > & and U+2028/U+2029.
// Invalid UTF-8 uses the active encoder's replacement-rune representation.
func jsonStringLen(s string) int {
	n := 2
	replacementLen := 0
	for i := 0; i < len(s); {
		b := s[i]
		if b < utf8.RuneSelf {
			switch {
			case b == '\\' || b == '"' || b == '\b' || b == '\f' || b == '\n' || b == '\r' || b == '\t':
				n += 2
			case b < 0x20 || b == '<' || b == '>' || b == '&':
				n += 6
			default:
				n++
			}
			i++
			continue
		}
		c, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case c == utf8.RuneError && size == 1:
			if replacementLen == 0 {
				// The legacy encoder escapes invalid UTF-8 as \ufffd;
				// the Go 1.27 JSON v2 encoder writes the UTF-8 rune.
				// Ask the active encoder once, without copying s.
				b, _ := json.Marshal("\xff")
				replacementLen = len(b) - 2 // exclude quotes
			}
			n += replacementLen
		case c == '\u2028' || c == '\u2029':
			n += 6
		default:
			n += size
		}
		i += size
	}
	return n
}

// countingWriter counts bytes written and keeps none.
type countingWriter struct{ n int }

func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += len(p)
	return len(p), nil
}

// firstJSONByte is b's first non-whitespace byte (0 for none).
func firstJSONByte(b []byte) byte {
	for _, c := range b {
		switch c {
		case ' ', '\t', '\n', '\r':
			continue
		}
		return c
	}
	return 0
}

// largestArrayField finds the top-level field of obj whose value is a JSON
// array with the most bytes, returning its name and decoded elements.
// Deterministic: obj's keys are walked in sorted order so a byte-size tie
// always resolves to the same field (internal/AGENTS.md's "map iteration order must
// never reach a result").
func largestArrayField(obj map[string]json.RawMessage) (field string, items []json.RawMessage, ok bool) {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	bestSize := -1
	for _, k := range keys {
		// An array by its first byte; only the winner is decoded (RT-90).
		if firstJSONByte(obj[k]) == '[' && len(obj[k]) > bestSize {
			bestSize = len(obj[k])
			field = k
		}
	}
	if bestSize < 0 {
		return "", nil, false
	}
	if err := json.Unmarshal(obj[field], &items); err != nil {
		return "", nil, false
	}
	return field, items, true
}
