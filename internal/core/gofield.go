package core

// WithoutShadowingFields is the one rule for a lookup that resolves a name
// to the ONE definition it means when Go struct fields share that name. A
// field is named Owner.Field, like a method, so a bare name or a member
// spelling also matches every same-named field: `Default` matches the
// function Default and the field Capability.Default. A GoField never
// shadows the func, method, type or other definition the name means: while
// nodes holds any non-field, every field is dropped unless name spells
// that field itself (its Owner.Field name or its key), which is the only
// way to name a field explicitly. A field-only set is returned unchanged,
// so a name that means a field still resolves to it.
//
// Readers that pick one target apply it (plan-row contracts, symbol_context,
// map anchors, gone rows, receipts). Listings and deletion checks do not:
// find_symbol lists every field (below callables, C-R14), and a delete
// check keeps a same-named field to withhold its proof. Order is kept.
func WithoutShadowingFields(nodes []SymbolNode, name string) []SymbolNode {
	nonField := false
	for _, n := range nodes {
		if n.Kind != KindGoField {
			nonField = true
			break
		}
	}
	if !nonField {
		return nodes
	}
	kept := make([]SymbolNode, 0, len(nodes))
	for _, n := range nodes {
		if n.Kind != KindGoField || n.Name == name || n.Key() == name {
			kept = append(kept, n)
		}
	}
	return kept
}
