package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	_ "modernc.org/sqlite"
)

type gracefulStoreState struct {
	schema  string
	rowID   int64
	value   string
	rows    int
	version int
}

func runGracefulHungHelper() int {
	code, err := strconv.Atoi(os.Getenv("RT_CLEANUP_CODE"))
	if err != nil {
		return 2
	}
	r := &serveRun{flog: dlog.New(os.Stdout, dlog.LevelDebug, time.Now),
		pid: os.Getpid(), startedAt: time.Now()}
	r.lcIn.ledger = os.Getenv("RT_CLEANUP_LEDGER")
	r.exitRow(tenancy.ExitSignal, "signal before cleanup", code)
	r.deferred(func() { select {} })
	r.watchExitDeadline(code, 200*time.Millisecond)
	r.runCleanups() // the watchdog must terminate this subprocess
	return 99
}

func TestCleanupDeadlineRecordsUncleanAndPreservesExitCode(t *testing.T) {
	for _, code := range []int{143, 0} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			ledger := filepath.Join(t.TempDir(), "lifecycle.jsonl")
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0])
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			cmd.Env = append(os.Environ(), testHelperEnv+"=rt-cleanup-stuck",
				"RT_CLEANUP_CODE="+strconv.Itoa(code), "RT_CLEANUP_LEDGER="+ledger)
			out, err := cmd.CombinedOutput()
			if code == 0 {
				if err != nil {
					t.Fatalf("exit %v, want 0: %s", err, out)
				}
			} else {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != code {
					t.Fatalf("exit %v, want %d: %s", err, code, out)
				}
			}
			if err := syscall.Kill(-cmd.Process.Pid, 0); !errors.Is(err, syscall.ESRCH) {
				t.Fatalf("owned group remains: %v", err)
			}
			rows := readRowsT(t, ledger)
			if len(rows) != 2 || rows[1].Reason != tenancy.ExitUnclean || rows[1].Detail != "cleanup incomplete at exit deadline" {
				t.Fatalf("unclean cleanup not recorded: %+v", rows)
			}
			if !strings.Contains(string(out), "cleanup incomplete at exit deadline") {
				t.Fatalf("incomplete cleanup not logged: %s", out)
			}
		})
	}
}

func TestRequestServeExitWaitsForInflightHTTP(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	r := &serveRun{
		exitRequested: make(chan struct{}), exitDone: make(chan struct{}),
		stopRefresh:   func() {},
		flog:          dlog.New(io.Discard, dlog.LevelDebug, time.Now),
		deadlineTimer: func(time.Duration, func()) {},
	}
	r.exitCtx, r.cancelExit = context.WithCancel(context.Background())
	r.httpSrv = &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(entered)
		<-release
	})}
	serveDone := make(chan struct{})
	go func() { _ = r.httpSrv.Serve(ln); close(serveDone) }()
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	<-entered
	go r.requestServeExit(143, true)
	select {
	case <-r.exitDone:
		t.Fatal("shutdown returned while the request was active")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	select {
	case <-r.exitDone:
	case <-time.After(waitSafety):
		t.Fatal("cooperative request did not finish during shutdown")
	}
	if code, ok := r.requestedExitCode(); !ok || code != 143 {
		t.Fatalf("exit = %d, requested=%v", code, ok)
	}
	<-serveDone
	_, _ = io.Copy(io.Discard, resp.Body)
}

func TestCompetingExitsArmDeadlineWithWinningCode(t *testing.T) {
	for _, first := range []int{143, 130} {
		t.Run(strconv.Itoa(first), func(t *testing.T) {
			armed, release := make(chan struct{}), make(chan struct{})
			var deadline func()
			forced := -1
			r := &serveRun{
				exitRequested: make(chan struct{}), exitDone: make(chan struct{}),
				stopRefresh: func() {}, httpSrv: &http.Server{},
				flog: dlog.New(io.Discard, dlog.LevelDebug, time.Now),
				deadlineTimer: func(_ time.Duration, f func()) {
					deadline = f
					close(armed)
					<-release
				},
				deadlineExit: func(code int) { forced = code },
			}
			r.exitCtx, r.cancelExit = context.WithCancel(context.Background())
			firstDone := make(chan struct{})
			go func() { r.requestServeExit(first, true); close(firstDone) }()
			<-armed // first exit holds the selection while its timer is armed
			secondDone := make(chan struct{})
			go func() { r.requestServeExit(273-first, true); close(secondDone) }()
			close(release)
			<-firstDone
			<-secondDone
			if got, ok := r.requestedExitCode(); !ok || got != first {
				t.Fatalf("selected exit = %d, requested=%v, want %d", got, ok, first)
			}
			deadline()
			if forced != first {
				t.Fatalf("forced exit = %d, selected = %d", forced, first)
			}
		})
	}
}

