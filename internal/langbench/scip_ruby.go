package langbench

// scip_ruby.go — scip-ruby's call-site rules (LB-F257, calibration/oracles/scip-ruby).
//
// scip-ruby (Sorbet) writes a reference only where it resolves a method: a call
// on a receiver Sorbet types T.untyped (an untyped local, a test's server
// object, a value read from a hash) and `super` carry no occurrence at all.
// fathom's pair for such a call is one the index cannot judge, so Ruby takes
// the oracle-blind rule (LB-F175): a pair the truth lacks whose every written
// call site (`name(`) has no occurrence of the index is left out of both
// counts and counted, under LB-F203's 10% cap. A call written without
// parentheses (most Ruby calls) has no written site the rule can find, so its
// pair stays scored: the rule can only shrink what Sorbet cannot see.
func rubySCIPOptions(src func(path string) []byte) SCIPOptions {
	return SCIPOptions{Source: src, OracleBlindSites: src != nil}
}
