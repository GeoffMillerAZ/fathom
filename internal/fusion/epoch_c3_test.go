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

// TestIngest_ExtractorEpoch_WaveC3ForcesReExtraction pins wave c3's one bump (56 to 57): a file
// fingerprinted under epoch 56 carried the old extraction, so each must be re-read. A TSX
// component is a JSComponent since the React kinds (LB-F377); a Jenkinsfile fell through to no
// extractor before the jenkinsfile dialect (LB-F373); an alertmanager.yml is read by the
// alertmanager dialect (LB-F362); a CloudFormation metric filter's metric is a CFNMetric (LB-F368)
// and a Terraform one a TerraformMetric (LB-F371). The seeds are salted with "56", the true prior
// epoch (wave lf2's bump), so the test fails if the bump reverts.
func TestIngest_ExtractorEpoch_WaveC3ForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"src/App.tsx": "export function App() {\n  return <main>shop</main>;\n}\n",
		"Jenkinsfile": "pipeline {\n    agent any\n    stages {\n        stage('Build') {\n            steps {\n                sh 'make test'\n            }\n        }\n    }\n}\n",
		"monitoring/alertmanager.yml": "route:\n  receiver: default\n  routes:\n    - receiver: pager\n      matchers:\n        - severity=\"critical\"\n" +
			"receivers:\n  - name: default\n  - name: pager\n",
		"infra/app.yaml": "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n  ErrorFilter:\n    Type: AWS::Logs::MetricFilter\n    Properties:\n" +
			"      LogGroupName: /aws/lambda/orders-api\n      FilterPattern: ERROR\n      MetricTransformations:\n" +
			"        - MetricNamespace: Shop/Orders\n          MetricName: Errors\n          MetricValue: \"1\"\n",
		"tf/main.tf": "resource \"aws_cloudwatch_log_metric_filter\" \"errors\" {\n  name           = \"errors\"\n  pattern        = \"ERROR\"\n" +
			"  log_group_name = \"/aws/lambda/tf-api\"\n\n  metric_transformation {\n    name      = \"Errors\"\n    namespace = \"TF/App\"\n    value     = \"1\"\n  }\n}\n",
	}
	st := store.NewMemoryStore()
	ctx := context.Background()
	svc := core.ServiceRef{ID: "epoch-c3", Path: dir}
	for name, src := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		old := sha256.Sum256(append([]byte(src), []byte("\x00fathom-extractor-epoch:56")...))
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
		t.Fatalf("%d file(s) skipped: an epoch-56 fingerprint was honored, ExtractorEpoch was not bumped for wave c3", rep.Skipped)
	}
	kinds := crSourceKinds(t, st, svc.ID)
	for rel, want := range map[string]string{
		"src/App.tsx":                 string(core.KindJSComponent),
		"Jenkinsfile":                 "JenkinsPipeline",
		"monitoring/alertmanager.yml": "AlertmanagerReceiver",
		"infra/app.yaml":              "CFNMetric",
		"tf/main.tf":                  "TerraformMetric",
	} {
		if !crHasKind(kinds[rel], want) {
			t.Errorf("%s re-read as %q, want %s among them", rel, kinds[rel], want)
		}
	}
}
