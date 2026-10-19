package callgraph

// ReachName places a reach endpoint's name as ReachableTyped does
// (canonicalize) and, when that refuses, returns every key carrying the
// name from the all-matches index CandidateKeys reads, sorted. Two or more
// candidates is an ambiguous name, none a name g does not hold: either way
// the reach was never walked, which "no typed path" would misreport.
func ReachName(g *Graph, name string) (key string, candidates []string) {
	if g == nil {
		return "", nil
	}
	if key = g.canonicalize(name); key != "" {
		return key, nil
	}
	return "", CandidateKeys(g, name)
}
