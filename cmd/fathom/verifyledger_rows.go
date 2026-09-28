package main

// verifyledger_rows.go — PE-1 (docs/design/progress-and-economics.md §3):
// the verify ledger's schema 3. Each reading carries a digest of the plan's
// per-row verdicts, and the verdicts themselves only when that digest
// changed since the plan's previous reading — the proof history the
// Progress views project (internal/console ProofHistory) without a second
// ledger file (PE-X3).

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"sort"
	"strconv"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// The schema-3 row states (design §3).
const (
	ledgerRowOpen            = "open"
	ledgerRowClaimedProven   = "claimed-proven"
	ledgerRowClaimedUnproven = "claimed-unproven"
	ledgerRowUnknown         = "unknown"
	ledgerRowDispositioned   = "dispositioned"
)

// verifyLedgerDigestTail bounds the read that finds each plan's previous
// digest: the newest bytes only. A plan whose last reading is older than
// the tail writes its array again — a bigger row, never a lost one.
const verifyLedgerDigestTail = 1 << 20

// maxLedgerCriterion caps a row's criterion text.
const maxLedgerCriterion = 200

// ledgerRowVerdicts maps a report's verdicts to schema-3 rows, sorted by id.
func ledgerRowVerdicts(rep verify.Report) []verifyLedgerRowVerdict {
	out := make([]verifyLedgerRowVerdict, 0, len(rep.Tasks))
	for _, tv := range rep.Tasks {
		r := verifyLedgerRowVerdict{ID: tv.TaskID, Weight: tv.Weight}
		if r.Weight < 1 {
			r.Weight = 1
		}
		switch {
		case tv.Skipped:
			r.State = ledgerRowDispositioned
		case !tv.Claimed:
			r.State = ledgerRowOpen
		case tv.Status == verify.Pass:
			r.State = ledgerRowClaimedProven
		case tv.Status == verify.Fail:
			r.State = ledgerRowClaimedUnproven
			r.Criterion = firstCriterion(tv.Criteria, verify.Fail)
		default:
			r.State = ledgerRowUnknown
			r.Criterion = firstCriterion(tv.Criteria, verify.Unknown)
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// firstCriterion names the first criterion with status st, "name: detail".
func firstCriterion(cs []verify.Criterion, st verify.CriterionStatus) string {
	for _, c := range cs {
		if c.Status != st {
			continue
		}
		s := c.Name
		if c.Detail != "" {
			s += ": " + c.Detail
		}
		if len(s) > maxLedgerCriterion {
			s = s[:maxLedgerCriterion-1] + "…"
		}
		return s
	}
	return ""
}

// ledgerRowsDigest is a short, order-independent digest of the rows (they
// are sorted by ledgerRowVerdicts).
func ledgerRowsDigest(rows []verifyLedgerRowVerdict) string {
	h := sha256.New()
	for _, r := range rows {
		_, _ = io.WriteString(h, r.ID+"\x00"+r.State+"\x00"+r.Criterion+"\x00"+strconv.Itoa(r.Weight)+"\n")
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// ledgerPlanKey keys a plan's readings: its service, root and plan path.
func ledgerPlanKey(service, root, plan string) string {
	return service + "\x00" + root + "\x00" + plan
}

// appendVerifyLedgerRows stamps row with the report's digest and, when the
// digest differs from the plan's previous reading, its per-row verdicts.
func appendVerifyLedgerRows(row *verifyLedgerRow, rep verify.Report, prev map[string]string) {
	rows := ledgerRowVerdicts(rep)
	row.Digest = ledgerRowsDigest(rows)
	key := ledgerPlanKey(row.Service, row.RepoRoot, row.Plan)
	if prev[key] != row.Digest {
		row.Rows = rows
	}
	prev[key] = row.Digest
}

// lastLedgerDigests reads the newest tail bytes of the open ledger and
// returns each plan's most recent digest. Best-effort: an unreadable tail
// is no previous digests (every plan writes its array).
func lastLedgerDigests(f *os.File, tail int64) map[string]string {
	out := map[string]string{}
	fi, err := f.Stat()
	if err != nil || fi.Size() == 0 {
		return out
	}
	start := fi.Size() - tail
	if start < 0 {
		start = 0
	}
	rf, err := os.Open(f.Name())
	if err != nil {
		return out
	}
	defer rf.Close()
	buf := make([]byte, fi.Size()-start)
	if _, err := rf.ReadAt(buf, start); err != nil && err != io.EOF {
		return out
	}
	if start > 0 {
		// Drop the partial first line.
		if i := bytes.IndexByte(buf, '\n'); i >= 0 {
			buf = buf[i+1:]
		}
	}
	for _, line := range bytes.Split(buf, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var r struct {
			Service  string `json:"service"`
			RepoRoot string `json:"repo_root"`
			Plan     string `json:"plan"`
			Digest   string `json:"digest"`
		}
		if json.Unmarshal(line, &r) != nil {
			continue
		}
		// A v1/v2 reading after a digest resets it: its rows are unknown.
		out[ledgerPlanKey(r.Service, r.RepoRoot, r.Plan)] = r.Digest
	}
	return out
}
