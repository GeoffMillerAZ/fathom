package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/hookguard"
)

// statuslineFake is a fake daemon answering GET /statusz with body (or a
// status), recording each root it was asked about.
type statuslineFake struct {
	srv   *httptest.Server
	mu    sync.Mutex
	roots []string
}

func newStatuslineFake(t *testing.T, status int, body string) *statuslineFake {
	t.Helper()
	f := &statuslineFake{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.roots = append(f.roots, r.URL.Query().Get("root"))
		f.mu.Unlock()
		if r.URL.Path != "/statusz" || r.URL.Query().Get("format") != "json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	url := f.srv.URL
	t.Cleanup(func() {
		f.srv.Close()
		(&hookguard.Guard{}).RecordSuccess(url) // leave no breaker state for a reused port
	})
	return f
}

func (f *statuslineFake) asked() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.roots...)
}

// statuslineAnswerJSON is a root answer for one service.
func statuslineAnswerJSON(behind, dirty string) string {
	return `{"schema_version":3,"root":{"dir":"/w/repo","indexed":true,"service":"svc"},"services":[{"service":"svc",` +
		`"indexed_commit":"abcdef1234567890abcdef1234567890abcdef12","head_commit":"abcdef1234567890abcdef1234567890abcdef12",` +
		`"behind":` + behind + `,"dirty":` + dirty + `,"future_field":{"x":1}}],"also_new":[1,2]}`
}

func runStatusline(t *testing.T, stdin string, args ...string) string {
	t.Helper()
	var out, errb bytes.Buffer
	if code := cmdStatusline(args, strings.NewReader(stdin), &out, &errb); code != 0 {
		t.Fatalf("statusline %v exit %d, stderr %q", args, code, errb.String())
	}
	return out.String()
}

const statuslineStdin = `{"session_id":"s","workspace":{"project_dir":"/w/repo","current_dir":"/w/repo/sub","later":true},"cwd":"/w/repo/sub","agentType":"Explore","effort":"high","model":{"id":"m"}}`

