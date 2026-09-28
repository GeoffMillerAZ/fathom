package main

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// RT-34 review finding 3: the serving daemon's refuse rung drops every
// cache — the graph and navigation caches and the obligations hot-path
// memo — on top of shed's console and fleet memos.
func TestServingRefuseDropsEveryCache(t *testing.T) {
	c := client.New(connector.NewLocalConnector(), store.NewMemoryStore())
	defer c.Close()
	a := servingLadderActions(mcp.NewServer(nil), c, newFleetAdapter(c, nil))
	names := map[string]bool{}
	for _, d := range append(append([]namedDrop{}, a.dropShed...), a.dropRefuse...) {
		names[d.name] = true
		d.drop() // every drop runs on a fresh daemon without panicking
	}
	for _, want := range []string{"console memos", "fleet memos", "graph and navigation caches", "obligations memos", "ingest stat records", "idle grammar modules"} {
		if !names[want] {
			t.Errorf("ladder drops %v lack %q", names, want)
		}
	}
}
