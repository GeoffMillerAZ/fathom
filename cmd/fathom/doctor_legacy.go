package main

// doctor_legacy.go -- the `legacy_files` row: old copies fathom no longer
// reads (docs/design/fathom-dir.md M4, sections 6 and 9). owned.json,
// leases.jsonl and scip/ moved to the state directory; risk.cue and
// learning.cue are shadowed once config.toml carries [risk] or [learning].
// The row names each stale file and never deletes one (I63), so it only ever
// warns.

import (
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/statedir"
	"github.com/geoffmilleraz/fathom/internal/wiring"
)

func checkLegacyFiles(root string) doctorCheck {
	const name = "legacy_files"
	var stale []string
	if wiring.LedgerLegacyStale(root) {
		stale = append(stale, projectdir.Owned.Rel()+" (superseded by the copy in the state directory)")
	}
	if statedir.LegacyStale(root, projectdir.Leases) {
		stale = append(stale, projectdir.Leases.Rel()+" (superseded by the copy in the state directory)")
	}
	// scip/ is merged per index name, so only a legacy index the state
	// directory also holds is superseded; the rest are still read.
	for _, p := range statedir.ShadowedSCIP(root) {
		stale = append(stale, projectdir.SCIP.Rel(filepath.Base(p))+" (superseded by the copy in the state directory)")
	}
	if l, err := config.Load("", projectdir.Config.Path(root)); err == nil {
		for _, p := range l.ShadowedLegacy {
			rel := projectdir.Dir + "/" + p[strings.LastIndexAny(p, `/\`)+1:]
			stale = append(stale, rel+" (shadowed by its section in config.toml)")
		}
	}
	if len(stale) == 0 {
		return doctorCheck{Name: name, Status: doctorPass, Reason: "no superseded copies under .fathom/"}
	}
	return doctorCheck{
		Name: name, Status: doctorWarn,
		Reason: "superseded and no longer read: " + strings.Join(stale, "; "),
		Remedy: "delete the old copies when you are ready; fathom never deletes them",
	}
}
