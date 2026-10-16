package client

// findings.go — P4, the consumer seam for the findings-loop's governed
// external import (vision docs/vision/30-findings-loop.md §3.5, plan FL-11).
// ImportFindings is the ONE code path `fathom findings import`
// (cmd/fathom/findings_import.go) and any other Go caller both run —
// identical receipts for identical input, proven by
// cmd/fathom's TestFindingsImportConformance (FL-14).

import (
	"context"
	"fmt"
	"time"

	"github.com/geoffmilleraz/fathom/internal/findings/external"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// ImportFindingsRequest bundles one import call's inputs.
type ImportFindingsRequest struct {
	// Service is the indexed service the import targets.
	Service string
	// Subject is the provisioned "org/repo/component" mapping (mirrors
	// .fathom/config.toml [findings] subject) — every event's own subject
	// and visibility must equal it, or the whole call refuses (FL-9).
	Subject string
	// Producer, when non-empty, requires every event's producer.name to
	// equal it exactly; "" accepts any catalogued producer.
	Producer string
	// Projection is the raw JSON bytes of the review kit's
	// #FathomProjectionOutbox projection.
	Projection []byte
	// DryRun computes the identical receipt without writing anything.
	DryRun bool
}

// ImportFindings decodes req.Projection (ParseProjection — a batch schema
// failure returns its error here, nothing written) and applies it via
// internal/findings/external.Importer, joined against this Client's own
// store. A cites link's hash is read at each event's own SourceRevision
// through git in the service's recorded corpus root (service_meta.Path;
// external.GitContentAt), never from the working tree, whose lines may
// have moved since the producer observed them (WAE-1): an empty or
// unreadable revision writes no cites link and the receipt's CitesSkew
// names it. A scope-fp link's fingerprint is the store's current one, as
// every other serve-time drift check reads it.
func (c *Client) ImportFindings(ctx context.Context, req ImportFindingsRequest) (external.Receipt, error) {
	proj, err := external.ParseProjection(req.Projection)
	if err != nil {
		return external.Receipt{}, err
	}

	root := ""
	if meta, found, ferr := c.st.GetServiceMeta(ctx, req.Service); ferr == nil && found {
		root = meta.Path
	}
	fingerprint := func(path string) (string, bool) {
		hash, found, err := c.st.Fingerprint(ctx, req.Service, path)
		if err != nil || !found {
			return "", false
		}
		return hash, true
	}

	im := external.Importer{
		Store:       c.st,
		Now:         time.Now,
		Catalog:     semantic.DefaultProducers(),
		Subject:     req.Subject,
		ContentAt:   external.GitContentAt(ctx, root),
		Fingerprint: fingerprint,
	}
	receipt, err := im.Import(ctx, req.Service, req.Producer, proj, req.DryRun)
	if err != nil {
		return external.Receipt{}, fmt.Errorf("client.ImportFindings: %w", err)
	}
	return receipt, nil
}
