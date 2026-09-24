package main

// Ingest honesty (docs/plans/instrument-truth.md IT-14 / IT-15, the O16
// incident): under a wedged reader `fathom ingest` names every file it could
// not read — errno and the external-volume hint — and refuses to replace an
// index that already holds evidence with the plausible-looking empty result a
// wedge produces. Read failures are simulated by a connector whose Fetch
// returns the *fs.PathError os.ReadFile returns under EPERM; no real TCC
// grant is touched.

import (
	"bytes"
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/hookcache"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

type wedgedConnector struct {
	refs    []core.SourceRef
	content map[string][]byte
	fail    map[string]error
}

func (c *wedgedConnector) Discover(_ context.Context, svc core.ServiceRef) ([]core.SourceRef, error) {
	out := make([]core.SourceRef, len(c.refs))
	for i, r := range c.refs {
		r.Service = svc.ID
		out[i] = r
	}
	return out, nil
}

func (c *wedgedConnector) Fetch(_ context.Context, ref core.SourceRef) ([]byte, error) {
	if err, ok := c.fail[ref.Path]; ok {
		return nil, err
	}
	return c.content[ref.Path], nil
}

func (c *wedgedConnector) Watermark(context.Context, core.ServiceRef) (string, error) {
	return "", nil
}

func epermFor(path string) error {
	return &fs.PathError{Op: "open", Path: "/vol/repo/" + path, Err: syscall.EPERM}
}

const honestyGoSrc = "package p\n\nfunc Exported() int { return 1 }\n"

// useConnectorForIngest routes cmdIngest's client construction through conn
// against the real store at the --store path, and isolates the ident cache
// bust from the developer's real cache directory.
func useConnectorForIngest(t *testing.T, conn *wedgedConnector) {
	t.Helper()
	origFactory := newIngestClient
	newIngestClient = func(_ client.Mode, storePath string) (*client.Client, error) {
		st, err := store.Open(storePath)
		if err != nil {
			return nil, err
		}
		return client.New(conn, st), nil
	}
	origIdent := hookcache.IdentDir
	hookcache.IdentDir = filepath.Join(t.TempDir(), "ident")
	t.Cleanup(func() {
		newIngestClient = origFactory
		hookcache.IdentDir = origIdent
	})
}

func observationCount(t *testing.T, storePath, service string) int {
	t.Helper()
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	obs, err := st.QueryObservations(context.Background(), service, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	return len(obs)
}

// certifies: invariant I25 (asserts)
func TestIngestReportNamesReadFailures(t *testing.T) {
	conn := &wedgedConnector{
		refs: []core.SourceRef{
			{Path: "a.go"},
			{Path: "b.go"},
			{Path: "c.go"},
		},
		content: map[string][]byte{"a.go": []byte(honestyGoSrc)},
		fail:    map[string]error{"b.go": epermFor("b.go"), "c.go": epermFor("c.go")},
	}
	useConnectorForIngest(t, conn)
	storePath := filepath.Join(t.TempDir(), "fathom.sqlite")

	var out, errBuf bytes.Buffer
	code := cmdIngest([]string{"--service", "svc", "--path", t.TempDir(), "--store", storePath}, &out, &errBuf)

	if code != 1 {
		t.Fatalf("exit = %d, want 1 (a read failure is never a clean ingest); stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}
	stdout := out.String()
	if !strings.Contains(stdout, "read-failed: 2 file(s)") {
		t.Fatalf("stdout must carry the failed column:\n%s", stdout)
	}
	for _, want := range []string{"b.go", "c.go", "operation not permitted"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout must name %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "(0 failed)") {
		t.Errorf("a wedged ingest must never print a clean (0 failed):\n%s", stdout)
	}
	stderr := errBuf.String()
	if !strings.Contains(stderr, "tccutil reset SystemPolicyRemovableVolumes") {
		t.Errorf("EPERM must carry the external-volume hint on stderr:\n%s", stderr)
	}
	if !strings.Contains(stderr, "operation not permitted") {
		t.Errorf("stderr must name the errno:\n%s", stderr)
	}
	if n := observationCount(t, storePath, "svc"); n == 0 {
		t.Fatal("the readable file must still have been indexed")
	}
}

func TestIngestRefusesEmptyOverwriteOnReadErrors(t *testing.T) {
	refs := []core.SourceRef{
		{Path: "a.go"},
		{Path: "b.go"},
	}
	storePath := filepath.Join(t.TempDir(), "fathom.sqlite")
	repo := t.TempDir()

	// Seed: a healthy ingest fills the index.
	useConnectorForIngest(t, &wedgedConnector{
		refs:    refs,
		content: map[string][]byte{"a.go": []byte(honestyGoSrc), "b.go": []byte(honestyGoSrc)},
	})
	var seedOut, seedErr bytes.Buffer
	if code := cmdIngest([]string{"--service", "svc", "--path", repo, "--store", storePath}, &seedOut, &seedErr); code != 0 {
		t.Fatalf("seed exit = %d, stderr=%s", code, seedErr.String())
	}
	seeded := observationCount(t, storePath, "svc")
	if seeded == 0 {
		t.Fatal("seed ingest stored nothing")
	}

	// Wedge: every read fails; the index must survive untouched.
	useConnectorForIngest(t, &wedgedConnector{
		refs: refs,
		fail: map[string]error{"a.go": epermFor("a.go"), "b.go": epermFor("b.go")},
	})
	var out, errBuf bytes.Buffer
	code := cmdIngest([]string{"--service", "svc", "--path", repo, "--store", storePath}, &out, &errBuf)

	if code != 1 {
		t.Fatalf("exit = %d, want 1; stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}
	stderr := errBuf.String()
	for _, want := range []string{"refused", "a.go", "operation not permitted", "tccutil reset SystemPolicyRemovableVolumes"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr must contain %q:\n%s", want, stderr)
		}
	}
	if got := observationCount(t, storePath, "svc"); got != seeded {
		t.Fatalf("index overwritten: %d observations before the wedge, %d after", seeded, got)
	}
}
