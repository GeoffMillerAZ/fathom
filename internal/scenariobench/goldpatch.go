package scenariobench

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GoldPatchFetcher reads one task's gold patch from source.Revision. It must
// honor that exact pin, never a dataset's moving default revision. Fetching is
// an operator-run bench concern; no network client is installed in the daemon.
type GoldPatchFetcher func(ctx context.Context, source SourcePin, taskID string) ([]byte, error)

// FetchGoldPatch returns verified patch bytes and caches them only in cacheDir,
// which the operator places under workspace/. source is the task's dataset pin
// (the existing corpus SourcePin until per-task provenance is available). It is
// mandatory even on a cache hit. The key includes the complete source pin, task
// ID and Gold.PatchSHA; neither dataset strings nor task IDs become path names.
// A cache hit is rehashed and corruption is refused. Patches are never applied
// to a subject or written to a tracked source path by this function.
func FetchGoldPatch(ctx context.Context, task Task, source SourcePin, cacheDir string, fetch GoldPatchFetcher) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(source.Revision) == "" {
		return nil, fmt.Errorf("gold patch %s: source revision is required", task.ID)
	}
	if strings.TrimSpace(source.Name) == "" {
		return nil, fmt.Errorf("gold patch %s: source dataset name is required", task.ID)
	}
	expected := strings.ToLower(task.Gold.PatchSHA)
	digest, err := hex.DecodeString(expected)
	if err != nil || len(digest) != sha256.Size {
		return nil, fmt.Errorf("gold patch %s: invalid patch sha256 %q", task.ID, task.Gold.PatchSHA)
	}
	if strings.TrimSpace(cacheDir) == "" || strings.TrimSpace(task.ID) == "" {
		return nil, fmt.Errorf("gold patch: cache directory and task ID are required")
	}
	keyBytes, err := json.Marshal(struct {
		Source SourcePin
		TaskID string
		SHA256 string
	}{source, task.ID, expected})
	if err != nil {
		return nil, err
	}
	key := sha256.Sum256(keyBytes)
	path := filepath.Join(cacheDir, fmt.Sprintf("%x.patch", key))
	verify := func(patch []byte) ([]byte, error) {
		actual := fmt.Sprintf("%x", sha256.Sum256(patch))
		if actual != expected {
			return nil, fmt.Errorf("gold patch %s at revision %s: sha256 mismatch: expected %s, got %s", task.ID, source.Revision, expected, actual)
		}
		return patch, nil
	}
	if cached, err := os.ReadFile(path); err == nil {
		return verify(cached)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading gold patch cache: %w", err)
	}
	if fetch == nil {
		return nil, fmt.Errorf("gold patch %s: a pinned dataset fetcher is required on a cache miss", task.ID)
	}
	patch, err := fetch(ctx, source, task.ID)
	if err != nil {
		return nil, fmt.Errorf("fetching gold patch %s at revision %s: %w", task.ID, source.Revision, err)
	}
	patch, err = verify(patch)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return nil, fmt.Errorf("creating gold patch cache: %w", err)
	}
	// Rename a file created inside the cache instead of truncating an existing
	// entry: interrupted writes and pre-existing file symlinks cannot redirect
	// a write into the subject tree.
	tmp, err := os.CreateTemp(cacheDir, ".gold-patch-")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(patch); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return nil, err
	}
	return patch, nil
}
