package main

// doctor_serviceroots.go — EW-20 (O127): `fathom doctor` names a registered
// service whose recorded root no longer exists, with the ingest call that
// re-points it.
//
// O127: a worker re-rooted the shared daemon's own fathom service onto its
// scratch clone (remap:true) and deleted the clone; every fathom answer in
// the orchestrating session then resolved against a tree that was gone and
// the SessionStart hook reported the index state unknown. Nothing named the
// cause. This row reads list_services' per-service root and stats it.
//
//   - A root that does not exist is named with the call that re-points its
//     service. It is a FAIL when the service is this repository's own (the
//     index this repo's edits and verifies read is a deleted tree) and a WARN
//     for another repository's.
//   - A root that exists but cannot be read (a wedged external volume answers
//     EPERM) is not a root that is gone: it is named as unreadable with no
//     re-point, because re-pointing a service whose root merely cannot be
//     read would move it off a tree that is fine (I38).
//   - A service the daemon reports no root for is not judged; when no service
//     reports one the row is skipped, never a guessed pass.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"sort"
	"strings"
	"syscall"
)

// serviceRootRow is one list_services row's root.
type serviceRootRow struct {
	Service string `json:"service"`
	Path    string `json:"path"`
}

// checkServiceRoots stats every registered service's root.
func checkServiceRoots(hc *http.Client, baseURL, root, explicitSvc string, daemonUp bool) doctorCheck {
	const name = "service_roots"
	if !daemonUp {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "daemon not reachable — the registered service roots are the daemon's own fact and cannot be read from here", Remedy: daemonRestartHint("")}
	}
	inner, err := callMCPTool(hc, baseURL, "list_services", map[string]any{})
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("list_services did not answer: %v", err)}
	}
	var payload struct {
		Services []serviceRootRow `json:"services"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("list_services returned an unreadable payload: %v", err)}
	}

	own := map[string]bool{}
	if explicitSvc != "" {
		own[strings.ToLower(explicitSvc)] = true
	}
	for _, c := range serviceCandidates(root) {
		own[strings.ToLower(c.name)] = true
	}

	var (
		checked          int
		gone, unreadable []serviceRootRow
		ownGone          bool
	)
	for _, s := range payload.Services {
		if s.Path == "" {
			continue
		}
		checked++
		_, serr := os.Stat(s.Path)
		switch {
		case serr == nil:
		case rootIsGone(serr):
			gone = append(gone, s)
			if own[strings.ToLower(s.Service)] {
				ownGone = true
			}
		default:
			unreadable = append(unreadable, s)
		}
	}
	if checked == 0 {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "list_services names no root for any service, so none can be checked"}
	}
	if len(gone) == 0 && len(unreadable) == 0 {
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("all %d registered service root(s) exist", checked)}
	}

	sort.Slice(gone, func(i, j int) bool { return gone[i].Service < gone[j].Service })
	sort.Slice(unreadable, func(i, j int) bool { return unreadable[i].Service < unreadable[j].Service })
	var reasons, remedies []string
	for _, s := range gone {
		reasons = append(reasons, fmt.Sprintf("service %q is rooted at %s, which no longer exists — its answers resolve against a tree that is gone", s.Service, s.Path))
		checkout := "<absolute path of its checkout>"
		if own[strings.ToLower(s.Service)] {
			checkout = root
		}
		remedies = append(remedies, fmt.Sprintf(`re-point %q with the daemon's ingest tool: {"service":%q,"path":%q,"remap":true} (remap_from is not needed while the old root is gone)`, s.Service, s.Service, checkout))
	}
	for _, s := range unreadable {
		reasons = append(reasons, fmt.Sprintf("service %q is rooted at %s, which exists but cannot be read (unreadable, not gone)", s.Service, s.Path))
	}
	if len(unreadable) > 0 {
		remedies = append(remedies, "an unreadable root is not a missing one — do not re-point it; on an external volume run `tccutil reset SystemPolicyRemovableVolumes com.mitchellh.ghostty` and retry")
	}
	status := doctorWarn
	if ownGone {
		status = doctorFail
	}
	return doctorCheck{Name: name, Status: status, Reason: strings.Join(reasons, "; "), Remedy: strings.Join(remedies, "; ")}
}

// rootIsGone reports whether a stat error means the path does not exist: not
// found, or a parent that is not a directory. Any other error (permission,
// an I/O failure, a wedged volume) leaves the root's existence unknown.
func rootIsGone(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}
