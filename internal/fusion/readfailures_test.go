package fusion_test

// Ingest honesty under a wedged reader (docs/plans/instrument-truth.md
// IT-14 / IT-15, .fathom/observations.md O16): a per-file read failure is
// recorded and named, never fatal and never silent; an ingest that could
// read NOTHING refuses to touch an index that already holds evidence.

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"os"
	"sort"
	"strings"
	"syscall"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// readErrConn is a Connector whose Fetch fails with a *fs.PathError for the
// refs listed in fail — the exact error shape os.ReadFile returns under an
// EPERM/EACCES/EIO verdict — and serves content for the rest. No real TCC
// grant is ever touched.
type readErrConn struct {
	refs    []core.SourceRef
	content map[string][]byte
	fail    map[string]error
}

func (c *readErrConn) Discover(_ context.Context, svc core.ServiceRef) ([]core.SourceRef, error) {
	out := make([]core.SourceRef, len(c.refs))
	for i, r := range c.refs {
		r.Service = svc.ID
		out[i] = r
	}
	return out, nil
}

func (c *readErrConn) Fetch(_ context.Context, ref core.SourceRef) ([]byte, error) {
	if err, ok := c.fail[ref.Path]; ok {
		return nil, err
	}
	return c.content[ref.Path], nil
}

func (c *readErrConn) Watermark(context.Context, core.ServiceRef) (string, error) { return "", nil }

func eperm(path string) error {
	return &fs.PathError{Op: "open", Path: "/vol/repo/" + path, Err: syscall.EPERM}
}

const goSrc = "package p\n\nfunc Hello() string { return \"hi\" }\n"

func goRef(path string) core.SourceRef {
	return core.SourceRef{Path: path}
}

func newHonestyPipeline(conn *readErrConn, st store.Store) *fusion.Pipeline {
	p := fusion.New(conn, st, fusion.Config{Concurrency: 3, DisableTypedEdges: true})
	p.WithGitHead(func(string) string { return "" })
	p.WithGitLog(func(string) string { return "" })
	return p
}

func TestIngest_ReadFailuresRecordedNotFatal(t *testing.T) {
	conn := &readErrConn{
		refs:    []core.SourceRef{goRef("a.go"), goRef("b.go"), goRef("c.go")},
		content: map[string][]byte{"a.go": []byte(goSrc)},
		fail:    map[string]error{"c.go": eperm("c.go"), "b.go": eperm("b.go")},
	}
	st := store.NewMemoryStore()
	p := newHonestyPipeline(conn, st)

	report, err := p.Ingest(context.Background(), core.ServiceRef{ID: "svc", Path: t.TempDir()})

	var rfe *fusion.ReadFailuresError
	if !errors.As(err, &rfe) {
		t.Fatalf("Ingest error = %v, want *fusion.ReadFailuresError", err)
	}
	if rfe.Refused {
		t.Fatalf("a partial read failure over an empty store must not be a refusal: %v", rfe)
	}
	if report.ReadFailed != 2 || len(report.ReadFailures) != 2 {
		t.Fatalf("ReadFailed = %d, ReadFailures = %v, want 2", report.ReadFailed, report.ReadFailures)
	}
	if !sort.SliceIsSorted(report.ReadFailures, func(i, j int) bool {
		return report.ReadFailures[i].Path < report.ReadFailures[j].Path
	}) {
		t.Fatalf("ReadFailures not sorted by path: %v", report.ReadFailures)
	}
	if got := report.ReadFailures[0]; got.Path != "b.go" || !strings.Contains(got.Err, "operation not permitted") {
		t.Fatalf("ReadFailures[0] = %+v, want b.go with the errno text", got)
	}
	if report.Discovered != 3 || report.Extracted != 1 || report.Skipped != 0 {
		t.Fatalf("counts = discovered %d extracted %d skipped %d, want 3/1/0", report.Discovered, report.Extracted, report.Skipped)
	}
	if report.Symbols == 0 {
		t.Fatal("the readable file must still be extracted and stored")
	}
	if !strings.Contains(rfe.Error(), "b.go") || !strings.Contains(rfe.Error(), "operation not permitted") {
		t.Fatalf("error must name the first failed path and errno: %q", rfe.Error())
	}
	if !strings.Contains(rfe.Hint(), "tccutil reset SystemPolicyRemovableVolumes") {
		t.Fatalf("EPERM must carry the external-volume hint, got %q", rfe.Hint())
	}
	obs, qErr := st.QueryObservations(context.Background(), "svc", "")
	if qErr != nil || len(obs) == 0 {
		t.Fatalf("readable file's observations must be stored: %d, %v", len(obs), qErr)
	}
}