// TestStatuslinePrintsOneOfFiveStatesNeverBlank drives `fathom statusline`
// through every state: indexed at HEAD (with the dirty qualifier and its
// unknown), commits behind (and behind unknown), daemon down, not checked
// with the hook guard's verdict when no GET is sent, and not indexed.
func TestStatuslinePrintsOneOfFiveStatesNeverBlank(t *testing.T) {
	for _, tc := range []struct {
		name, behind, dirty, want string
	}{
		{"indexed at HEAD, clean", "0", "false", "fathom ● @abcdef1\n"},
		{"indexed at HEAD, uncommitted", "0", "true", "fathom ● @abcdef1 +uncommitted\n"},
		{"indexed at HEAD, dirty unknown", "0", "null", "fathom ● @abcdef1 dirty unknown\n"},
		{"behind", "3", "false", "fathom ◐ 3 behind @abcdef1\n"},
		{"behind unknown", "null", "null", "fathom ◐ behind unknown @abcdef1\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStatuslineFake(t, http.StatusOK, statuslineAnswerJSON(tc.behind, tc.dirty))
			got := runStatusline(t, statuslineStdin, "--url", f.srv.URL)
			if got != tc.want {
				t.Fatalf("line = %q, want %q", got, tc.want)
			}
			if asked := f.asked(); len(asked) != 1 || asked[0] != "/w/repo" {
				t.Fatalf("roots asked = %q, want one GET for the project dir", asked)
			}
			if strings.Contains(got, "0 behind") {
				t.Fatalf("line %q reads an unknown as 0 behind", got)
			}
		})
	}

	t.Run("not indexed", func(t *testing.T) {
		f := newStatuslineFake(t, http.StatusOK, `{"root":{"dir":"/w/repo","indexed":false},"services":[]}`)
		if got := runStatusline(t, statuslineStdin, "--url", f.srv.URL); got != "fathom ? not indexed\n" {
			t.Fatalf("line = %q", got)
		}
	})

	for _, tc := range []struct{ name, body string }{
		{"a refused connection", ""},
		{"an error status", "boom"},
		{"an unreadable answer", "{not json"},
		{"a daemon that predates the root query", `{"schema_version":3,"services":[{"service":"svc"}]}`},
		{"a daemon that could not list its services", `{"root":{"dir":"/w/repo","indexed":null,"reason":"x"},"services":[]}`},
	} {
		t.Run("daemon down: "+tc.name, func(t *testing.T) {
			status := http.StatusOK
			if tc.name == "an error status" {
				status = http.StatusInternalServerError
			}
			f := newStatuslineFake(t, status, tc.body)
			url := f.srv.URL
			if tc.body == "" {
				f.srv.Close()
			}
			if got := runStatusline(t, statuslineStdin, "--url", url); got != "fathom ○ daemon down\n" {
				t.Fatalf("line = %q", got)
			}
		})
	}

	t.Run("not checked", func(t *testing.T) {
		for _, tc := range []struct {
			verdict string
			arm     func(url string)
		}{
			{"breaker_open", func(url string) {
				g := &hookguard.Guard{}
				g.RecordFailure(url, errors.New("refused"))
				g.RecordFailure(url, errors.New("refused"))
			}},
			{"busy", func(url string) { (&hookguard.Guard{}).RecordBusy(url, 30*time.Second) }},
			{"disabled", func(url string) { writeStatuslineSwitch(t, "disabled.json", url) }},
			{"paused", func(url string) { writeStatuslineSwitch(t, "paused.json", url) }},
		} {
			t.Run(tc.verdict, func(t *testing.T) {
				f := newStatuslineFake(t, http.StatusOK, statuslineAnswerJSON("0", "false"))
				tc.arm(f.srv.URL)
				want := "fathom · not checked (" + tc.verdict + ")\n"
				if got := runStatusline(t, statuslineStdin, "--url", f.srv.URL); got != want {
					t.Fatalf("line = %q, want %q", got, want)
				}
				if asked := f.asked(); len(asked) != 0 {
					t.Fatalf("the guard sent %d GETs, want none", len(asked))
				}
			})
		}
	})

	t.Run("stdin read defensively", func(t *testing.T) {
		wd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct{ name, stdin, wantRoot string }{
			{"malformed", "{not json", wd},
			{"empty", "", wd},
			{"wrong types", `{"workspace":"nope","cwd":7}`, wd},
			{"only the working directory", `{"workspace":{"current_dir":"/w/x"}}`, "/w/x"},
			{"a relative project dir", `{"workspace":{"project_dir":"rel"},"cwd":"/w/y"}`, "/w/y"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				f := newStatuslineFake(t, http.StatusOK, statuslineAnswerJSON("0", "false"))
				if got := runStatusline(t, tc.stdin, "--url", f.srv.URL); got != "fathom ● @abcdef1\n" {
					t.Fatalf("line = %q", got)
				}
				if asked := f.asked(); len(asked) != 1 || asked[0] != tc.wantRoot {
					t.Fatalf("roots asked = %q, want %q", asked, tc.wantRoot)
				}
			})
		}
	})

	t.Run("--json and --subagent print the same state as one object", func(t *testing.T) {
		f := newStatuslineFake(t, http.StatusOK, statuslineAnswerJSON("2", "null"))
		raw := runStatusline(t, statuslineStdin, "--url", f.srv.URL, "--subagent", "--json")
		var obj map[string]any
		if err := json.Unmarshal([]byte(raw), &obj); err != nil || strings.Count(strings.TrimSpace(raw), "\n") != 0 {
			t.Fatalf("not one JSON object on one line: %v\n%s", err, raw)
		}
		if obj["state"] != "behind" || obj["line"] != "fathom ◐ 2 behind @abcdef1" || obj["behind"] != float64(2) {
			t.Fatalf("object = %v", obj)
		}
		if v, ok := obj["dirty"]; !ok || v != nil {
			t.Fatalf("dirty = %v (present %v), want an explicit null", v, ok)
		}
		if got := runStatusline(t, statuslineStdin, "--url", f.srv.URL, "--subagent"); got != "fathom ◐ 2 behind @abcdef1\n" {
			t.Fatalf("--subagent line = %q", got)
		}
	})
}

// writeStatuslineSwitch writes the managed daemon's kill-switch file naming
// url (the hermetic home TestMain set), removed at cleanup.
func writeStatuslineSwitch(t *testing.T, name, url string) {
	t.Helper()
	dir := fathomhome.Path("daemon")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(`{"reason":"test","url":"`+url+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(p) })
}

// TestStatuslineCPUTimeP95AtMost100ms measures the CPU time one status-line
// run costs (process CPU, not wall clock: parallel gates make wall-clock
// bounds flip) over 200 runs against a fake daemon, and bounds the wall
// time of a run against a daemon that hangs by the status line's deadline.
// The runs are in process, so the binary's start-up is not in the sample.
func TestStatuslineCPUTimeP95AtMost100ms(t *testing.T) {
	if _, ok := processCPUTime(); !ok {
		t.Skip("process CPU time is not readable on this platform")
	}
	f := newStatuslineFake(t, http.StatusOK, statuslineAnswerJSON("0", "false"))
	const runs = 200
	samples := make([]time.Duration, 0, runs)
	for i := 0; i < runs; i++ {
		c0, _ := processCPUTime()
		got := runStatusline(t, statuslineStdin, "--url", f.srv.URL)
		c1, _ := processCPUTime()
		if got != "fathom ● @abcdef1\n" {
			t.Fatalf("run %d line = %q", i, got)
		}
		samples = append(samples, c1-c0)
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	p95 := samples[runs*95/100-1]
	t.Logf("status line CPU over %d runs: p50 %v, p95 %v, max %v", runs, samples[runs/2-1], p95, samples[runs-1])
	if p95 > 100*time.Millisecond {
		t.Fatalf("CPU time p95 = %v, want at most 100ms", p95)
	}

	t.Run("a hung daemon is bounded by the deadline", func(t *testing.T) {
		release := make(chan struct{})
		hang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-release:
			case <-time.After(10 * time.Second):
			}
		}))
		url := hang.URL
		t.Cleanup(func() {
			close(release)
			hang.Close()
			(&hookguard.Guard{}).RecordSuccess(url)
		})
		start := time.Now()
		got := runStatusline(t, statuslineStdin, "--url", url)
		elapsed := time.Since(start)
		if got != "fathom ○ daemon down\n" {
			t.Fatalf("line = %q", got)
		}
		budget := statuslineDeadline(start).Budget()
		if budget <= 0 || elapsed > budget+2*time.Second {
			t.Fatalf("a hung daemon held the status line %v, want it bounded by the %v deadline", elapsed, budget)
		}
	})
}