func TestDeadlineAndCleanupChooseOneTerminalOutcome(t *testing.T) {
	for _, forceFirst := range []bool{false, true} {
		name := "clean first"
		if forceFirst {
			name = "deadline first"
		}
		t.Run(name, func(t *testing.T) {
			ledger := filepath.Join(t.TempDir(), "lifecycle.jsonl")
			forced := -1
			r := &serveRun{flog: dlog.New(io.Discard, dlog.LevelDebug, time.Now),
				pid: os.Getpid(), startedAt: time.Now(), perProcessLedger: true,
				exitRequested: make(chan struct{}), deadlineExit: func(code int) { forced = code },
			}
			r.lcIn.ledger = ledger
			r.exitRow(tenancy.ExitSignal, "signal before cleanup", 143)
			close(r.exitRequested)
			cleanGate, deadlineGate := make(chan struct{}), make(chan struct{})
			cleanDone, deadlineDone := make(chan struct{}), make(chan struct{})
			go func() { <-cleanGate; r.completeCleanup(); close(cleanDone) }()
			go func() { <-deadlineGate; r.forceExitAtDeadline(143); close(deadlineDone) }()
			if forceFirst {
				close(deadlineGate)
				<-deadlineDone
				close(cleanGate)
				<-cleanDone
				rows := readRowsT(t, ledger)
				if forced != 143 || r.terminalState != 2 || len(rows) != 2 || rows[1].Reason != tenancy.ExitUnclean {
					t.Fatalf("forced outcome: code=%d state=%d rows=%+v", forced, r.terminalState, rows)
				}
			} else {
				close(cleanGate)
				<-cleanDone
				close(deadlineGate)
				<-deadlineDone
				if forced != -1 || r.terminalState != 1 {
					t.Fatalf("completed cleanup forced later: code=%d state=%d", forced, r.terminalState)
				}
				if _, err := os.Stat(ledger); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("clean ledger remains: %v", err)
				}
			}
		})
	}
}

func readGracefulStoreState(t *testing.T, path string) gracefulStoreState {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query("SELECT type, name, tbl_name, sql FROM sqlite_schema ORDER BY type, name")
	if err != nil {
		t.Fatal(err)
	}
	var s gracefulStoreState
	for rows.Next() {
		var kind, name, table string
		var statement sql.NullString
		if err := rows.Scan(&kind, &name, &table, &statement); err != nil {
			t.Fatal(err)
		}
		s.schema += fmt.Sprintf("%s|%s|%s|%s\n", kind, name, table, statement.String)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if err := db.QueryRow("SELECT rowid, value FROM graceful_fixture").Scan(&s.rowID, &s.value); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM graceful_fixture").Scan(&s.rows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("PRAGMA user_version").Scan(&s.version); err != nil {
		t.Fatal(err)
	}
	return s
}

// A real test binary runs serve-http in an owned process group. TERM and INT
// must return their conventional codes after the SQLite connection closes,
// without changing the existing schema or row.
func TestServeHTTPSignalsCloseStoreBeforeExit(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns serve-http")
	}
	for _, tc := range []struct {
		name    string
		sig     syscall.Signal
		code    int
		managed bool
	}{
		{"unmanaged TERM", syscall.SIGTERM, 143, false},
		{"unmanaged INT", syscall.SIGINT, 130, false},
		{"managed TERM", syscall.SIGTERM, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "scratch.sqlite")
			st, err := store.NewSQLiteStore(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := st.Close(); err != nil {
				t.Fatal(err)
			}
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec("CREATE TABLE graceful_fixture (value TEXT NOT NULL)"); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec("INSERT INTO graceful_fixture (rowid, value) VALUES (41, 'kept')"); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			before := readGracefulStoreState(t, path)
			addr := freeAddr(t)
			logPath := filepath.Join(dir, "serve.log")
			cmd := exec.Command(os.Args[0])
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			managed := "0"
			if tc.managed {
				managed = "1"
			}
			cmd.Env = append(os.Environ(), testHelperEnv+"=rt-graceful-serve", "RT_GRACEFUL_STORE="+path,
				"RT_GRACEFUL_ADDR="+addr, "RT_GRACEFUL_LOG="+logPath, "RT_GRACEFUL_MANAGED="+managed, "XPC_SERVICE_NAME=")
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			waited := make(chan error, 1)
			go func() { waited <- cmd.Wait() }()
			t.Cleanup(func() { _ = cmd.Process.Kill() })
			logText := func() string { b, _ := os.ReadFile(logPath); return string(b) }
			httpClient := &http.Client{Timeout: 300 * time.Millisecond}
			deadline := time.Now().Add(waitSafety)
			for {
				resp, err := httpClient.Get("http://" + addr + "/healthz")
				if err == nil {
					resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						break
					}
				}
				select {
				case err := <-waited:
					t.Fatalf("daemon exited before serving: %v\n%s", err, logText())
				default:
				}
				if time.Now().After(deadline) {
					t.Fatalf("daemon never served: %s", logText())
				}
				time.Sleep(50 * time.Millisecond)
			}
			if err := cmd.Process.Signal(tc.sig); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-waited:
				if tc.code == 0 {
					if err != nil {
						t.Fatalf("exit = %v, want 0\n%s", err, logText())
					}
				} else {
					var exit *exec.ExitError
					if !errors.As(err, &exit) || exit.ExitCode() != tc.code {
						t.Fatalf("exit = %v, want %d\n%s", err, tc.code, logText())
					}
				}
			case <-time.After(waitSafety):
				t.Fatalf("daemon did not exit after %s\n%s", tc.name, logText())
			}
			if err := syscall.Kill(-cmd.Process.Pid, 0); !errors.Is(err, syscall.ESRCH) {
				t.Fatalf("owned process group %d remains: %v", cmd.Process.Pid, err)
			}
			for _, sidecar := range []string{"-wal", "-shm"} {
				if _, err := os.Stat(path + sidecar); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("SQLite sidecar %s remains: %v", sidecar, err)
				}
			}
			if after := readGracefulStoreState(t, path); !reflect.DeepEqual(after, before) {
				t.Fatalf("store changed across signal: before=%+v after=%+v", before, after)
			}
			wantReason := "signal "
			if tc.managed {
				wantReason = "drain: done"
			}
			if !strings.Contains(logText(), wantReason) {
				t.Fatalf("exit reason %q not logged:\n%s", wantReason, logText())
			}
		})
	}
}
