package main

// storewait.go — bind before the store, and wait for an unavailable store
// instead of exiting (design §8.4; RT-42, RT-165). INC-23's EPERM wedge:
// the store's volume refused the daemon (a TCC verdict), serve-http exited
// 1, launchd relaunched it every 10 s, and every hook saw "daemon down".
// Now the daemon binds first, stays up in `waiting-for-store` — /healthz and
// /statusz answer with the errno, tools/call is 503 — reopens with backoff
// 5 s → 5 min, and names the remedy for the process that actually needs
// the grant: the daemon binary, never the operator's terminal.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/hookguard"
)

// Store-wait labels (§8.4).
const (
	storeWaitTCC         = "tcc-wedge"
	storeWaitUnmounted   = "volume-not-mounted"
	storeWaitUnavailable = "unavailable"
)

// Backoff for reopening the store (§8.4).
var (
	storeWaitFirst = 5 * time.Second
	storeWaitMax   = 5 * time.Minute
)

// sqliteCantOpen is SQLITE_CANTOPEN's primary code.
const sqliteCantOpen = 14

// storeUnavailable reports whether err means the store cannot be reached
// (permission, a missing path, an I/O error, SQLITE_CANTOPEN) — waited on —
// rather than a store that opened and is wrong (returned at once).
func storeUnavailable(err error) bool {
	for _, e := range []syscall.Errno{syscall.EPERM, syscall.EACCES, syscall.ENOENT, syscall.ENOTDIR, syscall.EIO, syscall.EROFS, syscall.ENXIO} {
		if errors.Is(err, e) {
			return true
		}
	}
	var coded interface{ Code() int }
	if errors.As(err, &coded) && coded.Code()&0xff == sqliteCantOpen {
		return true
	}
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unable to open database file")
}

// errnoName names err's errno ("EPERM"), else "".
func errnoName(err error) string {
	var en syscall.Errno
	if !errors.As(err, &en) {
		return ""
	}
	switch en {
	case syscall.EPERM:
		return "EPERM"
	case syscall.EACCES:
		return "EACCES"
	case syscall.ENOENT:
		return "ENOENT"
	case syscall.ENOTDIR:
		return "ENOTDIR"
	case syscall.EIO:
		return "EIO"
	case syscall.EROFS:
		return "EROFS"
	case syscall.ENXIO:
		return "ENXIO"
	}
	return fmt.Sprintf("errno %d", int(en))
}

// volumeRoot is /Volumes/<name> for a path under /Volumes, else "".
func volumeRoot(path string) string {
	rest, ok := strings.CutPrefix(filepath.Clean(path), "/Volumes/")
	if !ok || rest == "" {
		return ""
	}
	name, _, _ := strings.Cut(rest, "/")
	return "/Volumes/" + name
}

// classifyStoreWait labels an unavailable store (§8.4): EPERM on a path
// that stats (the file, or its directory when the file is not there yet)
// is a TCC wedge — the kernel can see it, the policy refuses it; ENOENT
// under /Volumes/ whose volume root does not stat is an unmounted volume;
// anything else is unavailable, errno named. The SQLite VFS drops the
// errno of an existing file it cannot open ("unable to open database file
// (14)") — INC-23's shape exactly — so when err carries none, probe (nil:
// none) opens the path itself to recover it.
func classifyStoreWait(err error, path string, stat, probe func(string) error) (errno, label string) {
	errno = errnoName(err)
	if errno == "" && probe != nil {
		errno = errnoName(probe(path))
	}
	if errno == "" {
		errno = "CANTOPEN"
	}
	switch errno {
	case "EPERM":
		if stat(path) == nil || stat(filepath.Dir(path)) == nil {
			return errno, storeWaitTCC
		}
	case "ENOENT", "ENOTDIR":
		if root := volumeRoot(path); root != "" && stat(root) != nil {
			return errno, storeWaitUnmounted
		}
	}
	return errno, storeWaitUnavailable
}

// storeWaitRemedy names what makes the store reachable (§8.4, RT-165). A
// tcc-wedge's remedy names the responsible process — the daemon binary, its
// signing identity and the Full Disk Access / Removable Volumes grant it
// needs — never a terminal's bundle id: a `tccutil reset` for a terminal
// does nothing for a launchd job. An ad-hoc signature is named as the
// likely cause: every rebuild voids its grant.
func storeWaitRemedy(label, exe string, si signingIdentity, storePath string) string {
	where := firstNonEmpty(volumeRoot(storePath), filepath.Dir(storePath))
	switch label {
	case storeWaitTCC:
		ident := si.identity()
		r := fmt.Sprintf("the daemon binary %s (signing identity: %s) is refused %s by macOS privacy (TCC): grant that binary Full Disk Access (System Settings → Privacy & Security → Full Disk Access → +, choose %s), or its Removable Volumes access for %s — a launchd job's grant is its own binary's, not your terminal's; the daemon reopens the store by itself within 5 min", exe, ident, where, exe, where)
		if si.AdHoc || si.Unsigned {
			r += "; likely cause: the binary is ad-hoc signed or unsigned, so every rebuild voids its grant — install a fathom-dev-signed binary with `task deploy`, then grant it once"
		}
		return r
	case storeWaitUnmounted:
		return fmt.Sprintf("mount the volume %s; the daemon reopens the store by itself (backoff 5 s → 5 min)", where)
	}
	return fmt.Sprintf("the store %s cannot be opened — fix the path or its permissions; the daemon reopens it by itself (backoff 5 s → 5 min)", storePath)
}

