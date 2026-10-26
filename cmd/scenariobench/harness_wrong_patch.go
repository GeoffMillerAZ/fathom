package main

import "github.com/geoffmilleraz/fathom/internal/scenariobench"

func init() {
	offlineHarnesses["wrong-patch"] = offlineHarnessFactory{
		help: "test patch alone or non-gold files, no spend; killed mutants wait for BX-10 (MineMutants)",
		new:  func(offlineHarnessConfig) scenariobench.Harness { return scenariobench.NewWrongPatchHarness() },
	}
}
