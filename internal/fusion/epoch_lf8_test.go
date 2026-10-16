package fusion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestIngest_ExtractorEpoch_WaveLF8ForcesReExtraction pins wave lf8's one bump (63 to 64): a file
// fingerprinted under epoch 63 carried the old extraction, so each must be re-read. A Kubernetes
// manifest's workloads store their per-container facts as K8sFact rows (LB-F451); a PowerShell
// script's top level is its module symbol, with per-file keys and file edges (LB-F521..LB-F523,
// owner ruling S5); a pipeline named <x>.Jenkinsfile reaches the jenkinsfile dialect (LB-F562,
// O1042), where it was fingerprinted unread. The seeds are salted with "63", the true prior epoch
// (wave lf7's bump), so the test fails if the bump reverts.
func TestIngest_ExtractorEpoch_WaveLF8ForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"deploy/web.yaml": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n  namespace: shop\n" +
			"spec:\n  replicas: 2\n  selector:\n    matchLabels:\n      app: web\n  template:\n    metadata:\n" +
			"      labels:\n        app: web\n    spec:\n      containers:\n        - name: web\n          image: acme/web:1\n" +
			"          resources:\n            requests:\n              cpu: 100m\n",
		"scripts/build.ps1":   "function Invoke-Build {\n  Write-Output 'ok'\n}\nInvoke-Build\n",
		"ci/main.Jenkinsfile": "pipeline {\n  agent any\n  stages {\n    stage('A') {\n      steps {\n        echo 'x'\n      }\n    }\n  }\n}\n",
		"docs/index.md":       "# index\n",
	}
	st := store.NewMemoryStore()
	ctx := context.Background()
	svc := core.ServiceRef{ID: "epoch-lf8", Path: dir}
	for name, src := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		if filepath.Ext(name) == ".md" {
			continue // a control: never seeded, always read
		}
		old := sha256.Sum256(append([]byte(src), []byte("\x00fathom-extractor-epoch:63")...))
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
		t.Fatalf("%d file(s) skipped: an epoch-63 fingerprint was honored, ExtractorEpoch was not bumped for wave lf8", rep.Skipped)
	}
	kinds := crSourceKinds(t, st, svc.ID)
	for file, kind := range map[string]string{
		"deploy/web.yaml":     "K8sFact",          // LB-F451: a workload's per-container facts
		"scripts/build.ps1":   "PowerShellModule", // LB-F521: the script's top level
		"ci/main.Jenkinsfile": "JenkinsPipeline",  // LB-F562: the case-variant extension is read
	} {
		if !crHasKind(kinds[file], kind) {
			t.Errorf("%s re-read as %q, want %s among them", file, kinds[file], kind)
		}
	}
}
