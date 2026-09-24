package main

// doctor_certifications.go — TC-14: doctor row "certifications". Unlike
// most rows (checkObligationsServed and friends), this one reads the
// repo's OWN local store directly rather than the daemon: the
// certifications layer is compiled by `fathom ingest`/`fathom tests
// ingest` into workspace/db/fathom.sqlite (the SAME default store path
// every store-reading verb falls back to, defaultpaths.go), and doctor
// diagnoses that file read-only. A store that was never created (no
// ingest has run against it) is treated exactly like "no manifests
// found" — os.Stat first, so an absent file is a named skip rather than
// store.Open silently creating an empty one (doctor never writes).
//
// Never-FAIL doctrine (EP-11, doctor_intent.go's own comment): a repo
// that has not adopted test certifications is not broken. This row's
// worst verdict is warn.
import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// certificationsStorePath is workspace/db/fathom.sqlite under root — the
// SAME relative location defaultStorePath() resolves to for a command
// run from inside root, computed against doctor's own --path/root instead
// of the process cwd (doctor's root and cwd may differ).
func certificationsStorePath(root string) string {
	return filepath.Join(root, "workspace", "db", "fathom.sqlite")
}

// certificationsDoctorService resolves the service identity this row
// reads: explicitSvc when --service was given, else the SAME first-
// candidate guess cmdDoctor's own header uses (serviceCandidates(root)).
func certificationsDoctorService(root, explicitSvc string) string {
	if explicitSvc != "" {
		return explicitSvc
	}
	if cands := serviceCandidates(root); len(cands) > 0 {
		return strings.ToLower(cands[0].name)
	}
	return ""
}

// checkCertifications reports the certifications layer's own declared/
// live/gap counts and the case-to-symbol join rate (TestSuiteStatuses'
// own fact) for service, read from the store at storePath.
func checkCertifications(storePath, service string) doctorCheck {
	const name = "certifications"

	if _, err := os.Stat(storePath); err != nil {
		return doctorCheck{
			Name: name, Status: doctorSkipped,
			Reason: "no manifests found — no store at " + storePath,
			Remedy: "fathom tests ingest --manifest <dir> --results <file|dir>",
		}
	}
	st, err := store.Open(storePath)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "no manifests found: could not open store: " + err.Error()}
	}
	defer st.Close()
	ctx := context.Background()

	statuses, err := st.TestSuiteStatuses(ctx, service)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "no manifests found: " + err.Error()}
	}
	if len(statuses) == 0 {
		return doctorCheck{
			Name: name, Status: doctorSkipped,
			Reason: "no manifests found — nothing declared",
			Remedy: "fathom tests ingest --manifest <dir> --results <file|dir>",
		}
	}

	casesTotal, casesJoined := 0, 0
	for _, s := range statuses {
		casesTotal += s.CasesTotal
		casesJoined += s.CasesJoined
	}
	joinRate := 0.0
	if casesTotal > 0 {
		joinRate = float64(casesJoined) / float64(casesTotal)
	}

	claims, err := st.ClaimsForService(ctx, service, certify.Layer)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorWarn, Reason: fmt.Sprintf("suites declared but could not read the certifications layer: %v", err)}
	}
	declared := map[string]bool{}
	live := map[string]bool{}
	for _, r := range certify.Records(claims) {
		declared[r.Subject] = true
		if r.State == string(certify.StateLive) {
			live[r.Subject] = true
		}
	}
	gaps := len(declared) - len(live)

	reason := fmt.Sprintf("%d declared, %d live, %d gap(s), joined %d of %d cases (%.0f%%)",
		len(declared), len(live), gaps, casesJoined, casesTotal, joinRate*100)
	if len(declared) == 0 {
		return doctorCheck{
			Name: name, Status: doctorWarn,
			Reason: "suites declared but no certifications compiled — " + reason,
			Remedy: "add a `// certifies:` marker above a Test* function, or a #Certifies entry in tests/*.cue",
		}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
}
