package core

// Rust members the CGO walker and the rust dialect extract beside the item
// kinds in model.go (LB-F82): a struct or union field, an enum variant, and an
// associated const or type of an impl or trait.
const (
	KindRustField      SourceKind = "RustField"
	KindRustVariant    SourceKind = "RustVariant"
	KindRustAssocConst SourceKind = "RustAssocConst"
	KindRustAssocType  SourceKind = "RustAssocType"
)

// KindRustUnion is a `union` item, a string the rust dialect has always
// emitted (dialects/rust/dialect.json); the CGO walker now emits it too so a
// union's fields have an owner.
const KindRustUnion SourceKind = "RustUnion"