func TestIngest_RefusesEmptyOverwriteWhenNothingReadable(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	good := &readErrConn{
		refs:    []core.SourceRef{goRef("a.go"), goRef("b.go")},
		content: map[string][]byte{"a.go": []byte(goSrc), "b.go": []byte(goSrc)},
	}
	if _, err := newHonestyPipeline(good, st).Ingest(ctx, core.ServiceRef{ID: "svc", Path: t.TempDir()}); err != nil {
		t.Fatalf("seed ingest: %v", err)
	}
	before, _ := st.QueryObservations(ctx, "svc", "")
	if len(before) == 0 {
		t.Fatal("seed ingest stored nothing")
	}
	metaBefore, _, _ := st.GetServiceMeta(ctx, "svc")

	wedged := &readErrConn{
		refs: good.refs,
		fail: map[string]error{"a.go": eperm("a.go"), "b.go": eperm("b.go")},
	}
	report, err := newHonestyPipeline(wedged, st).Ingest(ctx, core.ServiceRef{ID: "svc", Path: t.TempDir()})

	var rfe *fusion.ReadFailuresError
	if !errors.As(err, &rfe) || !rfe.Refused {
		t.Fatalf("Ingest error = %v, want a Refused *fusion.ReadFailuresError", err)
	}
	if rfe.Existing != len(before) {
		t.Fatalf("Existing = %d, want %d", rfe.Existing, len(before))
	}
	if report.ReadFailed != 2 || report.Symbols != 0 {
		t.Fatalf("report = %+v, want ReadFailed 2 / Symbols 0", report)
	}
	after, _ := st.QueryObservations(ctx, "svc", "")
	if len(after) != len(before) {
		t.Fatalf("refusal must leave the index untouched: %d observations before, %d after", len(before), len(after))
	}
	metaAfter, _, _ := st.GetServiceMeta(ctx, "svc")
	if metaAfter.IndexedAt != metaBefore.IndexedAt {
		t.Fatalf("refusal must not advance the freshness watermark: %d → %d", metaBefore.IndexedAt, metaAfter.IndexedAt)
	}
	if !strings.Contains(rfe.Error(), "refused") || !strings.Contains(rfe.Error(), "a.go") {
		t.Fatalf("error must name the cause: %q", rfe.Error())
	}
}

func TestIngest_AllReadsFailOverEmptyStoreIsNotRefused(t *testing.T) {
	wedged := &readErrConn{
		refs: []core.SourceRef{goRef("a.go")},
		fail: map[string]error{"a.go": eperm("a.go")},
	}
	st := store.NewMemoryStore()
	report, err := newHonestyPipeline(wedged, st).Ingest(context.Background(), core.ServiceRef{ID: "svc", Path: t.TempDir()})
	var rfe *fusion.ReadFailuresError
	if !errors.As(err, &rfe) || rfe.Refused {
		t.Fatalf("Ingest error = %v, want a non-refused *fusion.ReadFailuresError (nothing to protect)", err)
	}
	if report.ReadFailed != 1 {
		t.Fatalf("ReadFailed = %d, want 1", report.ReadFailed)
	}
}

func TestIngest_NonReadFetchErrorStaysFatal(t *testing.T) {
	transport := errors.New("connector: transport reset")
	conn := &readErrConn{
		refs:    []core.SourceRef{goRef("a.go"), goRef("b.go")},
		content: map[string][]byte{"a.go": []byte(goSrc)},
		fail:    map[string]error{"b.go": transport},
	}
	st := store.NewMemoryStore()
	_, err := newHonestyPipeline(conn, st).Ingest(context.Background(), core.ServiceRef{ID: "svc", Path: t.TempDir()})
	if !errors.Is(err, transport) {
		t.Fatalf("Ingest error = %v, want the transport error to abort", err)
	}
	var rfe *fusion.ReadFailuresError
	if errors.As(err, &rfe) {
		t.Fatalf("a transport error is not a read failure: %v", err)
	}
	if obs, _ := st.QueryObservations(context.Background(), "svc", ""); len(obs) != 0 {
		t.Fatalf("a fatal fetch must write nothing, got %d observations", len(obs))
	}
}

func TestReadFailuresError_HintOnlyForEPERM(t *testing.T) {
	e := &fusion.ReadFailuresError{Service: "svc", Failures: []fusion.ReadFailure{{Path: "a.go", Err: "open /vol/a.go: permission denied"}}}
	if e.Hint() != "" {
		t.Fatalf("EACCES is a real permission problem, not a TCC wedge: %q", e.Hint())
	}
	e.Failures = append(e.Failures, fusion.ReadFailure{Path: "b.go", Err: "open /vol/b.go: operation not permitted"})
	if !strings.Contains(e.Hint(), "tccutil reset SystemPolicyRemovableVolumes") {
		t.Fatalf("EPERM must carry the hint: %q", e.Hint())
	}
}

