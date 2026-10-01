package fusion

// proxydialects_test.go — LB-F101: the native proxy dialects (nginx, HAProxy,
// Caddyfile) dispatch by manifest basename, glob and content sniff, ingest
// and ReingestFile decide alike, and a .conf no sniff claims is never
// extracted (counted ambiguous).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// proxyCorpus is a repository with each way a proxy configuration is
// claimed: nginx.conf by basename, conf.d/app.conf by sniff, an
// extensionless sites-enabled/default by glob, haproxy.cfg by basename,
// lb/edge.cfg by sniff, a Caddyfile and a Caddyfile.dev by basename and
// glob; and the files that are none: a redis.conf and a supervisord.conf
// (.conf, no sniff claims them), a setup.cfg (.cfg, no sniff claims it).
func proxyCorpus() map[string]string {
	return map[string]string{
		"nginx/nginx.conf": `events {}
http {
    include conf.d/app.conf;
    upstream api {
        server api:8080;
    }
    server {
        listen 80;
        server_name example.com;
        location /api/ {
            proxy_pass http://api;
        }
    }
}
`,
		"nginx/conf.d/app.conf": `server {
    listen 8081;
    server_name app.example.com;
    location / {
        proxy_pass http://web:3000;
    }
}
`,
		"nginx/sites-enabled/default": `server {
    listen 80 default_server;
    root /var/www/html;
}
`,
		"haproxy.cfg": `frontend fe
    bind :80
    default_backend be
backend be
    server s1 app:80 check
`,
		"lb/edge.cfg": `backend edge
    server e1 edge:443
`,
		"Caddyfile": `(common) {
	encode gzip
}

example.com {
	import common
	reverse_proxy app:8080
}
`,
		"Caddyfile.dev": `localhost {
	reverse_proxy app:8080
}
`,
		"redis.conf":       "bind 127.0.0.1\nport 6379\n",
		"supervisord.conf": "[program:app]\ncommand=/usr/bin/app\n",
		"setup.cfg":        "[metadata]\nname = demo\n",
	}
}

// TestProxyDialectClassification: the manifests' match blocks classify
// each file as the dispatch expects.
func TestProxyDialectClassification(t *testing.T) {
	rules := dialectRules()
	src := proxyCorpus()
	for path, want := range map[string]struct{ dialect, by string }{
		"nginx/nginx.conf":            {"nginx", connector.ClassifiedByBasename},
		"nginx/conf.d/app.conf":       {"nginx", connector.ClassifiedBySniff},
		"nginx/sites-enabled/default": {"nginx", connector.ClassifiedByGlob},
		"haproxy.cfg":                 {"haproxy", connector.ClassifiedByBasename},
		"lb/edge.cfg":                 {"haproxy", connector.ClassifiedBySniff},
		"Caddyfile":                   {"caddyfile", connector.ClassifiedByBasename},
		"Caddyfile.dev":               {"caddyfile", connector.ClassifiedByGlob},
		"redis.conf":                  {"", connector.ClassifiedAmbiguous},
		"supervisord.conf":            {"", connector.ClassifiedAmbiguous},
		"setup.cfg":                   {"", connector.ClassifiedAmbiguous},
	} {
		v := connector.ClassifyDialect(rules, path, []byte(src[path]))
		if v.Dialect != want.dialect || v.By != want.by {
			t.Errorf("ClassifyDialect(%q) = %q by %q, want %q by %q", path, v.Dialect, v.By, want.dialect, want.by)
		}
	}
	for path, want := range map[string]string{"nginx/nginx.conf": "nginx", "haproxy.cfg": "haproxy", "Caddyfile": "caddyfile", "site.caddyfile": "caddyfile"} {
		if got := connector.LanguageForPath(path); got != want {
			t.Errorf("LanguageForPath(%q) = %q, want %q", path, got, want)
		}
	}
}

