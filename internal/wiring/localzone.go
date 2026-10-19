package wiring

import (
	"strings"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// localZoneIgnoreFile is the repo-relative .gitignore that keeps the local
// zone of .fathom/ out of version control.
const localZoneIgnoreFile = projectdir.Dir + "/.gitignore"

// localZoneIgnoreVersion is the template version of localZoneIgnoreRegion.
const localZoneIgnoreVersion = "v1"

// localZoneIgnoreRegion is the wiring-owned region of .fathom/.gitignore
// naming the files only this machine owns: the wiring ledger, the leases,
// imported SCIP indexes, the receipts log and the retired memories file.
// It rides the same plan, apply, receipt and undo path as every other
// region, so a hand-written line outside the markers is never touched.
func localZoneIgnoreRegion() Region {
	lines := []string{
		string(projectdir.Owned),
		string(projectdir.Leases),
		string(projectdir.SCIP) + "/",
		receiptsFileName,
		string(projectdir.Memories),
	}
	return Region{
		File:           localZoneIgnoreFile,
		Syntax:         HashMarkers,
		Version:        localZoneIgnoreVersion,
		Body:           strings.Join(lines, "\n"),
		AdoptUntracked: true,
	}
}

// LocalZoneIgnoreRegion is the region `fathom init` and `fleet reconcile`
// add to their wiring spec.
func LocalZoneIgnoreRegion() Region { return localZoneIgnoreRegion() }
