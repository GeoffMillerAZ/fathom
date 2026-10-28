### Added
- Index named Go struct fields as owner-qualified symbols, including unexported and embedded fields.
- Keep functions, methods and types ahead of Go fields in console search and `find_symbol`, resolve a name a field shares to the function, method or type in plan-row verification, `symbol_context`, map anchors and receipts (the field's own `Owner.Field` still names it), and show changed fields separately from the traversable impact radius with their unindexed-reference limit.
