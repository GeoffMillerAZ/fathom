package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
)

// scannerPolicy is deliberately a single immutable command recipe, not a subprocess API.
type scannerPolicy struct {
	Kind             string `json:"kind"`
	Profile          string `json:"profile"`
	Executable       string `json:"executable"`
	SHA256           string `json:"sha256"`
	TimeoutMillis    int    `json:"timeoutMillis"`
	MaxReportBytes   int    `json:"maxReportBytes"`
	ReuseEqualInputs bool   `json:"reuseEqualInputs,omitempty"`
}
type scannerReuseReceipt struct {
	Profile string `json:"profile"`
	Files   int    `json:"files"`
	Bytes   int    `json:"bytes"`
}
type scannerReceipt struct {
	ID             string               `json:"id"`
	Profile        string               `json:"profile"`
	Status         string               `json:"status"`
	Code           string               `json:"code"`
	ExpectedFiles  int                  `json:"expectedFiles"`
	AttemptedFiles int                  `json:"attemptedFiles"`
	InvokedFiles   int                  `json:"invokedFiles"`
	CompletedFiles int                  `json:"completedFiles"`
	BytesSubmitted int                  `json:"bytesSubmitted"`
	BytesObserved  int                  `json:"bytesObserved"`
	Findings       int                  `json:"findings"`
	Reuse          *scannerReuseReceipt `json:"reuse,omitempty"`
}
type scannerRunResult struct {
	out, errout []byte
	exit        int
	started     bool
	code        string
}
type scannerRunner func(context.Context, string, string, []byte, int, ...string) scannerRunResult

func validScannerControls(s scope, p policy) bool {
	if len(s.RequiredScannerIDs) == 0 {
		return p.Scanner == nil
	}
	if len(s.RequiredScannerIDs) != 1 || s.RequiredScannerIDs[0] != "gitleaks" || p.Scanner == nil {
		return false
	}
	c := p.Scanner
	if c.Kind != "gitleaks" || c.Profile != "gitleaks-stdin-8.30.1" || !filepath.IsAbs(c.Executable) || len(c.Executable) > 4096 || c.TimeoutMillis < 1 || c.TimeoutMillis > 30000 || c.MaxReportBytes < 1 || c.MaxReportBytes > 1<<20 || len(c.SHA256) != 64 {
		return false
	}
	for _, x := range c.SHA256 {
		if x < '0' || x > '9' && x < 'a' || x > 'f' {
			return false
		}
	}
	return true
}

type cappedWriter struct {
	b        bytes.Buffer
	cap      int
	exceeded bool
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if len(p) > w.cap-w.b.Len() {
		w.exceeded = true
		return 0, errors.New("bounded output")
	}
	return w.b.Write(p)
}
func scannerEnv(dir string) []string {
	env := []string{"HOME=" + dir, "TMPDIR=" + dir, "TMP=" + dir, "TEMP=" + dir, "XDG_CONFIG_HOME=" + dir, "XDG_DATA_HOME=" + dir, "XDG_CACHE_HOME=" + dir, "NO_COLOR=1", "TERM=dumb"}
	if runtime.GOOS == "windows" {
		if s := os.Getenv("SystemRoot"); s != "" {
			env = append(env, "SystemRoot="+s)
		}
	}
	return env
}
func scannerCommand(ctx context.Context, exe, dir string, input []byte, outCap int, args ...string) scannerRunResult {
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = dir
	cmd.Env = scannerEnv(dir)
	cmd.Stdin = bytes.NewReader(input)
	cmd.WaitDelay = 100 * time.Millisecond
	out := &cappedWriter{cap: outCap}
	errout := &cappedWriter{cap: 64 << 10}
	cmd.Stdout = out
	cmd.Stderr = errout
	if e := cmd.Start(); e != nil {
		return scannerRunResult{code: "execution_incomplete"}
	}
	e := cmd.Wait()
	if out.exceeded || errout.exceeded {
		return scannerRunResult{started: true, code: "output_limit"}
	}
	if ctx.Err() != nil {
		return scannerRunResult{started: true, code: "deadline"}
	}
	if e == nil {
		return scannerRunResult{out: out.b.Bytes(), errout: errout.b.Bytes(), started: true}
	}
	if x, ok := e.(*exec.ExitError); ok {
		return scannerRunResult{out: out.b.Bytes(), errout: errout.b.Bytes(), exit: x.ExitCode(), started: true}
	}
	return scannerRunResult{started: true, code: "execution_incomplete"}
}
func scannerBinary(ctx context.Context, c scannerPolicy) (string, string, func() error, string) {
	return executableSnapshot(ctx, c.Executable, c.SHA256)
}

