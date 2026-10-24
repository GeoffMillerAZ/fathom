package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// gitInitTempRepo makes dir a real (if empty) git repo, so gitToplevel
// (serviceident.go) can find it as the enclosing work tree for a
// subdirectory cwd — exactly what a Bash command's "cwd" resolves against
// in a real checkout, where the project directory and the git root are
// usually the same thing but a command can still run from a subdirectory.
func gitInitTempRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", "-q", dir)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v\n%s", dir, err, out)
	}
}

func bashPostToolUseStdin(t *testing.T, cwd, command string) {
	t.Helper()
	body := fmt.Sprintf(`{"session_id":"s1","cwd":%q,"tool_name":"Bash","tool_input":{"command":%q}}`, cwd, command)
	hookStdin(t, body)
}

func TestEditHookBashWriteSedOnlyEditedFile(t *testing.T) {
	for _, suffix := range []string{"''", ".bak"} {
		t.Run(suffix, func(t *testing.T) {
			root, transport := ruleDeliveryEnv(t)
			transport.rules = []any{cbRule("Keep HTML accessible.")}
			gitInitTempRepo(t, root)
			if err := os.Mkdir(filepath.Join(root, "dir"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "dir", "f.html"), []byte("<p>c/d</p>\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			var files []string
			http.DefaultTransport = ad5RoundTripper(func(r *http.Request) (*http.Response, error) {
				if r.Body != nil {
					raw, err := io.ReadAll(r.Body)
					if err != nil {
						return nil, err
					}
					r.Body.Close()
					r.Body = io.NopCloser(bytes.NewReader(raw))
					var req struct {
						Params struct {
							Name      string `json:"name"`
							Arguments struct {
								File string `json:"file"`
							} `json:"arguments"`
						} `json:"params"`
					}
					if err := json.Unmarshal(raw, &req); err != nil {
						return nil, err
					}
					if req.Params.Name == "obligations" {
						files = append(files, req.Params.Arguments.File)
					}
				}
				return transport.RoundTrip(r)
			})
			got := runBashWriteHook(t, "http://sed-test.invalid", "session-a", root, "sed -i "+suffix+" 's|a/b|c/d|g' dir/f.html")
			if !strings.Contains(got, "dir/f.html") || !strings.Contains(got, "Keep HTML accessible.") {
				t.Fatalf("edited file's governing rule missing: %q", got)
			}
			if strings.Contains(got, "s|a/b|c/d|g") || len(files) != 1 || files[0] != "dir/f.html" {
				t.Fatalf("sed script must never get rules: context=%q, obligations files=%q", got, files)
			}
		})
	}
}

// TestEditHookBashWrite_ConfidentTargetServesGoverningRule pins O57 item
// 4/5(b)'s main case: a Bash command that writes a file via cp — never
// matching Edit|Write|MultiEdit — still serves the rule that governs the
// written path.
func TestEditHookBashWrite_ConfidentTargetServesGoverningRule(t *testing.T) {
	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	srv := fakePromptDaemon(t, map[string]any{
		"internal/store/backend.go": map[string]any{
			"service": svc,
			"file":    "internal/store/backend.go",
			"rules": []any{
				map[string]any{
					"rule": "internal/store stays pure Go — the DuckDB backend was deleted outright (ADR-025); never reintroduce CGO or DuckDB there.",
					"src":  "ADR.md#L814-L814", "tier": "doc-claimed", "layer": "adr-rules",
				},
			},
		},
	}, nil)
	defer srv.Close()

	bashPostToolUseStdin(t, dir, "cp /tmp/draft.go internal/store/backend.go")

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("edit-hook exit = %d, want 0; stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "ADR-025") {
		t.Fatalf("output missing ADR-025's rule: %s", out.String())
	}
	if !strings.Contains(out.String(), "internal/store/backend.go") {
		t.Fatalf("output does not name the written path: %s", out.String())
	}
}

// TestEditHookBashWrite_DoubtNamesCwdNearestGuide pins O57 item 4/5(b)'s
// doubt path: `git apply` writes something, but the parser cannot name
// what — edit-hook falls back to the cwd's nearest guide rather than
// staying silent. Since EW-1 it says so in one line naming the directory
// and how many rules govern it, never the rule bodies
// (TestEditHookBashWriteDoubtServesAtMostOneLine pins the line's shape).
func TestEditHookBashWrite_DoubtNamesCwdNearestGuide(t *testing.T) {
	isolateBashDoubt(t)
	dir := t.TempDir()
	gitInitTempRepo(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, "internal", "mcp"), 0o755); err != nil {
		t.Fatal(err)
	}
	svc := strings.ToLower(filepath.Base(dir))
	srv := fakePromptDaemon(t, map[string]any{
		"internal/mcp/__bash_write__.go": map[string]any{
			"service": svc,
			"file":    "internal/mcp/__bash_write__.go",
			"rules": []any{
				map[string]any{"rule": "Register every tool in the mcp registry.", "src": "internal/CLAUDE.md#L4-L4", "tier": "doc-claimed", "layer": "obligations"},
			},
		},
	}, nil)
	defer srv.Close()

	got := runBashWriteHook(t, srv.URL, "s1", filepath.Join(dir, "internal", "mcp"), "git apply /tmp/fix.patch")
	if !strings.Contains(got, "1 rule(s) govern internal/mcp") {
		t.Fatalf("output does not name the cwd's directory and its rule count: %q", got)
	}
	if strings.Contains(got, "Register every tool in the mcp registry.") {
		t.Fatalf("the doubt line must not serve the rule body: %q", got)
	}
}

// TestEditHookBashWrite_SilentOnReadOnlyCommand: a Bash command that
// merely reads must stay silent — no beat-worthy output for `go test`.
func TestEditHookBashWrite_SilentOnReadOnlyCommand(t *testing.T) {
	dir := t.TempDir()
	srv := fakePromptDaemon(t, nil, nil)
	defer srv.Close()

	bashPostToolUseStdin(t, dir, "go test ./...")

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("edit-hook exit = %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for a read-only Bash command, got %q", out.String())
	}
}

