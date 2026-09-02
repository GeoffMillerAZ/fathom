package main

// doctor_provenance.go — PC-4 (docs/plans/proof-credibility.md, vision 26
// §1.1): `daemon_fresh` compares the DAEMON's build stamp (/statusz
// binary_commit, PC-1) to the binary running doctor. binary_fresh (doctor.go)
// can only know the local binary against its own source tree's HEAD and now
// says exactly that; this row is the one that answers O20's question — "is
// the store I am about to trust built by the code I think it is?"

import (
	"fmt"
	"net/http"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
)

// checkDaemonFresh: PASS when the daemon's binary_commit names the same
// revision as this binary; FAIL when it names another; SKIP with the
// reason when either side is unknown — a v1 daemon (no binary_commit), an
// unstamped binary, or an unreachable daemon. Never compares to the repo's
// HEAD: that comparison is wrong in every adopter repo (O5, O23) and is
// binary_fresh's, restricted to the fathom source tree.
func checkDaemonFresh(hc *http.Client, baseURL string, daemonUp bool) doctorCheck {
	const name = "daemon_fresh"
	remedy := "task deploy from the fathom checkout (signs, installs via temp+rename, restarts the daemon, asserts it serves HEAD) — never a raw cp over the running executable (O25)"
	if !daemonUp {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "daemon unreachable — cannot read its build stamp (see daemon_reachable)"}
	}
	st, err := fetchStatusz(hc, baseURL)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("could not read %s/statusz: %v", baseURL, err)}
	}
	if st.SchemaVersion < statuszSchemaVersion || st.BinaryCommit == "" {
		return doctorCheck{
			Name: name, Status: doctorSkipped,
			Reason: fmt.Sprintf("the daemon's /statusz is schema v%d and carries no binary_commit — it was started from a build predating this field (or built without -buildvcs); its store's provenance is unknown until it is restarted from a current build", st.SchemaVersion),
			Remedy: remedy,
		}
	}
	local, localDirty, ok := doctorBinaryRevision()
	if !ok {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("daemon binary @ %s; this binary carries no embedded VCS revision (built without -buildvcs) — compare `fathom version` by hand", shortSHA(st.BinaryCommit))}
	}
	daemonLabel := shortSHA(st.BinaryCommit)
	if st.BinaryDirty {
		daemonLabel += "-dirty"
	}
	localLabel := shortSHA(local)
	if localDirty {
		localLabel += "-dirty"
	}
	if buildinfo.SameRevision(st.BinaryCommit, local) {
		reason := fmt.Sprintf("daemon binary @ %s matches this binary @ %s", daemonLabel, localLabel)
		if st.BuiltAt != "" {
			reason += " (daemon built " + st.BuiltAt + ")"
		}
		if st.BinaryDirty || localDirty {
			reason += "; a -dirty stamp names the commit, not the exact code"
		}
		return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
	}
	return doctorCheck{
		Name: name, Status: doctorFail,
		Reason: fmt.Sprintf("daemon binary @ %s ≠ this binary @ %s — the shared store was built and is queried by the daemon's extractor, so verify verdicts and doctor's own daemon-backed rows reflect that build, not this one (O20)", daemonLabel, localLabel),
		Remedy: remedy,
	}
}