// storeWaitState is what the waiting daemon discloses.
type storeWaitState struct {
	Errno     string        `json:"errno"`
	Label     string        `json:"label"`
	Err       string        `json:"error"`
	Remedy    string        `json:"remedy"`
	Since     time.Time     `json:"since"`
	Attempts  int           `json:"attempts"`
	NextRetry time.Duration `json:"-"`
	Open      bool          `json:"-"`
}

// storeWaitDeps is openStoreWithWait's input; every effect is injected.
type storeWaitDeps[T any] struct {
	path     string
	open     func() (T, error)
	stat     func(string) error
	probe    func(string) error
	sleep    func(ctx context.Context, d time.Duration) error
	onWait   func(storeWaitState) // each failed attempt, then once Open after a wait
	now      func() time.Time
	exe      string
	codesign func(exe string) signingIdentity
}

// openStoreWithWait opens the store, waiting (never exiting) while it is
// unavailable: backoff 5 s doubling to 5 min, each attempt disclosed. A
// store error that is not unavailability is returned at once; ctx ends
// the wait.
func openStoreWithWait[T any](ctx context.Context, d storeWaitDeps[T]) (T, error) {
	var zero T
	backoff := storeWaitFirst
	var st storeWaitState
	lastErrno := ""
	for {
		v, err := d.open()
		if err == nil {
			if st.Attempts > 0 {
				st.Open = true
				d.onWait(st)
			}
			return v, nil
		}
		if !storeUnavailable(err) {
			return zero, err
		}
		errno, label := classifyStoreWait(err, d.path, d.stat, d.probe)
		if st.Attempts == 0 {
			st.Since = d.now()
		}
		st.Attempts++
		st.Err = err.Error()
		if errno != lastErrno || st.Label != label {
			st.Errno, st.Label = errno, label
			si := signingIdentity{Binary: d.exe, Error: "not read"}
			if label == storeWaitTCC && d.codesign != nil {
				si = d.codesign(d.exe)
			}
			st.Remedy = storeWaitRemedy(label, d.exe, si, d.path)
			lastErrno = errno
		}
		st.NextRetry = backoff
		d.onWait(st)
		if err := d.sleep(ctx, backoff); err != nil {
			return zero, err
		}
		backoff = min(backoff*2, storeWaitMax)
	}
}

// sleepCtx sleeps d or until ctx ends.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// codesignDescribe reads the daemon binary's signing identity.
func codesignDescribe(exe string) signingIdentity {
	return binarySigningIdentity(realDaemonOps{}, exe)
}

// probeStoreOpen opens the store file read-write as SQLite would, to
// recover the errno the VFS dropped; when the file is not there, the
// directory it would be created in is probed instead. nil: nothing refused
// (the failure is SQLite's own).
func probeStoreOpen(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err == nil {
		_ = f.Close()
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	d, derr := os.Open(filepath.Dir(path))
	if derr != nil {
		return derr
	}
	_ = d.Close()
	return nil
}

// statPath is os.Stat's error.
func statPath(p string) error {
	_, err := os.Stat(p)
	return err
}

// storeUnavailableReason is §3.4's reason for a tools/call while waiting.
func storeUnavailableReason(s *storeWaitState) string {
	msg := s.Err
	var en syscall.Errno
	switch s.Errno {
	case "EPERM":
		en = syscall.EPERM
	case "EACCES":
		en = syscall.EACCES
	case "ENOENT":
		en = syscall.ENOENT
	}
	if en != 0 {
		msg = en.Error()
	}
	return fmt.Sprintf("store unavailable: %s (%s, %s) — %s", msg, s.Errno, s.Label, s.Remedy)
}

// waitingHandler serves while the store is unavailable: /healthz and
// /statusz answer with the errno and label (503), every /rpc tools/call is
// 503 -32050 with retry_after_ms 30000, and anything else is 503 naming
// the wait.
func waitingHandler(st *atomic.Pointer[storeWaitState]) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := st.Load()
		if s == nil {
			s = &storeWaitState{Errno: "unknown", Label: storeWaitUnavailable}
		}
		switch {
		case r.URL.Path == "/healthz":
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "waiting-for-store: %s (%s) — %s", s.Errno, s.Label, s.Remedy)
		case r.URL.Path == "/statusz":
			b, _ := json.Marshal(map[string]any{
				"state": "waiting-for-store", "errno": s.Errno, "label": s.Label, "error": s.Err,
				"remedy": s.Remedy, "since": s.Since.UTC().Format(time.RFC3339), "attempts": s.Attempts,
			})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write(b)
		case r.URL.Path == "/rpc" || strings.HasPrefix(r.URL.Path, "/rpc/"):
			var msg struct {
				ID json.RawMessage `json:"id"`
			}
			_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&msg)
			writeBusy(w, msg.ID, storeUnavailableReason(s), "", hookguard.BusyScopeDaemon, 30*time.Second)
		default:
			http.Error(w, "fathom is waiting for its store: "+storeUnavailableReason(s), http.StatusServiceUnavailable)
		}
	})
}

// swapHandler serves one handler and lets serve-http replace it — the
// waiting handler, then the full one once the store opens — on one
// listener bound before the store.
type swapHandler struct{ h atomic.Pointer[http.Handler] }

func newSwapHandler(h http.Handler) *swapHandler {
	s := &swapHandler{}
	s.set(h)
	return s
}

func (s *swapHandler) set(h http.Handler) { s.h.Store(&h) }

func (s *swapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	(*s.h.Load()).ServeHTTP(w, r)
}