// assertFatal runs an ingest whose b.go Fetch fails with fetchErr and
// asserts the classic abort: the error chain carries fetchErr, no
// *ReadFailuresError is attached, and nothing was written.
func assertFatal(t *testing.T, fetchErr error, why string) {
	t.Helper()
	conn := &readErrConn{
		refs:    []core.SourceRef{goRef("a.go"), goRef("b.go")},
		content: map[string][]byte{"a.go": []byte(goSrc)},
		fail:    map[string]error{"b.go": fetchErr},
	}
	st := store.NewMemoryStore()
	report, err := newHonestyPipeline(conn, st).Ingest(context.Background(), core.ServiceRef{ID: "svc", Path: t.TempDir()})
	if !errors.Is(err, fetchErr) {
		t.Fatalf("%s: Ingest error = %v, want the fetch error to abort", why, err)
	}
	var rfe *fusion.ReadFailuresError
	if errors.As(err, &rfe) {
		t.Fatalf("%s: classified as a read failure: %v", why, err)
	}
	if report.ReadFailed != 0 {
		t.Fatalf("%s: ReadFailed = %d, want 0", why, report.ReadFailed)
	}
	if obs, _ := st.QueryObservations(context.Background(), "svc", ""); len(obs) != 0 {
		t.Fatalf("%s: a fatal fetch must write nothing, got %d observations", why, len(obs))
	}
}

// TestIngest_TransportErrnoStaysFatal pins the narrow classifier: an errno
// that arrives through a transport (*net.OpError), a bare errno, or a
// fork/exec *fs.PathError is NOT a per-file read failure even though each
// bottoms out in a syscall.Errno — only a file-read PathError / SyscallError
// with a reader's errno is skipped-and-named.
func TestIngest_TransportErrnoStaysFatal(t *testing.T) {
	t.Run("net.OpError ECONNREFUSED", func(t *testing.T) {
		assertFatal(t, &net.OpError{Op: "dial", Net: "tcp", Err: &os.SyscallError{Syscall: "connect", Err: syscall.ECONNREFUSED}}, "dial refused")
	})
	t.Run("net.OpError ECONNRESET", func(t *testing.T) {
		assertFatal(t, &net.OpError{Op: "read", Net: "tcp", Err: &os.SyscallError{Syscall: "read", Err: syscall.ECONNRESET}}, "transport reset")
	})
	t.Run("bare errno", func(t *testing.T) {
		assertFatal(t, syscall.ECONNRESET, "bare errno")
	})
	t.Run("bare fs sentinel", func(t *testing.T) {
		assertFatal(t, fs.ErrPermission, "bare sentinel without a file-read PathError")
	})
	t.Run("fork/exec EACCES", func(t *testing.T) {
		assertFatal(t, &fs.PathError{Op: "fork/exec", Path: "/usr/local/bin/gh", Err: syscall.EACCES}, "fork/exec is not a file read")
	})
	t.Run("file-read PathError with a transport errno", func(t *testing.T) {
		assertFatal(t, &fs.PathError{Op: "read", Path: "/vol/repo/b.go", Err: syscall.ECONNRESET}, "reader errno set is closed")
	})
}

// TestIngest_ReaderErrnosAreReadFailures pins the admitted set: each errno a
// file read produces is recorded and skipped, never fatal; only EPERM
// carries the external-volume hint.
func TestIngest_ReaderErrnosAreReadFailures(t *testing.T) {
	cases := []struct {
		name  string
		err   error
		hint  bool
		errno string
	}{
		{"open EPERM", &fs.PathError{Op: "open", Path: "/vol/repo/b.go", Err: syscall.EPERM}, true, "operation not permitted"},
		{"open EACCES", &fs.PathError{Op: "open", Path: "/vol/repo/b.go", Err: syscall.EACCES}, false, "permission denied"},
		{"read EIO", &fs.PathError{Op: "read", Path: "/vol/repo/b.go", Err: syscall.EIO}, false, "input/output error"},
		{"open ENOENT (deleted between Discover and Fetch)", &fs.PathError{Op: "open", Path: "/vol/repo/b.go", Err: syscall.ENOENT}, false, "no such file or directory"},
		{"open ENOTDIR", &fs.PathError{Op: "open", Path: "/vol/repo/b.go", Err: syscall.ENOTDIR}, false, "not a directory"},
		{"read EISDIR", &fs.PathError{Op: "read", Path: "/vol/repo/b.go", Err: syscall.EISDIR}, false, "is a directory"},
		{"SyscallError read EIO", &os.SyscallError{Syscall: "read", Err: syscall.EIO}, false, "input/output error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conn := &readErrConn{
				refs:    []core.SourceRef{goRef("a.go"), goRef("b.go")},
				content: map[string][]byte{"a.go": []byte(goSrc)},
				fail:    map[string]error{"b.go": tc.err},
			}
			st := store.NewMemoryStore()
			report, err := newHonestyPipeline(conn, st).Ingest(context.Background(), core.ServiceRef{ID: "svc", Path: t.TempDir()})
			var rfe *fusion.ReadFailuresError
			if !errors.As(err, &rfe) || rfe.Refused {
				t.Fatalf("Ingest error = %v, want a non-refused *fusion.ReadFailuresError", err)
			}
			if report.ReadFailed != 1 || report.Extracted != 1 || report.Symbols == 0 {
				t.Fatalf("report = %+v, want ReadFailed 1 / Extracted 1 / Symbols > 0", report)
			}
			if got := report.ReadFailures[0]; got.Path != "b.go" || !strings.Contains(got.Err, tc.errno) {
				t.Fatalf("ReadFailures[0] = %+v, want b.go with %q", got, tc.errno)
			}
			if (rfe.Hint() != "") != tc.hint {
				t.Fatalf("Hint() = %q, want hint=%v", rfe.Hint(), tc.hint)
			}
		})
	}
}

