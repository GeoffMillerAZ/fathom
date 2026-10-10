package fusion_test

// logtemplate_ingest_test.go — TL-4 (F67, ADR-064): ingest stores each
// logger call's template as a LogTemplate observation of its file, the
// templates follow their file through change and deletion, ReingestFile
// writes the same rows a fresh ingest does, and a store without them (from
// before TL-4, or after an ingest that died between a file's symbols and
// its templates) is swept once.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/logtemplate"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const ltSvc = "lt-svc"

var ltFiles = map[string]string{
	"shop/orders/order.py": `import logging
logger = logging.getLogger(__name__)


class Order:
    def add_line(self, qty):
        if qty <= 0:
            logger.warning("rejected line qty=%d for order %s", qty, self.id)
        self.qty = qty
`,
	"src/main/java/com/acme/Invoice.java": `package com.acme;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
public class Invoice {
  private static final Logger log = LoggerFactory.getLogger(Invoice.class);
  public int total(int n) {
    log.error("invoice {} total {} over limit", id, n);
    return n;
  }
}
`,
	"src/ship.js": `const logger = require('./log');
function ship(id) {
  logger.info({ id }, 'parcel %s left the depot', id);
}
module.exports = { ship };
`,
	"src/Auth.php": `<?php
namespace App;
class Auth {
    public function verify($token) {
        $this->logger->notice('token {token} expired for {user}', ['token' => $token, 'user' => 1]);
    }
}
`,
	"README.md": "# fixture\n",
}

func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func openLTStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "lt.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	return sq
}

// templateRows lists the service's LogTemplate rows as comparable strings.
func templateRows(t *testing.T, st store.Store) []string {
	t.Helper()
	obs, err := st.QueryObservations(context.Background(), ltSvc, "")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, o := range obs {
		if o.SourceKind == logtemplate.Kind {
			out = append(out, fmt.Sprintf("%s:%d %q pkg=%q [%s] %s", o.SourceRef, o.Line, o.Symbol, o.Package, o.Claim, o.EvidenceTier))
		}
	}
	sort.Strings(out)
	return out
}

func ingestLT(t *testing.T, st store.Store, dir string) {
	t.Helper()
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 2, DisableTypedEdges: true})
	if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: ltSvc, Path: dir}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
}

// TestIngest_StoresLogTemplatesAsObservations: one row per logger call, in
// its file, at its line, named by its template and signed with its API,
// enclosing symbol and placeholder spans.
func TestIngest_StoresLogTemplatesAsObservations(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, ltFiles)
	st := openLTStore(t)
	ingestLT(t, st, dir)
	got := templateRows(t, st)
	want := []string{
		`shop/orders/order.py:8 "rejected line qty=%d for order %s" pkg="order" [python.logging in=Order.add_line holes=18-20i,31-33] pre-scan`,
		`src/Auth.php:5 "token {token} expired for {user}" pkg="App" [php.psr3 in=Auth.verify holes=6-13,26-32] pre-scan`,
		`src/main/java/com/acme/Invoice.java:7 "invoice {} total {} over limit" pkg="com.acme" [java.slf4j in=Invoice.total holes=8-10,17-19] pre-scan`,
		`src/ship.js:3 "parcel %s left the depot" pkg="ship" [node.logger in=ship holes=7-9] pre-scan`,
	}
	if len(got) != len(want) {
		t.Fatalf("template rows:\n%s", joinLines(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d:\n got  %s\n want %s", i, got[i], want[i])
		}
	}
}

func joinLines(xs []string) string {
	out := ""
	for _, x := range xs {
		out += "  " + x + "\n"
	}
	return out
}

