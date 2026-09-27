package main

// verifyprovenance.go — PC-2 / PC-3 (docs/plans/proof-credibility.md,
// vision 26 §1.1, O20): a verify verdict is a function of the store, and
// the store is a function of the binary that built it. Two stores for the
// same tree gave 70/5 and 75/0 on 2026-09-01 because the daemon predated
// HEAD. So every ledger row now records WHICH daemon and WHICH watermark
// produced it, and the CLI says so — once, on stderr — when the daemon's
// binary is not the binary running this command.
//
// The comparison is daemon-vs-THIS-BINARY, never daemon-vs-the-cwd's HEAD:
// in an adopter repo HEAD is a different project's history and the
// comparison would always "fail" (O5, O23).

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// daemonStatusz is the subset of /statusz a CLI reads for provenance. A v1
// daemon (no schema_version, no binary_commit) decodes to the zero stamp,
// which every reader treats as "unknown", never as fresh.
type daemonStatusz struct {
	SchemaVersion int    `json:"schema_version"`
	BinaryCommit  string `json:"binary_commit"`
	BinaryDirty   bool   `json:"binary_dirty"`
	BuiltAt       string `json:"built_at"`
	Services      []struct {
		Service       string `json:"service"`
		IndexedCommit string `json:"indexed_commit"`
	} `json:"services"`
}

// fetchDaemonBuild is the daemon binary's build stamp: inside the daemon,
// this process's own (the value its statusz would report, without building
// the full-fleet report — every service's meta, typed-edge status and
// compile runs — RT-115); otherwise /statusz's.
func fetchDaemonBuild(hc *http.Client, baseURL string) (StatuszBuildInfo, error) {
	if r := inProcessDaemon(baseURL); r != nil && r.statusz != nil {
		return statuszBuildInfo(), nil
	}
	st, err := fetchStatusz(hc, baseURL)
	return StatuszBuildInfo{BinaryCommit: st.BinaryCommit, BinaryDirty: st.BinaryDirty, BuiltAt: st.BuiltAt}, err
}

// fetchStatusz GETs baseURL/statusz as JSON. Any failure is returned, not
// swallowed: the caller decides whether provenance is best-effort.
func fetchStatusz(hc *http.Client, baseURL string) (daemonStatusz, error) {
	var out daemonStatusz
	if r := inProcessDaemon(baseURL); r != nil && r.statusz != nil {
		// Inside the daemon: its own report, never a loopback (RT-115).
		raw, err := json.Marshal(r.statusz(context.Background()))
		if err != nil {
			return out, err
		}
		err = json.Unmarshal(raw, &out)
		return out, err
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/statusz?format=json", nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("GET %s/statusz: HTTP %d", baseURL, resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("parse /statusz: %w", err)
	}
	return out, nil
}

// stampString renders a build stamp the way internal/buildinfo.Version
// does — the commit plus "-dirty" when built from a modified tree — so a
// ledger reader compares like with like (buildinfo.SameRevision ignores
// the suffix and prefix length).
func stampString(commit string, dirty bool) string {
	if commit == "" {
		return ""
	}
	if dirty {
		return commit + "-dirty"
	}
	return commit
}

// verifyProvenance is what one verify run knew about where its verdicts
// came from. Empty strings are honest unknowns, and Note says why.
type verifyProvenance struct {
	// DaemonCommit is the daemon binary's stamp (HTTP mode); "" when the
	// run was store-mode, the daemon predates /statusz v2, or the probe
	// failed — DaemonKnown separates "old daemon" from "not asked".
	DaemonCommit string
	DaemonKnown  bool
	// StoreCommit is the service's ingest watermark (service_meta
	// .indexed_commit) in whichever store answered.
	StoreCommit string
	// BinaryCommit is THIS process's stamp — the comparator for the notice.
	BinaryCommit string
	Note         string
}

// verifyLedgerProvenance gathers the provenance for one verify run: over
// the daemon (baseURL) when storePath is empty, else from the store file
// itself. Best-effort by construction — a failed probe records the reason
// and never changes the gate.
func verifyLedgerProvenance(hc *http.Client, baseURL, storePath, service string) verifyProvenance {
	p := verifyProvenance{BinaryCommit: buildinfo.Version()}
	if storePath != "" {
		p.Note = "store mode: no daemon in the loop"
		c, err := client.NewConstantClient(client.ModeLocal, storePath)
		if err != nil {
			p.Note += "; store watermark unreadable: " + err.Error()
			return p
		}
		defer c.Close()
		if _, commit, found, err := c.ServiceMeta(context.Background(), service); err == nil && found {
			p.StoreCommit = commit
		} else {
			p.Note += "; no service_meta watermark for " + service
		}
		return p
	}
	st, err := fetchStatusz(hc, baseURL)
	if err != nil {
		p.Note = "daemon build stamp unknown: " + err.Error()
		return p
	}
	if st.SchemaVersion < statuszSchemaVersion || st.BinaryCommit == "" {
		p.Note = fmt.Sprintf("daemon build stamp unknown: its /statusz is schema v%d without binary_commit (a daemon started from a build predating this field, or built without -buildvcs)", st.SchemaVersion)
	} else {
		p.DaemonKnown = true
		p.DaemonCommit = stampString(st.BinaryCommit, st.BinaryDirty)
	}
	for _, s := range st.Services {
		if strings.EqualFold(s.Service, service) {
			p.StoreCommit = s.IndexedCommit
		}
	}
	return p
}

// provenanceNotice is the ONE line `fathom verify` prints (stderr) when
// the verdicts came from a daemon whose binary is not this binary — or
// whose binary it could not learn. "" when the daemon matches, or when no
// daemon was involved (store mode). Never compared to the repo's HEAD.
func provenanceNotice(p verifyProvenance) string {
	if p.Note != "" && strings.HasPrefix(p.Note, "store mode") {
		return ""
	}
	if !p.DaemonKnown {
		return fmt.Sprintf("provenance: %s — this fathom binary is @ %s; the verdicts above came from whatever built the daemon's store (O20: a stale daemon fails and passes rows the tree does not)", p.Note, shortStamp(p.BinaryCommit))
	}
	if buildinfo.SameRevision(p.DaemonCommit, p.BinaryCommit) {
		return ""
	}
	return fmt.Sprintf("provenance: daemon binary @ %s ≠ this fathom binary @ %s — the store (watermark %s) was built and queried by the daemon's extractor; if the two builds differ in extraction, so do these verdicts (task deploy, or restart the daemon from the current build, then re-run)",
		shortStamp(p.DaemonCommit), shortStamp(p.BinaryCommit), shortStamp(p.StoreCommit))
}

func shortStamp(s string) string {
	if s == "" {
		return "unknown"
	}
	dirty := strings.HasSuffix(s, "-dirty")
	s = strings.TrimSuffix(s, "-dirty")
	if len(s) > 12 {
		s = s[:12]
	}
	if dirty {
		s += "-dirty"
	}
	return s
}
