package gatereceipt

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Check accepts only a clean checkout and an exact tree/command identity,
// with the original successful log beside the receipt. Receipts are unsigned
// local evidence; this checks integrity, not the identity of their author.
func Check(ctx context.Context, o Options, ref string) (string, error) {
	if err := clean(ctx, o.Root); err != nil {
		return "", err
	}
	if o.Command == "" {
		o.Command = "validate"
	}
	if ref == "" {
		ref = "HEAD"
	}
	tree, err := git(ctx, o.Root, "rev-parse", "--verify", "--end-of-options", ref+"^{tree}")
	if err != nil {
		return "", err
	}
	ident, err := identity(o)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(o.Dir)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("missing receipt for tree %s", tree)
	}
	if err != nil {
		return "", err
	}
	reason := "missing receipt for tree " + tree
	// ReadDir sorts names; candidates and diagnostics are deterministic.
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			reason = "receipt is not a regular file"
			continue
		}
		b, err := os.ReadFile(filepath.Join(o.Dir, entry.Name()))
		if err != nil {
			reason = "receipt unreadable"
			continue
		}
		var r Receipt
		if err := json.Unmarshal(b, &r); err != nil {
			reason = "invalid receipt JSON"
			continue
		}
		if r.Tree != tree {
			continue
		}
		if r.Command != o.Command || r.CommandIdentity != ident {
			reason = "command identity differs"
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if digest(b) != id {
			reason = "receipt digest mismatch"
			continue
		}
		if r.Version != 1 || r.Dirty || r.Exit != 0 {
			reason = "receipt records a dirty tree or unsuccessful gate"
			continue
		}
		logPath := filepath.Join(o.Dir, id+".log")
		logInfo, err := os.Lstat(logPath)
		if err != nil {
			reason = "log missing or unreadable"
			continue
		}
		if !logInfo.Mode().IsRegular() {
			reason = "log is not a regular file"
			continue
		}
		log, err := os.ReadFile(logPath)
		if err != nil {
			reason = "log missing or unreadable"
			continue
		}
		if int64(len(log)) != r.LogBytes || digest(log) != r.LogSHA256 {
			reason = "log digest mismatch"
			continue
		}
		exit, start, end, source, err := logResult(log, o.Command, time.Time{})
		if err != nil || exit != 0 {
			reason = "log has no final success line"
			continue
		}
		if source == "log" && (r.TimeSource != source || r.Start != start || r.End != end) {
			reason = "log times differ"
			continue
		}
		// The checkout must still be clean at acceptance, not just before reading files.
		if err := clean(ctx, o.Root); err != nil {
			return "", err
		}
		current, err := git(ctx, o.Root, "rev-parse", "--verify", "--end-of-options", ref+"^{tree}")
		if err != nil {
			return "", err
		}
		if current != tree {
			return "", fmt.Errorf("tree changed while checking receipt")
		}
		return id, nil
	}
	return "", fmt.Errorf("%s", reason)
}
