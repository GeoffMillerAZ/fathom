package depgraph

import "fmt"

// ResolveDeclaredEdges resolves declared cross-ecosystem edges against
// real units. A declaration naming a unit that does not exist is refused
// because dropping a dangling declaration would under-key its consumer.
// Declarations resolve in input order.
func ResolveDeclaredEdges(decls []EdgeDecl, units []Unit) ([]Edge, error) {
	exists := make(map[UnitRef]bool, len(units))
	for _, u := range units {
		exists[UnitRef{Ecosystem: u.Ecosystem, Name: u.Name}] = true
	}
	out := make([]Edge, 0, len(decls))
	for _, d := range decls {
		if !exists[d.From] {
			return nil, fmt.Errorf("depgraph: declared edge from unknown unit %s/%s", d.From.Ecosystem, d.From.Name)
		}
		if !exists[d.To] {
			return nil, fmt.Errorf("depgraph: declared edge to unknown unit %s/%s", d.To.Ecosystem, d.To.Name)
		}
		out = append(out, Edge{From: d.From, To: d.To})
	}
	return out, nil
}
