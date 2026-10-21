package client

import (
	"context"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// SourceRefProjection reads facts sealed for an indexed ingest generation.
// found=false means unavailable or invalid; callers must not treat that as an
// empty corpus. The result makes no claim about current worktree bytes.
func (c *Client) SourceRefProjection(ctx context.Context, service string) (store.SourceRefProjection, bool, error) {
	reader, ok := c.st.(store.SourceRefProjectionReader)
	if !ok {
		return store.SourceRefProjection{}, false, nil
	}
	p, found, err := reader.ReadSourceRefProjection(ctx, service)
	if err != nil || !found {
		return p, found, err
	}
	local, ok := c.conn.(*connector.LocalConnector)
	if !ok {
		return store.SourceRefProjection{}, false, nil
	}
	// PolicyDigest's Git/walk mode follows Discover's actual selection. A
	// restarted read client has no remembered mode, so derive it on a
	// separate connector to avoid changing an in-flight ingest's roots.
	probe := &connector.LocalConnector{RespectGitignore: local.RespectGitignore}
	svc := core.ServiceRef{ID: service, Path: p.Root}
	members, err := probe.Discover(ctx, svc)
	if err != nil {
		return store.SourceRefProjection{}, false, err
	}
	if len(members) != len(p.Rows) {
		return store.SourceRefProjection{}, false, nil
	}
	for i, ref := range members {
		if ref.Path != p.Rows[i].Path {
			return store.SourceRefProjection{}, false, nil
		}
	}
	policy, err := probe.DiscoveryPolicyDigest(ctx, svc)
	if err != nil {
		return store.SourceRefProjection{}, false, err
	}
	if policy != p.Policy {
		return store.SourceRefProjection{}, false, nil
	}
	// file_stats is the store's scoped read. If a suppression hides even one
	// member, refuse the whole sealed projection rather than serving refs or
	// absence from an unfiltered optional store capability.
	stats, ok, err := c.st.FileStats(ctx, service)
	if err != nil {
		return store.SourceRefProjection{}, false, err
	}
	if !ok || len(stats) != len(p.Rows) {
		return store.SourceRefProjection{}, false, nil
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].Path < stats[j].Path })
	for i, row := range p.Rows {
		if stats[i].Path != row.Path || stats[i].Excluded != row.Excluded {
			return store.SourceRefProjection{}, false, nil
		}
	}
	return p, true, nil
}
