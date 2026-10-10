package main

// calllogidentity.go — J5 (app-kit O25, O15/O17/O19; the dogfood journal's
// P0 "one service/checkout/session/log identity"). `fathom instrument
// --adoption`, `fathom research report` and `fathom flywheel` each picked a
// call log by a different rule: instrument/research_report resolved a
// CWD-relative default (env, then [daemon].call_log in the CALLER's own
// .fathom/config.toml, then the daemon's literal default for the caller's
// cwd) regardless of any --service argument; flywheel didn't even accept
// one, and separately fell back to resolveCallLogCandidates rooted at a
// hardcoded "." rather than any caller-supplied root. One session's --service
// app-kit call could read a scratch clone's log while another, run moments
// later against the same repo, reported app-kit's real log absent.
//
// resolveServiceCallLog is the one algorithm all three now share, in the
// exact order named:
//
//  1. The named service's recorded corpus path (the daemon's list_services,
//     via fetchServices) — only when a service is named and the daemon can
//     both be reached and already knows it.
//  2. The daemon's own literal default call-log path FOR THAT ROOT
//     (callLogPathFor — state-dir- and git-toplevel-aware). A repository's
//     own [daemon].call_log is never consulted: SEC-2 drops it at Load.
//
// FATHOM_CALL_LOG always wins outright, exactly as configuredCallLogPath
// already treats it — root-independent by design (an explicit override
// names its own location). When no service is named, or the daemon cannot
// resolve it there (down, or genuinely unknown), the pre-existing
// CWD-relative resolution (resolveCallLogPath) is the answer — unchanged
// for the common "run from the repo you mean, name no service" case.
import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// resolveServiceCallLog returns the call-log path every identity-aware
// instrument should read for service, plus the service's own resolved
// corpus root (empty when resolution fell back to the CWD-relative
// default — nothing service-specific was ever consulted).
func resolveServiceCallLog(hc *http.Client, baseURL, service string) (path, resolvedRoot string) {
	if p := os.Getenv("FATHOM_CALL_LOG"); p != "" {
		return p, ""
	}
	if service != "" {
		if svcs, err := fetchServices(hc, baseURL); err == nil {
			if info, ok := svcs[strings.ToLower(service)]; ok && info.path != "" && filepath.IsAbs(info.path) {
				root := info.path
				return callLogPathFor(root), root
			}
		}
	}
	return resolveCallLogPath(), ""
}

// callLogIdentityLine (J5) is the one headline every identity-aware
// instrument renders when its resolved call log cannot be read at all
// (absent or unreadable) — "unknown", never a bare 0, so a missing log
// never LOOKS like a measured-empty one.
func callLogIdentityLine(path string, err error) string {
	errno := "no such file"
	if err != nil && !os.IsNotExist(err) {
		errno = err.Error()
	}
	return "call log: unknown (" + path + ": " + errno + ")"
}