// TestIngest_ProxyDialects: an ingest extracts every proxy configuration
// through its dialect (by basename, glob or sniff), never a .conf or .cfg no
// sniff claims (counted ambiguous instead), and a one-file refresh of a
// sniffed and a globbed file reads them through the same dialect.
func TestIngest_ProxyDialects(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, proxyCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "px", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	if rep.AmbiguousUnclassified != 3 {
		t.Errorf("AmbiguousUnclassified = %d, want 3 (redis.conf, supervisord.conf, setup.cfg)", rep.AmbiguousUnclassified)
	}
	obs, err := st.QueryObservations(ctx, "px", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, o := range obs {
		got[o.SourceRef+"|"+o.Symbol] = string(o.SourceKind)
		switch o.SourceRef {
		case "redis.conf", "supervisord.conf", "setup.cfg":
			t.Errorf("%s|%s: an unclaimed file was extracted as %s", o.SourceRef, o.Symbol, o.SourceKind)
		}
	}
	for key, kind := range map[string]string{
		"nginx/nginx.conf|nginx.conf":             "NginxConfig",
		"nginx/nginx.conf|api":                    "NginxUpstream",
		"nginx/nginx.conf|example.com /api/":      "NginxLocation",
		"nginx/conf.d/app.conf|app.example.com":   "NginxServer",
		"nginx/conf.d/app.conf|app.example.com /": "NginxLocation",
		"nginx/sites-enabled/default|80":          "NginxServer",
		"haproxy.cfg|fe":                          "HAProxyFrontend",
		"haproxy.cfg|be s1":                       "HAProxyServer",
		"lb/edge.cfg|edge":                        "HAProxyBackend",
		"Caddyfile|common":                        "CaddySnippet",
		"Caddyfile|example.com":                   "CaddySite",
		"Caddyfile|example.com reverse_proxy":     "CaddyRoute",
		"Caddyfile.dev|localhost reverse_proxy":   "CaddyRoute",
		"nginx/sites-enabled/default|default":     "NginxConfig",
		"nginx/conf.d/app.conf|app.conf":          "NginxConfig",
		"Caddyfile.dev|Caddyfile.dev":             "CaddyfileConfig",
		"haproxy.cfg|fe be":                       "HAProxyRoute",
		"nginx/nginx.conf|api api:8080":           "NginxUpstreamServer",
		"nginx/nginx.conf|example.com":            "NginxServer",
	} {
		if got[key] != kind {
			t.Errorf("%s = %q, want %q", key, got[key], kind)
		}
	}
	var calls []string
	for _, o := range obs {
		if len(o.Calls) > 0 {
			calls = append(calls, o.SourceRef+"|"+o.Symbol+"->"+o.Calls[0])
		}
	}
	sort.Strings(calls)
	wantCalls := []string{
		"Caddyfile|example.com->common",
		"haproxy.cfg|fe be->be",
		"nginx/nginx.conf|example.com /api/->api",
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Errorf("calls = %q, want %q", calls, wantCalls)
	}

	// ReingestFile decides as ingest does: an edit to the sniffed and the
	// globbed file is read through the nginx dialect.
	for rel, body := range map[string]string{
		"nginx/conf.d/app.conf":       "server {\n    listen 8082;\n    server_name edited.example.com;\n}\n",
		"nginx/sites-enabled/default": "server {\n    listen 8083;\n    server_name edited2.example.com;\n}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ReingestFile(ctx, st, "px", dir, rel); err != nil {
			t.Fatal(err)
		}
	}
	obs, err = st.QueryObservations(ctx, "px", "")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, o := range obs {
		seen[o.SourceRef+"|"+o.Symbol] = true
	}
	for _, key := range []string{"nginx/conf.d/app.conf|edited.example.com", "nginx/sites-enabled/default|edited2.example.com"} {
		if !seen[key] {
			t.Errorf("after ReingestFile: %s missing", key)
		}
	}
	if seen["nginx/conf.d/app.conf|app.example.com"] {
		t.Error("after ReingestFile: the edited file's old server survived")
	}
}

// TestResolveDocXrefs_NginxInclude: an include's repo-path candidates
// resolve to the one that is a file of the repository, entity to the
// included file's root (an include glob names no file: docxref.go matches a
// glob against directories, LB-Q-nginx-rest).
func TestResolveDocXrefs_NginxInclude(t *testing.T) {
	src := map[string]string{
		"nginx/nginx.conf":           "http {\n    include h5bp/basic.conf;\n    include conf.d/*.conf;\n}\n",
		"nginx/h5bp/basic.conf":      "include h5bp/security/x.conf;\n",
		"nginx/h5bp/security/x.conf": "add_header X-Frame-Options DENY;\n",
		"nginx/conf.d/a.conf":        "server {\n    listen 80;\n}\n",
	}
	var paths []string
	for p := range src {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	x := extractor.NewNginxExtractor()
	var files []DocFile
	for _, rel := range paths {
		r, err := x.ExtractDoc("svc", rel, []byte(src[rel]))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, DocFile{Path: rel, Result: r})
	}
	got := xrefBrief(ResolveDocXrefs(files))
	want := []string{
		"nginx/h5bp/basic.conf:basic.conf -includes-> nginx/h5bp/security/x.conf:x.conf",
		"nginx/nginx.conf:nginx.conf -includes-> nginx/h5bp/basic.conf:basic.conf",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

// TestIngest_ExtractorEpoch_ProxyDialectsForceReExtraction pins epoch 43
// (LB-F101, LB-F104; wave px, renumbered from 42 when wave lc took 42 first):
// an nginx.conf, a haproxy.cfg and a Caddyfile fingerprinted at epoch 42 were
// read by no extractor, and an envoy.yaml by the generic YAML one, so the
// bump must re-read them. The seeded fingerprints are salted with "42", the
// true prior epoch.
func TestIngest_ExtractorEpoch_ProxyDialectsForceReExtraction(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"nginx.conf":  "events {}\nhttp {\n    server {\n        listen 80;\n    }\n}\n",
		"haproxy.cfg": "backend be\n    server s1 app:80\n",
		"Caddyfile":   "example.com {\n\treverse_proxy app:8080\n}\n",
		"envoy.yaml":  gwEnvoyYAML,
	}
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "epoch43-svc", Path: dir}
	ctx := context.Background()
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		old := sha256.Sum256(append([]byte(src), []byte("\x00fathom-extractor-epoch:42")...))
		if err := st.SetFingerprint(ctx, svc.ID, name, hex.EncodeToString(old[:])); err != nil {
			t.Fatal(err)
		}
	}
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, o := range obs {
		kinds[o.SourceRef+"|"+string(o.SourceKind)] = true
	}
	for _, want := range []string{"nginx.conf|NginxServer", "haproxy.cfg|HAProxyServer", "Caddyfile|CaddySite", "envoy.yaml|EnvoyCluster"} {
		if !kinds[want] {
			t.Errorf("%s not extracted: a pre-epoch-43 fingerprint was honored", want)
		}
	}
}
