package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/internal/scip"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

const scipSourceStatePrefix = "scip-source:"

func (c *Client) scipSourceState(ctx context.Context, service, root, path string) (bool, error) {
	_, found, err := c.st.TypedEdgePackageFingerprint(ctx, service, scipSourceStatePrefix+scipIndexKey(root, path))
	return found, err
}
func (c *Client) rememberSCIPSource(ctx context.Context, service, root, path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	return c.st.SetTypedEdgePackageFingerprint(ctx, service, scipSourceStatePrefix+scipIndexKey(root, path), absolute)
}
func (c *Client) clearSCIPSource(ctx context.Context, service, root, path string) error {
	defer memo.EvictStoreService(c.StoreID(), service)
	prefix := scipStoreKeyPrefix(scipIndexKey(root, path))
	packages, err := c.st.ListTypedEdgePackages(ctx, service)
	if err != nil {
		return err
	}
	for _, key := range packages {
		if strings.HasPrefix(key, prefix) {
			if err := c.st.DeleteTypedEdgePackage(ctx, service, key); err != nil {
				return err
			}
		}
	}
	return nil
}

// reconcileSCIPSources runs on every shared ingest path, including the daemon
// queue and file refresh. Source-bound indexes remain remembered when a producer
// deletes its sidecar/index or leaves an incomplete attempt, so old rows cannot
// silently outlive the artifact that justified them. Legacy producers with no
// source receipt retain their existing explicit-import behavior.
func (c *Client) reconcileSCIPSources(ctx context.Context, service, root string, importFresh bool) error {
	paths := map[string]bool{}
	keys, err := c.st.ListTypedEdgePackages(ctx, service)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if strings.HasPrefix(key, scipSourceStatePrefix) {
			path, found, err := c.st.TypedEdgePackageFingerprint(ctx, service, key)
			if err != nil {
				return err
			}
			if found {
				paths[path] = true
			}
		}
	}
	candidates := []string{filepath.Join(root, "index.scip")}
	matches, err := filepath.Glob(filepath.Join(root, ".fathom", "scip", "*.scip"))
	if err != nil {
		return err
	}
	candidates = append(candidates, matches...)
	for _, path := range candidates {
		if scip.SourceReceiptExpected(path) {
			paths[path] = true
		}
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	slices.Sort(ordered)
	var failures []error
	for _, path := range ordered {
		if importFresh {
			if _, err := c.ImportSCIP(ctx, service, path); err != nil {
				failures = append(failures, err)
			}
			continue
		}
		raw, readErr := readSourceSCIP(ctx, path)
		_, checkErr := scip.ValidateSourceReceipt(ctx, root, path, raw, true)
		if readErr != nil {
			checkErr = fmt.Errorf("%w: index: %v", scip.ErrSourceReceipt, readErr)
		}
		if checkErr != nil {
			clearErr := c.clearSCIPSource(context.WithoutCancel(ctx), service, root, path)
			failures = append(failures, errors.Join(checkErr, clearErr))
		}
	}
	return errors.Join(failures...)
}

const sourceSCIPIndexLimit int64 = 64 << 20

func readSourceSCIP(ctx context.Context, path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > sourceSCIPIndexLimit {
		return nil, fmt.Errorf("source-bound SCIP index is not regular or exceeds the 64MiB import budget")
	}
	if err := tenancy.Charge(ctx, info.Size()); err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(io.LimitReader(file, sourceSCIPIndexLimit+1))
	if len(raw) > int(sourceSCIPIndexLimit) {
		return nil, fmt.Errorf("source-bound SCIP index grew beyond the 64MiB import budget")
	}
	return raw, err
}