// purgeFailStore fails the orphan purge's first store call after every
// write has landed — the shape of a store fault that arrives once the
// readable files are already indexed.
type purgeFailStore struct {
	store.Store
	err error
}

func (s *purgeFailStore) ListSourceRefs(context.Context, string) ([]string, error) {
	return nil, s.err
}

// TestIngest_PurgeErrorKeepsReadFailureSignal: when the orphan purge fails
// on an ingest that also hit read failures, BOTH signals survive in the
// returned error — an err-only caller sees the purge abort, an errors.As
// caller still finds the *ReadFailuresError it must name.
func TestIngest_PurgeErrorKeepsReadFailureSignal(t *testing.T) {
	purgeErr := errors.New("store: list source refs: disk gone")
	conn := &readErrConn{
		refs:    []core.SourceRef{goRef("a.go"), goRef("b.go")},
		content: map[string][]byte{"a.go": []byte(goSrc)},
		fail:    map[string]error{"b.go": eperm("b.go")},
	}
	inner := store.NewMemoryStore()
	st := &purgeFailStore{Store: inner, err: purgeErr}
	report, err := newHonestyPipeline(conn, st).Ingest(context.Background(), core.ServiceRef{ID: "svc", Path: t.TempDir()})

	if !errors.Is(err, purgeErr) {
		t.Fatalf("Ingest error = %v, want the purge error in the chain", err)
	}
	var rfe *fusion.ReadFailuresError
	if !errors.As(err, &rfe) || rfe.Refused {
		t.Fatalf("Ingest error = %v, want the non-refused *fusion.ReadFailuresError to survive the purge error", err)
	}
	if len(rfe.Failures) != 1 || rfe.Failures[0].Path != "b.go" {
		t.Fatalf("Failures = %+v, want b.go", rfe.Failures)
	}
	for _, want := range []string{"disk gone", "1 file(s) could not be read", "b.go"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("joined error must carry %q:\n%s", want, err.Error())
		}
	}
	if report.ReadFailed != 1 || report.Symbols == 0 {
		t.Fatalf("report = %+v, want ReadFailed 1 and the readable file's symbols", report)
	}
	if obs, _ := inner.QueryObservations(context.Background(), "svc", ""); len(obs) == 0 {
		t.Fatal("the readable file's observations were stored before the purge failed; they must remain")
	}
}

// TestIngest_PurgeErrorWithoutReadFailuresIsUnchanged: with no read
// failures, finish passes the purge error through untouched — no join, no
// *ReadFailuresError.
func TestIngest_PurgeErrorWithoutReadFailuresIsUnchanged(t *testing.T) {
	purgeErr := errors.New("store: list source refs: disk gone")
	conn := &readErrConn{
		refs:    []core.SourceRef{goRef("a.go")},
		content: map[string][]byte{"a.go": []byte(goSrc)},
	}
	st := &purgeFailStore{Store: store.NewMemoryStore(), err: purgeErr}
	_, err := newHonestyPipeline(conn, st).Ingest(context.Background(), core.ServiceRef{ID: "svc", Path: t.TempDir()})
	if !errors.Is(err, purgeErr) {
		t.Fatalf("Ingest error = %v, want the purge error", err)
	}
	var rfe *fusion.ReadFailuresError
	if errors.As(err, &rfe) {
		t.Fatalf("no read failures happened, yet a *ReadFailuresError was attached: %v", err)
	}
}