// bashPostToolUseStdinSession is bashPostToolUseStdin with the session id a
// test names, for the once-per-session cases.
func bashPostToolUseStdinSession(t *testing.T, session, cwd, command string) {
	t.Helper()
	body := fmt.Sprintf(`{"session_id":%q,"cwd":%q,"tool_name":"Bash","tool_input":{"command":%q}}`, session, cwd, command)
	hookStdin(t, body)
}

// isolateBashDoubt points the once-per-session doubt markers and the beat
// log at fresh temp locations and returns the beat log's path.
func isolateBashDoubt(t *testing.T) string {
	t.Helper()
	orig := bashDoubtDir
	bashDoubtDir = filepath.Join(t.TempDir(), "fathom-bash-doubt")
	t.Cleanup(func() { bashDoubtDir = orig })
	beatLog := filepath.Join(t.TempDir(), "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", beatLog)
	return beatLog
}

// runBashWriteHook runs one PostToolUse Bash call through edit-hook and
// returns the additionalContext it served ("" for silence).
func runBashWriteHook(t *testing.T, srvURL, session, cwd, command string) string {
	t.Helper()
	bashPostToolUseStdinSession(t, session, cwd, command)
	var out, errBuf bytes.Buffer
	if code := runWithStdin([]string{"edit-hook", "--url=" + srvURL}, nil, &out, &errBuf); code != 0 {
		t.Fatalf("edit-hook exit = %d, want 0; stderr=%s", code, errBuf.String())
	}
	if strings.TrimSpace(out.String()) == "" {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &payload); err != nil {
		t.Fatalf("edit-hook output not valid JSON: %v (%q)", err, out.String())
	}
	hook, _ := payload["hookSpecificOutput"].(map[string]any)
	ctx, _ := hook["additionalContext"].(string)
	return ctx
}

