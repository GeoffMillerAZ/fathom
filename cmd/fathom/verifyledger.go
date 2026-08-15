package main

// verifyledger.go — SD-1 (docs/plans/custodian.md): the verify ledger.
// Every `fathom verify` run appends one row per plan to
// ~/.fathom/verify-ledger.jsonl (user-central like the session ledger —
// gates run in every fleet repo; law 13: local, never shared). The ledger
// is what makes an SDD trend a measurement instead of a guess (SD-3
// renders nothing until it has data), and it is the recorded-run signal
// the flywheel's verify-gate loop reads (O30: git-native pre-push runs
// were invisible to the session-scoped instrumentation).
//
// Best-effort by construction: the gate's exit code must never depend on
// telemetry, so every failure here is swallowed.

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

type verifyLedgerRow struct {
	SchemaVersion int    `json:"schema_version"`
	At            string `json:"at"`
	RepoRoot      string `json:"repo_root"`
	Service       string `json:"service"`
	Plan          string `json:"plan"`
	Commit        string `json:"commit,omitempty"`
	Total         int    `json:"total"`
	Claimed       int    `json:"claimed"`
	Verified      int    `json:"verified"`
	Failing       int    `json:"failing"`
}

// gitHeadShort returns the short HEAD sha of root, or "" — absence over
// failure, this is telemetry.
func gitHeadShort(root string) string {
	out, err := exec.Command("git", "-C", root, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func verifyLedgerPath() string {
	if p := os.Getenv("FATHOM_VERIFY_LEDGER"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".fathom", "verify-ledger.jsonl")
}

func appendVerifyLedger(planFiles []string, service string, reports []verify.Report) {
	path := verifyLedgerPath()
	if path == "" || len(planFiles) != len(reports) {
		return
	}
	root, _ := os.Getwd()
	commit := gitHeadShort(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	at := time.Now().UTC().Format(time.RFC3339)
	for i, rep := range reports {
		row := verifyLedgerRow{
			SchemaVersion: 1, At: at, RepoRoot: root, Service: service,
			Plan: planFiles[i], Commit: commit,
		}
		for _, tv := range rep.Tasks {
			row.Total++
			if tv.Claimed && !tv.Skipped {
				row.Claimed++
				switch tv.Status {
				case verify.Pass:
					row.Verified++
				case verify.Fail:
					row.Failing++
				}
			}
		}
		_ = enc.Encode(row)
	}
}

// readVerifyLedger returns all parseable rows; absent file is zero rows,
// no error — a machine that has never gated is normal.
func readVerifyLedger() []verifyLedgerRow {
	path := verifyLedgerPath()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []verifyLedgerRow
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var r verifyLedgerRow
		if err := dec.Decode(&r); err != nil {
			break
		}
		out = append(out, r)
	}
	return out
}