// TestIngest_LogTemplatesFollowTheirFile: a changed file's templates are
// replaced (never duplicated), an unchanged file's stay as they were, and a
// deleted file's go with it.
func TestIngest_LogTemplatesFollowTheirFile(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, ltFiles)
	st := openLTStore(t)
	ingestLT(t, st, dir)
	before := templateRows(t, st)
	ingestLT(t, st, dir) // nothing changed
	if again := templateRows(t, st); !reflect.DeepEqual(again, before) {
		t.Fatalf("an unchanged re-ingest moved the templates:\n%s", joinLines(again))
	}
	writeTree(t, dir, map[string]string{"src/ship.js": "const logger = require('./log');\nfunction ship(id) {\n  logger.warn('parcel %s held at customs', id);\n}\n"})
	if err := os.Remove(filepath.Join(dir, "src", "Auth.php")); err != nil {
		t.Fatal(err)
	}
	ingestLT(t, st, dir)
	got := templateRows(t, st)
	want := []string{before[0], before[2], `src/ship.js:3 "parcel %s held at customs" pkg="ship" [node.logger in=ship holes=7-9] pre-scan`}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("after edit and delete:\n got\n%s want\n%s", joinLines(got), joinLines(want))
	}
}

// TestReingestFile_LogTemplateParity: the one-file refresh writes exactly
// the template rows a fresh ingest of the edited tree writes.
func TestReingestFile_LogTemplateParity(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, ltFiles)
	st := openLTStore(t)
	ingestLT(t, st, dir)
	edited := map[string]string{"shop/orders/order.py": `import logging
logger = logging.getLogger(__name__)


class Order:
    def add_line(self, qty):
        logger.info("adding %d", qty)
        if qty <= 0:
            logger.error(f"order {self.id} rejected a line")
        self.qty = qty
`}
	writeTree(t, dir, edited)
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 1, DisableTypedEdges: true})
	if _, err := p.ReingestFile(context.Background(), st, ltSvc, dir, "shop/orders/order.py"); err != nil {
		t.Fatalf("ReingestFile: %v", err)
	}
	got := templateRows(t, st)

	fresh := openLTStore(t)
	ingestLT(t, fresh, dir)
	want := templateRows(t, fresh)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReingestFile rows differ from a fresh ingest:\n got\n%s want\n%s", joinLines(got), joinLines(want))
	}
}

// dropTemplates rewrites a file's observations without its templates: a
// store as one from before TL-4 left it.
func dropTemplates(t *testing.T, st *store.SQLiteStore, file string) {
	t.Helper()
	ctx := context.Background()
	obs, err := st.SourceObservations(ctx, ltSvc, []string{file})
	if err != nil {
		t.Fatal(err)
	}
	var keep []core.Observation
	for _, o := range obs {
		if o.SourceKind != logtemplate.Kind {
			keep = append(keep, o)
		}
	}
	if err := st.PurgeSource(ctx, ltSvc, file); err != nil {
		t.Fatal(err)
	}
	if err := st.PutObservations(ctx, keep); err != nil {
		t.Fatal(err)
	}
}

// TestIngest_LogTemplateSweep: an unchanged file with no templates gets
// them once, when no template pass has completed for the service, or when
// the previous ingest never committed; otherwise an unchanged file is not
// read again.
func TestIngest_LogTemplateSweep(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, ltFiles)
	st := openLTStore(t)
	ingestLT(t, st, dir)
	want := templateRows(t, st)
	ctx := context.Background()

	// A store from before TL-4: no pass state, no templates.
	dropTemplates(t, st, "src/main/java/com/acme/Invoice.java")
	if err := st.DeleteTypedEdgePackage(ctx, ltSvc, "typed-pass-state:log-template"); err != nil {
		t.Fatal(err)
	}
	ingestLT(t, st, dir)
	if got := templateRows(t, st); !reflect.DeepEqual(got, want) {
		t.Errorf("sweep after a store without pass state:\n got\n%s want\n%s", joinLines(got), joinLines(want))
	}

	// The pass state stands but the previous ingest died in progress.
	dropTemplates(t, st, "shop/orders/order.py")
	ingestLT(t, st, dir) // state ok, nothing changed: not swept
	if got := templateRows(t, st); len(got) != len(want)-1 {
		t.Fatalf("an unchanged file was re-read with the pass state ok: %d rows", len(got))
	}
	if err := st.SetIngestState(ctx, store.IngestState{ServiceID: ltSvc, State: store.IngestInProgress, PID: 1}); err != nil {
		t.Fatal(err)
	}
	ingestLT(t, st, dir)
	if got := templateRows(t, st); !reflect.DeepEqual(got, want) {
		t.Errorf("sweep after an ingest that never committed:\n got\n%s want\n%s", joinLines(got), joinLines(want))
	}
}