// executableSnapshot pins an independently registered executable into private space.
func executableSnapshot(ctx context.Context, executable, digest string) (string, string, func() error, string) {
	if ctx.Err() != nil {
		return "", "", nil, "deadline"
	}
	st, e := os.Lstat(executable)
	if e != nil || !st.Mode().IsRegular() || st.Size() < 1 || st.Size() > 128<<20 {
		return "", "", nil, "binary_unavailable"
	}
	f, e := os.OpenFile(executable, os.O_RDONLY|nonblock, 0)
	if e != nil {
		return "", "", nil, "binary_unavailable"
	}
	defer f.Close()
	actual, e := f.Stat()
	if e != nil || !actual.Mode().IsRegular() || !os.SameFile(st, actual) {
		return "", "", nil, "binary_unavailable"
	}
	b, e := io.ReadAll(io.LimitReader(f, (128<<20)+1))
	if e != nil || len(b) < 1 || len(b) > 128<<20 || ctx.Err() != nil {
		if ctx.Err() != nil {
			return "", "", nil, "deadline"
		}
		return "", "", nil, "binary_unavailable"
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != digest {
		return "", "", nil, "hash_mismatch"
	}
	dir, e := os.MkdirTemp("", "command-guard-executable-")
	if e != nil {
		return "", "", nil, "binary_unavailable"
	}
	cleanup := func() error { return os.RemoveAll(dir) }
	if e = os.Chmod(dir, 0700); e != nil {
		cleanup()
		return "", "", nil, "binary_unavailable"
	}
	home := filepath.Join(dir, "home")
	if e = os.Mkdir(home, 0700); e != nil {
		cleanup()
		return "", "", nil, "binary_unavailable"
	}
	name := "tool"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	exe := filepath.Join(dir, name)
	if e = os.WriteFile(exe, b, 0500); e != nil {
		cleanup()
		return "", "", nil, "binary_unavailable"
	}
	if ctx.Err() != nil {
		cleanup()
		return "", "", nil, "deadline"
	}
	return exe, home, cleanup, ""
}

var scanSummary = regexp.MustCompile(`^[0-9]{1,2}:[0-9]{2}(?:AM|PM) INF scanned ~([0-9]+) bytes \([0-9]+(?:\.[0-9]+)?(?: [A-Za-z]+)?\) in [0-9]+(?:\.[0-9]+)?(?:ms|s)$`)
var cleanLog = regexp.MustCompile(`^[0-9]{1,2}:[0-9]{2}(?:AM|PM) INF no leaks found$`)
var findLog = regexp.MustCompile(`^[0-9]{1,2}:[0-9]{2}(?:AM|PM) WRN leaks found: ([0-9]+)$`)

func scannerLog(b []byte, supplied, count, exit int) (int, bool) {
	if !jsonwire.ValidUnicode(b) || len(b) == 0 || b[len(b)-1] != '\n' {
		return 0, false
	}
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	if len(lines) != 2 {
		return 0, false
	}
	m := scanSummary.FindStringSubmatch(lines[0])
	if m == nil {
		return 0, false
	}
	n, e := strconv.Atoi(m[1])
	if e != nil || n != supplied {
		return 0, false
	}
	if exit == 0 && count == 0 && cleanLog.MatchString(lines[1]) {
		return n, true
	}
	if exit == 3 && count > 0 {
		x := findLog.FindStringSubmatch(lines[1])
		if x != nil {
			v, e := strconv.Atoi(x[1])
			if e == nil && v == count {
				return n, true
			}
		}
	}
	return 0, false
}

var findingFields = map[string]string{"RuleID": "s", "Description": "s", "StartLine": "i", "EndLine": "i", "StartColumn": "i", "EndColumn": "i", "Match": "s", "Secret": "s", "File": "s", "SymlinkFile": "s", "Commit": "s", "Entropy": "n", "Author": "s", "Email": "s", "Date": "s", "Message": "s", "Tags": "a", "Fingerprint": "s"}

func scannerFindings(b []byte) (int, bool) {
	if !jsonwire.ValidUnicode(b) {
		return 0, false
	}
	d := json.NewDecoder(bytes.NewReader(b))
	if validateValue(d, 0) != nil {
		return 0, false
	}
	if _, e := d.Token(); e != io.EOF {
		return 0, false
	}
	var rows []map[string]json.RawMessage
	if json.Unmarshal(b, &rows) != nil || rows == nil || len(rows) > 1024 {
		return 0, false
	}
	for _, row := range rows {
		if row == nil {
			return 0, false
		}
		for k, v := range row {
			kind, ok := findingFields[k]
			if !ok {
				return 0, false
			}
			switch kind {
			case "s":
				var x string
				if json.Unmarshal(v, &x) != nil || bytes.Equal(v, []byte("null")) {
					return 0, false
				}
			case "n":
				var x float64
				if json.Unmarshal(v, &x) != nil || bytes.Equal(v, []byte("null")) {
					return 0, false
				}
			case "i":
				var x int
				if json.Unmarshal(v, &x) != nil || bytes.Equal(v, []byte("null")) || x < 0 {
					return 0, false
				}
			case "a":
				var x []string
				if json.Unmarshal(v, &x) != nil || bytes.Equal(v, []byte("null")) {
					return 0, false
				}
			}
		}
	}
	return len(rows), true
}

func scanSelectedWithRunner(c scannerPolicy, data [][]byte, r *scannerReceipt, invoke scannerRunner) (success bool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.TimeoutMillis)*time.Millisecond)
	defer cancel()
	return scanSelectedWithContext(ctx, c, data, r, invoke)
}
func scanSelectedWithContext(parent context.Context, c scannerPolicy, data [][]byte, r *scannerReceipt, invoke scannerRunner) (success bool) {
	return scanSelectedWithContextHash(parent, c, data, r, invoke, sha256.Sum256)
}