// obligationsCountingDaemon answers every obligations call with the same
// rules (so a wrongly served foreign write is visible in the output too)
// and counts those calls; list_services and everything else is empty.
func obligationsCountingDaemon(t *testing.T, rules []any) (*httptest.Server, *int32) {
	t.Helper()
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string `json:"name"`
				Arguments struct {
					File string `json:"file"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		inner := "{}"
		switch req.Params.Name {
		case "obligations":
			atomic.AddInt32(&n, 1)
			inner = mustJSON(map[string]any{"service": "svc", "file": req.Params.Arguments.File, "rules": rules})
		case "list_services":
			inner = mustJSON(map[string]any{"services": []any{}})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []any{map[string]any{"type": "text", "text": inner}}},
		})
	}))
	return srv, &n
}

func sixRootRules() []any {
	rules := make([]any, 0, 6)
	for i := 1; i <= 6; i++ {
		rules = append(rules, map[string]any{
			"rule": fmt.Sprintf("Root rule number %d that says something long enough to cost real tokens when it is served for no reason.", i),
			"src":  fmt.Sprintf("ADR.md#L%d-L%d", i*10, i*10+3), "tier": "doc-claimed", "layer": "adr-rules",
		})
	}
	return rules
}

// readBeats returns the beat log's rows for one channel.
func readBeats(t *testing.T, beatLog, channel string) []struct {
	Kind string            `json:"kind"`
	Meta map[string]string `json:"meta"`
} {
	t.Helper()
	type beat struct {
		Channel string            `json:"channel"`
		Kind    string            `json:"kind"`
		Meta    map[string]string `json:"meta"`
	}
	raw, err := os.ReadFile(beatLog)
	if err != nil {
		return nil
	}
	var out []struct {
		Kind string            `json:"kind"`
		Meta map[string]string `json:"meta"`
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var b beat
		if err := json.Unmarshal([]byte(line), &b); err != nil {
			t.Fatalf("beat row %q: %v", line, err)
		}
		if b.Channel == channel {
			out = append(out, struct {
				Kind string            `json:"kind"`
				Meta map[string]string `json:"meta"`
			}{b.Kind, b.Meta})
		}
	}
	return out
}

// EW-1 / O122: when the hook cannot name what a Bash command wrote it serves
// ONE line, not the nearest guide's rules, once per session per directory,
// and the emission is a payload-cost beat. Before this row the same call
// served six unrelated root rules on every write-shaped Bash command (4,835
// of 7,953 in 13 days).
func TestEditHookBashWriteDoubtServesAtMostOneLine(t *testing.T) {
	beatLog := isolateBashDoubt(t)
	dir := t.TempDir()
	gitInitTempRepo(t, dir)
	for _, sub := range []string{"internal/mcp", "internal/store"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	srv, _ := obligationsCountingDaemon(t, sixRootRules())
	defer srv.Close()

	mcpDir := filepath.Join(dir, "internal", "mcp")
	storeDir := filepath.Join(dir, "internal", "store")

	first := runBashWriteHook(t, srv.URL, "s1", mcpDir, "git apply /tmp/fix.patch")
	if first == "" {
		t.Fatal("the first doubt in a session and directory served nothing; want one line")
	}
	if strings.Contains(first, "\n") {
		t.Errorf("the doubt payload is more than one line: %q", first)
	}
	if len(first) > 320 {
		t.Errorf("the doubt line is %d bytes; want a line, not a paragraph: %q", len(first), first)
	}
	if strings.Contains(first, "Root rule number") {
		t.Errorf("the doubt line must not carry rule bodies: %q", first)
	}
	if !strings.Contains(first, "6 rule") {
		t.Errorf("the doubt line should say how many rules govern the directory: %q", first)
	}

	if again := runBashWriteHook(t, srv.URL, "s1", mcpDir, "git apply /tmp/other.patch"); again != "" {
		t.Errorf("a second doubt in the same session and directory served %q; want silence", again)
	}
	if other := runBashWriteHook(t, srv.URL, "s1", storeDir, "git apply /tmp/fix.patch"); other == "" || strings.Contains(other, "\n") {
		t.Errorf("a first doubt in another directory should serve one line, got %q", other)
	}
	if next := runBashWriteHook(t, srv.URL, "s2", mcpDir, "git apply /tmp/fix.patch"); next == "" || strings.Contains(next, "\n") {
		t.Errorf("a first doubt in a new session should serve one line, got %q", next)
	}

	beats := readBeats(t, beatLog, "payload-cost")
	var postBash []map[string]string
	for _, b := range beats {
		if b.Kind == "post-bash" {
			postBash = append(postBash, b.Meta)
		}
	}
	if len(postBash) != 3 {
		t.Fatalf("payload-cost post-bash beats = %d, want 3 (one per emission, none for the silent repeat): %v", len(postBash), beats)
	}
	if postBash[0]["bytes"] != fmt.Sprint(len(first)) {
		t.Errorf("the beat's bytes = %q, want the emitted %d", postBash[0]["bytes"], len(first))
	}
	if postBash[0]["doubt"] != "true" {
		t.Errorf("a doubt emission's beat should say doubt: %v", postBash[0])
	}
}

// EW-1 / O122: a quoted `>` is text. The O122 repro command writes nothing,
// so the hook is silent and never reaches the daemon.
func TestEditHookBashWriteQuotedArrowIsSilent(t *testing.T) {
	isolateBashDoubt(t)
	dir := t.TempDir()
	srv, calls := obligationsCountingDaemon(t, sixRootRules())
	defer srv.Close()
	for _, cmd := range []string{
		`for id in a b; do echo "$id -> x"; done`,
		`git commit -m 'cp the draft > over there, then sed -i it'`,
		"cat <<'EOF'\necho hi > out.txt\ncp a b\nEOF",
	} {
		if got := runBashWriteHook(t, srv.URL, "s1", dir, cmd); got != "" {
			t.Errorf("%q served %q; want silence", cmd, got)
		}
	}
	if n := atomic.LoadInt32(calls); n != 0 {
		t.Errorf("obligations calls = %d, want 0 for commands that write nothing", n)
	}
}

// EW-1 / O122 addendum: a write whose every target resolves outside all
// candidate roots is silent, not doubt: it is resolved and foreign, not
// unresolvable.
func TestEditHookBashWriteForeignTargetIsSilent(t *testing.T) {
	isolateBashDoubt(t)
	dir := t.TempDir()
	gitInitTempRepo(t, dir)
	t.Setenv("HOME", filepath.Join(t.TempDir(), "home"))
	srv, calls := obligationsCountingDaemon(t, sixRootRules())
	defer srv.Close()
	for _, cmd := range []string{
		`printf '%s\n' x >> /Users/someone/.claude/projects/p/memory/MEMORY.md`,
		"cat > /tmp/scratch.sh <<'EOF'\ncp bin/fathom /usr/local/bin/fathom\necho ${1:?usage} > out.txt\nEOF",
		`printf x >> ~/.claude/projects/p/memory/MEMORY.md`,
		`echo x > ../outside.go`,
		`cd /tmp/elsewhere && echo x > y.go`,
	} {
		if got := runBashWriteHook(t, srv.URL, "s1", dir, cmd); got != "" {
			t.Errorf("%q served %q; want silence for a resolved foreign target", cmd, got)
		}
	}
	if n := atomic.LoadInt32(calls); n != 0 {
		t.Errorf("obligations calls = %d, want 0 when every target is foreign", n)
	}
}

// A target that resolves INSIDE a candidate root still serves its governing
// rules, whether spelled absolute, relative or through a tilde, and the
// emission is a payload-cost beat carrying the payload's bytes.
func TestEditHookBashWriteResolvedTargetServesAndCountsAsPayloadCost(t *testing.T) {
	beatLog := isolateBashDoubt(t)
	dir := t.TempDir()
	gitInitTempRepo(t, dir)
	t.Setenv("HOME", filepath.Dir(dir))
	srv, calls := obligationsCountingDaemon(t, []any{
		map[string]any{"rule": "internal/store stays pure Go.", "src": "ADR.md#L814-L814", "tier": "doc-claimed", "layer": "adr-rules"},
	})
	defer srv.Close()

	var served []string
	for _, cmd := range []string{
		"cp /tmp/draft.go internal/store/backend.go",
		fmt.Sprintf("cat > %s <<'EOF'\npackage store\nEOF", filepath.Join(dir, "internal", "store", "abs.go")),
		fmt.Sprintf("printf x >> ~/%s/internal/store/tilde.go", filepath.Base(dir)),
	} {
		got := runBashWriteHook(t, srv.URL, "s1", dir, cmd)
		if !strings.Contains(got, "internal/store stays pure Go.") {
			t.Errorf("%q did not serve the governing rule: %q", cmd, got)
		}
		served = append(served, got)
	}
	if n := atomic.LoadInt32(calls); n != 3 {
		t.Errorf("obligations calls = %d, want 3 (one per resolved write)", n)
	}
	var beats []map[string]string
	for _, b := range readBeats(t, beatLog, "payload-cost") {
		if b.Kind == "post-bash" {
			beats = append(beats, b.Meta)
		}
	}
	if len(beats) != len(served) {
		t.Fatalf("payload-cost post-bash beats = %d, want %d", len(beats), len(served))
	}
	for i, b := range beats {
		if b["bytes"] != fmt.Sprint(len(served[i])) || b["rules"] != "1" || b["phase"] != "post-bash" {
			t.Errorf("beat %d = %v; want bytes=%d rules=1 phase=post-bash", i, b, len(served[i]))
		}
	}
}

func TestEditHookBashWriteSedSafety(t *testing.T) {
	// Asserts which files the hook names, not how fast: on a loaded gate the
	// in-process hook spent its deadline and stayed silent, so its budget is
	// widened as pinRepo widens it.
	prevScale := hookBudgetScale
	hookBudgetScale = 30
	t.Cleanup(func() { hookBudgetScale = prevScale })
	for _, tt := range []struct {
		name, command string
		wantFiles     string
		wantNote      bool
	}{
		{"bare backup", `sed -i bak 's/a/b/' f`, "f", true},
		{"tilde backup", `sed -i '~' 's/a/b/' f`, "f", true},
		{"failed sed", `sed -i 's/a/b/' missing`, "", true},
		{"quiet", `sed --quiet -i 's/a/b/' f`, "f", false},
		{"unknown before in-place", `sed --unknown value -i 's/a/b/' f`, "", true},
		{"BSD capital in-place", `sed -I '' 's/a/b/' f`, "f", false},
		{"find plus", `find . -exec sed -i '' 's/a/b/' {} +`, "", true},
		{"find semicolon", `find . -exec sed -i '' 's/a/b/' {} \;`, "", true},
		{"xargs", `xargs -I{} sed -i 's/a/b/' {}`, "", true},
		{"only dotfile", `sed -e 's/a/b/' -i .env`, ".env", false},
		{"only dotfile before flag", `sed -e 's/a/b/' -i .env -n`, ".env", false},
		{"only dotfile before expression", `sed -i .env -e 's/a/b/'`, ".env", false},
		{"ambiguous dotfile", `sed -e 's/a/b/' -i .env .bashrc`, ".bashrc", true},
		{"dotted backup", `sed -i .bak 's/a/b/' f`, "f", true},
		{"redirect absent", `echo x > missing`, "missing", false},
		{"copy absent", `cp source missing`, "missing", false},
		{"move absent", `mv source missing`, "missing", false},
		{"tee absent", `echo x | tee missing`, "missing", false},
		{"perl absent", `perl -pi -e 's/a/b/' missing`, "missing", false},
		{"ruby absent", `ruby -pi -e 'gsub(/a/, "b")' missing`, "missing", false},
		{"mixed writers", `sed -i 's/a/b/' missing; echo x > missing`, "missing", false},
		{"mixed writers reversed", `echo x > missing; sed -i 's/a/b/' missing`, "missing", false},
		{"mixed writer aliases", `sed -i 's/a/b/' missing; echo x > ./missing`, "missing", false},
		{"different writers", `sed -i 's/a/b/' missing; echo x > out`, "out", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			isolateBashDoubt(t)
			root, transport := ruleDeliveryEnv(t)
			transport.rules = []any{cbRule("Keep files readable.")}
			gitInitTempRepo(t, root)
			for _, file := range []string{"f", ".env", ".bashrc"} {
				if err := os.WriteFile(filepath.Join(root, file), []byte("b"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var files []string
			http.DefaultTransport = ad5RoundTripper(func(r *http.Request) (*http.Response, error) {
				if r.Body != nil {
					raw, err := io.ReadAll(r.Body)
					if err != nil {
						return nil, err
					}
					r.Body.Close()
					r.Body = io.NopCloser(bytes.NewReader(raw))
					var req struct {
						Params struct {
							Name      string `json:"name"`
							Arguments struct {
								File string `json:"file"`
							} `json:"arguments"`
						} `json:"params"`
					}
					if err := json.Unmarshal(raw, &req); err != nil {
						return nil, err
					}
					if req.Params.Name == "obligations" && req.Params.Arguments.File != "__bash_write__.go" {
						files = append(files, req.Params.Arguments.File)
					}
				}
				return transport.RoundTrip(r)
			})
			got := runBashWriteHook(t, "http://sed-test.invalid", "session-a", root, tt.command)
			if strings.Join(files, ",") != tt.wantFiles {
				t.Fatalf("obligations files=%q, want %q; context=%q", files, tt.wantFiles, got)
			}
			if note := strings.Contains(got, "hook could not name"); note != tt.wantNote {
				t.Fatalf("cannot-tell note=%v, want %v; context=%q", note, tt.wantNote, got)
			}
			if tt.wantFiles != "" && !strings.Contains(got, tt.wantFiles) {
				t.Fatalf("written file missing from context: %q", got)
			}
			if tt.wantNote && tt.wantFiles == "" && strings.Contains(got, "\n") {
				t.Fatalf("fallback must be one line: %q", got)
			}
			if tt.wantNote {
				again := runBashWriteHook(t, "http://sed-test.invalid", "session-a", root, tt.command)
				if strings.Contains(again, "hook could not name") {
					t.Fatalf("repeated doubt note: %q", again)
				}
			}
		})
	}
}

func TestEditHookBashWriteSedDoubtAfterRulesServed(t *testing.T) {
	isolateBashDoubt(t)
	root, _ := ruleDeliveryEnv(t)
	gitInitTempRepo(t, root)
	if err := os.WriteFile(filepath.Join(root, ".bashrc"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := runBashWriteHook(t, "http://sed-test.invalid", "session-a", root, `sed -e 's/a/b/' -i .bashrc`)
	if !strings.Contains(got, "Sort outputs.") {
		t.Fatalf("initial rules missing: %q", got)
	}
	got = runBashWriteHook(t, "http://sed-test.invalid", "session-a", root, `sed -e 's/a/b/' -i .env .bashrc`)
	if !strings.Contains(got, "hook could not name") || strings.Contains(got, "\n") {
		t.Fatalf("new uncertainty must survive rule dedupe as one line: %q", got)
	}
	raw, err := os.ReadFile(os.Getenv("FATHOM_HOOKS_LOG"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"doubt":"true"`)) {
		t.Fatalf("note-only delivery must be accounted as doubt: %s", raw)
	}
}
