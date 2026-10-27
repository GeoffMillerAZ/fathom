package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// TestCmdIngestRefusesBusyService (GDEV-02): a CLI ingest of a service
// whose ingest_state row another live process holds in_progress (a daemon's
// ingest, or another CLI's) is refused by name: exit 1, --json outcome
// "busy" with the owner's pid and start, the row and the index untouched,
// stderr naming the row (not a certainty) and the remedy. The probe the
// CLI installs is restored when it returns.
func TestCmdIngestRefusesBusyService(t *testing.T) {
	conn := &wedgedConnector{
		refs:    []core.SourceRef{{Path: "a.go"}},
		content: map[string][]byte{"a.go": []byte(honestyGoSrc)},
	}
	useConnectorForIngest(t, conn)
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	var out, errb bytes.Buffer
	if code := cmdIngest([]string{"--service", "svc", "--path", t.TempDir(), "--store", storePath}, &out, &errb); code != 0 {
		t.Fatalf("seed ingest: exit %d: %s", code, errb.String())
	}
	foreign := os.Getpid() + 1
	started := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	row := store.IngestState{ServiceID: "svc", State: store.IngestInProgress, PID: foreign, StartedAt: started.UnixNano(), TargetCommit: "t", Path: "/repo"}
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.(store.IngestStateStore).SetIngestState(context.Background(), row); err != nil {
		t.Fatal(err)
	}
	st.Close()
	before := observationCount(t, storePath, "svc")
	conn.content["a.go"] = []byte("package p\n\nfunc Changed() int { return 2 }\n")

	saved := newIngestOwnerProbe
	newIngestOwnerProbe = func() func(int, time.Time) bool {
		return func(pid int, at time.Time) bool { return pid == foreign && at.Equal(started) }
	}
	t.Cleanup(func() { newIngestOwnerProbe = saved })

	out.Reset()
	errb.Reset()
	code := cmdIngest([]string{"--service", "svc", "--path", t.TempDir(), "--store", storePath, "--json"}, &out, &errb)
	if code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, errb.String())
	}
	var env ingestJSON
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("--json must answer a busy refusal: %v: %q", err, out.String())
	}
	if env.Outcome != "busy" || env.ExitCode != 1 || env.OwnerPID != foreign || env.OwnerStartedAt != "2026-10-26T12:00:00Z" {
		t.Fatalf("envelope: %+v", env)
	}
	wantRow := "pid " + strconv.Itoa(foreign) + " (alive) began an ingest of svc at 2026-10-26T12:00:00Z that has not committed: it is still running or stopped partway"
	if e := errb.String(); !strings.Contains(e, wantRow) || !strings.Contains(e, "ingest tool") || !strings.Contains(e, "restart the daemon") {
		t.Fatalf("stderr must name the row and the remedy: %q", e)
	}
	got, found := readIngestState(t, storePath, "svc")
	if !found || got != row {
		t.Fatalf("ingest_state = %+v, want the owner's row untouched", got)
	}
	if n := observationCount(t, storePath, "svc"); n != before {
		t.Fatalf("observations %d -> %d: a refused ingest wrote", before, n)
	}

	// The CLI's probe is gone with it: a library ingest checks nothing.
	st2, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	if _, err := client.New(conn, st2).Ingest(context.Background(), client.ServiceRef{ID: "svc", Path: t.TempDir()}); err != nil {
		t.Fatalf("the CLI left its probe installed: %v", err)
	}
}
