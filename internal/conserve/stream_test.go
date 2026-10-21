package conserve

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSurfaceReadFailureIsUnproven(t *testing.T) {
	readErr := func(string) ([]byte, bool, error) { return nil, false, errors.New("permission denied") }
	readOK := fileReader(map[string]string{"a.go": "package a\nfunc A() {}\n"})
	for _, tc := range []struct {
		name       string
		base, head func(string) ([]byte, bool, error)
	}{
		{"base", readErr, readOK},
		{"head", readOK, readErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := Input{ChangedFiles: []string{"a.go"}, BaseFile: tc.base, HeadFile: tc.head}
			rep := Run(context.Background(), in, []Plane{surfacePlane{}}).Planes[0]
			if rep.Verdict != Unproven || len(rep.Missing) == 0 || !strings.Contains(strings.Join(rep.Missing, " "), "permission denied") {
				t.Fatalf("read failure became absence: %+v", rep)
			}
		})
	}
}

func TestSurfaceCancellationStopsNextRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	baseReads, headReads := 0, 0
	in := Input{ChangedFiles: []string{"a.go", "b.go"},
		BaseFile: func(string) ([]byte, bool, error) { baseReads++; cancel(); return []byte("package a\n"), true, nil },
		HeadFile: func(string) ([]byte, bool, error) { headReads++; return []byte("package a\n"), true, nil },
	}
	rep := Run(ctx, in, []Plane{surfacePlane{}}).Planes[0]
	if baseReads != 1 || headReads != 0 || rep.Verdict != Unproven || len(rep.Missing) == 0 {
		t.Fatalf("cancel: base=%d head=%d report=%+v", baseReads, headReads, rep)
	}
}

func TestSurfacePreCanceledReadsNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reads := 0
	read := func(string) ([]byte, bool, error) { reads++; return nil, false, nil }
	in := Input{ChangedFiles: []string{"a.go"}, BaseFile: read, HeadFile: read}
	rep := Run(ctx, in, []Plane{surfacePlane{}}).Planes[0]
	if reads != 0 || rep.Verdict != Unproven || len(rep.Missing) == 0 {
		t.Fatalf("pre-cancel: reads=%d report=%+v", reads, rep)
	}
}

func TestSurfaceExtractsBeforeReaderReusesBuffer(t *testing.T) {
	buf := make([]byte, len("package a\nfunc A() {}\n"))
	in := Input{ChangedFiles: []string{"a/a.go", "a/b.go"},
		BaseFile: func(path string) ([]byte, bool, error) {
			if path == "a/a.go" {
				copy(buf, "package a\nfunc A() {}\n")
			} else {
				copy(buf, "package a\nfunc B() {}\n")
			}
			return buf, true, nil
		},
		HeadFile: func(string) ([]byte, bool, error) { return nil, false, nil },
	}
	rep := Run(context.Background(), in, []Plane{surfacePlane{}}).Planes[0]
	keys := map[string]bool{}
	for _, f := range rep.Findings {
		keys[f.Key] = true
	}
	if !keys["api:a.A"] || !keys["api:a.B"] || rep.Verdict != Losses {
		t.Fatalf("reused buffer lost a file's surface: %+v", rep)
	}
}

type countingPlane struct {
	name  string
	calls *int
}

func (p countingPlane) Name() string { return p.name }
func (p countingPlane) Check(context.Context, Input) ([]Finding, []string) {
	*p.calls++
	return nil, nil
}

func TestRunCanceledDoesNotInvokeLaterPlanes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	first := cancelPlane{cancel: cancel}
	rep := Run(ctx, Input{}, []Plane{first, countingPlane{name: "later", calls: &calls}})
	if calls != 0 || len(rep.Planes) != 2 || rep.Planes[0].Verdict != Unproven || rep.Planes[1].Verdict != Unproven || len(rep.Planes[1].Missing) == 0 {
		t.Fatalf("later plane ran or looked conserved: calls=%d report=%+v", calls, rep.Planes)
	}
}

type cancelPlane struct{ cancel context.CancelFunc }

func (cancelPlane) Name() string                                         { return "first" }
func (p cancelPlane) Check(context.Context, Input) ([]Finding, []string) { p.cancel(); return nil, nil }

type cancelAfterChecks struct{ checks, limit int }

func (*cancelAfterChecks) Deadline() (time.Time, bool) { return time.Time{}, false }
func (*cancelAfterChecks) Done() <-chan struct{}       { return nil }
func (c *cancelAfterChecks) Err() error {
	c.checks++
	if c.checks >= c.limit {
		return context.Canceled
	}
	return nil
}
func (*cancelAfterChecks) Value(any) any { return nil }

func TestReadWorktreeFileStopsBetweenChunks(t *testing.T) {
	root := t.TempDir()
	body := bytes.Repeat([]byte("x"), 1024*1024)
	if err := os.WriteFile(filepath.Join(root, "huge.txt"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := &cancelAfterChecks{limit: 3}
	if b, ok, err := readWorktreeFile(ctx, root, "huge.txt"); !errors.Is(err, context.Canceled) || ok || b != nil {
		t.Fatalf("canceled read = %d bytes, ok=%v, err=%v", len(b), ok, err)
	}
	if got, ok, err := worktreeFile(root, "huge.txt"); err != nil || !ok || !bytes.Equal(got, body) {
		t.Fatalf("legacy read = %d bytes, ok=%v, err=%v", len(got), ok, err)
	}
}

func TestNewInputHeadReaderUsesRequestContext(t *testing.T) {
	repo := newInputTestRepo(t)
	repo.write("a.go", "package a\n")
	repo.commitAll("base")
	ctx, cancel := context.WithCancel(context.Background())
	in, err := NewInput(ctx, repo.root, repo.headSHA(), nil, "svc", nil)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if b, ok, err := in.HeadFile("a.go"); !errors.Is(err, context.Canceled) || ok || b != nil {
		t.Fatalf("HeadFile after cancellation: %d bytes, ok=%v, err=%v", len(b), ok, err)
	}
}
