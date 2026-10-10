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

// TestIngest_ExtractorEpoch_WaveC2ForcesReExtraction pins wave c2's one bump (54 to 55): a file
// fingerprinted under epoch 54 carried the old extraction, so each must be re-read. A Spring
// controller, an Express app, a Laravel routes file and a Flask app are read by the route seam's
// framework readers (LB-F321..LB-F336), so their service_routes rows change; a compose service's
// list-form OTEL_SERVICE_NAME is a telemetry identity only since the doc engine's k=v split (K20,
// LB-F337, TL-9). The seeds are salted with "54", the true prior epoch, so the test fails if the
// bump reverts.
func TestIngest_ExtractorEpoch_WaveC2ForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"src/main/java/app/PingController.java": "package app;\n\nimport org.springframework.web.bind.annotation.*;\n\n@RestController\n@RequestMapping(\"/api\")\nclass PingController {\n\t@GetMapping(\"/ping\")\n\tString ping() { return \"\"; }\n}\n",
		"server.js":                             "const express = require('express');\nconst app = express();\napp.get('/health', function health(req, res) { res.send('ok'); });\napp.listen(3000);\n",
		"routes/web.php":                        "<?php\n\nuse Illuminate\\Support\\Facades\\Route;\n\nRoute::get('/books', [BookController::class, 'index']);\n",
		"app.py":                                "from flask import Flask\n\napp = Flask(__name__)\n\n@app.route('/users', methods=['GET', 'POST'])\ndef users():\n    return ''\n",
		"docker-compose.yml":                    "services:\n  checkout:\n    image: shop/checkout:1.4\n    environment:\n      - OTEL_SERVICE_NAME=checkout-api\n",
	}
	st := store.NewMemoryStore()
	ctx := context.Background()
	svc := core.ServiceRef{ID: "epoch-c2", Path: dir}
	for name, src := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		old := sha256.Sum256(append([]byte(src), []byte("\x00fathom-extractor-epoch:54")...))
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
		t.Fatalf("%d file(s) skipped: an epoch-54 fingerprint was honored, ExtractorEpoch was not bumped for wave c2", rep.Skipped)
	}
	refs, complete, err := st.ServiceRefsFor(ctx, svc.ID)
	if err != nil || !complete {
		t.Fatalf("ServiceRefsFor: complete=%v err=%v", complete, err)
	}
	routes := map[string]bool{}
	for _, r := range serviceref.RoutesFromRows(refs.Routes) {
		routes[r.Method+" "+r.Path] = true
	}
	for _, want := range []string{"GET /api/ping", "GET /health", "GET /books", "GET /users", "POST /users"} {
		if !routes[want] {
			t.Errorf("re-extraction has no route %q", want)
		}
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	identity := false
	for _, o := range obs {
		if o.SourceRef == "docker-compose.yml" && o.SourceKind == "TelemetryIdentity" && o.Symbol == "checkout-api" {
			identity = true
		}
	}
	if !identity {
		t.Error("re-extraction has no docker-compose.yml TelemetryIdentity checkout-api (list form)")
	}
	if t.Failed() {
		keys := make([]string, 0, len(routes))
		for k := range routes {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		t.Logf("routes: %v", keys)
	}
}
