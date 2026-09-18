package main

// findings_outbox.go — FL-22 (docs/vision/30-findings-loop.md §3.7,
// ADR-040 decision 7): the outbox sync. With `[findings] outbox` set in
// .fathom/config.toml, the session hook (SessionStart and SubagentStart —
// see sessionhook.go and sessionhook_events.go) imports a changed outbox
// once per session using a sha256 digest cursor under the repo's state
// dir, and reports the receipt (or the failure reason) in the brief. No
// network, no daemon dependency — `findings import` is a store-local CLI
// subcommand, run as a subprocess of the fathom binary itself.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/statedir"
)

// findingsOutboxCursorFile is the cursor file name under the repo's state
// dir (internal/statedir.Resolve) — its content is the last imported
// outbox's hex sha256 digest, nothing else.
const findingsOutboxCursorFile = "findings-outbox.cursor"

// findingsImportTimeout bounds the `findings import` subprocess: long
// enough for a real projection file, short enough that a hung subprocess
// never stalls session start past a hook's own budget.
const findingsImportTimeout = 10 * time.Second

// findingsImportRunner runs `<exe> findings import --store STORE --service
// SVC --subject SUBJECT --file OUTBOX --json` and reports its combined
// stdout+stderr, exit code (-1 for a timeout or a failure to even start
// the process — never a real CLI exit code), and any error a real exit
// code cannot represent. A package var, not a hardcoded exec.Command call,
// so a test can inject a fake runner (the same injection style every other
// hook test uses for its own HTTP client) — a test must never exec a real
// fathom binary.
var findingsImportRunner = func(exe, store, service, subject, outbox string) (stdout []byte, exitCode int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), findingsImportTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "findings", "import",
		"--store", store, "--service", service, "--subject", subject, "--file", outbox, "--json")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	runErr := cmd.Run()
	if ctx.Err() != nil {
		return out.Bytes(), -1, ctx.Err() // timed out — never a real exit code
	}
	if runErr == nil {
		return out.Bytes(), 0, nil
	}
	var ee *exec.ExitError
	if errors.As(runErr, &ee) {
		return out.Bytes(), ee.ExitCode(), nil
	}
	return out.Bytes(), -1, runErr // could not even start the process
}

// findingsImportReceipt is the subset of `findings import --json`'s
// receipt this hook reports — the CLI's full schema (schema_version,
// truncated, cursors, dry_run, …) belongs to the concurrent import wave
// (FL-4/FL-5); this wave only consumes delivered/duplicate/stale/rejected.
type findingsImportReceipt struct {
	Delivered int `json:"delivered"`
	Duplicate int `json:"duplicate"`
	Stale     int `json:"stale"`
	Rejected  []struct {
		Index     int    `json:"index"`
		FindingID string `json:"finding_id"`
		Reason    string `json:"reason"`
	} `json:"rejected"`
}

// syncFindingsOutbox implements FL-22: when [findings] outbox names a
// repo-relative file (read from projDir's .fathom/config.toml), sha256 it
// and compare with the digest cursor under the repo's state dir
// (statedir.Resolve). On a change, run `findings import` against the
// service resolved for projDir (via the daemon at url) and [daemon] store
// from the same config, report the receipt as one brief line, and advance
// the cursor — but ONLY on exit 0 or 1 (a rejection still advances the
// cursor, so the SAME bad outbox is never retried every session); a
// timeout or crash leaves the cursor untouched, so a transient failure is
// retried next session. Returns "" — nothing to report — when no outbox is
// configured, the digest is unchanged since the last sync (this IS the
// "never more than one attempt per session" gate: whichever hook fires
// first writes the cursor, and every later call in the same session then
// sees no change), or the outbox file does not exist yet.
func syncFindingsOutbox(url, projDir string) string {
	if projDir == "" {
		return ""
	}
	loaded, cfgErr := config.Load("", filepath.Join(projDir, ".fathom", "config.toml"))
	if cfgErr != nil || loaded.Config.Findings.Outbox == "" {
		return ""
	}
	outboxRel := loaded.Config.Findings.Outbox
	outboxAbs := outboxRel
	if !filepath.IsAbs(outboxAbs) {
		outboxAbs = filepath.Join(projDir, outboxRel)
	}
	raw, rerr := os.ReadFile(outboxAbs)
	if rerr != nil {
		return "" // no outbox file on disk yet — nothing to sync, never an error line
	}
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])

	cursorPath := filepath.Join(statedir.Resolve(projDir), findingsOutboxCursorFile)
	if prior, perr := os.ReadFile(cursorPath); perr == nil && strings.TrimSpace(string(prior)) == digest {
		return "" // unchanged since the last sync
	}

	if loaded.Config.Daemon.Store == "" {
		return "findings outbox: sync failed — no store configured ([daemon] store in .fathom/config.toml)"
	}
	hc := &http.Client{Timeout: 2 * time.Second}
	svc, _, ok := indexedServiceFor(hc, url, projDir)
	if !ok {
		return "findings outbox: sync failed — this repo is not an indexed service"
	}
	exe, eerr := os.Executable()
	if eerr != nil {
		return "findings outbox: sync failed — cannot resolve the fathom executable"
	}

	stdout, code, runErr := findingsImportRunner(exe, loaded.Config.Daemon.Store, svc, loaded.Config.Findings.Subject, outboxAbs)
	if code < 0 {
		reason := "timed out"
		if runErr != nil && !errors.Is(runErr, context.DeadlineExceeded) {
			reason = runErr.Error()
		}
		return "findings outbox: sync failed — " + reason // cursor NOT advanced — retry next session
	}
	if code != 0 && code != 1 {
		return fmt.Sprintf("findings outbox: sync failed — exit %d", code) // cursor NOT advanced
	}

	// Cursor advances on exit 0 OR 1 — a rejection is still a completed,
	// reported sync, never silently retried forever.
	if merr := os.MkdirAll(filepath.Dir(cursorPath), 0o755); merr == nil {
		_ = os.WriteFile(cursorPath, []byte(digest), 0o644)
	}

	var receipt findingsImportReceipt
	if err := json.Unmarshal(stdout, &receipt); err != nil {
		return "findings outbox: sync failed — could not parse the import receipt"
	}
	return fmt.Sprintf("findings outbox: imported %d (duplicate %d, stale %d, rejected %d)",
		receipt.Delivered, receipt.Duplicate, receipt.Stale, len(receipt.Rejected))
}
