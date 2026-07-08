// Package connector — GitHubConnector reads source files from a GitHub
// repository via the gh CLI.  The exec function is injectable so tests can
// substitute a fake runner without real network access.
package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// GitHubConnector satisfies the Connector SPI by reading files from a GitHub
// repository through the gh CLI.
//
// Discover lists all blob entries in the repository's default-branch tree via
// the GitHub Trees API (gh api repos/{repo}/git/trees/HEAD?recursive=1).
// Fetch reads a single file's raw content via gh api --field mediaType with
// the raw blob endpoint.
//
// The runner field is an injectable exec function (defaults to runGH, which
// delegates to os/exec) so that unit tests can inject a fake without real
// network access.
type GitHubConnector struct {
	repo   string                               // "org/repo" slug
	runner func(args ...string) ([]byte, error) // injectable; see runGH
}

// NewGitHubConnector constructs a GitHubConnector for the given "org/repo"
// slug. If runner is nil the default gh-CLI runner is used.
func NewGitHubConnector(repo string, runner func(args ...string) ([]byte, error)) *GitHubConnector {
	if runner == nil {
		runner = runGH
	}
	return &GitHubConnector{repo: repo, runner: runner}
}

// runGH is the default runner that delegates to the gh CLI via os/exec.
func runGH(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	return cmd.Output()
}

// ghTreeEntry represents one entry in the GitHub Trees API response.
type ghTreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"` // "blob" or "tree"
	SHA  string `json:"sha"`
}

// ghTreeResponse is the top-level response from the GitHub Trees API.
type ghTreeResponse struct {
	SHA       string        `json:"sha"`
	Tree      []ghTreeEntry `json:"tree"`
	Truncated bool          `json:"truncated"`
}

// Discover calls the GitHub Trees API to enumerate all blobs in the
// repository identified by svc.Repo (falling back to gc.repo when
// svc.Repo is empty).  It returns a deterministically sorted slice of
// SourceRefs covering every blob whose extension maps to a known SourceKind.
func (gc *GitHubConnector) Discover(_ context.Context, svc core.ServiceRef) ([]core.SourceRef, error) {
	repo := gc.repo
	if svc.Repo != "" {
		repo = svc.Repo
	}

	// Use the recursive Trees API so we get the full tree in one call.
	apiPath := fmt.Sprintf("repos/%s/git/trees/HEAD", repo)
	data, err := gc.runner("api", apiPath, "--field", "recursive=1")
	if err != nil {
		return nil, fmt.Errorf("GitHubConnector.Discover: gh api %s: %w", apiPath, err)
	}

	var resp ghTreeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("GitHubConnector.Discover: parse tree response: %w", err)
	}

	var refs []core.SourceRef
	for _, entry := range resp.Tree {
		if entry.Type != "blob" {
			continue
		}
		base := filepath.Base(entry.Path)
		ext := filepath.Ext(entry.Path)
		kind := kindForFile(base, ext)
		refs = append(refs, core.SourceRef{
			Service: svc.ID,
			Path:    entry.Path,
			Kind:    kind,
		})
	}

	sort.Slice(refs, func(i, j int) bool {
		return refs[i].Path < refs[j].Path
	})
	return refs, nil
}

// Fetch fetches the raw content of the file identified by ref.Path from the
// GitHub repository.  It uses the raw blob endpoint via gh api with an
// Accept: application/vnd.github.raw+json header.
func (gc *GitHubConnector) Fetch(_ context.Context, ref core.SourceRef) ([]byte, error) {
	// Encode the path so that slashes are preserved in the API URL.
	// gh api accepts the full path directly: repos/{owner}/{repo}/contents/{path}
	repo := gc.repo
	apiPath := fmt.Sprintf("repos/%s/contents/%s",
		repo, strings.TrimPrefix(ref.Path, "/"))

	data, err := gc.runner("api", apiPath, "--header", "Accept: application/vnd.github.raw+json")
	if err != nil {
		return nil, fmt.Errorf("GitHubConnector.Fetch %q: %w", ref.Path, err)
	}
	return data, nil
}

// Watermark returns the SHA of the repository's default-branch tree — the
// same token Discover enumerates from — so drift detection needs one API
// call, not a corpus walk (ADR-017).
func (gc *GitHubConnector) Watermark(_ context.Context, svc core.ServiceRef) (string, error) {
	repo := gc.repo
	if svc.Repo != "" {
		repo = svc.Repo
	}
	apiPath := fmt.Sprintf("repos/%s/git/trees/HEAD", repo)
	data, err := gc.runner("api", apiPath)
	if err != nil {
		return "", fmt.Errorf("GitHubConnector.Watermark: gh api %s: %w", apiPath, err)
	}
	var resp ghTreeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("GitHubConnector.Watermark: parse tree response: %w", err)
	}
	return resp.SHA, nil
}
