// Command command-qualify checks a fixed synthetic file-inspection roster against
// an operator-supplied executable. It has ordinary host authority, not a sandbox.
// Concurrency: each invocation owns all mutable state; concurrent calls are independent.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
)

const version = "command-qualify.report/1"
const binaryCap = 128 << 20
const outputCap = 1 << 20

type pin struct {
	SHA256 string `json:"sha256"`
}
type caseResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Code   string `json:"code"`
}
type qualification struct {
	Version           string       `json:"version"`
	Profile           string       `json:"profile"`
	RosterSHA256      string       `json:"rosterSHA256"`
	GOOS              string       `json:"goos"`
	GOARCH            string       `json:"goarch"`
	Guard             pin          `json:"guard"`
	Scanner           *pin         `json:"scanner,omitempty"`
	SyntheticPeerOnly bool         `json:"syntheticPeerOnly,omitempty"`
	Required          int          `json:"required"`
	Passed            int          `json:"passed"`
	Failed            int          `json:"failed"`
	Unknown           int          `json:"unknown"`
	Status            string       `json:"status"`
	Cases             []caseResult `json:"cases"`
}

var coreIDs = []string{"read-json", "read-text", "read-compact", "ordinary-compact-fallback", "search-literal", "late-regex", "wrong-input-hash", "missing-control", "denied-prefix", "traversal", "case-alias", "reserved-name", "invalid-utf8", "file-cap", "symlink", "tiny-json", "tiny-text", "tiny-compact"}
var scannerIDs = []string{"scanner-clean", "scanner-empty", "scanner-aba-disabled", "scanner-aba-enabled", "scanner-canary", "scanner-late-pdf", "scanner-wrong-hash", "scanner-missing-scope", "scanner-ambient-ignore"}
var classifierIDs = []string{"classifier-no-concern", "classifier-concern", "classifier-unknown", "classifier-model-mismatch", "classifier-refusal", "classifier-duplicate-answer", "classifier-truncated", "classifier-unpinned-input", "classifier-policy-drift", "classifier-regex-hold", "classifier-redirect", "classifier-response-limit"}

func roster(profile string) []string {
	ids := append([]string(nil), coreIDs...)
	if profile == "files-gitleaks/1" {
		ids = append(ids, scannerIDs...)
	} else if profile == "files-loopback-fixture/1" {
		ids = append(ids, classifierIDs...)
	}
	return ids
}
func rosterHash(profile string) string {
	return rosterHashWith(profile, func(id string, i int) fixture {
		if profile == "files-loopback-fixture/1" && i >= len(coreIDs) {
			return classifierFixture(id)
		}
		if i >= len(coreIDs) {
			return scannerFixture(id)
		}
		return baseFixture(id)
	})
}

