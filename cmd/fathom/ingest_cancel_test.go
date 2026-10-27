package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/hookcache"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// signalConn serves a.go and b.go. Fetch of block (when set) delivers sig
// on the fake signal channel and waits for the ingest's context to end: the
// signal lands before the first write, with no sleep.
type signalConn struct {
	content map[string][]byte
	block   string
	sigs    chan os.Signal
	sig     os.Signal
	stops   *atomic.Int32
	// disarmedFirst records that the seam was stopped before the cancel
	// arrived (a second signal then takes the default action).
	disarmedFirst atomic.Bool
}

// deliver sends the signal and waits for the cancel it causes.
func (c *signalConn) deliver(ctx context.Context) error {
	c.sigs <- c.sig
	select {
	case <-ctx.Done():
		c.disarmedFirst.Store(c.stops.Load() > 0)
		return ctx.Err()
	case <-time.After(waitSafety):
		return errors.New("the signal never cancelled the ingest")
	}
}

func (c *signalConn) Discover(_ context.Context, svc core.ServiceRef) ([]core.SourceRef, error) {
	return []core.SourceRef{{Service: svc.ID, Path: "a.go"}, {Service: svc.ID, Path: "b.go"}}, nil
}

func (c *signalConn) Fetch(ctx context.Context, ref core.SourceRef) ([]byte, error) {
	if ref.Path != c.block {
		return c.content[ref.Path], nil
	}
	return nil, c.deliver(ctx)
}

func (c *signalConn) Watermark(context.Context, core.ServiceRef) (string, error) { return "", nil }

// firstWriteStore, when conn is set, lets the ingest's first observation
// write land and then delivers conn's signal from inside it: the stop
// comes after a write.
type firstWriteStore struct {
	*store.SQLiteStore
	conn *signalConn
	once sync.Once
}

func (s *firstWriteStore) PutObservations(ctx context.Context, obs []core.Observation) error {
	if err := s.SQLiteStore.PutObservations(ctx, obs); err != nil || s.conn == nil {
		return err
	}
	var err error
	s.once.Do(func() { err = s.conn.deliver(ctx) })
	return err
}

// useSignalIngest routes cmdIngest through conn (over a firstWriteStore at
// the --store path, signalling at the first write when afterWrite) and the
// signal seam through conn's channel, counting the seam's stops.
func useSignalIngest(t *testing.T, conn *signalConn, afterWrite bool) {
	t.Helper()
	origFactory, origSignals, origIdent := newIngestClient, ingestSignals, hookcache.IdentDir
	newIngestClient = func(_ client.Mode, storePath string) (*client.Client, error) {
		st, err := store.Open(storePath)
		if err != nil {
			return nil, err
		}
		fw := &firstWriteStore{SQLiteStore: st.(*store.SQLiteStore)}
		if afterWrite {
			fw.conn = conn
		}
		return client.New(conn, fw), nil
	}
	ingestSignals = func() (<-chan os.Signal, func()) {
		return conn.sigs, func() { conn.stops.Add(1) }
	}
	hookcache.IdentDir = filepath.Join(t.TempDir(), "ident")
	t.Cleanup(func() {
		newIngestClient, ingestSignals, hookcache.IdentDir = origFactory, origSignals, origIdent
	})
}

func readIngestState(t *testing.T, storePath, svc string) (store.IngestState, bool) {
	t.Helper()
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	row, found, err := st.(store.IngestStateStore).GetIngestState(context.Background(), svc)
	if err != nil {
		t.Fatal(err)
	}
	return row, found
}

