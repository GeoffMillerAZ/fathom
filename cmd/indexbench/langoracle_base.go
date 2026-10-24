package main

// langoracle_base.go — a composing dialect scored against its base's oracle
// (LB-Q-timoni-rest). The Timoni extractor composes the CUE extraction
// unchanged, so every CUE field and definition reference it records is a
// reference of the Timoni reading; the timoni oracle's truth covers only the
// Timoni-specific pairs. -base-oracle names the base dialect's oracle (cue lsp
// definition answers, calibration/oracles/cue): its calls and references join
// the references the own oracle grades, and its cross_references the
// cross-references, so the composed references are graded by the base
// dialect's oracle and the dialect's own pairs by its own.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// mergeBaseOracles returns tools with each oracle file that grades references
// (or cross_references) carrying the base oracles' pairs too, written under
// dir; a file with neither family is returned as given. A pair in both sets is
// kept once. notes disclose the base oracles by tool, version and pair count.
func mergeBaseOracles(tools, bases []string, dir string) ([]string, []string, error) {
	var refs []oraclePair
	var xrefs []xrefOracleEntry
	var notes []string
	for _, p := range bases {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, nil, err
		}
		var bf toolOracleFile
		if err := json.Unmarshal(b, &bf); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", p, err)
		}
		refs = append(refs, bf.Calls...)
		refs = append(refs, bf.References...)
		xrefs = append(xrefs, bf.CrossReferences...)
		notes = append(notes, fmt.Sprintf("base-dialect oracle %s %s (tier %s): %d reference and %d cross-reference pairs join the composing dialect's own oracle's, so the composed extraction's references are graded by the base dialect's oracle and the dialect's own pairs by its own", bf.Tool, bf.Version, orDefault(bf.Tier, "A"), len(bf.Calls)+len(bf.References), len(bf.CrossReferences)))
	}
	out := make([]string, len(tools))
	for i, p := range tools {
		out[i] = p
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, nil, err
		}
		var tf toolOracleFile
		if err := json.Unmarshal(b, &tf); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", p, err)
		}
		if len(tf.References) == 0 && len(tf.CrossReferences) == 0 {
			continue
		}
		if len(tf.References) > 0 {
			seen := map[oraclePair]bool{}
			for _, r := range tf.References {
				seen[r] = true
			}
			for _, r := range refs {
				if !seen[r] {
					seen[r] = true
					tf.References = append(tf.References, r)
				}
			}
		}
		if len(tf.CrossReferences) > 0 {
			seen := map[xrefOracleEntry]bool{}
			for _, x := range tf.CrossReferences {
				seen[x] = true
			}
			for _, x := range xrefs {
				if !seen[x] {
					seen[x] = true
					tf.CrossReferences = append(tf.CrossReferences, x)
				}
			}
		}
		mb, err := json.Marshal(tf)
		if err != nil {
			return nil, nil, err
		}
		out[i] = filepath.Join(dir, fmt.Sprintf("merged-%d-%s", i, filepath.Base(p)))
		if err := os.WriteFile(out[i], mb, 0o644); err != nil {
			return nil, nil, err
		}
	}
	return out, notes, nil
}