type rosterFile struct {
	Name       string `json:"name"`
	BodySHA256 string `json:"bodySHA256"`
	Symlink    string `json:"symlink"`
}
type rosterScanner struct {
	Status     string `json:"status"`
	Code       string `json:"code"`
	Expected   int    `json:"expected"`
	Attempted  int    `json:"attempted"`
	Invoked    int    `json:"invoked"`
	Completed  int    `json:"completed"`
	Submitted  int    `json:"submitted"`
	Observed   int    `json:"observed"`
	ReuseFiles int    `json:"reuseFiles"`
	ReuseBytes int    `json:"reuseBytes"`
	Finding    bool   `json:"finding"`
	Reuse      bool   `json:"reuse"`
}
type rosterCase struct {
	ID                  string            `json:"id"`
	Format              string            `json:"format"`
	Files               []rosterFile      `json:"files"`
	Paths               []string          `json:"paths"`
	Kind                string            `json:"kind"`
	Start               int               `json:"start"`
	End                 int               `json:"end"`
	Literal             string            `json:"literal"`
	MaxFile             int               `json:"maxFile"`
	MaxOutput           int               `json:"maxOutput"`
	WrongHash           bool              `json:"wrongHash"`
	MissingControl      bool              `json:"missingControl"`
	MissingScannerScope bool              `json:"missingScannerScope"`
	WrongScannerHash    bool              `json:"wrongScannerHash"`
	Reuse               bool              `json:"reuse"`
	Scanner             bool              `json:"scanner"`
	WantExit            int               `json:"wantExit"`
	WantStatus          string            `json:"wantStatus"`
	WantCode            string            `json:"wantCode"`
	WantRows            []fileRow         `json:"wantRows"`
	WantTotal           int               `json:"wantTotal"`
	WantRead            int               `json:"wantRead"`
	WantScanned         int               `json:"wantScanned"`
	WantScanner         *rosterScanner    `json:"wantScanner"`
	ActualCompact       bool              `json:"actualCompact"`
	Classifier          *rosterClassifier `json:"classifier,omitempty"`
}
type rosterControls struct {
	RulePattern             string `json:"rulePattern"`
	DeniedPrefix            string `json:"deniedPrefix"`
	MaxTotal                int    `json:"maxTotal"`
	MaxResults              int    `json:"maxResults"`
	ScannerProfile          string `json:"scannerProfile"`
	ScannerTimeoutMillis    int    `json:"scannerTimeoutMillis"`
	ScannerMaxReportBytes   int    `json:"scannerMaxReportBytes"`
	AmbientConfigSHA256     string `json:"ambientConfigSHA256"`
	ClassifierCriterion     string `json:"classifierCriterion,omitempty"`
	ClassifierModel         string `json:"classifierModel,omitempty"`
	ClassifierResponseModel string `json:"classifierResponseModel,omitempty"`
	ClassifierMaxInput      int    `json:"classifierMaxInput,omitempty"`
	ClassifierMaxResponse   int    `json:"classifierMaxResponse,omitempty"`
	ClassifierMaxTokens     int    `json:"classifierMaxTokens,omitempty"`
}

func caseDescriptor(f fixture) rosterCase {
	d := rosterCase{ID: f.id, Format: f.format, Paths: f.paths, Kind: f.kind, Start: f.start, End: f.end, Literal: f.literal, MaxFile: f.maxFile, MaxOutput: f.maxOutput, WrongHash: f.wrongHash, MissingControl: f.missingControl, MissingScannerScope: f.missingScannerScope, WrongScannerHash: f.wrongScannerHash, Reuse: f.reuse, Scanner: f.scanner, WantExit: f.wantExit, WantStatus: f.wantStatus, WantCode: f.wantCode, WantRows: f.wantRows, WantTotal: f.wantTotal, WantRead: f.wantRead, WantScanned: f.wantScanned, ActualCompact: f.actualCompact}
	for _, file := range f.files {
		d.Files = append(d.Files, rosterFile{Name: file.name, BodySHA256: sum(file.body), Symlink: file.symlink})
	}
	if w := f.wantScanner; w != nil {
		d.WantScanner = &rosterScanner{Status: w.status, Code: w.code, Expected: w.expected, Attempted: w.attempted, Invoked: w.invoked, Completed: w.completed, Submitted: w.submitted, Observed: w.observed, ReuseFiles: w.reuseFiles, ReuseBytes: w.reuseBytes, Finding: w.finding, Reuse: w.reuse}
	}
	if f.classifier {
		d.Classifier = classifierDescriptor(f)
	}
	return d
}
func rosterHashWith(profile string, pick func(string, int) fixture) string {
	d := struct {
		Version  string         `json:"version"`
		Profile  string         `json:"profile"`
		Controls rosterControls `json:"controls"`
		Cases    []rosterCase   `json:"cases"`
	}{Version: "command-qualify.roster/1", Profile: profile, Controls: rosterControls{RulePattern: qualifyRulePattern, DeniedPrefix: qualifyDeniedPrefix, MaxTotal: qualifyMaxTotal, MaxResults: qualifyMaxResults, ScannerProfile: qualifyScannerProfile, ScannerTimeoutMillis: qualifyScannerTimeoutMillis, ScannerMaxReportBytes: outputCap, AmbientConfigSHA256: sum([]byte(qualifyAmbientConfig))}}
	if profile == "files-loopback-fixture/1" {
		d.Controls.ClassifierCriterion = fixtureCriterion
		d.Controls.ClassifierModel = fixtureModel
		d.Controls.ClassifierResponseModel = fixtureResponseModel
		d.Controls.ClassifierMaxInput = fixtureMaxInput
		d.Controls.ClassifierMaxResponse = fixtureMaxResponse
		d.Controls.ClassifierMaxTokens = fixtureMaxTokens
	}
	for i, id := range roster(profile) {
		d.Cases = append(d.Cases, caseDescriptor(pick(id, i)))
	}
	b, _ := json.Marshal(d)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
func validHash(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' && c < 'a' || c > 'f' {
			return false
		}
	}
	return true
}
func newReport(profile, guardHash, scannerHash string) qualification {
	r := qualification{Version: version, Profile: profile, RosterSHA256: rosterHash(profile), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Guard: pin{guardHash}, Status: "unknown"}
	r.SyntheticPeerOnly = profile == "files-loopback-fixture/1"
	if profile == "files-gitleaks/1" {
		r.Scanner = &pin{scannerHash}
	}
	for _, id := range roster(profile) {
		r.Cases = append(r.Cases, caseResult{ID: id, Status: "unknown", Code: "unrun"})
	}
	r.Required = len(r.Cases)
	r.Unknown = r.Required
	return r
}
func (r *qualification) finish() int {
	r.Passed, r.Failed, r.Unknown = 0, 0, 0
	for _, c := range r.Cases {
		switch c.Status {
		case "pass":
			r.Passed++
		case "fail":
			r.Failed++
		default:
			r.Unknown++
		}
	}
	if r.Unknown > 0 {
		r.Status = "unknown"
		return 2
	}
	if r.Failed > 0 {
		r.Status = "fail"
		return 1
	}
	r.Status = "pass"
	return 0
}

