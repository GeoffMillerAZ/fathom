package fusion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestIngest_ExtractorEpoch_WaveH3ForcesReExtraction pins wave h3's one bump (57 to 58): a file
// fingerprinted under epoch 57 carried the old service_routes rows, so each must be re-read. A
// module whose exported constants hold a route path ("/app") now stores them as ConstMethod rows,
// and a React Router route whose path is an imported member chain (paths.app.users.path) is a
// deferred row the seam resolves on read (SG-46, O735). Before the bump the router file held no row
// for those routes and the constants module none at all. The seeds are salted with "57", the true
// prior epoch (wave c3's bump), so the test fails if the bump reverts.
func TestIngest_ExtractorEpoch_WaveH3ForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"web/src/config/paths.ts": "export const paths = {\n  app: { root: { path: '/app' }, users: { path: 'users' } },\n} as const;\n",
		"web/src/app/router.tsx": "import { createBrowserRouter } from 'react-router';\nimport { paths } from '@/config/paths';\n" +
			"export const router = createBrowserRouter([\n  { path: paths.app.root.path, element: <Root />, children: [\n" +
			"    { path: paths.app.users.path, element: <Users /> },\n  ] },\n]);\n",
	}
	st := store.NewMemoryStore()
	ctx := context.Background()
	svc := core.ServiceRef{ID: "epoch-h3", Path: dir}
	for name, src := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		old := sha256.Sum256(append([]byte(src), []byte("\x00fathom-extractor-epoch:57")...))
		if err := st.SetFingerprint(ctx, svc.ID, name, hex.EncodeToString(old[:])); err != nil {
			t.Fatal(err)
		}
	}
	p := New(connector.NewLocalConnector(), st, Config{Concurrency: 1})
	t.Cleanup(p.Close)
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Skipped != 0 {
		t.Fatalf("%d file(s) skipped: an epoch-57 fingerprint was honored, ExtractorEpoch was not bumped for wave h3", rep.Skipped)
	}
	refs, complete, err := st.ServiceRefsFor(ctx, svc.ID)
	if err != nil || !complete {
		t.Fatalf("ServiceRefsFor: complete=%v err=%v", complete, err)
	}
	routes := map[string]bool{}
	for _, r := range serviceref.RoutesFromRows(refs.Routes) {
		routes[r.Method+" "+r.Path] = true
	}
	for _, want := range []string{"GET /app", "GET /app/users"} {
		if !routes[want] {
			keys := make([]string, 0, len(routes))
			for k := range routes {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			t.Errorf("re-extraction has no composed route %q; routes: %v", want, keys)
		}
	}
}
