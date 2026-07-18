package editor

import "github.com/geoffmilleraz/agentic-kit/surfaces/forms/cueform"

// BuildFieldModel converts a compiled [cueform.ResolvedForm] into a flat,
// ordered slice of [FieldModel] values suitable for rendering in an editor.
//
// The algorithm performs a depth-first walk of the resolved field tree:
//   - Top-level fields are emitted at depth 0.
//   - Group children are emitted in definition order, depth = parent depth + 1.
//   - For list fields, the item template is emitted immediately after the list
//     field itself, at depth = list depth + 1, with IsListItem = true.
//
// The output slice length and order are fully deterministic for identical
// ResolvedForm inputs.
func BuildFieldModel(rf cueform.ResolvedForm) []FieldModel {
	var out []FieldModel
	for _, f := range rf.Fields {
		out = appendFieldModels(out, f, 0, false)
	}
	return out
}

// appendFieldModels recursively walks a ResolvedField tree and appends
// FieldModel entries to dst in depth-first, definition order.
func appendFieldModels(dst []FieldModel, f cueform.ResolvedField, depth int, isListItem bool) []FieldModel {
	placeholder := placeholderFor(f)

	opts := make([]FieldOption, len(f.Options))
	for i, o := range f.Options {
		opts[i] = FieldOption{Value: o.Value, Label: o.Label}
	}

	dst = append(dst, FieldModel{
		Path:          f.Path,
		Key:           f.Key,
		Kind:          resolvedFieldKind(f.Kind),
		Label:         f.Label,
		Description:   f.Description,
		Required:      f.Required,
		Options:       opts,
		Depth:         depth,
		IsListItem:    isListItem,
		DefaultValue:  f.Default,
		Placeholder:   placeholder,
		HasConditions: len(f.Conditions) > 0,
		HasValidation: f.Validation != nil,
	})

	// Recurse into group children.
	if f.Kind == cueform.FieldKindGroup {
		for _, child := range f.Fields {
			dst = appendFieldModels(dst, child, depth+1, false)
		}
	}

	// Emit the list item template immediately after the list field.
	if f.Kind == cueform.FieldKindList && f.Item != nil {
		dst = appendFieldModels(dst, *f.Item, depth+1, true)
	}

	return dst
}

// placeholderFor derives the placeholder value for a field.
// Precedence: Example > Prefill > nil.
func placeholderFor(f cueform.ResolvedField) any {
	if f.Example != nil {
		return f.Example
	}
	if f.Prefill != nil {
		return f.Prefill
	}
	return nil
}