// scanSelectedWithContextHash is invocation-local. The hash only indexes candidates;
// byte equality decides reuse. The supplied runner is trusted production transport.
func scanSelectedWithContextHash(parent context.Context, c scannerPolicy, data [][]byte, r *scannerReceipt, invoke scannerRunner, indexHash func([]byte) [32]byte) (success bool) {
	return scanSelectedWithContextDeps(parent, c, data, r, invoke, indexHash, sha256.Sum256, scannerBinary)
}

func scanSelectedWithContextDeps(parent context.Context, c scannerPolicy, data [][]byte, r *scannerReceipt, invoke scannerRunner, indexHash, witnessHash func([]byte) [32]byte, snapshot func(context.Context, scannerPolicy) (string, string, func() error, string)) (success bool) {
	ctx, cancel := context.WithTimeout(parent, time.Duration(c.TimeoutMillis)*time.Millisecond)
	defer cancel()
	if c.ReuseEqualInputs {
		r.Reuse = &scannerReuseReceipt{Profile: "invocation-exact/1"}
	}
	exe, dir, cleanup, code := snapshot(ctx, c)
	if code != "" {
		r.Status = "incomplete"
		r.Code = code
		return false
	}
	defer func() {
		if cleanup() != nil {
			r.Status = "incomplete"
			r.Code = "cleanup_failed"
			success = false
		}
	}()
	initial := make([][32]byte, len(data))
	for i, b := range data {
		initial[i] = witnessHash(b)
	}
	v := invoke(ctx, exe, dir, nil, 64, "version")
	if !v.started || v.code != "" || v.exit != 0 || string(v.out) != "8.30.1\n" || len(v.errout) != 0 {
		r.Status = "incomplete"
		r.Code = "version_mismatch"
		if v.code != "" {
			r.Code = v.code
		}
		if !v.started && v.code == "" {
			r.Code = "execution_incomplete"
		}
		if ctx.Err() != nil {
			r.Code = "deadline"
		}
		return false
	}
	args := []string{"stdin", "--report-path", "-", "--report-format", "json", "--redact=100", "--no-banner", "--no-color", "--exit-code=3", "--max-decode-depth=0", "--max-archive-depth=0", "--timeout=0", "--log-level", "info", "--ignore-gitleaks-allow"}
	allUnchanged := func() bool {
		for i, b := range data {
			if witnessHash(b) != initial[i] {
				return false
			}
		}
		return true
	}
	index := make(map[[32]byte][][]byte)
	retainedEntries := 0
	retainedBytes := 0
inputLoop:
	for i, b := range data {
		if ctx.Err() != nil {
			r.Status = "incomplete"
			r.Code = "deadline"
			return false
		}
		if witnessHash(b) != initial[i] {
			r.Status = "incomplete"
			r.Code = "coverage_mismatch"
			if ctx.Err() != nil {
				r.Code = "deadline"
			}
			return false
		}
		if c.ReuseEqualInputs {
			h := indexHash(b)
			for _, original := range index[h] {
				if bytes.Equal(original, b) {
					if ctx.Err() != nil {
						r.Status = "incomplete"
						r.Code = "deadline"
						return false
					}
					r.Reuse.Files++
					r.Reuse.Bytes += len(b)
					continue inputLoop
				}
			}
			if ctx.Err() != nil {
				r.Status = "incomplete"
				r.Code = "deadline"
				return false
			}
		}
		r.AttemptedFiles++
		r.BytesSubmitted += len(b)
		res := invoke(ctx, exe, dir, b, c.MaxReportBytes, args...)
		if res.started {
			r.InvokedFiles++
		}
		if witnessHash(b) != initial[i] {
			r.Status = "incomplete"
			r.Code = "coverage_mismatch"
			if ctx.Err() != nil {
				r.Code = "deadline"
			}
			return false
		}
		if !res.started {
			r.Status = "incomplete"
			r.Code = res.code
			if r.Code == "" {
				r.Code = "execution_incomplete"
			}
			if ctx.Err() != nil {
				r.Code = "deadline"
			}
			return false
		}
		if res.code != "" {
			r.Status = "incomplete"
			r.Code = res.code
			if ctx.Err() != nil {
				r.Code = "deadline"
			}
			return false
		}
		count, valid := scannerFindings(res.out)
		if !valid {
			r.Status = "incomplete"
			r.Code = "report_invalid"
			return false
		}
		observed, valid := scannerLog(res.errout, len(b), count, res.exit)
		if !valid || ctx.Err() != nil {
			r.Status = "incomplete"
			r.Code = "coverage_mismatch"
			if ctx.Err() != nil {
				r.Code = "deadline"
			}
			return false
		}
		r.CompletedFiles++
		r.BytesObserved += observed
		r.Findings += count
		if c.ReuseEqualInputs && count == 0 && retainedEntries < 256 && len(b) <= (8<<20)-retainedBytes {
			h := indexHash(b)
			index[h] = append(index[h], bytes.Clone(b))
			retainedEntries++
			retainedBytes += len(b)
		}
		if ctx.Err() != nil {
			r.Status = "incomplete"
			r.Code = "deadline"
			return false
		}
	}
	if ctx.Err() != nil {
		r.Status = "incomplete"
		r.Code = "deadline"
		return false
	}
	if !allUnchanged() {
		r.Status = "incomplete"
		r.Code = "coverage_mismatch"
		if ctx.Err() != nil {
			r.Code = "deadline"
		}
		return false
	}
	if ctx.Err() != nil {
		r.Status = "incomplete"
		r.Code = "deadline"
		return false
	}
	covered := r.CompletedFiles
	if r.Reuse != nil {
		covered += r.Reuse.Files
	}
	if covered != r.ExpectedFiles {
		r.Status = "incomplete"
		r.Code = "coverage_mismatch"
		return false
	}
	if r.Findings > 0 {
		r.Status = "finding"
		r.Code = "finding"
	} else {
		r.Status = "complete"
		r.Code = "verified"
	}
	return true
}
