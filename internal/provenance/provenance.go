// Package provenance records declared agent provenance in local git notes.
package provenance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/gitro"
)

// Fields are declarations, not authenticated identities. HostLabel is supplied
// by the operator; it is never inferred from the machine or git identity.
type Fields struct {
	Harness   string `json:"harness"`
	Model     string `json:"model"`
	HostLabel string `json:"host_label"`
	Session   string `json:"session"`
	Lane      string `json:"lane"`
}

// ProvenanceNote replaces the commit's note under refs/notes/fathom/provenance.
// An empty commit means HEAD. It leaves the target history unchanged and never
// pushes. It returns the resolved commit SHA on success. Missing fields are
// explicitly unknown; no clock or store is needed.
func ProvenanceNote(ctx context.Context, root, commit string, fields Fields) (string, error) {
	if commit == "" {
		commit = "HEAD"
	}
	resolved, err := gitro.Output(ctx, root, "rev-parse", "--verify", "--end-of-options", commit+"^{commit}")
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("resolve provenance commit %q: %w: %s", commit, err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("resolve provenance commit %q: %w", commit, err)
	}
	for _, field := range []*string{&fields.Harness, &fields.Model, &fields.HostLabel, &fields.Session, &fields.Lane} {
		if strings.TrimSpace(*field) == "" {
			*field = "unknown"
		}
	}
	sha := strings.TrimSpace(string(resolved))
	var note bytes.Buffer
	if err := json.NewEncoder(&note).Encode(fields); err != nil {
		return "", fmt.Errorf("encode provenance note: %w", err)
	}
	// This command intentionally writes the notes ref. Reuse gitro's no-fetch,
	// no-prompt environment, but never route a mutation through its read API.
	cmd := exec.CommandContext(ctx, "git", "-c", "maintenance.auto=false", "-c", "gc.auto=0",
		"notes", "--ref=fathom/provenance", "add", "-f", "-F", "-", sha)
	cmd.Dir = root
	cmd.Env = gitro.Env()
	cmd.Stdin = &note
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("write provenance note: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return sha, nil
}
