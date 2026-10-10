package fusion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestIngest_ExtractorEpoch_WaveC1ForcesReExtraction pins wave c1's one bump (53 to 54): a file
// fingerprinted under epoch 53 carried the old extraction, so each must be re-read. A workflow is
// served by the github-actions dialect, which names a run step by position (LB-F306, LB-F310, O630);
// composer.json is read as a manifest (LB-Q-composer); a prometheus.yml is claimed through the
// late-YAML hub and its scrape job is a telemetry identity (LB-F309, LB-F319); a Kubernetes
// container's and a compose service's OTEL_SERVICE_NAME is a telemetry identity (LB-F314, TL-5).
// The seeds are salted with "53", the true prior epoch, so the test fails if the bump reverts.
func TestIngest_ExtractorEpoch_WaveC1ForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		".github/workflows/ci.yml": "name: ci\non: push\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v4\n      - name: Build\n        run: go build ./...\n",
		"composer.json":            "{\n  \"name\": \"acme/app\",\n  \"require\": {\n    \"php\": \"^8.3\"\n  }\n}\n",
		"prometheus.yml":           "global:\n  scrape_interval: 15s\nscrape_configs:\n  - job_name: checkout\n    static_configs:\n      - targets: [\"checkout:8080\"]\n",
		"deploy.yaml":              "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: cart\n  namespace: shop\nspec:\n  template:\n    spec:\n      containers:\n        - name: cart\n          image: shop/cart:1.0\n          env:\n            - name: OTEL_SERVICE_NAME\n              value: cart-api\n",
		"docker-compose.yml":       "services:\n  checkout:\n    image: shop/checkout:1.4\n    environment:\n      OTEL_SERVICE_NAME: checkout-api\n",
	}
	st := store.NewMemoryStore()
	ctx := context.Background()
	svc := core.ServiceRef{ID: "epoch-c1", Path: dir}
	for name, src := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		old := sha256.Sum256(append([]byte(src), []byte("\x00fathom-extractor-epoch:53")...))
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
		t.Fatalf("%d file(s) skipped: an epoch-53 fingerprint was honored, ExtractorEpoch was not bumped for wave c1", rep.Skipped)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, o := range obs {
		got[o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol] = true
	}
	for _, want := range []string{
		".github/workflows/ci.yml|GHARunStep|jobs.build.steps#1",
		"composer.json|ComposerPackage|acme/app",
		"prometheus.yml|TelemetryIdentity|checkout",
		"deploy.yaml|TelemetryIdentity|cart-api",
		"docker-compose.yml|TelemetryIdentity|checkout-api",
	} {
		if !got[want] {
			t.Errorf("re-extraction has no %q", want)
		}
	}
	if t.Failed() {
		keys := make([]string, 0, len(got))
		for k := range got {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		t.Logf("observations: %v", keys)
	}
}
