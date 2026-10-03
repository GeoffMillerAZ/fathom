package main

// langoracle_iac.go — what wave iac's dialects count but never extract,
// disclosed in a reading's notes so the count a reading cites is one this
// harness took: a Terragrunt reference no static reading resolves, by reason
// (extractor.TerragruntUnresolvedRefs, LB-F142), a Timoni bundle instance's
// module url no static reading resolves, by reason
// (extractor.TimoniUnresolvedRefs, LB-F218), and a subject's YAML file whose
// Argo, Flux or Crossplane documents sit beside other documents, so ingest
// keeps it Kubernetes, whole (fusion.CRMixed, LB-F149).

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/fusion"
)

// crMixedDialects are the dialects fusion's crDispatch claims a file for only
// when every document of it is theirs.
var crMixedDialects = map[string]bool{"argo": true, "flux": true, "crossplane": true}

// iacCountNotes is the subject's notes for dialect: measured are the files
// the dialect read, yamls every YAML file of the subject (a mixed file is
// none of the dialect's, so only the whole subject shows it). Every other
// dialect, and a subject with nothing to count, gets none.
func iacCountNotes(dialect, name string, measured, yamls []core.SourceFile) []string {
	switch {
	case dialect == "terragrunt":
		by := map[string]int{}
		for _, f := range measured {
			for _, u := range extractor.TerragruntUnresolvedRefs(f.Path, f.Content) {
				by[u.Reason]++
			}
		}
		return unresolvedNote(name, "Terragrunt references", "TerragruntUnresolvedRefs", by)
	case dialect == "timoni":
		by := map[string]int{}
		for _, f := range measured {
			for _, u := range extractor.TimoniUnresolvedRefs(f.Path, f.Content) {
				by[u.Reason]++
			}
		}
		return unresolvedNote(name, "Timoni module references", "TimoniUnresolvedRefs", by)
	case crMixedDialects[dialect]:
		var mixed []string
		for _, f := range yamls {
			if fusion.CRMixed(f.Path, f.Content) {
				mixed = append(mixed, f.Path)
			}
		}
		if len(mixed) == 0 {
			return nil
		}
		sort.Strings(mixed)
		return []string{fmt.Sprintf("%s: %d files hold Argo, Flux or Crossplane documents beside other documents and stay Kubernetes, whole (fusion.CRMixed): %s", name, len(mixed), strings.Join(firstNames(mixed, 5), ", "))}
	}
	return nil
}

// unresolvedNote is the one note a subject's unresolved references make,
// counted by reason in sorted order; none when nothing is unresolved.
func unresolvedNote(name, what, counter string, by map[string]int) []string {
	total := 0
	parts := make([]string, 0, len(by))
	for _, k := range sortedKeys(by) {
		total += by[k]
		parts = append(parts, fmt.Sprintf("%s %d", k, by[k]))
	}
	if total == 0 {
		return nil
	}
	return []string{fmt.Sprintf("%s: %d %s no static reading resolves, counted not guessed (%s): %s", name, total, what, counter, strings.Join(parts, ", "))}
}

// subjectYAML reads every .yaml and .yml file of the subject for a CR
// dialect's mixed count; nil for any other dialect. A file it cannot read
// fails the measurement, as an owned file does (O400: a skipped read would
// shrink the count silently).
func subjectYAML(dialect, root string, refs []core.SourceRef) ([]core.SourceFile, error) {
	if !crMixedDialects[dialect] {
		return nil, nil
	}
	var out []core.SourceFile
	for _, ref := range refs {
		if ext := strings.ToLower(filepath.Ext(ref.Path)); ext != ".yaml" && ext != ".yml" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ref.Path)))
		if err != nil {
			return nil, err
		}
		out = append(out, core.SourceFile{Path: ref.Path, Content: b})
	}
	return out, nil
}
