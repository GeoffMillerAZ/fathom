package readout

// UnplacedIdents lists the identifier roots of recs that defs place nowhere
// (compileDefFor finds no one definition), each once, in output order: the
// identifiers the readout hook's daemon join asks the index about (RO-30).
func UnplacedIdents(recs []Record, defs []CompileDef) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range recs {
		if r.Kind != RecordCompileError {
			continue
		}
		id := field(r, "ident")
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		if _, ok := compileDefFor(id, defs); !ok {
			out = append(out, id)
		}
	}
	return out
}