// snapshot rechecks the opened inode and reads no more than the fixed executable cap.
// The operator trusts the executable; this copy only prevents later path replacement.
func snapshot(ctx context.Context, path, hash, dir, name string) (string, string) {
	if !filepath.IsAbs(path) || !validHash(hash) {
		return "", "invalid_pin"
	}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Size() < 1 || before.Size() > binaryCap {
		return "", "binary_unavailable"
	}
	if ctx.Err() != nil {
		return "", "deadline"
	}
	f, err := os.OpenFile(path, os.O_RDONLY|nonblock, 0)
	if err != nil {
		return "", "binary_unavailable"
	}
	defer f.Close()
	if ctx.Err() != nil {
		return "", "deadline"
	}
	after, err := f.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		return "", "binary_unavailable"
	}
	b, err := io.ReadAll(io.LimitReader(f, binaryCap+1))
	if err != nil || len(b) < 1 || len(b) > binaryCap {
		return "", "binary_unavailable"
	}
	if ctx.Err() != nil {
		return "", "deadline"
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != hash {
		return "", "hash_mismatch"
	}
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	dest := filepath.Join(dir, name)
	if err := os.WriteFile(dest, b, 0500); err != nil {
		return "", "preparation_failed"
	}
	return dest, ""
}

type boundedWriter struct {
	buf      bytes.Buffer
	overflow bool
}

func (w *boundedWriter) Len() int      { return w.buf.Len() }
func (w *boundedWriter) Bytes() []byte { return w.buf.Bytes() }

func (w *boundedWriter) Write(p []byte) (int, error) {
	if len(p) > outputCap-w.Len() {
		w.overflow = true
		return 0, errors.New("output limit")
	}
	return w.buf.Write(p)
}

type observation struct {
	stdout, stderr []byte
	exit           int
	started        bool
	code           string
}
type transport func(context.Context, string, string, []string) observation

