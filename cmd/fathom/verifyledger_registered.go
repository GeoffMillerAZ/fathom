package main

// verifyledger_registered.go — PE-24 (docs/design/progress-and-economics.md
// §11, observation O121): every reader of the verify ledger reads only rows
// whose repo_root is a registered service root (PE-23's ledgerroots rule:
// exact canonical equality; a worktree or a scratch clone is not the fleet's
// work) and names how many rows it ignored. The file itself is never
// rewritten by a reader; the writers (appendVerifyLedgerFrom, and the stop
// hook's tail read lastLedgerDigests) stay unfiltered.

import (
	"net/http"
	"time"

	"github.com/geoffmilleraz/fathom/internal/ledgerroots"
)

// verifyLedgerNoun names a verify-ledger row in an ignored phrase.
const verifyLedgerNoun = "verify-ledger row"

// verifyLedgerRegisteredRows keeps the rows whose repo_root is a registered
// service root under roots and returns what it ignored. An unread registry
// keeps every row and says why (I38).
func verifyLedgerRegisteredRows(rows []verifyLedgerRow, roots ledgerroots.Roots) ([]verifyLedgerRow, ledgerroots.Ignored) {
	f := roots.NewFilter()
	out := make([]verifyLedgerRow, 0, len(rows))
	for _, r := range rows {
		if f.Keep(r.RepoRoot) {
			out = append(out, r)
		}
	}
	return out, f.Ignored()
}

// readRegisteredVerifyLedger is readVerifyLedger kept to the registered
// roots.
func readRegisteredVerifyLedger(roots ledgerroots.Roots) ([]verifyLedgerRow, ledgerroots.Ignored) {
	return verifyLedgerRegisteredRows(readVerifyLedger(), roots)
}

// readRegisteredVerifyLedgerErr is readVerifyLedgerErr kept to the
// registered roots; the read's error is readVerifyLedgerErr's.
func readRegisteredVerifyLedgerErr(path string, roots ledgerroots.Roots) ([]verifyLedgerRow, ledgerroots.Ignored, error) {
	rows, err := readVerifyLedgerErr(path)
	if err != nil {
		return nil, ledgerroots.Ignored{}, err
	}
	kept, ign := verifyLedgerRegisteredRows(rows, roots)
	return kept, ign, nil
}

// withIgnored appends what a reader ignored to note ("note; phrase"), or
// returns note unchanged when nothing was ignored and the registry was read.
func withIgnored(note string, ign ledgerroots.Ignored, noun string) string {
	p := ign.Phrase(noun)
	switch {
	case p == "":
		return note
	case note == "":
		return p
	}
	return note + "; " + p
}

// cliLedgerRoots is a CLI verb's registered roots: the daemon at baseURL
// asked once, with the verb's own short timeout.
func cliLedgerRoots(baseURL string) ledgerroots.Roots {
	return registeredLedgerRoots(&http.Client{Timeout: 5 * time.Second}, baseURL)
}

// ledgerRoots is the registered roots the observatory model already read
// (its services' recorded paths): a console page filters with no second
// registry read. Unread, with the model's reason, when the model's
// registry read failed.
func (m observatoryModel) ledgerRoots() ledgerroots.Roots {
	if !m.registry.Read {
		note := m.registry.Note
		if note == "" {
			note = m.DegradedNote
		}
		if note == "" {
			note = "the service registry was not read"
		}
		return ledgerroots.Unread(note + " — no ledger row was checked against the registered roots")
	}
	paths := make([]string, 0, len(m.Services))
	for _, s := range m.Services {
		if s.Path != "" {
			paths = append(paths, s.Path)
		}
	}
	return ledgerroots.RegisteredRoots(paths)
}

// doctorLedgerRoots is the registered roots doctor's ledger rows read
// under. Inside a hook (doctor --brief, the harness Setup hook) doctor asks
// the daemon nothing more for them: the roots are unread, so no row is
// excluded, and the reason names the full run that filters.
func doctorLedgerRoots(hc *http.Client, baseURL string) ledgerroots.Roots {
	if activeHookGuard() != nil {
		return ledgerroots.Unread("doctor --brief asks the daemon nothing more for its ledger rows (the Setup hook's budget); `fathom doctor` reads them under the registered roots")
	}
	return registeredLedgerRoots(hc, baseURL)
}
