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
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/gitro"
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
	// Provenance (schema 2, PC-2): which daemon binary produced the
	// verdicts, which watermark the store carried, and which CLI binary
	// wrote the row. "" is an honest unknown (store mode, a v1 daemon).
	DaemonCommit string `json:"daemon_commit,omitempty"`
	StoreCommit  string `json:"store_commit,omitempty"`
	BinaryCommit string `json:"binary_commit,omitempty"`
	// Origin names which path ran the verify (vision 26 §4, PC-22):
	// "cli" (`fathom verify`, incl. the pre-push gate), "stop" (the Stop
	// hook), "claim-moment" (the PostToolUse edit hook on a newly checked
	// [x]). Additive; rows written before the field are origin-unknown,
	// never backfilled as cli.
	Origin string `json:"origin,omitempty"`
	// Digest (schema 3, PE-1) names the plan's per-row verdicts at this
	// reading — every schema-3 row carries it. Rows is those verdicts,
	// written only when Digest differs from the plan's previous reading, so
	// a gate re-run on an unchanged tree appends counts without the array
	// (design §3's size bound). v1/v2 rows have neither.
	Digest string                   `json:"digest,omitempty"`
	Rows   []verifyLedgerRowVerdict `json:"rows,omitempty"`
}

// verifyLedgerRowVerdict is one plan row at one reading (schema 3). State
// is open, claimed-proven, claimed-unproven, unknown or dispositioned;
// Criterion is the first failing (else undecidable) criterion of a claimed
// row the gate did not prove; Weight is the row's `weight:` (1 unset).
type verifyLedgerRowVerdict struct {
	ID        string `json:"id"`
	State     string `json:"state"`
	Criterion string `json:"criterion,omitempty"`
	Weight    int    `json:"weight"`
}

// verifyLedgerSchemaVersion: 1 = counts + tree commit; 2 adds the
// provenance triple and the origin above; 3 adds the row digest and the
// per-row verdicts (additive — v1 and v2 rows still parse).
const verifyLedgerSchemaVersion = 3

// The three verify origins. PC-26's pre-registered reading counts rows
// with verifyOriginClaimMoment whose Failing > 0.
const (
	verifyOriginCLI         = "cli"
	verifyOriginStop        = "stop"
	verifyOriginClaimMoment = "claim-moment"
)

// gitHeadShort returns the short HEAD sha of root, or "" — absence over
// failure, this is telemetry.
func gitHeadShort(root string) string {
	//fathom:tenancy-allow exec-noctx one-shot CLI verb with no request context; the git/child call ends with the process
	out, err := exec.Command("git", "-C", root, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// repoHeadNoFork is root's HEAD commit read from the repository's files
// (gitro.Head), or "" — never a git process: the console reads it on
// request paths (RT-115, design §14 "fork no git per service").
func repoHeadNoFork(root string) string {
	h, err := gitro.Head(context.Background(), root)
	if err != nil {
		return ""
	}
	return h
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

func appendVerifyLedger(planFiles []string, service string, reports []verify.Report, prov verifyProvenance) {
	root, _ := os.Getwd()
	appendVerifyLedgerFrom(verifyOriginCLI, root, planFiles, service, reports, prov)
}

// appendVerifyLedgerFrom is appendVerifyLedger with an explicit origin and
// repo root — the hooks run from wherever the harness put them, so the
// root is the project dir they resolved, never their cwd.
func appendVerifyLedgerFrom(origin, root string, planFiles []string, service string, reports []verify.Report, prov verifyProvenance) {
	path := verifyLedgerPath()
	if path == "" || len(planFiles) != len(reports) {
		return
	}
	commit := gitHeadShort(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	prev := lastLedgerDigests(f, verifyLedgerDigestTail)
	enc := json.NewEncoder(f)
	at := time.Now().UTC().Format(time.RFC3339)
	for i, rep := range reports {
		row := verifyLedgerRow{
			SchemaVersion: verifyLedgerSchemaVersion, At: at, RepoRoot: root, Service: service,
			Plan: planFiles[i], Commit: commit, Origin: origin,
			DaemonCommit: prov.DaemonCommit, StoreCommit: prov.StoreCommit, BinaryCommit: prov.BinaryCommit,
		}
		appendVerifyLedgerRows(&row, rep, prev)
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
	rows, _ := readVerifyLedgerErr(verifyLedgerPath())
	return rows
}

// readVerifyLedgerErr is readVerifyLedger over path with the read's error:
// os.ErrNotExist (wrapped) when there is no ledger, the open or read
// failure when it exists and cannot be read — a reader that must tell an
// absent ledger from an unreadable one (the catches timeline) uses this.
func readVerifyLedgerErr(path string) ([]verifyLedgerRow, error) {
	if path == "" {
		return nil, os.ErrNotExist
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
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
	return out, nil
}