// TestCmdIngestNamesCancellation (GDEV-02): `fathom ingest` stops on SIGINT
// or SIGTERM by name. stderr says `error: ingest cancelled (<signal>)` and
// how far the ingest got; --json says outcome "cancelled"; the exit is 130
// or 143; no SCIP import, [tests] pickup or certify step runs after the
// stop; the seam is disarmed before the cancel, so a second signal takes
// the default action; and ingest_state follows RT-109: the previous row
// back when the stop came before the first write, in_progress after one.
func TestCmdIngestNamesCancellation(t *testing.T) {
	content := map[string][]byte{"a.go": []byte(honestyGoSrc), "b.go": []byte("package p\n\nfunc Other() int { return 2 }\n")}
	run := func(t *testing.T, conn *signalConn, afterWrite bool, args ...string) (int, string, string) {
		t.Helper()
		useSignalIngest(t, conn, afterWrite)
		var out, errb bytes.Buffer
		code := cmdIngest(args, &out, &errb)
		return code, out.String(), errb.String()
	}
	newConn := func(block string, sig os.Signal) *signalConn {
		return &signalConn{content: content, block: block, sigs: make(chan os.Signal, 2), sig: sig, stops: &atomic.Int32{}}
	}
	setup := func(t *testing.T) (root, storePath, scip string) {
		t.Helper()
		root = t.TempDir()
		storePath = filepath.Join(t.TempDir(), "s.sqlite")
		// A SCIP index the post-ingest import would read and warn about.
		scip = filepath.Join(root, "broken.scip")
		if err := os.WriteFile(scip, []byte("not a scip index"), 0o600); err != nil {
			t.Fatal(err)
		}
		// Seed: one ingest that completes, so a previous row exists.
		if code, _, errText := run(t, newConn("", nil), false, "--service", "svc", "--path", root, "--store", storePath); code != 0 {
			t.Fatalf("seed ingest: exit %d: %s", code, errText)
		}
		return root, storePath, scip
	}

	t.Run("control: the trap fires without a signal", func(t *testing.T) {
		root, storePath, scip := setup(t)
		code, _, errText := run(t, newConn("", nil), false, "--service", "svc", "--path", root, "--store", storePath, "--scip", scip)
		if code != 0 || !strings.Contains(errText, "warning: scip import") {
			t.Fatalf("the post-ingest SCIP import must run and warn on an ordinary ingest: exit %d: %s", code, errText)
		}
	})

	t.Run("SIGINT before the first write", func(t *testing.T) {
		root, storePath, scip := setup(t)
		prev, found := readIngestState(t, storePath, "svc")
		if !found || prev.State != store.IngestComplete {
			t.Fatalf("seed row: %+v found=%v", prev, found)
		}
		before := observationCount(t, storePath, "svc")
		conn := newConn("a.go", os.Interrupt)
		code, out, errText := run(t, conn, false, "--service", "svc", "--path", root, "--store", storePath, "--scip", scip)
		if code != 130 {
			t.Fatalf("exit %d, want 130: %s", code, errText)
		}
		if !strings.Contains(errText, "error: ingest cancelled (SIGINT)") || !strings.Contains(errText, "discovered 2") {
			t.Fatalf("stderr must name the cancel and how far it got: %q", errText)
		}
		if strings.Contains(errText, "warning:") || strings.Contains(out, "tests:") || strings.Contains(out, "certify:") {
			t.Fatalf("a post-ingest step ran after the cancel: stdout %q stderr %q", out, errText)
		}
		if !conn.disarmedFirst.Load() {
			t.Fatal("the signal seam must be stopped before the cancel, so a second signal exits at once")
		}
		if row, found := readIngestState(t, storePath, "svc"); !found || row != prev {
			t.Fatalf("ingest_state = %+v found=%v, want the previous row back %+v (RT-109)", row, found, prev)
		}
		if n := observationCount(t, storePath, "svc"); n != before {
			t.Fatalf("observations %d -> %d: a stop before the first write wrote", before, n)
		}
	})

	t.Run("SIGTERM after the first write", func(t *testing.T) {
		root, storePath, scip := setup(t)
		conn := newConn("", syscall.SIGTERM)
		// Changed content: the hash gate would otherwise skip both files and
		// the ingest would write nothing.
		conn.content = map[string][]byte{"a.go": []byte("package p\n\nfunc Changed() int { return 3 }\n"), "b.go": content["b.go"]}
		code, _, errText := run(t, conn, true, "--service", "svc", "--path", root, "--store", storePath, "--scip", scip)
		if code != 143 {
			t.Fatalf("exit %d, want 143: %s", code, errText)
		}
		if !strings.Contains(errText, "error: ingest cancelled (SIGTERM)") || strings.Contains(errText, "warning:") {
			t.Fatalf("stderr: %q", errText)
		}
		row, found := readIngestState(t, storePath, "svc")
		if !found || row.State != store.IngestInProgress || row.PID != os.Getpid() {
			t.Fatalf("ingest_state = %+v found=%v, want in_progress under this pid (RT-109)", row, found)
		}
	})

	t.Run("json", func(t *testing.T) {
		root, storePath, scip := setup(t)
		conn := newConn("a.go", os.Interrupt)
		code, out, errText := run(t, conn, false, "--service", "svc", "--path", root, "--store", storePath, "--scip", scip, "--json")
		if code != 130 {
			t.Fatalf("exit %d, want 130: %s", code, errText)
		}
		var env ingestJSON
		if err := json.Unmarshal([]byte(out), &env); err != nil {
			t.Fatalf("--json must answer on the cancel path: %v: %q", err, out)
		}
		if env.Outcome != "cancelled" || env.ExitCode != 130 || env.Signal != "SIGINT" || env.Discovered != 2 || env.Error == "" {
			t.Fatalf("envelope: %+v", env)
		}
		if len(env.ScipImports) != 0 {
			t.Fatalf("a SCIP import ran after the cancel: %+v", env.ScipImports)
		}
	})
}