func childEnv(home string) []string {
	env := []string{"HOME=" + home, "TMPDIR=" + home, "TMP=" + home, "TEMP=" + home, "XDG_CONFIG_HOME=" + home, "XDG_DATA_HOME=" + home, "XDG_CACHE_HOME=" + home, "NO_COLOR=1", "TERM=dumb"}
	if runtime.GOOS == "windows" {
		if s := os.Getenv("SystemRoot"); s != "" {
			env = append(env, "SystemRoot="+s)
		}
	}
	return env
}
func invoke(ctx context.Context, exe, work string, args []string) observation {
	childCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(childCtx, exe, args...)
	cmd.Dir = work
	cmd.Env = childEnv(work)
	cmd.WaitDelay = 100 * time.Millisecond
	out, errout := &boundedWriter{}, &boundedWriter{}
	cmd.Stdout, cmd.Stderr = out, errout
	if err := cmd.Start(); err != nil {
		return observation{code: "start_failed"}
	}
	err := cmd.Wait()
	if out.overflow || errout.overflow {
		return observation{started: true, code: "output_limit"}
	}
	if childCtx.Err() != nil {
		return observation{started: true, code: "deadline"}
	}
	if err == nil {
		return observation{stdout: out.Bytes(), stderr: errout.Bytes(), started: true}
	}
	if x, ok := err.(*exec.ExitError); ok {
		return observation{stdout: out.Bytes(), stderr: errout.Bytes(), exit: x.ExitCode(), started: true}
	}
	return observation{started: true, code: "wait_failed"}
}

func execute(args []string, w io.Writer, call transport) int {
	if len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") || len(args) == 2 && args[0] == "run" && (args[1] == "--help" || args[1] == "-h") {
		const usage = "command-qualify run --profile files-core/1|files-gitleaks/1|files-loopback-fixture/1 --guard ABS --guard-sha256 HASH [--scanner ABS --scanner-sha256 HASH] [--format json|text]\nExit 0: all required cases pass; 1: observed failure; 2: unknown or output failure.\nExecutables are operator-trusted; this is a diagnostic, not an installer or attestation.\n"
		if n, err := io.WriteString(w, usage); err != nil || n != len(usage) {
			return 2
		}
		return 0
	}
	if len(args) < 1 || args[0] != "run" {
		return 2
	}
	if !strictArgs(args[1:]) {
		return 2
	}
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var profile, guard, guardHash, scanner, scannerHash, format string
	fs.StringVar(&profile, "profile", "", "")
	fs.StringVar(&guard, "guard", "", "")
	fs.StringVar(&guardHash, "guard-sha256", "", "")
	fs.StringVar(&scanner, "scanner", "", "")
	fs.StringVar(&scannerHash, "scanner-sha256", "", "")
	fs.StringVar(&format, "format", "json", "")
	if fs.Parse(args[1:]) != nil || len(fs.Args()) != 0 || profile != "files-core/1" && profile != "files-gitleaks/1" && profile != "files-loopback-fixture/1" || !filepath.IsAbs(guard) || !validHash(guardHash) || format != "json" && format != "text" {
		return 2
	}
	if profile != "files-gitleaks/1" && (scanner != "" || scannerHash != "") || profile == "files-gitleaks/1" && (!filepath.IsAbs(scanner) || !validHash(scannerHash)) {
		return 2
	}
	r := newReport(profile, guardHash, scannerHash)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	work, err := os.MkdirTemp("", "command-qualify-")
	if err != nil {
		return emit(w, &r, format, "preparation_failed")
	}
	if err := os.Chmod(work, 0700); err != nil {
		_ = os.RemoveAll(work)
		return emit(w, &r, format, "preparation_failed")
	}
	guardCopy, preflight := snapshot(ctx, guard, guardHash, work, "guard")
	var scannerCopy string
	if preflight == "" && profile == "files-gitleaks/1" {
		scannerCopy, preflight = snapshot(ctx, scanner, scannerHash, work, "scanner")
	}
	if preflight == "" {
		runCases(ctx, &r, guardCopy, scannerCopy, work, call)
	}
	if err := os.RemoveAll(work); err != nil {
		preflight = "cleanup_failed"
	}
	return emit(w, &r, format, preflight)
}