// TestStatuslineJudgedTokenNeverPassStyled is JG-4 (ADR-073 decision 6):
// a configured decision port adds J:off, J:on(<n>) or J:killed after the
// state, plain text with no colour escape and none of the states' glyphs,
// so it never reads as a pass; --json keeps it out of state and line (the
// evidence band's and the proof line's inputs) in decision_port, with the
// kill source KS-6's gate names; with no [decide] block in either layer the
// bytes are unchanged.
func TestStatuslineJudgedTokenNeverPassStyled(t *testing.T) {
	f := newStatuslineFake(t, http.StatusOK, statuslineAnswerJSON("0", "false"))
	const plain = "fathom ● @abcdef1"
	for _, tc := range []struct {
		name, user, project, token, source string
		setup                              func(t *testing.T, home string)
	}{
		{name: "no [decide] block", user: "[hooks]\npin_check = \"record\"\n", project: "[readouts]\n"},
		{name: "provider none", project: "[decide]\nprovider = \"none\"\n", token: "J:off"},
		{name: "open", user: "[decide]\nprovider = \"typesafe\"\napi_key_env = \"FATHOM_TEST_JG4_KEY\"\nsites = [\"route\", \"certify_suggest\"]\n", token: "J:on(2)"},
		{name: "no open site", user: "[decide]\nprovider = \"typesafe\"\nsites = [\"route\"]\ndeny_sites = [\"route\"]\n", token: "J:off"},
		{name: "killed by the file", user: "[decide]\nprovider = \"typesafe\"\n", token: "J:killed", source: decide.KillSourceFile,
			setup: func(t *testing.T, home string) {
				if err := os.WriteFile(filepath.Join(home, decide.KillFileName), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "killed by the environment", user: "[decide]\nprovider = \"typesafe\"\n", token: "J:killed", source: decide.KillSourceEnv,
			setup: func(t *testing.T, _ string) { t.Setenv(decide.KillEnv, decide.KillValue) }},
		{name: "killed by enabled", user: "[decide]\nprovider = \"typesafe\"\nenabled = false\n", token: "J:killed", source: decide.KillSourceEnabled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("FATHOM_HOME", home)
			t.Setenv(decide.KillEnv, "")
			repo := t.TempDir()
			if tc.user != "" {
				if err := os.WriteFile(fathomhome.Path("config.toml"), []byte(tc.user), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.project != "" {
				if err := os.MkdirAll(filepath.Join(repo, ".fathom"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(repo, ".fathom", "config.toml"), []byte(tc.project), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.setup != nil {
				tc.setup(t, home)
			}
			stdin := `{"workspace":{"project_dir":"` + repo + `"}}`
			got := runStatusline(t, stdin, "--url", f.srv.URL)
			want := plain + "\n"
			if tc.token != "" {
				want = plain + " " + tc.token + "\n"
			}
			if got != want {
				t.Fatalf("line = %q, want %q", got, want)
			}
			if strings.Contains(got, "\x1b[") || strings.Contains(got, "judged") {
				t.Errorf("line %q carries a colour escape or a judged token", got)
			}
			if tok := strings.TrimPrefix(strings.TrimSuffix(got, "\n"), plain); strings.ContainsAny(tok, "●◐○·?") {
				t.Errorf("the token %q borrows a state glyph", tok)
			}
			var st statuslineState
			if err := json.Unmarshal([]byte(runStatusline(t, stdin, "--url", f.srv.URL, "--json")), &st); err != nil {
				t.Fatal(err)
			}
			if st.State != statuslineStIndexed || st.Line != plain || st.DecisionPort != tc.token || st.KillSource != tc.source {
				t.Errorf("--json = %+v, want state indexed, line %q, decision_port %q, kill_source %q", st, plain, tc.token, tc.source)
			}
		})
	}
}