func strictArgs(args []string) bool {
	allowed := map[string]bool{"--profile": true, "--guard": true, "--guard-sha256": true, "--scanner": true, "--scanner-sha256": true, "--format": true}
	seen := map[string]bool{}
	if len(args)%2 != 0 {
		return false
	}
	for i := 0; i < len(args); i += 2 {
		key := args[i]
		if !allowed[key] || seen[key] || args[i+1] == "" {
			return false
		}
		seen[key] = true
	}
	return seen["--profile"] && seen["--guard"] && seen["--guard-sha256"]
}
func emit(w io.Writer, r *qualification, format, preflight string) int {
	code := r.finish()
	if preflight != "" {
		code = 2
		r.Status = "unknown"
		if r.Unknown == 0 && len(r.Cases) > 0 {
			r.Cases[len(r.Cases)-1] = caseResult{ID: r.Cases[len(r.Cases)-1].ID, Status: "unknown", Code: preflight}
			r.finish()
		}
		for i := range r.Cases {
			if r.Cases[i].Status == "unknown" {
				r.Cases[i].Code = preflight
			}
		}
	}
	if !validQualification(*r) {
		return 2
	}
	var b []byte
	if format == "text" {
		b, _ = json.MarshalIndent(r, "", "  ")
	} else {
		b, _ = json.Marshal(r)
	}
	b = append(b, '\n')
	if len(b) > outputCap {
		return 2
	}
	if n, err := w.Write(b); err != nil || n != len(b) {
		return 2
	}
	return code
}

// validQualification is the semantic companion to the closed CUE receipt.
// It checks the compiled roster, pins, counts, and status relation.
func validQualification(r qualification) bool {
	if r.Version != version || r.Profile != "files-core/1" && r.Profile != "files-gitleaks/1" && r.Profile != "files-loopback-fixture/1" || r.RosterSHA256 != rosterHash(r.Profile) || r.GOOS == "" || r.GOARCH == "" || !validHash(r.Guard.SHA256) || r.SyntheticPeerOnly != (r.Profile == "files-loopback-fixture/1") {
		return false
	}
	if r.Profile != "files-gitleaks/1" && r.Scanner != nil || r.Profile == "files-gitleaks/1" && (r.Scanner == nil || !validHash(r.Scanner.SHA256)) {
		return false
	}
	ids := roster(r.Profile)
	if r.Required != len(ids) || len(r.Cases) != len(ids) || r.Passed+r.Failed+r.Unknown != r.Required {
		return false
	}
	pass, fail, unknown := 0, 0, 0
	unknownCodes := map[string]bool{"unrun": true, "setup_failed": true, "start_failed": true, "wait_failed": true, "deadline": true, "output_limit": true, "preparation_failed": true, "cleanup_failed": true, "binary_unavailable": true, "hash_mismatch": true}
	for i, c := range r.Cases {
		if c.ID != ids[i] {
			return false
		}
		switch c.Status {
		case "pass":
			pass++
			if c.Code != "oracle_met" {
				return false
			}
		case "fail":
			fail++
			if c.Code != "oracle_mismatch" {
				return false
			}
		case "unknown":
			unknown++
			if !unknownCodes[c.Code] {
				return false
			}
		default:
			return false
		}
	}
	if pass != r.Passed || fail != r.Failed || unknown != r.Unknown {
		return false
	}
	return r.Status == "pass" && unknown == 0 && fail == 0 || r.Status == "fail" && unknown == 0 && fail > 0 || r.Status == "unknown" && unknown > 0
}
func main() { os.Exit(execute(os.Args[1:], os.Stdout, invoke)) }

func validJSON(b []byte, out any) bool {
	if !jsonwire.ValidUnicode(b) {
		return false
	}
	d := json.NewDecoder(bytes.NewReader(b))
	if !walkJSON(d, 0) {
		return false
	}
	if _, err := d.Token(); err != io.EOF {
		return false
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	return d.Decode(out) == nil
}
func walkJSON(d *json.Decoder, depth int) bool {
	if depth > 24 {
		return false
	}
	t, err := d.Token()
	if err != nil || t == nil {
		return false
	}
	switch t {
	case json.Delim('{'):
		seen := map[string]bool{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return false
			}
			s, ok := k.(string)
			if !ok || seen[s] {
				return false
			}
			seen[s] = true
			if !walkJSON(d, depth+1) {
				return false
			}
		}
		_, err = d.Token()
		return err == nil
	case json.Delim('['):
		for d.More() {
			if !walkJSON(d, depth+1) {
				return false
			}
		}
		_, err = d.Token()
		return err == nil
	default:
		return true
	}
}
